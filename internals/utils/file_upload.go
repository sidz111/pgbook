package utils

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

// FileStorageConfig holds file storage configuration
type FileStorageConfig struct {
	StorageType string // "local" or "s3"
	LocalPath   string // For local storage: /uploads
	MaxFileSize int64  // In bytes (default: 5MB)
}

// FileUploadService handles file operations
type FileUploadService struct {
	config FileStorageConfig
}

// NewFileUploadService creates new file upload service
func NewFileUploadService(config FileStorageConfig) *FileUploadService {
	if config.MaxFileSize == 0 {
		config.MaxFileSize = 5 * 1024 * 1024 // 5MB default
	}
	return &FileUploadService{config: config}
}

// UploadTenantDocument uploads tenant ID proof or photo
func (f *FileUploadService) UploadTenantDocument(file *multipart.FileHeader, docType string) (string, error) {
	// Validate file size
	if file.Size > f.config.MaxFileSize {
		return "", errors.New("file size exceeds maximum allowed")
	}

	// Validate file type
	allowedTypes := map[string]bool{
		"image/jpeg":      true,
		"image/png":       true,
		"image/jpg":       true,
		"application/pdf": true,
	}

	if !allowedTypes[file.Header.Get("Content-Type")] {
		return "", errors.New("invalid file type - only JPEG, PNG, PDF allowed")
	}

	// Generate secure filename using UUID
	fileExt := filepath.Ext(file.Filename)
	secureFilename := fmt.Sprintf("%s_%s%s", docType, uuid.New().String(), fileExt)

	switch f.config.StorageType {
	case "local":
		return f.uploadLocal(file, secureFilename)
	case "s3":
		return f.uploadS3(file, secureFilename)
	default:
		return "", errors.New("unsupported storage type")
	}
}

// uploadLocal saves file to local storage
func (f *FileUploadService) uploadLocal(file *multipart.FileHeader, filename string) (string, error) {
	// Create directory if not exists
	uploadDir := filepath.Join(f.config.LocalPath, "tenant-documents")
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create upload directory: %w", err)
	}

	// Open uploaded file
	src, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer src.Close()

	// Create destination file
	filepath := filepath.Join(uploadDir, filename)
	dst, err := os.Create(filepath)
	if err != nil {
		return "", fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dst.Close()

	// Copy file
	if _, err := io.Copy(dst, src); err != nil {
		return "", fmt.Errorf("failed to copy file: %w", err)
	}

	// Return secure path (not exposing full filesystem path)
	return fmt.Sprintf("/api/v1/documents/%s", filename), nil
}

// uploadS3 uploads file to S3 bucket (placeholder for AWS SDK integration)
func (f *FileUploadService) uploadS3(file *multipart.FileHeader, filename string) (string, error) {
	// TODO: Implement AWS S3 upload using aws-sdk-go
	// For now, return error
	return "", errors.New("S3 upload not yet implemented - use local storage")
}

// GetDocumentPath constructs secure document access path
func (f *FileUploadService) GetDocumentPath(filename string) string {
	return fmt.Sprintf("/api/v1/documents/%s", filename)
}

// ServeSecureDocument serves document with access control
// Should be called from handler after permission check
func (f *FileUploadService) ServeSecureDocument(filename string) (string, error) {
	if f.config.StorageType == "local" {
		filepath := filepath.Join(f.config.LocalPath, "tenant-documents", filename)

		// Validate file exists
		if _, err := os.Stat(filepath); os.IsNotExist(err) {
			return "", errors.New("document not found")
		}

		return filepath, nil
	}

	return "", errors.New("S3 retrieval not yet implemented")
}

// DeleteDocument removes uploaded document
func (f *FileUploadService) DeleteDocument(filename string) error {
	if f.config.StorageType == "local" {
		filepath := filepath.Join(f.config.LocalPath, "tenant-documents", filename)
		return os.Remove(filepath)
	}

	return errors.New("S3 deletion not yet implemented")
}
