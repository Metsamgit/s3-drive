// Package awsclient wraps the AWS SDK with the bare set of operations the
// app needs. Every call takes a context so request lifetimes can be bound
// to HTTP request timeouts.
package awsclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscreds "github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"

	"github.com/Metsamgit/s3-drive/internal/auth"
)

type Client struct {
	s3       *s3.Client
	uploader *manager.Uploader
}

// New builds an S3 client from session credentials. The HTTP client is the
// SDK default; we rely on its built-in timeouts and retries.
func New(c auth.Creds) *Client {
	provider := awscreds.NewStaticCredentialsProvider(
		c.AccessKeyID, c.SecretAccessKey, c.SessionToken,
	)
	cli := s3.NewFromConfig(aws.Config{
		Region:      c.Region,
		Credentials: provider,
	})
	return &Client{
		s3: cli,
		uploader: manager.NewUploader(cli, func(u *manager.Uploader) {
			u.PartSize = 5 * 1024 * 1024
			u.Concurrency = 4
		}),
	}
}

type Bucket struct {
	Name string
}

func (c *Client) ListBuckets(ctx context.Context) ([]Bucket, error) {
	out, err := c.s3.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, wrap(err)
	}
	res := make([]Bucket, 0, len(out.Buckets))
	for _, b := range out.Buckets {
		if b.Name != nil {
			res = append(res, Bucket{Name: *b.Name})
		}
	}
	return res, nil
}

type Object struct {
	Key          string
	Size         int64
	LastModified time.Time
}

type ListResult struct {
	Folders []string // CommonPrefixes
	Files   []Object
}

// ListPrefix lists immediate children of `prefix` in bucket. Folders are
// returned as their full S3 prefix (ending in "/").
func (c *Client) ListPrefix(ctx context.Context, bucket, prefix string) (*ListResult, error) {
	out, err := c.s3.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:    aws.String(bucket),
		Prefix:    aws.String(prefix),
		Delimiter: aws.String("/"),
		MaxKeys:   aws.Int32(1000),
	})
	if err != nil {
		return nil, wrap(err)
	}
	r := &ListResult{}
	for _, cp := range out.CommonPrefixes {
		if cp.Prefix != nil {
			r.Folders = append(r.Folders, *cp.Prefix)
		}
	}
	for _, o := range out.Contents {
		if o.Key == nil || *o.Key == prefix {
			continue
		}
		obj := Object{Key: *o.Key}
		if o.Size != nil {
			obj.Size = *o.Size
		}
		if o.LastModified != nil {
			obj.LastModified = *o.LastModified
		}
		r.Files = append(r.Files, obj)
	}
	return r, nil
}

// Upload streams a reader to S3. The manager auto-switches to multipart
// for parts >= 5 MB.
func (c *Client) Upload(ctx context.Context, bucket, key, contentType string, body io.Reader) error {
	_, err := c.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	})
	return wrap(err)
}

// DownloadStream returns the object body. Callers must close it.
//
// We chose to stream-proxy rather than hand out a presigned URL because
// the latter would expose the bucket hostname (and indirectly the AWS
// account) to the user, and we don't want to.
func (c *Client) DownloadStream(ctx context.Context, bucket, key string) (io.ReadCloser, *DownloadMeta, error) {
	out, err := c.s3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, nil, wrap(err)
	}
	m := &DownloadMeta{}
	if out.ContentType != nil {
		m.ContentType = *out.ContentType
	}
	if out.ContentLength != nil {
		m.ContentLength = *out.ContentLength
	}
	return out.Body, m, nil
}

type DownloadMeta struct {
	ContentType   string
	ContentLength int64
}

func (c *Client) DeleteObject(ctx context.Context, bucket, key string) error {
	_, err := c.s3.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	return wrap(err)
}

// CreateEmptyFolder creates a zero-byte object whose key ends with "/".
// S3 has no real folders; this is the convention every console uses.
func (c *Client) CreateEmptyFolder(ctx context.Context, bucket, key string) error {
	_, err := c.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	return wrap(err)
}

// HeadBucket returns nil if the credentials can access the bucket.
// Useful at login to fail fast on a wrong bucket name or missing perms.
func (c *Client) HeadBucket(ctx context.Context, bucket string) error {
	_, err := c.s3.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(bucket)})
	return wrap(err)
}

// Err is the typed error surface for handlers. We map AWS-internal types
// to a small set so the handler layer doesn't import the SDK directly.
type Err struct {
	Code    string
	Message string
}

func (e *Err) Error() string { return e.Code + ": " + e.Message }

func wrap(err error) error {
	if err == nil {
		return nil
	}
	var ae *types.NoSuchBucket
	if errors.As(err, &ae) {
		return &Err{Code: "NoSuchBucket", Message: "bucket introuvable"}
	}
	var ne *types.NoSuchKey
	if errors.As(err, &ne) {
		return &Err{Code: "NoSuchKey", Message: "objet introuvable"}
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "AccessDenied":
			return &Err{Code: "AccessDenied", Message: "accès refusé par AWS"}
		case "InvalidAccessKeyId", "SignatureDoesNotMatch":
			return &Err{Code: "BadCredentials", Message: "identifiants AWS invalides"}
		case "NotFound", "404":
			return &Err{Code: "NotFound", Message: "ressource introuvable"}
		}
		return &Err{Code: apiErr.ErrorCode(), Message: apiErr.ErrorMessage()}
	}
	return &Err{Code: "Unknown", Message: fmt.Sprintf("erreur S3: %s", err.Error())}
}
