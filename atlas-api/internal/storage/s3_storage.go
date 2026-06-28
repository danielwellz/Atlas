package storage

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type S3StorageConfig struct {
	Endpoint      string
	AccessKeyID   string
	SecretAccess  string
	Region        string
	DefaultBucket string
	UseSSL        bool
}

type S3Storage struct {
	client        *minio.Client
	endpointHost  string
	defaultBucket string
}

func NewS3Storage(config S3StorageConfig) (*S3Storage, error) {
	endpoint := strings.TrimSpace(config.Endpoint)
	if endpoint == "" {
		return nil, errors.New("s3 endpoint is required")
	}

	endpointHost := normalizeEndpointHost(endpoint)
	if endpointHost == "" {
		return nil, errors.New("s3 endpoint is invalid")
	}

	client, err := minio.New(endpointHost, &minio.Options{
		Creds: credentials.NewStaticV4(
			strings.TrimSpace(config.AccessKeyID),
			strings.TrimSpace(config.SecretAccess),
			"",
		),
		Secure: config.UseSSL,
		Region: strings.TrimSpace(config.Region),
	})
	if err != nil {
		return nil, err
	}

	return &S3Storage{
		client:        client,
		endpointHost:  endpointHost,
		defaultBucket: strings.TrimSpace(config.DefaultBucket),
	}, nil
}

func (s *S3Storage) NormalizeURI(uri string) (string, error) {
	trimmed := strings.TrimSpace(uri)
	if trimmed == "" {
		return "", ErrEmptyURI
	}

	var (
		bucket string
		key    string
		err    error
	)

	switch {
	case strings.HasPrefix(trimmed, "s3://"):
		bucket, key, err = parseS3URI(trimmed)
	case strings.HasPrefix(trimmed, "http://"), strings.HasPrefix(trimmed, "https://"):
		bucket, key, err = parseHTTPObjectURI(trimmed)
	default:
		bucket = s.defaultBucket
		key, err = normalizeObjectKey(trimmed)
		if bucket == "" && err == nil {
			err = errors.New("s3 default bucket is required for non-s3 URIs")
		}
	}
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("s3://%s/%s", bucket, key), nil
}

func (s *S3Storage) Exists(ctx context.Context, uri string) (bool, error) {
	normalizedURI, err := s.NormalizeURI(uri)
	if err != nil {
		return false, err
	}

	bucket, key, err := parseS3URI(normalizedURI)
	if err != nil {
		return false, err
	}

	_, err = s.client.StatObject(ctx, bucket, key, minio.StatObjectOptions{})
	if err == nil {
		return true, nil
	}

	errorResponse := minio.ToErrorResponse(err)
	if errorResponse.Code == "NoSuchKey" ||
		errorResponse.Code == "NoSuchBucket" ||
		errorResponse.Code == "NotFound" {
		return false, nil
	}

	return false, err
}

func (s *S3Storage) SignedURL(ctx context.Context, uri string, expires time.Duration) (string, error) {
	if expires <= 0 {
		expires = 15 * time.Minute
	}

	normalizedURI, err := s.NormalizeURI(uri)
	if err != nil {
		return "", err
	}

	bucket, key, err := parseS3URI(normalizedURI)
	if err != nil {
		return "", err
	}

	presignedURL, err := s.client.PresignedGetObject(ctx, bucket, key, expires, url.Values{})
	if err != nil {
		return "", err
	}

	return presignedURL.String(), nil
}

func normalizeEndpointHost(endpoint string) string {
	trimmed := strings.TrimSpace(endpoint)
	if trimmed == "" {
		return ""
	}

	if !strings.Contains(trimmed, "://") {
		return strings.TrimRight(trimmed, "/")
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(parsed.Host)
}

func parseS3URI(uri string) (bucket string, key string, err error) {
	parsed, err := url.Parse(uri)
	if err != nil {
		return "", "", err
	}
	if !strings.EqualFold(parsed.Scheme, "s3") {
		return "", "", errors.New("uri must use s3 scheme")
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return "", "", errors.New("s3 bucket is required")
	}

	normalizedKey, err := normalizeObjectKey(parsed.Path)
	if err != nil {
		return "", "", err
	}
	return parsed.Host, normalizedKey, nil
}

func parseHTTPObjectURI(uri string) (bucket string, key string, err error) {
	parsed, err := url.Parse(uri)
	if err != nil {
		return "", "", err
	}

	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(segments) < 2 {
		return "", "", errors.New("http object uri must include /{bucket}/{key}")
	}

	objectKey, err := normalizeObjectKey(strings.Join(segments[1:], "/"))
	if err != nil {
		return "", "", err
	}

	return segments[0], objectKey, nil
}

func normalizeObjectKey(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", ErrEmptyURI
	}

	cleaned := strings.TrimPrefix(path.Clean("/"+trimmed), "/")
	if cleaned == "" || cleaned == "." {
		return "", ErrEmptyURI
	}
	if strings.HasPrefix(cleaned, "../") || cleaned == ".." {
		return "", errors.New("s3 object key cannot escape root")
	}

	return cleaned, nil
}
