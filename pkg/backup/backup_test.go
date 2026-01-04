package backup

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	tests := []struct {
		name       string
		kubeconfig string
		wantNS     string
	}{
		{
			name:       "with kubeconfig",
			kubeconfig: "/path/to/kubeconfig",
			wantNS:     "velero",
		},
		{
			name:       "empty kubeconfig",
			kubeconfig: "",
			wantNS:     "velero",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager(tt.kubeconfig)
			if m == nil {
				t.Fatal("NewManager returned nil")
			}
			if m.kubeconfig != tt.kubeconfig {
				t.Errorf("kubeconfig = %q, want %q", m.kubeconfig, tt.kubeconfig)
			}
			if m.namespace != tt.wantNS {
				t.Errorf("namespace = %q, want %q", m.namespace, tt.wantNS)
			}
		})
	}
}

func TestSetNamespace(t *testing.T) {
	m := NewManager("")
	m.SetNamespace("custom-velero")
	if m.namespace != "custom-velero" {
		t.Errorf("namespace = %q, want %q", m.namespace, "custom-velero")
	}
}

func TestSetVerbose(t *testing.T) {
	m := NewManager("")
	m.SetVerbose(true)
	if !m.verbose {
		t.Error("verbose should be true")
	}
}

func TestBackupStatus(t *testing.T) {
	tests := []struct {
		status BackupStatus
		want   string
	}{
		{StatusNew, "New"},
		{StatusInProgress, "InProgress"},
		{StatusCompleted, "Completed"},
		{StatusFailed, "Failed"},
		{StatusPartiallyFailed, "PartiallyFailed"},
		{StatusDeleting, "Deleting"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if string(tt.status) != tt.want {
				t.Errorf("status = %q, want %q", tt.status, tt.want)
			}
		})
	}
}

func TestRestoreStatus(t *testing.T) {
	tests := []struct {
		status RestoreStatus
		want   string
	}{
		{RestoreStatusNew, "New"},
		{RestoreStatusInProgress, "InProgress"},
		{RestoreStatusCompleted, "Completed"},
		{RestoreStatusFailed, "Failed"},
		{RestoreStatusPartiallyFailed, "PartiallyFailed"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if string(tt.status) != tt.want {
				t.Errorf("status = %q, want %q", tt.status, tt.want)
			}
		})
	}
}

func TestBackupConfig(t *testing.T) {
	config := BackupConfig{
		Name:               "test-backup",
		IncludedNamespaces: []string{"default", "app"},
		ExcludedNamespaces: []string{"kube-system"},
		IncludedResources:  []string{"deployments", "services"},
		ExcludedResources:  []string{"secrets"},
		Labels:             map[string]string{"env": "test"},
		StorageLocation:    "default",
		TTL:                "720h",
		SnapshotVolumes:    true,
		Wait:               true,
		Timeout:            30 * time.Minute,
	}

	if config.Name != "test-backup" {
		t.Errorf("Name = %q, want %q", config.Name, "test-backup")
	}
	if len(config.IncludedNamespaces) != 2 {
		t.Errorf("IncludedNamespaces length = %d, want %d", len(config.IncludedNamespaces), 2)
	}
	if config.TTL != "720h" {
		t.Errorf("TTL = %q, want %q", config.TTL, "720h")
	}
	if !config.SnapshotVolumes {
		t.Error("SnapshotVolumes should be true")
	}
}

func TestRestoreConfig(t *testing.T) {
	config := RestoreConfig{
		Name:               "test-restore",
		BackupName:         "test-backup",
		IncludedNamespaces: []string{"default"},
		ExcludedNamespaces: []string{"kube-system"},
		IncludedResources:  []string{"deployments"},
		ExcludedResources:  []string{"secrets"},
		RestorePVs:         true,
		PreserveNodePorts:  true,
		Wait:               true,
		Timeout:            30 * time.Minute,
	}

	if config.Name != "test-restore" {
		t.Errorf("Name = %q, want %q", config.Name, "test-restore")
	}
	if config.BackupName != "test-backup" {
		t.Errorf("BackupName = %q, want %q", config.BackupName, "test-backup")
	}
	if !config.RestorePVs {
		t.Error("RestorePVs should be true")
	}
	if !config.PreserveNodePorts {
		t.Error("PreserveNodePorts should be true")
	}
}

func TestScheduleConfig(t *testing.T) {
	config := ScheduleConfig{
		Name:               "daily-backup",
		Schedule:           "0 0 * * *",
		IncludedNamespaces: []string{"default"},
		ExcludedNamespaces: []string{"kube-system"},
		TTL:                "720h",
		SnapshotVolumes:    true,
	}

	if config.Name != "daily-backup" {
		t.Errorf("Name = %q, want %q", config.Name, "daily-backup")
	}
	if config.Schedule != "0 0 * * *" {
		t.Errorf("Schedule = %q, want %q", config.Schedule, "0 0 * * *")
	}
}

func TestBackupStruct(t *testing.T) {
	now := time.Now()
	backup := Backup{
		Name:                "test-backup",
		Namespace:           "velero",
		Status:              StatusCompleted,
		Phase:               "Completed",
		IncludedNamespaces:  []string{"default"},
		ExcludedNamespaces:  []string{"kube-system"},
		IncludedResources:   []string{"deployments"},
		ExcludedResources:   []string{"secrets"},
		Labels:              map[string]string{"env": "test"},
		StorageLocation:     "default",
		TTL:                 "720h",
		StartTimestamp:      now,
		CompletionTimestamp: now.Add(5 * time.Minute),
		Expiration:          now.Add(720 * time.Hour),
		TotalItems:          100,
		ItemsBackedUp:       100,
		Errors:              0,
		Warnings:            2,
	}

	if backup.Name != "test-backup" {
		t.Errorf("Name = %q, want %q", backup.Name, "test-backup")
	}
	if backup.Status != StatusCompleted {
		t.Errorf("Status = %q, want %q", backup.Status, StatusCompleted)
	}
	if backup.TotalItems != 100 {
		t.Errorf("TotalItems = %d, want %d", backup.TotalItems, 100)
	}
	if backup.Warnings != 2 {
		t.Errorf("Warnings = %d, want %d", backup.Warnings, 2)
	}
}

func TestRestoreStruct(t *testing.T) {
	now := time.Now()
	restore := Restore{
		Name:                "test-restore",
		Namespace:           "velero",
		BackupName:          "test-backup",
		Status:              RestoreStatusCompleted,
		Phase:               "Completed",
		IncludedNamespaces:  []string{"default"},
		ExcludedNamespaces:  []string{"kube-system"},
		IncludedResources:   []string{"deployments"},
		ExcludedResources:   []string{"secrets"},
		RestorePVs:          true,
		StartTimestamp:      now,
		CompletionTimestamp: now.Add(5 * time.Minute),
		Errors:              0,
		Warnings:            1,
	}

	if restore.Name != "test-restore" {
		t.Errorf("Name = %q, want %q", restore.Name, "test-restore")
	}
	if restore.BackupName != "test-backup" {
		t.Errorf("BackupName = %q, want %q", restore.BackupName, "test-backup")
	}
	if restore.Status != RestoreStatusCompleted {
		t.Errorf("Status = %q, want %q", restore.Status, RestoreStatusCompleted)
	}
}

func TestScheduleStruct(t *testing.T) {
	now := time.Now()
	schedule := Schedule{
		Name:               "daily-backup",
		Namespace:          "velero",
		Schedule:           "0 0 * * *",
		IncludedNamespaces: []string{"default"},
		ExcludedNamespaces: []string{"kube-system"},
		TTL:                "720h",
		LastBackup:         now,
		Paused:             false,
	}

	if schedule.Name != "daily-backup" {
		t.Errorf("Name = %q, want %q", schedule.Name, "daily-backup")
	}
	if schedule.Schedule != "0 0 * * *" {
		t.Errorf("Schedule = %q, want %q", schedule.Schedule, "0 0 * * *")
	}
	if schedule.Paused {
		t.Error("Paused should be false")
	}
}

func TestBackupLocationStruct(t *testing.T) {
	location := BackupLocation{
		Name:       "default",
		Provider:   "aws",
		Bucket:     "my-backup-bucket",
		Region:     "us-east-1",
		Default:    true,
		AccessMode: "ReadWrite",
	}

	if location.Name != "default" {
		t.Errorf("Name = %q, want %q", location.Name, "default")
	}
	if location.Provider != "aws" {
		t.Errorf("Provider = %q, want %q", location.Provider, "aws")
	}
	if !location.Default {
		t.Error("Default should be true")
	}
}

func TestParseBackupFromDescribe(t *testing.T) {
	m := NewManager("")

	tests := []struct {
		name       string
		backupName string
		output     string
		wantStatus BackupStatus
		wantErrors int
	}{
		{
			name:       "completed backup",
			backupName: "test-backup",
			output: `Name: test-backup
Phase: Completed
Errors: 0
Warnings: 2`,
			wantStatus: StatusCompleted,
			wantErrors: 0,
		},
		{
			name:       "failed backup",
			backupName: "failed-backup",
			output: `Name: failed-backup
Phase: Failed
Errors: 5
Warnings: 0`,
			wantStatus: StatusFailed,
			wantErrors: 5,
		},
		{
			name:       "in progress backup",
			backupName: "running-backup",
			output: `Name: running-backup
Phase: InProgress
Errors: 0
Warnings: 0`,
			wantStatus: StatusInProgress,
			wantErrors: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backup := m.parseBackupFromDescribe(tt.backupName, tt.output)
			if backup.Name != tt.backupName {
				t.Errorf("Name = %q, want %q", backup.Name, tt.backupName)
			}
			if backup.Status != tt.wantStatus {
				t.Errorf("Status = %q, want %q", backup.Status, tt.wantStatus)
			}
			if backup.Errors != tt.wantErrors {
				t.Errorf("Errors = %d, want %d", backup.Errors, tt.wantErrors)
			}
		})
	}
}

func TestParseRestoreFromDescribe(t *testing.T) {
	m := NewManager("")

	tests := []struct {
		name        string
		restoreName string
		output      string
		wantStatus  RestoreStatus
		wantBackup  string
	}{
		{
			name:        "completed restore",
			restoreName: "test-restore",
			output: `Name: test-restore
Backup: test-backup
Phase: Completed
Errors: 0
Warnings: 1`,
			wantStatus: RestoreStatusCompleted,
			wantBackup: "test-backup",
		},
		{
			name:        "failed restore",
			restoreName: "failed-restore",
			output: `Name: failed-restore
Backup: old-backup
Phase: Failed
Errors: 3
Warnings: 0`,
			wantStatus: RestoreStatusFailed,
			wantBackup: "old-backup",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restore := m.parseRestoreFromDescribe(tt.restoreName, tt.output)
			if restore.Name != tt.restoreName {
				t.Errorf("Name = %q, want %q", restore.Name, tt.restoreName)
			}
			if restore.Status != tt.wantStatus {
				t.Errorf("Status = %q, want %q", restore.Status, tt.wantStatus)
			}
			if restore.BackupName != tt.wantBackup {
				t.Errorf("BackupName = %q, want %q", restore.BackupName, tt.wantBackup)
			}
		})
	}
}

func TestParseScheduleFromDescribe(t *testing.T) {
	m := NewManager("")

	tests := []struct {
		name         string
		scheduleName string
		output       string
		wantSchedule string
		wantPaused   bool
	}{
		{
			name:         "active schedule",
			scheduleName: "daily-backup",
			output: `Name: daily-backup
Schedule: 0 0 * * *
Paused: false`,
			wantSchedule: "0 0 * * *",
			wantPaused:   false,
		},
		{
			name:         "paused schedule",
			scheduleName: "weekly-backup",
			output: `Name: weekly-backup
Schedule: 0 0 * * 0
Paused: true`,
			wantSchedule: "0 0 * * 0",
			wantPaused:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schedule := m.parseScheduleFromDescribe(tt.scheduleName, tt.output)
			if schedule.Name != tt.scheduleName {
				t.Errorf("Name = %q, want %q", schedule.Name, tt.scheduleName)
			}
			if schedule.Schedule != tt.wantSchedule {
				t.Errorf("Schedule = %q, want %q", schedule.Schedule, tt.wantSchedule)
			}
			if schedule.Paused != tt.wantPaused {
				t.Errorf("Paused = %v, want %v", schedule.Paused, tt.wantPaused)
			}
		})
	}
}

func TestBackupConfigDefaults(t *testing.T) {
	// Test empty config
	config := BackupConfig{}

	if config.Name != "" {
		t.Errorf("Name should be empty, got %q", config.Name)
	}
	if config.TTL != "" {
		t.Errorf("TTL should be empty, got %q", config.TTL)
	}
	if config.SnapshotVolumes {
		t.Error("SnapshotVolumes should default to false")
	}
	if config.Wait {
		t.Error("Wait should default to false")
	}
}

func TestRestoreConfigDefaults(t *testing.T) {
	// Test empty config
	config := RestoreConfig{}

	if config.Name != "" {
		t.Errorf("Name should be empty, got %q", config.Name)
	}
	if config.BackupName != "" {
		t.Errorf("BackupName should be empty, got %q", config.BackupName)
	}
	if config.RestorePVs {
		t.Error("RestorePVs should default to false")
	}
	if config.PreserveNodePorts {
		t.Error("PreserveNodePorts should default to false")
	}
}

func TestScheduleConfigDefaults(t *testing.T) {
	// Test empty config
	config := ScheduleConfig{}

	if config.Name != "" {
		t.Errorf("Name should be empty, got %q", config.Name)
	}
	if config.Schedule != "" {
		t.Errorf("Schedule should be empty, got %q", config.Schedule)
	}
	if config.SnapshotVolumes {
		t.Error("SnapshotVolumes should default to false")
	}
}

func TestBackupStatusConversion(t *testing.T) {
	// Test status conversion from string
	statusStr := "Completed"
	status := BackupStatus(statusStr)

	if status != StatusCompleted {
		t.Errorf("Status conversion failed: got %q, want %q", status, StatusCompleted)
	}
}

func TestRestoreStatusConversion(t *testing.T) {
	// Test status conversion from string
	statusStr := "InProgress"
	status := RestoreStatus(statusStr)

	if status != RestoreStatusInProgress {
		t.Errorf("Status conversion failed: got %q, want %q", status, RestoreStatusInProgress)
	}
}

func TestManagerKubeconfigPath(t *testing.T) {
	tests := []struct {
		name       string
		kubeconfig string
	}{
		{"default path", "/root/.kube/config"},
		{"custom path", "/custom/kubeconfig"},
		{"home path", "~/.kube/config"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager(tt.kubeconfig)
			if m.kubeconfig != tt.kubeconfig {
				t.Errorf("kubeconfig = %q, want %q", m.kubeconfig, tt.kubeconfig)
			}
		})
	}
}

func TestBackupLabels(t *testing.T) {
	backup := Backup{
		Name: "test-backup",
		Labels: map[string]string{
			"env":     "production",
			"team":    "platform",
			"version": "v1",
		},
	}

	if len(backup.Labels) != 3 {
		t.Errorf("Labels count = %d, want %d", len(backup.Labels), 3)
	}
	if backup.Labels["env"] != "production" {
		t.Errorf("Label env = %q, want %q", backup.Labels["env"], "production")
	}
}

func TestBackupDuration(t *testing.T) {
	start := time.Now()
	end := start.Add(10 * time.Minute)

	backup := Backup{
		Name:                "test-backup",
		StartTimestamp:      start,
		CompletionTimestamp: end,
	}

	duration := backup.CompletionTimestamp.Sub(backup.StartTimestamp)
	if duration != 10*time.Minute {
		t.Errorf("Duration = %v, want %v", duration, 10*time.Minute)
	}
}

func TestScheduleCronExpressions(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		desc       string
	}{
		{"daily", "0 0 * * *", "Every day at midnight"},
		{"hourly", "0 * * * *", "Every hour"},
		{"weekly", "0 0 * * 0", "Every Sunday at midnight"},
		{"monthly", "0 0 1 * *", "First of every month"},
		{"every-6-hours", "0 */6 * * *", "Every 6 hours"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := ScheduleConfig{
				Name:     tt.name + "-backup",
				Schedule: tt.expression,
			}
			if config.Schedule != tt.expression {
				t.Errorf("Schedule = %q, want %q", config.Schedule, tt.expression)
			}
		})
	}
}
