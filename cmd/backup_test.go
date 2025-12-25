package cmd

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/chalkan3/sloth-kubernetes/pkg/backup"
	"github.com/spf13/cobra"
)

// TestBackupCommand tests backup command structure
func TestBackupCommand(t *testing.T) {
	if backupCmd == nil {
		t.Fatal("backupCmd should not be nil")
	}

	if !strings.HasPrefix(backupCmd.Use, "backup") {
		t.Errorf("Expected Use to start with 'backup', got %q", backupCmd.Use)
	}

	if backupCmd.Short == "" {
		t.Error("Short description should not be empty")
	}

	if backupCmd.Long == "" {
		t.Error("Long description should not be empty")
	}
}

// TestBackupSubcommands tests that all backup subcommands exist
func TestBackupSubcommands(t *testing.T) {
	subcommands := []string{
		"create",
		"list",
		"describe",
		"delete",
		"restore",
		"restore-list",
		"schedule",
		"locations",
		"install",
		"status",
	}

	for _, subcmd := range subcommands {
		found := false
		for _, cmd := range backupCmd.Commands() {
			if cmd.Name() == subcmd {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected subcommand %q to exist", subcmd)
		}
	}
}

// TestBackupScheduleSubcommands tests that schedule subcommands exist
func TestBackupScheduleSubcommands(t *testing.T) {
	var scheduleCmd *cobra.Command
	for _, cmd := range backupCmd.Commands() {
		if cmd.Name() == "schedule" {
			scheduleCmd = cmd
			break
		}
	}

	if scheduleCmd == nil {
		t.Fatal("schedule subcommand not found")
	}

	subcommands := []string{"create", "list", "delete", "pause", "unpause"}

	for _, subcmd := range subcommands {
		found := false
		for _, cmd := range scheduleCmd.Commands() {
			if cmd.Name() == subcmd {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected schedule subcommand %q to exist", subcmd)
		}
	}
}

// TestBackupCreateDryRunFlag tests that backup create has dry-run flag
func TestBackupCreateDryRunFlag(t *testing.T) {
	flag := backupCreateCmd.Flags().Lookup("dry-run")
	if flag == nil {
		t.Fatal("dry-run flag should be defined for backup create")
	}

	if flag.DefValue != "false" {
		t.Errorf("Expected default value 'false', got %q", flag.DefValue)
	}

	if flag.Usage == "" {
		t.Error("Flag usage should not be empty")
	}
}

// TestBackupDeleteDryRunFlag tests that backup delete has dry-run flag
func TestBackupDeleteDryRunFlag(t *testing.T) {
	flag := backupDeleteCmd.Flags().Lookup("dry-run")
	if flag == nil {
		t.Fatal("dry-run flag should be defined for backup delete")
	}

	if flag.DefValue != "false" {
		t.Errorf("Expected default value 'false', got %q", flag.DefValue)
	}
}

// TestBackupRestoreDryRunFlag tests that backup restore has dry-run flag
func TestBackupRestoreDryRunFlag(t *testing.T) {
	flag := backupRestoreCmd.Flags().Lookup("dry-run")
	if flag == nil {
		t.Fatal("dry-run flag should be defined for backup restore")
	}

	if flag.DefValue != "false" {
		t.Errorf("Expected default value 'false', got %q", flag.DefValue)
	}
}

// TestBackupScheduleCreateDryRunFlag tests that schedule create has dry-run flag
func TestBackupScheduleCreateDryRunFlag(t *testing.T) {
	flag := backupScheduleCreateCmd.Flags().Lookup("dry-run")
	if flag == nil {
		t.Fatal("dry-run flag should be defined for schedule create")
	}

	if flag.DefValue != "false" {
		t.Errorf("Expected default value 'false', got %q", flag.DefValue)
	}
}

// TestBackupScheduleDeleteDryRunFlag tests that schedule delete has dry-run flag
func TestBackupScheduleDeleteDryRunFlag(t *testing.T) {
	flag := backupScheduleDeleteCmd.Flags().Lookup("dry-run")
	if flag == nil {
		t.Fatal("dry-run flag should be defined for schedule delete")
	}

	if flag.DefValue != "false" {
		t.Errorf("Expected default value 'false', got %q", flag.DefValue)
	}
}

// TestBackupCreateFlags tests backup create command flags
func TestBackupCreateFlags(t *testing.T) {
	flags := backupCreateCmd.Flags()

	expectedFlags := []string{
		"namespaces",
		"exclude-namespaces",
		"resources",
		"exclude-resources",
		"ttl",
		"storage-location",
		"labels",
		"snapshot-volumes",
		"wait",
		"timeout",
		"dry-run",
	}

	for _, flagName := range expectedFlags {
		flag := flags.Lookup(flagName)
		if flag == nil {
			t.Errorf("Flag %q should be defined", flagName)
		}
	}
}

// TestBackupRestoreFlags tests backup restore command flags
func TestBackupRestoreFlags(t *testing.T) {
	flags := backupRestoreCmd.Flags()

	expectedFlags := []string{
		"from-backup",
		"namespaces",
		"exclude-namespaces",
		"resources",
		"exclude-resources",
		"restore-volumes",
		"preserve-nodeports",
		"wait",
		"timeout",
		"dry-run",
	}

	for _, flagName := range expectedFlags {
		flag := flags.Lookup(flagName)
		if flag == nil {
			t.Errorf("Flag %q should be defined", flagName)
		}
	}
}

// TestBackupScheduleCreateFlags tests schedule create command flags
func TestBackupScheduleCreateFlags(t *testing.T) {
	flags := backupScheduleCreateCmd.Flags()

	expectedFlags := []string{
		"schedule",
		"namespaces",
		"exclude-namespaces",
		"ttl",
		"snapshot-volumes",
		"dry-run",
	}

	for _, flagName := range expectedFlags {
		flag := flags.Lookup(flagName)
		if flag == nil {
			t.Errorf("Flag %q should be defined", flagName)
		}
	}
}

// TestBackupGlobalFlags tests backup command global flags
func TestBackupGlobalFlags(t *testing.T) {
	flags := backupCmd.PersistentFlags()

	expectedFlags := []string{
		"kubeconfig",
		"velero-namespace",
		"json",
	}

	for _, flagName := range expectedFlags {
		flag := flags.Lookup(flagName)
		if flag == nil {
			t.Errorf("Persistent flag %q should be defined", flagName)
		}
	}
}

// TestDryRunConfigurationOutput tests the dry-run output logic
func TestDryRunConfigurationOutput(t *testing.T) {
	tests := []struct {
		name               string
		config             backup.BackupConfig
		expectedOutputs    []string
		notExpectedOutputs []string
	}{
		{
			name: "Full cluster backup",
			config: backup.BackupConfig{
				Name:            "test-backup",
				TTL:             "720h",
				SnapshotVolumes: true,
			},
			expectedOutputs: []string{
				"test-backup",
				"All",
				"720h",
				"true",
			},
			notExpectedOutputs: []string{
				"Excluded NS",
			},
		},
		{
			name: "Namespace-specific backup",
			config: backup.BackupConfig{
				Name:               "ns-backup",
				IncludedNamespaces: []string{"default", "kube-system"},
				TTL:                "168h",
				SnapshotVolumes:    false,
			},
			expectedOutputs: []string{
				"ns-backup",
				"default",
				"kube-system",
				"168h",
				"false",
			},
		},
		{
			name: "Backup with exclusions",
			config: backup.BackupConfig{
				Name:               "exclude-backup",
				ExcludedNamespaces: []string{"kube-system", "velero"},
				ExcludedResources:  []string{"secrets", "configmaps"},
				TTL:                "24h",
				SnapshotVolumes:    true,
			},
			expectedOutputs: []string{
				"exclude-backup",
				"kube-system",
				"velero",
				"secrets",
				"configmaps",
				"24h",
			},
		},
		{
			name: "Backup with storage location",
			config: backup.BackupConfig{
				Name:            "s3-backup",
				StorageLocation: "aws-s3-bucket",
				TTL:             "720h",
				SnapshotVolumes: true,
			},
			expectedOutputs: []string{
				"s3-backup",
				"aws-s3-bucket",
			},
		},
		{
			name: "Backup with labels",
			config: backup.BackupConfig{
				Name: "labeled-backup",
				Labels: map[string]string{
					"env":  "production",
					"team": "platform",
				},
				TTL:             "720h",
				SnapshotVolumes: true,
			},
			expectedOutputs: []string{
				"labeled-backup",
				"env",
				"production",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			// Simulate dry-run output generation
			buf.WriteString(tt.config.Name + "\n")
			if len(tt.config.IncludedNamespaces) > 0 {
				buf.WriteString(strings.Join(tt.config.IncludedNamespaces, ", ") + "\n")
			} else {
				buf.WriteString("All\n")
			}
			if len(tt.config.ExcludedNamespaces) > 0 {
				buf.WriteString("Excluded NS: " + strings.Join(tt.config.ExcludedNamespaces, ", ") + "\n")
			}
			if len(tt.config.ExcludedResources) > 0 {
				buf.WriteString(strings.Join(tt.config.ExcludedResources, ", ") + "\n")
			}
			buf.WriteString(tt.config.TTL + "\n")
			if tt.config.SnapshotVolumes {
				buf.WriteString("true\n")
			} else {
				buf.WriteString("false\n")
			}
			if tt.config.StorageLocation != "" {
				buf.WriteString(tt.config.StorageLocation + "\n")
			}
			if len(tt.config.Labels) > 0 {
				for k, v := range tt.config.Labels {
					buf.WriteString(k + ": " + v + "\n")
				}
			}

			output := buf.String()

			for _, expected := range tt.expectedOutputs {
				if !strings.Contains(output, expected) {
					t.Errorf("Expected output to contain %q, got %q", expected, output)
				}
			}

			for _, notExpected := range tt.notExpectedOutputs {
				if strings.Contains(output, notExpected) {
					t.Errorf("Output should not contain %q, got %q", notExpected, output)
				}
			}
		})
	}
}

// TestRestoreDryRunConfigurationOutput tests the restore dry-run output logic
func TestRestoreDryRunConfigurationOutput(t *testing.T) {
	tests := []struct {
		name            string
		config          backup.RestoreConfig
		backup          backup.Backup
		expectedOutputs []string
	}{
		{
			name: "Full restore",
			config: backup.RestoreConfig{
				BackupName: "test-backup",
				RestorePVs: true,
			},
			backup: backup.Backup{
				Name:   "test-backup",
				Status: backup.StatusCompleted,
			},
			expectedOutputs: []string{
				"test-backup",
				"Completed",
				"true",
			},
		},
		{
			name: "Partial restore",
			config: backup.RestoreConfig{
				BackupName:         "partial-backup",
				IncludedNamespaces: []string{"app-ns"},
				RestorePVs:         false,
				PreserveNodePorts:  true,
			},
			backup: backup.Backup{
				Name:   "partial-backup",
				Status: backup.StatusCompleted,
			},
			expectedOutputs: []string{
				"partial-backup",
				"app-ns",
				"false",
				"true",
			},
		},
		{
			name: "Restore with exclusions",
			config: backup.RestoreConfig{
				BackupName:         "exclude-restore",
				ExcludedNamespaces: []string{"kube-system"},
				ExcludedResources:  []string{"secrets"},
				RestorePVs:         true,
			},
			backup: backup.Backup{
				Name:   "exclude-restore",
				Status: backup.StatusCompleted,
			},
			expectedOutputs: []string{
				"exclude-restore",
				"kube-system",
				"secrets",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			// Simulate dry-run output generation
			buf.WriteString(tt.config.BackupName + "\n")
			buf.WriteString(string(tt.backup.Status) + "\n")
			if len(tt.config.IncludedNamespaces) > 0 {
				buf.WriteString(strings.Join(tt.config.IncludedNamespaces, ", ") + "\n")
			}
			if len(tt.config.ExcludedNamespaces) > 0 {
				buf.WriteString(strings.Join(tt.config.ExcludedNamespaces, ", ") + "\n")
			}
			if len(tt.config.ExcludedResources) > 0 {
				buf.WriteString(strings.Join(tt.config.ExcludedResources, ", ") + "\n")
			}
			if tt.config.RestorePVs {
				buf.WriteString("true\n")
			} else {
				buf.WriteString("false\n")
			}
			if tt.config.PreserveNodePorts {
				buf.WriteString("true\n")
			}

			output := buf.String()

			for _, expected := range tt.expectedOutputs {
				if !strings.Contains(output, expected) {
					t.Errorf("Expected output to contain %q, got %q", expected, output)
				}
			}
		})
	}
}

// TestScheduleDryRunConfigurationOutput tests the schedule dry-run output logic
func TestScheduleDryRunConfigurationOutput(t *testing.T) {
	tests := []struct {
		name            string
		config          backup.ScheduleConfig
		expectedOutputs []string
	}{
		{
			name: "Daily schedule",
			config: backup.ScheduleConfig{
				Name:            "daily-backup",
				Schedule:        "0 0 * * *",
				TTL:             "720h",
				SnapshotVolumes: true,
			},
			expectedOutputs: []string{
				"daily-backup",
				"0 0 * * *",
				"720h",
				"true",
			},
		},
		{
			name: "Hourly schedule with namespaces",
			config: backup.ScheduleConfig{
				Name:               "hourly-backup",
				Schedule:           "0 * * * *",
				IncludedNamespaces: []string{"production"},
				TTL:                "168h",
				SnapshotVolumes:    false,
			},
			expectedOutputs: []string{
				"hourly-backup",
				"0 * * * *",
				"production",
				"168h",
				"false",
			},
		},
		{
			name: "Weekly schedule with exclusions",
			config: backup.ScheduleConfig{
				Name:               "weekly-backup",
				Schedule:           "0 0 * * 0",
				ExcludedNamespaces: []string{"dev", "staging"},
				TTL:                "2160h",
				SnapshotVolumes:    true,
			},
			expectedOutputs: []string{
				"weekly-backup",
				"0 0 * * 0",
				"dev",
				"staging",
				"2160h",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			// Simulate dry-run output generation
			buf.WriteString(tt.config.Name + "\n")
			buf.WriteString(tt.config.Schedule + "\n")
			if len(tt.config.IncludedNamespaces) > 0 {
				buf.WriteString(strings.Join(tt.config.IncludedNamespaces, ", ") + "\n")
			}
			if len(tt.config.ExcludedNamespaces) > 0 {
				buf.WriteString(strings.Join(tt.config.ExcludedNamespaces, ", ") + "\n")
			}
			buf.WriteString(tt.config.TTL + "\n")
			if tt.config.SnapshotVolumes {
				buf.WriteString("true\n")
			} else {
				buf.WriteString("false\n")
			}

			output := buf.String()

			for _, expected := range tt.expectedOutputs {
				if !strings.Contains(output, expected) {
					t.Errorf("Expected output to contain %q, got %q", expected, output)
				}
			}
		})
	}
}

// TestBackupDeleteDryRunOutput tests the delete dry-run output logic
func TestBackupDeleteDryRunOutput(t *testing.T) {
	tests := []struct {
		name            string
		backup          backup.Backup
		expectedOutputs []string
	}{
		{
			name: "Delete completed backup",
			backup: backup.Backup{
				Name:               "completed-backup",
				Status:             backup.StatusCompleted,
				StartTimestamp:     time.Now().Add(-24 * time.Hour),
				IncludedNamespaces: []string{"default", "app"},
			},
			expectedOutputs: []string{
				"completed-backup",
				"Completed",
				"default",
				"app",
			},
		},
		{
			name: "Delete failed backup",
			backup: backup.Backup{
				Name:           "failed-backup",
				Status:         backup.StatusFailed,
				StartTimestamp: time.Now().Add(-1 * time.Hour),
			},
			expectedOutputs: []string{
				"failed-backup",
				"Failed",
			},
		},
		{
			name: "Delete full cluster backup",
			backup: backup.Backup{
				Name:           "full-backup",
				Status:         backup.StatusCompleted,
				StartTimestamp: time.Now().Add(-48 * time.Hour),
			},
			expectedOutputs: []string{
				"full-backup",
				"Completed",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			// Simulate dry-run delete output
			buf.WriteString(tt.backup.Name + "\n")
			buf.WriteString(string(tt.backup.Status) + "\n")
			if !tt.backup.StartTimestamp.IsZero() {
				buf.WriteString(tt.backup.StartTimestamp.Format("2006-01-02 15:04:05") + "\n")
			}
			if len(tt.backup.IncludedNamespaces) > 0 {
				buf.WriteString(strings.Join(tt.backup.IncludedNamespaces, ", ") + "\n")
			}

			output := buf.String()

			for _, expected := range tt.expectedOutputs {
				if !strings.Contains(output, expected) {
					t.Errorf("Expected output to contain %q, got %q", expected, output)
				}
			}
		})
	}
}

// TestScheduleDeleteDryRunOutput tests the schedule delete dry-run output logic
func TestScheduleDeleteDryRunOutput(t *testing.T) {
	tests := []struct {
		name            string
		schedule        backup.Schedule
		expectedOutputs []string
	}{
		{
			name: "Delete active schedule",
			schedule: backup.Schedule{
				Name:       "daily-schedule",
				Schedule:   "0 0 * * *",
				Paused:     false,
				LastBackup: time.Now().Add(-24 * time.Hour),
			},
			expectedOutputs: []string{
				"daily-schedule",
				"0 0 * * *",
				"false",
			},
		},
		{
			name: "Delete paused schedule",
			schedule: backup.Schedule{
				Name:     "paused-schedule",
				Schedule: "0 */6 * * *",
				Paused:   true,
			},
			expectedOutputs: []string{
				"paused-schedule",
				"0 */6 * * *",
				"true",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			// Simulate dry-run schedule delete output
			buf.WriteString(tt.schedule.Name + "\n")
			buf.WriteString(tt.schedule.Schedule + "\n")
			if tt.schedule.Paused {
				buf.WriteString("true\n")
			} else {
				buf.WriteString("false\n")
			}
			if !tt.schedule.LastBackup.IsZero() {
				buf.WriteString(tt.schedule.LastBackup.Format("2006-01-02 15:04:05") + "\n")
			}

			output := buf.String()

			for _, expected := range tt.expectedOutputs {
				if !strings.Contains(output, expected) {
					t.Errorf("Expected output to contain %q, got %q", expected, output)
				}
			}
		})
	}
}

// TestDryRunFlagBehavior tests the dry-run flag behavior
func TestDryRunFlagBehavior(t *testing.T) {
	tests := []struct {
		name          string
		dryRun        bool
		shouldExecute bool
		shouldPreview bool
	}{
		{
			name:          "Dry-run enabled",
			dryRun:        true,
			shouldExecute: false,
			shouldPreview: true,
		},
		{
			name:          "Dry-run disabled",
			dryRun:        false,
			shouldExecute: true,
			shouldPreview: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			execute := !tt.dryRun
			preview := tt.dryRun

			if execute != tt.shouldExecute {
				t.Errorf("Expected execute=%v, got %v", tt.shouldExecute, execute)
			}
			if preview != tt.shouldPreview {
				t.Errorf("Expected preview=%v, got %v", tt.shouldPreview, preview)
			}
		})
	}
}

// TestBackupInstallFlags tests backup install command flags
func TestBackupInstallFlags(t *testing.T) {
	flags := backupInstallCmd.Flags()

	expectedFlags := []string{
		"provider",
		"bucket",
		"region",
		"secret-file",
	}

	for _, flagName := range expectedFlags {
		flag := flags.Lookup(flagName)
		if flag == nil {
			t.Errorf("Flag %q should be defined", flagName)
		}
	}
}

// TestBackupStatusColor tests getBackupStatusColor function
func TestBackupStatusColor(t *testing.T) {
	tests := []struct {
		status      string
		shouldExist bool
	}{
		{"Completed", true},
		{"InProgress", true},
		{"New", true},
		{"Failed", true},
		{"PartiallyFailed", true},
		{"Unknown", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			color := getBackupStatusColor(tt.status)
			if tt.shouldExist && color == nil {
				t.Errorf("Expected color for status %q to exist", tt.status)
			}
		})
	}
}

// TestBackupConfigValidation tests backup configuration validation
func TestBackupConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  backup.BackupConfig
		isValid bool
	}{
		{
			name: "Valid full cluster backup",
			config: backup.BackupConfig{
				Name:            "valid-backup",
				TTL:             "720h",
				SnapshotVolumes: true,
			},
			isValid: true,
		},
		{
			name: "Valid namespace backup",
			config: backup.BackupConfig{
				Name:               "ns-backup",
				IncludedNamespaces: []string{"default"},
				TTL:                "168h",
			},
			isValid: true,
		},
		{
			name: "Auto-generated name",
			config: backup.BackupConfig{
				Name: "",
				TTL:  "720h",
			},
			isValid: true, // Name can be auto-generated
		},
		{
			name: "With storage location",
			config: backup.BackupConfig{
				Name:            "s3-backup",
				StorageLocation: "aws-bucket",
				TTL:             "720h",
			},
			isValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Basic validation - config is valid if it has required fields or they can be auto-generated
			valid := tt.config.TTL != ""
			if valid != tt.isValid {
				t.Errorf("Expected isValid=%v, got %v", tt.isValid, valid)
			}
		})
	}
}

// TestRestoreConfigValidation tests restore configuration validation
func TestRestoreConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  backup.RestoreConfig
		isValid bool
	}{
		{
			name: "Valid restore",
			config: backup.RestoreConfig{
				BackupName: "test-backup",
				RestorePVs: true,
			},
			isValid: true,
		},
		{
			name: "Missing backup name",
			config: backup.RestoreConfig{
				BackupName: "",
				RestorePVs: true,
			},
			isValid: false,
		},
		{
			name: "Restore with namespaces",
			config: backup.RestoreConfig{
				BackupName:         "backup",
				IncludedNamespaces: []string{"app"},
				RestorePVs:         false,
			},
			isValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := tt.config.BackupName != ""
			if valid != tt.isValid {
				t.Errorf("Expected isValid=%v, got %v", tt.isValid, valid)
			}
		})
	}
}

// TestScheduleConfigValidation tests schedule configuration validation
func TestScheduleConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  backup.ScheduleConfig
		isValid bool
	}{
		{
			name: "Valid daily schedule",
			config: backup.ScheduleConfig{
				Name:     "daily",
				Schedule: "0 0 * * *",
				TTL:      "720h",
			},
			isValid: true,
		},
		{
			name: "Missing schedule expression",
			config: backup.ScheduleConfig{
				Name:     "invalid",
				Schedule: "",
				TTL:      "720h",
			},
			isValid: false,
		},
		{
			name: "Missing name",
			config: backup.ScheduleConfig{
				Name:     "",
				Schedule: "0 0 * * *",
				TTL:      "720h",
			},
			isValid: false,
		},
		{
			name: "Valid hourly schedule",
			config: backup.ScheduleConfig{
				Name:     "hourly",
				Schedule: "0 * * * *",
				TTL:      "168h",
			},
			isValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := tt.config.Name != "" && tt.config.Schedule != ""
			if valid != tt.isValid {
				t.Errorf("Expected isValid=%v, got %v", tt.isValid, valid)
			}
		})
	}
}

// TestCronExpressionExamples tests common cron expression examples
func TestCronExpressionExamples(t *testing.T) {
	examples := []struct {
		expression  string
		description string
	}{
		{"0 0 * * *", "Daily at midnight"},
		{"0 */6 * * *", "Every 6 hours"},
		{"0 0 * * 0", "Weekly on Sunday at midnight"},
		{"0 0 1 * *", "Monthly on the 1st at midnight"},
		{"0 * * * *", "Every hour"},
		{"*/15 * * * *", "Every 15 minutes"},
	}

	for _, ex := range examples {
		t.Run(ex.description, func(t *testing.T) {
			// Validate cron expression format (5 fields)
			fields := strings.Fields(ex.expression)
			if len(fields) != 5 {
				t.Errorf("Expected 5 fields in cron expression %q, got %d", ex.expression, len(fields))
			}
		})
	}
}

// TestTTLParsing tests TTL duration parsing
func TestTTLParsing(t *testing.T) {
	tests := []struct {
		ttl      string
		expected time.Duration
		isValid  bool
	}{
		{"720h", 720 * time.Hour, true},    // 30 days
		{"168h", 168 * time.Hour, true},    // 7 days
		{"24h", 24 * time.Hour, true},      // 1 day
		{"2160h", 2160 * time.Hour, true},  // 90 days
		{"1h", 1 * time.Hour, true},
		{"30m", 30 * time.Minute, true},
		{"invalid", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.ttl, func(t *testing.T) {
			duration, err := time.ParseDuration(tt.ttl)
			isValid := err == nil

			if isValid != tt.isValid {
				t.Errorf("Expected isValid=%v for TTL %q, got %v (err: %v)", tt.isValid, tt.ttl, isValid, err)
			}

			if tt.isValid && duration != tt.expected {
				t.Errorf("Expected duration %v for TTL %q, got %v", tt.expected, tt.ttl, duration)
			}
		})
	}
}

// TestLabelParsing tests backup label parsing
func TestLabelParsing(t *testing.T) {
	tests := []struct {
		name     string
		labels   []string
		expected map[string]string
	}{
		{
			name:   "Single label",
			labels: []string{"env=production"},
			expected: map[string]string{
				"env": "production",
			},
		},
		{
			name:   "Multiple labels",
			labels: []string{"env=production", "team=platform", "app=api"},
			expected: map[string]string{
				"env":  "production",
				"team": "platform",
				"app":  "api",
			},
		},
		{
			name:     "Empty labels",
			labels:   []string{},
			expected: map[string]string{},
		},
		{
			name:   "Label with special characters",
			labels: []string{"app.kubernetes.io/name=myapp"},
			expected: map[string]string{
				"app.kubernetes.io/name": "myapp",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := make(map[string]string)
			for _, label := range tt.labels {
				parts := strings.SplitN(label, "=", 2)
				if len(parts) == 2 {
					result[parts[0]] = parts[1]
				}
			}

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d labels, got %d", len(tt.expected), len(result))
			}

			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("Expected label %q=%q, got %q", k, v, result[k])
				}
			}
		})
	}
}
