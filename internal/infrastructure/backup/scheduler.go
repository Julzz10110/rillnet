package backup

import (
	"context"
	"fmt"
	"time"

	"rillnet/internal/core/domain"
	"rillnet/internal/core/ports"
	"rillnet/pkg/backup"
	"go.uber.org/zap"
)

// Scheduler manages automatic backups
type Scheduler struct {
	backupService *backup.BackupService
	streamRepo    ports.StreamRepository
	peerRepo      ports.PeerRepository
	meshRepo      ports.MeshRepository
	interval      time.Duration
	retentionDays int
	logger        *zap.SugaredLogger
	stopChan      chan struct{}
}

// Config contains scheduler configuration
type Config struct {
	Interval      time.Duration
	RetentionDays int
}

// NewScheduler creates a new backup scheduler
func NewScheduler(
	backupService *backup.BackupService,
	streamRepo ports.StreamRepository,
	peerRepo ports.PeerRepository,
	meshRepo ports.MeshRepository,
	cfg Config,
	logger *zap.SugaredLogger,
) *Scheduler {
	return &Scheduler{
		backupService: backupService,
		streamRepo:    streamRepo,
		peerRepo:      peerRepo,
		meshRepo:      meshRepo,
		interval:      cfg.Interval,
		retentionDays: cfg.RetentionDays,
		logger:        logger,
		stopChan:      make(chan struct{}),
	}
}

// Start starts the backup scheduler
func (s *Scheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// Run initial backup
	s.runBackup(ctx)

	for {
		select {
		case <-ticker.C:
			s.runBackup(ctx)
		case <-s.stopChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

// Stop stops the backup scheduler
func (s *Scheduler) Stop() {
	close(s.stopChan)
}

// runBackup performs a backup
func (s *Scheduler) runBackup(ctx context.Context) {
	s.logger.Info("starting scheduled backup")

	// Collect data
	backupData, err := s.collectData(ctx)
	if err != nil {
		s.logger.Errorw("failed to collect backup data", "error", err)
		return
	}

	// Create backup
	backupName, err := s.backupService.CreateBackup(ctx, backupData)
	if err != nil {
		s.logger.Errorw("failed to create backup", "error", err)
		return
	}

	s.logger.Infow("backup created successfully", "backup_name", backupName)

	// Cleanup old backups
	if err := s.cleanupOldBackups(ctx); err != nil {
		s.logger.Warnw("failed to cleanup old backups", "error", err)
	}
}

// collectData collects data from repositories
func (s *Scheduler) collectData(ctx context.Context) (*backup.BackupData, error) {
	data := &backup.BackupData{
		Streams:  make(map[string]interface{}),
		Peers:     make(map[string]interface{}),
		Mesh:      make(map[string]interface{}),
		Metadata:  make(map[string]interface{}),
	}

	// Collect streams
	streams, err := s.streamRepo.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list streams: %w", err)
	}

	for _, stream := range streams {
		data.Streams[string(stream.ID)] = stream
	}

	// Collect peers (for each stream)
	for streamIDStr := range data.Streams {
		streamID := domain.StreamID(streamIDStr)
		peers, err := s.peerRepo.FindByStream(ctx, streamID)
		if err != nil {
			s.logger.Warnw("failed to find peers for stream", "stream_id", streamID, "error", err)
			continue
		}

		for _, peer := range peers {
			data.Peers[string(peer.ID)] = peer
		}
	}

	// Add metadata
	data.Metadata["stream_count"] = len(data.Streams)
	data.Metadata["peer_count"] = len(data.Peers)
	data.Metadata["backup_type"] = "scheduled"

	return data, nil
}

// cleanupOldBackups removes backups older than retention period
func (s *Scheduler) cleanupOldBackups(ctx context.Context) error {
	backups, err := s.backupService.ListBackups(ctx)
	if err != nil {
		return fmt.Errorf("failed to list backups: %w", err)
	}

	cutoffTime := time.Now().AddDate(0, 0, -s.retentionDays)

	for _, backupName := range backups {
		// Parse timestamp from backup name (format: backup-20060102-150405.json)
		if len(backupName) < 20 {
			continue
		}

		// Extract timestamp part
		timestampStr := backupName[7:22] // "backup-" + "20060102-150405"
		timestamp, err := time.Parse("20060102-150405", timestampStr)
		if err != nil {
			s.logger.Warnw("failed to parse backup timestamp", "backup_name", backupName, "error", err)
			continue
		}

		// Delete if older than retention period
		if timestamp.Before(cutoffTime) {
			if err := s.backupService.DeleteBackup(ctx, backupName); err != nil {
				s.logger.Warnw("failed to delete old backup", "backup_name", backupName, "error", err)
				continue
			}
			s.logger.Infow("deleted old backup", "backup_name", backupName, "age", time.Since(timestamp))
		}
	}

	return nil
}

