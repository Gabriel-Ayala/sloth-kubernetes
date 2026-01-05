// Package manifests provides manifest versioning and registry capabilities
// for tracking Kubernetes manifests and their lifecycle.
package manifests

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"
)

// ManifestType represents the type of manifest
type ManifestType string

const (
	// ManifestTypeRKE is an RKE cluster configuration
	ManifestTypeRKE ManifestType = "rke"
	// ManifestTypeRKE2 is an RKE2 cluster configuration
	ManifestTypeRKE2 ManifestType = "rke2"
	// ManifestTypeK3s is a K3s cluster configuration
	ManifestTypeK3s ManifestType = "k3s"
	// ManifestTypeArgoCD is an ArgoCD application
	ManifestTypeArgoCD ManifestType = "argocd"
	// ManifestTypeHelm is a Helm release
	ManifestTypeHelm ManifestType = "helm"
	// ManifestTypeKustomize is a Kustomize configuration
	ManifestTypeKustomize ManifestType = "kustomize"
	// ManifestTypeRaw is a raw Kubernetes manifest
	ManifestTypeRaw ManifestType = "raw"
)

// ManifestStatus represents the status of a manifest
type ManifestStatus string

const (
	// StatusPending means the manifest is registered but not applied
	StatusPending ManifestStatus = "pending"
	// StatusApplied means the manifest has been applied successfully
	StatusApplied ManifestStatus = "applied"
	// StatusFailed means the manifest application failed
	StatusFailed ManifestStatus = "failed"
	// StatusDeleted means the manifest has been deleted
	StatusDeleted ManifestStatus = "deleted"
	// StatusOutOfSync means the manifest differs from the live state
	StatusOutOfSync ManifestStatus = "out_of_sync"
)

// Manifest represents a versioned manifest
type Manifest struct {
	// ID is a unique identifier for this manifest
	ID string `json:"id"`
	// Name is the human-readable name
	Name string `json:"name"`
	// Type is the type of manifest
	Type ManifestType `json:"type"`
	// Version is the semantic version or hash
	Version string `json:"version"`
	// Hash is the SHA256 hash of the content
	Hash string `json:"hash"`
	// Content is the YAML/JSON content
	Content string `json:"content"`
	// Status is the current status
	Status ManifestStatus `json:"status"`
	// CreatedAt is when this manifest was registered
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is when this manifest was last updated
	UpdatedAt time.Time `json:"updated_at"`
	// AppliedAt is when this manifest was applied
	AppliedAt *time.Time `json:"applied_at,omitempty"`
	// Metadata holds additional metadata
	Metadata map[string]string `json:"metadata,omitempty"`
	// Dependencies lists manifest IDs this depends on
	Dependencies []string `json:"dependencies,omitempty"`
	// ParentHash is the hash of the previous version
	ParentHash string `json:"parent_hash,omitempty"`
}

// ManifestVersion represents a version of a manifest in history
type ManifestVersion struct {
	Hash      string    `json:"hash"`
	Version   string    `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	Status    ManifestStatus `json:"status"`
	Content   string    `json:"content,omitempty"` // Optional, for rollback
}

// ManifestHistory tracks the version history of a manifest
type ManifestHistory struct {
	ManifestID string            `json:"manifest_id"`
	Versions   []ManifestVersion `json:"versions"`
	MaxSize    int               `json:"max_size"`
}

// Registry manages manifest registration and versioning
type Registry struct {
	mu        sync.RWMutex
	manifests map[string]*Manifest
	history   map[string]*ManifestHistory
	maxHistory int
}

// NewRegistry creates a new manifest registry
func NewRegistry(maxHistory int) *Registry {
	if maxHistory <= 0 {
		maxHistory = 50
	}
	return &Registry{
		manifests:  make(map[string]*Manifest),
		history:    make(map[string]*ManifestHistory),
		maxHistory: maxHistory,
	}
}

// Register registers a new manifest or updates an existing one
func (r *Registry) Register(name string, manifestType ManifestType, content string, metadata map[string]string) (*Manifest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if name == "" {
		return nil, fmt.Errorf("manifest name cannot be empty")
	}
	if content == "" {
		return nil, fmt.Errorf("manifest content cannot be empty")
	}

	hash := computeHash(content)
	id := fmt.Sprintf("%s-%s", name, hash[:8])
	now := time.Now().UTC()

	// Check if manifest already exists with same hash
	if existing, exists := r.manifests[name]; exists {
		if existing.Hash == hash {
			// No changes, return existing
			return existing, nil
		}
		// Update existing manifest
		existing.ParentHash = existing.Hash
		existing.Hash = hash
		existing.Content = content
		existing.UpdatedAt = now
		existing.Status = StatusPending
		existing.Version = r.nextVersion(name)
		if metadata != nil {
			existing.Metadata = metadata
		}

		r.addToHistory(name, existing)
		return existing, nil
	}

	// Create new manifest
	manifest := &Manifest{
		ID:        id,
		Name:      name,
		Type:      manifestType,
		Version:   "v1",
		Hash:      hash,
		Content:   content,
		Status:    StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
		Metadata:  metadata,
	}

	r.manifests[name] = manifest
	r.addToHistory(name, manifest)

	return manifest, nil
}

// Get retrieves a manifest by name
func (r *Registry) Get(name string) (*Manifest, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	m, exists := r.manifests[name]
	return m, exists
}

// GetByHash retrieves a manifest by its hash
func (r *Registry) GetByHash(hash string) (*Manifest, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, m := range r.manifests {
		if m.Hash == hash {
			return m, true
		}
	}
	return nil, false
}

// List returns all registered manifests
func (r *Registry) List() []*Manifest {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Manifest, 0, len(r.manifests))
	for _, m := range r.manifests {
		result = append(result, m)
	}

	// Sort by name for consistent ordering
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result
}

// ListByType returns manifests of a specific type
func (r *Registry) ListByType(manifestType ManifestType) []*Manifest {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*Manifest
	for _, m := range r.manifests {
		if m.Type == manifestType {
			result = append(result, m)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result
}

// ListByStatus returns manifests with a specific status
func (r *Registry) ListByStatus(status ManifestStatus) []*Manifest {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*Manifest
	for _, m := range r.manifests {
		if m.Status == status {
			result = append(result, m)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result
}

// UpdateStatus updates the status of a manifest
func (r *Registry) UpdateStatus(name string, status ManifestStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	m, exists := r.manifests[name]
	if !exists {
		return fmt.Errorf("manifest %s not found", name)
	}

	m.Status = status
	m.UpdatedAt = time.Now().UTC()

	if status == StatusApplied {
		now := time.Now().UTC()
		m.AppliedAt = &now
	}

	return nil
}

// Delete removes a manifest from the registry
func (r *Registry) Delete(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.manifests[name]; !exists {
		return fmt.Errorf("manifest %s not found", name)
	}

	// Mark as deleted in history before removing
	if m, ok := r.manifests[name]; ok {
		m.Status = StatusDeleted
		r.addToHistory(name, m)
	}

	delete(r.manifests, name)
	return nil
}

// GetHistory returns the version history of a manifest
func (r *Registry) GetHistory(name string) (*ManifestHistory, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	h, exists := r.history[name]
	return h, exists
}

// GetVersionFromHistory retrieves a specific version from history
func (r *Registry) GetVersionFromHistory(name, hash string) (*ManifestVersion, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	h, exists := r.history[name]
	if !exists {
		return nil, false
	}

	for _, v := range h.Versions {
		if v.Hash == hash {
			return &v, true
		}
	}

	return nil, false
}

// ComputeRegistryHash computes a combined hash of all manifests
func (r *Registry) ComputeRegistryHash() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var hashes []string
	for _, m := range r.manifests {
		hashes = append(hashes, m.Hash)
	}
	sort.Strings(hashes)

	combined := ""
	for _, h := range hashes {
		combined += h
	}

	return computeHash(combined)
}

// Diff compares two registries and returns the differences
func (r *Registry) Diff(other *Registry) *RegistryDiff {
	r.mu.RLock()
	defer r.mu.RUnlock()

	diff := &RegistryDiff{
		Added:    make([]*Manifest, 0),
		Removed:  make([]*Manifest, 0),
		Modified: make([]*ManifestChange, 0),
	}

	otherManifests := make(map[string]*Manifest)
	for _, m := range other.List() {
		otherManifests[m.Name] = m
	}

	// Find added and modified
	for _, m := range r.manifests {
		if om, exists := otherManifests[m.Name]; !exists {
			diff.Added = append(diff.Added, m)
		} else if m.Hash != om.Hash {
			diff.Modified = append(diff.Modified, &ManifestChange{
				Name:    m.Name,
				OldHash: om.Hash,
				NewHash: m.Hash,
				OldVersion: om.Version,
				NewVersion: m.Version,
			})
		}
	}

	// Find removed
	for name, m := range otherManifests {
		if _, exists := r.manifests[name]; !exists {
			diff.Removed = append(diff.Removed, m)
		}
	}

	return diff
}

// Export exports the registry to a serializable format
func (r *Registry) Export() (*RegistryExport, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	export := &RegistryExport{
		Version:    "1.0",
		ExportedAt: time.Now().UTC(),
		Hash:       r.ComputeRegistryHash(),
		Manifests:  make(map[string]*Manifest),
		History:    make(map[string]*ManifestHistory),
	}

	for k, v := range r.manifests {
		export.Manifests[k] = v
	}

	for k, v := range r.history {
		export.History[k] = v
	}

	return export, nil
}

// Import imports a registry from an exported format
func (r *Registry) Import(export *RegistryExport) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if export == nil {
		return fmt.Errorf("export cannot be nil")
	}

	r.manifests = make(map[string]*Manifest)
	r.history = make(map[string]*ManifestHistory)

	for k, v := range export.Manifests {
		r.manifests[k] = v
	}

	for k, v := range export.History {
		r.history[k] = v
	}

	return nil
}

// ToJSON serializes the registry to JSON
func (r *Registry) ToJSON() ([]byte, error) {
	export, err := r.Export()
	if err != nil {
		return nil, err
	}
	return json.Marshal(export)
}

// FromJSON deserializes the registry from JSON
func (r *Registry) FromJSON(data []byte) error {
	var export RegistryExport
	if err := json.Unmarshal(data, &export); err != nil {
		return fmt.Errorf("failed to unmarshal registry: %w", err)
	}
	return r.Import(&export)
}

// addToHistory adds a manifest version to history
func (r *Registry) addToHistory(name string, m *Manifest) {
	h, exists := r.history[name]
	if !exists {
		h = &ManifestHistory{
			ManifestID: name,
			Versions:   make([]ManifestVersion, 0),
			MaxSize:    r.maxHistory,
		}
		r.history[name] = h
	}

	version := ManifestVersion{
		Hash:      m.Hash,
		Version:   m.Version,
		CreatedAt: time.Now().UTC(),
		Status:    m.Status,
		Content:   m.Content,
	}

	h.Versions = append(h.Versions, version)

	// Trim history if needed
	if len(h.Versions) > h.MaxSize {
		h.Versions = h.Versions[len(h.Versions)-h.MaxSize:]
	}
}

// nextVersion generates the next version string
func (r *Registry) nextVersion(name string) string {
	h, exists := r.history[name]
	if !exists {
		return "v1"
	}
	return fmt.Sprintf("v%d", len(h.Versions)+1)
}

// RegistryDiff represents the difference between two registries
type RegistryDiff struct {
	Added    []*Manifest       `json:"added"`
	Removed  []*Manifest       `json:"removed"`
	Modified []*ManifestChange `json:"modified"`
}

// HasChanges returns true if there are any changes
func (d *RegistryDiff) HasChanges() bool {
	return len(d.Added) > 0 || len(d.Removed) > 0 || len(d.Modified) > 0
}

// ManifestChange represents a change to a manifest
type ManifestChange struct {
	Name       string `json:"name"`
	OldHash    string `json:"old_hash"`
	NewHash    string `json:"new_hash"`
	OldVersion string `json:"old_version"`
	NewVersion string `json:"new_version"`
}

// RegistryExport is the exportable format of a registry
type RegistryExport struct {
	Version    string                      `json:"version"`
	ExportedAt time.Time                   `json:"exported_at"`
	Hash       string                      `json:"hash"`
	Manifests  map[string]*Manifest        `json:"manifests"`
	History    map[string]*ManifestHistory `json:"history"`
}

// computeHash computes SHA256 hash of content
func computeHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}
