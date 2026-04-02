package storage

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type S3Client interface {
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

type S3Store struct {
	client S3Client
	bucket string
}

func NewS3Store(client S3Client, bucket string) *S3Store {
	return &S3Store{client: client, bucket: bucket}
}

func (s *S3Store) Put(ctx context.Context, key string, data []byte, opts PutOptions) error {
	input := &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),
	}

	if opts.ContentType != "" {
		input.ContentType = aws.String(opts.ContentType)
	}

	if opts.ObjectLock {
		input.ObjectLockMode = types.ObjectLockModeCompliance
		if opts.RetentionDays > 0 {
			retainUntil := time.Now().UTC().AddDate(0, 0, opts.RetentionDays)
			input.ObjectLockRetainUntilDate = &retainUntil
		}
	}

	_, err := s.client.PutObject(ctx, input)
	if err != nil {
		return fmt.Errorf("putting object %s to bucket %s: %w", key, s.bucket, err)
	}
	return nil
}
