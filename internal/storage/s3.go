package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithy "github.com/aws/smithy-go"
)

// S3Config configures an S3-compatible object-storage backend.
type S3Config struct {
	// Endpoint is the service URL. Leave empty to use AWS's default
	// regional endpoint. Required for Garage, SeaweedFS, R2, B2, etc.
	Endpoint string

	// Bucket is the target bucket name. Required.
	Bucket string

	// Region is the AWS region. Required; SigV4 signing needs it even for
	// non-AWS backends ("us-east-1" is a safe placeholder for Garage/SeaweedFS).
	Region string

	// AccessKeyID and SecretAccessKey are static credentials. Required —
	// we don't fall back to the default AWS credential chain because
	// containerized deployments rarely have EC2/EKS metadata endpoints.
	AccessKeyID     string
	SecretAccessKey string

	// UsePathStyle forces path-style addressing (host/bucket/key) instead
	// of virtual-host style (bucket.host/key). Default true — most
	// self-hosted S3-compatible services require it.
	UsePathStyle bool

	// ForceSSL, if true, requires the endpoint URL to use https://.
	// Purely advisory — validation happens at Open time.
	ForceSSL bool
}

// S3Blob is a Blob backend talking directly to an S3-compatible service via
// aws-sdk-go-v2. Works against AWS S3, Cloudflare R2, Backblaze B2, Wasabi,
// Garage, SeaweedFS — any service that implements the S3 API.
type S3Blob struct {
	client   *s3.Client
	uploader *manager.Uploader
	bucket   string
}

// NewS3Blob opens an S3-compatible bucket.
func NewS3Blob(ctx context.Context, cfg S3Config) (*S3Blob, error) {
	if cfg.Bucket == "" {
		return nil, errors.New("s3blob: Bucket is required")
	}
	if cfg.Region == "" {
		return nil, errors.New("s3blob: Region is required")
	}
	if cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" {
		return nil, errors.New("s3blob: AccessKeyID and SecretAccessKey are required")
	}
	if cfg.ForceSSL && cfg.Endpoint != "" && !strings.HasPrefix(cfg.Endpoint, "https://") {
		return nil, errors.New("s3blob: Endpoint must use https:// when ForceSSL is true")
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID, cfg.SecretAccessKey, "",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("s3blob: load AWS config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
		o.UsePathStyle = cfg.UsePathStyle
		// Third-party S3 providers (Garage, SeaweedFS) don't support
		// SDK-side checksum enforcement — only send checksums when the
		// service explicitly asks for them.
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
	})

	return &S3Blob{
		client:   client,
		uploader: manager.NewUploader(client),
		bucket:   cfg.Bucket,
	}, nil
}

// NewWriter returns a writer that uploads to the object at key on Close.
// Internally it pipes writes into manager.Uploader, which handles multipart
// chunking transparently. Callers must Close the writer; abandoning it leaks
// the background upload goroutine until Close (or the request context) cancels it.
func (b *S3Blob) NewWriter(ctx context.Context, key string, opts *WriterOpts) (io.WriteCloser, error) {
	if err := ValidateKey(key); err != nil {
		return nil, err
	}
	pr, pw := io.Pipe()
	input := &s3.PutObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(key),
		Body:   pr,
	}
	if opts != nil && opts.ContentType != "" {
		input.ContentType = aws.String(opts.ContentType)
	}
	w := &s3Writer{pw: pw, done: make(chan error, 1)}
	go func() {
		_, err := b.uploader.Upload(ctx, input)
		// Closing the pipe reader unblocks any pending Write on pw with
		// io.ErrClosedPipe — matters when Upload errors mid-stream.
		_ = pr.CloseWithError(err)
		w.done <- err
	}()
	return w, nil
}

// s3Writer pipes Writes into a background manager.Uploader.Upload.
type s3Writer struct {
	pw   *io.PipeWriter
	done chan error
}

func (w *s3Writer) Write(p []byte) (int, error) { return w.pw.Write(p) }

// Close signals end-of-input and waits for the background upload to finish.
// Close errors propagate the upload error if any.
func (w *s3Writer) Close() error {
	if err := w.pw.Close(); err != nil {
		<-w.done
		return err
	}
	return <-w.done
}

// NewReader opens the object for streaming reads.
func (b *S3Blob) NewReader(ctx context.Context, key string) (io.ReadCloser, error) {
	if err := ValidateKey(key); err != nil {
		return nil, err
	}
	out, err := b.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, mapS3Err("s3blob: new reader", err)
	}
	return out.Body, nil
}

// NewRangeReader reads length bytes starting at offset. A negative length
// reads to end-of-object.
func (b *S3Blob) NewRangeReader(ctx context.Context, key string, offset, length int64) (io.ReadCloser, error) {
	if err := ValidateKey(key); err != nil {
		return nil, err
	}
	if offset < 0 {
		return nil, fmt.Errorf("s3blob: negative offset %d", offset)
	}
	var rangeHeader string
	if length < 0 {
		rangeHeader = fmt.Sprintf("bytes=%d-", offset)
	} else {
		rangeHeader = fmt.Sprintf("bytes=%d-%d", offset, offset+length-1)
	}
	out, err := b.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(key),
		Range:  aws.String(rangeHeader),
	})
	if err != nil {
		return nil, mapS3Err("s3blob: new range reader", err)
	}
	return out.Body, nil
}

// Delete removes the object. Returns nil when already absent.
func (b *S3Blob) Delete(ctx context.Context, key string) error {
	if err := ValidateKey(key); err != nil {
		return err
	}
	_, err := b.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		// S3 itself returns 204 even on missing keys; some compatible services
		// (older Garage, SeaweedFS variants) propagate NoSuchKey instead.
		if isNotFound(err) {
			return nil
		}
		return mapS3Err("s3blob: delete", err)
	}
	return nil
}

// Stat returns metadata for the object.
func (b *S3Blob) Stat(ctx context.Context, key string) (Attrs, error) {
	if err := ValidateKey(key); err != nil {
		return Attrs{}, err
	}
	out, err := b.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return Attrs{}, mapS3Err("s3blob: head", err)
	}
	attrs := Attrs{}
	if out.ContentLength != nil {
		attrs.Size = *out.ContentLength
	}
	if out.LastModified != nil {
		attrs.ModTime = *out.LastModified
	}
	if out.ContentType != nil {
		attrs.ContentType = *out.ContentType
	}
	if out.ETag != nil {
		attrs.ETag = *out.ETag
	}
	return attrs, nil
}

// List returns an iterator over keys with the given prefix.
func (b *S3Blob) List(_ context.Context, prefix string) Iterator {
	input := &s3.ListObjectsV2Input{Bucket: aws.String(b.bucket)}
	if prefix != "" {
		input.Prefix = aws.String(prefix)
	}
	return &s3Iterator{
		paginator: s3.NewListObjectsV2Paginator(b.client, input),
	}
}

// Close releases resources. The aws-sdk-go-v2 client itself doesn't hold any
// long-lived connections that need closing — the underlying http.Client uses
// the default idle-connection pool — so this is a no-op kept to satisfy Blob.
func (b *S3Blob) Close() error { return nil }

// mapS3Err converts NoSuchKey / NotFound errors into ones that wrap fs.ErrNotExist
// so callers can use errors.Is(err, fs.ErrNotExist). Other errors pass through.
func mapS3Err(ctx string, err error) error {
	if isNotFound(err) {
		return fmt.Errorf("%s: %w", ctx, errors.Join(fs.ErrNotExist, err))
	}
	return fmt.Errorf("%s: %w", ctx, err)
}

// isNotFound recognizes the various ways aws-sdk-go-v2 reports a missing
// object: typed NoSuchKey on GetObject/DeleteObject, generic APIError with
// code "NoSuchKey" or "NotFound" on HeadObject (which has no response body
// for the SDK to deserialize into a typed error).
func isNotFound(err error) bool {
	if _, ok := errors.AsType[*s3types.NoSuchKey](err); ok {
		return true
	}
	if apiErr, ok := errors.AsType[smithy.APIError](err); ok {
		switch apiErr.ErrorCode() {
		case "NoSuchKey", "NotFound":
			return true
		}
	}
	return false
}

// s3Iterator wraps a paginator and yields keys lazily across pages.
type s3Iterator struct {
	paginator *s3.ListObjectsV2Paginator
	page      []s3types.Object
	idx       int
	curKey    string
	err       error
}

// Next advances to the next key. Returns false at EOF or on error.
func (it *s3Iterator) Next(ctx context.Context) bool {
	if it.err != nil {
		return false
	}
	for {
		if it.idx < len(it.page) {
			obj := it.page[it.idx]
			it.idx++
			if obj.Key == nil {
				continue
			}
			it.curKey = *obj.Key
			return true
		}
		if !it.paginator.HasMorePages() {
			return false
		}
		out, err := it.paginator.NextPage(ctx)
		if err != nil {
			it.err = err
			return false
		}
		it.page = out.Contents
		it.idx = 0
	}
}

// Key returns the current key. Only valid after Next returns true.
func (it *s3Iterator) Key() string { return it.curKey }

// Err returns the first error encountered during iteration, if any.
func (it *s3Iterator) Err() error { return it.err }
