package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"gocloud.dev/blob"
	"gocloud.dev/blob/s3blob"
	"gocloud.dev/gcerrors"
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

// S3Blob wraps gocloud.dev/blob/s3blob to speak the Blob interface.
// Works against AWS S3, Cloudflare R2, Backblaze B2, Wasabi, Garage,
// SeaweedFS — any service that implements the S3 API.
type S3Blob struct {
	bucket *blob.Bucket
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
	})

	bucket, err := s3blob.OpenBucketV2(ctx, client, cfg.Bucket, &s3blob.Options{
		// Third-party S3 providers (Garage, SeaweedFS) don't support
		// SDK-side checksum enforcement — set to when_required so we
		// only send checksums when the service asks for them.
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
	})
	if err != nil {
		return nil, fmt.Errorf("s3blob: open bucket %q: %w", cfg.Bucket, err)
	}

	return &S3Blob{bucket: bucket}, nil
}

// NewWriter returns a writer that uploads to the object at key on Close.
func (b *S3Blob) NewWriter(ctx context.Context, key string, opts *WriterOpts) (io.WriteCloser, error) {
	if err := ValidateKey(key); err != nil {
		return nil, err
	}
	var wopts *blob.WriterOptions
	if opts != nil && opts.ContentType != "" {
		wopts = &blob.WriterOptions{ContentType: opts.ContentType}
	}
	w, err := b.bucket.NewWriter(ctx, key, wopts)
	if err != nil {
		return nil, mapS3Err("s3blob: new writer", err)
	}
	return w, nil
}

// NewReader opens the object for streaming reads.
func (b *S3Blob) NewReader(ctx context.Context, key string) (io.ReadCloser, error) {
	if err := ValidateKey(key); err != nil {
		return nil, err
	}
	r, err := b.bucket.NewReader(ctx, key, nil)
	if err != nil {
		return nil, mapS3Err("s3blob: new reader", err)
	}
	return r, nil
}

// NewRangeReader reads length bytes starting at offset.
func (b *S3Blob) NewRangeReader(ctx context.Context, key string, offset, length int64) (io.ReadCloser, error) {
	if err := ValidateKey(key); err != nil {
		return nil, err
	}
	if offset < 0 {
		return nil, fmt.Errorf("s3blob: negative offset %d", offset)
	}
	r, err := b.bucket.NewRangeReader(ctx, key, offset, length, nil)
	if err != nil {
		return nil, mapS3Err("s3blob: new range reader", err)
	}
	return r, nil
}

// Delete removes the object. Returns nil when already absent.
func (b *S3Blob) Delete(ctx context.Context, key string) error {
	if err := ValidateKey(key); err != nil {
		return err
	}
	if err := b.bucket.Delete(ctx, key); err != nil {
		if gcerrors.Code(err) == gcerrors.NotFound {
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
	attrs, err := b.bucket.Attributes(ctx, key)
	if err != nil {
		return Attrs{}, mapS3Err("s3blob: attributes", err)
	}
	return Attrs{
		Size:        attrs.Size,
		ModTime:     attrs.ModTime,
		ContentType: attrs.ContentType,
		ETag:        attrs.ETag,
	}, nil
}

// List returns an iterator over keys with the given prefix.
func (b *S3Blob) List(_ context.Context, prefix string) Iterator {
	return &s3Iterator{
		it: b.bucket.List(&blob.ListOptions{Prefix: prefix}),
	}
}

// Close releases the bucket's underlying connections.
func (b *S3Blob) Close() error { return b.bucket.Close() }

// mapS3Err converts a gocloud error code of NotFound into fs.ErrNotExist so
// callers can use errors.Is(err, fs.ErrNotExist). Other errors pass through.
func mapS3Err(ctx string, err error) error {
	if gcerrors.Code(err) == gcerrors.NotFound {
		return fmt.Errorf("%s: %w", ctx, errors.Join(fs.ErrNotExist, err))
	}
	return fmt.Errorf("%s: %w", ctx, err)
}

// s3Iterator wraps gocloud's ListIterator.
type s3Iterator struct {
	it     *blob.ListIterator
	curKey string
	err    error
}

// Next advances to the next key. Returns false at EOF or on error.
func (it *s3Iterator) Next(ctx context.Context) bool {
	if it.err != nil {
		return false
	}
	for {
		obj, err := it.it.Next(ctx)
		if errors.Is(err, io.EOF) {
			return false
		}
		if err != nil {
			it.err = err
			return false
		}
		// Skip synthetic directory entries (prefixes); yield only keys.
		if obj.IsDir {
			continue
		}
		it.curKey = obj.Key
		return true
	}
}

// Key returns the current key. Only valid after Next returns true.
func (it *s3Iterator) Key() string { return it.curKey }

// Err returns the first error encountered during iteration, if any.
func (it *s3Iterator) Err() error { return it.err }
