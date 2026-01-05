// Package state provides state management capabilities for tracking
// deployment state, snapshots, and enabling rollback functionality.
package state

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"
)

// SnapshotStatus represents the status of a snapshot
type SnapshotStatus string

const (
	// SnapshotStatusActive means this is the current active state
	SnapshotStatusActive SnapshotStatus = "active"
	// SnapshotStatusArchived means this snapshot is archived
	SnapshotStatusArchived SnapshotStatus = "archived"
	// SnapshotStatusRolledBack means this snapshot was used for rollback
	SnapshotStatusRolledBack SnapshotStatus = "rolled_back"
)

// StateSnapshot represents a point-in-time snapshot of deployment state
type StateSnapshot struct {
	// ID is a unique identifier for this snapshot
	ID string `json:"id"`
	// DeploymentID is the deployment this snapshot belongs to
	DeploymentID string `json:"deployment_id"`
	// Timestamp is when this snapshot was created
	Timestamp time.Time `json:"timestamp"`
	// ConfigHash is the SHA256 hash of the configuration
	ConfigHash string `json:"config_hash"`
	// ManifestHash is the SHA256 hash of all manifests
	ManifestHash string `json:"manifest_hash"`
	// Status is the current status of this snapshot
	Status SnapshotStatus `json:"status"`
	// NodeCount is the number of nodes at this point
	NodeCount int `json:"node_count"`
	// NodePools maps pool names to their node counts
	NodePools map[string]int `json:"node_pools"`
	// Metadata holds additional snapshot metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	// Resources lists the resources in this snapshot
	Resources []ResourceState `json:"resources,omitempty"`
	// ParentID is the ID of the parent snapshot (if any)
	ParentID string `json:"parent_id,omitempty"`
}

// ResourceState represents the state of a single resource
type ResourceState struct {
	// Type is the resource type (e.g., "node", "vpc", "firewall")
	Type string `json:"type"`
	// Name is the resource name
	Name string `json:"name"`
	// ID is the resource ID
	ID string `json:"id"`
	// Provider is the cloud provider
	Provider string `json:"provider"`
	// Region is the resource region
	Region string `json:"region,omitempty"`
	// Status is the resource status
	Status string `json:"status"`
	// Properties holds resource-specific properties
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// StateDiff represents the difference between two snapshots
type StateDiff struct {
	// FromSnapshot is the source snapshot ID
	FromSnapshot string `json:"from_snapshot"`
	// ToSnapshot is the target snapshot ID
	ToSnapshot string `json:"to_snapshot"`
	// ConfigChanged indicates if the configuration changed
	ConfigChanged bool `json:"config_changed"`
	// ManifestChanged indicates if manifests changed
	ManifestChanged bool `json:"manifest_changed"`
	// NodeCountDelta is the change in node count
	NodeCountDelta int `json:"node_count_delta"`
	// PoolChanges lists changes to node pools
	PoolChanges []PoolChange `json:"pool_changes,omitempty"`
	// ResourceChanges lists changes to resources
	ResourceChanges []ResourceChange `json:"resource_changes,omitempty"`
	// TimeDelta is the time between snapshots
	TimeDelta time.Duration `json:"time_delta"`
}

// PoolChange represents a change to a node pool
type PoolChange struct {
	PoolName string `json:"pool_name"`
	Action   string `json:"action"` // added, removed, scaled
	OldCount int    `json:"old_count"`
	NewCount int    `json:"new_count"`
}

// ResourceChange represents a change to a resource
type ResourceChange struct {
	ResourceType string                 `json:"resource_type"`
	ResourceName string                 `json:"resource_name"`
	Action       string                 `json:"action"` // created, updated, deleted
	OldState     map[string]interface{} `json:"old_state,omitempty"`
	NewState     map[string]interface{} `json:"new_state,omitempty"`
}

// HasChanges returns true if there are any changes
func (d *StateDiff) HasChanges() bool {
	return d.ConfigChanged || d.ManifestChanged || d.NodeCountDelta != 0 ||
		len(d.PoolChanges) > 0 || len(d.ResourceChanges) > 0
}

// Manager provides state management capabilities
type Manager interface {
	// SaveSnapshot saves a new state snapshot
	SaveSnapshot(ctx context.Context, snapshot *StateSnapshot) error

	// GetSnapshot retrieves a snapshot by ID
	GetSnapshot(ctx context.Context, id string) (*StateSnapshot, error)

	// GetLatestSnapshot returns the most recent snapshot
	GetLatestSnapshot(ctx context.Context) (*StateSnapshot, error)

	// ListSnapshots returns snapshots, optionally filtered
	ListSnapshots(ctx context.Context, opts ListOptions) ([]*StateSnapshot, error)

	// DiffSnapshots compares two snapshots
	DiffSnapshots(ctx context.Context, fromID, toID string) (*StateDiff, error)

	// DeleteSnapshot removes a snapshot
	DeleteSnapshot(ctx context.Context, id string) error

	// Rollback reverts to a previous snapshot
	Rollback(ctx context.Context, toSnapshotID string) (*StateSnapshot, error)

	// GetRollbackPoints returns snapshots that can be used for rollback
	GetRollbackPoints(ctx context.Context, limit int) ([]*StateSnapshot, error)

	// Export exports all state data
	Export(ctx context.Context) (*StateExport, error)

	// Import imports state data
	Import(ctx context.Context, data *StateExport) error
}

// ListOptions provides filtering options for listing snapshots
type ListOptions struct {
	// Limit is the maximum number of snapshots to return
	Limit int
	// Since returns only snapshots after this time
	Since *time.Time
	// Until returns only snapshots before this time
	Until *time.Time
	// Status filters by snapshot status
	Status *SnapshotStatus
	// DeploymentID filters by deployment ID
	DeploymentID string
}

// StateExport is the exportable format of state data
type StateExport struct {
	Version    string           `json:"version"`
	ExportedAt time.Time        `json:"exported_at"`
	Snapshots  []*StateSnapshot `json:"snapshots"`
}

// InMemoryManager is an in-memory implementation of Manager
type InMemoryManager struct {
	mu        sync.RWMutex
	snapshots map[string]*StateSnapshot
	order     []string // Ordered list of snapshot IDs
	maxSize   int
}

// NewInMemoryManager creates a new in-memory state manager
func NewInMemoryManager(maxSize int) *InMemoryManager {
	if maxSize <= 0 {
		maxSize = 100
	}
	return &InMemoryManager{
		snapshots: make(map[string]*StateSnapshot),
		order:     make([]string, 0),
		maxSize:   maxSize,
	}
}

// SaveSnapshot saves a new state snapshot
func (m *InMemoryManager) SaveSnapshot(ctx context.Context, snapshot *StateSnapshot) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if snapshot == nil {
		return fmt.Errorf("snapshot cannot be nil")
	}

	if snapshot.ID == "" {
		snapshot.ID = generateSnapshotID(snapshot)
	}

	if snapshot.Timestamp.IsZero() {
		snapshot.Timestamp = time.Now().UTC()
	}

	// Mark previous active snapshot as archived
	for _, s := range m.snapshots {
		if s.Status == SnapshotStatusActive {
			s.Status = SnapshotStatusArchived
		}
	}

	snapshot.Status = SnapshotStatusActive
	m.snapshots[snapshot.ID] = snapshot
	m.order = append(m.order, snapshot.ID)

	// Trim if needed
	if len(m.order) > m.maxSize {
		oldID := m.order[0]
		m.order = m.order[1:]
		delete(m.snapshots, oldID)
	}

	return nil
}

// GetSnapshot retrieves a snapshot by ID
func (m *InMemoryManager) GetSnapshot(ctx context.Context, id string) (*StateSnapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshot, exists := m.snapshots[id]
	if !exists {
		return nil, fmt.Errorf("snapshot %s not found", id)
	}

	return snapshot, nil
}

// GetLatestSnapshot returns the most recent snapshot
func (m *InMemoryManager) GetLatestSnapshot(ctx context.Context) (*StateSnapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.order) == 0 {
		return nil, fmt.Errorf("no snapshots available")
	}

	latestID := m.order[len(m.order)-1]
	return m.snapshots[latestID], nil
}

// ListSnapshots returns snapshots, optionally filtered
func (m *InMemoryManager) ListSnapshots(ctx context.Context, opts ListOptions) ([]*StateSnapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*StateSnapshot

	for _, id := range m.order {
		s := m.snapshots[id]

		// Apply filters
		if opts.Since != nil && s.Timestamp.Before(*opts.Since) {
			continue
		}
		if opts.Until != nil && s.Timestamp.After(*opts.Until) {
			continue
		}
		if opts.Status != nil && s.Status != *opts.Status {
			continue
		}
		if opts.DeploymentID != "" && s.DeploymentID != opts.DeploymentID {
			continue
		}

		result = append(result, s)
	}

	// Apply limit
	if opts.Limit > 0 && len(result) > opts.Limit {
		result = result[len(result)-opts.Limit:]
	}

	return result, nil
}

// DiffSnapshots compares two snapshots
func (m *InMemoryManager) DiffSnapshots(ctx context.Context, fromID, toID string) (*StateDiff, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	from, exists := m.snapshots[fromID]
	if !exists {
		return nil, fmt.Errorf("source snapshot %s not found", fromID)
	}

	to, exists := m.snapshots[toID]
	if !exists {
		return nil, fmt.Errorf("target snapshot %s not found", toID)
	}

	diff := &StateDiff{
		FromSnapshot:    fromID,
		ToSnapshot:      toID,
		ConfigChanged:   from.ConfigHash != to.ConfigHash,
		ManifestChanged: from.ManifestHash != to.ManifestHash,
		NodeCountDelta:  to.NodeCount - from.NodeCount,
		TimeDelta:       to.Timestamp.Sub(from.Timestamp),
	}

	// Calculate pool changes
	diff.PoolChanges = calculatePoolChanges(from.NodePools, to.NodePools)

	// Calculate resource changes
	diff.ResourceChanges = calculateResourceChanges(from.Resources, to.Resources)

	return diff, nil
}

// DeleteSnapshot removes a snapshot
func (m *InMemoryManager) DeleteSnapshot(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.snapshots[id]; !exists {
		return fmt.Errorf("snapshot %s not found", id)
	}

	delete(m.snapshots, id)

	// Remove from order
	newOrder := make([]string, 0, len(m.order)-1)
	for _, oid := range m.order {
		if oid != id {
			newOrder = append(newOrder, oid)
		}
	}
	m.order = newOrder

	return nil
}

// Rollback reverts to a previous snapshot
func (m *InMemoryManager) Rollback(ctx context.Context, toSnapshotID string) (*StateSnapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	target, exists := m.snapshots[toSnapshotID]
	if !exists {
		return nil, fmt.Errorf("target snapshot %s not found", toSnapshotID)
	}

	// Mark current active as archived
	for _, s := range m.snapshots {
		if s.Status == SnapshotStatusActive {
			s.Status = SnapshotStatusArchived
		}
	}

	// Mark target as rolled back (create a copy)
	target.Status = SnapshotStatusRolledBack

	// Create new snapshot based on target
	newSnapshot := &StateSnapshot{
		ID:           generateSnapshotID(target),
		DeploymentID: target.DeploymentID,
		Timestamp:    time.Now().UTC(),
		ConfigHash:   target.ConfigHash,
		ManifestHash: target.ManifestHash,
		Status:       SnapshotStatusActive,
		NodeCount:    target.NodeCount,
		NodePools:    target.NodePools,
		Metadata:     target.Metadata,
		Resources:    target.Resources,
		ParentID:     toSnapshotID,
	}

	m.snapshots[newSnapshot.ID] = newSnapshot
	m.order = append(m.order, newSnapshot.ID)

	return newSnapshot, nil
}

// GetRollbackPoints returns snapshots that can be used for rollback
func (m *InMemoryManager) GetRollbackPoints(ctx context.Context, limit int) ([]*StateSnapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*StateSnapshot

	// Iterate in reverse order (most recent first)
	for i := len(m.order) - 1; i >= 0; i-- {
		s := m.snapshots[m.order[i]]
		if s.Status != SnapshotStatusActive { // Don't include current active
			result = append(result, s)
		}
		if limit > 0 && len(result) >= limit {
			break
		}
	}

	return result, nil
}

// Export exports all state data
func (m *InMemoryManager) Export(ctx context.Context) (*StateExport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshots := make([]*StateSnapshot, 0, len(m.order))
	for _, id := range m.order {
		snapshots = append(snapshots, m.snapshots[id])
	}

	return &StateExport{
		Version:    "1.0",
		ExportedAt: time.Now().UTC(),
		Snapshots:  snapshots,
	}, nil
}

// Import imports state data
func (m *InMemoryManager) Import(ctx context.Context, data *StateExport) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if data == nil {
		return fmt.Errorf("import data cannot be nil")
	}

	m.snapshots = make(map[string]*StateSnapshot)
	m.order = make([]string, 0, len(data.Snapshots))

	for _, s := range data.Snapshots {
		m.snapshots[s.ID] = s
		m.order = append(m.order, s.ID)
	}

	// Sort by timestamp
	sort.Slice(m.order, func(i, j int) bool {
		return m.snapshots[m.order[i]].Timestamp.Before(m.snapshots[m.order[j]].Timestamp)
	})

	return nil
}

// ToJSON serializes the manager state to JSON
func (m *InMemoryManager) ToJSON() ([]byte, error) {
	export, err := m.Export(context.Background())
	if err != nil {
		return nil, err
	}
	return json.Marshal(export)
}

// FromJSON deserializes the manager state from JSON
func (m *InMemoryManager) FromJSON(data []byte) error {
	var export StateExport
	if err := json.Unmarshal(data, &export); err != nil {
		return fmt.Errorf("failed to unmarshal state: %w", err)
	}
	return m.Import(context.Background(), &export)
}

// generateSnapshotID generates a unique snapshot ID
func generateSnapshotID(s *StateSnapshot) string {
	data := fmt.Sprintf("%s-%s-%s-%d",
		s.DeploymentID,
		s.ConfigHash,
		s.ManifestHash,
		time.Now().UnixNano(),
	)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])[:16]
}

// calculatePoolChanges calculates changes between two pool configurations
func calculatePoolChanges(from, to map[string]int) []PoolChange {
	var changes []PoolChange

	allPools := make(map[string]bool)
	for k := range from {
		allPools[k] = true
	}
	for k := range to {
		allPools[k] = true
	}

	for pool := range allPools {
		oldCount := from[pool]
		newCount := to[pool]

		if oldCount == 0 && newCount > 0 {
			changes = append(changes, PoolChange{
				PoolName: pool,
				Action:   "added",
				OldCount: oldCount,
				NewCount: newCount,
			})
		} else if oldCount > 0 && newCount == 0 {
			changes = append(changes, PoolChange{
				PoolName: pool,
				Action:   "removed",
				OldCount: oldCount,
				NewCount: newCount,
			})
		} else if oldCount != newCount {
			changes = append(changes, PoolChange{
				PoolName: pool,
				Action:   "scaled",
				OldCount: oldCount,
				NewCount: newCount,
			})
		}
	}

	return changes
}

// calculateResourceChanges calculates changes between two resource lists
func calculateResourceChanges(from, to []ResourceState) []ResourceChange {
	var changes []ResourceChange

	fromMap := make(map[string]ResourceState)
	for _, r := range from {
		key := fmt.Sprintf("%s/%s", r.Type, r.Name)
		fromMap[key] = r
	}

	toMap := make(map[string]ResourceState)
	for _, r := range to {
		key := fmt.Sprintf("%s/%s", r.Type, r.Name)
		toMap[key] = r
	}

	// Find created and updated
	for key, toRes := range toMap {
		if fromRes, exists := fromMap[key]; !exists {
			changes = append(changes, ResourceChange{
				ResourceType: toRes.Type,
				ResourceName: toRes.Name,
				Action:       "created",
				NewState:     toRes.Properties,
			})
		} else if !resourcesEqual(fromRes, toRes) {
			changes = append(changes, ResourceChange{
				ResourceType: toRes.Type,
				ResourceName: toRes.Name,
				Action:       "updated",
				OldState:     fromRes.Properties,
				NewState:     toRes.Properties,
			})
		}
	}

	// Find deleted
	for key, fromRes := range fromMap {
		if _, exists := toMap[key]; !exists {
			changes = append(changes, ResourceChange{
				ResourceType: fromRes.Type,
				ResourceName: fromRes.Name,
				Action:       "deleted",
				OldState:     fromRes.Properties,
			})
		}
	}

	return changes
}

// resourcesEqual checks if two resources are equal
func resourcesEqual(a, b ResourceState) bool {
	if a.ID != b.ID || a.Status != b.Status || a.Region != b.Region {
		return false
	}
	// Simple property comparison via JSON
	aJSON, _ := json.Marshal(a.Properties)
	bJSON, _ := json.Marshal(b.Properties)
	return string(aJSON) == string(bJSON)
}
