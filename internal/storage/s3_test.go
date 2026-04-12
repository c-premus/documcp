package storage_test

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"

	"github.com/c-premus/documcp/internal/storage"
)

// TestS3Blob runs the shared Blob contract against gofakes3 — a pure-Go,
// in-process S3 mock. No Docker, no containers, fast enough to run in the
// default unit-test tier. Each subtest spins up its own gofakes3 server and
// bucket so state doesn't leak between cases.
func TestS3Blob(t *testing.T) {
	t.Parallel()
	RunBlobSuite(t, func(t *testing.T) storage.Blob {
		t.Helper()
		return newGofakes3Blob(t)
	})
}

// newGofakes3Blob spins up a gofakes3 HTTP server, creates a bucket, and
// returns an S3Blob pointed at it. Server and blob are cleaned up via t.Cleanup.
func newGofakes3Blob(t *testing.T) storage.Blob {
	t.Helper()

	backend := s3mem.New()
	faker := gofakes3.New(backend)
	ts := httptest.NewServer(faker.Server())
	t.Cleanup(ts.Close)

	const (
		bucket    = "test-bucket"
		region    = "us-east-1"
		accessKey = "TESTKEY"
		secretKey = "TESTSECRET"
	)

	ctx := context.Background()

	// Create the bucket via aws-sdk-go-v2 so gofakes3 registers it.
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		t.Fatalf("aws config: %v", err)
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(ts.URL)
		o.UsePathStyle = true
	})
	if _, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil {
		t.Fatalf("create bucket: %v", err)
	}

	blob, err := storage.NewS3Blob(ctx, storage.S3Config{
		Endpoint:        ts.URL,
		Bucket:          bucket,
		Region:          region,
		AccessKeyID:     accessKey,
		SecretAccessKey: secretKey,
		UsePathStyle:    true,
	})
	if err != nil {
		t.Fatalf("NewS3Blob: %v", err)
	}
	t.Cleanup(func() { _ = blob.Close() })
	return blob
}

// TestNewS3Blob_RequiresFields covers the constructor validation.
func TestNewS3Blob_RequiresFields(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tests := []struct {
		name string
		cfg  storage.S3Config
	}{
		{"missing bucket", storage.S3Config{Region: "us-east-1", AccessKeyID: "k", SecretAccessKey: "s"}},
		{"missing region", storage.S3Config{Bucket: "b", AccessKeyID: "k", SecretAccessKey: "s"}},
		{"missing access key", storage.S3Config{Bucket: "b", Region: "us-east-1", SecretAccessKey: "s"}},
		{"missing secret key", storage.S3Config{Bucket: "b", Region: "us-east-1", AccessKeyID: "k"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := storage.NewS3Blob(ctx, tt.cfg); err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}
