package s3

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/regrada-ai/regrada-be/internal/storage"
)

// Ensure Service implements storage.FileStorageService interface at compile time
var _ storage.FileStorageService = (*Service)(nil)

type Service struct {
	client           *s3.Client
	presignClient    *s3.PresignClient
	bucket           string
	cloudFrontDomain string
}

// NewService creates a new S3 service
func NewService(region, bucket, cloudFrontDomain string) (*Service, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := s3.NewFromConfig(cfg)
	presignClient := s3.NewPresignClient(client)

	return &Service{
		client:           client,
		presignClient:    presignClient,
		bucket:           bucket,
		cloudFrontDomain: cloudFrontDomain,
	}, nil
}

// UploadFile uploads a file to S3 and returns the S3 key
func (s *Service) UploadFile(ctx context.Context, key string, file multipart.File, contentType string) error {
	// Read file content
	buffer := bytes.NewBuffer(nil)
	if _, err := io.Copy(buffer, file); err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Upload to S3
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(buffer.Bytes()),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("failed to upload to S3: %w", err)
	}

	return nil
}

// DeleteFile deletes a file from S3
func (s *Service) DeleteFile(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete from S3: %w", err)
	}

	return nil
}

// GetPresignedURL generates a presigned URL for accessing an S3 object
func (s *Service) GetPresignedURL(ctx context.Context, key string, expiresIn time.Duration) (string, error) {
	if key == "" {
		return "", nil
	}

	req, err := s.presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expiresIn))

	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return req.URL, nil
}

// GetCloudFrontURL returns the CloudFront URL for a given S3 key
func (s *Service) GetCloudFrontURL(key string) string {
	if key == "" {
		return ""
	}
	// Remove leading slash if present
	key = strings.TrimPrefix(key, "/")
	return fmt.Sprintf("https://%s/%s", s.cloudFrontDomain, key)
}

// ValidateImageFile validates that the uploaded file is an image
func ValidateImageFile(header *multipart.FileHeader) error {
	// Check file size (max 5MB)
	const maxSize = 5 * 1024 * 1024
	if header.Size > maxSize {
		return fmt.Errorf("file size exceeds 5MB limit")
	}

	// Check content type
	contentType := header.Header.Get("Content-Type")
	allowedTypes := map[string]bool{
		"image/jpeg": true,
		"image/jpg":  true,
		"image/png":  true,
		"image/gif":  true,
		"image/webp": true,
	}

	if !allowedTypes[contentType] {
		return fmt.Errorf("invalid file type: %s. Allowed types: jpeg, jpg, png, gif, webp", contentType)
	}

	return nil
}

// GetFileExtension returns the file extension from the filename
func GetFileExtension(filename string) string {
	ext := filepath.Ext(filename)
	return strings.ToLower(ext)
}
