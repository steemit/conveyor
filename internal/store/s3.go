package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Store is a BlobStore backed by AWS S3, mirroring TS's s3-blob-store.
// Credentials and region are resolved via the default AWS SDK chain
// (env vars, shared credentials file, IAM role).
type S3Store struct {
	client *s3.Client
	bucket string
}

// NewS3Store creates an S3-backed store for the given bucket. It loads AWS
// config via the default chain (AWS_REGION, AWS_ACCESS_KEY_ID, etc.).
func NewS3Store(ctx context.Context, bucket string) (*S3Store, error) {
	if bucket == "" {
		return nil, errors.New("s3 bucket is required")
	}
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	return &S3Store{
		client: s3.NewFromConfig(cfg),
		bucket: bucket,
	}, nil
}

func (s *S3Store) Read(ctx context.Context, key string) ([]byte, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	defer out.Body.Close()

	// Read fully into memory (blobs are small JSON — drafts, feature-flags).
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(out.Body); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (s *S3Store) SafeRead(ctx context.Context, key string) ([]byte, error) {
	b, err := s.Read(ctx, key)
	if err == ErrNotFound {
		return nil, nil
	}
	return b, err
}

func (s *S3Store) Write(ctx context.Context, key string, data []byte) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),
	})
	return err
}

func (s *S3Store) ReadJSON(ctx context.Context, key string, target any) error {
	return readJSONFrom(ctx, s, key, target)
}

func (s *S3Store) WriteJSON(ctx context.Context, key string, value any) error {
	return writeJSONTo(ctx, s, key, value)
}
