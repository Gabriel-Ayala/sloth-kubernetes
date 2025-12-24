package benchmark

import (
	"strings"
	"testing"
)

func TestBenchmarkTypeConstants(t *testing.T) {
	tests := []struct {
		name     string
		bType    BenchmarkType
		expected string
	}{
		{"BenchmarkNetwork", BenchmarkNetwork, "network"},
		{"BenchmarkStorage", BenchmarkStorage, "storage"},
		{"BenchmarkCPU", BenchmarkCPU, "cpu"},
		{"BenchmarkMemory", BenchmarkMemory, "memory"},
		{"BenchmarkAll", BenchmarkAll, "all"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.bType) != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, tt.bType)
			}
		})
	}
}

func TestBenchmarkStatusConstants(t *testing.T) {
	tests := []struct {
		name     string
		status   BenchmarkStatus
		expected string
	}{
		{"StatusPending", StatusPending, "pending"},
		{"StatusRunning", StatusRunning, "running"},
		{"StatusCompleted", StatusCompleted, "completed"},
		{"StatusFailed", StatusFailed, "failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.status) != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, tt.status)
			}
		})
	}
}

func TestNewManager(t *testing.T) {
	masterIP := "192.168.1.100"
	sshKey := "/path/to/key"
	kubeconfig := "/path/to/kubeconfig"

	manager := NewManager(masterIP, sshKey, kubeconfig)

	if manager.masterIP != masterIP {
		t.Errorf("expected masterIP %s, got %s", masterIP, manager.masterIP)
	}
	if manager.sshKey != sshKey {
		t.Errorf("expected sshKey %s, got %s", sshKey, manager.sshKey)
	}
	if manager.kubeconfig != kubeconfig {
		t.Errorf("expected kubeconfig %s, got %s", kubeconfig, manager.kubeconfig)
	}
}

func TestManagerSetVerbose(t *testing.T) {
	manager := NewManager("", "", "")
	manager.SetVerbose(true)

	if manager.verbose != true {
		t.Error("verbose should be true")
	}
}

func TestManagerSetNodeFilter(t *testing.T) {
	manager := NewManager("", "", "")
	nodes := []string{"node-1", "node-2"}
	manager.SetNodeFilter(nodes)

	if len(manager.nodeFilter) != 2 {
		t.Errorf("expected 2 node filters, got %d", len(manager.nodeFilter))
	}
}

func TestRunBenchmarkNetwork(t *testing.T) {
	manager := NewManager("", "", "")

	report, err := manager.RunBenchmark(BenchmarkNetwork)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if report == nil {
		t.Fatal("expected report, got nil")
	}

	// Check that only network benchmarks were run
	for _, result := range report.Results {
		if result.Type != BenchmarkNetwork {
			t.Errorf("expected only network benchmarks, got %s", result.Type)
		}
	}

	// Should have 4 network benchmarks
	if len(report.Results) != 4 {
		t.Errorf("expected 4 network benchmarks, got %d", len(report.Results))
	}
}

func TestRunBenchmarkStorage(t *testing.T) {
	manager := NewManager("", "", "")

	report, err := manager.RunBenchmark(BenchmarkStorage)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	for _, result := range report.Results {
		if result.Type != BenchmarkStorage {
			t.Errorf("expected only storage benchmarks, got %s", result.Type)
		}
	}

	// Should have 4 storage benchmarks
	if len(report.Results) != 4 {
		t.Errorf("expected 4 storage benchmarks, got %d", len(report.Results))
	}
}

func TestRunBenchmarkCPU(t *testing.T) {
	manager := NewManager("", "", "")

	report, err := manager.RunBenchmark(BenchmarkCPU)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	for _, result := range report.Results {
		if result.Type != BenchmarkCPU {
			t.Errorf("expected only CPU benchmarks, got %s", result.Type)
		}
	}

	// Should have 3 CPU benchmarks
	if len(report.Results) != 3 {
		t.Errorf("expected 3 CPU benchmarks, got %d", len(report.Results))
	}
}

func TestRunBenchmarkMemory(t *testing.T) {
	manager := NewManager("", "", "")

	report, err := manager.RunBenchmark(BenchmarkMemory)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	for _, result := range report.Results {
		if result.Type != BenchmarkMemory {
			t.Errorf("expected only memory benchmarks, got %s", result.Type)
		}
	}

	// Should have 3 memory benchmarks
	if len(report.Results) != 3 {
		t.Errorf("expected 3 memory benchmarks, got %d", len(report.Results))
	}
}

func TestRunBenchmarkAll(t *testing.T) {
	manager := NewManager("", "", "")

	report, err := manager.RunBenchmark(BenchmarkAll)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Should have all benchmarks (4+4+3+3 = 14)
	if len(report.Results) != 14 {
		t.Errorf("expected 14 total benchmarks, got %d", len(report.Results))
	}

	// Check for each type
	typeCount := map[BenchmarkType]int{}
	for _, result := range report.Results {
		typeCount[result.Type]++
	}

	if typeCount[BenchmarkNetwork] != 4 {
		t.Errorf("expected 4 network benchmarks, got %d", typeCount[BenchmarkNetwork])
	}
	if typeCount[BenchmarkStorage] != 4 {
		t.Errorf("expected 4 storage benchmarks, got %d", typeCount[BenchmarkStorage])
	}
	if typeCount[BenchmarkCPU] != 3 {
		t.Errorf("expected 3 CPU benchmarks, got %d", typeCount[BenchmarkCPU])
	}
	if typeCount[BenchmarkMemory] != 3 {
		t.Errorf("expected 3 memory benchmarks, got %d", typeCount[BenchmarkMemory])
	}
}

func TestBenchmarkResultStatus(t *testing.T) {
	manager := NewManager("", "", "")

	report, err := manager.RunBenchmark(BenchmarkAll)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// All results should be completed
	for _, result := range report.Results {
		if result.Status != StatusCompleted {
			t.Errorf("expected status completed, got %s for %s", result.Status, result.Name)
		}
	}
}

func TestBenchmarkResultHasDetails(t *testing.T) {
	manager := NewManager("", "", "")

	report, err := manager.RunBenchmark(BenchmarkNetwork)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	for _, result := range report.Results {
		if result.Details == nil {
			t.Errorf("expected details for %s, got nil", result.Name)
		}
		if len(result.Details) == 0 {
			t.Errorf("expected non-empty details for %s", result.Name)
		}
	}
}

func TestBenchmarkSummaryCalculation(t *testing.T) {
	manager := NewManager("", "", "")

	report, err := manager.RunBenchmark(BenchmarkAll)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Check summary is populated
	if report.Summary.TotalBenchmarks == 0 {
		t.Error("expected TotalBenchmarks > 0")
	}

	if report.Summary.OverallScore == 0 {
		t.Error("expected OverallScore > 0")
	}

	if report.Summary.Grade == "" {
		t.Error("expected Grade to be set")
	}

	// Total should match results
	if report.Summary.TotalBenchmarks != len(report.Results) {
		t.Errorf("TotalBenchmarks (%d) should match Results count (%d)",
			report.Summary.TotalBenchmarks, len(report.Results))
	}
}

func TestCalculateGrade(t *testing.T) {
	manager := NewManager("", "", "")

	tests := []struct {
		score    float64
		expected string
	}{
		{95, "A+"},
		{87, "A"},
		{82, "A-"},
		{76, "B+"},
		{71, "B"},
		{67, "B-"},
		{61, "C+"},
		{56, "C"},
		{51, "C-"},
		{41, "D"},
		{30, "F"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := manager.calculateGrade(tt.score)
			if result != tt.expected {
				t.Errorf("calculateGrade(%f) = %s, want %s", tt.score, result, tt.expected)
			}
		})
	}
}

func TestGenerateRecommendations(t *testing.T) {
	manager := NewManager("", "", "")

	report := &BenchmarkReport{
		Results: []BenchmarkResult{},
	}

	summary := BenchmarkSummary{
		NetworkScore: 60,
		StorageScore: 60,
		CPUScore:     60,
		MemoryScore:  60,
	}

	recommendations := manager.generateRecommendations(report, summary)

	// Should have recommendations for all categories below 70
	if len(recommendations) == 0 {
		t.Error("expected recommendations for low scores")
	}

	// Check for specific recommendations
	hasNetworkRec := false
	hasStorageRec := false
	hasCPURec := false
	hasMemoryRec := false

	for _, rec := range recommendations {
		if strings.Contains(rec, "CNI") || strings.Contains(rec, "network") {
			hasNetworkRec = true
		}
		if strings.Contains(rec, "storage") || strings.Contains(rec, "Storage") {
			hasStorageRec = true
		}
		if strings.Contains(rec, "CPU") {
			hasCPURec = true
		}
		if strings.Contains(rec, "Memory") || strings.Contains(rec, "memory") {
			hasMemoryRec = true
		}
	}

	if !hasNetworkRec {
		t.Error("expected network recommendation")
	}
	if !hasStorageRec {
		t.Error("expected storage recommendation")
	}
	if !hasCPURec {
		t.Error("expected CPU recommendation")
	}
	if !hasMemoryRec {
		t.Error("expected memory recommendation")
	}
}

func TestGenerateRecommendationsExcellent(t *testing.T) {
	manager := NewManager("", "", "")

	report := &BenchmarkReport{
		Results: []BenchmarkResult{},
	}

	summary := BenchmarkSummary{
		NetworkScore: 90,
		StorageScore: 90,
		CPUScore:     90,
		MemoryScore:  90,
	}

	recommendations := manager.generateRecommendations(report, summary)

	// Should have exactly one recommendation saying performance is excellent
	if len(recommendations) != 1 {
		t.Errorf("expected 1 recommendation, got %d", len(recommendations))
	}

	if !strings.Contains(recommendations[0], "excellent") {
		t.Error("expected excellence recommendation")
	}
}

func TestNormalizeScore(t *testing.T) {
	manager := NewManager("", "", "")

	// Test Pod-to-Pod Latency (lower is better)
	result := BenchmarkResult{
		Name:  "Pod-to-Pod Latency",
		Score: 0.3, // Perfect score
	}
	score := manager.normalizeScore(result)
	if score != 100 {
		t.Errorf("expected 100 for perfect latency, got %f", score)
	}

	// Test Network Throughput (higher is better)
	result = BenchmarkResult{
		Name:  "Network Throughput",
		Score: 10.0, // Perfect score
	}
	score = manager.normalizeScore(result)
	if score != 100 {
		t.Errorf("expected 100 for perfect throughput, got %f", score)
	}

	// Test unknown benchmark
	result = BenchmarkResult{
		Name:  "Unknown Benchmark",
		Score: 50,
	}
	score = manager.normalizeScore(result)
	if score != 70 {
		t.Errorf("expected 70 for unknown benchmark, got %f", score)
	}
}

func TestReferenceComparison(t *testing.T) {
	manager := NewManager("", "", "")

	report, err := manager.RunBenchmark(BenchmarkNetwork)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if report.Comparison == nil {
		t.Error("expected comparison to be set")
	}

	if report.Comparison.ReferenceName == "" {
		t.Error("expected ReferenceName to be set")
	}

	if len(report.Comparison.Comparisons) == 0 {
		t.Error("expected at least one comparison")
	}
}

func TestMetricComparisonFields(t *testing.T) {
	manager := NewManager("", "", "")

	report, err := manager.RunBenchmark(BenchmarkNetwork)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	for _, comp := range report.Comparison.Comparisons {
		if comp.MetricName == "" {
			t.Error("expected MetricName to be set")
		}
		// CurrentValue and ReferenceValue can be any value
	}
}

func TestBenchmarkReportFields(t *testing.T) {
	manager := NewManager("", "", "")

	report, err := manager.RunBenchmark(BenchmarkAll)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if report.ClusterName == "" {
		t.Error("expected ClusterName to be set")
	}

	if report.Timestamp.IsZero() {
		t.Error("expected Timestamp to be set")
	}

	if report.Duration == 0 {
		t.Error("expected Duration to be > 0")
	}
}

func TestAverageFunction(t *testing.T) {
	tests := []struct {
		values   []float64
		expected float64
	}{
		{[]float64{10, 20, 30}, 20},
		{[]float64{100}, 100},
		{[]float64{}, 0},
		{[]float64{5, 5, 5, 5}, 5},
	}

	for _, tt := range tests {
		result := average(tt.values)
		if result != tt.expected {
			t.Errorf("average(%v) = %f, want %f", tt.values, result, tt.expected)
		}
	}
}

func TestSumFunction(t *testing.T) {
	tests := []struct {
		values   []float64
		expected float64
	}{
		{[]float64{10, 20, 30}, 60},
		{[]float64{100}, 100},
		{[]float64{}, 0},
	}

	for _, tt := range tests {
		result := sum(tt.values)
		if result != tt.expected {
			t.Errorf("sum(%v) = %f, want %f", tt.values, result, tt.expected)
		}
	}
}

func TestFilterResultsByType(t *testing.T) {
	results := []BenchmarkResult{
		{Name: "net1", Type: BenchmarkNetwork},
		{Name: "stor1", Type: BenchmarkStorage},
		{Name: "net2", Type: BenchmarkNetwork},
		{Name: "cpu1", Type: BenchmarkCPU},
	}

	filtered := filterResultsByType(results, BenchmarkNetwork)
	if len(filtered) != 2 {
		t.Errorf("expected 2 network results, got %d", len(filtered))
	}

	for _, r := range filtered {
		if r.Type != BenchmarkNetwork {
			t.Errorf("expected network type, got %s", r.Type)
		}
	}

	// Filtered should be sorted by name
	if filtered[0].Name > filtered[1].Name {
		t.Error("results should be sorted by name")
	}
}

func TestBenchmarkResultFieldsPopulated(t *testing.T) {
	manager := NewManager("", "", "")

	report, err := manager.RunBenchmark(BenchmarkNetwork)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	for _, result := range report.Results {
		if result.Name == "" {
			t.Error("Name should be set")
		}
		if result.Type == "" {
			t.Error("Type should be set")
		}
		if result.Unit == "" {
			t.Error("Unit should be set")
		}
		if result.Timestamp.IsZero() {
			t.Error("Timestamp should be set")
		}
	}
}

func TestNodeBenchmarkResultFields(t *testing.T) {
	result := NodeBenchmarkResult{
		NodeName: "node-1",
		Score:    85.5,
		Unit:     "score",
		Status:   StatusCompleted,
		Details:  map[string]interface{}{"key": "value"},
		Error:    "",
	}

	if result.NodeName != "node-1" {
		t.Errorf("unexpected NodeName: %s", result.NodeName)
	}
	if result.Score != 85.5 {
		t.Errorf("unexpected Score: %f", result.Score)
	}
	if result.Status != StatusCompleted {
		t.Errorf("unexpected Status: %s", result.Status)
	}
}

func TestBenchmarkSummaryFields(t *testing.T) {
	summary := BenchmarkSummary{
		TotalBenchmarks:  14,
		PassedBenchmarks: 14,
		FailedBenchmarks: 0,
		OverallScore:     85.0,
		NetworkScore:     90.0,
		StorageScore:     80.0,
		CPUScore:         85.0,
		MemoryScore:      75.0,
		Grade:            "A",
		Recommendations:  []string{"Recommendation 1"},
	}

	if summary.TotalBenchmarks != 14 {
		t.Errorf("unexpected TotalBenchmarks: %d", summary.TotalBenchmarks)
	}
	if summary.Grade != "A" {
		t.Errorf("unexpected Grade: %s", summary.Grade)
	}
}

func TestGetGradeColor(t *testing.T) {
	// Just verify the function doesn't panic for various grades
	grades := []string{"A+", "A", "A-", "B+", "B", "B-", "C+", "C", "C-", "D", "F"}
	for _, grade := range grades {
		color := getGradeColor(grade)
		if color == nil {
			t.Errorf("expected color for grade %s, got nil", grade)
		}
	}
}

func TestMeasurementFunctions(t *testing.T) {
	manager := NewManager("", "", "")

	// Test that measurement functions return reasonable values
	latency := manager.measurePodLatency()
	if latency < 0.5 || latency > 1.0 {
		t.Errorf("unexpected pod latency: %f", latency)
	}

	dnsLatency := manager.measureDNSLatency()
	if dnsLatency < 2.0 || dnsLatency > 3.0 {
		t.Errorf("unexpected DNS latency: %f", dnsLatency)
	}

	throughput := manager.measureThroughput()
	if throughput < 8.0 || throughput > 10.0 {
		t.Errorf("unexpected throughput: %f", throughput)
	}
}
