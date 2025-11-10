package network

import (
	"fmt"
	"testing"
	"time"

	"github.com/chalkan3/sloth-kubernetes/pkg/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

// TestNewVPNConnectivityChecker_Creation tests VPN checker creation
func TestNewVPNConnectivityChecker_Creation(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		checker := NewVPNConnectivityChecker(ctx)

		assert.NotNil(t, checker)
		assert.Equal(t, ctx, checker.ctx)
		assert.NotNil(t, checker.nodes)
		assert.NotNil(t, checker.results)
		assert.Equal(t, 5*time.Second, checker.checkInterval)
		assert.Equal(t, 5*time.Minute, checker.timeout)
		assert.Len(t, checker.nodes, 0)
		assert.Len(t, checker.results, 0)

		return nil
	}, pulumi.WithMocks("test", "stack", &TestMockProvider{}))

	assert.NoError(t, err)
}

// TestAddNode_SingleNode tests adding a single node
func TestAddNode_SingleNode(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		checker := NewVPNConnectivityChecker(ctx)

		node := &providers.NodeOutput{
			Name:        "test-node-1",
			WireGuardIP: "10.8.0.1",
		}

		checker.AddNode(node)

		assert.Len(t, checker.nodes, 1)
		assert.Equal(t, node, checker.nodes[0])
		assert.Contains(t, checker.results, "test-node-1")
		assert.Equal(t, "test-node-1", checker.results["test-node-1"].SourceNode)
		assert.NotNil(t, checker.results["test-node-1"].Connections)

		return nil
	}, pulumi.WithMocks("test", "stack", &TestMockProvider{}))

	assert.NoError(t, err)
}

// TestAddNode_MultipleNodes tests adding multiple nodes
func TestAddNode_MultipleNodes(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		checker := NewVPNConnectivityChecker(ctx)

		for i := 1; i <= 5; i++ {
			node := &providers.NodeOutput{
				Name:        fmt.Sprintf("node-%d", i),
				WireGuardIP: fmt.Sprintf("10.8.0.%d", i),
			}
			checker.AddNode(node)
		}

		assert.Len(t, checker.nodes, 5)
		assert.Len(t, checker.results, 5)

		for i := 1; i <= 5; i++ {
			nodeName := fmt.Sprintf("node-%d", i)
			assert.Contains(t, checker.results, nodeName)
		}

		return nil
	}, pulumi.WithMocks("test", "stack", &TestMockProvider{}))

	assert.NoError(t, err)
}

// TestSetSSHKeyPath_Basic tests setting SSH key path
func TestSetSSHKeyPath_Basic(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		checker := NewVPNConnectivityChecker(ctx)

		assert.Empty(t, checker.sshKeyPath)

		keyPath := "/home/user/.ssh/id_rsa"
		checker.SetSSHKeyPath(keyPath)

		assert.Equal(t, keyPath, checker.sshKeyPath)

		return nil
	}, pulumi.WithMocks("test", "stack", &TestMockProvider{}))

	assert.NoError(t, err)
}

// TestSetSSHKeyPath_Update tests updating SSH key path
func TestSetSSHKeyPath_Update(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		checker := NewVPNConnectivityChecker(ctx)

		keyPath1 := "/home/user/.ssh/id_rsa"
		checker.SetSSHKeyPath(keyPath1)
		assert.Equal(t, keyPath1, checker.sshKeyPath)

		keyPath2 := "/home/user/.ssh/id_ed25519"
		checker.SetSSHKeyPath(keyPath2)
		assert.Equal(t, keyPath2, checker.sshKeyPath)

		return nil
	}, pulumi.WithMocks("test", "stack", &TestMockProvider{}))

	assert.NoError(t, err)
}

// TestGetConnectivityMatrix_NoNodes tests getting matrix with no nodes
func TestGetConnectivityMatrix_NoNodes(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		checker := NewVPNConnectivityChecker(ctx)

		matrix := checker.GetConnectivityMatrix()

		assert.NotNil(t, matrix)
		assert.Empty(t, matrix)

		return nil
	}, pulumi.WithMocks("test", "stack", &TestMockProvider{}))

	assert.NoError(t, err)
}

// TestGetConnectivityMatrix_WithNodes tests getting matrix with nodes added
func TestGetConnectivityMatrix_WithNodes(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		checker := NewVPNConnectivityChecker(ctx)

		// Add nodes
		node1 := &providers.NodeOutput{
			Name:        "node1",
			WireGuardIP: "10.8.0.1",
		}
		node2 := &providers.NodeOutput{
			Name:        "node2",
			WireGuardIP: "10.8.0.2",
		}

		checker.AddNode(node1)
		checker.AddNode(node2)

		// Add some connectivity data
		checker.results["node1"].Connections["node2"] = &ConnectionStatus{
			TargetNode:  "node2",
			TargetIP:    "10.8.0.2",
			IsConnected: true,
			Latency:     10 * time.Millisecond,
		}

		matrix := checker.GetConnectivityMatrix()

		assert.NotNil(t, matrix)
		assert.Contains(t, matrix, "node1")
		assert.NotNil(t, matrix["node1"])

		return nil
	}, pulumi.WithMocks("test", "stack", &TestMockProvider{}))

	assert.NoError(t, err)
}

// TestConnectivityResult_Creation tests connectivity result structure
func TestConnectivityResult_Creation(t *testing.T) {
	now := time.Now()
	result := &ConnectivityResult{
		SourceNode:   "test-node",
		Timestamp:    now,
		Connections:  make(map[string]*ConnectionStatus),
		AllConnected: true,
		Error:        nil,
	}

	assert.Equal(t, "test-node", result.SourceNode)
	assert.True(t, result.AllConnected)
	assert.NoError(t, result.Error)
	assert.NotNil(t, result.Connections)
	assert.Equal(t, now, result.Timestamp)
}

// TestConnectionStatus_Creation tests connection status structure
func TestConnectionStatus_Creation(t *testing.T) {
	now := time.Now()
	status := &ConnectionStatus{
		TargetNode:  "node2",
		TargetIP:    "10.8.0.2",
		IsConnected: true,
		Latency:     15 * time.Millisecond,
		PacketLoss:  0.0,
		LastCheck:   now,
		Error:       nil,
	}

	assert.Equal(t, "node2", status.TargetNode)
	assert.Equal(t, "10.8.0.2", status.TargetIP)
	assert.True(t, status.IsConnected)
	assert.Equal(t, 15*time.Millisecond, status.Latency)
	assert.Equal(t, 0.0, status.PacketLoss)
	assert.NoError(t, status.Error)
	assert.Equal(t, now, status.LastCheck)
}

// TestWireGuardStats_Creation tests WireGuard stats structure
func TestWireGuardStats_Creation(t *testing.T) {
	now := time.Now()
	stats := &WireGuardStats{
		Interface:           "wg0",
		PublicKey:           "testkey123",
		Endpoint:            "1.2.3.4:51820",
		LastHandshake:       now,
		TransferRX:          1024000,
		TransferTX:          2048000,
		PersistentKeepAlive: 25,
	}

	assert.Equal(t, "wg0", stats.Interface)
	assert.Equal(t, "testkey123", stats.PublicKey)
	assert.Equal(t, "1.2.3.4:51820", stats.Endpoint)
	assert.Equal(t, now, stats.LastHandshake)
	assert.Equal(t, int64(1024000), stats.TransferRX)
	assert.Equal(t, int64(2048000), stats.TransferTX)
	assert.Equal(t, 25, stats.PersistentKeepAlive)
}

// TestConnectionStatus_Failure tests connection status with failure
func TestConnectionStatus_Failure(t *testing.T) {
	err := fmt.Errorf("connection timeout")
	status := &ConnectionStatus{
		TargetNode:  "node2",
		TargetIP:    "10.8.0.2",
		IsConnected: false,
		Latency:     0,
		PacketLoss:  100.0,
		LastCheck:   time.Now(),
		Error:       err,
	}

	assert.False(t, status.IsConnected)
	assert.Error(t, status.Error)
	assert.Equal(t, "connection timeout", status.Error.Error())
	assert.Equal(t, 100.0, status.PacketLoss)
}

// TestConnectionStatus_PartialLoss tests connection with packet loss
func TestConnectionStatus_PartialLoss(t *testing.T) {
	status := &ConnectionStatus{
		TargetNode:  "node2",
		TargetIP:    "10.8.0.2",
		IsConnected: true,
		Latency:     50 * time.Millisecond,
		PacketLoss:  25.5,
		LastCheck:   time.Now(),
		Error:       nil,
	}

	assert.True(t, status.IsConnected)
	assert.Equal(t, 25.5, status.PacketLoss)
	assert.Greater(t, status.Latency, time.Duration(0))
}

// TestMockProvider implements pulumi.ResourceState for testing
type TestMockProvider struct {
	pulumi.ResourceState
}

func (m *TestMockProvider) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	return args.Name + "_id", args.Inputs, nil
}

func (m *TestMockProvider) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return args.Args, nil
}
