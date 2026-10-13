// Package blobs3 stores attachment contents in an S3-compatible bucket (AWS S3, MinIO, Ceph,
// ...) under the key <workspace>/<id>.
package blobs3

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/bakhod1r/kyber/internal/attachment/domain"
	kid "github.com/bakhod1r/kyber/internal/platform/id"
	"github.com/bakhod1r/kyber/internal/platform/tenant"
)

// partSize bounds the memory one upload buffers (S3's minimum part size).
const partSize = 5 << 20

// Config locates the bucket. Endpoint is host[:port] without a scheme.
type Config struct {
	Endpoint, Bucket, Region, AccessKey, SecretKey string
	TLS                                            bool
}

type Blobs struct {
	client *minio.Client
	bucket string
}

// New connects to the bucket, which must exist.
func New(ctx context.Context, c Config) (*Blobs, error) {
	client, err := minio.New(c.Endpoint, &minio.Options{
		Creds: credentials.NewStaticV4(c.AccessKey, c.SecretKey, ""), Secure: c.TLS, Region: c.Region,
	})
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
	return &Blobs{client: client, bucket: c.Bucket}, nil
}

// key places a blob; ids and workspaces are UUIDs.
func key(ctx context.Context, id domain.ID) (string, error) {
	ws := tenant.Workspace(ctx)
	if !kid.Valid(string(id)) || !kid.Valid(ws) {
		return "", domain.ErrAttachmentNotFound
	}
	return ws + "/" + string(id), nil
}

func (b *Blobs) Put(ctx context.Context, id domain.ID, r io.Reader) (int64, string, error) {
	k, err := key(ctx, id)
	if err != nil {
		return 0, "", err
	}
	h := sha256.New()
	info, err := b.client.PutObject(ctx, b.bucket, k, io.TeeReader(r, h), -1,
		// Unsigned payload: TLS protects the bytes; we hash them ourselves.
		minio.PutObjectOptions{ContentType: "application/octet-stream", PartSize: partSize, DisableContentSha256: true})
	if err != nil {
		return 0, "", err
	}
	return info.Size, hex.EncodeToString(h.Sum(nil)), nil
}

func (b *Blobs) Open(ctx context.Context, id domain.ID) (io.ReadCloser, error) {
	k, err := key(ctx, id)
	if err != nil {
		return nil, err
	}
	if _, err := b.client.StatObject(ctx, b.bucket, k, minio.StatObjectOptions{}); err != nil {
		if minio.ToErrorResponse(err).Code == minio.NoSuchKey {
			return nil, domain.ErrAttachmentNotFound
		}
		return nil, err
	}
	return b.client.GetObject(ctx, b.bucket, k, minio.GetObjectOptions{})
}

// Delete is idempotent: removing a missing object succeeds.
func (b *Blobs) Delete(ctx context.Context, id domain.ID) error {
	k, err := key(ctx, id)
	if err != nil {
		return err
	}
	return b.client.RemoveObject(ctx, b.bucket, k, minio.RemoveObjectOptions{})
}
