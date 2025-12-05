package backup

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"rillnet/internal/core/domain"
	"rillnet/internal/core/ports"
	"rillnet/pkg/backup"
	"go.uber.org/zap"
)

// RestoreService handles restore operations
type RestoreService struct {
	backupService *backup.BackupService
	streamRepo    ports.StreamRepository
	peerRepo      ports.PeerRepository
	meshRepo      ports.MeshRepository
	logger        *zap.SugaredLogger
}

// NewRestoreService creates a new restore service
func NewRestoreService(
	backupService *backup.BackupService,
	streamRepo ports.StreamRepository,
	peerRepo ports.PeerRepository,
	meshRepo ports.MeshRepository,
	logger *zap.SugaredLogger,
) *RestoreService {
	return &RestoreService{
		backupService: backupService,
		streamRepo:    streamRepo,
		peerRepo:      peerRepo,
		meshRepo:      meshRepo,
		logger:        logger,
	}
}

// RestoreFromBackup restores data from a specific backup
func (rs *RestoreService) RestoreFromBackup(ctx context.Context, backupName string, options RestoreOptions) error {
	rs.logger.Infow("starting restore", "backup_name", backupName, "options", options)

	// Load backup
	backupData, err := rs.backupService.RestoreBackup(ctx, backupName)
	if err != nil {
		return fmt.Errorf("failed to load backup: %w", err)
	}

	// Validate backup version
	if backupData.Version == "" {
		return fmt.Errorf("invalid backup: missing version")
	}

	// Restore streams
	if err := rs.restoreStreams(ctx, backupData.Streams, options); err != nil {
		return fmt.Errorf("failed to restore streams: %w", err)
	}

	// Restore peers
	if err := rs.restorePeers(ctx, backupData.Peers, options); err != nil {
		return fmt.Errorf("failed to restore peers: %w", err)
	}

	// Restore mesh connections
	if err := rs.restoreMesh(ctx, backupData.Mesh, options); err != nil {
		return fmt.Errorf("failed to restore mesh: %w", err)
	}

	rs.logger.Infow("restore completed successfully", "backup_name", backupName)
	return nil
}

// RestoreOptions contains restore options
type RestoreOptions struct {
	OverwriteExisting bool
	RestoreStreams   bool
	RestorePeers      bool
	RestoreMesh       bool
	PointInTime       *time.Time // For point-in-time recovery
}

// DefaultRestoreOptions returns default restore options
func DefaultRestoreOptions() RestoreOptions {
	return RestoreOptions{
		OverwriteExisting: false,
		RestoreStreams:    true,
		RestorePeers:      true,
		RestoreMesh:       true,
	}
}

// restoreStreams restores streams from backup
func (rs *RestoreService) restoreStreams(ctx context.Context, streams map[string]interface{}, options RestoreOptions) error {
	if !options.RestoreStreams {
		return nil
	}

	for streamIDStr, streamData := range streams {
		streamID := domain.StreamID(streamIDStr)

		// Check if stream exists
		existing, err := rs.streamRepo.GetByID(ctx, streamID)
		if err == nil && existing != nil {
			if !options.OverwriteExisting {
				rs.logger.Debugw("skipping existing stream", "stream_id", streamID)
				continue
			}
		}

		// Convert to domain.Stream
		streamJSON, err := json.Marshal(streamData)
		if err != nil {
			return fmt.Errorf("failed to marshal stream: %w", err)
		}

		var stream domain.Stream
		if err := json.Unmarshal(streamJSON, &stream); err != nil {
			return fmt.Errorf("failed to unmarshal stream: %w", err)
		}

		// Create or update stream
		if existing == nil {
			if err := rs.streamRepo.Create(ctx, &stream); err != nil {
				return fmt.Errorf("failed to create stream: %w", err)
			}
		} else {
			if err := rs.streamRepo.Update(ctx, &stream); err != nil {
				return fmt.Errorf("failed to update stream: %w", err)
			}
		}

		rs.logger.Debugw("restored stream", "stream_id", streamID)
	}

	return nil
}

// restorePeers restores peers from backup
func (rs *RestoreService) restorePeers(ctx context.Context, peers map[string]interface{}, options RestoreOptions) error {
	if !options.RestorePeers {
		return nil
	}

	for peerIDStr, peerData := range peers {
		peerID := domain.PeerID(peerIDStr)

		// Check if peer exists
		existing, err := rs.peerRepo.GetByID(ctx, peerID)
		if err == nil && existing != nil {
			if !options.OverwriteExisting {
				rs.logger.Debugw("skipping existing peer", "peer_id", peerID)
				continue
			}
		}

		// Convert to domain.Peer
		peerJSON, err := json.Marshal(peerData)
		if err != nil {
			return fmt.Errorf("failed to marshal peer: %w", err)
		}

		var peer domain.Peer
		if err := json.Unmarshal(peerJSON, &peer); err != nil {
			return fmt.Errorf("failed to unmarshal peer: %w", err)
		}

		// Create or update peer
		if existing == nil {
			if err := rs.peerRepo.Add(ctx, &peer); err != nil {
				return fmt.Errorf("failed to add peer: %w", err)
			}
		} else {
			// Update peer (if repository supports update)
			if err := rs.peerRepo.Add(ctx, &peer); err != nil {
				return fmt.Errorf("failed to update peer: %w", err)
			}
		}

		rs.logger.Debugw("restored peer", "peer_id", peerID)
	}

	return nil
}

// restoreMesh restores mesh connections from backup
func (rs *RestoreService) restoreMesh(ctx context.Context, mesh map[string]interface{}, options RestoreOptions) error {
	if !options.RestoreMesh {
		return nil
	}

	// Mesh restoration would depend on the mesh repository implementation
	// For now, we'll just log that mesh restoration is not fully implemented
	rs.logger.Info("mesh restoration is not fully implemented yet")

	return nil
}

// FindBackupByTime finds the closest backup to a given time (for point-in-time recovery)
func (rs *RestoreService) FindBackupByTime(ctx context.Context, targetTime time.Time) (string, error) {
	backups, err := rs.backupService.ListBackups(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list backups: %w", err)
	}

	var closestBackup string
	var closestTime time.Time
	var found bool

	for _, backupName := range backups {
		// Parse timestamp from backup name
		if len(backupName) < 20 {
			continue
		}

		timestampStr := backupName[7:22]
		timestamp, err := time.Parse("20060102-150405", timestampStr)
		if err != nil {
			continue
		}

		// Find backup closest to target time (but not after)
		if timestamp.Before(targetTime) || timestamp.Equal(targetTime) {
			if !found || timestamp.After(closestTime) {
				closestBackup = backupName
				closestTime = timestamp
				found = true
			}
		}
	}

	if !found {
		return "", fmt.Errorf("no backup found before or at target time: %v", targetTime)
	}

	return closestBackup, nil
}

