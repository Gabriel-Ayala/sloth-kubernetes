package cloudinit

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateUserDataWithHostname(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		expected []string
	}{
		{
			name:     "WithHostname",
			hostname: "node01",
			expected: []string{
				"#cloud-config",
				"hostname: node01",
				"fqdn: node01.cluster.local",
				"manage_etc_hosts: true",
				"curl",
				"wget",
				"git",
				"wireguard",
				"wireguard-tools",
				"net-tools",
				"sysctl -w net.ipv4.ip_forward=1",
			},
		},
		{
			name:     "EmptyHostname",
			hostname: "",
			expected: []string{
				"#cloud-config",
				"packages:",
				"curl",
				"wireguard",
				"runcmd:",
			},
		},
		{
			name:     "LongHostname",
			hostname: "very-long-hostname-master-01",
			expected: []string{
				"hostname: very-long-hostname-master-01",
				"fqdn: very-long-hostname-master-01.cluster.local",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateUserDataWithHostname(tt.hostname)

			// Check that it starts with cloud-config directive
			assert.True(t, strings.HasPrefix(result, "#cloud-config"))

			// Check all expected strings are present
			for _, expected := range tt.expected {
				assert.Contains(t, result, expected,
					"Expected to find '%s' in generated user data", expected)
			}

			// Should NOT contain Salt-related configs when no master IP
			assert.NotContains(t, result, "salt-minion")
			assert.NotContains(t, result, "/etc/salt/minion.d/master.conf")
		})
	}
}

func TestGenerateUserDataWithHostnameAndSalt(t *testing.T) {
	tests := []struct {
		name         string
		hostname     string
		saltMasterIP string
		expected     []string
		notExpected  []string
	}{
		{
			name:         "WithSaltMaster",
			hostname:     "worker-01",
			saltMasterIP: "10.0.1.100",
			expected: []string{
				"#cloud-config",
				"hostname: worker-01",
				"fqdn: worker-01.cluster.local",
				"write_files:",
				"/etc/salt/minion.d/master.conf",
				"master: 10.0.1.100",
				"/etc/salt/minion.d/minion_id.conf",
				"id: worker-01",
				"Installing Salt Minion",
				"bootstrap-salt.sh",
				"systemctl restart salt-minion",
				"systemctl enable salt-minion",
			},
			notExpected: []string{},
		},
		{
			name:         "WithoutSaltMaster",
			hostname:     "master-01",
			saltMasterIP: "",
			expected: []string{
				"#cloud-config",
				"hostname: master-01",
				"packages:",
				"wireguard",
			},
			notExpected: []string{
				"salt-minion",
				"/etc/salt/",
				"bootstrap-salt.sh",
			},
		},
		{
			name:         "EmptyHostnameWithSalt",
			hostname:     "",
			saltMasterIP: "192.168.1.10",
			expected: []string{
				"#cloud-config",
				"master: 192.168.1.10",
				"Installing Salt Minion",
			},
			notExpected: []string{
				"hostname:",
				"fqdn:",
			},
		},
		{
			name:         "IPv6SaltMaster",
			hostname:     "node-ipv6",
			saltMasterIP: "fe80::1",
			expected: []string{
				"master: fe80::1",
				"id: node-ipv6",
			},
			notExpected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateUserDataWithHostnameAndSalt(tt.hostname, tt.saltMasterIP)

			// Check all expected strings are present
			for _, expected := range tt.expected {
				assert.Contains(t, result, expected,
					"Expected to find '%s' in generated user data", expected)
			}

			// Check that unwanted strings are NOT present
			for _, notExpected := range tt.notExpected {
				assert.NotContains(t, result, notExpected,
					"Did not expect to find '%s' in generated user data", notExpected)
			}

			// Verify structure
			assert.True(t, strings.HasPrefix(result, "#cloud-config"))
			assert.Contains(t, result, "packages:")
			assert.Contains(t, result, "runcmd:")
		})
	}
}

func TestUserDataPackages(t *testing.T) {
	// Ensure all required packages are present
	result := GenerateUserDataWithHostname("test-node")

	requiredPackages := []string{
		"curl",
		"wget",
		"git",
		"wireguard",
		"wireguard-tools",
		"net-tools",
	}

	for _, pkg := range requiredPackages {
		assert.Contains(t, result, pkg,
			"Expected package '%s' to be in packages list", pkg)
	}
}

func TestUserDataNetworkSettings(t *testing.T) {
	// Ensure IP forwarding is configured
	result := GenerateUserDataWithHostname("test-node")

	networkSettings := []string{
		"sysctl -w net.ipv4.ip_forward=1",
		"net.ipv4.ip_forward=1",
		"sysctl -w net.ipv6.conf.all.forwarding=1",
		"net.ipv6.conf.all.forwarding=1",
	}

	for _, setting := range networkSettings {
		assert.Contains(t, result, setting,
			"Expected network setting '%s' in user data", setting)
	}
}

func TestUserDataSaltConfiguration(t *testing.T) {
	hostname := "salt-test-node"
	saltMaster := "10.20.30.40"

	result := GenerateUserDataWithHostnameAndSalt(hostname, saltMaster)

	// Check Salt master configuration
	assert.Contains(t, result, "/etc/salt/minion.d/master.conf")
	assert.Contains(t, result, "master: 10.20.30.40")

	// Check Salt minion ID configuration
	assert.Contains(t, result, "/etc/salt/minion.d/minion_id.conf")
	assert.Contains(t, result, "id: salt-test-node")

	// Check file permissions
	assert.Contains(t, result, "owner: root:root")
	assert.Contains(t, result, "permissions: '0644'")

	// Check Salt installation commands
	assert.Contains(t, result, "bootstrap-salt.sh")
	assert.Contains(t, result, "sh /tmp/bootstrap-salt.sh stable")
}

func TestUserDataFormat(t *testing.T) {
	tests := []struct {
		name         string
		hostname     string
		saltMasterIP string
	}{
		{"BasicFormat", "node1", ""},
		{"WithSaltFormat", "node2", "10.0.0.1"},
		{"EmptyHostnameFormat", "", "10.0.0.2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateUserDataWithHostnameAndSalt(tt.hostname, tt.saltMasterIP)

			// Must start with cloud-config directive
			assert.True(t, strings.HasPrefix(result, "#cloud-config\n"))

			// Must contain packages section
			assert.Contains(t, result, "packages:")

			// Must contain runcmd section
			assert.Contains(t, result, "runcmd:")

			// Should be valid YAML-like structure (basic checks)
			assert.NotContains(t, result, "}\n{") // No broken JSON
			lines := strings.Split(result, "\n")
			assert.Greater(t, len(lines), 10, "Should have multiple lines")
		})
	}
}

func TestUserDataHostnameFormats(t *testing.T) {
	tests := []struct {
		hostname     string
		expectedFQDN string
	}{
		{"simple", "simple.cluster.local"},
		{"node-01", "node-01.cluster.local"},
		{"master-node-123", "master-node-123.cluster.local"},
		{"k8s-worker", "k8s-worker.cluster.local"},
	}

	for _, tt := range tests {
		t.Run(tt.hostname, func(t *testing.T) {
			result := GenerateUserDataWithHostname(tt.hostname)
			assert.Contains(t, result, "hostname: "+tt.hostname)
			assert.Contains(t, result, "fqdn: "+tt.expectedFQDN)
		})
	}
}

func TestUserDataSaltMasterIPFormats(t *testing.T) {
	tests := []struct {
		name     string
		masterIP string
	}{
		{"IPv4_Private", "10.0.0.1"},
		{"IPv4_Public", "203.0.113.1"},
		{"IPv6", "2001:db8::1"},
		{"IPv6_Short", "::1"},
		{"Hostname", "salt-master.example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateUserDataWithHostnameAndSalt("test-node", tt.masterIP)
			assert.Contains(t, result, "master: "+tt.masterIP)
		})
	}
}
