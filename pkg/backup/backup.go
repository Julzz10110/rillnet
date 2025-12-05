package backup

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// BackupData represents backup data structure
type BackupData struct {
	Version     string                 `json:"version"`
	Timestamp   time.Time              `json:"timestamp"`
	Streams     map[string]interface{} `json:"streams,omitempty"`
	Peers       map[string]interface{} `json:"peers,omitempty"`
	Mesh        map[string]interface{} `json:"mesh,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// Storage defines interface for backup storage
type Storage interface {
	Save(ctx context.Context, name string, data io.Reader) error
	Load(ctx context.Context, name string) (io.ReadCloser, error)
	List(ctx context.Context, prefix string) ([]string, error)
	Delete(ctx context.Context, name string) error
}

// BackupService handles backup operations
type BackupService struct {
	storage Storage
	version string
}

// NewBackupService creates a new backup service
func NewBackupService(storage Storage, version string) *BackupService {
	return &BackupService{
		storage: storage,
		version: version,
	}
}

// CreateBackup creates a backup of the provided data
func (bs *BackupService) CreateBackup(ctx context.Context, data *BackupData) (string, error) {
	data.Version = bs.version
	data.Timestamp = time.Now()

	// Serialize to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal backup data: %w", err)
	}

	// Generate backup name with timestamp
	backupName := fmt.Sprintf("backup-%s.json", data.Timestamp.Format("20060102-150405"))

	// Save to storage
	reader := &byteReader{data: jsonData}
	if err := bs.storage.Save(ctx, backupName, reader); err != nil {
		return "", fmt.Errorf("failed to save backup: %w", err)
	}

	return backupName, nil
}

// RestoreBackup restores data from a backup
func (bs *BackupService) RestoreBackup(ctx context.Context, name string) (*BackupData, error) {
	// Load from storage
	reader, err := bs.storage.Load(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to load backup: %w", err)
	}
	defer reader.Close()

	// Read data
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read backup data: %w", err)
	}

	// Deserialize
	var backupData BackupData
	if err := json.Unmarshal(data, &backupData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal backup data: %w", err)
	}

	return &backupData, nil
}

// ListBackups lists all available backups
func (bs *BackupService) ListBackups(ctx context.Context) ([]string, error) {
	return bs.storage.List(ctx, "backup-")
}

// DeleteBackup deletes a backup
func (bs *BackupService) DeleteBackup(ctx context.Context, name string) error {
	return bs.storage.Delete(ctx, name)
}

// byteReader implements io.Reader for byte slice
type byteReader struct {
	data []byte
	pos  int
}

func (br *byteReader) Read(p []byte) (n int, err error) {
	if br.pos >= len(br.data) {
		return 0, io.EOF
	}
	n = copy(p, br.data[br.pos:])
	br.pos += n
	return n, nil
}

