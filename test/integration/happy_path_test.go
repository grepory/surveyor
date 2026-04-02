//go:build integration

package integration

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"surveyor/agent/cdx"
	"surveyor/agent/signing"
	"surveyor/agent/storage"
	pb "surveyor/sdk/go/gen/probev1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func localstackS3Client(t *testing.T) *s3.Client {
	t.Helper()
	cfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion("us-east-1"),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
	)
	require.NoError(t, err)

	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String("http://localhost:4566")
		o.UsePathStyle = true
	})
}

func TestHappyPath_SignAndStore(t *testing.T) {
	ctx := context.Background()
	client := localstackS3Client(t)
	bucket := "test-evidence-" + time.Now().Format("20060102150405")

	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	})
	require.NoError(t, err)

	store := storage.NewS3Store(client, bucket)

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	keyPath := filepath.Join(t.TempDir(), "test.key")
	der, _ := x509.MarshalECPrivateKey(key)
	f, _ := os.Create(keyPath)
	pem.Encode(f, &pem.Block{Type: "EC PRIVATE KEY", Bytes: der})
	f.Close()

	signer, err := signing.NewBYOKSigner(keyPath)
	require.NoError(t, err)

	env := &pb.Envelope{
		Probe: &pb.ProbeMetadata{
			Id:           "test-probe",
			Version:      "0.1.0",
			EvidenceType: "test-evidence",
		},
		Host:              "test-host",
		IpAddress:         "10.0.0.1",
		Platform:          "linux",
		CollectedAt:       timestamppb.New(time.Now()),
		ContentType:       "application/json",
		Payload:           []byte(`{"test": true}`),
		CollectionTrigger: "manual",
		Status:            pb.CollectStatus_COLLECT_STATUS_SUCCESS,
	}

	declaration, err := cdx.Serialize(env, []string{"SC-7"})
	require.NoError(t, err)

	result, err := signer.Sign(ctx, declaration)
	require.NoError(t, err)
	assert.NotEmpty(t, result.ContentHash)

	s3Key := storage.GenerateKey("test-probe", "test-host", time.Now(), result.ContentHash)
	err = store.Put(ctx, s3Key, declaration, storage.PutOptions{
		ContentType: "application/json",
	})
	require.NoError(t, err)

	out, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(s3Key),
	})
	require.NoError(t, err)
	defer out.Body.Close()
	assert.Equal(t, "application/json", *out.ContentType)
}
