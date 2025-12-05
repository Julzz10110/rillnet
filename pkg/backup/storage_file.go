package backup

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// FileStorage implements Storage interface using local filesystem
type FileStorage struct {
	basePath string
}

// NewFileStorage creates a new file storage
func NewFileStorage(basePath string) (*FileStorage, error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}

	return &FileStorage{
		basePath: basePath,
	}, nil
}

// Save saves data to a file
func (fs *FileStorage) Save(ctx context.Context, name string, data io.Reader) error {
	filePath := filepath.Join(fs.basePath, name)

	// Create file
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create backup file: %w", err)
	}
	defer file.Close()

	// Copy data
	if _, err := io.Copy(file, data); err != nil {
		return fmt.Errorf("failed to write backup data: %w", err)
	}

	return nil
}

// Load loads data from a file
func (fs *FileStorage) Load(ctx context.Context, name string) (io.ReadCloser, error) {
	filePath := filepath.Join(fs.basePath, name)

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open backup file: %w", err)
	}

	return file, nil
}

// List lists all files with the given prefix
func (fs *FileStorage) List(ctx context.Context, prefix string) ([]string, error) {
	entries, err := os.ReadDir(fs.basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read backup directory: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), prefix) {
			files = append(files, entry.Name())
		}
	}

	return files, nil
}

// Delete deletes a file
func (fs *FileStorage) Delete(ctx context.Context, name string) error {
	filePath := filepath.Join(fs.basePath, name)
	return os.Remove(filePath)
}

