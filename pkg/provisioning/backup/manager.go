// Package backup provides cluster backup and restore functionality
package backup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/chalkan3/sloth-kubernetes/pkg/provisioning"
)

// =============================================================================
// Backup Manager
// =============================================================================

// Manager orchestrates backup operations
type Manager struct {
	config       *config.BackupConfig
	storage      provisioning.BackupStorage
	components   map[string]provisioning.BackupComponent
	scheduler    *Scheduler
	eventEmitter provisioning.EventEmitter

	// State
	backups       []*provisioning.Backup
	isScheduled   bool
	stopScheduler chan struct{}

	mu sync.RWMutex
}

// ManagerConfig holds manager configuration
type ManagerConfig struct {
	BackupConfig *config.BackupConfig
	Storage      provisioning.BackupStorage
	EventEmitter provisioning.EventEmitter
}

// NewManager creates a new backup manager
func NewManager(cfg *ManagerConfig) (*Manager, error) {
	if cfg.Storage == nil {
		return nil, fmt.Errorf("backup storage is required")
	}

	return &Manager{
		config:        cfg.BackupConfig,
		storage:       cfg.Storage,
		components:    make(map[string]provisioning.BackupComponent),
		eventEmitter:  cfg.EventEmitter,
		backups:       make([]*provisioning.Backup, 0),
		stopScheduler: make(chan struct{}),
	}, nil
}

// RegisterComponent registers a backup component
func (m *Manager) RegisterComponent(component provisioning.BackupComponent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.components[component.Name()] = component
}

// CreateBackup creates a new backup
func (m *Manager) CreateBackup(ctx context.Context, componentNames []string) (*provisioning.Backup, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	backupID := m.generateBackupID()

	m.emitEvent("backup_started", map[string]interface{}{
		"backup_id":  backupID,
		"components": componentNames,
	})

	// Determine which components to backup
	if len(componentNames) == 0 {
		componentNames = make([]string, 0, len(m.components))
		for name := range m.components {
			componentNames = append(componentNames, name)
		}
	}

	// Create backup
	backup := &provisioning.Backup{
		ID:         backupID,
		Components: componentNames,
		CreatedAt:  time.Now().Unix(),
		Status:     "in_progress",
		Metadata:   make(map[string]string),
	}

	// Backup each component
	totalSize := int64(0)
	for _, name := range componentNames {
		component, exists := m.components[name]
		if !exists {
			m.emitEvent("backup_component_skipped", map[string]interface{}{
				"backup_id": backupID,
				"component": name,
				"reason":    "not_registered",
			})
			continue
		}

		// Create component backup
		data, err := component.Backup(ctx)
		if err != nil {
			backup.Status = "failed"
			m.emitEvent("backup_component_failed", map[string]interface{}{
				"backup_id": backupID,
				"component": name,
				"error":     err.Error(),
			})
			return backup, fmt.Errorf("failed to backup component %s: %w", name, err)
		}

		// Store component data
		componentKey := fmt.Sprintf("%s/%s", backupID, name)
		if err := m.storage.Upload(ctx, componentKey, data); err != nil {
			backup.Status = "failed"
			return backup, fmt.Errorf("failed to store component %s: %w", name, err)
		}

		totalSize += int64(len(data))
		backup.Metadata[name] = fmt.Sprintf("%d bytes", len(data))

		m.emitEvent("backup_component_completed", map[string]interface{}{
			"backup_id": backupID,
			"component": name,
			"size":      len(data),
		})
	}

	backup.Size = totalSize
	backup.Status = "completed"
	backup.Location = m.storage.Name()

	// Calculate expiration
	if m.config != nil && m.config.RetentionDays > 0 {
		backup.ExpiresAt = time.Now().AddDate(0, 0, m.config.RetentionDays).Unix()
	}

	m.backups = append(m.backups, backup)

	m.emitEvent("backup_completed", map[string]interface{}{
		"backup_id":  backupID,
		"size":       totalSize,
		"components": componentNames,
	})

	return backup, nil
}

// RestoreBackup restores from a backup
func (m *Manager) RestoreBackup(ctx context.Context, backupID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find backup
	var backup *provisioning.Backup
	for _, b := range m.backups {
		if b.ID == backupID {
			backup = b
			break
		}
	}

	if backup == nil {
		return fmt.Errorf("backup %s not found", backupID)
	}

	m.emitEvent("restore_started", map[string]interface{}{
		"backup_id": backupID,
	})

	// Restore each component
	for _, name := range backup.Components {
		component, exists := m.components[name]
		if !exists {
			m.emitEvent("restore_component_skipped", map[string]interface{}{
				"backup_id": backupID,
				"component": name,
				"reason":    "not_registered",
			})
			continue
		}

		// Download component data
		componentKey := fmt.Sprintf("%s/%s", backupID, name)
		data, err := m.storage.Download(ctx, componentKey)
		if err != nil {
			m.emitEvent("restore_component_failed", map[string]interface{}{
				"backup_id": backupID,
				"component": name,
				"error":     err.Error(),
			})
			return fmt.Errorf("failed to download component %s: %w", name, err)
		}

		// Restore component
		if err := component.Restore(ctx, data); err != nil {
			return fmt.Errorf("failed to restore component %s: %w", name, err)
		}

		m.emitEvent("restore_component_completed", map[string]interface{}{
			"backup_id": backupID,
			"component": name,
		})
	}

	m.emitEvent("restore_completed", map[string]interface{}{
		"backup_id": backupID,
	})

	return nil
}

// ListBackups returns all available backups
func (m *Manager) ListBackups(ctx context.Context) ([]*provisioning.Backup, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return copy of backups
	result := make([]*provisioning.Backup, len(m.backups))
	copy(result, m.backups)

	return result, nil
}

// DeleteBackup deletes a backup
func (m *Manager) DeleteBackup(ctx context.Context, backupID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find and remove backup
	var backup *provisioning.Backup
	for i, b := range m.backups {
		if b.ID == backupID {
			backup = b
			m.backups = append(m.backups[:i], m.backups[i+1:]...)
			break
		}
	}

	if backup == nil {
		return fmt.Errorf("backup %s not found", backupID)
	}

	// Delete from storage
	for _, component := range backup.Components {
		componentKey := fmt.Sprintf("%s/%s", backupID, component)
		if err := m.storage.Delete(ctx, componentKey); err != nil {
			// Log but continue
			m.emitEvent("backup_delete_component_failed", map[string]interface{}{
				"backup_id": backupID,
				"component": component,
				"error":     err.Error(),
			})
		}
	}

	m.emitEvent("backup_deleted", map[string]interface{}{
		"backup_id": backupID,
	})

	return nil
}

// ScheduleBackup starts scheduled backups
func (m *Manager) ScheduleBackup(ctx context.Context, schedule string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.isScheduled {
		return fmt.Errorf("backups are already scheduled")
	}

	m.scheduler = NewScheduler(schedule, func() {
		_, err := m.CreateBackup(ctx, nil)
		if err != nil {
			m.emitEvent("scheduled_backup_failed", map[string]interface{}{
				"error": err.Error(),
			})
		}
	})

	m.isScheduled = true
	go m.scheduler.Start(ctx)

	m.emitEvent("backup_schedule_started", map[string]interface{}{
		"schedule": schedule,
	})

	return nil
}

// StopScheduledBackups stops scheduled backups
func (m *Manager) StopScheduledBackups() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.scheduler != nil {
		m.scheduler.Stop()
		m.isScheduled = false
	}
}

// CleanupExpiredBackups removes expired backups
func (m *Manager) CleanupExpiredBackups(ctx context.Context) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().Unix()
	deleted := 0

	// Find expired backups
	for i := len(m.backups) - 1; i >= 0; i-- {
		backup := m.backups[i]
		if backup.ExpiresAt > 0 && backup.ExpiresAt < now {
			// Delete from storage
			for _, component := range backup.Components {
				componentKey := fmt.Sprintf("%s/%s", backup.ID, component)
				m.storage.Delete(ctx, componentKey)
			}

			// Remove from list
			m.backups = append(m.backups[:i], m.backups[i+1:]...)
			deleted++

			m.emitEvent("backup_expired_deleted", map[string]interface{}{
				"backup_id": backup.ID,
			})
		}
	}

	return deleted, nil
}

func (m *Manager) generateBackupID() string {
	timestamp := time.Now().Format("20060102-150405")
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s-%d", timestamp, time.Now().UnixNano())))
	return fmt.Sprintf("backup-%s-%s", timestamp, hex.EncodeToString(hash[:4]))
}

func (m *Manager) emitEvent(eventType string, data map[string]interface{}) {
	if m.eventEmitter != nil {
		m.eventEmitter.Emit(provisioning.Event{
			Type:      eventType,
			Timestamp: time.Now().Unix(),
			Data:      data,
			Source:    "backup_manager",
		})
	}
}

// =============================================================================
// Backup Scheduler
// =============================================================================

// Scheduler handles scheduled backup execution
type Scheduler struct {
	schedule  string
	callback  func()
	stopChan  chan struct{}
	isRunning bool
	mu        sync.Mutex
}

// NewScheduler creates a new scheduler
func NewScheduler(schedule string, callback func()) *Scheduler {
	return &Scheduler{
		schedule: schedule,
		callback: callback,
		stopChan: make(chan struct{}),
	}
}

// Start begins the scheduler
func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	if s.isRunning {
		s.mu.Unlock()
		return
	}
	s.isRunning = true
	s.mu.Unlock()

	interval := s.parseSchedule()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.callback()
		}
	}
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isRunning {
		close(s.stopChan)
		s.stopChan = make(chan struct{})
		s.isRunning = false
	}
}

func (s *Scheduler) parseSchedule() time.Duration {
	// Simplified cron parsing - production would use proper cron library
	// Default to daily if can't parse
	return 24 * time.Hour
}
