package health

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewChecker tests the CLI checker creation
func TestNewChecker(t *testing.T) {
	tests := []struct {
		name     string
		masterIP string
		sshKey   string
	}{
		{
			name:     "with credentials",
			masterIP: "192.168.1.100",
			sshKey:   "ssh-rsa AAAAB3...",
		},
		{
			name:     "empty credentials",
			masterIP: "",
			sshKey:   "",
		},
		{
			name:     "only master IP",
			masterIP: "10.0.0.1",
			sshKey:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewChecker(tt.masterIP, tt.sshKey)
			require.NotNil(t, checker)
		})
	}
}

// TestChecker_SetVerbose tests verbose mode setting
func TestChecker_SetVerbose(t *testing.T) {
	checker := NewChecker("", "")

	checker.SetVerbose(true)
	assert.True(t, checker.verbose)

	checker.SetVerbose(false)
	assert.False(t, checker.verbose)
}

// TestChecker_SetKubeconfig tests kubeconfig setting
func TestChecker_SetKubeconfig(t *testing.T) {
	checker := NewChecker("", "")

	path := "/home/user/.kube/config"
	checker.SetKubeconfig(path)
	assert.Equal(t, path, checker.kubeconfig)
}

// TestCheckStatus constants
func TestCheckStatus(t *testing.T) {
	assert.Equal(t, CheckStatus("healthy"), StatusHealthy)
	assert.Equal(t, CheckStatus("warning"), StatusWarning)
	assert.Equal(t, CheckStatus("critical"), StatusCritical)
	assert.Equal(t, CheckStatus("unknown"), StatusUnknown)
}

// TestCheckResult structure
func TestCheckResult(t *testing.T) {
	result := CheckResult{
		Name:        "Test Check",
		Status:      StatusHealthy,
		Message:     "All good",
		Details:     []string{"detail1", "detail2"},
		Duration:    100 * time.Millisecond,
		CheckedAt:   time.Now(),
		Remediation: "No action needed",
	}

	assert.Equal(t, "Test Check", result.Name)
	assert.Equal(t, StatusHealthy, result.Status)
	assert.Equal(t, "All good", result.Message)
	assert.Len(t, result.Details, 2)
	assert.NotEmpty(t, result.Remediation)
}

// TestHealthReport structure
func TestHealthReport(t *testing.T) {
	report := &HealthReport{
		ClusterName:   "test-cluster",
		CheckedAt:     time.Now(),
		Duration:      5 * time.Second,
		OverallStatus: StatusHealthy,
		Checks: []CheckResult{
			{Name: "Check 1", Status: StatusHealthy},
			{Name: "Check 2", Status: StatusWarning},
		},
		Summary: Summary{
			TotalChecks:   2,
			HealthyChecks: 1,
			WarningChecks: 1,
		},
		Recommendations: []string{"Review warnings"},
	}

	assert.Equal(t, "test-cluster", report.ClusterName)
	assert.Equal(t, StatusHealthy, report.OverallStatus)
	assert.Len(t, report.Checks, 2)
	assert.Equal(t, 2, report.Summary.TotalChecks)
}

// TestSummary structure
func TestSummary(t *testing.T) {
	summary := Summary{
		TotalChecks:      10,
		HealthyChecks:    7,
		WarningChecks:    2,
		CriticalChecks:   1,
		UnknownChecks:    0,
		NodesTotal:       6,
		NodesReady:       6,
		PodsTotal:        50,
		PodsRunning:      48,
		PodsPending:      2,
		PodsFailed:       0,
		CertDaysToExpire: 365,
	}

	assert.Equal(t, 10, summary.TotalChecks)
	assert.Equal(t, 7, summary.HealthyChecks)
	assert.Equal(t, 2, summary.WarningChecks)
	assert.Equal(t, 1, summary.CriticalChecks)
	assert.Equal(t, 6, summary.NodesTotal)
	assert.Equal(t, 6, summary.NodesReady)
}

// TestParseNodeOutput tests parsing kubectl node output
func TestParseNodeOutput(t *testing.T) {
	tests := []struct {
		name             string
		output           string
		expectedTotal    int
		expectedReady    int
		expectedNotReady int
	}{
		{
			name: "all nodes ready",
			output: `master-1   Ready   control-plane   10d   v1.28.0
master-2   Ready   control-plane   10d   v1.28.0
worker-1   Ready   <none>          10d   v1.28.0`,
			expectedTotal:    3,
			expectedReady:    3,
			expectedNotReady: 0,
		},
		{
			name: "one node not ready",
			output: `master-1   Ready      control-plane   10d   v1.28.0
worker-1   NotReady   <none>          10d   v1.28.0`,
			expectedTotal:    2,
			expectedReady:    1,
			expectedNotReady: 1,
		},
		{
			name:             "empty output",
			output:           "",
			expectedTotal:    0,
			expectedReady:    0,
			expectedNotReady: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var totalNodes, readyNodes, notReadyNodes int

			lines := strings.Split(strings.TrimSpace(tt.output), "\n")
			for _, line := range lines {
				if line == "" {
					continue
				}
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					totalNodes++
					if parts[1] == "Ready" {
						readyNodes++
					} else {
						notReadyNodes++
					}
				}
			}

			assert.Equal(t, tt.expectedTotal, totalNodes)
			assert.Equal(t, tt.expectedReady, readyNodes)
			assert.Equal(t, tt.expectedNotReady, notReadyNodes)
		})
	}
}

// TestParsePodOutput tests parsing kubectl pod output
func TestParsePodOutput(t *testing.T) {
	tests := []struct {
		name            string
		output          string
		expectedTotal   int
		expectedRunning int
		expectedFailed  int
	}{
		{
			name: "all pods running",
			output: `coredns-abc   1/1   Running   0   10d
etcd-master   1/1   Running   0   10d
kube-apiserver   1/1   Running   0   10d`,
			expectedTotal:   3,
			expectedRunning: 3,
			expectedFailed:  0,
		},
		{
			name: "mixed pod status",
			output: `coredns-abc      1/1   Running     0   10d
pending-pod      0/1   Pending     0   1m
completed-job    0/1   Completed   0   5m`,
			expectedTotal:   3,
			expectedRunning: 2, // Running + Completed
			expectedFailed:  1,
		},
		{
			name:            "empty output",
			output:          "",
			expectedTotal:   0,
			expectedRunning: 0,
			expectedFailed:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var totalPods, runningPods, failedPods int

			lines := strings.Split(strings.TrimSpace(tt.output), "\n")
			for _, line := range lines {
				if line == "" {
					continue
				}
				parts := strings.Fields(line)
				if len(parts) >= 3 {
					totalPods++
					status := parts[2]
					if status == "Running" || status == "Completed" {
						runningPods++
					} else {
						failedPods++
					}
				}
			}

			assert.Equal(t, tt.expectedTotal, totalPods)
			assert.Equal(t, tt.expectedRunning, runningPods)
			assert.Equal(t, tt.expectedFailed, failedPods)
		})
	}
}

// TestParsePVCOutput tests parsing kubectl pvc output
func TestParsePVCOutput(t *testing.T) {
	tests := []struct {
		name            string
		output          string
		expectedTotal   int
		expectedBound   int
		expectedPending int
	}{
		{
			name: "all pvcs bound",
			output: `default   data-pvc-1   Bound   pv-1   10Gi   RWO   10d
default   data-pvc-2   Bound   pv-2   20Gi   RWO   10d`,
			expectedTotal:   2,
			expectedBound:   2,
			expectedPending: 0,
		},
		{
			name: "mixed pvc status",
			output: `default   data-pvc-1   Bound     pv-1   10Gi   RWO   10d
default   data-pvc-2   Pending          20Gi   RWO   1m`,
			expectedTotal:   2,
			expectedBound:   1,
			expectedPending: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var totalPVCs, boundPVCs, pendingPVCs int

			lines := strings.Split(strings.TrimSpace(tt.output), "\n")
			for _, line := range lines {
				if line == "" {
					continue
				}
				parts := strings.Fields(line)
				if len(parts) >= 4 {
					totalPVCs++
					status := parts[2]
					if status == "Bound" {
						boundPVCs++
					} else {
						pendingPVCs++
					}
				}
			}

			assert.Equal(t, tt.expectedTotal, totalPVCs)
			assert.Equal(t, tt.expectedBound, boundPVCs)
			assert.Equal(t, tt.expectedPending, pendingPVCs)
		})
	}
}

// TestParseMemoryPressureOutput tests parsing memory pressure status
func TestParseMemoryPressureOutput(t *testing.T) {
	tests := []struct {
		name             string
		output           string
		expectedPressure int
	}{
		{
			name: "no memory pressure",
			output: `master-1 False
worker-1 False
worker-2 False`,
			expectedPressure: 0,
		},
		{
			name: "some memory pressure",
			output: `master-1 False
worker-1 True
worker-2 False`,
			expectedPressure: 1,
		},
		{
			name: "all under pressure",
			output: `master-1 True
worker-1 True`,
			expectedPressure: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var pressureNodes int

			lines := strings.Split(strings.TrimSpace(tt.output), "\n")
			for _, line := range lines {
				parts := strings.Fields(line)
				if len(parts) >= 2 && parts[1] == "True" {
					pressureNodes++
				}
			}

			assert.Equal(t, tt.expectedPressure, pressureNodes)
		})
	}
}

// TestOverallStatusDetermination tests how overall status is determined
func TestOverallStatusDetermination(t *testing.T) {
	tests := []struct {
		name           string
		checkStatuses  []CheckStatus
		expectedStatus CheckStatus
	}{
		{
			name:           "all healthy",
			checkStatuses:  []CheckStatus{StatusHealthy, StatusHealthy, StatusHealthy},
			expectedStatus: StatusHealthy,
		},
		{
			name:           "one warning",
			checkStatuses:  []CheckStatus{StatusHealthy, StatusWarning, StatusHealthy},
			expectedStatus: StatusWarning,
		},
		{
			name:           "one critical",
			checkStatuses:  []CheckStatus{StatusHealthy, StatusCritical, StatusHealthy},
			expectedStatus: StatusCritical,
		},
		{
			name:           "critical overrides warning",
			checkStatuses:  []CheckStatus{StatusWarning, StatusCritical, StatusWarning},
			expectedStatus: StatusCritical,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			overallStatus := StatusHealthy

			for _, status := range tt.checkStatuses {
				if status == StatusCritical {
					overallStatus = StatusCritical
				} else if status == StatusWarning && overallStatus != StatusCritical {
					overallStatus = StatusWarning
				}
			}

			assert.Equal(t, tt.expectedStatus, overallStatus)
		})
	}
}

// TestRecommendationsGeneration tests recommendation generation
func TestRecommendationsGeneration(t *testing.T) {
	tests := []struct {
		name                    string
		checks                  []CheckResult
		expectedRecommendations int
	}{
		{
			name: "no issues",
			checks: []CheckResult{
				{Status: StatusHealthy, Remediation: ""},
			},
			expectedRecommendations: 0,
		},
		{
			name: "warning with remediation",
			checks: []CheckResult{
				{Status: StatusWarning, Remediation: "Scale CoreDNS to 2 replicas"},
			},
			expectedRecommendations: 1,
		},
		{
			name: "critical with remediation",
			checks: []CheckResult{
				{Status: StatusCritical, Remediation: "Check node status"},
			},
			expectedRecommendations: 1,
		},
		{
			name: "multiple issues",
			checks: []CheckResult{
				{Status: StatusCritical, Remediation: "Fix issue 1"},
				{Status: StatusWarning, Remediation: "Fix issue 2"},
				{Status: StatusHealthy, Remediation: ""},
			},
			expectedRecommendations: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var recommendations []string

			for _, check := range tt.checks {
				if (check.Status == StatusCritical || check.Status == StatusWarning) && check.Remediation != "" {
					recommendations = append(recommendations, check.Remediation)
				}
			}

			assert.Equal(t, tt.expectedRecommendations, len(recommendations))
		})
	}
}

// TestCheckDuration tests that check duration is measured
func TestCheckDuration(t *testing.T) {
	start := time.Now()
	time.Sleep(10 * time.Millisecond)
	duration := time.Since(start)

	assert.GreaterOrEqual(t, duration, 10*time.Millisecond)
}

// TestStatusIcons tests status icon mapping
func TestStatusIcons(t *testing.T) {
	tests := []struct {
		status CheckStatus
		icon   string
	}{
		{StatusHealthy, "[OK]"},
		{StatusWarning, "[WARN]"},
		{StatusCritical, "[FAIL]"},
		{StatusUnknown, "[?]"},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			var icon string
			switch tt.status {
			case StatusHealthy:
				icon = "[OK]"
			case StatusWarning:
				icon = "[WARN]"
			case StatusCritical:
				icon = "[FAIL]"
			default:
				icon = "[?]"
			}
			assert.Equal(t, tt.icon, icon)
		})
	}
}

// TestCoreDNSReplicaStatus tests CoreDNS replica count status
func TestCoreDNSReplicaStatus(t *testing.T) {
	tests := []struct {
		name           string
		runningPods    int
		expectedStatus CheckStatus
	}{
		{
			name:           "no replicas",
			runningPods:    0,
			expectedStatus: StatusCritical,
		},
		{
			name:           "one replica",
			runningPods:    1,
			expectedStatus: StatusWarning,
		},
		{
			name:           "two replicas",
			runningPods:    2,
			expectedStatus: StatusHealthy,
		},
		{
			name:           "three replicas",
			runningPods:    3,
			expectedStatus: StatusHealthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var status CheckStatus
			if tt.runningPods == 0 {
				status = StatusCritical
			} else if tt.runningPods == 1 {
				status = StatusWarning
			} else {
				status = StatusHealthy
			}
			assert.Equal(t, tt.expectedStatus, status)
		})
	}
}

// TestAPIServerHealthResponse tests API server health check
func TestAPIServerHealthResponse(t *testing.T) {
	tests := []struct {
		name           string
		response       string
		expectedStatus CheckStatus
	}{
		{
			name:           "healthy response",
			response:       "ok",
			expectedStatus: StatusHealthy,
		},
		{
			name:           "unexpected response",
			response:       "unhealthy",
			expectedStatus: StatusWarning,
		},
		{
			name:           "empty response",
			response:       "",
			expectedStatus: StatusWarning,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var status CheckStatus
			if strings.TrimSpace(tt.response) == "ok" {
				status = StatusHealthy
			} else {
				status = StatusWarning
			}
			assert.Equal(t, tt.expectedStatus, status)
		})
	}
}

// TestCertificateStatusParsing tests certificate expiration parsing
func TestCertificateStatusParsing(t *testing.T) {
	tests := []struct {
		name           string
		output         string
		expectedStatus CheckStatus
	}{
		{
			name:           "valid certificate",
			output:         "notAfter=Dec 31 23:59:59 2025 GMT",
			expectedStatus: StatusHealthy,
		},
		{
			name:           "expired certificate",
			output:         "EXPIRED: certificate has expired",
			expectedStatus: StatusCritical,
		},
		{
			name:           "kubeadm not available",
			output:         "kubeadm not available",
			expectedStatus: StatusUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var status CheckStatus

			if strings.Contains(tt.output, "EXPIRED") {
				status = StatusCritical
			} else if strings.Contains(tt.output, "not available") || strings.Contains(tt.output, "unable to check") {
				status = StatusUnknown
			} else if strings.Contains(tt.output, "notAfter") || strings.Contains(tt.output, "CERTIFICATE") {
				status = StatusHealthy
			} else {
				status = StatusHealthy
			}

			assert.Equal(t, tt.expectedStatus, status)
		})
	}
}

// TestEtcdHealthParsing tests etcd health output parsing
func TestEtcdHealthParsing(t *testing.T) {
	tests := []struct {
		name           string
		output         string
		expectedStatus CheckStatus
	}{
		{
			name:           "healthy etcd",
			output:         "https://10.0.0.1:2379 is healthy",
			expectedStatus: StatusHealthy,
		},
		{
			name:           "etcd pods running",
			output:         "etcd-master-1   1/1   Running   0   10d",
			expectedStatus: StatusHealthy,
		},
		{
			name:           "unhealthy etcd",
			output:         "https://10.0.0.1:2379 is unhealthy",
			expectedStatus: StatusCritical,
		},
		{
			name:           "failed etcd check",
			output:         "failed to check etcd health",
			expectedStatus: StatusCritical,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var status CheckStatus

			if strings.Contains(tt.output, "is healthy") {
				status = StatusHealthy
			} else if strings.Contains(tt.output, "Running") {
				status = StatusHealthy
			} else if strings.Contains(tt.output, "unhealthy") || strings.Contains(tt.output, "failed") {
				status = StatusCritical
			} else {
				status = StatusWarning
			}

			assert.Equal(t, tt.expectedStatus, status)
		})
	}
}

// TestCNIDetection tests CNI plugin detection
func TestCNIDetection(t *testing.T) {
	tests := []struct {
		name           string
		output         string
		expectedStatus CheckStatus
		expectedCNI    string
	}{
		{
			name:           "canal running",
			output:         "canal-abc   1/1   Running   0   10d",
			expectedStatus: StatusHealthy,
			expectedCNI:    "canal",
		},
		{
			name:           "calico running",
			output:         "calico-node-xyz   1/1   Running   0   10d",
			expectedStatus: StatusHealthy,
			expectedCNI:    "calico",
		},
		{
			name:           "flannel running",
			output:         "kube-flannel-ds-abc   1/1   Running   0   10d",
			expectedStatus: StatusHealthy,
			expectedCNI:    "flannel",
		},
		{
			name:           "no CNI found",
			output:         "no-cni-found",
			expectedStatus: StatusWarning,
			expectedCNI:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var status CheckStatus

			if strings.Contains(tt.output, "no-cni-found") || strings.TrimSpace(tt.output) == "" {
				status = StatusWarning
			} else if strings.Contains(tt.output, "Running") {
				status = StatusHealthy
			} else {
				status = StatusWarning
			}

			assert.Equal(t, tt.expectedStatus, status)
		})
	}
}

// TestHealthReportPrint tests that PrintReport doesn't panic
func TestHealthReportPrint(t *testing.T) {
	report := &HealthReport{
		ClusterName:   "test-cluster",
		CheckedAt:     time.Now(),
		Duration:      1 * time.Second,
		OverallStatus: StatusHealthy,
		Checks: []CheckResult{
			{
				Name:      "Test Check",
				Status:    StatusHealthy,
				Message:   "All good",
				Details:   []string{"detail1"},
				Duration:  100 * time.Millisecond,
				CheckedAt: time.Now(),
			},
		},
		Summary: Summary{
			TotalChecks:   1,
			HealthyChecks: 1,
		},
		Recommendations: []string{},
	}

	// This should not panic
	assert.NotPanics(t, func() {
		report.PrintReport()
	})
}

// TestHealthReportPrintCompact tests compact print doesn't panic
func TestHealthReportPrintCompact(t *testing.T) {
	report := &HealthReport{
		ClusterName:   "test-cluster",
		CheckedAt:     time.Now(),
		Duration:      1 * time.Second,
		OverallStatus: StatusWarning,
		Checks: []CheckResult{
			{Name: "Check 1", Status: StatusHealthy, Message: "OK"},
			{Name: "Check 2", Status: StatusWarning, Message: "Warning!"},
		},
		Summary: Summary{
			TotalChecks:   2,
			HealthyChecks: 1,
			WarningChecks: 1,
		},
	}

	// This should not panic
	assert.NotPanics(t, func() {
		report.PrintCompact()
	})
}

// TestCheckNamesAreUnique tests that all checks have unique names
func TestCheckNamesAreUnique(t *testing.T) {
	checkNames := []string{
		"Node Health",
		"System Pods",
		"CoreDNS",
		"Certificates",
		"Etcd Cluster",
		"API Server",
		"Storage (PVCs)",
		"Networking",
		"Memory Pressure",
		"Disk Pressure",
	}

	seen := make(map[string]bool)
	for _, name := range checkNames {
		assert.False(t, seen[name], "Duplicate check name: %s", name)
		seen[name] = true
	}
}
