// Package s3 keeps blobs (chat files, huddle recordings) in an S3-compatible bucket (AWS S3, MinIO, …) so that
// every replica sees them (S82). It shares the bucket with attachments under its own key prefix.
package s3

import (
	"context"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/bakhod1r/kyber/internal/platform/blob"
)

// partSize bounds what one upload buffers (S3's minimum part size).
const partSize = 5 << 20

// Config locates the bucket; Endpoint is host[:port] without a scheme; Prefix (e.g. "chat/") starts every key.
type Config struct {
	Endpoint, Bucket, Region, AccessKey, SecretKey, Prefix string
	TLS                                                    bool
}

type Store struct {
	client         *minio.Client
	bucket, prefix string
}

// New connects to the bucket, which must exist.
func New(ctx context.Context, c Config) (*Store, error) {
	client, err := minio.New(c.Endpoint, &minio.Options{Creds: credentials.NewStaticV4(c.AccessKey, c.SecretKey, ""), Secure: c.TLS, Region: c.Region})
	if err != nil {
		return nil, fmt.Errorf("s3 client: %w", err)
	}
	ok, err := client.BucketExists(ctx, c.Bucket)
	if err != nil {
		return nil, fmt.Errorf("s3 bucket %q: %w", c.Bucket, err)
	}
	if !ok {
		return nil, fmt.Errorf("s3 bucket %q does not exist", c.Bucket)
	}
	return &Store{client: client, bucket: c.Bucket, prefix: c.Prefix}, nil
}

func (s *Store) Put(ctx context.Context, key string, r io.Reader) error {
	if !blob.ValidKey(key) {
		return blob.ErrInvalidKey
	}
	_, err := s.client.PutObject(ctx, s.bucket, s.prefix+key, r, -1,
		minio.PutObjectOptions{ContentType: "application/octet-stream", PartSize: partSize, DisableContentSha256: true})
	return err
}

func (s *Store) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	if !blob.ValidKey(key) {
		return nil, blob.ErrNotFound
	}
	if _, err := s.client.StatObject(ctx, s.bucket, s.prefix+key, minio.StatObjectOptions{}); err != nil {
		if minio.ToErrorResponse(err).Code == minio.NoSuchKey {
			return nil, blob.ErrNotFound
		}
		return nil, err
	}
	return s.client.GetObject(ctx, s.bucket, s.prefix+key, minio.GetObjectOptions{})
}

// Delete is idempotent.
func (s *Store) Delete(ctx context.Context, key string) error {
	if !blob.ValidKey(key) {
		return blob.ErrInvalidKey
	}
	return s.client.RemoveObject(ctx, s.bucket, s.prefix+key, minio.RemoveObjectOptions{})
}
