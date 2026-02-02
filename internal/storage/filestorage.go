package storage

import (
	"context"
	"mime/multipart"
	"time"
)

// FileStorageService defines the interface for file storage operations
type FileStorageService interface {
	UploadFile(ctx context.Context, key string, file multipart.File, contentType string) error
	DeleteFile(ctx context.Context, key string) error
	GetPresignedURL(ctx context.Context, key string, expiresIn time.Duration) (string, error)
	GetCloudFrontURL(key string) string
}
