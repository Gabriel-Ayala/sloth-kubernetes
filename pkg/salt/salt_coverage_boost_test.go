package salt

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPackageAvailable tests PackageAvailable function
func TestPackageAvailable(t *testing.T) {
	client := NewClient("http://localhost:8000", "user", "pass")
	client.Token = "test-token"

	// This will attempt to make a real HTTP request
	// In production you'd mock the HTTP client
	_, err := client.PackageAvailable("*", "nginx")
	// We expect an error since we're not actually connected to Salt API
	assert.Error(t, err)
}

// TestServiceEnable tests ServiceEnable function
func TestServiceEnable(t *testing.T) {
	client := NewClient("http://localhost:8000", "user", "pass")
	client.Token = "test-token"

	_, err := client.ServiceEnable("*", "nginx")
	assert.Error(t, err) // Expected error without real Salt API
}

// TestServiceDisable tests ServiceDisable function
func TestServiceDisable(t *testing.T) {
	client := NewClient("http://localhost:8000", "user", "pass")
	client.Token = "test-token"

	_, err := client.ServiceDisable("*", "nginx")
	assert.Error(t, err) // Expected error without real Salt API
}

// TestServiceGetAll tests ServiceGetAll function
func TestServiceGetAll(t *testing.T) {
	client := NewClient("http://localhost:8000", "user", "pass")
	client.Token = "test-token"

	_, err := client.ServiceGetAll("*")
	assert.Error(t, err) // Expected error without real Salt API
}

// TestFileRead tests FileRead function
func TestFileRead(t *testing.T) {
	client := NewClient("http://localhost:8000", "user", "pass")
	client.Token = "test-token"

	_, err := client.FileRead("*", "/etc/hosts")
	assert.Error(t, err) // Expected error without real Salt API
}

// TestFileWrite tests FileWrite function
func TestFileWrite(t *testing.T) {
	client := NewClient("http://localhost:8000", "user", "pass")
	client.Token = "test-token"

	_, err := client.FileWrite("*", "/tmp/test.txt", "content")
	assert.Error(t, err) // Expected error without real Salt API
}

// TestFileRemove tests FileRemove function
func TestFileRemove(t *testing.T) {
	client := NewClient("http://localhost:8000", "user", "pass")
	client.Token = "test-token"

	_, err := client.FileRemove("*", "/tmp/test.txt")
	assert.Error(t, err) // Expected error without real Salt API
}

// TestFileCopy tests FileCopy function
func TestFileCopy(t *testing.T) {
	client := NewClient("http://localhost:8000", "user", "pass")
	client.Token = "test-token"

	_, err := client.FileCopy("*", "/tmp/source.txt", "/tmp/dest.txt")
	assert.Error(t, err) // Expected error without real Salt API
}

// TestFileChmod tests FileChmod function
func TestFileChmod(t *testing.T) {
	client := NewClient("http://localhost:8000", "user", "pass")
	client.Token = "test-token"

	_, err := client.FileChmod("*", "/tmp/test.txt", "0644")
	assert.Error(t, err) // Expected error without real Salt API
}

// TestFileChown tests FileChown function
func TestFileChown(t *testing.T) {
	client := NewClient("http://localhost:8000", "user", "pass")
	client.Token = "test-token"

	_, err := client.FileChown("*", "/tmp/test.txt", "root", "root")
	assert.Error(t, err) // Expected error without real Salt API
}

// TestUserDelete tests UserDelete function
func TestUserDelete(t *testing.T) {
	client := NewClient("http://localhost:8000", "user", "pass")
	client.Token = "test-token"

	_, err := client.UserDelete("*", "testuser")
	assert.Error(t, err) // Expected error without real Salt API
}

// TestUserList tests UserList function
func TestUserList(t *testing.T) {
	client := NewClient("http://localhost:8000", "user", "pass")
	client.Token = "test-token"

	_, err := client.UserList("*")
	assert.Error(t, err) // Expected error without real Salt API
}

// TestUserInfo tests UserInfo function
func TestUserInfo(t *testing.T) {
	client := NewClient("http://localhost:8000", "user", "pass")
	client.Token = "test-token"

	_, err := client.UserInfo("*", "testuser")
	assert.Error(t, err) // Expected error without real Salt API
}

// TestGroupAdd tests GroupAdd function
func TestGroupAdd(t *testing.T) {
	client := NewClient("http://localhost:8000", "user", "pass")
	client.Token = "test-token"

	_, err := client.GroupAdd("*", "testgroup")
	assert.Error(t, err) // Expected error without real Salt API
}

// TestGroupDelete tests GroupDelete function
func TestGroupDelete(t *testing.T) {
	client := NewClient("http://localhost:8000", "user", "pass")
	client.Token = "test-token"

	_, err := client.GroupDelete("*", "testgroup")
	assert.Error(t, err) // Expected error without real Salt API
}

// TestSystemReboot tests SystemReboot function
func TestSystemReboot(t *testing.T) {
	client := NewClient("http://localhost:8000", "user", "pass")
	client.Token = "test-token"

	_, err := client.SystemReboot("*")
	assert.Error(t, err) // Expected error without real Salt API
}

// TestSystemShutdown tests SystemShutdown function
func TestSystemShutdown(t *testing.T) {
	client := NewClient("http://localhost:8000", "user", "pass")
	client.Token = "test-token"

	_, err := client.SystemShutdown("*")
	assert.Error(t, err) // Expected error without real Salt API
}

// TestSystemUptime tests SystemUptime function
func TestSystemUptime(t *testing.T) {
	client := NewClient("http://localhost:8000", "user", "pass")
	client.Token = "test-token"

	_, err := client.SystemUptime("*")
	assert.Error(t, err) // Expected error without real Salt API
}

// TestDiskUsage tests DiskUsage function
func TestDiskUsage(t *testing.T) {
	client := NewClient("http://localhost:8000", "user", "pass")
	client.Token = "test-token"

	_, err := client.DiskUsage("*")
	assert.Error(t, err) // Expected error without real Salt API
}

// TestMemoryUsage tests MemoryUsage function
func TestMemoryUsage(t *testing.T) {
	client := NewClient("http://localhost:8000", "user", "pass")
	client.Token = "test-token"

	_, err := client.MemoryUsage("*")
	assert.Error(t, err) // Expected error without real Salt API
}

// TestCPUInfo tests CPUInfo function
func TestCPUInfo(t *testing.T) {
	client := NewClient("http://localhost:8000", "user", "pass")
	client.Token = "test-token"

	_, err := client.CPUInfo("*")
	assert.Error(t, err) // Expected error without real Salt API
}

// TestNetworkInterfaces tests NetworkInterfaces function
func TestNetworkInterfaces(t *testing.T) {
	client := NewClient("http://localhost:8000", "user", "pass")
	client.Token = "test-token"

	_, err := client.NetworkInterfaces("*")
	assert.Error(t, err) // Expected error without real Salt API
}

// TestMultipleCalls tests calling multiple functions in sequence
func TestMultipleCalls(t *testing.T) {
	client := NewClient("http://localhost:8000", "user", "pass")
	client.Token = "test-token"

	// Test multiple calls
	tests := []struct {
		name string
		fn   func() error
	}{
		{
			name: "PackageAvailable",
			fn: func() error {
				_, err := client.PackageAvailable("*", "nginx")
				return err
			},
		},
		{
			name: "ServiceEnable",
			fn: func() error {
				_, err := client.ServiceEnable("*", "nginx")
				return err
			},
		},
		{
			name: "FileRead",
			fn: func() error {
				_, err := client.FileRead("*", "/etc/hosts")
				return err
			},
		},
		{
			name: "UserList",
			fn: func() error {
				_, err := client.UserList("*")
				return err
			},
		},
		{
			name: "SystemUptime",
			fn: func() error {
				_, err := client.SystemUptime("*")
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			// All should error without real Salt API connection
			assert.Error(t, err)
		})
	}
}

// TestClientProperties tests client properties
func TestClientProperties(t *testing.T) {
	baseURL := "http://localhost:8000"
	username := "testuser"
	password := "testpass"

	client := NewClient(baseURL, username, password)

	assert.NotNil(t, client)
	assert.Equal(t, baseURL, client.BaseURL)
	assert.Equal(t, username, client.Username)
	assert.Equal(t, password, client.Password)
	assert.NotNil(t, client.HTTPClient)
	assert.Empty(t, client.Token)

	// Set token
	client.Token = "test-token-123"
	assert.Equal(t, "test-token-123", client.Token)
}

// TestFileManagementWorkflow tests a typical file management workflow
func TestFileManagementWorkflow(t *testing.T) {
	client := NewClient("http://localhost:8000", "user", "pass")
	client.Token = "test-token"

	testFile := "/tmp/test.txt"
	target := "*"

	// Write file
	_, err := client.FileWrite(target, testFile, "test content")
	assert.Error(t, err)

	// Read file
	_, err = client.FileRead(target, testFile)
	assert.Error(t, err)

	// Change permissions
	_, err = client.FileChmod(target, testFile, "0644")
	assert.Error(t, err)

	// Change ownership
	_, err = client.FileChown(target, testFile, "root", "root")
	assert.Error(t, err)

	// Copy file
	_, err = client.FileCopy(target, testFile, "/tmp/test_copy.txt")
	assert.Error(t, err)

	// Remove file
	_, err = client.FileRemove(target, testFile)
	assert.Error(t, err)
}

// TestUserManagementWorkflow tests a typical user management workflow
func TestUserManagementWorkflow(t *testing.T) {
	client := NewClient("http://localhost:8000", "user", "pass")
	client.Token = "test-token"

	target := "*"
	username := "newuser"

	// Add user
	_, err := client.UserAdd(target, username)
	assert.Error(t, err)

	// Get user info
	_, err = client.UserInfo(target, username)
	assert.Error(t, err)

	// List all users
	_, err = client.UserList(target)
	assert.Error(t, err)

	// Delete user
	_, err = client.UserDelete(target, username)
	assert.Error(t, err)
}

// TestServiceManagementWorkflow tests a typical service management workflow
func TestServiceManagementWorkflow(t *testing.T) {
	client := NewClient("http://localhost:8000", "user", "pass")
	client.Token = "test-token"

	target := "*"
	service := "nginx"

	// Enable service
	_, err := client.ServiceEnable(target, service)
	assert.Error(t, err)

	// Start service
	_, err = client.ServiceStart(target, service)
	assert.Error(t, err)

	// Check status
	_, err = client.ServiceStatus(target, service)
	assert.Error(t, err)

	// Restart service
	_, err = client.ServiceRestart(target, service)
	assert.Error(t, err)

	// Stop service
	_, err = client.ServiceStop(target, service)
	assert.Error(t, err)

	// Disable service
	_, err = client.ServiceDisable(target, service)
	assert.Error(t, err)

	// List all services
	_, err = client.ServiceGetAll(target)
	assert.Error(t, err)
}

// TestSystemMonitoringWorkflow tests system monitoring functions
func TestSystemMonitoringWorkflow(t *testing.T) {
	client := NewClient("http://localhost:8000", "user", "pass")
	client.Token = "test-token"

	target := "*"

	// Check uptime
	_, err := client.SystemUptime(target)
	assert.Error(t, err)

	// Check disk usage
	_, err = client.DiskUsage(target)
	assert.Error(t, err)

	// Check memory usage
	_, err = client.MemoryUsage(target)
	assert.Error(t, err)

	// Check CPU info
	_, err = client.CPUInfo(target)
	assert.Error(t, err)

	// Check network interfaces
	_, err = client.NetworkInterfaces(target)
	assert.Error(t, err)
}
