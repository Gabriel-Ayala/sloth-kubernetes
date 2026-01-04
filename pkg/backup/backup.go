// Package backup provides Kubernetes cluster backup and restore functionality using Velero
package backup

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/chalkan3/sloth-kubernetes/pkg/retry"
)

// BackupStatus represents the status of a backup
type BackupStatus string

const (
	StatusNew             BackupStatus = "New"
	StatusInProgress      BackupStatus = "InProgress"
	StatusCompleted       BackupStatus = "Completed"
	StatusFailed          BackupStatus = "Failed"
	StatusPartiallyFailed BackupStatus = "PartiallyFailed"
	StatusDeleting        BackupStatus = "Deleting"
)

// RestoreStatus represents the status of a restore
type RestoreStatus string

const (
	RestoreStatusNew             RestoreStatus = "New"
	RestoreStatusInProgress      RestoreStatus = "InProgress"
	RestoreStatusCompleted       RestoreStatus = "Completed"
	RestoreStatusFailed          RestoreStatus = "Failed"
	RestoreStatusPartiallyFailed RestoreStatus = "PartiallyFailed"
)

// Backup represents a Velero backup
type Backup struct {
	Name                string            `json:"name"`
	Namespace           string            `json:"namespace"`
	Status              BackupStatus      `json:"status"`
	Phase               string            `json:"phase"`
	IncludedNamespaces  []string          `json:"includedNamespaces"`
	ExcludedNamespaces  []string          `json:"excludedNamespaces"`
	IncludedResources   []string          `json:"includedResources"`
	ExcludedResources   []string          `json:"excludedResources"`
	Labels              map[string]string `json:"labels"`
	StorageLocation     string            `json:"storageLocation"`
	TTL                 string            `json:"ttl"`
	StartTimestamp      time.Time         `json:"startTimestamp"`
	CompletionTimestamp time.Time         `json:"completionTimestamp"`
	Expiration          time.Time         `json:"expiration"`
	TotalItems          int               `json:"totalItems"`
	ItemsBackedUp       int               `json:"itemsBackedUp"`
	Errors              int               `json:"errors"`
	Warnings            int               `json:"warnings"`
}

// Restore represents a Velero restore
type Restore struct {
	Name                string        `json:"name"`
	Namespace           string        `json:"namespace"`
	BackupName          string        `json:"backupName"`
	Status              RestoreStatus `json:"status"`
	Phase               string        `json:"phase"`
	IncludedNamespaces  []string      `json:"includedNamespaces"`
	ExcludedNamespaces  []string      `json:"excludedNamespaces"`
	IncludedResources   []string      `json:"includedResources"`
	ExcludedResources   []string      `json:"excludedResources"`
	RestorePVs          bool          `json:"restorePVs"`
	StartTimestamp      time.Time     `json:"startTimestamp"`
	CompletionTimestamp time.Time     `json:"completionTimestamp"`
	Errors              int           `json:"errors"`
	Warnings            int           `json:"warnings"`
}

// Schedule represents a Velero backup schedule
type Schedule struct {
	Name               string    `json:"name"`
	Namespace          string    `json:"namespace"`
	Schedule           string    `json:"schedule"`
	IncludedNamespaces []string  `json:"includedNamespaces"`
	ExcludedNamespaces []string  `json:"excludedNamespaces"`
	TTL                string    `json:"ttl"`
	LastBackup         time.Time `json:"lastBackup"`
	Paused             bool      `json:"paused"`
}

// BackupLocation represents a backup storage location
type BackupLocation struct {
	Name       string `json:"name"`
	Provider   string `json:"provider"`
	Bucket     string `json:"bucket"`
	Region     string `json:"region"`
	Default    bool   `json:"default"`
	AccessMode string `json:"accessMode"`
}

// BackupConfig holds configuration for creating a backup
type BackupConfig struct {
	Name               string
	IncludedNamespaces []string
	ExcludedNamespaces []string
	IncludedResources  []string
	ExcludedResources  []string
	Labels             map[string]string
	StorageLocation    string
	TTL                string
	SnapshotVolumes    bool
	Wait               bool
	Timeout            time.Duration
}

// RestoreConfig holds configuration for restoring a backup
type RestoreConfig struct {
	Name               string
	BackupName         string
	IncludedNamespaces []string
	ExcludedNamespaces []string
	IncludedResources  []string
	ExcludedResources  []string
	RestorePVs         bool
	PreserveNodePorts  bool
	Wait               bool
	Timeout            time.Duration
}

// ScheduleConfig holds configuration for creating a schedule
type ScheduleConfig struct {
	Name               string
	Schedule           string // Cron expression
	IncludedNamespaces []string
	ExcludedNamespaces []string
	TTL                string
	SnapshotVolumes    bool
}

// Manager handles backup operations
type Manager struct {
	kubeconfig      string
	namespace       string
	veleroInstalled bool
	verbose         bool
}

// NewManager creates a new backup manager
func NewManager(kubeconfig string) *Manager {
	return &Manager{
		kubeconfig: kubeconfig,
		namespace:  "velero",
	}
}

// SetNamespace sets the Velero namespace
func (m *Manager) SetNamespace(ns string) {
	m.namespace = ns
}

// SetVerbose enables verbose output
func (m *Manager) SetVerbose(v bool) {
	m.verbose = v
}

// CheckVeleroInstalled checks if Velero is installed
func (m *Manager) CheckVeleroInstalled() (bool, error) {
	output, err := m.runKubectl("get deployment velero -n " + m.namespace + " -o name")
	if err != nil {
		return false, nil
	}
	return strings.Contains(output, "velero"), nil
}

// InstallVelero installs Velero with the specified configuration
func (m *Manager) InstallVelero(provider, bucket, region, secretFile string) error {
	// Check if already installed
	installed, _ := m.CheckVeleroInstalled()
	if installed {
		return fmt.Errorf("velero is already installed")
	}

	// Build velero install command
	args := []string{
		"install",
		"--provider", provider,
		"--bucket", bucket,
		"--secret-file", secretFile,
		"--backup-location-config", fmt.Sprintf("region=%s", region),
		"--snapshot-location-config", fmt.Sprintf("region=%s", region),
		"--use-volume-snapshots=true",
		"--use-restic",
	}

	if m.kubeconfig != "" {
		args = append(args, "--kubeconfig", m.kubeconfig)
	}

	cmd := exec.Command("velero", args...)
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install velero: %w\nOutput: %s", err, output)
	}

	return nil
}

// CreateBackup creates a new backup
func (m *Manager) CreateBackup(config BackupConfig) (*Backup, error) {
	if config.Name == "" {
		config.Name = fmt.Sprintf("backup-%s", time.Now().Format("20060102-150405"))
	}

	args := []string{"backup", "create", config.Name}

	if len(config.IncludedNamespaces) > 0 {
		args = append(args, "--include-namespaces", strings.Join(config.IncludedNamespaces, ","))
	}
	if len(config.ExcludedNamespaces) > 0 {
		args = append(args, "--exclude-namespaces", strings.Join(config.ExcludedNamespaces, ","))
	}
	if len(config.IncludedResources) > 0 {
		args = append(args, "--include-resources", strings.Join(config.IncludedResources, ","))
	}
	if len(config.ExcludedResources) > 0 {
		args = append(args, "--exclude-resources", strings.Join(config.ExcludedResources, ","))
	}
	if config.StorageLocation != "" {
		args = append(args, "--storage-location", config.StorageLocation)
	}
	if config.TTL != "" {
		args = append(args, "--ttl", config.TTL)
	}
	if config.SnapshotVolumes {
		args = append(args, "--snapshot-volumes")
	}
	if config.Wait {
		args = append(args, "--wait")
	}

	for k, v := range config.Labels {
		args = append(args, "--labels", fmt.Sprintf("%s=%s", k, v))
	}

	output, err := m.runVelero(args...)
	if err != nil {
		return nil, fmt.Errorf("failed to create backup: %w\nOutput: %s", err, output)
	}

	// Get backup details
	return m.GetBackup(config.Name)
}

// GetBackup retrieves backup details
func (m *Manager) GetBackup(name string) (*Backup, error) {
	output, err := m.runVelero("backup", "describe", name, "-o", "json")
	if err != nil {
		return nil, fmt.Errorf("failed to get backup: %w", err)
	}

	var backup Backup
	if err := json.Unmarshal([]byte(output), &backup); err != nil {
		// Parse from velero describe output
		backup = m.parseBackupFromDescribe(name, output)
	}

	return &backup, nil
}

// ListBackups lists all backups
func (m *Manager) ListBackups() ([]Backup, error) {
	output, err := m.runVelero("backup", "get", "-o", "json")
	if err != nil {
		// If velero command fails, try kubectl
		output, err = m.runKubectl("get backups.velero.io -n " + m.namespace + " -o json")
		if err != nil {
			return nil, fmt.Errorf("failed to list backups: %w", err)
		}
	}

	var backups []Backup
	// Try to parse as JSON list
	var result struct {
		Items []struct {
			Metadata struct {
				Name              string    `json:"name"`
				Namespace         string    `json:"namespace"`
				CreationTimestamp time.Time `json:"creationTimestamp"`
			} `json:"metadata"`
			Spec struct {
				IncludedNamespaces []string `json:"includedNamespaces"`
				ExcludedNamespaces []string `json:"excludedNamespaces"`
				TTL                string   `json:"ttl"`
				StorageLocation    string   `json:"storageLocation"`
			} `json:"spec"`
			Status struct {
				Phase               string    `json:"phase"`
				StartTimestamp      time.Time `json:"startTimestamp"`
				CompletionTimestamp time.Time `json:"completionTimestamp"`
				Expiration          time.Time `json:"expiration"`
				Errors              int       `json:"errors"`
				Warnings            int       `json:"warnings"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal([]byte(output), &result); err == nil {
		for _, item := range result.Items {
			backups = append(backups, Backup{
				Name:                item.Metadata.Name,
				Namespace:           item.Metadata.Namespace,
				Phase:               item.Status.Phase,
				Status:              BackupStatus(item.Status.Phase),
				IncludedNamespaces:  item.Spec.IncludedNamespaces,
				ExcludedNamespaces:  item.Spec.ExcludedNamespaces,
				StorageLocation:     item.Spec.StorageLocation,
				TTL:                 item.Spec.TTL,
				StartTimestamp:      item.Status.StartTimestamp,
				CompletionTimestamp: item.Status.CompletionTimestamp,
				Expiration:          item.Status.Expiration,
				Errors:              item.Status.Errors,
				Warnings:            item.Status.Warnings,
			})
		}
	}

	return backups, nil
}

// DeleteBackup deletes a backup
func (m *Manager) DeleteBackup(name string) error {
	_, err := m.runVelero("backup", "delete", name, "--confirm")
	if err != nil {
		return fmt.Errorf("failed to delete backup: %w", err)
	}
	return nil
}

// CreateRestore creates a restore from a backup
func (m *Manager) CreateRestore(config RestoreConfig) (*Restore, error) {
	if config.Name == "" {
		config.Name = fmt.Sprintf("restore-%s-%s", config.BackupName, time.Now().Format("20060102-150405"))
	}

	args := []string{"restore", "create", config.Name, "--from-backup", config.BackupName}

	if len(config.IncludedNamespaces) > 0 {
		args = append(args, "--include-namespaces", strings.Join(config.IncludedNamespaces, ","))
	}
	if len(config.ExcludedNamespaces) > 0 {
		args = append(args, "--exclude-namespaces", strings.Join(config.ExcludedNamespaces, ","))
	}
	if len(config.IncludedResources) > 0 {
		args = append(args, "--include-resources", strings.Join(config.IncludedResources, ","))
	}
	if len(config.ExcludedResources) > 0 {
		args = append(args, "--exclude-resources", strings.Join(config.ExcludedResources, ","))
	}
	if config.RestorePVs {
		args = append(args, "--restore-volumes")
	}
	if config.PreserveNodePorts {
		args = append(args, "--preserve-nodeports")
	}
	if config.Wait {
		args = append(args, "--wait")
	}

	output, err := m.runVelero(args...)
	if err != nil {
		return nil, fmt.Errorf("failed to create restore: %w\nOutput: %s", err, output)
	}

	return m.GetRestore(config.Name)
}

// GetRestore retrieves restore details
func (m *Manager) GetRestore(name string) (*Restore, error) {
	output, err := m.runVelero("restore", "describe", name, "-o", "json")
	if err != nil {
		return nil, fmt.Errorf("failed to get restore: %w", err)
	}

	var restore Restore
	if err := json.Unmarshal([]byte(output), &restore); err != nil {
		restore = m.parseRestoreFromDescribe(name, output)
	}

	return &restore, nil
}

// ListRestores lists all restores
func (m *Manager) ListRestores() ([]Restore, error) {
	output, err := m.runVelero("restore", "get", "-o", "json")
	if err != nil {
		output, err = m.runKubectl("get restores.velero.io -n " + m.namespace + " -o json")
		if err != nil {
			return nil, fmt.Errorf("failed to list restores: %w", err)
		}
	}

	var restores []Restore
	var result struct {
		Items []struct {
			Metadata struct {
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
			} `json:"metadata"`
			Spec struct {
				BackupName         string   `json:"backupName"`
				IncludedNamespaces []string `json:"includedNamespaces"`
				ExcludedNamespaces []string `json:"excludedNamespaces"`
				RestorePVs         bool     `json:"restorePVs"`
			} `json:"spec"`
			Status struct {
				Phase               string    `json:"phase"`
				StartTimestamp      time.Time `json:"startTimestamp"`
				CompletionTimestamp time.Time `json:"completionTimestamp"`
				Errors              int       `json:"errors"`
				Warnings            int       `json:"warnings"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal([]byte(output), &result); err == nil {
		for _, item := range result.Items {
			restores = append(restores, Restore{
				Name:                item.Metadata.Name,
				Namespace:           item.Metadata.Namespace,
				BackupName:          item.Spec.BackupName,
				Phase:               item.Status.Phase,
				Status:              RestoreStatus(item.Status.Phase),
				IncludedNamespaces:  item.Spec.IncludedNamespaces,
				ExcludedNamespaces:  item.Spec.ExcludedNamespaces,
				RestorePVs:          item.Spec.RestorePVs,
				StartTimestamp:      item.Status.StartTimestamp,
				CompletionTimestamp: item.Status.CompletionTimestamp,
				Errors:              item.Status.Errors,
				Warnings:            item.Status.Warnings,
			})
		}
	}

	return restores, nil
}

// DeleteRestore deletes a restore
func (m *Manager) DeleteRestore(name string) error {
	_, err := m.runVelero("restore", "delete", name, "--confirm")
	if err != nil {
		return fmt.Errorf("failed to delete restore: %w", err)
	}
	return nil
}

// CreateSchedule creates a backup schedule
func (m *Manager) CreateSchedule(config ScheduleConfig) (*Schedule, error) {
	args := []string{"schedule", "create", config.Name, "--schedule", config.Schedule}

	if len(config.IncludedNamespaces) > 0 {
		args = append(args, "--include-namespaces", strings.Join(config.IncludedNamespaces, ","))
	}
	if len(config.ExcludedNamespaces) > 0 {
		args = append(args, "--exclude-namespaces", strings.Join(config.ExcludedNamespaces, ","))
	}
	if config.TTL != "" {
		args = append(args, "--ttl", config.TTL)
	}
	if config.SnapshotVolumes {
		args = append(args, "--snapshot-volumes")
	}

	output, err := m.runVelero(args...)
	if err != nil {
		return nil, fmt.Errorf("failed to create schedule: %w\nOutput: %s", err, output)
	}

	return m.GetSchedule(config.Name)
}

// GetSchedule retrieves schedule details
func (m *Manager) GetSchedule(name string) (*Schedule, error) {
	output, err := m.runVelero("schedule", "describe", name, "-o", "json")
	if err != nil {
		return nil, fmt.Errorf("failed to get schedule: %w", err)
	}

	var schedule Schedule
	if err := json.Unmarshal([]byte(output), &schedule); err != nil {
		schedule = m.parseScheduleFromDescribe(name, output)
	}

	return &schedule, nil
}

// ListSchedules lists all schedules
func (m *Manager) ListSchedules() ([]Schedule, error) {
	output, err := m.runVelero("schedule", "get", "-o", "json")
	if err != nil {
		output, err = m.runKubectl("get schedules.velero.io -n " + m.namespace + " -o json")
		if err != nil {
			return nil, fmt.Errorf("failed to list schedules: %w", err)
		}
	}

	var schedules []Schedule
	var result struct {
		Items []struct {
			Metadata struct {
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
			} `json:"metadata"`
			Spec struct {
				Schedule           string   `json:"schedule"`
				IncludedNamespaces []string `json:"includedNamespaces"`
				ExcludedNamespaces []string `json:"excludedNamespaces"`
				TTL                string   `json:"ttl"`
			} `json:"spec"`
			Status struct {
				LastBackup time.Time `json:"lastBackup"`
				Phase      string    `json:"phase"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal([]byte(output), &result); err == nil {
		for _, item := range result.Items {
			schedules = append(schedules, Schedule{
				Name:               item.Metadata.Name,
				Namespace:          item.Metadata.Namespace,
				Schedule:           item.Spec.Schedule,
				IncludedNamespaces: item.Spec.IncludedNamespaces,
				ExcludedNamespaces: item.Spec.ExcludedNamespaces,
				TTL:                item.Spec.TTL,
				LastBackup:         item.Status.LastBackup,
				Paused:             item.Status.Phase == "Paused",
			})
		}
	}

	return schedules, nil
}

// DeleteSchedule deletes a schedule
func (m *Manager) DeleteSchedule(name string) error {
	_, err := m.runVelero("schedule", "delete", name, "--confirm")
	if err != nil {
		return fmt.Errorf("failed to delete schedule: %w", err)
	}
	return nil
}

// PauseSchedule pauses a schedule
func (m *Manager) PauseSchedule(name string) error {
	_, err := m.runKubectl(fmt.Sprintf("patch schedule %s -n %s --type merge -p '{\"spec\":{\"paused\":true}}'", name, m.namespace))
	if err != nil {
		return fmt.Errorf("failed to pause schedule: %w", err)
	}
	return nil
}

// UnpauseSchedule unpauses a schedule
func (m *Manager) UnpauseSchedule(name string) error {
	_, err := m.runKubectl(fmt.Sprintf("patch schedule %s -n %s --type merge -p '{\"spec\":{\"paused\":false}}'", name, m.namespace))
	if err != nil {
		return fmt.Errorf("failed to unpause schedule: %w", err)
	}
	return nil
}

// GetBackupLocations lists backup storage locations
func (m *Manager) GetBackupLocations() ([]BackupLocation, error) {
	output, err := m.runKubectl("get backupstoragelocations.velero.io -n " + m.namespace + " -o json")
	if err != nil {
		return nil, fmt.Errorf("failed to get backup locations: %w", err)
	}

	var locations []BackupLocation
	var result struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			Spec struct {
				Provider string `json:"provider"`
				Default  bool   `json:"default"`
				Config   struct {
					Region string `json:"region"`
				} `json:"config"`
				ObjectStorage struct {
					Bucket string `json:"bucket"`
				} `json:"objectStorage"`
				AccessMode string `json:"accessMode"`
			} `json:"spec"`
		} `json:"items"`
	}

	if err := json.Unmarshal([]byte(output), &result); err == nil {
		for _, item := range result.Items {
			locations = append(locations, BackupLocation{
				Name:       item.Metadata.Name,
				Provider:   item.Spec.Provider,
				Bucket:     item.Spec.ObjectStorage.Bucket,
				Region:     item.Spec.Config.Region,
				Default:    item.Spec.Default,
				AccessMode: item.Spec.AccessMode,
			})
		}
	}

	return locations, nil
}

// WaitForBackup waits for a backup to complete
func (m *Manager) WaitForBackup(name string, timeout time.Duration) (*Backup, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout waiting for backup %s to complete", name)
		case <-ticker.C:
			backup, err := m.GetBackup(name)
			if err != nil {
				continue
			}

			switch backup.Status {
			case StatusCompleted:
				return backup, nil
			case StatusFailed, StatusPartiallyFailed:
				return backup, fmt.Errorf("backup %s failed with status: %s", name, backup.Status)
			}
		}
	}
}

// WaitForRestore waits for a restore to complete
func (m *Manager) WaitForRestore(name string, timeout time.Duration) (*Restore, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout waiting for restore %s to complete", name)
		case <-ticker.C:
			restore, err := m.GetRestore(name)
			if err != nil {
				continue
			}

			switch restore.Status {
			case RestoreStatusCompleted:
				return restore, nil
			case RestoreStatusFailed, RestoreStatusPartiallyFailed:
				return restore, fmt.Errorf("restore %s failed with status: %s", name, restore.Status)
			}
		}
	}
}

// Helper functions

func (m *Manager) runVelero(args ...string) (string, error) {
	if m.kubeconfig != "" {
		args = append(args, "--kubeconfig", m.kubeconfig)
	}
	args = append(args, "-n", m.namespace)

	retryConfig := retry.Config{
		MaxRetries:   3,
		InitialDelay: 2 * time.Second,
		MaxDelay:     15 * time.Second,
		Multiplier:   2.0,
		Jitter:       true,
		JitterFactor: 0.2,
		OnRetry: func(attempt int, err error, delay time.Duration) {
			if m.verbose {
				fmt.Printf("    Velero retry %d (waiting %v)\n", attempt, delay)
			}
		},
	}

	r := retry.New(retryConfig)

	var result string
	err := r.Do(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, "velero", args...)
		cmd.Env = os.Environ()
		output, err := cmd.CombinedOutput()

		if ctx.Err() == context.DeadlineExceeded {
			return retry.NewRetryableError(fmt.Errorf("velero command timed out"))
		}
		if err != nil {
			outputStr := string(output)
			if isTransientVeleroError(outputStr) {
				return retry.NewRetryableError(fmt.Errorf("%w: %s", err, outputStr))
			}
			return fmt.Errorf("%w: %s", err, outputStr)
		}
		result = string(output)
		return nil
	})

	return result, err
}

func (m *Manager) runKubectl(args string) (string, error) {
	retryConfig := retry.Config{
		MaxRetries:   3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     10 * time.Second,
		Multiplier:   2.0,
		Jitter:       true,
		JitterFactor: 0.2,
		OnRetry: func(attempt int, err error, delay time.Duration) {
			if m.verbose {
				fmt.Printf("    Kubectl retry %d (waiting %v)\n", attempt, delay)
			}
		},
	}

	r := retry.New(retryConfig)

	var result string
	err := r.Do(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		cmdArgs := strings.Fields(args)
		if m.kubeconfig != "" {
			cmdArgs = append([]string{"--kubeconfig", m.kubeconfig}, cmdArgs...)
		}

		cmd := exec.CommandContext(ctx, "kubectl", cmdArgs...)
		cmd.Env = os.Environ()
		cmd.Stdin = nil
		output, err := cmd.CombinedOutput()

		if ctx.Err() == context.DeadlineExceeded {
			return retry.NewRetryableError(fmt.Errorf("kubectl command timed out"))
		}
		if err != nil {
			outputStr := string(output)
			if isTransientKubectlError(outputStr) {
				return retry.NewRetryableError(fmt.Errorf("%w: %s", err, outputStr))
			}
			return fmt.Errorf("%w: %s", err, outputStr)
		}
		result = string(output)
		return nil
	})

	return result, err
}

// isTransientVeleroError checks if the error is transient
func isTransientVeleroError(output string) bool {
	transientErrors := []string{
		"connection refused",
		"connection reset",
		"timeout",
		"i/o timeout",
		"temporary failure",
		"server is currently unable",
		"etcdserver: leader changed",
	}

	lowerOutput := strings.ToLower(output)
	for _, transient := range transientErrors {
		if strings.Contains(lowerOutput, transient) {
			return true
		}
	}
	return false
}

// isTransientKubectlError checks if the kubectl error is transient
func isTransientKubectlError(output string) bool {
	transientErrors := []string{
		"connection refused",
		"connection reset",
		"timeout",
		"i/o timeout",
		"TLS handshake timeout",
		"Unable to connect to the server",
		"EOF",
		"server is currently unable",
	}

	lowerOutput := strings.ToLower(output)
	for _, transient := range transientErrors {
		if strings.Contains(lowerOutput, strings.ToLower(transient)) {
			return true
		}
	}
	return false
}

func (m *Manager) parseBackupFromDescribe(name, output string) Backup {
	backup := Backup{Name: name}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Phase:") {
			backup.Phase = strings.TrimSpace(strings.TrimPrefix(line, "Phase:"))
			backup.Status = BackupStatus(backup.Phase)
		} else if strings.HasPrefix(line, "Errors:") {
			fmt.Sscanf(strings.TrimPrefix(line, "Errors:"), "%d", &backup.Errors)
		} else if strings.HasPrefix(line, "Warnings:") {
			fmt.Sscanf(strings.TrimPrefix(line, "Warnings:"), "%d", &backup.Warnings)
		}
	}

	return backup
}

func (m *Manager) parseRestoreFromDescribe(name, output string) Restore {
	restore := Restore{Name: name}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Phase:") {
			restore.Phase = strings.TrimSpace(strings.TrimPrefix(line, "Phase:"))
			restore.Status = RestoreStatus(restore.Phase)
		} else if strings.HasPrefix(line, "Backup:") {
			restore.BackupName = strings.TrimSpace(strings.TrimPrefix(line, "Backup:"))
		} else if strings.HasPrefix(line, "Errors:") {
			fmt.Sscanf(strings.TrimPrefix(line, "Errors:"), "%d", &restore.Errors)
		} else if strings.HasPrefix(line, "Warnings:") {
			fmt.Sscanf(strings.TrimPrefix(line, "Warnings:"), "%d", &restore.Warnings)
		}
	}

	return restore
}

func (m *Manager) parseScheduleFromDescribe(name, output string) Schedule {
	schedule := Schedule{Name: name}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Schedule:") {
			schedule.Schedule = strings.TrimSpace(strings.TrimPrefix(line, "Schedule:"))
		} else if strings.HasPrefix(line, "Paused:") {
			schedule.Paused = strings.Contains(line, "true")
		}
	}

	return schedule
}
