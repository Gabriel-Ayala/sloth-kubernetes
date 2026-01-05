package manifests

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRegistry(t *testing.T) {
	r := NewRegistry(10)
	assert.NotNil(t, r)
	assert.Equal(t, 10, r.maxHistory)
}

func TestNewRegistry_DefaultMaxHistory(t *testing.T) {
	r := NewRegistry(0)
	assert.Equal(t, 50, r.maxHistory)

	r = NewRegistry(-1)
	assert.Equal(t, 50, r.maxHistory)
}

func TestRegistry_Register(t *testing.T) {
	r := NewRegistry(10)

	manifest, err := r.Register("test-manifest", ManifestTypeRaw, "apiVersion: v1\nkind: ConfigMap", nil)
	require.NoError(t, err)
	assert.NotNil(t, manifest)
	assert.Equal(t, "test-manifest", manifest.Name)
	assert.Equal(t, ManifestTypeRaw, manifest.Type)
	assert.Equal(t, StatusPending, manifest.Status)
	assert.NotEmpty(t, manifest.Hash)
	assert.Equal(t, "v1", manifest.Version)
}

func TestRegistry_Register_WithMetadata(t *testing.T) {
	r := NewRegistry(10)

	metadata := map[string]string{
		"environment": "production",
		"team":        "platform",
	}

	manifest, err := r.Register("test-manifest", ManifestTypeRaw, "content", metadata)
	require.NoError(t, err)
	assert.Equal(t, metadata, manifest.Metadata)
}

func TestRegistry_Register_EmptyName(t *testing.T) {
	r := NewRegistry(10)

	_, err := r.Register("", ManifestTypeRaw, "content", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "name cannot be empty")
}

func TestRegistry_Register_EmptyContent(t *testing.T) {
	r := NewRegistry(10)

	_, err := r.Register("test", ManifestTypeRaw, "", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "content cannot be empty")
}

func TestRegistry_Register_Update(t *testing.T) {
	r := NewRegistry(10)

	m1, err := r.Register("test", ManifestTypeRaw, "content-v1", nil)
	require.NoError(t, err)
	originalHash := m1.Hash

	m2, err := r.Register("test", ManifestTypeRaw, "content-v2", nil)
	require.NoError(t, err)

	assert.NotEqual(t, originalHash, m2.Hash)
	assert.Equal(t, originalHash, m2.ParentHash)
	assert.Equal(t, "v2", m2.Version)
}

func TestRegistry_Register_NoChange(t *testing.T) {
	r := NewRegistry(10)

	m1, err := r.Register("test", ManifestTypeRaw, "same-content", nil)
	require.NoError(t, err)

	m2, err := r.Register("test", ManifestTypeRaw, "same-content", nil)
	require.NoError(t, err)

	assert.Equal(t, m1.Hash, m2.Hash)
	assert.Equal(t, m1.Version, m2.Version)
}

func TestRegistry_Get(t *testing.T) {
	r := NewRegistry(10)

	_, err := r.Register("test", ManifestTypeRaw, "content", nil)
	require.NoError(t, err)

	m, exists := r.Get("test")
	assert.True(t, exists)
	assert.NotNil(t, m)
	assert.Equal(t, "test", m.Name)

	_, exists = r.Get("nonexistent")
	assert.False(t, exists)
}

func TestRegistry_GetByHash(t *testing.T) {
	r := NewRegistry(10)

	m1, err := r.Register("test", ManifestTypeRaw, "content", nil)
	require.NoError(t, err)

	m2, exists := r.GetByHash(m1.Hash)
	assert.True(t, exists)
	assert.Equal(t, m1.Name, m2.Name)

	_, exists = r.GetByHash("nonexistent-hash")
	assert.False(t, exists)
}

func TestRegistry_List(t *testing.T) {
	r := NewRegistry(10)

	_, _ = r.Register("manifest-b", ManifestTypeRaw, "content", nil)
	_, _ = r.Register("manifest-a", ManifestTypeRaw, "content", nil)
	_, _ = r.Register("manifest-c", ManifestTypeRaw, "content", nil)

	list := r.List()
	assert.Len(t, list, 3)
	// Should be sorted by name
	assert.Equal(t, "manifest-a", list[0].Name)
	assert.Equal(t, "manifest-b", list[1].Name)
	assert.Equal(t, "manifest-c", list[2].Name)
}

func TestRegistry_ListByType(t *testing.T) {
	r := NewRegistry(10)

	_, _ = r.Register("rke-config", ManifestTypeRKE, "content", nil)
	_, _ = r.Register("argocd-app", ManifestTypeArgoCD, "content", nil)
	_, _ = r.Register("helm-release", ManifestTypeHelm, "content", nil)

	rkeManifests := r.ListByType(ManifestTypeRKE)
	assert.Len(t, rkeManifests, 1)
	assert.Equal(t, "rke-config", rkeManifests[0].Name)

	argoManifests := r.ListByType(ManifestTypeArgoCD)
	assert.Len(t, argoManifests, 1)
}

func TestRegistry_ListByStatus(t *testing.T) {
	r := NewRegistry(10)

	_, _ = r.Register("pending-1", ManifestTypeRaw, "content1", nil)
	_, _ = r.Register("pending-2", ManifestTypeRaw, "content2", nil)
	m, _ := r.Register("applied", ManifestTypeRaw, "content3", nil)
	_ = r.UpdateStatus(m.Name, StatusApplied)

	pending := r.ListByStatus(StatusPending)
	assert.Len(t, pending, 2)

	applied := r.ListByStatus(StatusApplied)
	assert.Len(t, applied, 1)
}

func TestRegistry_UpdateStatus(t *testing.T) {
	r := NewRegistry(10)

	_, err := r.Register("test", ManifestTypeRaw, "content", nil)
	require.NoError(t, err)

	err = r.UpdateStatus("test", StatusApplied)
	require.NoError(t, err)

	m, _ := r.Get("test")
	assert.Equal(t, StatusApplied, m.Status)
	assert.NotNil(t, m.AppliedAt)
}

func TestRegistry_UpdateStatus_NotFound(t *testing.T) {
	r := NewRegistry(10)

	err := r.UpdateStatus("nonexistent", StatusApplied)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRegistry_Delete(t *testing.T) {
	r := NewRegistry(10)

	_, err := r.Register("test", ManifestTypeRaw, "content", nil)
	require.NoError(t, err)

	err = r.Delete("test")
	require.NoError(t, err)

	_, exists := r.Get("test")
	assert.False(t, exists)
}

func TestRegistry_Delete_NotFound(t *testing.T) {
	r := NewRegistry(10)

	err := r.Delete("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRegistry_GetHistory(t *testing.T) {
	r := NewRegistry(10)

	_, _ = r.Register("test", ManifestTypeRaw, "content-v1", nil)
	_, _ = r.Register("test", ManifestTypeRaw, "content-v2", nil)
	_, _ = r.Register("test", ManifestTypeRaw, "content-v3", nil)

	h, exists := r.GetHistory("test")
	require.True(t, exists)
	assert.Equal(t, "test", h.ManifestID)
	assert.Len(t, h.Versions, 3)
}

func TestRegistry_GetHistory_NotFound(t *testing.T) {
	r := NewRegistry(10)

	_, exists := r.GetHistory("nonexistent")
	assert.False(t, exists)
}

func TestRegistry_GetVersionFromHistory(t *testing.T) {
	r := NewRegistry(10)

	m1, _ := r.Register("test", ManifestTypeRaw, "content-v1", nil)
	_, _ = r.Register("test", ManifestTypeRaw, "content-v2", nil)

	v, exists := r.GetVersionFromHistory("test", m1.Hash)
	require.True(t, exists)
	assert.Equal(t, m1.Hash, v.Hash)
}

func TestRegistry_GetVersionFromHistory_NotFound(t *testing.T) {
	r := NewRegistry(10)

	_, _ = r.Register("test", ManifestTypeRaw, "content", nil)

	_, exists := r.GetVersionFromHistory("test", "nonexistent-hash")
	assert.False(t, exists)

	_, exists = r.GetVersionFromHistory("nonexistent", "hash")
	assert.False(t, exists)
}

func TestRegistry_ComputeRegistryHash(t *testing.T) {
	r := NewRegistry(10)

	hash1 := r.ComputeRegistryHash()

	_, _ = r.Register("test", ManifestTypeRaw, "content", nil)

	hash2 := r.ComputeRegistryHash()

	assert.NotEqual(t, hash1, hash2)
}

func TestRegistry_ComputeRegistryHash_Deterministic(t *testing.T) {
	r := NewRegistry(10)

	_, _ = r.Register("a", ManifestTypeRaw, "content-a", nil)
	_, _ = r.Register("b", ManifestTypeRaw, "content-b", nil)

	hash1 := r.ComputeRegistryHash()
	hash2 := r.ComputeRegistryHash()

	assert.Equal(t, hash1, hash2)
}

func TestRegistry_Diff(t *testing.T) {
	r1 := NewRegistry(10)
	r2 := NewRegistry(10)

	// Add to r1
	_, _ = r1.Register("common", ManifestTypeRaw, "same-content", nil)
	_, _ = r1.Register("only-in-r1", ManifestTypeRaw, "content", nil)
	_, _ = r1.Register("modified", ManifestTypeRaw, "content-v1", nil)

	// Add to r2
	_, _ = r2.Register("common", ManifestTypeRaw, "same-content", nil)
	_, _ = r2.Register("only-in-r2", ManifestTypeRaw, "content", nil)
	_, _ = r2.Register("modified", ManifestTypeRaw, "content-v2", nil)

	diff := r1.Diff(r2)

	assert.Len(t, diff.Added, 1)
	assert.Equal(t, "only-in-r1", diff.Added[0].Name)

	assert.Len(t, diff.Removed, 1)
	assert.Equal(t, "only-in-r2", diff.Removed[0].Name)

	assert.Len(t, diff.Modified, 1)
	assert.Equal(t, "modified", diff.Modified[0].Name)
}

func TestRegistryDiff_HasChanges(t *testing.T) {
	tests := []struct {
		name string
		diff *RegistryDiff
		want bool
	}{
		{
			name: "no changes",
			diff: &RegistryDiff{},
			want: false,
		},
		{
			name: "has added",
			diff: &RegistryDiff{Added: []*Manifest{{}}},
			want: true,
		},
		{
			name: "has removed",
			diff: &RegistryDiff{Removed: []*Manifest{{}}},
			want: true,
		},
		{
			name: "has modified",
			diff: &RegistryDiff{Modified: []*ManifestChange{{}}},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.diff.HasChanges())
		})
	}
}

func TestRegistry_Export_Import(t *testing.T) {
	r1 := NewRegistry(10)

	_, _ = r1.Register("test1", ManifestTypeRaw, "content1", map[string]string{"key": "value"})
	_, _ = r1.Register("test2", ManifestTypeArgoCD, "content2", nil)

	export, err := r1.Export()
	require.NoError(t, err)
	assert.NotEmpty(t, export.Version)
	assert.NotEmpty(t, export.Hash)
	assert.Len(t, export.Manifests, 2)

	r2 := NewRegistry(10)
	err = r2.Import(export)
	require.NoError(t, err)

	m, exists := r2.Get("test1")
	assert.True(t, exists)
	assert.Equal(t, "content1", m.Content)
}

func TestRegistry_Import_Nil(t *testing.T) {
	r := NewRegistry(10)

	err := r.Import(nil)
	assert.Error(t, err)
}

func TestRegistry_ToJSON_FromJSON(t *testing.T) {
	r1 := NewRegistry(10)

	_, _ = r1.Register("test", ManifestTypeRaw, "content", nil)

	jsonData, err := r1.ToJSON()
	require.NoError(t, err)
	assert.NotEmpty(t, jsonData)

	// Verify it's valid JSON
	var export RegistryExport
	err = json.Unmarshal(jsonData, &export)
	require.NoError(t, err)

	r2 := NewRegistry(10)
	err = r2.FromJSON(jsonData)
	require.NoError(t, err)

	m, exists := r2.Get("test")
	assert.True(t, exists)
	assert.Equal(t, "content", m.Content)
}

func TestRegistry_FromJSON_Invalid(t *testing.T) {
	r := NewRegistry(10)

	err := r.FromJSON([]byte("invalid json"))
	assert.Error(t, err)
}

func TestRegistry_HistoryTrimming(t *testing.T) {
	r := NewRegistry(3) // Small history for testing

	for i := 0; i < 5; i++ {
		_, _ = r.Register("test", ManifestTypeRaw, "content-"+string(rune('a'+i)), nil)
	}

	h, _ := r.GetHistory("test")
	assert.Len(t, h.Versions, 3)
}

func TestManifestTypes(t *testing.T) {
	assert.Equal(t, ManifestType("rke"), ManifestTypeRKE)
	assert.Equal(t, ManifestType("rke2"), ManifestTypeRKE2)
	assert.Equal(t, ManifestType("k3s"), ManifestTypeK3s)
	assert.Equal(t, ManifestType("argocd"), ManifestTypeArgoCD)
	assert.Equal(t, ManifestType("helm"), ManifestTypeHelm)
	assert.Equal(t, ManifestType("kustomize"), ManifestTypeKustomize)
	assert.Equal(t, ManifestType("raw"), ManifestTypeRaw)
}

func TestManifestStatuses(t *testing.T) {
	assert.Equal(t, ManifestStatus("pending"), StatusPending)
	assert.Equal(t, ManifestStatus("applied"), StatusApplied)
	assert.Equal(t, ManifestStatus("failed"), StatusFailed)
	assert.Equal(t, ManifestStatus("deleted"), StatusDeleted)
	assert.Equal(t, ManifestStatus("out_of_sync"), StatusOutOfSync)
}

func TestComputeHash(t *testing.T) {
	hash1 := computeHash("content")
	hash2 := computeHash("content")
	hash3 := computeHash("different")

	assert.Equal(t, hash1, hash2)
	assert.NotEqual(t, hash1, hash3)
	assert.Len(t, hash1, 64)
}

func TestManifest_Timestamps(t *testing.T) {
	r := NewRegistry(10)

	before := time.Now().Add(-time.Second)
	m, _ := r.Register("test", ManifestTypeRaw, "content", nil)
	after := time.Now().Add(time.Second)

	assert.True(t, m.CreatedAt.After(before))
	assert.True(t, m.CreatedAt.Before(after))
	assert.True(t, m.UpdatedAt.After(before))
	assert.True(t, m.UpdatedAt.Before(after))
}
