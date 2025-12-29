package cluster

import (
	"strings"
	"testing"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/chalkan3/sloth-kubernetes/pkg/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRKE2Manager(t *testing.T) {
	k8sConfig := &config.KubernetesConfig{
		Version:       "v1.28.5+rke2r1",
		Distribution:  "rke2",
		NetworkPlugin: "calico",
		PodCIDR:       "10.244.0.0/16",
		ServiceCIDR:   "10.96.0.0/12",
		ClusterDNS:    "10.96.0.10",
		ClusterDomain: "cluster.local",
	}

	manager := NewRKE2Manager(nil, k8sConfig)

	require.NotNil(t, manager)
	assert.Equal(t, k8sConfig, manager.config)
	assert.NotNil(t, manager.rke2Config)
	assert.Empty(t, manager.nodes)
}

func TestRKE2Manager_AddNode(t *testing.T) {
	k8sConfig := &config.KubernetesConfig{
		Version:      "v1.28.5+rke2r1",
		Distribution: "rke2",
	}

	manager := NewRKE2Manager(nil, k8sConfig)

	node := &providers.NodeOutput{
		Name:        "test-master-1",
		WireGuardIP: "10.8.0.10",
		SSHUser:     "root",
		Labels:      map[string]string{"role": "master"},
	}

	manager.AddNode(node)

	assert.Len(t, manager.nodes, 1)
	assert.Equal(t, "test-master-1", manager.nodes[0].Name)
}

func TestRKE2Manager_GetNodes(t *testing.T) {
	k8sConfig := &config.KubernetesConfig{
		Version:      "v1.28.5+rke2r1",
		Distribution: "rke2",
	}

	manager := NewRKE2Manager(nil, k8sConfig)

	nodes := []*providers.NodeOutput{
		{Name: "master-1", Labels: map[string]string{"role": "master"}},
		{Name: "worker-1", Labels: map[string]string{"role": "worker"}},
		{Name: "worker-2", Labels: map[string]string{"role": "worker"}},
	}

	for _, node := range nodes {
		manager.AddNode(node)
	}

	result := manager.GetNodes()
	assert.Len(t, result, 3)
}

func TestRKE2Manager_IsMasterNode(t *testing.T) {
	k8sConfig := &config.KubernetesConfig{
		Version:      "v1.28.5+rke2r1",
		Distribution: "rke2",
	}

	manager := NewRKE2Manager(nil, k8sConfig)

	tests := []struct {
		name     string
		node     *providers.NodeOutput
		isMaster bool
	}{
		{
			name:     "Master by label",
			node:     &providers.NodeOutput{Name: "node1", Labels: map[string]string{"role": "master"}},
			isMaster: true,
		},
		{
			name:     "Controlplane by label",
			node:     &providers.NodeOutput{Name: "node2", Labels: map[string]string{"role": "controlplane"}},
			isMaster: true,
		},
		{
			name:     "Control-plane by label",
			node:     &providers.NodeOutput{Name: "node3", Labels: map[string]string{"role": "control-plane"}},
			isMaster: true,
		},
		{
			name:     "Worker by label",
			node:     &providers.NodeOutput{Name: "node4", Labels: map[string]string{"role": "worker"}},
			isMaster: false,
		},
		{
			name:     "Master by name",
			node:     &providers.NodeOutput{Name: "prod-master-1", Labels: map[string]string{}},
			isMaster: true,
		},
		{
			name:     "Control by name",
			node:     &providers.NodeOutput{Name: "control-plane-1", Labels: map[string]string{}},
			isMaster: true,
		},
		{
			name:     "Worker by name",
			node:     &providers.NodeOutput{Name: "worker-1", Labels: map[string]string{}},
			isMaster: false,
		},
		{
			name:     "Arbitrary name defaults to worker",
			node:     &providers.NodeOutput{Name: "node-abc", Labels: map[string]string{}},
			isMaster: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.isMasterNode(tt.node)
			assert.Equal(t, tt.isMaster, result)
		})
	}
}

func TestRKE2Manager_GetMasterNodes(t *testing.T) {
	k8sConfig := &config.KubernetesConfig{
		Version:      "v1.28.5+rke2r1",
		Distribution: "rke2",
	}

	manager := NewRKE2Manager(nil, k8sConfig)

	nodes := []*providers.NodeOutput{
		{Name: "master-1", Labels: map[string]string{"role": "master"}},
		{Name: "master-2", Labels: map[string]string{"role": "master"}},
		{Name: "master-3", Labels: map[string]string{"role": "master"}},
		{Name: "worker-1", Labels: map[string]string{"role": "worker"}},
		{Name: "worker-2", Labels: map[string]string{"role": "worker"}},
	}

	for _, node := range nodes {
		manager.AddNode(node)
	}

	masters := manager.getMasterNodes()
	assert.Len(t, masters, 3)

	for _, master := range masters {
		assert.True(t, strings.Contains(master.Name, "master"))
	}
}

func TestRKE2Manager_GetWorkerNodes(t *testing.T) {
	k8sConfig := &config.KubernetesConfig{
		Version:      "v1.28.5+rke2r1",
		Distribution: "rke2",
	}

	manager := NewRKE2Manager(nil, k8sConfig)

	nodes := []*providers.NodeOutput{
		{Name: "master-1", Labels: map[string]string{"role": "master"}},
		{Name: "worker-1", Labels: map[string]string{"role": "worker"}},
		{Name: "worker-2", Labels: map[string]string{"role": "worker"}},
		{Name: "worker-3", Labels: map[string]string{"role": "worker"}},
	}

	for _, node := range nodes {
		manager.AddNode(node)
	}

	workers := manager.getWorkerNodes()
	assert.Len(t, workers, 3)

	for _, worker := range workers {
		assert.True(t, strings.Contains(worker.Name, "worker"))
	}
}

func TestRKE2Manager_SetSSHPrivateKey(t *testing.T) {
	k8sConfig := &config.KubernetesConfig{
		Version:      "v1.28.5+rke2r1",
		Distribution: "rke2",
	}

	manager := NewRKE2Manager(nil, k8sConfig)

	testKey := "-----BEGIN RSA PRIVATE KEY-----\ntest-key-content\n-----END RSA PRIVATE KEY-----"
	manager.SetSSHPrivateKey(testKey)

	assert.Equal(t, testKey, manager.sshPrivateKey)
}

func TestRKE2Manager_DefaultConfig(t *testing.T) {
	// Test that default RKE2 config is properly merged
	k8sConfig := &config.KubernetesConfig{
		Version:      "v1.28.5+rke2r1",
		Distribution: "rke2",
	}

	manager := NewRKE2Manager(nil, k8sConfig)

	// Check defaults are applied
	assert.Equal(t, "stable", manager.rke2Config.Channel)
	assert.NotEmpty(t, manager.rke2Config.ClusterToken)
	assert.Equal(t, "/var/lib/rancher/rke2", manager.rke2Config.DataDir)
	assert.Equal(t, "0 */12 * * *", manager.rke2Config.SnapshotScheduleCron)
	assert.Equal(t, 5, manager.rke2Config.SnapshotRetention)
}

func TestRKE2Manager_CustomConfig(t *testing.T) {
	k8sConfig := &config.KubernetesConfig{
		Version:      "v1.28.5+rke2r1",
		Distribution: "rke2",
		RKE2: &config.RKE2Config{
			Version:              "v1.29.0+rke2r1",
			Channel:              "latest",
			ClusterToken:         "custom-token-123",
			TLSSan:               []string{"api.example.com", "10.0.0.1"},
			DisableComponents:    []string{"rke2-ingress-nginx", "rke2-metrics-server"},
			SecretsEncryption:    true,
			SnapshotScheduleCron: "0 0 * * *",
			SnapshotRetention:    10,
		},
	}

	manager := NewRKE2Manager(nil, k8sConfig)

	// Check custom values are used
	assert.Equal(t, "v1.29.0+rke2r1", manager.rke2Config.Version)
	assert.Equal(t, "latest", manager.rke2Config.Channel)
	assert.Equal(t, "custom-token-123", manager.rke2Config.ClusterToken)
	assert.Contains(t, manager.rke2Config.TLSSan, "api.example.com")
	assert.Contains(t, manager.rke2Config.DisableComponents, "rke2-ingress-nginx")
	assert.True(t, manager.rke2Config.SecretsEncryption)
	assert.Equal(t, "0 0 * * *", manager.rke2Config.SnapshotScheduleCron)
	assert.Equal(t, 10, manager.rke2Config.SnapshotRetention)
}

func TestRKE2Manager_EmptyNodeList(t *testing.T) {
	k8sConfig := &config.KubernetesConfig{
		Version:      "v1.28.5+rke2r1",
		Distribution: "rke2",
	}

	manager := NewRKE2Manager(nil, k8sConfig)

	masters := manager.getMasterNodes()
	workers := manager.getWorkerNodes()
	allNodes := manager.GetNodes()

	assert.Empty(t, masters)
	assert.Empty(t, workers)
	assert.Empty(t, allNodes)
}

func TestRKE2Manager_MixedNodeLabels(t *testing.T) {
	k8sConfig := &config.KubernetesConfig{
		Version:      "v1.28.5+rke2r1",
		Distribution: "rke2",
	}

	manager := NewRKE2Manager(nil, k8sConfig)

	// Add nodes with various label combinations
	nodes := []*providers.NodeOutput{
		{Name: "master-with-label", Labels: map[string]string{"role": "master", "env": "prod"}},
		{Name: "worker-with-label", Labels: map[string]string{"role": "worker", "env": "prod"}},
		{Name: "master-by-name-only", Labels: map[string]string{"env": "prod"}},
		{Name: "control-plane-test", Labels: map[string]string{}},
		{Name: "random-node", Labels: nil},
	}

	for _, node := range nodes {
		if node.Labels == nil {
			node.Labels = map[string]string{}
		}
		manager.AddNode(node)
	}

	masters := manager.getMasterNodes()
	workers := manager.getWorkerNodes()

	// Should have 3 masters: master-with-label, master-by-name-only, control-plane-test
	assert.Len(t, masters, 3)
	// Should have 2 workers: worker-with-label, random-node
	assert.Len(t, workers, 2)
}

func TestRKE2Config_BuildServerConfig(t *testing.T) {
	rke2Config := &config.RKE2Config{
		Version:              "v1.28.5+rke2r1",
		Channel:              "stable",
		ClusterToken:         "test-token",
		TLSSan:               []string{"api.example.com"},
		DisableComponents:    []string{"rke2-ingress-nginx"},
		SnapshotScheduleCron: "0 0 * * *",
		SnapshotRetention:    5,
		SecretsEncryption:    true,
	}

	k8sConfig := &config.KubernetesConfig{
		PodCIDR:       "10.244.0.0/16",
		ServiceCIDR:   "10.96.0.0/12",
		ClusterDNS:    "10.96.0.10",
		NetworkPlugin: "calico",
	}

	serverConfig := config.BuildRKE2ServerConfig(rke2Config, "10.8.0.10", "master-1", true, "", k8sConfig)

	// Verify key config elements
	assert.Contains(t, serverConfig, "token: test-token")
	assert.Contains(t, serverConfig, "node-name: master-1")
	assert.Contains(t, serverConfig, "node-ip: 10.8.0.10")
	assert.Contains(t, serverConfig, "bind-address: 10.8.0.10")
	assert.Contains(t, serverConfig, "cluster-cidr: 10.244.0.0/16")
	assert.Contains(t, serverConfig, "service-cidr: 10.96.0.0/12")
	assert.Contains(t, serverConfig, "secrets-encryption: true")
	assert.Contains(t, serverConfig, "api.example.com")
}

func TestRKE2Config_BuildAgentConfig(t *testing.T) {
	rke2Config := &config.RKE2Config{
		ClusterToken: "test-token",
		SeLinux:      false,
	}

	agentConfig := config.BuildRKE2AgentConfig(rke2Config, "10.8.0.20", "worker-1", "10.8.0.10")

	// Verify key config elements
	assert.Contains(t, agentConfig, "token: test-token")
	assert.Contains(t, agentConfig, "server: https://10.8.0.10:9345")
	assert.Contains(t, agentConfig, "node-name: worker-1")
	assert.Contains(t, agentConfig, "node-ip: 10.8.0.20")
}

func TestRKE2Config_GetInstallCommand(t *testing.T) {
	tests := []struct {
		name       string
		config     *config.RKE2Config
		isServer   bool
		shouldHave []string
	}{
		{
			name: "Server install with version",
			config: &config.RKE2Config{
				Version: "v1.28.5+rke2r1",
			},
			isServer: true,
			shouldHave: []string{
				"curl -sfL https://get.rke2.io",
				"INSTALL_RKE2_TYPE=server",
				"INSTALL_RKE2_VERSION=v1.28.5+rke2r1",
			},
		},
		{
			name: "Agent install with channel",
			config: &config.RKE2Config{
				Channel: "stable",
			},
			isServer: false,
			shouldHave: []string{
				"curl -sfL https://get.rke2.io",
				"INSTALL_RKE2_TYPE=agent",
				"INSTALL_RKE2_CHANNEL=stable",
			},
		},
		{
			name: "Server install with channel (no version)",
			config: &config.RKE2Config{
				Channel: "latest",
			},
			isServer: true,
			shouldHave: []string{
				"INSTALL_RKE2_TYPE=server",
				"INSTALL_RKE2_CHANNEL=latest",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := config.GetRKE2InstallCommand(tt.config, tt.isServer)

			for _, expected := range tt.shouldHave {
				assert.Contains(t, cmd, expected)
			}
		})
	}
}
