package storage

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/stretchr/testify/require"
)

func TestS3StorageExistsAgainstMinIO(t *testing.T) {
	t.Parallel()

	endpoint := "127.0.0.1:9000"
	accessKey := "atlasminio"
	secretKey := "atlasminio"
	bucket := fmt.Sprintf("atlas-biomech-test-%d", time.Now().UnixNano())
	objectKey := "biomechanics/back-squat/clip_v1.fbx"

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, listErr := client.ListBuckets(ctx); listErr != nil {
		t.Skipf("skipping minio integration test: %v", listErr)
	}

	err = client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{})
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_ = client.RemoveObject(cleanupCtx, bucket, objectKey, minio.RemoveObjectOptions{})
		_ = client.RemoveBucket(cleanupCtx, bucket)
	})

	_, err = client.PutObject(
		ctx,
		bucket,
		objectKey,
		bytes.NewReader([]byte("fixture-animation-content")),
		int64(len("fixture-animation-content")),
		minio.PutObjectOptions{},
	)
	require.NoError(t, err)

	storage, err := NewS3Storage(S3StorageConfig{
		Endpoint:      endpoint,
		AccessKeyID:   accessKey,
		SecretAccess:  secretKey,
		DefaultBucket: bucket,
		UseSSL:        false,
	})
	require.NoError(t, err)

	normalizedURI, err := storage.NormalizeURI(objectKey)
	require.NoError(t, err)
	require.Equal(t, "s3://"+bucket+"/"+objectKey, normalizedURI)

	exists, err := storage.Exists(ctx, normalizedURI)
	require.NoError(t, err)
	require.True(t, exists)

	missing, err := storage.Exists(ctx, "missing/clip.fbx")
	require.NoError(t, err)
	require.False(t, missing)
}
