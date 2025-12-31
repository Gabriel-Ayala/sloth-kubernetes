package backup

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/chalkan3/sloth-kubernetes/pkg/provisioning"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Mocks
// =============================================================================

// MockBackupStorage implements BackupStorage for testing
type MockBackupStorage struct {
	name            string
	data            map[string][]byte
	uploadError     error
	downloadError   error
	deleteError     error
	uploadCalls     int
	downloadCalls   int
	deleteCalls     int
}

func NewMockBackupStorage(name string) *MockBackupStorage {
	return &MockBackupStorage{
		name: name,
		data: make(map[string][]byte),
	}
}

func (m *MockBackupStorage) Name() string {
	return m.name
}

func (m *MockBackupStorage) Upload(ctx context.Context, key string, data []byte) error {
	m.uploadCalls++
	if m.uploadError != nil {
		return m.uploadError
	}
	m.data[key] = data
	return nil
}

func (m *MockBackupStorage) Download(ctx context.Context, key string) ([]byte, error) {
	m.downloadCalls++
	if m.downloadError != nil {
		return nil, m.downloadError
	}
	data, exists := m.data[key]
	if !exists {
		return nil, errors.New("not found")
	}
	return data, nil
}

func (m *MockBackupStorage) Delete(ctx context.Context, key string) error {
	m.deleteCalls++
	if m.deleteError != nil {
		return m.deleteError
	}
	delete(m.data, key)
	return nil
}

func (m *MockBackupStorage) List(ctx context.Context) ([]string, error) {
	keys := make([]string, 0)
	for key := range m.data {
		keys = append(keys, key)
	}
	return keys, nil
}

// MockBackupComponent implements BackupComponent for testing
type MockBackupComponent struct {
	name         string
	backupData   []byte
	backupError  error
	restoreError error
	backupCalls  int
	restoreCalls int
	restoredData []byte
}

func (m *MockBackupComponent) Name() string {
	return m.name
}

func (m *MockBackupComponent) Backup(ctx context.Context) ([]byte, error) {
	m.backupCalls++
	if m.backupError != nil {
		return nil, m.backupError
	}
	return m.backupData, nil
}

func (m *MockBackupComponent) Restore(ctx context.Context, data []byte) error {
	m.restoreCalls++
	if m.restoreError != nil {
		return m.restoreError
	}
	m.restoredData = data
	return nil
}

// MockEventEmitter implements EventEmitter for testing
type MockEventEmitter struct {
	events []provisioning.Event
}

func (m *MockEventEmitter) Emit(event provisioning.Event) {
	m.events = append(m.events, event)
}

func (m *MockEventEmitter) Subscribe(eventType string, handler provisioning.EventHandler) string {
	return "sub-1"
}

func (m *MockEventEmitter) Unsubscribe(subscriptionID string) {}

// =============================================================================
// Manager Tests
// =============================================================================

func TestNewManager_Success(t *testing.T) {
	storage := NewMockBackupStorage("test-storage")
	cfg := &ManagerConfig{
		Storage: storage,
	}

	manager, err := NewManager(cfg)
	require.NoError(t, err)
	assert.NotNil(t, manager)
}

func TestNewManager_NoStorage(t *testing.T) {
	cfg := &ManagerConfig{
		Storage: nil,
	}

	_, err := NewManager(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "storage is required")
}

func TestNewManager_WithConfig(t *testing.T) {
	storage := NewMockBackupStorage("test-storage")
	cfg := &ManagerConfig{
		Storage: storage,
		BackupConfig: &config.BackupConfig{
			RetentionDays: 30,
		},
	}

	manager, err := NewManager(cfg)
	require.NoError(t, err)
	assert.Equal(t, 30, manager.config.RetentionDays)
}

func TestManager_RegisterComponent(t *testing.T) {
	storage := NewMockBackupStorage("test-storage")
	manager, _ := NewManager(&ManagerConfig{Storage: storage})

	component := &MockBackupComponent{name: "etcd"}
	manager.RegisterComponent(component)

	assert.Len(t, manager.components, 1)
}

func TestManager_RegisterMultipleComponents(t *testing.T) {
	storage := NewMockBackupStorage("test-storage")
	manager, _ := NewManager(&ManagerConfig{Storage: storage})

	components := []*MockBackupComponent{
		{name: "etcd"},
		{name: "pki"},
		{name: "manifests"},
	}

	for _, c := range components {
		manager.RegisterComponent(c)
	}

	assert.Len(t, manager.components, 3)
}

func TestManager_CreateBackup_Success(t *testing.T) {
	storage := NewMockBackupStorage("test-storage")
	eventEmitter := &MockEventEmitter{}

	manager, _ := NewManager(&ManagerConfig{
		Storage:      storage,
		EventEmitter: eventEmitter,
	})

	component := &MockBackupComponent{
		name:       "etcd",
		backupData: []byte("etcd backup data"),
	}
	manager.RegisterComponent(component)

	backup, err := manager.CreateBackup(context.Background(), []string{"etcd"})
	require.NoError(t, err)

	assert.NotEmpty(t, backup.ID)
	assert.Equal(t, "completed", backup.Status)
	assert.Equal(t, "test-storage", backup.Location)
	assert.Contains(t, backup.Components, "etcd")
	assert.Greater(t, backup.Size, int64(0))
	assert.Equal(t, 1, component.backupCalls)
	assert.Equal(t, 1, storage.uploadCalls)

	// Verify events
	hasStarted := false
	hasCompleted := false
	for _, event := range eventEmitter.events {
		if event.Type == "backup_started" {
			hasStarted = true
		}
		if event.Type == "backup_completed" {
			hasCompleted = true
		}
	}
	assert.True(t, hasStarted)
	assert.True(t, hasCompleted)
}

func TestManager_CreateBackup_AllComponents(t *testing.T) {
	storage := NewMockBackupStorage("test-storage")
	manager, _ := NewManager(&ManagerConfig{Storage: storage})

	component1 := &MockBackupComponent{name: "etcd", backupData: []byte("etcd data")}
	component2 := &MockBackupComponent{name: "pki", backupData: []byte("pki data")}
	manager.RegisterComponent(component1)
	manager.RegisterComponent(component2)

	// Pass nil to backup all components
	backup, err := manager.CreateBackup(context.Background(), nil)
	require.NoError(t, err)

	assert.Len(t, backup.Components, 2)
	assert.Equal(t, 1, component1.backupCalls)
	assert.Equal(t, 1, component2.backupCalls)
}

func TestManager_CreateBackup_ComponentError(t *testing.T) {
	storage := NewMockBackupStorage("test-storage")
	eventEmitter := &MockEventEmitter{}

	manager, _ := NewManager(&ManagerConfig{
		Storage:      storage,
		EventEmitter: eventEmitter,
	})

	component := &MockBackupComponent{
		name:        "etcd",
		backupError: errors.New("backup failed"),
	}
	manager.RegisterComponent(component)

	backup, err := manager.CreateBackup(context.Background(), []string{"etcd"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to backup component etcd")
	assert.Equal(t, "failed", backup.Status)
}

func TestManager_CreateBackup_StorageError(t *testing.T) {
	storage := NewMockBackupStorage("test-storage")
	storage.uploadError = errors.New("storage upload failed")

	manager, _ := NewManager(&ManagerConfig{Storage: storage})

	component := &MockBackupComponent{name: "etcd", backupData: []byte("data")}
	manager.RegisterComponent(component)

	backup, err := manager.CreateBackup(context.Background(), []string{"etcd"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to store component")
	assert.Equal(t, "failed", backup.Status)
}

func TestManager_CreateBackup_UnregisteredComponent(t *testing.T) {
	storage := NewMockBackupStorage("test-storage")
	eventEmitter := &MockEventEmitter{}

	manager, _ := NewManager(&ManagerConfig{
		Storage:      storage,
		EventEmitter: eventEmitter,
	})

	// Try to backup unregistered component
	backup, err := manager.CreateBackup(context.Background(), []string{"nonexistent"})
	require.NoError(t, err)

	assert.Equal(t, "completed", backup.Status)
	assert.Equal(t, int64(0), backup.Size)

	// Should have skipped event
	hasSkipped := false
	for _, event := range eventEmitter.events {
		if event.Type == "backup_component_skipped" {
			hasSkipped = true
		}
	}
	assert.True(t, hasSkipped)
}

func TestManager_CreateBackup_WithRetention(t *testing.T) {
	storage := NewMockBackupStorage("test-storage")
	manager, _ := NewManager(&ManagerConfig{
		Storage: storage,
		BackupConfig: &config.BackupConfig{
			RetentionDays: 7,
		},
	})

	component := &MockBackupComponent{name: "etcd", backupData: []byte("data")}
	manager.RegisterComponent(component)

	backup, err := manager.CreateBackup(context.Background(), []string{"etcd"})
	require.NoError(t, err)

	expectedExpiration := time.Now().AddDate(0, 0, 7).Unix()
	// Allow 1 second tolerance
	assert.InDelta(t, expectedExpiration, backup.ExpiresAt, 1)
}

func TestManager_RestoreBackup_Success(t *testing.T) {
	storage := NewMockBackupStorage("test-storage")
	eventEmitter := &MockEventEmitter{}

	manager, _ := NewManager(&ManagerConfig{
		Storage:      storage,
		EventEmitter: eventEmitter,
	})

	component := &MockBackupComponent{name: "etcd", backupData: []byte("etcd backup data")}
	manager.RegisterComponent(component)

	// Create backup first
	backup, err := manager.CreateBackup(context.Background(), []string{"etcd"})
	require.NoError(t, err)

	// Clear component's restored data to verify restore
	component.restoredData = nil

	// Restore backup
	err = manager.RestoreBackup(context.Background(), backup.ID)
	require.NoError(t, err)

	assert.Equal(t, 1, component.restoreCalls)
	assert.Equal(t, []byte("etcd backup data"), component.restoredData)

	// Verify events
	hasRestoreCompleted := false
	for _, event := range eventEmitter.events {
		if event.Type == "restore_completed" {
			hasRestoreCompleted = true
		}
	}
	assert.True(t, hasRestoreCompleted)
}

func TestManager_RestoreBackup_NotFound(t *testing.T) {
	storage := NewMockBackupStorage("test-storage")
	manager, _ := NewManager(&ManagerConfig{Storage: storage})

	err := manager.RestoreBackup(context.Background(), "nonexistent-backup")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestManager_RestoreBackup_DownloadError(t *testing.T) {
	storage := NewMockBackupStorage("test-storage")
	manager, _ := NewManager(&ManagerConfig{Storage: storage})

	component := &MockBackupComponent{name: "etcd", backupData: []byte("data")}
	manager.RegisterComponent(component)

	// Create backup
	backup, _ := manager.CreateBackup(context.Background(), []string{"etcd"})

	// Set download error
	storage.downloadError = errors.New("download failed")

	err := manager.RestoreBackup(context.Background(), backup.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to download")
}

func TestManager_RestoreBackup_RestoreError(t *testing.T) {
	storage := NewMockBackupStorage("test-storage")
	manager, _ := NewManager(&ManagerConfig{Storage: storage})

	component := &MockBackupComponent{name: "etcd", backupData: []byte("data")}
	manager.RegisterComponent(component)

	// Create backup
	backup, _ := manager.CreateBackup(context.Background(), []string{"etcd"})

	// Set restore error
	component.restoreError = errors.New("restore failed")

	err := manager.RestoreBackup(context.Background(), backup.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to restore")
}

func TestManager_ListBackups(t *testing.T) {
	storage := NewMockBackupStorage("test-storage")
	manager, _ := NewManager(&ManagerConfig{Storage: storage})

	component := &MockBackupComponent{name: "etcd", backupData: []byte("data")}
	manager.RegisterComponent(component)

	// Create multiple backups
	_, err := manager.CreateBackup(context.Background(), []string{"etcd"})
	require.NoError(t, err)
	_, err = manager.CreateBackup(context.Background(), []string{"etcd"})
	require.NoError(t, err)
	_, err = manager.CreateBackup(context.Background(), []string{"etcd"})
	require.NoError(t, err)

	backups, err := manager.ListBackups(context.Background())
	require.NoError(t, err)

	assert.Len(t, backups, 3)
}

func TestManager_ListBackups_Empty(t *testing.T) {
	storage := NewMockBackupStorage("test-storage")
	manager, _ := NewManager(&ManagerConfig{Storage: storage})

	backups, err := manager.ListBackups(context.Background())
	require.NoError(t, err)

	assert.Empty(t, backups)
}

func TestManager_DeleteBackup_Success(t *testing.T) {
	storage := NewMockBackupStorage("test-storage")
	eventEmitter := &MockEventEmitter{}

	manager, _ := NewManager(&ManagerConfig{
		Storage:      storage,
		EventEmitter: eventEmitter,
	})

	component := &MockBackupComponent{name: "etcd", backupData: []byte("data")}
	manager.RegisterComponent(component)

	// Create backup
	backup, _ := manager.CreateBackup(context.Background(), []string{"etcd"})

	// Delete backup
	err := manager.DeleteBackup(context.Background(), backup.ID)
	require.NoError(t, err)

	// Verify removed from list
	backups, _ := manager.ListBackups(context.Background())
	assert.Empty(t, backups)

	// Verify delete event
	hasDeleteEvent := false
	for _, event := range eventEmitter.events {
		if event.Type == "backup_deleted" {
			hasDeleteEvent = true
		}
	}
	assert.True(t, hasDeleteEvent)
}

func TestManager_DeleteBackup_NotFound(t *testing.T) {
	storage := NewMockBackupStorage("test-storage")
	manager, _ := NewManager(&ManagerConfig{Storage: storage})

	err := manager.DeleteBackup(context.Background(), "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestManager_DeleteBackup_StorageError(t *testing.T) {
	storage := NewMockBackupStorage("test-storage")
	eventEmitter := &MockEventEmitter{}

	manager, _ := NewManager(&ManagerConfig{
		Storage:      storage,
		EventEmitter: eventEmitter,
	})

	component := &MockBackupComponent{name: "etcd", backupData: []byte("data")}
	manager.RegisterComponent(component)

	// Create backup
	backup, _ := manager.CreateBackup(context.Background(), []string{"etcd"})

	// Set storage delete error
	storage.deleteError = errors.New("delete failed")

	// Delete should still succeed (logs error but continues)
	err := manager.DeleteBackup(context.Background(), backup.ID)
	require.NoError(t, err)

	// Should have error event
	hasErrorEvent := false
	for _, event := range eventEmitter.events {
		if event.Type == "backup_delete_component_failed" {
			hasErrorEvent = true
		}
	}
	assert.True(t, hasErrorEvent)
}

func TestManager_ScheduleBackup(t *testing.T) {
	storage := NewMockBackupStorage("test-storage")
	eventEmitter := &MockEventEmitter{}

	manager, _ := NewManager(&ManagerConfig{
		Storage:      storage,
		EventEmitter: eventEmitter,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := manager.ScheduleBackup(ctx, "0 2 * * *")
	require.NoError(t, err)

	assert.True(t, manager.isScheduled)
	assert.NotNil(t, manager.scheduler)

	// Verify event
	hasScheduleEvent := false
	for _, event := range eventEmitter.events {
		if event.Type == "backup_schedule_started" {
			hasScheduleEvent = true
		}
	}
	assert.True(t, hasScheduleEvent)

	// Cleanup
	manager.StopScheduledBackups()
}

func TestManager_ScheduleBackup_AlreadyScheduled(t *testing.T) {
	storage := NewMockBackupStorage("test-storage")
	manager, _ := NewManager(&ManagerConfig{Storage: storage})

	ctx := context.Background()

	err := manager.ScheduleBackup(ctx, "0 2 * * *")
	require.NoError(t, err)

	// Try to schedule again
	err = manager.ScheduleBackup(ctx, "0 3 * * *")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already scheduled")

	manager.StopScheduledBackups()
}

func TestManager_StopScheduledBackups(t *testing.T) {
	storage := NewMockBackupStorage("test-storage")
	manager, _ := NewManager(&ManagerConfig{Storage: storage})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager.ScheduleBackup(ctx, "0 2 * * *")
	manager.StopScheduledBackups()

	assert.False(t, manager.isScheduled)
}

func TestManager_StopScheduledBackups_NotRunning(t *testing.T) {
	storage := NewMockBackupStorage("test-storage")
	manager, _ := NewManager(&ManagerConfig{Storage: storage})

	// Should not panic
	manager.StopScheduledBackups()
}

func TestManager_CleanupExpiredBackups(t *testing.T) {
	storage := NewMockBackupStorage("test-storage")
	eventEmitter := &MockEventEmitter{}

	manager, _ := NewManager(&ManagerConfig{
		Storage:      storage,
		EventEmitter: eventEmitter,
		BackupConfig: &config.BackupConfig{
			RetentionDays: 0, // No retention
		},
	})

	component := &MockBackupComponent{name: "etcd", backupData: []byte("data")}
	manager.RegisterComponent(component)

	// Create backup and manually set it as expired
	backup, _ := manager.CreateBackup(context.Background(), []string{"etcd"})
	backup.ExpiresAt = time.Now().Add(-1 * time.Hour).Unix() // Expired 1 hour ago

	// Cleanup
	deleted, err := manager.CleanupExpiredBackups(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 1, deleted)

	// Verify removed
	backups, _ := manager.ListBackups(context.Background())
	assert.Empty(t, backups)
}

func TestManager_CleanupExpiredBackups_NoExpired(t *testing.T) {
	storage := NewMockBackupStorage("test-storage")
	manager, _ := NewManager(&ManagerConfig{
		Storage: storage,
		BackupConfig: &config.BackupConfig{
			RetentionDays: 30,
		},
	})

	component := &MockBackupComponent{name: "etcd", backupData: []byte("data")}
	manager.RegisterComponent(component)

	// Create backup (will have 30 days retention)
	manager.CreateBackup(context.Background(), []string{"etcd"})

	// Cleanup should delete nothing
	deleted, err := manager.CleanupExpiredBackups(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 0, deleted)

	backups, _ := manager.ListBackups(context.Background())
	assert.Len(t, backups, 1)
}

func TestManager_EventEmitter_Nil(t *testing.T) {
	storage := NewMockBackupStorage("test-storage")
	manager, _ := NewManager(&ManagerConfig{
		Storage:      storage,
		EventEmitter: nil,
	})

	component := &MockBackupComponent{name: "etcd", backupData: []byte("data")}
	manager.RegisterComponent(component)

	// Should not panic
	_, err := manager.CreateBackup(context.Background(), []string{"etcd"})
	require.NoError(t, err)
}

// =============================================================================
// Scheduler Tests
// =============================================================================

func TestNewScheduler(t *testing.T) {
	called := false
	scheduler := NewScheduler("0 2 * * *", func() {
		called = true
	})

	assert.NotNil(t, scheduler)
	assert.Equal(t, "0 2 * * *", scheduler.schedule)
	assert.False(t, called)
}

func TestScheduler_Stop(t *testing.T) {
	scheduler := NewScheduler("0 2 * * *", func() {})

	ctx, cancel := context.WithCancel(context.Background())

	// Start scheduler in goroutine
	go scheduler.Start(ctx)

	// Give it time to start
	time.Sleep(10 * time.Millisecond)

	// Stop scheduler
	scheduler.Stop()

	assert.False(t, scheduler.isRunning)

	cancel()
}

func TestScheduler_StopTwice(t *testing.T) {
	scheduler := NewScheduler("0 2 * * *", func() {})

	ctx, cancel := context.WithCancel(context.Background())
	go scheduler.Start(ctx)
	time.Sleep(10 * time.Millisecond)

	// Stop twice should not panic
	scheduler.Stop()
	scheduler.Stop()

	cancel()
}

func TestScheduler_ContextCancel(t *testing.T) {
	scheduler := NewScheduler("0 2 * * *", func() {})

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan bool)
	go func() {
		scheduler.Start(ctx)
		done <- true
	}()

	time.Sleep(10 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// Success
	case <-time.After(time.Second):
		t.Error("Scheduler did not stop on context cancel")
	}
}

// =============================================================================
// Integration-like Tests
// =============================================================================

func TestManager_FullBackupRestoreWorkflow(t *testing.T) {
	storage := NewMockBackupStorage("test-storage")
	eventEmitter := &MockEventEmitter{}

	manager, _ := NewManager(&ManagerConfig{
		Storage:      storage,
		EventEmitter: eventEmitter,
		BackupConfig: &config.BackupConfig{
			RetentionDays: 7,
		},
	})

	// Register multiple components
	etcdComponent := &MockBackupComponent{name: "etcd", backupData: []byte("etcd snapshot data")}
	pkiComponent := &MockBackupComponent{name: "pki", backupData: []byte("pki certificates")}
	manifestsComponent := &MockBackupComponent{name: "manifests", backupData: []byte("kubernetes manifests")}

	manager.RegisterComponent(etcdComponent)
	manager.RegisterComponent(pkiComponent)
	manager.RegisterComponent(manifestsComponent)

	ctx := context.Background()

	// Step 1: Create full backup
	backup, err := manager.CreateBackup(ctx, nil)
	require.NoError(t, err)

	assert.Equal(t, "completed", backup.Status)
	assert.Len(t, backup.Components, 3)
	assert.Equal(t, 1, etcdComponent.backupCalls)
	assert.Equal(t, 1, pkiComponent.backupCalls)
	assert.Equal(t, 1, manifestsComponent.backupCalls)

	// Step 2: List backups
	backups, _ := manager.ListBackups(ctx)
	assert.Len(t, backups, 1)

	// Step 3: Restore backup
	err = manager.RestoreBackup(ctx, backup.ID)
	require.NoError(t, err)

	assert.Equal(t, []byte("etcd snapshot data"), etcdComponent.restoredData)
	assert.Equal(t, []byte("pki certificates"), pkiComponent.restoredData)
	assert.Equal(t, []byte("kubernetes manifests"), manifestsComponent.restoredData)

	// Step 4: Delete backup
	err = manager.DeleteBackup(ctx, backup.ID)
	require.NoError(t, err)

	backups, _ = manager.ListBackups(ctx)
	assert.Empty(t, backups)
}

func TestManager_PartialBackup(t *testing.T) {
	storage := NewMockBackupStorage("test-storage")
	manager, _ := NewManager(&ManagerConfig{Storage: storage})

	etcdComponent := &MockBackupComponent{name: "etcd", backupData: []byte("etcd data")}
	pkiComponent := &MockBackupComponent{name: "pki", backupData: []byte("pki data")}

	manager.RegisterComponent(etcdComponent)
	manager.RegisterComponent(pkiComponent)

	// Backup only etcd
	backup, err := manager.CreateBackup(context.Background(), []string{"etcd"})
	require.NoError(t, err)

	assert.Len(t, backup.Components, 1)
	assert.Contains(t, backup.Components, "etcd")
	assert.Equal(t, 1, etcdComponent.backupCalls)
	assert.Equal(t, 0, pkiComponent.backupCalls)
}

func TestManager_MultipleBackupsAndCleanup(t *testing.T) {
	storage := NewMockBackupStorage("test-storage")
	manager, _ := NewManager(&ManagerConfig{
		Storage: storage,
		BackupConfig: &config.BackupConfig{
			RetentionDays: 1,
		},
	})

	component := &MockBackupComponent{name: "etcd", backupData: []byte("data")}
	manager.RegisterComponent(component)

	ctx := context.Background()

	// Create 3 backups
	backup1, _ := manager.CreateBackup(ctx, nil)
	backup2, _ := manager.CreateBackup(ctx, nil)
	backup3, _ := manager.CreateBackup(ctx, nil)

	// Mark first two as expired
	backup1.ExpiresAt = time.Now().Add(-2 * time.Hour).Unix()
	backup2.ExpiresAt = time.Now().Add(-1 * time.Hour).Unix()
	// backup3 keeps its future expiration

	// Cleanup
	deleted, err := manager.CleanupExpiredBackups(ctx)
	require.NoError(t, err)

	assert.Equal(t, 2, deleted)

	// Only backup3 should remain
	backups, _ := manager.ListBackups(ctx)
	assert.Len(t, backups, 1)
	assert.Equal(t, backup3.ID, backups[0].ID)
}
