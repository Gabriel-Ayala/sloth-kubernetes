package provisioning

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDependencyCheck_Structure(t *testing.T) {
	check := DependencyCheck{
		Name:        "test-dep",
		Description: "Test dependency",
		Command:     "test --version",
		ExpectedIn:  "version",
	}

	assert.Equal(t, "test-dep", check.Name)
	assert.Equal(t, "Test dependency", check.Description)
	assert.Equal(t, "test --version", check.Command)
	assert.Equal(t, "version", check.ExpectedIn)
}

func TestDependencyValidationResult_Structure(t *testing.T) {
	result := DependencyValidationResult{
		Name:    "test",
		Success: true,
		Output:  "test output",
		Error:   nil,
	}

	assert.Equal(t, "test", result.Name)
	assert.True(t, result.Success)
	assert.Equal(t, "test output", result.Output)
	assert.Nil(t, result.Error)
}

func TestGetStandardDependencyChecks(t *testing.T) {
	checks := GetStandardDependencyChecks()

	assert.NotNil(t, checks)
	assert.Greater(t, len(checks), 0, "Should return at least one dependency check")

	// Verify all required dependencies are present
	checkNames := make(map[string]bool)
	for _, check := range checks {
		checkNames[check.Name] = true

		// Verify structure is complete
		assert.NotEmpty(t, check.Name, "Check name should not be empty")
		assert.NotEmpty(t, check.Description, "Check description should not be empty")
		assert.NotEmpty(t, check.Command, "Check command should not be empty")
		// ExpectedIn can be empty for some checks (like disk-space)
	}

	// Verify essential dependencies are included
	essentialDeps := []string{"docker", "wireguard-tools", "curl", "systemctl", "ip-forwarding"}
	for _, dep := range essentialDeps {
		assert.True(t, checkNames[dep], "Expected dependency '%s' to be in standard checks", dep)
	}
}

func TestGetStandardDependencyChecks_Docker(t *testing.T) {
	checks := GetStandardDependencyChecks()

	var dockerCheck *DependencyCheck
	for _, check := range checks {
		if check.Name == "docker" {
			dockerCheck = &check
			break
		}
	}

	require := assert.New(t)
	require.NotNil(dockerCheck, "Docker check should exist")
	require.Equal("docker", dockerCheck.Name)
	require.Contains(dockerCheck.Command, "docker")
	require.Contains(dockerCheck.Command, "--version")
	require.Equal("Docker version", dockerCheck.ExpectedIn)
}

func TestGetStandardDependencyChecks_WireGuard(t *testing.T) {
	checks := GetStandardDependencyChecks()

	var wgCheck *DependencyCheck
	for _, check := range checks {
		if check.Name == "wireguard-tools" {
			wgCheck = &check
			break
		}
	}

	require := assert.New(t)
	require.NotNil(wgCheck, "WireGuard check should exist")
	require.Equal("wireguard-tools", wgCheck.Name)
	require.Contains(wgCheck.Command, "wg")
	require.Contains(wgCheck.Command, "--version")
	require.Equal("wireguard-tools", wgCheck.ExpectedIn)
}

func TestGetStandardDependencyChecks_Curl(t *testing.T) {
	checks := GetStandardDependencyChecks()

	var curlCheck *DependencyCheck
	for _, check := range checks {
		if check.Name == "curl" {
			curlCheck = &check
			break
		}
	}

	require := assert.New(t)
	require.NotNil(curlCheck, "Curl check should exist")
	require.Equal("curl", curlCheck.Name)
	require.Contains(curlCheck.Command, "curl")
	require.Equal("curl", curlCheck.ExpectedIn)
}

func TestGetStandardDependencyChecks_Systemctl(t *testing.T) {
	checks := GetStandardDependencyChecks()

	var systemctlCheck *DependencyCheck
	for _, check := range checks {
		if check.Name == "systemctl" {
			systemctlCheck = &check
			break
		}
	}

	require := assert.New(t)
	require.NotNil(systemctlCheck, "Systemctl check should exist")
	require.Equal("systemctl", systemctlCheck.Name)
	require.Contains(systemctlCheck.Command, "systemctl")
	require.Equal("systemd", systemctlCheck.ExpectedIn)
}

func TestGetStandardDependencyChecks_IPForwarding(t *testing.T) {
	checks := GetStandardDependencyChecks()

	var ipCheck *DependencyCheck
	for _, check := range checks {
		if check.Name == "ip-forwarding" {
			ipCheck = &check
			break
		}
	}

	require := assert.New(t)
	require.NotNil(ipCheck, "IP forwarding check should exist")
	require.Equal("ip-forwarding", ipCheck.Name)
	require.Contains(ipCheck.Command, "sysctl")
	require.Contains(ipCheck.Command, "net.ipv4.ip_forward")
	require.Equal("net.ipv4.ip_forward = 1", ipCheck.ExpectedIn)
}

func TestGetStandardDependencyChecks_DiskSpace(t *testing.T) {
	checks := GetStandardDependencyChecks()

	var diskCheck *DependencyCheck
	for _, check := range checks {
		if check.Name == "disk-space" {
			diskCheck = &check
			break
		}
	}

	require := assert.New(t)
	require.NotNil(diskCheck, "Disk space check should exist")
	require.Equal("disk-space", diskCheck.Name)
	require.Contains(diskCheck.Command, "df")
}

func TestGetStandardDependencyChecks_Immutable(t *testing.T) {
	// Call the function multiple times and verify it returns consistent results
	checks1 := GetStandardDependencyChecks()
	checks2 := GetStandardDependencyChecks()

	assert.Equal(t, len(checks1), len(checks2), "Function should return same number of checks")

	for i := range checks1 {
		assert.Equal(t, checks1[i].Name, checks2[i].Name)
		assert.Equal(t, checks1[i].Description, checks2[i].Description)
		assert.Equal(t, checks1[i].Command, checks2[i].Command)
		assert.Equal(t, checks1[i].ExpectedIn, checks2[i].ExpectedIn)
	}
}

func TestGetStandardDependencyChecks_AllFieldsPopulated(t *testing.T) {
	checks := GetStandardDependencyChecks()

	for _, check := range checks {
		t.Run(check.Name, func(t *testing.T) {
			assert.NotEmpty(t, check.Name, "Name should not be empty")
			assert.NotEmpty(t, check.Description, "Description should not be empty")
			assert.NotEmpty(t, check.Command, "Command should not be empty")
			// ExpectedIn can be empty for some checks
		})
	}
}

func TestDependencyCheck_CustomChecks(t *testing.T) {
	customChecks := []DependencyCheck{
		{
			Name:        "python",
			Description: "Python 3 installed",
			Command:     "python3 --version",
			ExpectedIn:  "Python 3",
		},
		{
			Name:        "git",
			Description: "Git installed",
			Command:     "git --version",
			ExpectedIn:  "git version",
		},
		{
			Name:        "nginx",
			Description: "Nginx installed and running",
			Command:     "systemctl is-active nginx",
			ExpectedIn:  "active",
		},
	}

	assert.Len(t, customChecks, 3)

	for _, check := range customChecks {
		assert.NotEmpty(t, check.Name)
		assert.NotEmpty(t, check.Description)
		assert.NotEmpty(t, check.Command)
		assert.NotEmpty(t, check.ExpectedIn)
	}
}

func TestValidateDependenciesArgs_Structure(t *testing.T) {
	// Test that ValidateDependenciesArgs can be created with required fields
	args := &ValidateDependenciesArgs{
		NodeName: "test-node",
		Checks:   GetStandardDependencyChecks(),
	}

	assert.Equal(t, "test-node", args.NodeName)
	assert.NotNil(t, args.Checks)
	assert.Greater(t, len(args.Checks), 0)
}

func TestDependencyValidationResult_SuccessCase(t *testing.T) {
	result := DependencyValidationResult{
		Name:    "docker",
		Success: true,
		Output:  "Docker version 20.10.21",
		Error:   nil,
	}

	assert.Equal(t, "docker", result.Name)
	assert.True(t, result.Success)
	assert.Contains(t, result.Output, "Docker version")
	assert.NoError(t, result.Error)
}

func TestDependencyValidationResult_FailureCase(t *testing.T) {
	result := DependencyValidationResult{
		Name:    "missing-dep",
		Success: false,
		Output:  "",
		Error:   assert.AnError,
	}

	assert.Equal(t, "missing-dep", result.Name)
	assert.False(t, result.Success)
	assert.Empty(t, result.Output)
	assert.Error(t, result.Error)
}

func TestGetStandardDependencyChecks_Count(t *testing.T) {
	checks := GetStandardDependencyChecks()

	// Verify we have the expected number of standard checks
	expectedChecks := 6 // docker, wireguard-tools, curl, systemctl, ip-forwarding, disk-space
	assert.Equal(t, expectedChecks, len(checks), "Expected %d standard dependency checks", expectedChecks)
}

func TestGetStandardDependencyChecks_CommandFormats(t *testing.T) {
	checks := GetStandardDependencyChecks()

	for _, check := range checks {
		t.Run(check.Name+"_CommandFormat", func(t *testing.T) {
			// All commands should be valid shell commands (non-empty strings)
			assert.NotEmpty(t, check.Command)
			assert.IsType(t, "", check.Command)

			// Commands should not contain dangerous characters (basic safety check)
			assert.NotContains(t, check.Command, "rm -rf")
			assert.NotContains(t, check.Command, "; rm")
		})
	}
}
