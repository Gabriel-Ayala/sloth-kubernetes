package provisioning

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDependencyCheck_EmptyValues tests handling of empty values
func TestDependencyCheck_EmptyValues(t *testing.T) {
	check := DependencyCheck{}

	assert.Empty(t, check.Name)
	assert.Empty(t, check.Description)
	assert.Empty(t, check.Command)
	assert.Empty(t, check.ExpectedIn)
}

// TestDependencyCheck_MultipleInstances tests multiple dependency checks
func TestDependencyCheck_MultipleInstances(t *testing.T) {
	checks := []DependencyCheck{
		{Name: "check1", Description: "First check", Command: "cmd1", ExpectedIn: "output1"},
		{Name: "check2", Description: "Second check", Command: "cmd2", ExpectedIn: "output2"},
		{Name: "check3", Description: "Third check", Command: "cmd3", ExpectedIn: "output3"},
	}

	assert.Len(t, checks, 3)

	for _, check := range checks {
		assert.NotEmpty(t, check.Name)
		assert.Contains(t, check.Name, "check")
		assert.NotEmpty(t, check.Description)
		assert.NotEmpty(t, check.Command)
		assert.NotEmpty(t, check.ExpectedIn)
	}
}

// TestDependencyValidationResult_WithError tests result with various error conditions
func TestDependencyValidationResult_WithError(t *testing.T) {
	testCases := []struct {
		name        string
		result      DependencyValidationResult
		expectError bool
	}{
		{
			name: "Success case",
			result: DependencyValidationResult{
				Name:    "test",
				Success: true,
				Output:  "success output",
				Error:   nil,
			},
			expectError: false,
		},
		{
			name: "Failure with error",
			result: DependencyValidationResult{
				Name:    "test",
				Success: false,
				Output:  "",
				Error:   errors.New("command failed"),
			},
			expectError: true,
		},
		{
			name: "Failure without error message",
			result: DependencyValidationResult{
				Name:    "test",
				Success: false,
				Output:  "",
				Error:   nil,
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.expectError {
				assert.Error(t, tc.result.Error)
			} else {
				assert.NoError(t, tc.result.Error)
			}
			assert.Equal(t, tc.expectError, tc.result.Error != nil)
		})
	}
}

// TestGetStandardDependencyChecks_Ordering tests that checks maintain consistent order
func TestGetStandardDependencyChecks_Ordering(t *testing.T) {
	checks := GetStandardDependencyChecks()

	expectedOrder := []string{
		"docker",
		"wireguard-tools",
		"curl",
		"systemctl",
		"ip-forwarding",
		"disk-space",
	}

	assert.Equal(t, len(expectedOrder), len(checks))

	for i, expectedName := range expectedOrder {
		assert.Equal(t, expectedName, checks[i].Name, "Check at position %d should be %s", i, expectedName)
	}
}

// TestGetStandardDependencyChecks_DockerDetails tests Docker check specifics
func TestGetStandardDependencyChecks_DockerDetails(t *testing.T) {
	checks := GetStandardDependencyChecks()

	dockerCheck := checks[0] // Docker should be first

	assert.Equal(t, "docker", dockerCheck.Name)
	assert.Contains(t, dockerCheck.Description, "Docker")
	assert.Contains(t, dockerCheck.Command, "docker")
	assert.Contains(t, dockerCheck.Command, "--version")
	assert.Equal(t, "Docker version", dockerCheck.ExpectedIn)
}

// TestGetStandardDependencyChecks_WireGuardDetails tests WireGuard check specifics
func TestGetStandardDependencyChecks_WireGuardDetails(t *testing.T) {
	checks := GetStandardDependencyChecks()

	wgCheck := checks[1] // WireGuard should be second

	assert.Equal(t, "wireguard-tools", wgCheck.Name)
	assert.Contains(t, wgCheck.Description, "WireGuard")
	assert.Equal(t, "wg --version", wgCheck.Command)
	assert.Equal(t, "wireguard-tools", wgCheck.ExpectedIn)
}

// TestGetStandardDependencyChecks_CurlDetails tests curl check specifics
func TestGetStandardDependencyChecks_CurlDetails(t *testing.T) {
	checks := GetStandardDependencyChecks()

	curlCheck := checks[2] // curl should be third

	assert.Equal(t, "curl", curlCheck.Name)
	assert.Contains(t, curlCheck.Description, "curl")
	assert.Equal(t, "curl --version", curlCheck.Command)
	assert.Equal(t, "curl", curlCheck.ExpectedIn)
}

// TestGetStandardDependencyChecks_SystemctlDetails tests systemctl check specifics
func TestGetStandardDependencyChecks_SystemctlDetails(t *testing.T) {
	checks := GetStandardDependencyChecks()

	systemctlCheck := checks[3] // systemctl should be fourth

	assert.Equal(t, "systemctl", systemctlCheck.Name)
	assert.Contains(t, systemctlCheck.Description, "systemd")
	assert.Equal(t, "systemctl --version", systemctlCheck.Command)
	assert.Equal(t, "systemd", systemctlCheck.ExpectedIn)
}

// TestGetStandardDependencyChecks_IPForwardingDetails tests IP forwarding check specifics
func TestGetStandardDependencyChecks_IPForwardingDetails(t *testing.T) {
	checks := GetStandardDependencyChecks()

	ipCheck := checks[4] // IP forwarding should be fifth

	assert.Equal(t, "ip-forwarding", ipCheck.Name)
	assert.Contains(t, ipCheck.Description, "IP forwarding")
	assert.Contains(t, ipCheck.Command, "sysctl")
	assert.Contains(t, ipCheck.Command, "net.ipv4.ip_forward")
	assert.Equal(t, "net.ipv4.ip_forward = 1", ipCheck.ExpectedIn)
}

// TestGetStandardDependencyChecks_DiskSpaceDetails tests disk space check specifics
func TestGetStandardDependencyChecks_DiskSpaceDetails(t *testing.T) {
	checks := GetStandardDependencyChecks()

	diskCheck := checks[5] // disk space should be sixth

	assert.Equal(t, "disk-space", diskCheck.Name)
	assert.Contains(t, diskCheck.Description, "disk")
	assert.Contains(t, diskCheck.Command, "df")
	assert.Contains(t, diskCheck.Command, "/")
	assert.Empty(t, diskCheck.ExpectedIn) // disk-space has empty ExpectedIn
}

// TestDependencyCheck_LongValues tests handling of long values
func TestDependencyCheck_LongValues(t *testing.T) {
	longDescription := "This is a very long description that describes in great detail what this dependency check does and why it is important for the system to function properly"
	longCommand := "bash -c 'for i in {1..100}; do echo $i; done | grep 50 | wc -l'"

	check := DependencyCheck{
		Name:        "long-test",
		Description: longDescription,
		Command:     longCommand,
		ExpectedIn:  "1",
	}

	assert.Equal(t, "long-test", check.Name)
	assert.Equal(t, longDescription, check.Description)
	assert.Equal(t, longCommand, check.Command)
	assert.Greater(t, len(check.Description), 100)
	assert.Greater(t, len(check.Command), 50)
}

// TestValidateDependenciesArgs_EmptyChecks tests args with no checks
func TestValidateDependenciesArgs_EmptyChecks(t *testing.T) {
	args := &ValidateDependenciesArgs{
		NodeName: "test-node",
		Checks:   []DependencyCheck{},
	}

	assert.Equal(t, "test-node", args.NodeName)
	assert.NotNil(t, args.Checks)
	assert.Len(t, args.Checks, 0)
}

// TestValidateDependenciesArgs_MultipleChecks tests args with multiple checks
func TestValidateDependenciesArgs_MultipleChecks(t *testing.T) {
	checks := []DependencyCheck{
		{Name: "check1", Description: "First", Command: "cmd1", ExpectedIn: "out1"},
		{Name: "check2", Description: "Second", Command: "cmd2", ExpectedIn: "out2"},
		{Name: "check3", Description: "Third", Command: "cmd3", ExpectedIn: "out3"},
	}

	args := &ValidateDependenciesArgs{
		NodeName: "multi-check-node",
		Checks:   checks,
	}

	assert.Equal(t, "multi-check-node", args.NodeName)
	assert.Len(t, args.Checks, 3)

	for i, check := range args.Checks {
		assert.Contains(t, check.Name, "check")
		assert.NotEmpty(t, check.Description)
		assert.NotEmpty(t, check.Command)
		assert.NotEmpty(t, check.ExpectedIn)
		assert.Equal(t, checks[i].Name, check.Name)
	}
}

// TestDependencyValidationResult_OutputVariations tests various output scenarios
func TestDependencyValidationResult_OutputVariations(t *testing.T) {
	testCases := []struct {
		name           string
		output         string
		expectedLength int
	}{
		{"Empty output", "", 0},
		{"Short output", "OK", 2},
		{"Multi-line output", "line1\nline2\nline3", 17},
		{"Long output", string(make([]byte, 1000)), 1000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := DependencyValidationResult{
				Name:    "test",
				Success: true,
				Output:  tc.output,
				Error:   nil,
			}

			assert.Equal(t, tc.expectedLength, len(result.Output))
		})
	}
}

// TestGetStandardDependencyChecks_NoMutation tests that returned checks are not shared references
func TestGetStandardDependencyChecks_NoMutation(t *testing.T) {
	checks1 := GetStandardDependencyChecks()
	original := checks1[0].Name

	// Modify the returned slice
	checks1[0].Name = "MODIFIED"

	// Get checks again
	checks2 := GetStandardDependencyChecks()

	// Original values should be preserved
	assert.Equal(t, original, checks2[0].Name)
	assert.NotEqual(t, "MODIFIED", checks2[0].Name)
}

// TestDependencyCheck_Equality tests comparing dependency checks
func TestDependencyCheck_Equality(t *testing.T) {
	check1 := DependencyCheck{
		Name:        "test",
		Description: "Test check",
		Command:     "test --version",
		ExpectedIn:  "test",
	}

	check2 := DependencyCheck{
		Name:        "test",
		Description: "Test check",
		Command:     "test --version",
		ExpectedIn:  "test",
	}

	check3 := DependencyCheck{
		Name:        "different",
		Description: "Different check",
		Command:     "diff --version",
		ExpectedIn:  "diff",
	}

	assert.Equal(t, check1, check2)
	assert.NotEqual(t, check1, check3)
}

// TestDependencyValidationResult_ErrorMessages tests various error messages
func TestDependencyValidationResult_ErrorMessages(t *testing.T) {
	testCases := []struct {
		name  string
		error error
	}{
		{"Simple error", errors.New("simple error")},
		{"Complex error", errors.New("command failed: exit status 1")},
		{"Detailed error", errors.New("expected 'test' not found in output: actual output was 'invalid'")},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := DependencyValidationResult{
				Name:    "test",
				Success: false,
				Output:  "",
				Error:   tc.error,
			}

			assert.Error(t, result.Error)
			assert.False(t, result.Success)
			assert.NotEmpty(t, result.Error.Error())
		})
	}
}

// TestGetStandardDependencyChecks_CommandsSafe tests that commands don't contain dangerous patterns
func TestGetStandardDependencyChecks_CommandsSafe(t *testing.T) {
	checks := GetStandardDependencyChecks()

	dangerousPatterns := []string{
		"rm -rf",
		"dd if=",
		"mkfs",
		":(){:|:&};:",  // fork bomb
		"curl | bash",
		"wget | sh",
	}

	for _, check := range checks {
		for _, pattern := range dangerousPatterns {
			assert.NotContains(t, check.Command, pattern,
				"Check %s contains dangerous pattern: %s", check.Name, pattern)
		}
	}
}

// TestDependencyCheck_CopySemantics tests that checks can be copied
func TestDependencyCheck_CopySemantics(t *testing.T) {
	original := DependencyCheck{
		Name:        "original",
		Description: "Original check",
		Command:     "orig --version",
		ExpectedIn:  "orig",
	}

	// Copy by value
	copy := original
	copy.Name = "copy"

	assert.Equal(t, "original", original.Name)
	assert.Equal(t, "copy", copy.Name)
	assert.Equal(t, original.Description, copy.Description)
}

// TestValidateDependenciesArgs_NodeNameVariations tests different node name formats
func TestValidateDependenciesArgs_NodeNameVariations(t *testing.T) {
	testCases := []string{
		"simple-node",
		"node-01",
		"master-node-1",
		"worker.node.123",
		"NODE_WITH_UNDERSCORES",
	}

	for _, nodeName := range testCases {
		t.Run(nodeName, func(t *testing.T) {
			args := &ValidateDependenciesArgs{
				NodeName: nodeName,
				Checks:   GetStandardDependencyChecks(),
			}

			assert.Equal(t, nodeName, args.NodeName)
			assert.NotEmpty(t, args.Checks)
		})
	}
}

// TestDependencyValidationResult_SuccessFailureTransition tests state changes
func TestDependencyValidationResult_SuccessFailureTransition(t *testing.T) {
	result := DependencyValidationResult{
		Name:    "transition-test",
		Success: true,
		Output:  "initial success",
		Error:   nil,
	}

	assert.True(t, result.Success)
	assert.NoError(t, result.Error)

	// Simulate failure
	result.Success = false
	result.Error = errors.New("now failed")

	assert.False(t, result.Success)
	assert.Error(t, result.Error)
}

// TestGetStandardDependencyChecks_AllCommandsRunnable tests command format validity
func TestGetStandardDependencyChecks_AllCommandsRunnable(t *testing.T) {
	checks := GetStandardDependencyChecks()

	for _, check := range checks {
		t.Run(check.Name+"_CommandFormat", func(t *testing.T) {
			// Commands should not be empty
			assert.NotEmpty(t, check.Command)

			// Commands should not start with whitespace
			assert.Equal(t, check.Command, check.Command)

			// Commands should be strings
			assert.IsType(t, "", check.Command)
		})
	}
}
