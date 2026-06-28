package storage

import (
	"context"
	"time"
)

// Storage provides a backend-agnostic contract for media URI normalization and existence checks.
type Storage interface {
	NormalizeURI(uri string) (string, error)
	Exists(ctx context.Context, uri string) (bool, error)
	SignedURL(ctx context.Context, uri string, expires time.Duration) (string, error)
}
