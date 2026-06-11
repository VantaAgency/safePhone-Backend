package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

// MediaContent is a (possibly partial) read of a stored media object — enough
// for the HTTP handler to answer a Range request and let a browser stream
// video instead of downloading the whole file first.
type MediaContent struct {
	Body          io.ReadCloser
	ContentType   string
	ContentLength int64  // bytes in Body; -1 if unknown
	ContentRange  string // "bytes start-end/total" when Partial
	Partial       bool   // true → respond 206
}

// parseByteRange handles a single "bytes=start-end" / "bytes=start-" /
// "bytes=-suffix" range against a known size. ok=false when unparseable or
// unsatisfiable (caller then serves the whole object).
func parseByteRange(header string, size int64) (start, end int64, ok bool) {
	if !strings.HasPrefix(header, "bytes=") || size <= 0 {
		return 0, 0, false
	}
	spec := strings.TrimPrefix(header, "bytes=")
	if spec == "" || strings.Contains(spec, ",") {
		return 0, 0, false
	}
	dash := strings.IndexByte(spec, '-')
	if dash < 0 {
		return 0, 0, false
	}
	startStr, endStr := spec[:dash], spec[dash+1:]
	if startStr == "" {
		n, err := strconv.ParseInt(endStr, 10, 64)
		if err != nil || n <= 0 {
			return 0, 0, false
		}
		if n > size {
			n = size
		}
		return size - n, size - 1, true
	}
	start, err := strconv.ParseInt(startStr, 10, 64)
	if err != nil || start < 0 || start >= size {
		return 0, 0, false
	}
	end = size - 1
	if endStr != "" {
		end, err = strconv.ParseInt(endStr, 10, 64)
		if err != nil || end < start {
			return 0, 0, false
		}
	}
	if end >= size {
		end = size - 1
	}
	return start, end, true
}

type limitedReadCloser struct {
	r io.Reader
	c io.Closer
}

func (l *limitedReadCloser) Read(p []byte) (int, error) { return l.r.Read(p) }
func (l *limitedReadCloser) Close() error               { return l.c.Close() }

func bytesReader(data []byte) io.Reader { return bytes.NewReader(data) }

// VerificationMediaStore persists and retrieves device verification
// photos + videos uploaded during the plans v2 sign-up flow.
//
// Keys live under <prefix>/<userID>/<uuid>.<ext> so a) the admin tab
// can render every upload via a single token, and b) authorization
// is trivial: the GET handler checks the token's userID matches the
// authenticated user (or the user is admin).
type VerificationMediaStore interface {
	Store(ctx context.Context, userID, ext, contentType string, data []byte) (token string, err error)
	Open(ctx context.Context, token string) (io.ReadCloser, string, error)
	// OpenRange reads an object honoring an HTTP Range header (empty = whole
	// object), so video can stream / seek.
	OpenRange(ctx context.Context, token, rangeHeader string) (*MediaContent, error)
}

// LocalVerificationMediaStore writes to disk. Used in dev and as the
// S3 store's fallback for local-only operations.
type LocalVerificationMediaStore struct {
	Root string
}

func NewLocalVerificationMediaStore(root string) *LocalVerificationMediaStore {
	return &LocalVerificationMediaStore{Root: root}
}

func (s *LocalVerificationMediaStore) Store(_ context.Context, userID, ext, contentType string, data []byte) (string, error) {
	dir := filepath.Join(s.Root, userID)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("failed to prepare verification upload dir: %w", err)
	}
	name := strings.ReplaceAll(uuid.NewString(), "-", "") + ext
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o640); err != nil {
		return "", fmt.Errorf("failed to store verification media: %w", err)
	}
	// Also persist the content-type alongside so Open can return it.
	if contentType != "" {
		_ = os.WriteFile(path+".ct", []byte(contentType), 0o640)
	}
	return userID + "/" + name, nil
}

func (s *LocalVerificationMediaStore) Open(_ context.Context, token string) (io.ReadCloser, string, error) {
	path := filepath.Join(s.Root, filepath.FromSlash(token))

	// Defense in depth: filepath.Join cleans "..", so a malformed token can
	// resolve outside Root. Reject anything that escapes the store root before
	// touching the filesystem, so callers can't reintroduce a traversal bug.
	rootAbs, err := filepath.Abs(s.Root)
	if err != nil {
		return nil, "", err
	}
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return nil, "", err
	}
	if pathAbs != rootAbs && !strings.HasPrefix(pathAbs, rootAbs+string(os.PathSeparator)) {
		return nil, "", fmt.Errorf("verification media token escapes store root: %q", token)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	ct := ""
	if b, err := os.ReadFile(path + ".ct"); err == nil {
		ct = strings.TrimSpace(string(b))
	}
	return f, ct, nil
}

func (s *LocalVerificationMediaStore) OpenRange(_ context.Context, token, rangeHeader string) (*MediaContent, error) {
	path := filepath.Join(s.Root, filepath.FromSlash(token))
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	ct := ""
	if b, err := os.ReadFile(path + ".ct"); err == nil {
		ct = strings.TrimSpace(string(b))
	}
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	size := fi.Size()
	if rangeHeader != "" {
		if start, end, ok := parseByteRange(rangeHeader, size); ok {
			if _, err := f.Seek(start, io.SeekStart); err != nil {
				f.Close()
				return nil, err
			}
			return &MediaContent{
				Body:          &limitedReadCloser{r: io.LimitReader(f, end-start+1), c: f},
				ContentType:   ct,
				ContentLength: end - start + 1,
				ContentRange:  fmt.Sprintf("bytes %d-%d/%d", start, end, size),
				Partial:       true,
			}, nil
		}
	}
	return &MediaContent{Body: f, ContentType: ct, ContentLength: size}, nil
}

type S3VerificationMediaStoreConfig struct {
	Endpoint       string
	Region         string
	Bucket         string
	AccessKeyID    string
	SecretKey      string
	Prefix         string
	ForcePathStyle bool
	Fallback       VerificationMediaStore
}

type S3VerificationMediaStore struct {
	client   *s3.Client
	bucket   string
	prefix   string
	fallback VerificationMediaStore
}

func NewS3VerificationMediaStore(ctx context.Context, cfg S3VerificationMediaStoreConfig) (*S3VerificationMediaStore, error) {
	region := strings.TrimSpace(cfg.Region)
	if region == "" {
		region = "auto"
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID, cfg.SecretKey, "",
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
		options.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
	})
	return &S3VerificationMediaStore{
		client:   client,
		bucket:   strings.TrimSpace(cfg.Bucket),
		prefix:   cleanS3Prefix(cfg.Prefix),
		fallback: cfg.Fallback,
	}, nil
}

func (s *S3VerificationMediaStore) Store(ctx context.Context, userID, ext, contentType string, data []byte) (string, error) {
	name := strings.ReplaceAll(uuid.NewString(), "-", "") + ext
	rel := userID + "/" + name
	key := rel
	if s.prefix != "" {
		key = s.prefix + "/" + rel
	}
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytesReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("failed to store verification media in bucket: %w", err)
	}
	return rel, nil
}

func (s *S3VerificationMediaStore) Open(ctx context.Context, token string) (io.ReadCloser, string, error) {
	key := token
	if s.prefix != "" {
		key = s.prefix + "/" + token
	}
	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if s.fallback != nil {
			return s.fallback.Open(ctx, token)
		}
		return nil, "", fmt.Errorf("failed to read verification media from bucket: %w", err)
	}
	ct := ""
	if output.ContentType != nil {
		ct = *output.ContentType
	}
	return output.Body, ct, nil
}

func (s *S3VerificationMediaStore) OpenRange(ctx context.Context, token, rangeHeader string) (*MediaContent, error) {
	key := token
	if s.prefix != "" {
		key = s.prefix + "/" + token
	}
	in := &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}
	if rangeHeader != "" {
		in.Range = aws.String(rangeHeader)
	}
	output, err := s.client.GetObject(ctx, in)
	if err != nil {
		if s.fallback != nil {
			return s.fallback.OpenRange(ctx, token, rangeHeader)
		}
		return nil, fmt.Errorf("failed to read verification media from bucket: %w", err)
	}
	mc := &MediaContent{Body: output.Body, ContentLength: -1}
	if output.ContentType != nil {
		mc.ContentType = *output.ContentType
	}
	if output.ContentLength != nil {
		mc.ContentLength = *output.ContentLength
	}
	if output.ContentRange != nil && *output.ContentRange != "" {
		mc.ContentRange = *output.ContentRange
		mc.Partial = true
	}
	return mc, nil
}
