package storage

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockS3Client struct {
	lastInput *s3.PutObjectInput
	err       error
}

func (m *mockS3Client) PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	m.lastInput = params
	return &s3.PutObjectOutput{}, m.err
}

func TestS3Store_Put(t *testing.T) {
	mock := &mockS3Client{}
	store := NewS3Store(mock, "test-bucket")

	err := store.Put(context.Background(), "test/key.cdx.json", []byte("data"), PutOptions{
		ContentType: "application/json",
	})
	require.NoError(t, err)
	assert.Equal(t, "test-bucket", *mock.lastInput.Bucket)
	assert.Equal(t, "test/key.cdx.json", *mock.lastInput.Key)
	assert.Equal(t, "application/json", *mock.lastInput.ContentType)
}

func TestS3Store_PutWithObjectLock(t *testing.T) {
	mock := &mockS3Client{}
	store := NewS3Store(mock, "test-bucket")

	err := store.Put(context.Background(), "key.cdx.json", []byte("data"), PutOptions{
		ObjectLock:    true,
		RetentionDays: 365,
		ContentType:   "application/json",
	})
	require.NoError(t, err)
	assert.Equal(t, types.ObjectLockModeCompliance, mock.lastInput.ObjectLockMode)
	assert.NotNil(t, mock.lastInput.ObjectLockRetainUntilDate)
}

func TestS3Store_PutError(t *testing.T) {
	mock := &mockS3Client{err: assert.AnError}
	store := NewS3Store(mock, "test-bucket")

	err := store.Put(context.Background(), "key.cdx.json", []byte("data"), PutOptions{})
	assert.Error(t, err)
}
