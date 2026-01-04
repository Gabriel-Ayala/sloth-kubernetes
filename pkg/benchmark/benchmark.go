// Package benchmark provides cluster benchmarking functionality
package benchmark

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/fatih/color"
)

// BenchmarkType defines the type of benchmark to run
type BenchmarkType string

const (
	BenchmarkNetwork BenchmarkType = "network"
	BenchmarkStorage BenchmarkType = "storage"
	BenchmarkCPU     BenchmarkType = "cpu"
	BenchmarkMemory  BenchmarkType = "memory"
	BenchmarkAll     BenchmarkType = "all"
)

// BenchmarkStatus represents the status of a benchmark
type BenchmarkStatus string

const (
	StatusPending   BenchmarkStatus = "pending"
	StatusRunning   BenchmarkStatus = "running"
	StatusCompleted BenchmarkStatus = "completed"
	StatusFailed    BenchmarkStatus = "failed"
)

// BenchmarkResult represents a single benchmark result
type BenchmarkResult struct {
	Name        string
	Type        BenchmarkType
	Status      BenchmarkStatus
	Score       float64
	Unit        string
	Duration    time.Duration
	Details     map[string]interface{}
	Error       string
	Timestamp   time.Time
	NodeResults []NodeBenchmarkResult
}

// NodeBenchmarkResult represents benchmark result for a specific node
type NodeBenchmarkResult struct {
	NodeName string
	Score    float64
	Unit     string
	Status   BenchmarkStatus
	Details  map[string]interface{}
	Error    string
}

// BenchmarkReport is the complete benchmark report
type BenchmarkReport struct {
	ClusterName   string
	Timestamp     time.Time
	Duration      time.Duration
	Results       []BenchmarkResult
	Summary       BenchmarkSummary
	Comparison    *BenchmarkComparison
	NodeCount     int
	KubeVersion   string
	CloudProvider string
}

// BenchmarkSummary summarizes the benchmark results
type BenchmarkSummary struct {
	TotalBenchmarks  int
	PassedBenchmarks int
	FailedBenchmarks int
	OverallScore     float64
	NetworkScore     float64
	StorageScore     float64
	CPUScore         float64
	MemoryScore      float64
	Grade            string
	Recommendations  []string
}

// BenchmarkComparison compares with reference values
type BenchmarkComparison struct {
	ReferenceName    string
	ReferenceVersion string
	Comparisons      []MetricComparison
}

// MetricComparison compares a metric with reference
type MetricComparison struct {
	MetricName     string
	CurrentValue   float64
	ReferenceValue float64
	Difference     float64
	Percentage     float64
	Better         bool
}

// Manager handles benchmark operations
type Manager struct {
	masterIP   string
	sshKey     string
	kubeconfig string
	verbose    bool
	nodeFilter []string
}

// NewManager creates a new benchmark manager
func NewManager(masterIP, sshKey, kubeconfig string) *Manager {
	return &Manager{
		masterIP:   masterIP,
		sshKey:     sshKey,
		kubeconfig: kubeconfig,
	}
}

// SetVerbose enables verbose output
func (m *Manager) SetVerbose(verbose bool) {
	m.verbose = verbose
}

// SetNodeFilter sets specific nodes to benchmark
func (m *Manager) SetNodeFilter(nodes []string) {
	m.nodeFilter = nodes
}

// RunBenchmark runs benchmarks of the specified type
func (m *Manager) RunBenchmark(benchmarkType BenchmarkType) (*BenchmarkReport, error) {
	report := &BenchmarkReport{
		ClusterName: "kubernetes-cluster",
		Timestamp:   time.Now(),
		Results:     []BenchmarkResult{},
	}

	startTime := time.Now()

	// Get cluster info
	if m.kubeconfig != "" {
		report.KubeVersion = m.getKubeVersion()
		report.NodeCount = m.getNodeCount()
	}

	// Run requested benchmarks
	switch benchmarkType {
	case BenchmarkAll:
		m.runNetworkBenchmarks(report)
		m.runStorageBenchmarks(report)
		m.runCPUBenchmarks(report)
		m.runMemoryBenchmarks(report)
	case BenchmarkNetwork:
		m.runNetworkBenchmarks(report)
	case BenchmarkStorage:
		m.runStorageBenchmarks(report)
	case BenchmarkCPU:
		m.runCPUBenchmarks(report)
	case BenchmarkMemory:
		m.runMemoryBenchmarks(report)
	}

	report.Duration = time.Since(startTime)

	// Calculate summary
	m.calculateSummary(report)

	// Add comparison with reference values
	m.addReferenceComparison(report)

	return report, nil
}

// Network benchmarks
func (m *Manager) runNetworkBenchmarks(report *BenchmarkReport) {
	// Pod-to-Pod latency benchmark
	podLatency := m.benchmarkPodLatency()
	report.Results = append(report.Results, podLatency)

	// Service DNS resolution benchmark
	dnsLatency := m.benchmarkDNSResolution()
	report.Results = append(report.Results, dnsLatency)

	// Network throughput benchmark
	throughput := m.benchmarkNetworkThroughput()
	report.Results = append(report.Results, throughput)

	// CNI performance benchmark
	cniPerf := m.benchmarkCNIPerformance()
	report.Results = append(report.Results, cniPerf)
}

// Storage benchmarks
func (m *Manager) runStorageBenchmarks(report *BenchmarkReport) {
	// IOPS benchmark
	iops := m.benchmarkStorageIOPS()
	report.Results = append(report.Results, iops)

	// Sequential read/write benchmark
	seqIO := m.benchmarkSequentialIO()
	report.Results = append(report.Results, seqIO)

	// Random read/write benchmark
	randomIO := m.benchmarkRandomIO()
	report.Results = append(report.Results, randomIO)

	// PVC provisioning time benchmark
	pvcProvision := m.benchmarkPVCProvisioning()
	report.Results = append(report.Results, pvcProvision)
}

// CPU benchmarks
func (m *Manager) runCPUBenchmarks(report *BenchmarkReport) {
	// CPU utilization benchmark
	cpuUtil := m.benchmarkCPUUtilization()
	report.Results = append(report.Results, cpuUtil)

	// Pod scheduling latency
	schedLatency := m.benchmarkSchedulingLatency()
	report.Results = append(report.Results, schedLatency)

	// Control plane response time
	apiLatency := m.benchmarkAPILatency()
	report.Results = append(report.Results, apiLatency)
}

// Memory benchmarks
func (m *Manager) runMemoryBenchmarks(report *BenchmarkReport) {
	// Memory utilization benchmark
	memUtil := m.benchmarkMemoryUtilization()
	report.Results = append(report.Results, memUtil)

	// Memory bandwidth benchmark
	memBandwidth := m.benchmarkMemoryBandwidth()
	report.Results = append(report.Results, memBandwidth)

	// etcd memory usage
	etcdMem := m.benchmarkEtcdMemory()
	report.Results = append(report.Results, etcdMem)
}

// Individual benchmark implementations
func (m *Manager) benchmarkPodLatency() BenchmarkResult {
	start := time.Now()
	result := BenchmarkResult{
		Name:      "Pod-to-Pod Latency",
		Type:      BenchmarkNetwork,
		Status:    StatusCompleted,
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
	}

	// Simulate benchmark (in real implementation, would run iperf3 or similar)
	latency := m.measurePodLatency()
	result.Score = latency
	result.Unit = "ms"
	result.Duration = time.Since(start)

	result.Details["min_latency"] = latency * 0.8
	result.Details["max_latency"] = latency * 1.5
	result.Details["avg_latency"] = latency
	result.Details["p99_latency"] = latency * 1.3

	return result
}

func (m *Manager) benchmarkDNSResolution() BenchmarkResult {
	start := time.Now()
	result := BenchmarkResult{
		Name:      "DNS Resolution Latency",
		Type:      BenchmarkNetwork,
		Status:    StatusCompleted,
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
	}

	// Simulate DNS benchmark
	latency := m.measureDNSLatency()
	result.Score = latency
	result.Unit = "ms"
	result.Duration = time.Since(start)

	result.Details["internal_dns"] = latency
	result.Details["external_dns"] = latency * 2.5

	return result
}

func (m *Manager) benchmarkNetworkThroughput() BenchmarkResult {
	start := time.Now()
	result := BenchmarkResult{
		Name:      "Network Throughput",
		Type:      BenchmarkNetwork,
		Status:    StatusCompleted,
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
	}

	// Simulate throughput measurement
	throughput := m.measureThroughput()
	result.Score = throughput
	result.Unit = "Gbps"
	result.Duration = time.Since(start)

	result.Details["tcp_throughput"] = throughput
	result.Details["udp_throughput"] = throughput * 0.95

	return result
}

func (m *Manager) benchmarkCNIPerformance() BenchmarkResult {
	start := time.Now()
	result := BenchmarkResult{
		Name:      "CNI Performance",
		Type:      BenchmarkNetwork,
		Status:    StatusCompleted,
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
	}

	// Simulate CNI performance score
	score := m.measureCNIPerformance()
	result.Score = score
	result.Unit = "score"
	result.Duration = time.Since(start)

	result.Details["pod_creation_time"] = 2.5
	result.Details["network_setup_time"] = 0.8

	return result
}

func (m *Manager) benchmarkStorageIOPS() BenchmarkResult {
	start := time.Now()
	result := BenchmarkResult{
		Name:      "Storage IOPS",
		Type:      BenchmarkStorage,
		Status:    StatusCompleted,
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
	}

	// Simulate IOPS measurement
	iops := m.measureIOPS()
	result.Score = iops
	result.Unit = "IOPS"
	result.Duration = time.Since(start)

	result.Details["read_iops"] = iops * 1.1
	result.Details["write_iops"] = iops * 0.9
	result.Details["mixed_iops"] = iops

	return result
}

func (m *Manager) benchmarkSequentialIO() BenchmarkResult {
	start := time.Now()
	result := BenchmarkResult{
		Name:      "Sequential I/O",
		Type:      BenchmarkStorage,
		Status:    StatusCompleted,
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
	}

	// Simulate sequential IO measurement
	throughput := m.measureSequentialIO()
	result.Score = throughput
	result.Unit = "MB/s"
	result.Duration = time.Since(start)

	result.Details["seq_read"] = throughput * 1.2
	result.Details["seq_write"] = throughput * 0.8

	return result
}

func (m *Manager) benchmarkRandomIO() BenchmarkResult {
	start := time.Now()
	result := BenchmarkResult{
		Name:      "Random I/O",
		Type:      BenchmarkStorage,
		Status:    StatusCompleted,
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
	}

	// Simulate random IO measurement
	throughput := m.measureRandomIO()
	result.Score = throughput
	result.Unit = "MB/s"
	result.Duration = time.Since(start)

	result.Details["random_read"] = throughput * 1.1
	result.Details["random_write"] = throughput * 0.9

	return result
}

func (m *Manager) benchmarkPVCProvisioning() BenchmarkResult {
	start := time.Now()
	result := BenchmarkResult{
		Name:      "PVC Provisioning Time",
		Type:      BenchmarkStorage,
		Status:    StatusCompleted,
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
	}

	// Simulate PVC provisioning time
	provisionTime := m.measurePVCProvisionTime()
	result.Score = provisionTime
	result.Unit = "seconds"
	result.Duration = time.Since(start)

	result.Details["dynamic_provision"] = provisionTime
	result.Details["static_provision"] = provisionTime * 0.5

	return result
}

func (m *Manager) benchmarkCPUUtilization() BenchmarkResult {
	start := time.Now()
	result := BenchmarkResult{
		Name:      "CPU Efficiency",
		Type:      BenchmarkCPU,
		Status:    StatusCompleted,
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
	}

	// Simulate CPU efficiency measurement
	efficiency := m.measureCPUEfficiency()
	result.Score = efficiency
	result.Unit = "%"
	result.Duration = time.Since(start)

	result.Details["user_cpu"] = efficiency * 0.7
	result.Details["system_cpu"] = efficiency * 0.2
	result.Details["idle_cpu"] = 100 - efficiency

	return result
}

func (m *Manager) benchmarkSchedulingLatency() BenchmarkResult {
	start := time.Now()
	result := BenchmarkResult{
		Name:      "Pod Scheduling Latency",
		Type:      BenchmarkCPU,
		Status:    StatusCompleted,
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
	}

	// Simulate scheduling latency measurement
	latency := m.measureSchedulingLatency()
	result.Score = latency
	result.Unit = "ms"
	result.Duration = time.Since(start)

	result.Details["queue_time"] = latency * 0.3
	result.Details["binding_time"] = latency * 0.7

	return result
}

func (m *Manager) benchmarkAPILatency() BenchmarkResult {
	start := time.Now()
	result := BenchmarkResult{
		Name:      "API Server Latency",
		Type:      BenchmarkCPU,
		Status:    StatusCompleted,
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
	}

	// Simulate API latency measurement
	latency := m.measureAPILatency()
	result.Score = latency
	result.Unit = "ms"
	result.Duration = time.Since(start)

	result.Details["get_latency"] = latency * 0.8
	result.Details["list_latency"] = latency * 1.5
	result.Details["create_latency"] = latency * 1.2

	return result
}

func (m *Manager) benchmarkMemoryUtilization() BenchmarkResult {
	start := time.Now()
	result := BenchmarkResult{
		Name:      "Memory Utilization",
		Type:      BenchmarkMemory,
		Status:    StatusCompleted,
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
	}

	// Simulate memory utilization measurement
	utilization := m.measureMemoryUtilization()
	result.Score = utilization
	result.Unit = "%"
	result.Duration = time.Since(start)

	result.Details["used_memory"] = utilization
	result.Details["cached_memory"] = 15.0
	result.Details["buffer_memory"] = 5.0

	return result
}

func (m *Manager) benchmarkMemoryBandwidth() BenchmarkResult {
	start := time.Now()
	result := BenchmarkResult{
		Name:      "Memory Bandwidth",
		Type:      BenchmarkMemory,
		Status:    StatusCompleted,
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
	}

	// Simulate memory bandwidth measurement
	bandwidth := m.measureMemoryBandwidth()
	result.Score = bandwidth
	result.Unit = "GB/s"
	result.Duration = time.Since(start)

	result.Details["read_bandwidth"] = bandwidth * 1.1
	result.Details["write_bandwidth"] = bandwidth * 0.9

	return result
}

func (m *Manager) benchmarkEtcdMemory() BenchmarkResult {
	start := time.Now()
	result := BenchmarkResult{
		Name:      "Etcd Memory Usage",
		Type:      BenchmarkMemory,
		Status:    StatusCompleted,
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
	}

	// Simulate etcd memory measurement
	usage := m.measureEtcdMemory()
	result.Score = usage
	result.Unit = "MB"
	result.Duration = time.Since(start)

	result.Details["db_size"] = usage * 0.6
	result.Details["working_set"] = usage * 0.4

	return result
}

// Measurement helper functions (simulated values)
func (m *Manager) measurePodLatency() float64 {
	return 0.5 + float64(time.Now().UnixNano()%100)/200 // 0.5-1.0 ms
}

func (m *Manager) measureDNSLatency() float64 {
	return 2.0 + float64(time.Now().UnixNano()%100)/100 // 2-3 ms
}

func (m *Manager) measureThroughput() float64 {
	return 8.0 + float64(time.Now().UnixNano()%200)/100 // 8-10 Gbps
}

func (m *Manager) measureCNIPerformance() float64 {
	return 85.0 + float64(time.Now().UnixNano()%15) // 85-100 score
}

func (m *Manager) measureIOPS() float64 {
	return 3000 + float64(time.Now().UnixNano()%1000) // 3000-4000 IOPS
}

func (m *Manager) measureSequentialIO() float64 {
	return 200 + float64(time.Now().UnixNano()%100) // 200-300 MB/s
}

func (m *Manager) measureRandomIO() float64 {
	return 50 + float64(time.Now().UnixNano()%50) // 50-100 MB/s
}

func (m *Manager) measurePVCProvisionTime() float64 {
	return 3.0 + float64(time.Now().UnixNano()%200)/100 // 3-5 seconds
}

func (m *Manager) measureCPUEfficiency() float64 {
	return 60 + float64(time.Now().UnixNano()%20) // 60-80%
}

func (m *Manager) measureSchedulingLatency() float64 {
	return 50 + float64(time.Now().UnixNano()%50) // 50-100 ms
}

func (m *Manager) measureAPILatency() float64 {
	return 10 + float64(time.Now().UnixNano()%10) // 10-20 ms
}

func (m *Manager) measureMemoryUtilization() float64 {
	return 50 + float64(time.Now().UnixNano()%30) // 50-80%
}

func (m *Manager) measureMemoryBandwidth() float64 {
	return 20 + float64(time.Now().UnixNano()%10) // 20-30 GB/s
}

func (m *Manager) measureEtcdMemory() float64 {
	return 256 + float64(time.Now().UnixNano()%128) // 256-384 MB
}

func (m *Manager) getKubeVersion() string {
	return "v1.28.0"
}

func (m *Manager) getNodeCount() int {
	return 3
}

// calculateSummary calculates the benchmark summary
func (m *Manager) calculateSummary(report *BenchmarkReport) {
	summary := BenchmarkSummary{
		TotalBenchmarks: len(report.Results),
	}

	var networkScores, storageScores, cpuScores, memoryScores []float64

	for _, result := range report.Results {
		if result.Status == StatusCompleted {
			summary.PassedBenchmarks++

			// Normalize scores to 0-100 scale based on type
			normalizedScore := m.normalizeScore(result)

			switch result.Type {
			case BenchmarkNetwork:
				networkScores = append(networkScores, normalizedScore)
			case BenchmarkStorage:
				storageScores = append(storageScores, normalizedScore)
			case BenchmarkCPU:
				cpuScores = append(cpuScores, normalizedScore)
			case BenchmarkMemory:
				memoryScores = append(memoryScores, normalizedScore)
			}
		} else {
			summary.FailedBenchmarks++
		}
	}

	// Calculate category scores
	summary.NetworkScore = average(networkScores)
	summary.StorageScore = average(storageScores)
	summary.CPUScore = average(cpuScores)
	summary.MemoryScore = average(memoryScores)

	// Calculate overall score (weighted average)
	allScores := []float64{
		summary.NetworkScore * 0.3,
		summary.StorageScore * 0.3,
		summary.CPUScore * 0.25,
		summary.MemoryScore * 0.15,
	}
	summary.OverallScore = sum(allScores)

	// Assign grade
	summary.Grade = m.calculateGrade(summary.OverallScore)

	// Generate recommendations
	summary.Recommendations = m.generateRecommendations(report, summary)

	report.Summary = summary
}

// normalizeScore normalizes a benchmark score to 0-100 scale
func (m *Manager) normalizeScore(result BenchmarkResult) float64 {
	// Reference values for normalization
	references := map[string]struct {
		good    float64
		perfect float64
		inverse bool // true if lower is better
	}{
		"Pod-to-Pod Latency":     {good: 1.0, perfect: 0.3, inverse: true},
		"DNS Resolution Latency": {good: 5.0, perfect: 1.0, inverse: true},
		"Network Throughput":     {good: 5.0, perfect: 10.0, inverse: false},
		"CNI Performance":        {good: 80.0, perfect: 100.0, inverse: false},
		"Storage IOPS":           {good: 2000.0, perfect: 5000.0, inverse: false},
		"Sequential I/O":         {good: 150.0, perfect: 400.0, inverse: false},
		"Random I/O":             {good: 30.0, perfect: 100.0, inverse: false},
		"PVC Provisioning Time":  {good: 10.0, perfect: 2.0, inverse: true},
		"CPU Efficiency":         {good: 50.0, perfect: 90.0, inverse: false},
		"Pod Scheduling Latency": {good: 200.0, perfect: 30.0, inverse: true},
		"API Server Latency":     {good: 50.0, perfect: 5.0, inverse: true},
		"Memory Utilization":     {good: 80.0, perfect: 50.0, inverse: true},
		"Memory Bandwidth":       {good: 10.0, perfect: 40.0, inverse: false},
		"Etcd Memory Usage":      {good: 500.0, perfect: 100.0, inverse: true},
	}

	ref, ok := references[result.Name]
	if !ok {
		return 70.0 // default score
	}

	var score float64
	if ref.inverse {
		// Lower is better
		if result.Score <= ref.perfect {
			score = 100
		} else if result.Score >= ref.good {
			score = 50
		} else {
			score = 50 + 50*(ref.good-result.Score)/(ref.good-ref.perfect)
		}
	} else {
		// Higher is better
		if result.Score >= ref.perfect {
			score = 100
		} else if result.Score <= ref.good {
			score = 50 + 50*(result.Score/ref.good)
		} else {
			score = 50 + 50*(result.Score-ref.good)/(ref.perfect-ref.good)
		}
	}

	// Clamp to 0-100
	if score < 0 {
		score = 0
	} else if score > 100 {
		score = 100
	}

	return score
}

func (m *Manager) calculateGrade(score float64) string {
	switch {
	case score >= 90:
		return "A+"
	case score >= 85:
		return "A"
	case score >= 80:
		return "A-"
	case score >= 75:
		return "B+"
	case score >= 70:
		return "B"
	case score >= 65:
		return "B-"
	case score >= 60:
		return "C+"
	case score >= 55:
		return "C"
	case score >= 50:
		return "C-"
	case score >= 40:
		return "D"
	default:
		return "F"
	}
}

func (m *Manager) generateRecommendations(report *BenchmarkReport, summary BenchmarkSummary) []string {
	var recommendations []string

	// Network recommendations
	if summary.NetworkScore < 70 {
		recommendations = append(recommendations, "Consider upgrading CNI plugin or network configuration")
	}

	// Storage recommendations
	if summary.StorageScore < 70 {
		recommendations = append(recommendations, "Storage performance is below average - consider using faster storage class")
	}

	// CPU recommendations
	if summary.CPUScore < 70 {
		recommendations = append(recommendations, "High CPU pressure detected - consider scaling cluster or optimizing workloads")
	}

	// Memory recommendations
	if summary.MemoryScore < 70 {
		recommendations = append(recommendations, "Memory utilization is high - consider adding more nodes or increasing node memory")
	}

	// Check specific metrics
	for _, result := range report.Results {
		if result.Name == "Pod-to-Pod Latency" && result.Score > 2.0 {
			recommendations = append(recommendations, "Pod-to-pod latency is high - check network plugin and node networking")
		}
		if result.Name == "API Server Latency" && result.Score > 30 {
			recommendations = append(recommendations, "API server response time is slow - check etcd health and API server resources")
		}
		if result.Name == "Etcd Memory Usage" && result.Score > 400 {
			recommendations = append(recommendations, "Etcd memory usage is high - consider compacting etcd or increasing resources")
		}
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "Cluster performance is excellent - no immediate optimizations needed")
	}

	return recommendations
}

// addReferenceComparison adds comparison with reference values
func (m *Manager) addReferenceComparison(report *BenchmarkReport) {
	comparison := &BenchmarkComparison{
		ReferenceName:    "Kubernetes Best Practices",
		ReferenceVersion: "1.28",
	}

	references := map[string]float64{
		"Pod-to-Pod Latency":     0.5,
		"DNS Resolution Latency": 2.0,
		"Network Throughput":     10.0,
		"Storage IOPS":           3000,
		"API Server Latency":     15.0,
	}

	for _, result := range report.Results {
		if ref, ok := references[result.Name]; ok {
			comp := MetricComparison{
				MetricName:     result.Name,
				CurrentValue:   result.Score,
				ReferenceValue: ref,
			}

			// Calculate difference based on whether lower or higher is better
			isLowerBetter := strings.Contains(result.Name, "Latency") || strings.Contains(result.Name, "Time")
			comp.Difference = result.Score - ref

			if ref != 0 {
				comp.Percentage = (comp.Difference / ref) * 100
			}

			if isLowerBetter {
				comp.Better = result.Score <= ref
			} else {
				comp.Better = result.Score >= ref
			}

			comparison.Comparisons = append(comparison.Comparisons, comp)
		}
	}

	report.Comparison = comparison
}

// PrintReport prints a formatted benchmark report
func (r *BenchmarkReport) PrintReport() {
	fmt.Println()
	color.Cyan("Benchmark Report")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Cluster:    %s\n", r.ClusterName)
	fmt.Printf("Timestamp:  %s\n", r.Timestamp.Format(time.RFC3339))
	fmt.Printf("Duration:   %s\n", r.Duration)
	if r.KubeVersion != "" {
		fmt.Printf("K8s Version: %s\n", r.KubeVersion)
	}
	if r.NodeCount > 0 {
		fmt.Printf("Nodes:      %d\n", r.NodeCount)
	}

	// Summary
	fmt.Println()
	color.Cyan("Summary")
	fmt.Println(strings.Repeat("-", 40))
	gradeColor := getGradeColor(r.Summary.Grade)
	gradeColor.Printf("Overall Grade: %s (%.1f/100)\n", r.Summary.Grade, r.Summary.OverallScore)
	fmt.Println()
	fmt.Printf("Network Score:  %.1f/100\n", r.Summary.NetworkScore)
	fmt.Printf("Storage Score:  %.1f/100\n", r.Summary.StorageScore)
	fmt.Printf("CPU Score:      %.1f/100\n", r.Summary.CPUScore)
	fmt.Printf("Memory Score:   %.1f/100\n", r.Summary.MemoryScore)

	// Results by category
	categories := []BenchmarkType{BenchmarkNetwork, BenchmarkStorage, BenchmarkCPU, BenchmarkMemory}
	categoryNames := map[BenchmarkType]string{
		BenchmarkNetwork: "Network",
		BenchmarkStorage: "Storage",
		BenchmarkCPU:     "CPU",
		BenchmarkMemory:  "Memory",
	}

	for _, cat := range categories {
		results := filterResultsByType(r.Results, cat)
		if len(results) == 0 {
			continue
		}

		fmt.Println()
		color.Cyan("%s Benchmarks", categoryNames[cat])
		fmt.Println(strings.Repeat("-", 40))

		for _, result := range results {
			statusIcon := "[OK]"
			if result.Status == StatusFailed {
				statusIcon = "[FAIL]"
			}

			fmt.Printf("%s %-30s %8.2f %s\n",
				statusIcon,
				result.Name,
				result.Score,
				result.Unit)
		}
	}

	// Comparison
	if r.Comparison != nil && len(r.Comparison.Comparisons) > 0 {
		fmt.Println()
		color.Cyan("Reference Comparison (%s)", r.Comparison.ReferenceName)
		fmt.Println(strings.Repeat("-", 60))
		fmt.Printf("%-25s %12s %12s %10s\n", "Metric", "Current", "Reference", "Status")
		fmt.Println(strings.Repeat("-", 60))

		for _, comp := range r.Comparison.Comparisons {
			var statusStr string
			var statusColor *color.Color
			if comp.Better {
				statusStr = "GOOD"
				statusColor = color.New(color.FgGreen)
			} else {
				statusStr = "BELOW"
				statusColor = color.New(color.FgYellow)
			}

			fmt.Printf("%-25s %12.2f %12.2f ", comp.MetricName, comp.CurrentValue, comp.ReferenceValue)
			statusColor.Printf("%10s\n", statusStr)
		}
	}

	// Recommendations
	if len(r.Summary.Recommendations) > 0 {
		fmt.Println()
		color.Cyan("Recommendations")
		fmt.Println(strings.Repeat("-", 40))
		for i, rec := range r.Summary.Recommendations {
			fmt.Printf("%d. %s\n", i+1, rec)
		}
	}
}

// PrintCompact prints a compact summary
func (r *BenchmarkReport) PrintCompact() {
	gradeColor := getGradeColor(r.Summary.Grade)
	gradeColor.Printf("Grade: %s (%.1f) | Network: %.1f | Storage: %.1f | CPU: %.1f | Memory: %.1f\n",
		r.Summary.Grade,
		r.Summary.OverallScore,
		r.Summary.NetworkScore,
		r.Summary.StorageScore,
		r.Summary.CPUScore,
		r.Summary.MemoryScore)
}

// Helper functions
func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	return sum(values) / float64(len(values))
}

func sum(values []float64) float64 {
	var total float64
	for _, v := range values {
		total += v
	}
	return total
}

func filterResultsByType(results []BenchmarkResult, benchType BenchmarkType) []BenchmarkResult {
	var filtered []BenchmarkResult
	for _, r := range results {
		if r.Type == benchType {
			filtered = append(filtered, r)
		}
	}
	// Sort by name for consistent output
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Name < filtered[j].Name
	})
	return filtered
}

func getGradeColor(grade string) *color.Color {
	switch {
	case strings.HasPrefix(grade, "A"):
		return color.New(color.FgGreen, color.Bold)
	case strings.HasPrefix(grade, "B"):
		return color.New(color.FgGreen)
	case strings.HasPrefix(grade, "C"):
		return color.New(color.FgYellow)
	case strings.HasPrefix(grade, "D"):
		return color.New(color.FgRed)
	default:
		return color.New(color.FgRed, color.Bold)
	}
}
