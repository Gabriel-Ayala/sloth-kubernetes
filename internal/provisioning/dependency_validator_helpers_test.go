package provisioning

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetStandardDependencyChecks_Detailed tests that standard dependency checks are properly configured
func TestGetStandardDependencyChecks_Detailed(t *testing.T) {
	checks := GetStandardDependencyChecks()

	// Verify we have all expected checks
	require.NotNil(t, checks)
	assert.Len(t, checks, 6, "Should have 6 standard dependency checks")

	// Create a map for easier lookup
	checkMap := make(map[string]DependencyCheck)
	for _, check := range checks {
		checkMap[check.Name] = check
	}

	// Verify Docker check
	dockerCheck, exists := checkMap["docker"]
	assert.True(t, exists, "Docker check should exist")
	assert.Equal(t, "Docker engine installed and running", dockerCheck.Description)
	assert.Equal(t, "docker --version", dockerCheck.Command)
	assert.Equal(t, "Docker version", dockerCheck.ExpectedIn)

	// Verify WireGuard tools check
	wgCheck, exists := checkMap["wireguard-tools"]
	assert.True(t, exists, "WireGuard tools check should exist")
	assert.Equal(t, "WireGuard tools installed", wgCheck.Description)
	assert.Equal(t, "wg --version", wgCheck.Command)
	assert.Equal(t, "wireguard-tools", wgCheck.ExpectedIn)

	// Verify curl check
	curlCheck, exists := checkMap["curl"]
	assert.True(t, exists, "curl check should exist")
	assert.Equal(t, "curl utility available", curlCheck.Description)
	assert.Equal(t, "curl --version", curlCheck.Command)
	assert.Equal(t, "curl", curlCheck.ExpectedIn)

	// Verify systemctl check
	systemctlCheck, exists := checkMap["systemctl"]
	assert.True(t, exists, "systemctl check should exist")
	assert.Equal(t, "systemd is available", systemctlCheck.Description)
	assert.Equal(t, "systemctl --version", systemctlCheck.Command)
	assert.Equal(t, "systemd", systemctlCheck.ExpectedIn)

	// Verify IP forwarding check
	ipForwardingCheck, exists := checkMap["ip-forwarding"]
	assert.True(t, exists, "IP forwarding check should exist")
	assert.Equal(t, "IP forwarding enabled", ipForwardingCheck.Description)
	assert.Equal(t, "sysctl net.ipv4.ip_forward", ipForwardingCheck.Command)
	assert.Equal(t, "net.ipv4.ip_forward = 1", ipForwardingCheck.ExpectedIn)

	// Verify disk space check
	diskCheck, exists := checkMap["disk-space"]
	assert.True(t, exists, "Disk space check should exist")
	assert.Equal(t, "Sufficient disk space available", diskCheck.Description)
	assert.Contains(t, diskCheck.Command, "df -h")
	assert.Empty(t, diskCheck.ExpectedIn, "Disk check has custom logic, no expected string")
}

// TestGetStandardDependencyChecks_AllFieldsSet verifies all checks have required fields
func TestGetStandardDependencyChecks_AllFieldsSet(t *testing.T) {
	checks := GetStandardDependencyChecks()

	for _, check := range checks {
		assert.NotEmpty(t, check.Name, "Check name should not be empty")
		assert.NotEmpty(t, check.Description, "Check description should not be empty")
		assert.NotEmpty(t, check.Command, "Check command should not be empty")
		// ExpectedIn can be empty for checks with custom validation logic
	}
}

// TestGetStandardDependencyChecks_UniqueNames verifies all check names are unique
func TestGetStandardDependencyChecks_UniqueNames(t *testing.T) {
	checks := GetStandardDependencyChecks()

	nameSet := make(map[string]bool)
	for _, check := range checks {
		assert.False(t, nameSet[check.Name], "Check name '%s' should be unique", check.Name)
		nameSet[check.Name] = true
	}
}

// TestGetStandardDependencyChecks_CommandsAreValid tests that commands are well-formed
func TestGetStandardDependencyChecks_CommandsAreValid(t *testing.T) {
	checks := GetStandardDependencyChecks()

	for _, check := range checks {
		// Verify commands don't contain dangerous patterns
		assert.NotContains(t, check.Command, "rm -rf", "Command should not contain destructive operations")
		assert.NotContains(t, check.Command, "dd if=", "Command should not contain destructive operations")

		// Verify commands are not empty
		assert.NotEmpty(t, check.Command, "Command for %s should not be empty", check.Name)

		// Verify commands don't have trailing semicolons or ampersands (unless intentional)
		assert.NotContains(t, check.Command, "&&", "Command should be simple, not chained")
	}
}

// TestDependencyCheck_Struct tests the DependencyCheck structure
func TestDependencyCheck_Struct(t *testing.T) {
	check := DependencyCheck{
		Name:        "test-check",
		Description: "Test dependency check",
		Command:     "echo 'test'",
		ExpectedIn:  "test",
	}

	assert.Equal(t, "test-check", check.Name)
	assert.Equal(t, "Test dependency check", check.Description)
	assert.Equal(t, "echo 'test'", check.Command)
	assert.Equal(t, "test", check.ExpectedIn)
}

// TestDependencyCheck_EmptyStruct tests zero values
func TestDependencyCheck_EmptyStruct(t *testing.T) {
	var check DependencyCheck

	assert.Empty(t, check.Name)
	assert.Empty(t, check.Description)
	assert.Empty(t, check.Command)
	assert.Empty(t, check.ExpectedIn)
}

// TestDependencyValidationResult_Struct tests the result structure
func TestDependencyValidationResult_Struct(t *testing.T) {
	result := DependencyValidationResult{
		Name:    "test-result",
		Success: true,
		Output:  "test output",
		Error:   nil,
	}

	assert.Equal(t, "test-result", result.Name)
	assert.True(t, result.Success)
	assert.Equal(t, "test output", result.Output)
	assert.Nil(t, result.Error)
}

// TestDependencyValidationResult_WithErrorDetailed tests result with error
func TestDependencyValidationResult_WithErrorDetailed(t *testing.T) {
	result := DependencyValidationResult{
		Name:    "failed-check",
		Success: false,
		Output:  "",
		Error:   assert.AnError,
	}

	assert.Equal(t, "failed-check", result.Name)
	assert.False(t, result.Success)
	assert.Empty(t, result.Output)
	assert.NotNil(t, result.Error)
}

// TestDependencyValidationResult_EmptyStruct tests zero values
func TestDependencyValidationResult_EmptyStruct(t *testing.T) {
	var result DependencyValidationResult

	assert.Empty(t, result.Name)
	assert.False(t, result.Success) // bool defaults to false
	assert.Empty(t, result.Output)
	assert.Nil(t, result.Error)
}

// TestGetStandardDependencyChecks_OrderConsistency verifies check order is consistent
func TestGetStandardDependencyChecks_OrderConsistency(t *testing.T) {
	// Call multiple times and verify order is same
	checks1 := GetStandardDependencyChecks()
	checks2 := GetStandardDependencyChecks()

	require.Len(t, checks1, len(checks2))

	for i := range checks1 {
		assert.Equal(t, checks1[i].Name, checks2[i].Name, "Check order should be consistent at index %d", i)
	}
}

// TestGetStandardDependencyChecks_DockerCheck tests Docker check specifics
func TestGetStandardDependencyChecks_DockerCheck(t *testing.T) {
	checks := GetStandardDependencyChecks()

	var dockerCheck *DependencyCheck
	for i, check := range checks {
		if check.Name == "docker" {
			dockerCheck = &checks[i]
			break
		}
	}

	require.NotNil(t, dockerCheck, "Docker check should exist")
	assert.Equal(t, "docker --version", dockerCheck.Command)
	assert.Equal(t, "Docker version", dockerCheck.ExpectedIn)
	assert.Contains(t, dockerCheck.Description, "Docker")
}

// TestGetStandardDependencyChecks_WireGuardCheck tests WireGuard check specifics
func TestGetStandardDependencyChecks_WireGuardCheck(t *testing.T) {
	checks := GetStandardDependencyChecks()

	var wgCheck *DependencyCheck
	for i, check := range checks {
		if check.Name == "wireguard-tools" {
			wgCheck = &checks[i]
			break
		}
	}

	require.NotNil(t, wgCheck, "WireGuard check should exist")
	assert.Equal(t, "wg --version", wgCheck.Command)
	assert.Equal(t, "wireguard-tools", wgCheck.ExpectedIn)
	assert.Contains(t, wgCheck.Description, "WireGuard")
}

// TestGetStandardDependencyChecks_SystemdCheck tests systemd check specifics
func TestGetStandardDependencyChecks_SystemdCheck(t *testing.T) {
	checks := GetStandardDependencyChecks()

	var systemdCheck *DependencyCheck
	for i, check := range checks {
		if check.Name == "systemctl" {
			systemdCheck = &checks[i]
			break
		}
	}

	require.NotNil(t, systemdCheck, "Systemd check should exist")
	assert.Equal(t, "systemctl --version", systemdCheck.Command)
	assert.Equal(t, "systemd", systemdCheck.ExpectedIn)
	assert.Contains(t, systemdCheck.Description, "systemd")
}

// TestGetStandardDependencyChecks_IPForwardingCheck tests IP forwarding check specifics
func TestGetStandardDependencyChecks_IPForwardingCheck(t *testing.T) {
	checks := GetStandardDependencyChecks()

	var ipCheck *DependencyCheck
	for i, check := range checks {
		if check.Name == "ip-forwarding" {
			ipCheck = &checks[i]
			break
		}
	}

	require.NotNil(t, ipCheck, "IP forwarding check should exist")
	assert.Contains(t, ipCheck.Command, "sysctl")
	assert.Contains(t, ipCheck.Command, "net.ipv4.ip_forward")
	assert.Equal(t, "net.ipv4.ip_forward = 1", ipCheck.ExpectedIn)
}

// TestGetStandardDependencyChecks_DiskSpaceCheck tests disk space check specifics
func TestGetStandardDependencyChecks_DiskSpaceCheck(t *testing.T) {
	checks := GetStandardDependencyChecks()

	var diskCheck *DependencyCheck
	for i, check := range checks {
		if check.Name == "disk-space" {
			diskCheck = &checks[i]
			break
		}
	}

	require.NotNil(t, diskCheck, "Disk space check should exist")
	assert.Contains(t, diskCheck.Command, "df")
	assert.Contains(t, diskCheck.Command, "/")
	assert.Empty(t, diskCheck.ExpectedIn, "Disk check uses custom validation")
}

// TestGetStandardDependencyChecks_CurlCheck tests curl check specifics
func TestGetStandardDependencyChecks_CurlCheck(t *testing.T) {
	checks := GetStandardDependencyChecks()

	var curlCheck *DependencyCheck
	for i, check := range checks {
		if check.Name == "curl" {
			curlCheck = &checks[i]
			break
		}
	}

	require.NotNil(t, curlCheck, "curl check should exist")
	assert.Equal(t, "curl --version", curlCheck.Command)
	assert.Equal(t, "curl", curlCheck.ExpectedIn)
}

// TestGetStandardDependencyChecks_NoNilChecks verifies no nil checks in the list
func TestGetStandardDependencyChecks_NoNilChecks(t *testing.T) {
	checks := GetStandardDependencyChecks()

	for i, check := range checks {
		assert.NotEqual(t, DependencyCheck{}, check, "Check at index %d should not be zero value", i)
	}
}

// TestGetStandardDependencyChecks_DescriptionsAreDescriptive tests description quality
func TestGetStandardDependencyChecks_DescriptionsAreDescriptive(t *testing.T) {
	checks := GetStandardDependencyChecks()

	for _, check := range checks {
		// Descriptions should be at least 10 characters
		assert.GreaterOrEqual(t, len(check.Description), 10, "Description for %s should be descriptive", check.Name)

		// Descriptions should not just be the name
		assert.NotEqual(t, check.Name, check.Description, "Description should not be same as name for %s", check.Name)
	}
}

// TestGetStandardDependencyChecks_CommandsHaveNoInjection verifies commands are safe
func TestGetStandardDependencyChecks_CommandsHaveNoInjection(t *testing.T) {
	checks := GetStandardDependencyChecks()

	dangerousPatterns := []string{
		"; rm",
		"| rm",
		"&& rm",
		"$(rm",
		"`rm",
		"; wget",
		"; curl http",
		">/dev/null; ",
	}

	for _, check := range checks {
		for _, pattern := range dangerousPatterns {
			assert.NotContains(t, check.Command, pattern, "Command for %s should not contain dangerous pattern '%s'", check.Name, pattern)
		}
	}
}
