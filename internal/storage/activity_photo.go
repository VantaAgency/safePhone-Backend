// Package storage provides file/object storage adapters.
package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

// ActivityPhotoStore persists and retrieves commercial activity photos.
type ActivityPhotoStore interface {
	Store(ctx context.Context, data []byte, contentType, extension string) (string, error)
	Open(ctx context.Context, storagePath string) (io.ReadCloser, error)
}

// LocalActivityPhotoStore stores photos on the local filesystem.
type LocalActivityPhotoStore struct {
	Root string
}

func NewLocalActivityPhotoStore(root string) *LocalActivityPhotoStore {
	return &LocalActivityPhotoStore{Root: root}
}

func (s *LocalActivityPhotoStore) Store(_ context.Context, data []byte, _ string, extension string) (string, error) {
	if err := os.MkdirAll(s.Root, 0o750); err != nil {
		return "", fmt.Errorf("failed to prepare upload storage: %w", err)
	}
	name := strings.ReplaceAll(uuid.NewString(), "-", "") + extension
	path := filepath.Join(s.Root, name)
	if err := os.WriteFile(path, data, 0o640); err != nil {
		return "", fmt.Errorf("failed to store photo: %w", err)
	}
	return path, nil
}

func (s *LocalActivityPhotoStore) Open(_ context.Context, storagePath string) (io.ReadCloser, error) {
	return os.Open(storagePath)
}

type S3ActivityPhotoStoreConfig struct {
	Endpoint       string
	Region         string
	Bucket         string
	AccessKeyID    string
	SecretKey      string
	Prefix         string
	ForcePathStyle bool
	Fallback       ActivityPhotoStore
}

// S3ActivityPhotoStore stores photos in an S3-compatible bucket, such as Railway Buckets.
type S3ActivityPhotoStore struct {
	client   *s3.Client
	bucket   string
	prefix   string
	fallback ActivityPhotoStore
}

func NewS3ActivityPhotoStore(ctx context.Context, cfg S3ActivityPhotoStoreConfig) (*S3ActivityPhotoStore, error) {
	region := strings.TrimSpace(cfg.Region)
	if region == "" {
		region = "auto"
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretKey,
			"",
		)),
	)
	if err != nil {
		return nil, err
	}

	endpoint := strings.TrimRight(strings.TrimSpace(cfg.Endpoint), "/")
	client := s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		if endpoint != "" {
			options.BaseEndpoint = aws.String(endpoint)
		}
		options.UsePathStyle = cfg.ForcePathStyle
	})

	return &S3ActivityPhotoStore{
		client:   client,
		bucket:   strings.TrimSpace(cfg.Bucket),
		prefix:   cleanS3Prefix(cfg.Prefix),
		fallback: cfg.Fallback,
	}, nil
}

func (s *S3ActivityPhotoStore) Store(ctx context.Context, data []byte, contentType, extension string) (string, error) {
	key := s.newKey(extension)
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("failed to store photo in bucket: %w", err)
	}
	return s3StoragePath(s.bucket, key), nil
}

func (s *S3ActivityPhotoStore) Open(ctx context.Context, storagePath string) (io.ReadCloser, error) {
	bucket, key, ok := parseS3StoragePath(storagePath)
	if !ok {
		if s.fallback != nil {
			return s.fallback.Open(ctx, storagePath)
		}
		return nil, fmt.Errorf("invalid s3 storage path")
	}
	if bucket == "" {
		bucket = s.bucket
	}
	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to read photo from bucket: %w", err)
	}
	return output.Body, nil
}

func (s *S3ActivityPhotoStore) newKey(extension string) string {
	name := strings.ReplaceAll(uuid.NewString(), "-", "") + extension
	if s.prefix == "" {
		return name
	}
	return s.prefix + "/" + name
}

func cleanS3Prefix(prefix string) string {
	return strings.Trim(strings.TrimSpace(prefix), "/")
}

func s3StoragePath(bucket, key string) string {
	return "s3://" + bucket + "/" + strings.TrimLeft(key, "/")
}

func parseS3StoragePath(storagePath string) (string, string, bool) {
	if !strings.HasPrefix(storagePath, "s3://") {
		return "", "", false
	}
	rest := strings.TrimPrefix(storagePath, "s3://")
	bucket, key, found := strings.Cut(rest, "/")
	if !found || strings.TrimSpace(key) == "" {
		return "", "", false
	}
	return bucket, key, true
}
