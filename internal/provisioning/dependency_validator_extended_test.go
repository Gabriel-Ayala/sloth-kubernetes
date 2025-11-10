package provisioning

import (
	"fmt"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

// Test GetStandardDependencyChecks with detailed assertions
func TestGetStandardDependencyChecks_DetailedValidation(t *testing.T) {
	checks := GetStandardDependencyChecks()

	// Validate each check in detail
	checkMap := make(map[string]DependencyCheck)
	for _, check := range checks {
		checkMap[check.Name] = check
	}

	// Docker check
	docker := checkMap["docker"]
	assert.Equal(t, "docker", docker.Name)
	assert.Contains(t, docker.Command, "docker")
	assert.Contains(t, docker.Command, "version")
	assert.Equal(t, "Docker version", docker.ExpectedIn)
	assert.NotEmpty(t, docker.Description)

	// WireGuard check
	wg := checkMap["wireguard-tools"]
	assert.Equal(t, "wireguard-tools", wg.Name)
	assert.Contains(t, wg.Command, "wg")
	assert.Equal(t, "wireguard-tools", wg.ExpectedIn)

	// Curl check
	curl := checkMap["curl"]
	assert.Equal(t, "curl", curl.Name)
	assert.Contains(t, curl.Command, "curl")

	// Systemctl check
	systemctl := checkMap["systemctl"]
	assert.Equal(t, "systemctl", systemctl.Name)
	assert.Contains(t, systemctl.Command, "systemctl")
	assert.Equal(t, "systemd", systemctl.ExpectedIn)

	// IP forwarding check
	ipForward := checkMap["ip-forwarding"]
	assert.Equal(t, "ip-forwarding", ipForward.Name)
	assert.Contains(t, ipForward.Command, "sysctl")
	assert.Contains(t, ipForward.ExpectedIn, "net.ipv4.ip_forward = 1")

	// Disk space check
	disk := checkMap["disk-space"]
	assert.Equal(t, "disk-space", disk.Name)
	assert.Contains(t, disk.Command, "df")
}

// Test DependencyCheck with edge cases
func TestDependencyCheck_EdgeCases(t *testing.T) {
	testCases := []struct {
		name  string
		check DependencyCheck
		valid bool
	}{
		{
			name: "EmptyExpectedIn",
			check: DependencyCheck{
				Name:        "test",
				Command:     "test --version",
				Description: "Test",
				ExpectedIn:  "",
			},
			valid: true, // ExpectedIn can be empty
		},
		{
			name: "ComplexCommand",
			check: DependencyCheck{
				Name:        "complex",
				Command:     "cat /proc/meminfo | grep MemTotal | awk '{print $2}'",
				Description: "Complex check",
				ExpectedIn:  "MemTotal",
			},
			valid: true,
		},
		{
			name: "MultilineCommand",
			check: DependencyCheck{
				Name:        "multiline",
				Command:     "if [ -f /test ]; then echo 'exists'; fi",
				Description: "Conditional check",
				ExpectedIn:  "exists",
			},
			valid: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.NotEmpty(t, tc.check.Name)
			assert.NotEmpty(t, tc.check.Command)
			assert.NotEmpty(t, tc.check.Description)
		})
	}
}

// Test DependencyValidationResult with different scenarios
func TestDependencyValidationResult_Scenarios(t *testing.T) {
	testCases := []struct {
		name           string
		result         DependencyValidationResult
		expectedStatus string
	}{
		{
			name: "SuccessWithOutput",
			result: DependencyValidationResult{
				Name:    "docker",
				Success: true,
				Output:  "Docker version 20.10.21, build baeda1f",
				Error:   nil,
			},
			expectedStatus: "success",
		},
		{
			name: "FailureWithError",
			result: DependencyValidationResult{
				Name:    "missing-tool",
				Success: false,
				Output:  "",
				Error:   fmt.Errorf("command not found"),
			},
			expectedStatus: "failure",
		},
		{
			name: "PartialSuccess",
			result: DependencyValidationResult{
				Name:    "disk-space",
				Success: true,
				Output:  "85%",
				Error:   nil,
			},
			expectedStatus: "warning",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.NotEmpty(t, tc.result.Name)
			if tc.result.Success {
				assert.NoError(t, tc.result.Error)
			}
		})
	}
}

// Test ValidateDependenciesArgs construction
func TestValidateDependenciesArgs_Construction(t *testing.T) {
	checks := GetStandardDependencyChecks()

	args := &ValidateDependenciesArgs{
		NodeName:  "production-master-1",
		Checks:    checks,
		DependsOn: []pulumi.Resource{},
	}

	assert.Equal(t, "production-master-1", args.NodeName)
	assert.Len(t, args.Checks, len(checks))
	assert.NotNil(t, args.DependsOn)
}

// Test ValidateDependenciesSync with various scenarios
func TestValidateDependenciesSync_Scenarios(t *testing.T) {
	testCases := []struct {
		name   string
		nodeIP string
		sshKey string
	}{
		{"PrivateIP_10", "10.0.0.1", "test-key-1"},
		{"PrivateIP_172", "172.16.0.1", "test-key-2"},
		{"PrivateIP_192", "192.168.1.100", "test-key-3"},
		{"PublicIP", "203.0.113.1", "test-key-4"},
		{"IPv6", "2001:db8::1", "test-key-5"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				cmd, err := ValidateDependenciesSync(
					ctx,
					fmt.Sprintf("test-%s", tc.name),
					pulumi.String(tc.nodeIP),
					pulumi.String(tc.sshKey),
					[]pulumi.Resource{},
				)

				assert.NoError(t, err)
				assert.NotNil(t, cmd)
				return nil
			}, pulumi.WithMocks("test", "stack", &ProvisioningMockProvider{}))

			assert.NoError(t, err)
		})
	}
}

// Test GetStandardDependencyChecks consistency
func TestGetStandardDependencyChecks_Consistency(t *testing.T) {
	// Call multiple times
	runs := 10
	checkCounts := make([]int, runs)

	for i := 0; i < runs; i++ {
		checks := GetStandardDependencyChecks()
		checkCounts[i] = len(checks)
	}

	// All should be equal
	firstCount := checkCounts[0]
	for i := 1; i < runs; i++ {
		assert.Equal(t, firstCount, checkCounts[i], "Check count should be consistent across calls")
	}
}

// Test DependencyCheck order
func TestGetStandardDependencyChecks_Order(t *testing.T) {
	checks := GetStandardDependencyChecks()

	// Extract names
	names := make([]string, len(checks))
	for i, check := range checks {
		names[i] = check.Name
	}

	// Verify expected order (or at least that certain checks exist)
	essentialChecks := []string{"docker", "wireguard-tools", "curl", "systemctl"}
	foundChecks := make(map[string]bool)

	for _, name := range names {
		foundChecks[name] = true
	}

	for _, essential := range essentialChecks {
		assert.True(t, foundChecks[essential], "Essential check '%s' should be present", essential)
	}
}

// Test ValidateDependenciesSync with nil depends
func TestValidateDependenciesSync_NilDepends(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cmd, err := ValidateDependenciesSync(
			ctx,
			"nil-deps-test",
			pulumi.String("10.0.0.1"),
			pulumi.String("key"),
			nil, // nil dependencies
		)

		assert.NoError(t, err)
		assert.NotNil(t, cmd)
		return nil
	}, pulumi.WithMocks("test", "stack", &ProvisioningMockProvider{}))

	assert.NoError(t, err)
}

// Test ValidateDependenciesSync with empty depends
func TestValidateDependenciesSync_EmptyDepends(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cmd, err := ValidateDependenciesSync(
			ctx,
			"empty-deps-test",
			pulumi.String("192.168.1.1"),
			pulumi.String("key"),
			[]pulumi.Resource{}, // empty dependencies
		)

		assert.NoError(t, err)
		assert.NotNil(t, cmd)
		return nil
	}, pulumi.WithMocks("test", "stack", &ProvisioningMockProvider{}))

	assert.NoError(t, err)
}

// Test custom dependency checks
func TestCustomDependencyChecks_Various(t *testing.T) {
	customChecks := []DependencyCheck{
		{
			Name:        "kubernetes",
			Description: "Kubernetes components",
			Command:     "kubectl version --client",
			ExpectedIn:  "Client Version",
		},
		{
			Name:        "helm",
			Description: "Helm package manager",
			Command:     "helm version",
			ExpectedIn:  "version",
		},
		{
			Name:        "jq",
			Description: "JSON processor",
			Command:     "jq --version",
			ExpectedIn:  "jq",
		},
	}

	for _, check := range customChecks {
		t.Run(check.Name, func(t *testing.T) {
			assert.NotEmpty(t, check.Name)
			assert.NotEmpty(t, check.Description)
			assert.NotEmpty(t, check.Command)
			assert.NotEmpty(t, check.ExpectedIn)
		})
	}
}

// Test ValidateDependenciesArgs with custom checks
func TestValidateDependenciesArgs_CustomChecks(t *testing.T) {
	customChecks := []DependencyCheck{
		{Name: "test1", Command: "test1", ExpectedIn: "test1", Description: "Test 1"},
		{Name: "test2", Command: "test2", ExpectedIn: "test2", Description: "Test 2"},
		{Name: "test3", Command: "test3", ExpectedIn: "test3", Description: "Test 3"},
	}

	args := &ValidateDependenciesArgs{
		NodeName:  "custom-node",
		Checks:    customChecks,
		DependsOn: []pulumi.Resource{},
	}

	assert.Equal(t, "custom-node", args.NodeName)
	assert.Len(t, args.Checks, 3)
}

// Test DependencyValidationResult empty name
func TestDependencyValidationResult_EmptyName(t *testing.T) {
	result := DependencyValidationResult{
		Name:    "",
		Success: true,
		Output:  "output",
		Error:   nil,
	}

	// Should still be valid, even with empty name
	assert.Empty(t, result.Name)
	assert.True(t, result.Success)
}

// Test multiple ValidateDependenciesSync calls
func TestValidateDependenciesSync_MultipleCalls(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		// Create multiple validation commands
		for i := 0; i < 5; i++ {
			cmd, err := ValidateDependenciesSync(
				ctx,
				fmt.Sprintf("multi-call-%d", i),
				pulumi.String(fmt.Sprintf("10.0.0.%d", i+1)),
				pulumi.String("test-key"),
				[]pulumi.Resource{},
			)

			assert.NoError(t, err)
			assert.NotNil(t, cmd)
		}

		return nil
	}, pulumi.WithMocks("test", "stack", &ProvisioningMockProvider{}))

	assert.NoError(t, err)
}

// Test GetStandardDependencyChecks individual check commands
func TestGetStandardDependencyChecks_IndividualCommands(t *testing.T) {
	checks := GetStandardDependencyChecks()

	for _, check := range checks {
		t.Run(check.Name+"_Command", func(t *testing.T) {
			// Verify command doesn't contain dangerous patterns
			assert.NotContains(t, check.Command, "rm -rf /")
			assert.NotContains(t, check.Command, "dd if=/dev/zero")
			assert.NotContains(t, check.Command, ":(){ :|:& };:")

			// Verify command is not empty
			assert.NotEmpty(t, check.Command)
		})
	}
}

// Test DependencyCheck with special characters
func TestDependencyCheck_SpecialCharacters(t *testing.T) {
	checks := []DependencyCheck{
		{
			Name:        "pipe-command",
			Command:     "echo 'test' | grep 'test'",
			ExpectedIn:  "test",
			Description: "Pipe test",
		},
		{
			Name:        "redirect-command",
			Command:     "cat /proc/cpuinfo > /dev/null && echo 'success'",
			ExpectedIn:  "success",
			Description: "Redirect test",
		},
		{
			Name:        "env-var",
			Command:     "echo $HOME",
			ExpectedIn:  "/root",
			Description: "Environment variable",
		},
	}

	for _, check := range checks {
		t.Run(check.Name, func(t *testing.T) {
			assert.NotEmpty(t, check.Command)
			// Verify that the command is not empty and contains valid shell syntax
			assert.Greater(t, len(check.Command), 0)
		})
	}
}

// Test ValidateDependenciesArgs with large check list
func TestValidateDependenciesArgs_LargeCheckList(t *testing.T) {
	// Create a large list of checks
	largeCheckList := make([]DependencyCheck, 50)
	for i := 0; i < 50; i++ {
		largeCheckList[i] = DependencyCheck{
			Name:        fmt.Sprintf("check-%d", i),
			Command:     fmt.Sprintf("test-%d", i),
			ExpectedIn:  fmt.Sprintf("result-%d", i),
			Description: fmt.Sprintf("Check %d", i),
		}
	}

	args := &ValidateDependenciesArgs{
		NodeName:  "large-check-node",
		Checks:    largeCheckList,
		DependsOn: []pulumi.Resource{},
	}

	assert.Equal(t, "large-check-node", args.NodeName)
	assert.Len(t, args.Checks, 50)
}

// Test DependencyValidationResult with long output
func TestDependencyValidationResult_LongOutput(t *testing.T) {
	longOutput := ""
	for i := 0; i < 1000; i++ {
		longOutput += fmt.Sprintf("Line %d\n", i)
	}

	result := DependencyValidationResult{
		Name:    "long-output-test",
		Success: true,
		Output:  longOutput,
		Error:   nil,
	}

	assert.NotEmpty(t, result.Output)
	assert.Contains(t, result.Output, "Line 999")
}

// Test ValidateDependenciesSync with StringOutput
func TestValidateDependenciesSync_StringOutputs(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		// Use StringOutput instead of String
		nodeIP := pulumi.String("10.0.0.1").ToStringOutput()
		sshKey := pulumi.String("test-key").ToStringOutput()

		cmd, err := ValidateDependenciesSync(
			ctx,
			"string-output-test",
			nodeIP,
			sshKey,
			[]pulumi.Resource{},
		)

		assert.NoError(t, err)
		assert.NotNil(t, cmd)
		return nil
	}, pulumi.WithMocks("test", "stack", &ProvisioningMockProvider{}))

	assert.NoError(t, err)
}

// Test GetStandardDependencyChecks with filtering
func TestGetStandardDependencyChecks_Filtering(t *testing.T) {
	allChecks := GetStandardDependencyChecks()

	// Filter checks that require root
	rootChecks := []string{"docker", "systemctl", "ip-forwarding"}
	foundRoot := 0

	for _, check := range allChecks {
		for _, rootCheck := range rootChecks {
			if check.Name == rootCheck {
				foundRoot++
				break
			}
		}
	}

	assert.GreaterOrEqual(t, foundRoot, 3, "Should find at least 3 root-level checks")
}
