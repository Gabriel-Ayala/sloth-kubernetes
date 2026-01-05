package state

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewInMemoryManager(t *testing.T) {
	m := NewInMemoryManager(10)
	assert.NotNil(t, m)
	assert.Equal(t, 10, m.maxSize)
}

func TestNewInMemoryManager_DefaultMaxSize(t *testing.T) {
	m := NewInMemoryManager(0)
	assert.Equal(t, 100, m.maxSize)

	m = NewInMemoryManager(-1)
	assert.Equal(t, 100, m.maxSize)
}

func TestInMemoryManager_SaveSnapshot(t *testing.T) {
	ctx := context.Background()
	m := NewInMemoryManager(10)

	snapshot := &StateSnapshot{
		DeploymentID: "deploy-1",
		ConfigHash:   "config-hash",
		ManifestHash: "manifest-hash",
		NodeCount:    3,
		NodePools:    map[string]int{"workers": 3},
	}

	err := m.SaveSnapshot(ctx, snapshot)
	require.NoError(t, err)
	assert.NotEmpty(t, snapshot.ID)
	assert.Equal(t, SnapshotStatusActive, snapshot.Status)
	assert.False(t, snapshot.Timestamp.IsZero())
}

func TestInMemoryManager_SaveSnapshot_Nil(t *testing.T) {
	ctx := context.Background()
	m := NewInMemoryManager(10)

	err := m.SaveSnapshot(ctx, nil)
	assert.Error(t, err)
}

func TestInMemoryManager_SaveSnapshot_ArchivesPrevious(t *testing.T) {
	ctx := context.Background()
	m := NewInMemoryManager(10)

	s1 := &StateSnapshot{DeploymentID: "deploy-1", ConfigHash: "hash1"}
	err := m.SaveSnapshot(ctx, s1)
	require.NoError(t, err)
	assert.Equal(t, SnapshotStatusActive, s1.Status)

	s2 := &StateSnapshot{DeploymentID: "deploy-2", ConfigHash: "hash2"}
	err = m.SaveSnapshot(ctx, s2)
	require.NoError(t, err)

	// First snapshot should be archived
	retrieved, _ := m.GetSnapshot(ctx, s1.ID)
	assert.Equal(t, SnapshotStatusArchived, retrieved.Status)

	// Second should be active
	assert.Equal(t, SnapshotStatusActive, s2.Status)
}

func TestInMemoryManager_GetSnapshot(t *testing.T) {
	ctx := context.Background()
	m := NewInMemoryManager(10)

	s := &StateSnapshot{DeploymentID: "deploy-1", ConfigHash: "hash"}
	_ = m.SaveSnapshot(ctx, s)

	retrieved, err := m.GetSnapshot(ctx, s.ID)
	require.NoError(t, err)
	assert.Equal(t, s.DeploymentID, retrieved.DeploymentID)
}

func TestInMemoryManager_GetSnapshot_NotFound(t *testing.T) {
	ctx := context.Background()
	m := NewInMemoryManager(10)

	_, err := m.GetSnapshot(ctx, "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestInMemoryManager_GetLatestSnapshot(t *testing.T) {
	ctx := context.Background()
	m := NewInMemoryManager(10)

	s1 := &StateSnapshot{DeploymentID: "deploy-1", ConfigHash: "hash1"}
	_ = m.SaveSnapshot(ctx, s1)

	s2 := &StateSnapshot{DeploymentID: "deploy-2", ConfigHash: "hash2"}
	_ = m.SaveSnapshot(ctx, s2)

	latest, err := m.GetLatestSnapshot(ctx)
	require.NoError(t, err)
	assert.Equal(t, s2.ID, latest.ID)
}

func TestInMemoryManager_GetLatestSnapshot_Empty(t *testing.T) {
	ctx := context.Background()
	m := NewInMemoryManager(10)

	_, err := m.GetLatestSnapshot(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no snapshots")
}

func TestInMemoryManager_ListSnapshots(t *testing.T) {
	ctx := context.Background()
	m := NewInMemoryManager(10)

	for i := 0; i < 5; i++ {
		s := &StateSnapshot{DeploymentID: "deploy", ConfigHash: string(rune('a' + i))}
		_ = m.SaveSnapshot(ctx, s)
	}

	list, err := m.ListSnapshots(ctx, ListOptions{})
	require.NoError(t, err)
	assert.Len(t, list, 5)
}

func TestInMemoryManager_ListSnapshots_WithLimit(t *testing.T) {
	ctx := context.Background()
	m := NewInMemoryManager(10)

	for i := 0; i < 5; i++ {
		s := &StateSnapshot{DeploymentID: "deploy", ConfigHash: string(rune('a' + i))}
		_ = m.SaveSnapshot(ctx, s)
	}

	list, err := m.ListSnapshots(ctx, ListOptions{Limit: 2})
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestInMemoryManager_ListSnapshots_WithTimeFilters(t *testing.T) {
	ctx := context.Background()
	m := NewInMemoryManager(10)

	now := time.Now()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	s := &StateSnapshot{
		DeploymentID: "deploy",
		ConfigHash:   "hash",
		Timestamp:    now,
	}
	m.snapshots[s.ID] = s
	m.order = append(m.order, s.ID)

	// Should include
	list, _ := m.ListSnapshots(ctx, ListOptions{Since: &past})
	assert.Len(t, list, 1)

	// Should exclude
	list, _ = m.ListSnapshots(ctx, ListOptions{Since: &future})
	assert.Len(t, list, 0)
}

func TestInMemoryManager_ListSnapshots_ByStatus(t *testing.T) {
	ctx := context.Background()
	m := NewInMemoryManager(10)

	_ = m.SaveSnapshot(ctx, &StateSnapshot{DeploymentID: "d1", ConfigHash: "h1"})
	_ = m.SaveSnapshot(ctx, &StateSnapshot{DeploymentID: "d2", ConfigHash: "h2"})

	activeStatus := SnapshotStatusActive
	list, err := m.ListSnapshots(ctx, ListOptions{Status: &activeStatus})
	require.NoError(t, err)
	assert.Len(t, list, 1)

	archivedStatus := SnapshotStatusArchived
	list, err = m.ListSnapshots(ctx, ListOptions{Status: &archivedStatus})
	require.NoError(t, err)
	assert.Len(t, list, 1)
}

func TestInMemoryManager_ListSnapshots_ByDeploymentID(t *testing.T) {
	ctx := context.Background()
	m := NewInMemoryManager(10)

	_ = m.SaveSnapshot(ctx, &StateSnapshot{DeploymentID: "deploy-1", ConfigHash: "h1"})
	_ = m.SaveSnapshot(ctx, &StateSnapshot{DeploymentID: "deploy-2", ConfigHash: "h2"})
	_ = m.SaveSnapshot(ctx, &StateSnapshot{DeploymentID: "deploy-1", ConfigHash: "h3"})

	list, err := m.ListSnapshots(ctx, ListOptions{DeploymentID: "deploy-1"})
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestInMemoryManager_DiffSnapshots(t *testing.T) {
	ctx := context.Background()
	m := NewInMemoryManager(10)

	s1 := &StateSnapshot{
		DeploymentID: "deploy",
		ConfigHash:   "config-1",
		ManifestHash: "manifest-1",
		NodeCount:    3,
		NodePools:    map[string]int{"workers": 3},
	}
	_ = m.SaveSnapshot(ctx, s1)

	s2 := &StateSnapshot{
		DeploymentID: "deploy",
		ConfigHash:   "config-2",
		ManifestHash: "manifest-1",
		NodeCount:    5,
		NodePools:    map[string]int{"workers": 5},
	}
	_ = m.SaveSnapshot(ctx, s2)

	diff, err := m.DiffSnapshots(ctx, s1.ID, s2.ID)
	require.NoError(t, err)

	assert.True(t, diff.ConfigChanged)
	assert.False(t, diff.ManifestChanged)
	assert.Equal(t, 2, diff.NodeCountDelta)
	assert.True(t, diff.HasChanges())
}

func TestInMemoryManager_DiffSnapshots_NotFound(t *testing.T) {
	ctx := context.Background()
	m := NewInMemoryManager(10)

	s := &StateSnapshot{DeploymentID: "deploy", ConfigHash: "hash"}
	_ = m.SaveSnapshot(ctx, s)

	_, err := m.DiffSnapshots(ctx, "nonexistent", s.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "source snapshot")

	_, err = m.DiffSnapshots(ctx, s.ID, "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "target snapshot")
}

func TestInMemoryManager_DeleteSnapshot(t *testing.T) {
	ctx := context.Background()
	m := NewInMemoryManager(10)

	s := &StateSnapshot{DeploymentID: "deploy", ConfigHash: "hash"}
	_ = m.SaveSnapshot(ctx, s)

	err := m.DeleteSnapshot(ctx, s.ID)
	require.NoError(t, err)

	_, err = m.GetSnapshot(ctx, s.ID)
	assert.Error(t, err)
}

func TestInMemoryManager_DeleteSnapshot_NotFound(t *testing.T) {
	ctx := context.Background()
	m := NewInMemoryManager(10)

	err := m.DeleteSnapshot(ctx, "nonexistent")
	assert.Error(t, err)
}

func TestInMemoryManager_Rollback(t *testing.T) {
	ctx := context.Background()
	m := NewInMemoryManager(10)

	s1 := &StateSnapshot{
		DeploymentID: "deploy",
		ConfigHash:   "config-1",
		NodeCount:    3,
	}
	_ = m.SaveSnapshot(ctx, s1)

	s2 := &StateSnapshot{
		DeploymentID: "deploy",
		ConfigHash:   "config-2",
		NodeCount:    5,
	}
	_ = m.SaveSnapshot(ctx, s2)

	newSnapshot, err := m.Rollback(ctx, s1.ID)
	require.NoError(t, err)

	assert.NotEqual(t, s1.ID, newSnapshot.ID)
	assert.Equal(t, s1.ConfigHash, newSnapshot.ConfigHash)
	assert.Equal(t, s1.NodeCount, newSnapshot.NodeCount)
	assert.Equal(t, s1.ID, newSnapshot.ParentID)
	assert.Equal(t, SnapshotStatusActive, newSnapshot.Status)

	// Original should be marked as rolled back
	original, _ := m.GetSnapshot(ctx, s1.ID)
	assert.Equal(t, SnapshotStatusRolledBack, original.Status)
}

func TestInMemoryManager_Rollback_NotFound(t *testing.T) {
	ctx := context.Background()
	m := NewInMemoryManager(10)

	_, err := m.Rollback(ctx, "nonexistent")
	assert.Error(t, err)
}

func TestInMemoryManager_GetRollbackPoints(t *testing.T) {
	ctx := context.Background()
	m := NewInMemoryManager(10)

	for i := 0; i < 5; i++ {
		s := &StateSnapshot{DeploymentID: "deploy", ConfigHash: string(rune('a' + i))}
		_ = m.SaveSnapshot(ctx, s)
	}

	points, err := m.GetRollbackPoints(ctx, 3)
	require.NoError(t, err)
	assert.Len(t, points, 3)

	// All should be archived (not active)
	for _, p := range points {
		assert.NotEqual(t, SnapshotStatusActive, p.Status)
	}
}

func TestInMemoryManager_Export_Import(t *testing.T) {
	ctx := context.Background()
	m1 := NewInMemoryManager(10)

	_ = m1.SaveSnapshot(ctx, &StateSnapshot{
		DeploymentID: "deploy-1",
		ConfigHash:   "hash-1",
		NodeCount:    3,
	})
	_ = m1.SaveSnapshot(ctx, &StateSnapshot{
		DeploymentID: "deploy-2",
		ConfigHash:   "hash-2",
		NodeCount:    5,
	})

	export, err := m1.Export(ctx)
	require.NoError(t, err)
	assert.Equal(t, "1.0", export.Version)
	assert.Len(t, export.Snapshots, 2)

	m2 := NewInMemoryManager(10)
	err = m2.Import(ctx, export)
	require.NoError(t, err)

	list, _ := m2.ListSnapshots(ctx, ListOptions{})
	assert.Len(t, list, 2)
}

func TestInMemoryManager_Import_Nil(t *testing.T) {
	ctx := context.Background()
	m := NewInMemoryManager(10)

	err := m.Import(ctx, nil)
	assert.Error(t, err)
}

func TestInMemoryManager_ToJSON_FromJSON(t *testing.T) {
	ctx := context.Background()
	m1 := NewInMemoryManager(10)

	_ = m1.SaveSnapshot(ctx, &StateSnapshot{
		DeploymentID: "deploy",
		ConfigHash:   "hash",
		NodeCount:    3,
	})

	jsonData, err := m1.ToJSON()
	require.NoError(t, err)
	assert.NotEmpty(t, jsonData)

	var export StateExport
	err = json.Unmarshal(jsonData, &export)
	require.NoError(t, err)

	m2 := NewInMemoryManager(10)
	err = m2.FromJSON(jsonData)
	require.NoError(t, err)

	list, _ := m2.ListSnapshots(context.Background(), ListOptions{})
	assert.Len(t, list, 1)
}

func TestInMemoryManager_FromJSON_Invalid(t *testing.T) {
	m := NewInMemoryManager(10)

	err := m.FromJSON([]byte("invalid json"))
	assert.Error(t, err)
}

func TestInMemoryManager_Trimming(t *testing.T) {
	ctx := context.Background()
	m := NewInMemoryManager(3)

	for i := 0; i < 5; i++ {
		s := &StateSnapshot{DeploymentID: "deploy", ConfigHash: string(rune('a' + i))}
		_ = m.SaveSnapshot(ctx, s)
	}

	list, _ := m.ListSnapshots(ctx, ListOptions{})
	assert.Len(t, list, 3)
}

func TestStateDiff_HasChanges(t *testing.T) {
	tests := []struct {
		name string
		diff *StateDiff
		want bool
	}{
		{
			name: "no changes",
			diff: &StateDiff{},
			want: false,
		},
		{
			name: "config changed",
			diff: &StateDiff{ConfigChanged: true},
			want: true,
		},
		{
			name: "manifest changed",
			diff: &StateDiff{ManifestChanged: true},
			want: true,
		},
		{
			name: "node count changed",
			diff: &StateDiff{NodeCountDelta: 2},
			want: true,
		},
		{
			name: "pool changes",
			diff: &StateDiff{PoolChanges: []PoolChange{{}}},
			want: true,
		},
		{
			name: "resource changes",
			diff: &StateDiff{ResourceChanges: []ResourceChange{{}}},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.diff.HasChanges())
		})
	}
}

func TestCalculatePoolChanges(t *testing.T) {
	from := map[string]int{
		"workers":   3,
		"removed":   2,
		"unchanged": 1,
	}
	to := map[string]int{
		"workers":   5,
		"added":     2,
		"unchanged": 1,
	}

	changes := calculatePoolChanges(from, to)

	assert.Len(t, changes, 3)

	changeMap := make(map[string]PoolChange)
	for _, c := range changes {
		changeMap[c.PoolName] = c
	}

	assert.Equal(t, "scaled", changeMap["workers"].Action)
	assert.Equal(t, 3, changeMap["workers"].OldCount)
	assert.Equal(t, 5, changeMap["workers"].NewCount)

	assert.Equal(t, "added", changeMap["added"].Action)
	assert.Equal(t, "removed", changeMap["removed"].Action)
}

func TestCalculateResourceChanges(t *testing.T) {
	from := []ResourceState{
		{Type: "node", Name: "node-1", ID: "id-1", Status: "running"},
		{Type: "node", Name: "deleted", ID: "id-2", Status: "running"},
		{Type: "vpc", Name: "vpc-1", ID: "id-3", Status: "active"},
	}
	to := []ResourceState{
		{Type: "node", Name: "node-1", ID: "id-1", Status: "stopped"},  // updated
		{Type: "node", Name: "created", ID: "id-4", Status: "running"}, // created
		{Type: "vpc", Name: "vpc-1", ID: "id-3", Status: "active"},     // unchanged
	}

	changes := calculateResourceChanges(from, to)

	assert.Len(t, changes, 3)

	changeMap := make(map[string]ResourceChange)
	for _, c := range changes {
		key := c.ResourceType + "/" + c.ResourceName
		changeMap[key] = c
	}

	assert.Equal(t, "updated", changeMap["node/node-1"].Action)
	assert.Equal(t, "created", changeMap["node/created"].Action)
	assert.Equal(t, "deleted", changeMap["node/deleted"].Action)
}

func TestSnapshotStatusConstants(t *testing.T) {
	assert.Equal(t, SnapshotStatus("active"), SnapshotStatusActive)
	assert.Equal(t, SnapshotStatus("archived"), SnapshotStatusArchived)
	assert.Equal(t, SnapshotStatus("rolled_back"), SnapshotStatusRolledBack)
}
