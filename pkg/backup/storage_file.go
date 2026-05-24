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
	if err := os.MkdirAll(basePath, 0750); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}

	return &FileStorage{
		basePath: basePath,
	}, nil
}

func (fs *FileStorage) validateName(name string) error {
	if name == "" || strings.Contains(name, "..") || filepath.IsAbs(name) {
		return fmt.Errorf("invalid backup name: %q", name)
	}
	if strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("invalid backup name: %q", name)
	}
	return nil
}

func (fs *FileStorage) openRoot() (*os.Root, error) {
	return os.OpenRoot(fs.basePath)
}

// Save saves data to a file
func (fs *FileStorage) Save(ctx context.Context, name string, data io.Reader) error {
	if err := fs.validateName(name); err != nil {
		return err
	}

	root, err := fs.openRoot()
	if err != nil {
		return fmt.Errorf("failed to open backup root: %w", err)
	}
	defer root.Close()

	file, err := root.Create(name)
	if err != nil {
		return fmt.Errorf("failed to create backup file: %w", err)
	}
	defer func() { _ = file.Close() }()

	if _, err := io.Copy(file, data); err != nil {
		return fmt.Errorf("failed to write backup data: %w", err)
	}

	return nil
}

// Load loads data from a file
func (fs *FileStorage) Load(ctx context.Context, name string) (io.ReadCloser, error) {
	if err := fs.validateName(name); err != nil {
		return nil, err
	}

	root, err := fs.openRoot()
	if err != nil {
		return nil, fmt.Errorf("failed to open backup root: %w", err)
	}

	file, err := root.Open(name)
	if err != nil {
		_ = root.Close()
		return nil, fmt.Errorf("failed to open backup file: %w", err)
	}

	return &rootFile{Root: root, File: file}, nil
}

// rootFile closes the os.Root when the opened file is closed.
type rootFile struct {
	Root *os.Root
	File *os.File
}

func (f *rootFile) Read(p []byte) (int, error) {
	return f.File.Read(p)
}

func (f *rootFile) Close() error {
	err := f.File.Close()
	if closeErr := f.Root.Close(); err == nil {
		err = closeErr
	}
	return err
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
	if err := fs.validateName(name); err != nil {
		return err
	}

	root, err := fs.openRoot()
	if err != nil {
		return fmt.Errorf("failed to open backup root: %w", err)
	}
	defer root.Close()

	return root.Remove(name)
}
