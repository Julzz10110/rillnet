package backup

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBackupService_CreateBackup(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()
	storage, err := NewFileStorage(tmpDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	service := NewBackupService(storage, "1.0.0")

	data := &BackupData{
		Streams: map[string]interface{}{
			"stream-1": map[string]interface{}{
				"id":   "stream-1",
				"name": "Test Stream",
			},
		},
		Peers: map[string]interface{}{
			"peer-1": map[string]interface{}{
				"id": "peer-1",
			},
		},
	}

	backupName, err := service.CreateBackup(context.Background(), data)
	if err != nil {
		t.Fatalf("failed to create backup: %v", err)
	}

	if backupName == "" {
		t.Error("expected non-empty backup name")
	}

	// Verify file exists
	filePath := filepath.Join(tmpDir, backupName)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Errorf("backup file does not exist: %s", filePath)
	}
}

func TestBackupService_RestoreBackup(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewFileStorage(tmpDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	service := NewBackupService(storage, "1.0.0")

	// Create backup
	data := &BackupData{
		Streams: map[string]interface{}{
			"stream-1": map[string]interface{}{
				"id": "stream-1",
			},
		},
	}

	backupName, err := service.CreateBackup(context.Background(), data)
	if err != nil {
		t.Fatalf("failed to create backup: %v", err)
	}

	// Restore backup
	restored, err := service.RestoreBackup(context.Background(), backupName)
	if err != nil {
		t.Fatalf("failed to restore backup: %v", err)
	}

	if restored.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got '%s'", restored.Version)
	}

	if len(restored.Streams) != 1 {
		t.Errorf("expected 1 stream, got %d", len(restored.Streams))
	}
}

func TestBackupService_ListBackups(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewFileStorage(tmpDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	service := NewBackupService(storage, "1.0.0")

	// Create multiple backups with delays to ensure different timestamps
	for i := 0; i < 3; i++ {
		data := &BackupData{
			Streams: map[string]interface{}{},
		}
		_, err := service.CreateBackup(context.Background(), data)
		if err != nil {
			t.Fatalf("failed to create backup: %v", err)
		}
		if i < 2 { // Don't sleep after last backup
			time.Sleep(1100 * time.Millisecond) // Ensure different timestamps (backup name includes seconds)
		}
	}

	backups, err := service.ListBackups(context.Background())
	if err != nil {
		t.Fatalf("failed to list backups: %v", err)
	}

	if len(backups) < 1 {
		t.Errorf("expected at least 1 backup, got %d", len(backups))
	}
}

func TestBackupService_DeleteBackup(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewFileStorage(tmpDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	service := NewBackupService(storage, "1.0.0")

	// Create backup
	data := &BackupData{
		Streams: map[string]interface{}{},
	}
	backupName, err := service.CreateBackup(context.Background(), data)
	if err != nil {
		t.Fatalf("failed to create backup: %v", err)
	}

	// Delete backup
	err = service.DeleteBackup(context.Background(), backupName)
	if err != nil {
		t.Fatalf("failed to delete backup: %v", err)
	}

	// Verify file is deleted
	filePath := filepath.Join(tmpDir, backupName)
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("backup file should be deleted")
	}
}

func TestFileStorage(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewFileStorage(tmpDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	// Test Save
	data := []byte("test data")
	reader := &byteReader{data: data}
	err = storage.Save(context.Background(), "test.txt", reader)
	if err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	// Test Load
	loaded, err := storage.Load(context.Background(), "test.txt")
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}
	loaded.Close() // Close immediately to allow deletion

	// Test List
	files, err := storage.List(context.Background(), "test")
	if err != nil {
		t.Fatalf("failed to list: %v", err)
	}

	if len(files) != 1 {
		t.Errorf("expected 1 file, got %d", len(files))
	}

	// Test Delete
	err = storage.Delete(context.Background(), "test.txt")
	if err != nil {
		t.Fatalf("failed to delete: %v", err)
	}
}

