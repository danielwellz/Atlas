package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var ErrEmptyURI = errors.New("uri is required")

type LocalFileStorage struct {
	baseDir string
}

func NewLocalFileStorage(baseDir string) *LocalFileStorage {
	return &LocalFileStorage{baseDir: strings.TrimSpace(baseDir)}
}

func (s *LocalFileStorage) NormalizeURI(uri string) (string, error) {
	trimmed := strings.TrimSpace(uri)
	if trimmed == "" {
		return "", ErrEmptyURI
	}

	cleaned := filepath.Clean(trimmed)
	if filepath.IsAbs(cleaned) {
		return cleaned, nil
	}

	baseDir := s.baseDir
	if baseDir == "" {
		workingDir, err := os.Getwd()
		if err != nil {
			return "", err
		}
		baseDir = workingDir
	}

	absolutePath, err := filepath.Abs(filepath.Join(baseDir, cleaned))
	if err != nil {
		return "", err
	}
	return absolutePath, nil
}

func (s *LocalFileStorage) Exists(_ context.Context, uri string) (bool, error) {
	normalized, err := s.NormalizeURI(uri)
	if err != nil {
		return false, err
	}

	_, err = os.Stat(normalized)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func (s *LocalFileStorage) SignedURL(_ context.Context, uri string, _ time.Duration) (string, error) {
	return s.NormalizeURI(uri)
}
