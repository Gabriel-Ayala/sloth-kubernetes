// Package versioning provides configuration versioning and migration capabilities
// for managing cluster configurations across different schema versions.
package versioning

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

// SchemaVersion represents a configuration schema version
type SchemaVersion string

const (
	// SchemaV1 is the initial schema version
	SchemaV1 SchemaVersion = "1.0"
	// SchemaV2 adds manifest tracking
	SchemaV2 SchemaVersion = "2.0"
	// CurrentSchema is the current schema version
	CurrentSchema SchemaVersion = SchemaV2
)

// ConfigVersion holds version metadata for a configuration
type ConfigVersion struct {
	// SchemaVersion is the schema version of the configuration
	SchemaVersion SchemaVersion `json:"schema_version"`
	// ConfigHash is the SHA256 hash of the configuration content
	ConfigHash string `json:"config_hash"`
	// CreatedAt is when this version was created
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is when this version was last updated
	UpdatedAt time.Time `json:"updated_at"`
	// Migrations lists migrations that have been applied
	Migrations []string `json:"migrations,omitempty"`
	// ParentHash is the hash of the previous configuration version
	ParentHash string `json:"parent_hash,omitempty"`
}

// VersionedConfig wraps a configuration with version metadata
type VersionedConfig struct {
	Version ConfigVersion          `json:"version"`
	Config  map[string]interface{} `json:"config"`
}

// Migration represents a configuration migration
type Migration struct {
	// ID is a unique identifier for this migration
	ID string
	// FromVersion is the source schema version
	FromVersion SchemaVersion
	// ToVersion is the target schema version
	ToVersion SchemaVersion
	// Description describes what this migration does
	Description string
	// Migrate performs the migration on the configuration
	Migrate func(config map[string]interface{}) (map[string]interface{}, error)
}

// MigrationRegistry holds all available migrations
type MigrationRegistry struct {
	migrations map[string]*Migration
	order      []string
}

// NewMigrationRegistry creates a new migration registry
func NewMigrationRegistry() *MigrationRegistry {
	return &MigrationRegistry{
		migrations: make(map[string]*Migration),
		order:      make([]string, 0),
	}
}

// Register adds a migration to the registry
func (r *MigrationRegistry) Register(m *Migration) error {
	if m.ID == "" {
		return fmt.Errorf("migration ID cannot be empty")
	}
	if _, exists := r.migrations[m.ID]; exists {
		return fmt.Errorf("migration %s already registered", m.ID)
	}
	r.migrations[m.ID] = m
	r.order = append(r.order, m.ID)
	return nil
}

// GetMigrationPath returns the sequence of migrations needed to go from one version to another
func (r *MigrationRegistry) GetMigrationPath(from, to SchemaVersion) ([]*Migration, error) {
	if from == to {
		return nil, nil
	}

	var path []*Migration
	currentVersion := from

	for _, id := range r.order {
		m := r.migrations[id]
		if m.FromVersion == currentVersion {
			path = append(path, m)
			currentVersion = m.ToVersion
			if currentVersion == to {
				return path, nil
			}
		}
	}

	if currentVersion != to {
		return nil, fmt.Errorf("no migration path found from %s to %s", from, to)
	}

	return path, nil
}

// VersionManager handles configuration versioning
type VersionManager struct {
	registry *MigrationRegistry
}

// NewVersionManager creates a new version manager
func NewVersionManager() *VersionManager {
	vm := &VersionManager{
		registry: NewMigrationRegistry(),
	}
	vm.registerDefaultMigrations()
	return vm
}

// registerDefaultMigrations registers the default set of migrations
func (vm *VersionManager) registerDefaultMigrations() {
	// Migration from v1.0 to v2.0: Add manifest tracking fields
	_ = vm.registry.Register(&Migration{
		ID:          "v1_to_v2_manifest_tracking",
		FromVersion: SchemaV1,
		ToVersion:   SchemaV2,
		Description: "Add manifest tracking and versioning fields",
		Migrate: func(config map[string]interface{}) (map[string]interface{}, error) {
			// Add manifest_tracking section if not present
			if _, exists := config["manifest_tracking"]; !exists {
				config["manifest_tracking"] = map[string]interface{}{
					"enabled":         true,
					"retention_count": 10,
				}
			}
			// Add versioning section if not present
			if _, exists := config["versioning"]; !exists {
				config["versioning"] = map[string]interface{}{
					"auto_migrate": true,
				}
			}
			return config, nil
		},
	})
}

// ComputeHash calculates the SHA256 hash of a configuration
func (vm *VersionManager) ComputeHash(config interface{}) (string, error) {
	data, err := json.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("failed to marshal config for hashing: %w", err)
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

// CreateVersion creates a new ConfigVersion for the given configuration
func (vm *VersionManager) CreateVersion(config map[string]interface{}, parentHash string) (*ConfigVersion, error) {
	hash, err := vm.ComputeHash(config)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	return &ConfigVersion{
		SchemaVersion: CurrentSchema,
		ConfigHash:    hash,
		CreatedAt:     now,
		UpdatedAt:     now,
		ParentHash:    parentHash,
	}, nil
}

// Migrate migrates a configuration from one schema version to another
func (vm *VersionManager) Migrate(config map[string]interface{}, from, to SchemaVersion) (map[string]interface{}, []string, error) {
	migrations, err := vm.registry.GetMigrationPath(from, to)
	if err != nil {
		return nil, nil, err
	}

	if len(migrations) == 0 {
		return config, nil, nil
	}

	result := make(map[string]interface{})
	for k, v := range config {
		result[k] = v
	}

	var appliedMigrations []string
	for _, m := range migrations {
		result, err = m.Migrate(result)
		if err != nil {
			return nil, appliedMigrations, fmt.Errorf("migration %s failed: %w", m.ID, err)
		}
		appliedMigrations = append(appliedMigrations, m.ID)
	}

	return result, appliedMigrations, nil
}

// ValidateVersion checks if a configuration version is valid
func (vm *VersionManager) ValidateVersion(v *ConfigVersion) error {
	if v.SchemaVersion == "" {
		return fmt.Errorf("schema version is required")
	}
	if v.ConfigHash == "" {
		return fmt.Errorf("config hash is required")
	}
	if v.CreatedAt.IsZero() {
		return fmt.Errorf("created_at is required")
	}
	return nil
}

// CompareVersions compares two config versions and returns a diff
func (vm *VersionManager) CompareVersions(v1, v2 *ConfigVersion) *VersionDiff {
	diff := &VersionDiff{
		HashChanged:   v1.ConfigHash != v2.ConfigHash,
		SchemaChanged: v1.SchemaVersion != v2.SchemaVersion,
		OldHash:       v1.ConfigHash,
		NewHash:       v2.ConfigHash,
		OldSchema:     v1.SchemaVersion,
		NewSchema:     v2.SchemaVersion,
		TimeDelta:     v2.UpdatedAt.Sub(v1.UpdatedAt),
		NewMigrations: getMigrationDiff(v1.Migrations, v2.Migrations),
	}
	return diff
}

// VersionDiff represents the difference between two versions
type VersionDiff struct {
	HashChanged   bool
	SchemaChanged bool
	OldHash       string
	NewHash       string
	OldSchema     SchemaVersion
	NewSchema     SchemaVersion
	TimeDelta     time.Duration
	NewMigrations []string
}

// HasChanges returns true if there are any changes
func (d *VersionDiff) HasChanges() bool {
	return d.HashChanged || d.SchemaChanged || len(d.NewMigrations) > 0
}

// getMigrationDiff returns migrations in v2 that are not in v1
func getMigrationDiff(v1, v2 []string) []string {
	v1Set := make(map[string]bool)
	for _, m := range v1 {
		v1Set[m] = true
	}

	var diff []string
	for _, m := range v2 {
		if !v1Set[m] {
			diff = append(diff, m)
		}
	}
	return diff
}

// VersionHistory tracks the history of configuration versions
type VersionHistory struct {
	Versions []ConfigVersion `json:"versions"`
	MaxSize  int             `json:"max_size"`
}

// NewVersionHistory creates a new version history with the given max size
func NewVersionHistory(maxSize int) *VersionHistory {
	if maxSize <= 0 {
		maxSize = 100
	}
	return &VersionHistory{
		Versions: make([]ConfigVersion, 0),
		MaxSize:  maxSize,
	}
}

// Add adds a version to the history
func (h *VersionHistory) Add(v ConfigVersion) {
	h.Versions = append(h.Versions, v)
	if len(h.Versions) > h.MaxSize {
		h.Versions = h.Versions[1:]
	}
}

// GetByHash retrieves a version by its hash
func (h *VersionHistory) GetByHash(hash string) *ConfigVersion {
	for i := range h.Versions {
		if h.Versions[i].ConfigHash == hash {
			return &h.Versions[i]
		}
	}
	return nil
}

// GetLatest returns the most recent version
func (h *VersionHistory) GetLatest() *ConfigVersion {
	if len(h.Versions) == 0 {
		return nil
	}
	return &h.Versions[len(h.Versions)-1]
}

// GetVersionsAfter returns all versions after the given timestamp
func (h *VersionHistory) GetVersionsAfter(t time.Time) []ConfigVersion {
	var result []ConfigVersion
	for _, v := range h.Versions {
		if v.CreatedAt.After(t) {
			result = append(result, v)
		}
	}
	return result
}

// Sort sorts the versions by creation time
func (h *VersionHistory) Sort() {
	sort.Slice(h.Versions, func(i, j int) bool {
		return h.Versions[i].CreatedAt.Before(h.Versions[j].CreatedAt)
	})
}
