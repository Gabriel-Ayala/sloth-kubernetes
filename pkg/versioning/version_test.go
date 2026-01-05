package versioning

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewVersionManager(t *testing.T) {
	vm := NewVersionManager()
	assert.NotNil(t, vm)
	assert.NotNil(t, vm.registry)
}

func TestVersionManager_ComputeHash(t *testing.T) {
	vm := NewVersionManager()

	tests := []struct {
		name    string
		config  interface{}
		wantErr bool
	}{
		{
			name:    "simple map",
			config:  map[string]interface{}{"key": "value"},
			wantErr: false,
		},
		{
			name:    "nested map",
			config:  map[string]interface{}{"outer": map[string]interface{}{"inner": "value"}},
			wantErr: false,
		},
		{
			name:    "empty map",
			config:  map[string]interface{}{},
			wantErr: false,
		},
		{
			name:    "with numbers",
			config:  map[string]interface{}{"count": 42, "ratio": 3.14},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := vm.ComputeHash(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, hash)
				assert.Len(t, hash, 64) // SHA256 hex string length
			}
		})
	}
}

func TestVersionManager_ComputeHash_Deterministic(t *testing.T) {
	vm := NewVersionManager()
	config := map[string]interface{}{
		"name":    "test-cluster",
		"version": "1.0.0",
		"nodes":   3,
	}

	hash1, err := vm.ComputeHash(config)
	require.NoError(t, err)

	hash2, err := vm.ComputeHash(config)
	require.NoError(t, err)

	assert.Equal(t, hash1, hash2, "Hash should be deterministic")
}

func TestVersionManager_ComputeHash_Different(t *testing.T) {
	vm := NewVersionManager()

	config1 := map[string]interface{}{"key": "value1"}
	config2 := map[string]interface{}{"key": "value2"}

	hash1, err := vm.ComputeHash(config1)
	require.NoError(t, err)

	hash2, err := vm.ComputeHash(config2)
	require.NoError(t, err)

	assert.NotEqual(t, hash1, hash2, "Different configs should have different hashes")
}

func TestVersionManager_CreateVersion(t *testing.T) {
	vm := NewVersionManager()
	config := map[string]interface{}{"name": "test"}

	version, err := vm.CreateVersion(config, "")
	require.NoError(t, err)
	assert.NotNil(t, version)
	assert.Equal(t, CurrentSchema, version.SchemaVersion)
	assert.NotEmpty(t, version.ConfigHash)
	assert.False(t, version.CreatedAt.IsZero())
	assert.False(t, version.UpdatedAt.IsZero())
	assert.Empty(t, version.ParentHash)
}

func TestVersionManager_CreateVersion_WithParent(t *testing.T) {
	vm := NewVersionManager()
	config := map[string]interface{}{"name": "test"}
	parentHash := "abc123def456"

	version, err := vm.CreateVersion(config, parentHash)
	require.NoError(t, err)
	assert.Equal(t, parentHash, version.ParentHash)
}

func TestVersionManager_ValidateVersion(t *testing.T) {
	vm := NewVersionManager()

	tests := []struct {
		name    string
		version *ConfigVersion
		wantErr bool
	}{
		{
			name: "valid version",
			version: &ConfigVersion{
				SchemaVersion: SchemaV2,
				ConfigHash:    "abc123",
				CreatedAt:     time.Now(),
			},
			wantErr: false,
		},
		{
			name: "missing schema version",
			version: &ConfigVersion{
				ConfigHash: "abc123",
				CreatedAt:  time.Now(),
			},
			wantErr: true,
		},
		{
			name: "missing config hash",
			version: &ConfigVersion{
				SchemaVersion: SchemaV2,
				CreatedAt:     time.Now(),
			},
			wantErr: true,
		},
		{
			name: "missing created_at",
			version: &ConfigVersion{
				SchemaVersion: SchemaV2,
				ConfigHash:    "abc123",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := vm.ValidateVersion(tt.version)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMigrationRegistry_Register(t *testing.T) {
	registry := NewMigrationRegistry()

	m := &Migration{
		ID:          "test_migration",
		FromVersion: SchemaV1,
		ToVersion:   SchemaV2,
		Description: "Test migration",
		Migrate: func(config map[string]interface{}) (map[string]interface{}, error) {
			return config, nil
		},
	}

	err := registry.Register(m)
	assert.NoError(t, err)
}

func TestMigrationRegistry_Register_Duplicate(t *testing.T) {
	registry := NewMigrationRegistry()

	m := &Migration{
		ID:          "test_migration",
		FromVersion: SchemaV1,
		ToVersion:   SchemaV2,
		Migrate: func(config map[string]interface{}) (map[string]interface{}, error) {
			return config, nil
		},
	}

	err := registry.Register(m)
	require.NoError(t, err)

	err = registry.Register(m)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestMigrationRegistry_Register_EmptyID(t *testing.T) {
	registry := NewMigrationRegistry()

	m := &Migration{
		ID:          "",
		FromVersion: SchemaV1,
		ToVersion:   SchemaV2,
		Migrate: func(config map[string]interface{}) (map[string]interface{}, error) {
			return config, nil
		},
	}

	err := registry.Register(m)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

func TestMigrationRegistry_GetMigrationPath(t *testing.T) {
	registry := NewMigrationRegistry()

	m1 := &Migration{
		ID:          "v1_to_v2",
		FromVersion: SchemaV1,
		ToVersion:   SchemaV2,
		Migrate: func(config map[string]interface{}) (map[string]interface{}, error) {
			config["migrated"] = true
			return config, nil
		},
	}

	err := registry.Register(m1)
	require.NoError(t, err)

	path, err := registry.GetMigrationPath(SchemaV1, SchemaV2)
	require.NoError(t, err)
	assert.Len(t, path, 1)
	assert.Equal(t, "v1_to_v2", path[0].ID)
}

func TestMigrationRegistry_GetMigrationPath_SameVersion(t *testing.T) {
	registry := NewMigrationRegistry()

	path, err := registry.GetMigrationPath(SchemaV1, SchemaV1)
	assert.NoError(t, err)
	assert.Nil(t, path)
}

func TestMigrationRegistry_GetMigrationPath_NoPath(t *testing.T) {
	registry := NewMigrationRegistry()

	_, err := registry.GetMigrationPath(SchemaV1, "v99.0")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no migration path found")
}

func TestVersionManager_Migrate(t *testing.T) {
	vm := NewVersionManager()

	config := map[string]interface{}{
		"name": "test-cluster",
	}

	migrated, appliedMigrations, err := vm.Migrate(config, SchemaV1, SchemaV2)
	require.NoError(t, err)
	assert.NotNil(t, migrated)
	assert.Contains(t, migrated, "manifest_tracking")
	assert.Contains(t, migrated, "versioning")
	assert.Len(t, appliedMigrations, 1)
	assert.Equal(t, "v1_to_v2_manifest_tracking", appliedMigrations[0])
}

func TestVersionManager_Migrate_SameVersion(t *testing.T) {
	vm := NewVersionManager()

	config := map[string]interface{}{
		"name": "test-cluster",
	}

	migrated, appliedMigrations, err := vm.Migrate(config, SchemaV2, SchemaV2)
	require.NoError(t, err)
	assert.Equal(t, config, migrated)
	assert.Empty(t, appliedMigrations)
}

func TestVersionManager_CompareVersions(t *testing.T) {
	vm := NewVersionManager()

	now := time.Now()
	v1 := &ConfigVersion{
		SchemaVersion: SchemaV1,
		ConfigHash:    "hash1",
		CreatedAt:     now,
		UpdatedAt:     now,
		Migrations:    []string{"m1"},
	}

	v2 := &ConfigVersion{
		SchemaVersion: SchemaV2,
		ConfigHash:    "hash2",
		CreatedAt:     now.Add(time.Hour),
		UpdatedAt:     now.Add(time.Hour),
		Migrations:    []string{"m1", "m2"},
	}

	diff := vm.CompareVersions(v1, v2)
	assert.True(t, diff.HashChanged)
	assert.True(t, diff.SchemaChanged)
	assert.Equal(t, "hash1", diff.OldHash)
	assert.Equal(t, "hash2", diff.NewHash)
	assert.Equal(t, SchemaV1, diff.OldSchema)
	assert.Equal(t, SchemaV2, diff.NewSchema)
	assert.Equal(t, time.Hour, diff.TimeDelta)
	assert.Contains(t, diff.NewMigrations, "m2")
	assert.True(t, diff.HasChanges())
}

func TestVersionDiff_HasChanges(t *testing.T) {
	tests := []struct {
		name string
		diff *VersionDiff
		want bool
	}{
		{
			name: "no changes",
			diff: &VersionDiff{},
			want: false,
		},
		{
			name: "hash changed",
			diff: &VersionDiff{HashChanged: true},
			want: true,
		},
		{
			name: "schema changed",
			diff: &VersionDiff{SchemaChanged: true},
			want: true,
		},
		{
			name: "new migrations",
			diff: &VersionDiff{NewMigrations: []string{"m1"}},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.diff.HasChanges())
		})
	}
}

func TestVersionHistory_Add(t *testing.T) {
	h := NewVersionHistory(3)

	for i := 0; i < 5; i++ {
		h.Add(ConfigVersion{
			ConfigHash: string(rune('a' + i)),
			CreatedAt:  time.Now(),
		})
	}

	assert.Len(t, h.Versions, 3)
	assert.Equal(t, "c", h.Versions[0].ConfigHash)
	assert.Equal(t, "e", h.Versions[2].ConfigHash)
}

func TestVersionHistory_GetByHash(t *testing.T) {
	h := NewVersionHistory(10)
	h.Add(ConfigVersion{ConfigHash: "abc", CreatedAt: time.Now()})
	h.Add(ConfigVersion{ConfigHash: "def", CreatedAt: time.Now()})

	v := h.GetByHash("abc")
	assert.NotNil(t, v)
	assert.Equal(t, "abc", v.ConfigHash)

	v = h.GetByHash("notexist")
	assert.Nil(t, v)
}

func TestVersionHistory_GetLatest(t *testing.T) {
	h := NewVersionHistory(10)

	// Empty history
	assert.Nil(t, h.GetLatest())

	h.Add(ConfigVersion{ConfigHash: "first", CreatedAt: time.Now()})
	h.Add(ConfigVersion{ConfigHash: "second", CreatedAt: time.Now()})

	latest := h.GetLatest()
	assert.NotNil(t, latest)
	assert.Equal(t, "second", latest.ConfigHash)
}

func TestVersionHistory_GetVersionsAfter(t *testing.T) {
	h := NewVersionHistory(10)
	baseTime := time.Now()

	h.Add(ConfigVersion{ConfigHash: "old", CreatedAt: baseTime.Add(-time.Hour)})
	h.Add(ConfigVersion{ConfigHash: "new1", CreatedAt: baseTime.Add(time.Minute)})
	h.Add(ConfigVersion{ConfigHash: "new2", CreatedAt: baseTime.Add(2 * time.Minute)})

	versions := h.GetVersionsAfter(baseTime)
	assert.Len(t, versions, 2)
}

func TestVersionHistory_Sort(t *testing.T) {
	h := NewVersionHistory(10)
	now := time.Now()

	h.Add(ConfigVersion{ConfigHash: "c", CreatedAt: now.Add(2 * time.Hour)})
	h.Add(ConfigVersion{ConfigHash: "a", CreatedAt: now})
	h.Add(ConfigVersion{ConfigHash: "b", CreatedAt: now.Add(time.Hour)})

	h.Sort()

	assert.Equal(t, "a", h.Versions[0].ConfigHash)
	assert.Equal(t, "b", h.Versions[1].ConfigHash)
	assert.Equal(t, "c", h.Versions[2].ConfigHash)
}

func TestNewVersionHistory_DefaultMaxSize(t *testing.T) {
	h := NewVersionHistory(0)
	assert.Equal(t, 100, h.MaxSize)

	h = NewVersionHistory(-1)
	assert.Equal(t, 100, h.MaxSize)
}

func TestSchemaVersionConstants(t *testing.T) {
	assert.Equal(t, SchemaVersion("1.0"), SchemaV1)
	assert.Equal(t, SchemaVersion("2.0"), SchemaV2)
	assert.Equal(t, SchemaV2, CurrentSchema)
}
