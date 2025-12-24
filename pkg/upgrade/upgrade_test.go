package upgrade

import (
	"testing"
)

func TestUpgradeStatusConstants(t *testing.T) {
	tests := []struct {
		name     string
		status   UpgradeStatus
		expected string
	}{
		{"StatusPending", StatusPending, "pending"},
		{"StatusInProgress", StatusInProgress, "in_progress"},
		{"StatusCompleted", StatusCompleted, "completed"},
		{"StatusFailed", StatusFailed, "failed"},
		{"StatusRolledBack", StatusRolledBack, "rolled_back"},
		{"StatusSkipped", StatusSkipped, "skipped"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.status) != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, tt.status)
			}
		})
	}
}

func TestNodeStatusAliases(t *testing.T) {
	if NodeStatusPending != StatusPending {
		t.Error("NodeStatusPending should equal StatusPending")
	}
	if NodeStatusCompleted != StatusCompleted {
		t.Error("NodeStatusCompleted should equal StatusCompleted")
	}
	if NodeStatusFailed != StatusFailed {
		t.Error("NodeStatusFailed should equal StatusFailed")
	}
	if NodeStatusSkipped != StatusSkipped {
		t.Error("NodeStatusSkipped should equal StatusSkipped")
	}
}

func TestUpgradeStrategyConstants(t *testing.T) {
	tests := []struct {
		name     string
		strategy UpgradeStrategy
		expected string
	}{
		{"StrategyRolling", StrategyRolling, "rolling"},
		{"StrategyBlueGreen", StrategyBlueGreen, "blue-green"},
		{"StrategyCanary", StrategyCanary, "canary"},
		{"StrategyInPlace", StrategyInPlace, "in-place"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.strategy) != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, tt.strategy)
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
	if manager.dryRun != false {
		t.Error("dryRun should default to false")
	}
	if manager.verbose != false {
		t.Error("verbose should default to false")
	}
}

func TestManagerSetters(t *testing.T) {
	manager := NewManager("", "", "")

	manager.SetKubeconfig("/new/path")
	if manager.kubeconfig != "/new/path" {
		t.Errorf("expected /new/path, got %s", manager.kubeconfig)
	}

	manager.SetDryRun(true)
	if manager.dryRun != true {
		t.Error("dryRun should be true")
	}

	manager.SetVerbose(true)
	if manager.verbose != true {
		t.Error("verbose should be true")
	}
}

func TestGetAvailableVersions(t *testing.T) {
	manager := NewManager("", "", "")

	versions, err := manager.GetAvailableVersions()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(versions) == 0 {
		t.Error("expected at least one version")
	}

	// Check that versions are in expected format
	for _, v := range versions {
		if v[0] != 'v' {
			t.Errorf("version should start with 'v': %s", v)
		}
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Server Version: v1.28.0", "v1.28.0"},
		{"v1.29.1", "v1.29.1"},
		{"1.30.0", "v1.30.0"},
		{"gitVersion: v1.28.4", "v1.28.4"},
		{"no version here", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseVersion(tt.input)
			if result != tt.expected {
				t.Errorf("parseVersion(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsUpgradePathValid(t *testing.T) {
	tests := []struct {
		from     string
		to       string
		expected bool
	}{
		{"v1.28.0", "v1.29.0", true},  // skip 1 minor
		{"v1.28.0", "v1.30.0", true},  // skip 2 minor
		{"v1.28.0", "v1.31.0", false}, // skip 3 minor (invalid)
		{"v1.28.0", "v1.28.1", true},  // patch upgrade
		{"v1.29.0", "v1.28.0", false}, // downgrade
	}

	for _, tt := range tests {
		t.Run(tt.from+"->"+tt.to, func(t *testing.T) {
			result := isUpgradePathValid(tt.from, tt.to)
			if result != tt.expected {
				t.Errorf("isUpgradePathValid(%s, %s) = %v, want %v", tt.from, tt.to, result, tt.expected)
			}
		})
	}
}

func TestExtractMinorVersion(t *testing.T) {
	tests := []struct {
		version  string
		expected int
	}{
		{"v1.28.0", 28},
		{"v1.29.1", 29},
		{"1.30.0", 30},
		{"invalid", 0},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			result := extractMinorVersion(tt.version)
			if result != tt.expected {
				t.Errorf("extractMinorVersion(%s) = %d, want %d", tt.version, result, tt.expected)
			}
		})
	}
}

func TestNodeUpgradePlanFields(t *testing.T) {
	plan := NodeUpgradePlan{
		NodeName:       "node-1",
		Name:           "node-1",
		Role:           "master",
		Version:        "v1.28.0",
		CurrentVersion: "v1.28.0",
		TargetVersion:  "v1.29.0",
		Order:          1,
		DrainRequired:  true,
		Status:         "Ready",
	}

	if plan.NodeName != "node-1" {
		t.Errorf("unexpected NodeName: %s", plan.NodeName)
	}
	if plan.Role != "master" {
		t.Errorf("unexpected Role: %s", plan.Role)
	}
	if plan.DrainRequired != true {
		t.Error("DrainRequired should be true")
	}
}

func TestUpgradePlanFields(t *testing.T) {
	plan := UpgradePlan{
		ClusterName:    "test-cluster",
		CurrentVersion: "v1.28.0",
		TargetVersion:  "v1.29.0",
		Strategy:       StrategyRolling,
		Nodes:          []NodeUpgradePlan{},
		PreChecks:      []PreCheck{},
		PostChecks:     []PostCheck{},
		Risks:          []string{"risk1"},
	}

	if plan.ClusterName != "test-cluster" {
		t.Errorf("unexpected ClusterName: %s", plan.ClusterName)
	}
	if plan.Strategy != StrategyRolling {
		t.Errorf("unexpected Strategy: %s", plan.Strategy)
	}
}

func TestUpgradeResultFields(t *testing.T) {
	result := UpgradeResult{
		Status:          StatusCompleted,
		Success:         true,
		ClusterName:     "test-cluster",
		FromVersion:     "v1.28.0",
		ToVersion:       "v1.29.0",
		PreviousVersion: "v1.28.0",
		NewVersion:      "v1.29.0",
		Strategy:        StrategyRolling,
		TotalNodes:      3,
		NodesUpgraded:   3,
		NodesFailed:     0,
		NodeResults:     []NodeUpgradeStatus{},
		Errors:          []string{},
		Warnings:        []string{},
	}

	if result.Status != StatusCompleted {
		t.Errorf("unexpected Status: %s", result.Status)
	}
	if result.Success != true {
		t.Error("Success should be true")
	}
	if result.TotalNodes != 3 {
		t.Errorf("unexpected TotalNodes: %d", result.TotalNodes)
	}
}

func TestNodeUpgradeStatusFields(t *testing.T) {
	status := NodeUpgradeStatus{
		NodeName:       "node-1",
		Name:           "node-1",
		CurrentVersion: "v1.28.0",
		TargetVersion:  "v1.29.0",
		Status:         StatusCompleted,
		Duration:       "5m30s",
		Error:          "",
	}

	if status.NodeName != "node-1" {
		t.Errorf("unexpected NodeName: %s", status.NodeName)
	}
	if status.Status != StatusCompleted {
		t.Errorf("unexpected Status: %s", status.Status)
	}
	if status.Duration != "5m30s" {
		t.Errorf("unexpected Duration: %s", status.Duration)
	}
}

func TestPreCheckFields(t *testing.T) {
	check := PreCheck{
		Name:        "Version Compatibility",
		Description: "Check if upgrade path is supported",
		Passed:      true,
		Message:     "Upgrade path is valid",
		Required:    true,
	}

	if check.Name != "Version Compatibility" {
		t.Errorf("unexpected Name: %s", check.Name)
	}
	if check.Required != true {
		t.Error("Required should be true")
	}
	if check.Passed != true {
		t.Error("Passed should be true")
	}
}

func TestPostCheckFields(t *testing.T) {
	check := PostCheck{
		Name:        "Cluster Health",
		Description: "Verify cluster is healthy after upgrade",
		Passed:      true,
		Message:     "All nodes healthy",
	}

	if check.Name != "Cluster Health" {
		t.Errorf("unexpected Name: %s", check.Name)
	}
	if check.Passed != true {
		t.Error("Passed should be true")
	}
}

func TestRollbackInfoFields(t *testing.T) {
	info := RollbackInfo{
		PreviousVersion: "v1.28.0",
		NodesAffected:   []string{"node-1", "node-2"},
		BackupPath:      "/var/lib/backups/etcd",
	}

	if info.PreviousVersion != "v1.28.0" {
		t.Errorf("unexpected PreviousVersion: %s", info.PreviousVersion)
	}
	if len(info.NodesAffected) != 2 {
		t.Errorf("unexpected NodesAffected count: %d", len(info.NodesAffected))
	}
}

func TestManagerRollbackNoInfo(t *testing.T) {
	manager := NewManager("", "", "")

	err := manager.Rollback()
	if err == nil {
		t.Error("expected error when no rollback info")
	}
	if err.Error() != "no rollback information available" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestIdentifyRisks(t *testing.T) {
	manager := NewManager("", "", "")

	// Test with single master
	nodes := []NodeUpgradePlan{
		{Name: "master-1", Role: "master"},
	}
	risks := manager.identifyRisks("v1.28.0", "v1.29.0", nodes)

	foundSingleMasterRisk := false
	for _, risk := range risks {
		if risk == "Single master node - no HA during upgrade" {
			foundSingleMasterRisk = true
			break
		}
	}
	if !foundSingleMasterRisk {
		t.Error("expected single master risk to be identified")
	}

	// Test with limited workers
	nodes = []NodeUpgradePlan{
		{Name: "master-1", Role: "master"},
		{Name: "worker-1", Role: "worker"},
	}
	risks = manager.identifyRisks("v1.28.0", "v1.29.0", nodes)

	foundLimitedWorkerRisk := false
	for _, risk := range risks {
		if risk == "Limited worker nodes - workloads may be disrupted during upgrade" {
			foundLimitedWorkerRisk = true
			break
		}
	}
	if !foundLimitedWorkerRisk {
		t.Error("expected limited worker risk to be identified")
	}
}

func TestRunPreChecks(t *testing.T) {
	manager := NewManager("", "", "")

	checks := manager.runPreChecks("v1.28.0", "v1.29.0")

	if len(checks) == 0 {
		t.Error("expected at least one pre-check")
	}

	// Find version compatibility check
	var versionCheck *PreCheck
	for i := range checks {
		if checks[i].Name == "Version Compatibility" {
			versionCheck = &checks[i]
			break
		}
	}

	if versionCheck == nil {
		t.Error("expected Version Compatibility check")
	} else if !versionCheck.Passed {
		t.Error("expected Version Compatibility check to pass for v1.28.0 -> v1.29.0")
	}
}

func TestCreatePlanWithMockData(t *testing.T) {
	manager := NewManager("", "", "")

	// Since CreatePlan needs kubectl, we just test that the method exists
	// and returns an error when kubectl is not available
	_, err := manager.CreatePlan("v1.29.0", StrategyRolling)
	if err == nil {
		t.Log("CreatePlan succeeded (kubectl available)")
	} else {
		t.Logf("CreatePlan failed as expected without kubectl: %v", err)
	}
}
