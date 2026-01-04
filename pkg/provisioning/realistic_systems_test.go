package provisioning_test

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/chalkan3/sloth-kubernetes/pkg/providers"
	"github.com/chalkan3/sloth-kubernetes/pkg/provisioning"
	"github.com/chalkan3/sloth-kubernetes/pkg/provisioning/backup"
	"github.com/chalkan3/sloth-kubernetes/pkg/provisioning/hooks"
	"github.com/chalkan3/sloth-kubernetes/pkg/provisioning/taints"
	"github.com/chalkan3/sloth-kubernetes/pkg/provisioning/upgrades"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// REALISTIC KUBERNETES API SIMULATOR
// =============================================================================

// K8sAPISimulator simulates Kubernetes API behavior
type K8sAPISimulator struct {
	mu sync.RWMutex

	// Node state
	nodes map[string]*SimulatedNode

	// API behavior config
	apiLatency     time.Duration
	apiErrorRate   float64
	maxConcurrency int

	// Rate limiting
	requestCount  int64
	requestWindow time.Time
	rateLimitMu   sync.Mutex

	// Metrics
	totalRequests int64
	failedReqs    int64
	throttledReqs int64

	rng *rand.Rand
}

// SimulatedNode represents a K8s node with full state
type SimulatedNode struct {
	Name            string
	Ready           bool
	Schedulable     bool
	Taints          []*taints.Taint
	Labels          map[string]string
	Version         string
	PodCount        int
	Conditions      map[string]string
	CreatedAt       time.Time
	LastHeartbeat   time.Time
	DrainInProgress bool
	Cordoned        bool
}

// K8sAPIConfig configures the simulator
type K8sAPIConfig struct {
	APILatency     time.Duration
	APIErrorRate   float64
	MaxConcurrency int
}

// NewK8sAPISimulator creates a new K8s API simulator
func NewK8sAPISimulator(cfg *K8sAPIConfig) *K8sAPISimulator {
	if cfg == nil {
		cfg = &K8sAPIConfig{
			APILatency:     20 * time.Millisecond,
			APIErrorRate:   0.01,
			MaxConcurrency: 100,
		}
	}

	return &K8sAPISimulator{
		nodes:          make(map[string]*SimulatedNode),
		apiLatency:     cfg.APILatency,
		apiErrorRate:   cfg.APIErrorRate,
		maxConcurrency: cfg.MaxConcurrency,
		requestWindow:  time.Now(),
		rng:            rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// AddNode adds a node to the cluster
func (s *K8sAPISimulator) AddNode(name string, ready bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nodes[name] = &SimulatedNode{
		Name:          name,
		Ready:         ready,
		Schedulable:   true,
		Taints:        make([]*taints.Taint, 0),
		Labels:        make(map[string]string),
		Version:       "v1.28.0",
		PodCount:      s.rng.Intn(50) + 10, // 10-60 pods
		Conditions:    map[string]string{"Ready": "True"},
		CreatedAt:     time.Now(),
		LastHeartbeat: time.Now(),
	}
}

// simulateLatency adds realistic API latency
func (s *K8sAPISimulator) simulateLatency() {
	jitter := time.Duration(s.rng.Int63n(int64(s.apiLatency / 2)))
	time.Sleep(s.apiLatency + jitter)
}

// maybeError randomly returns API errors
func (s *K8sAPISimulator) maybeError() error {
	if s.rng.Float64() < s.apiErrorRate {
		atomic.AddInt64(&s.failedReqs, 1)
		errors := []string{
			"connection refused",
			"context deadline exceeded",
			"etcd leader changed",
			"too many requests",
		}
		return fmt.Errorf("kube-apiserver: %s", errors[s.rng.Intn(len(errors))])
	}
	return nil
}

// GetMetrics returns API metrics
func (s *K8sAPISimulator) GetMetrics() map[string]int64 {
	return map[string]int64{
		"total_requests":     atomic.LoadInt64(&s.totalRequests),
		"failed_requests":    atomic.LoadInt64(&s.failedReqs),
		"throttled_requests": atomic.LoadInt64(&s.throttledReqs),
	}
}

// =============================================================================
// REALISTIC KUBE CLIENT FOR TAINTS
// =============================================================================

// RealisticKubeClient implements taints.KubeClient with realistic behavior
type RealisticKubeClient struct {
	simulator *K8sAPISimulator
}

func NewRealisticKubeClient(sim *K8sAPISimulator) *RealisticKubeClient {
	return &RealisticKubeClient{simulator: sim}
}

func (c *RealisticKubeClient) ApplyTaint(ctx context.Context, nodeName string, taint *taints.Taint) error {
	atomic.AddInt64(&c.simulator.totalRequests, 1)
	c.simulator.simulateLatency()

	if err := c.simulator.maybeError(); err != nil {
		return err
	}

	c.simulator.mu.Lock()
	defer c.simulator.mu.Unlock()

	node, exists := c.simulator.nodes[nodeName]
	if !exists {
		return fmt.Errorf("node %q not found", nodeName)
	}

	// Check if taint already exists
	for i, existing := range node.Taints {
		if existing.Key == taint.Key {
			// Update existing taint
			node.Taints[i] = taint
			return nil
		}
	}

	// Add new taint
	node.Taints = append(node.Taints, taint)

	// If NoExecute, simulate pod eviction
	if taint.Effect == taints.TaintEffectNoExecute {
		// Pods without tolerations get evicted (simulated)
		node.PodCount = node.PodCount / 2
	}

	return nil
}

func (c *RealisticKubeClient) RemoveTaint(ctx context.Context, nodeName string, taintKey string) error {
	atomic.AddInt64(&c.simulator.totalRequests, 1)
	c.simulator.simulateLatency()

	if err := c.simulator.maybeError(); err != nil {
		return err
	}

	c.simulator.mu.Lock()
	defer c.simulator.mu.Unlock()

	node, exists := c.simulator.nodes[nodeName]
	if !exists {
		return fmt.Errorf("node %q not found", nodeName)
	}

	for i, taint := range node.Taints {
		if taint.Key == taintKey {
			node.Taints = append(node.Taints[:i], node.Taints[i+1:]...)
			return nil
		}
	}

	return nil // Taint not found is OK
}

func (c *RealisticKubeClient) GetNodeTaints(ctx context.Context, nodeName string) ([]*taints.Taint, error) {
	atomic.AddInt64(&c.simulator.totalRequests, 1)
	c.simulator.simulateLatency()

	if err := c.simulator.maybeError(); err != nil {
		return nil, err
	}

	c.simulator.mu.RLock()
	defer c.simulator.mu.RUnlock()

	node, exists := c.simulator.nodes[nodeName]
	if !exists {
		return nil, fmt.Errorf("node %q not found", nodeName)
	}

	// Return copy
	result := make([]*taints.Taint, len(node.Taints))
	copy(result, node.Taints)
	return result, nil
}

func (c *RealisticKubeClient) ExecuteCommand(ctx context.Context, args ...string) (string, error) {
	atomic.AddInt64(&c.simulator.totalRequests, 1)
	c.simulator.simulateLatency()

	if err := c.simulator.maybeError(); err != nil {
		return "", err
	}

	return "command executed successfully", nil
}

// =============================================================================
// REALISTIC BACKUP STORAGE SIMULATOR
// =============================================================================

// RealisticBackupStorage simulates S3/GCS-like backup storage
type RealisticBackupStorage struct {
	mu sync.RWMutex

	// Storage
	data map[string][]byte

	// Configuration
	uploadLatency   time.Duration
	downloadLatency time.Duration
	errorRate       float64
	maxSize         int64

	// Metrics
	uploads   int64
	downloads int64
	deletes   int64
	bytes     int64

	rng *rand.Rand
}

// NewRealisticBackupStorage creates a new backup storage simulator
func NewRealisticBackupStorage() *RealisticBackupStorage {
	return &RealisticBackupStorage{
		data:            make(map[string][]byte),
		uploadLatency:   50 * time.Millisecond,
		downloadLatency: 30 * time.Millisecond,
		errorRate:       0.02,
		maxSize:         100 * 1024 * 1024, // 100MB
		rng:             rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (s *RealisticBackupStorage) Name() string {
	return "realistic-s3-storage"
}

func (s *RealisticBackupStorage) Upload(ctx context.Context, key string, data []byte) error {
	// Simulate upload latency based on size
	latency := s.uploadLatency + time.Duration(len(data)/1024)*time.Microsecond
	time.Sleep(latency)

	atomic.AddInt64(&s.uploads, 1)

	// Random error
	if s.rng.Float64() < s.errorRate {
		return fmt.Errorf("S3 PutObject error: SlowDown - reduce request rate")
	}

	// Check size
	if int64(len(data)) > s.maxSize {
		return fmt.Errorf("object too large: %d > %d", len(data), s.maxSize)
	}

	s.mu.Lock()
	s.data[key] = make([]byte, len(data))
	copy(s.data[key], data)
	atomic.AddInt64(&s.bytes, int64(len(data)))
	s.mu.Unlock()

	return nil
}

func (s *RealisticBackupStorage) Download(ctx context.Context, key string) ([]byte, error) {
	atomic.AddInt64(&s.downloads, 1)

	// Random error
	if s.rng.Float64() < s.errorRate {
		return nil, fmt.Errorf("S3 GetObject error: InternalError")
	}

	s.mu.RLock()
	data, exists := s.data[key]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("S3 GetObject error: NoSuchKey - %s", key)
	}

	// Simulate download latency
	latency := s.downloadLatency + time.Duration(len(data)/1024)*time.Microsecond
	time.Sleep(latency)

	// Return copy
	result := make([]byte, len(data))
	copy(result, data)
	return result, nil
}

func (s *RealisticBackupStorage) Delete(ctx context.Context, key string) error {
	atomic.AddInt64(&s.deletes, 1)

	// Random error
	if s.rng.Float64() < s.errorRate {
		return fmt.Errorf("S3 DeleteObject error: ServiceUnavailable")
	}

	s.mu.Lock()
	delete(s.data, key)
	s.mu.Unlock()

	return nil
}

func (s *RealisticBackupStorage) List(ctx context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var keys []string
	for k := range s.data {
		keys = append(keys, k)
	}
	return keys, nil
}

func (s *RealisticBackupStorage) GetMetrics() map[string]int64 {
	return map[string]int64{
		"uploads":   atomic.LoadInt64(&s.uploads),
		"downloads": atomic.LoadInt64(&s.downloads),
		"deletes":   atomic.LoadInt64(&s.deletes),
		"bytes":     atomic.LoadInt64(&s.bytes),
	}
}

// =============================================================================
// REALISTIC BACKUP COMPONENTS
// =============================================================================

// EtcdBackupComponent simulates etcd backup
type EtcdBackupComponent struct {
	dataSize   int
	latency    time.Duration
	failChance float64
}

func NewEtcdBackupComponent(dataSize int) *EtcdBackupComponent {
	return &EtcdBackupComponent{
		dataSize:   dataSize,
		latency:    100 * time.Millisecond,
		failChance: 0.05,
	}
}

func (c *EtcdBackupComponent) Name() string {
	return "etcd"
}

func (c *EtcdBackupComponent) Backup(ctx context.Context) ([]byte, error) {
	time.Sleep(c.latency)

	if rand.Float64() < c.failChance {
		return nil, fmt.Errorf("etcdctl snapshot save: failed to reach quorum")
	}

	// Generate fake etcd snapshot
	data := make([]byte, c.dataSize)
	for i := range data {
		data[i] = byte(rand.Intn(256))
	}
	return data, nil
}

func (c *EtcdBackupComponent) Restore(ctx context.Context, data []byte) error {
	time.Sleep(c.latency * 2) // Restore takes longer

	if rand.Float64() < c.failChance {
		return fmt.Errorf("etcdctl snapshot restore: data directory not empty")
	}

	return nil
}

// VolumeBackupComponent simulates PV backup
type VolumeBackupComponent struct {
	volumes  []string
	dataSize int
	latency  time.Duration
}

func NewVolumeBackupComponent(volumes []string) *VolumeBackupComponent {
	return &VolumeBackupComponent{
		volumes:  volumes,
		dataSize: 1024 * 10, // 10KB per volume
		latency:  50 * time.Millisecond,
	}
}

func (c *VolumeBackupComponent) Name() string {
	return "volumes"
}

func (c *VolumeBackupComponent) Backup(ctx context.Context) ([]byte, error) {
	time.Sleep(c.latency * time.Duration(len(c.volumes)))

	// Generate fake volume snapshot data
	totalSize := c.dataSize * len(c.volumes)
	data := make([]byte, totalSize)
	for i := range data {
		data[i] = byte(rand.Intn(256))
	}
	return data, nil
}

func (c *VolumeBackupComponent) Restore(ctx context.Context, data []byte) error {
	time.Sleep(c.latency * time.Duration(len(c.volumes)) * 2)
	return nil
}

// =============================================================================
// REALISTIC UPGRADE COMPONENTS
// =============================================================================

// RealisticHealthChecker simulates K8s node health checks
type RealisticHealthChecker struct {
	simulator     *K8sAPISimulator
	healthyDelay  time.Duration
	unhealthyRate float64
}

func NewRealisticHealthChecker(sim *K8sAPISimulator) *RealisticHealthChecker {
	return &RealisticHealthChecker{
		simulator:     sim,
		healthyDelay:  500 * time.Millisecond,
		unhealthyRate: 0.1, // 10% chance node stays unhealthy
	}
}

func (h *RealisticHealthChecker) IsNodeHealthy(ctx context.Context, nodeName string) (bool, error) {
	h.simulator.simulateLatency()

	h.simulator.mu.RLock()
	node, exists := h.simulator.nodes[nodeName]
	h.simulator.mu.RUnlock()

	if !exists {
		return false, fmt.Errorf("node %s not found", nodeName)
	}

	// Simulate node becoming healthy after upgrade
	if !node.Ready {
		if rand.Float64() > h.unhealthyRate {
			h.simulator.mu.Lock()
			node.Ready = true
			h.simulator.mu.Unlock()
		}
	}

	return node.Ready, nil
}

// RealisticNodeDrainer simulates node draining
type RealisticNodeDrainer struct {
	simulator   *K8sAPISimulator
	drainTime   time.Duration
	failureRate float64
}

func NewRealisticNodeDrainer(sim *K8sAPISimulator) *RealisticNodeDrainer {
	return &RealisticNodeDrainer{
		simulator:   sim,
		drainTime:   200 * time.Millisecond,
		failureRate: 0.05,
	}
}

func (d *RealisticNodeDrainer) DrainNode(ctx context.Context, nodeName string) error {
	d.simulator.mu.Lock()
	node, exists := d.simulator.nodes[nodeName]
	if !exists {
		d.simulator.mu.Unlock()
		return fmt.Errorf("node %s not found", nodeName)
	}
	node.DrainInProgress = true
	node.Cordoned = true
	podCount := node.PodCount
	d.simulator.mu.Unlock()

	// Simulate draining pods
	for i := podCount; i > 0; i-- {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Simulate pod eviction
		time.Sleep(d.drainTime / time.Duration(podCount+1))

		// Random failure
		if rand.Float64() < d.failureRate/float64(podCount) {
			return fmt.Errorf("pod pdb-protected-pod has disruption budget which prevents eviction")
		}

		d.simulator.mu.Lock()
		node.PodCount--
		d.simulator.mu.Unlock()
	}

	d.simulator.mu.Lock()
	node.DrainInProgress = false
	d.simulator.mu.Unlock()

	return nil
}

func (d *RealisticNodeDrainer) UncordonNode(ctx context.Context, nodeName string) error {
	d.simulator.mu.Lock()
	defer d.simulator.mu.Unlock()

	node, exists := d.simulator.nodes[nodeName]
	if !exists {
		return fmt.Errorf("node %s not found", nodeName)
	}

	node.Cordoned = false
	node.Schedulable = true
	return nil
}

// =============================================================================
// REALISTIC WEBHOOK SERVER FOR HOOKS
// =============================================================================

// WebhookServer simulates external webhook endpoints
type WebhookServer struct {
	server      *httptest.Server
	mu          sync.Mutex
	requests    []WebhookRequest
	failureRate float64
	latency     time.Duration
}

type WebhookRequest struct {
	Path      string
	Method    string
	Timestamp time.Time
	Body      string
}

func NewWebhookServer(failureRate float64, latency time.Duration) *WebhookServer {
	ws := &WebhookServer{
		requests:    make([]WebhookRequest, 0),
		failureRate: failureRate,
		latency:     latency,
	}

	ws.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(ws.latency)

		ws.mu.Lock()
		ws.requests = append(ws.requests, WebhookRequest{
			Path:      r.URL.Path,
			Method:    r.Method,
			Timestamp: time.Now(),
		})
		ws.mu.Unlock()

		if rand.Float64() < ws.failureRate {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}))

	return ws
}

func (ws *WebhookServer) URL() string {
	return ws.server.URL
}

func (ws *WebhookServer) Close() {
	ws.server.Close()
}

func (ws *WebhookServer) GetRequests() []WebhookRequest {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	result := make([]WebhookRequest, len(ws.requests))
	copy(result, ws.requests)
	return result
}

// =============================================================================
// REALISTIC INTEGRATION TESTS - TAINTS
// =============================================================================

func TestRealistic_Taints_ApplyWithLatency(t *testing.T) {
	sim := NewK8sAPISimulator(&K8sAPIConfig{
		APILatency:   20 * time.Millisecond,
		APIErrorRate: 0,
	})

	// Add nodes
	for i := 0; i < 5; i++ {
		sim.AddNode(fmt.Sprintf("worker-%d", i), true)
	}

	kubeClient := NewRealisticKubeClient(sim)
	manager := taints.NewManager(&taints.ManagerConfig{
		KubeClient: kubeClient,
	})

	ctx := context.Background()

	t.Log("=== Testing Realistic Taint Application ===")

	// Apply taints to multiple nodes
	startTime := time.Now()
	taintsToApply := []config.TaintConfig{
		{Key: "dedicated", Value: "gpu", Effect: "NoSchedule"},
		{Key: "env", Value: "production", Effect: "PreferNoSchedule"},
	}

	for i := 0; i < 5; i++ {
		nodeName := fmt.Sprintf("worker-%d", i)
		err := manager.ApplyTaints(ctx, nodeName, taintsToApply)
		require.NoError(t, err)
		t.Logf("  Applied taints to %s", nodeName)
	}

	duration := time.Since(startTime)
	t.Logf("\nTotal time: %v (avg %v per node)", duration, duration/5)

	// Verify taints were applied
	for i := 0; i < 5; i++ {
		nodeName := fmt.Sprintf("worker-%d", i)
		appliedTaints, err := manager.GetTaints(ctx, nodeName)
		require.NoError(t, err)
		assert.Len(t, appliedTaints, 2)
	}

	metrics := sim.GetMetrics()
	t.Logf("\nAPI Metrics:")
	t.Logf("  Total requests: %d", metrics["total_requests"])

	t.Log("\n✅ Taint application with realistic latency working")
}

func TestRealistic_Taints_NoExecuteEviction(t *testing.T) {
	sim := NewK8sAPISimulator(&K8sAPIConfig{
		APILatency:   10 * time.Millisecond,
		APIErrorRate: 0,
	})

	sim.AddNode("worker-1", true)

	// Get initial pod count
	sim.mu.RLock()
	initialPods := sim.nodes["worker-1"].PodCount
	sim.mu.RUnlock()

	kubeClient := NewRealisticKubeClient(sim)
	manager := taints.NewManager(&taints.ManagerConfig{
		KubeClient: kubeClient,
	})

	ctx := context.Background()

	t.Log("=== Testing NoExecute Taint Pod Eviction ===")
	t.Logf("Initial pod count: %d", initialPods)

	// Apply NoExecute taint
	err := manager.ApplyTaints(ctx, "worker-1", []config.TaintConfig{
		{Key: "node.kubernetes.io/maintenance", Value: "true", Effect: "NoExecute"},
	})
	require.NoError(t, err)

	// Check pod count after eviction
	sim.mu.RLock()
	finalPods := sim.nodes["worker-1"].PodCount
	sim.mu.RUnlock()

	t.Logf("Final pod count: %d (evicted: %d)", finalPods, initialPods-finalPods)

	assert.Less(t, finalPods, initialPods, "NoExecute should evict pods")

	t.Log("\n✅ NoExecute taint eviction simulation working")
}

func TestRealistic_Taints_SyncWithDrift(t *testing.T) {
	sim := NewK8sAPISimulator(&K8sAPIConfig{
		APILatency:   5 * time.Millisecond,
		APIErrorRate: 0,
	})

	sim.AddNode("worker-1", true)

	kubeClient := NewRealisticKubeClient(sim)
	manager := taints.NewManager(&taints.ManagerConfig{
		KubeClient: kubeClient,
	})

	ctx := context.Background()

	t.Log("=== Testing Taint Sync with Configuration Drift ===")

	// Apply initial taints (simulating drift)
	sim.mu.Lock()
	sim.nodes["worker-1"].Taints = []*taints.Taint{
		{Key: "old-taint", Value: "remove-me", Effect: taints.TaintEffectNoSchedule},
		{Key: "keep-this", Value: "old-value", Effect: taints.TaintEffectNoSchedule},
	}
	sim.mu.Unlock()

	t.Log("Initial state (drifted):")
	currentTaints, _ := manager.GetTaints(ctx, "worker-1")
	for _, taint := range currentTaints {
		t.Logf("  - %s=%s:%s", taint.Key, taint.Value, taint.Effect)
	}

	// Define desired state
	desiredTaints := []config.TaintConfig{
		{Key: "keep-this", Value: "new-value", Effect: "NoSchedule"}, // Update
		{Key: "new-taint", Value: "add-me", Effect: "NoSchedule"},    // Add
		// old-taint should be removed
	}

	t.Log("\nSyncing to desired state...")
	err := manager.SyncTaints(ctx, "worker-1", desiredTaints)
	require.NoError(t, err)

	// Verify sync
	finalTaints, _ := manager.GetTaints(ctx, "worker-1")
	t.Log("\nFinal state:")
	for _, taint := range finalTaints {
		t.Logf("  - %s=%s:%s", taint.Key, taint.Value, taint.Effect)
	}

	assert.Len(t, finalTaints, 2)

	// Verify old-taint was removed
	for _, taint := range finalTaints {
		assert.NotEqual(t, "old-taint", taint.Key)
	}

	t.Log("\n✅ Taint sync with drift correction working")
}

// =============================================================================
// REALISTIC INTEGRATION TESTS - BACKUP
// =============================================================================

func TestRealistic_Backup_FullClusterBackup(t *testing.T) {
	storage := NewRealisticBackupStorage()

	manager, err := backup.NewManager(&backup.ManagerConfig{
		BackupConfig: &config.BackupConfig{
			RetentionDays: 7,
		},
		Storage: storage,
	})
	require.NoError(t, err)

	// Register components
	manager.RegisterComponent(NewEtcdBackupComponent(50 * 1024)) // 50KB etcd
	manager.RegisterComponent(NewVolumeBackupComponent([]string{
		"pvc-postgres-data",
		"pvc-redis-data",
		"pvc-app-uploads",
	}))

	ctx := context.Background()

	t.Log("=== Testing Realistic Full Cluster Backup ===")

	startTime := time.Now()
	backup, err := manager.CreateBackup(ctx, nil)
	require.NoError(t, err)
	duration := time.Since(startTime)

	t.Logf("\nBackup completed:")
	t.Logf("  ID: %s", backup.ID)
	t.Logf("  Status: %s", backup.Status)
	t.Logf("  Size: %d bytes", backup.Size)
	t.Logf("  Components: %v", backup.Components)
	t.Logf("  Duration: %v", duration)

	assert.Equal(t, "completed", backup.Status)
	assert.Greater(t, backup.Size, int64(0))

	metrics := storage.GetMetrics()
	t.Logf("\nStorage metrics:")
	t.Logf("  Uploads: %d", metrics["uploads"])
	t.Logf("  Total bytes: %d", metrics["bytes"])

	t.Log("\n✅ Full cluster backup working")
}

func TestRealistic_Backup_RestoreWithVerification(t *testing.T) {
	storage := NewRealisticBackupStorage()

	manager, err := backup.NewManager(&backup.ManagerConfig{
		BackupConfig: &config.BackupConfig{},
		Storage:      storage,
	})
	require.NoError(t, err)

	manager.RegisterComponent(NewEtcdBackupComponent(10 * 1024))

	ctx := context.Background()

	t.Log("=== Testing Backup Restore with Verification ===")

	// Create backup
	t.Log("1. Creating backup...")
	backup, err := manager.CreateBackup(ctx, []string{"etcd"})
	require.NoError(t, err)
	t.Logf("   Created backup: %s", backup.ID)

	// Restore
	t.Log("\n2. Restoring backup...")
	err = manager.RestoreBackup(ctx, backup.ID)
	require.NoError(t, err)
	t.Log("   Restore completed successfully")

	metrics := storage.GetMetrics()
	t.Logf("\n3. Verification:")
	t.Logf("   Uploads: %d", metrics["uploads"])
	t.Logf("   Downloads: %d", metrics["downloads"])

	assert.Equal(t, int64(1), metrics["uploads"])
	assert.Equal(t, int64(1), metrics["downloads"])

	t.Log("\n✅ Backup restore with verification working")
}

func TestRealistic_Backup_ConcurrentBackups(t *testing.T) {
	storage := NewRealisticBackupStorage()

	manager, err := backup.NewManager(&backup.ManagerConfig{
		BackupConfig: &config.BackupConfig{},
		Storage:      storage,
	})
	require.NoError(t, err)

	manager.RegisterComponent(NewEtcdBackupComponent(5 * 1024))

	ctx := context.Background()

	t.Log("=== Testing Concurrent Backup Operations ===")

	var wg sync.WaitGroup
	backups := make([]*provisioning.Backup, 5)
	errors := make([]error, 5)

	startTime := time.Now()

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			backups[idx], errors[idx] = manager.CreateBackup(ctx, []string{"etcd"})
		}(i)
	}

	wg.Wait()
	duration := time.Since(startTime)

	t.Logf("\nConcurrent backup results:")
	successCount := 0
	for i, b := range backups {
		if errors[i] != nil {
			t.Logf("  Backup %d: FAILED - %v", i, errors[i])
		} else {
			t.Logf("  Backup %d: %s (size: %d)", i, b.ID, b.Size)
			successCount++
		}
	}

	t.Logf("\nTotal duration: %v", duration)
	t.Logf("Success rate: %d/%d", successCount, 5)

	assert.GreaterOrEqual(t, successCount, 3, "Most concurrent backups should succeed")

	t.Log("\n✅ Concurrent backup operations working")
}

// =============================================================================
// REALISTIC INTEGRATION TESTS - UPGRADES
// =============================================================================

func TestRealistic_Upgrade_RollingUpgrade(t *testing.T) {
	sim := NewK8sAPISimulator(&K8sAPIConfig{
		APILatency:   10 * time.Millisecond,
		APIErrorRate: 0,
	})

	// Add cluster nodes
	for i := 0; i < 3; i++ {
		sim.AddNode(fmt.Sprintf("master-%d", i), true)
	}
	for i := 0; i < 6; i++ {
		sim.AddNode(fmt.Sprintf("worker-%d", i), true)
	}

	healthChecker := NewRealisticHealthChecker(sim)

	orchestrator := upgrades.NewOrchestrator(&upgrades.OrchestratorConfig{
		UpgradeConfig: &config.UpgradeConfig{
			Strategy:            "rolling",
			MaxUnavailable:      2,
			HealthCheckInterval: 1,
		},
		HealthChecker: healthChecker,
	})

	ctx := context.Background()

	t.Log("=== Testing Realistic Rolling Upgrade ===")

	// Create nodes for upgrade
	nodes := make([]*providers.NodeOutput, 0)
	for i := 0; i < 6; i++ {
		nodes = append(nodes, &providers.NodeOutput{
			Name: fmt.Sprintf("worker-%d", i),
		})
	}

	// Plan upgrade
	t.Log("\n1. Planning upgrade...")
	plan, err := orchestrator.Plan(ctx, "v1.29.0", nodes)
	require.NoError(t, err)

	t.Logf("   Plan ID: %s", plan.ID)
	t.Logf("   Current: %s -> Target: %s", plan.CurrentVersion, plan.TargetVersion)
	t.Logf("   Strategy: %s", plan.Strategy)
	t.Logf("   Nodes: %d", len(plan.Nodes))
	t.Logf("   Estimated time: %ds", plan.EstimatedTime)

	// Check batch distribution
	batchCount := make(map[int]int)
	for _, node := range plan.Nodes {
		batchCount[node.Batch]++
	}
	t.Logf("   Batches: %v", batchCount)

	assert.Equal(t, 6, len(plan.Nodes))
	assert.Equal(t, "v1.29.0", plan.TargetVersion)

	t.Log("\n✅ Rolling upgrade planning working")
}

func TestRealistic_Upgrade_StatusTracking(t *testing.T) {
	sim := NewK8sAPISimulator(&K8sAPIConfig{
		APILatency:   5 * time.Millisecond,
		APIErrorRate: 0,
	})

	for i := 0; i < 3; i++ {
		sim.AddNode(fmt.Sprintf("worker-%d", i), true)
	}

	healthChecker := NewRealisticHealthChecker(sim)

	orchestrator := upgrades.NewOrchestrator(&upgrades.OrchestratorConfig{
		UpgradeConfig: &config.UpgradeConfig{
			Strategy:       "rolling",
			MaxUnavailable: 1,
		},
		HealthChecker: healthChecker,
	})

	ctx := context.Background()

	t.Log("=== Testing Upgrade Status Tracking ===")

	nodes := []*providers.NodeOutput{
		{Name: "worker-0"},
		{Name: "worker-1"},
		{Name: "worker-2"},
	}

	// Plan
	plan, err := orchestrator.Plan(ctx, "v1.29.0", nodes)
	require.NoError(t, err)

	// Check initial status
	status, err := orchestrator.GetStatus(ctx)
	require.NoError(t, err)

	t.Logf("\nInitial status:")
	t.Logf("  Phase: %s", status.Phase)
	t.Logf("  Progress: %d%%", status.Progress)
	t.Logf("  Plan ID: %s", status.PlanID)

	assert.Equal(t, "planned", status.Phase)
	assert.Equal(t, plan.ID, status.PlanID)

	t.Log("\n✅ Upgrade status tracking working")
}

// =============================================================================
// REALISTIC INTEGRATION TESTS - HOOKS
// =============================================================================

func TestRealistic_Hooks_WebhookExecution(t *testing.T) {
	// Create webhook server
	webhook := NewWebhookServer(0.0, 20*time.Millisecond)
	defer webhook.Close()

	engine := hooks.NewEngine(&hooks.EngineConfig{})

	t.Log("=== Testing Realistic Webhook Execution ===")
	t.Logf("Webhook URL: %s", webhook.URL())

	// Register HTTP hook
	engine.RegisterHook(provisioning.HookEventPostClusterReady, &config.HookAction{
		Type:       "http",
		URL:        webhook.URL() + "/cluster-ready",
		Timeout:    30,
		RetryCount: 3,
	}, 1)

	ctx := context.Background()

	// Trigger hooks
	t.Log("\nTriggering PostClusterReady hook...")
	err := engine.TriggerHooks(ctx, provisioning.HookEventPostClusterReady, map[string]interface{}{
		"cluster_name": "production",
		"node_count":   5,
	})
	require.NoError(t, err)

	// Verify webhook was called
	requests := webhook.GetRequests()
	t.Logf("\nWebhook requests received: %d", len(requests))
	for _, req := range requests {
		t.Logf("  - %s %s at %v", req.Method, req.Path, req.Timestamp)
	}

	assert.Len(t, requests, 1)
	assert.Equal(t, "/cluster-ready", requests[0].Path)

	t.Log("\n✅ Webhook execution working")
}

func TestRealistic_Hooks_RetryOnFailure(t *testing.T) {
	// Create webhook server with 50% failure rate
	webhook := NewWebhookServer(0.5, 10*time.Millisecond)
	defer webhook.Close()

	engine := hooks.NewEngine(&hooks.EngineConfig{})

	t.Log("=== Testing Hook Retry on Failure ===")
	t.Log("Webhook failure rate: 50%")

	// Register hook with retries
	engine.RegisterHook(provisioning.HookEventPostNodeCreate, &config.HookAction{
		Type:       "http",
		URL:        webhook.URL() + "/node-created",
		Timeout:    5,
		RetryCount: 5, // 5 retries
	}, 1)

	ctx := context.Background()

	// Trigger multiple times
	successCount := 0
	for i := 0; i < 10; i++ {
		err := engine.TriggerHooks(ctx, provisioning.HookEventPostNodeCreate, map[string]interface{}{
			"node_name": fmt.Sprintf("worker-%d", i),
		})
		if err == nil {
			successCount++
		}
	}

	requests := webhook.GetRequests()
	t.Logf("\nResults:")
	t.Logf("  Triggers: 10")
	t.Logf("  Total requests (with retries): %d", len(requests))
	t.Logf("  Successful: %d", successCount)

	// With 50% failure rate and 5 retries, most should succeed
	assert.GreaterOrEqual(t, successCount, 7, "Most hooks should succeed with retries")

	t.Log("\n✅ Hook retry mechanism working")
}

func TestRealistic_Hooks_MultipleHooksSequence(t *testing.T) {
	webhook := NewWebhookServer(0.0, 5*time.Millisecond)
	defer webhook.Close()

	engine := hooks.NewEngine(&hooks.EngineConfig{})

	t.Log("=== Testing Multiple Hooks in Sequence ===")

	// Register multiple hooks for same event with different priorities
	engine.RegisterHook(provisioning.HookEventPostClusterReady, &config.HookAction{
		Type: "http",
		URL:  webhook.URL() + "/hook-1-priority-high",
	}, 1) // High priority

	engine.RegisterHook(provisioning.HookEventPostClusterReady, &config.HookAction{
		Type: "http",
		URL:  webhook.URL() + "/hook-2-priority-medium",
	}, 5) // Medium priority

	engine.RegisterHook(provisioning.HookEventPostClusterReady, &config.HookAction{
		Type: "http",
		URL:  webhook.URL() + "/hook-3-priority-low",
	}, 10) // Low priority

	ctx := context.Background()

	t.Log("\nTriggering hooks...")
	err := engine.TriggerHooks(ctx, provisioning.HookEventPostClusterReady, nil)
	require.NoError(t, err)

	requests := webhook.GetRequests()
	t.Logf("\nExecution order:")
	for i, req := range requests {
		t.Logf("  %d. %s", i+1, req.Path)
	}

	assert.Len(t, requests, 3)
	// Verify priority order
	assert.Contains(t, requests[0].Path, "priority-high")
	assert.Contains(t, requests[1].Path, "priority-medium")
	assert.Contains(t, requests[2].Path, "priority-low")

	t.Log("\n✅ Multiple hooks sequential execution working")
}

func TestRealistic_Hooks_TimeoutHandling(t *testing.T) {
	// Create webhook server with high latency
	webhook := NewWebhookServer(0.0, 2*time.Second)
	defer webhook.Close()

	engine := hooks.NewEngine(&hooks.EngineConfig{})

	t.Log("=== Testing Hook Timeout Handling ===")
	t.Log("Webhook latency: 2s, Hook timeout: 100ms")

	// Register hook with short timeout
	engine.RegisterHook(provisioning.HookEventPostNodeCreate, &config.HookAction{
		Type:       "http",
		URL:        webhook.URL() + "/slow-endpoint",
		Timeout:    1, // 1 second timeout (will fail)
		RetryCount: 1,
	}, 1)

	ctx := context.Background()

	startTime := time.Now()
	err := engine.TriggerHooks(ctx, provisioning.HookEventPostNodeCreate, nil)
	duration := time.Since(startTime)

	t.Logf("\nExecution duration: %v", duration)
	t.Logf("Error: %v", err)

	// Hook should fail or timeout
	// Note: The hook continues on error by default
	assert.Less(t, duration, 3*time.Second, "Should not wait for full webhook response")

	t.Log("\n✅ Hook timeout handling working")
}

// =============================================================================
// REALISTIC END-TO-END TEST
// =============================================================================

func TestRealistic_E2E_ClusterLifecycle(t *testing.T) {
	// Setup all simulators
	k8sSim := NewK8sAPISimulator(&K8sAPIConfig{
		APILatency:   10 * time.Millisecond,
		APIErrorRate: 0.01,
	})

	backupStorage := NewRealisticBackupStorage()

	webhook := NewWebhookServer(0.1, 20*time.Millisecond)
	defer webhook.Close()

	t.Log("=== Testing Realistic E2E Cluster Lifecycle ===")

	ctx := context.Background()

	// Step 1: Create cluster nodes
	t.Log("\n📋 Step 1: Creating cluster nodes...")
	for i := 0; i < 3; i++ {
		k8sSim.AddNode(fmt.Sprintf("master-%d", i), true)
	}
	for i := 0; i < 5; i++ {
		k8sSim.AddNode(fmt.Sprintf("worker-%d", i), true)
	}
	t.Logf("   Created 3 masters + 5 workers")

	// Step 2: Apply taints
	t.Log("\n🏷️  Step 2: Applying taints to masters...")
	kubeClient := NewRealisticKubeClient(k8sSim)
	taintManager := taints.NewManager(&taints.ManagerConfig{
		KubeClient: kubeClient,
	})

	masterTaints := []config.TaintConfig{
		{Key: "node-role.kubernetes.io/control-plane", Value: "", Effect: "NoSchedule"},
	}

	for i := 0; i < 3; i++ {
		err := taintManager.ApplyTaints(ctx, fmt.Sprintf("master-%d", i), masterTaints)
		require.NoError(t, err)
	}
	t.Log("   Applied control-plane taints to all masters")

	// Step 3: Setup backup
	t.Log("\n💾 Step 3: Creating initial backup...")
	backupManager, err := backup.NewManager(&backup.ManagerConfig{
		BackupConfig: &config.BackupConfig{RetentionDays: 30},
		Storage:      backupStorage,
	})
	require.NoError(t, err)

	backupManager.RegisterComponent(NewEtcdBackupComponent(20 * 1024))

	initialBackup, err := backupManager.CreateBackup(ctx, nil)
	require.NoError(t, err)
	t.Logf("   Backup created: %s (%d bytes)", initialBackup.ID, initialBackup.Size)

	// Step 4: Setup hooks
	t.Log("\n🪝 Step 4: Configuring lifecycle hooks...")
	hookEngine := hooks.NewEngine(&hooks.EngineConfig{})

	hookEngine.RegisterHook(provisioning.HookEventPostClusterReady, &config.HookAction{
		Type: "http",
		URL:  webhook.URL() + "/cluster-ready",
	}, 1)

	hookEngine.RegisterHook(provisioning.HookEventPreClusterDestroy, &config.HookAction{
		Type: "http",
		URL:  webhook.URL() + "/pre-destroy",
	}, 1)

	// Trigger cluster ready hook
	err = hookEngine.TriggerHooks(ctx, provisioning.HookEventPostClusterReady, map[string]interface{}{
		"cluster_name": "production",
	})
	require.NoError(t, err)
	t.Log("   PostClusterReady hook executed")

	// Step 5: Plan upgrade
	t.Log("\n⬆️  Step 5: Planning cluster upgrade...")
	healthChecker := NewRealisticHealthChecker(k8sSim)
	upgradeOrch := upgrades.NewOrchestrator(&upgrades.OrchestratorConfig{
		UpgradeConfig: &config.UpgradeConfig{
			Strategy:       "rolling",
			MaxUnavailable: 2,
		},
		HealthChecker: healthChecker,
	})

	workerNodes := make([]*providers.NodeOutput, 5)
	for i := 0; i < 5; i++ {
		workerNodes[i] = &providers.NodeOutput{Name: fmt.Sprintf("worker-%d", i)}
	}

	upgradePlan, err := upgradeOrch.Plan(ctx, "v1.29.0", workerNodes)
	require.NoError(t, err)
	t.Logf("   Upgrade plan created: %s", upgradePlan.ID)
	t.Logf("   Target version: %s", upgradePlan.TargetVersion)

	// Step 6: Collect metrics
	t.Log("\n📊 Final Metrics:")

	k8sMetrics := k8sSim.GetMetrics()
	t.Logf("   K8s API requests: %d (failed: %d)", k8sMetrics["total_requests"], k8sMetrics["failed_requests"])

	storageMetrics := backupStorage.GetMetrics()
	t.Logf("   Storage uploads: %d, bytes: %d", storageMetrics["uploads"], storageMetrics["bytes"])

	webhookRequests := webhook.GetRequests()
	t.Logf("   Webhook calls: %d", len(webhookRequests))

	t.Log("\n✅ E2E Cluster Lifecycle test completed successfully!")
}
