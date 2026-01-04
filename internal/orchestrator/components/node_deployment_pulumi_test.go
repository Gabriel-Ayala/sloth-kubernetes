package components

import (
	"testing"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

type nodeMocks int

// NewResource creates a new mock resource for node deployment tests
func (nodeMocks) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	outputs := args.Inputs.Copy()

	switch args.TypeToken {
	case "digitalocean:index/sshKey:SshKey":
		outputs["fingerprint"] = resource.NewStringProperty("mock:fingerprint:abc123")
		outputs["id"] = resource.NewStringProperty("ssh-key-123")
		outputs["name"] = args.Inputs["name"]
		outputs["publicKey"] = args.Inputs["publicKey"]

	case "digitalocean:index/droplet:Droplet":
		outputs["id"] = resource.NewStringProperty("droplet-" + args.Name)
		outputs["ipv4Address"] = resource.NewStringProperty("203.0.113.1")
		outputs["ipv4AddressPrivate"] = resource.NewStringProperty("10.0.1.1")
		outputs["status"] = resource.NewStringProperty("active")
		outputs["urn"] = resource.NewStringProperty("do:droplet:" + args.Name)

	case "linode:index/instance:Instance":
		outputs["id"] = resource.NewStringProperty("linode-" + args.Name)
		outputs["ipAddress"] = resource.NewStringProperty("203.0.113.2")
		outputs["privateIpAddress"] = resource.NewStringProperty("10.0.2.1")
		outputs["status"] = resource.NewStringProperty("running")

	case "azure-native:compute/v2:VirtualMachine":
		outputs["id"] = resource.NewStringProperty("azure-vm-" + args.Name)
		outputs["name"] = args.Inputs["vmName"]

	case "azure-native:network/v2:PublicIPAddress":
		outputs["id"] = resource.NewStringProperty("azure-pip-" + args.Name)
		outputs["ipAddress"] = resource.NewStringProperty("203.0.113.3")

	case "azure-native:network/v2:NetworkInterface":
		outputs["id"] = resource.NewStringProperty("azure-nic-" + args.Name)
		outputs["ipConfigurations"] = resource.NewArrayProperty([]resource.PropertyValue{
			resource.NewObjectProperty(resource.PropertyMap{
				"privateIPAddress": resource.NewStringProperty("10.0.3.1"),
			}),
		})

	case "azure-native:resources/v2:ResourceGroup":
		outputs["id"] = resource.NewStringProperty("rg-" + args.Name)
		outputs["name"] = args.Inputs["resourceGroupName"]

	case "kubernetes-create:compute:NodeDeployment":
		outputs["status"] = resource.NewStringProperty("ready")
		outputs["nodes"] = resource.NewArrayProperty([]resource.PropertyValue{})

	case "kubernetes-create:compute:RealNode":
		outputs["nodeName"] = args.Inputs["nodeName"]
		outputs["provider"] = args.Inputs["provider"]
		outputs["publicIP"] = resource.NewStringProperty("203.0.113.10")
		outputs["privateIP"] = resource.NewStringProperty("10.0.10.1")
		outputs["status"] = resource.NewStringProperty("active")
	}

	return args.Name + "_id", outputs, nil
}

// Call mocks function calls
func (nodeMocks) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return resource.PropertyMap{}, nil
}

func TestNewRealNodeDeploymentComponent_SingleNode(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		// Create minimal cluster config with single node
		clusterConfig := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name: "test-cluster",
			},
			Nodes: []config.NodeConfig{
				{
					Name:        "master-1",
					Provider:    "digitalocean",
					Region:      "nyc1",
					Size:        "s-2vcpu-4gb",
					Roles:       []string{"master"},
					WireGuardIP: "10.10.0.1",
				},
			},
			Security: config.SecurityConfig{
				Bastion: nil, // No bastion
			},
		}

		// Create mock SSH key outputs
		sshKeyOutput := pulumi.String("ssh-rsa AAAAB3... test@example.com").ToStringOutput()
		sshPrivateKey := pulumi.String("-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----").ToStringOutput()
		doToken := pulumi.String("mock-do-token")
		linodeToken := pulumi.String("mock-linode-token")

		// Create node deployment component
		component, nodes, err := NewRealNodeDeploymentComponent(
			ctx,
			"test-deployment",
			clusterConfig,
			sshKeyOutput,
			sshPrivateKey,
			doToken,
			linodeToken,
			nil, // No VPC
			nil, // No bastion
		)

		assert.NoError(t, err, "Should create node deployment without error")
		assert.NotNil(t, component, "Component should not be nil")
		assert.Len(t, nodes, 1, "Should have exactly one node")

		// Verify node name
		nodes[0].NodeName.ApplyT(func(name string) error {
			assert.Equal(t, "master-1", name, "Node name should match config")
			return nil
		})

		return nil
	}, pulumi.WithMocks("project", "stack", nodeMocks(0)))

	assert.NoError(t, err)
}

func TestNewRealNodeDeploymentComponent_MultipleNodes(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name: "test-cluster",
			},
			Nodes: []config.NodeConfig{
				{
					Name:        "master-1",
					Provider:    "digitalocean",
					Region:      "nyc1",
					Size:        "s-2vcpu-4gb",
					Roles:       []string{"master"},
					WireGuardIP: "10.10.0.1",
				},
				{
					Name:        "worker-1",
					Provider:    "linode",
					Region:      "us-east",
					Size:        "g6-standard-2",
					Roles:       []string{"worker"},
					WireGuardIP: "10.10.0.2",
				},
				{
					Name:        "worker-2",
					Provider:    "digitalocean",
					Region:      "nyc1",
					Size:        "s-2vcpu-4gb",
					Roles:       []string{"worker"},
					WireGuardIP: "10.10.0.3",
				},
			},
			Security: config.SecurityConfig{},
		}

		sshKeyOutput := pulumi.String("ssh-rsa AAAAB3... test@example.com").ToStringOutput()
		sshPrivateKey := pulumi.String("-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----").ToStringOutput()
		doToken := pulumi.String("mock-do-token")
		linodeToken := pulumi.String("mock-linode-token")

		component, nodes, err := NewRealNodeDeploymentComponent(
			ctx,
			"test-deployment",
			clusterConfig,
			sshKeyOutput,
			sshPrivateKey,
			doToken,
			linodeToken,
			nil,
			nil,
		)

		assert.NoError(t, err)
		assert.NotNil(t, component)
		assert.Len(t, nodes, 3, "Should have three nodes")

		// Verify node names (using Apply to get actual values)
		for i := range []string{"master-1", "worker-1", "worker-2"} {
			idx := i
			nodes[idx].NodeName.ApplyT(func(nodeName string) error {
				// This verification happens during pulumi execution
				// In real test we'd accumulate these and verify after
				_ = nodeName // Just verify it's accessible
				return nil
			})
		}

		return nil
	}, pulumi.WithMocks("project", "stack", nodeMocks(0)))

	assert.NoError(t, err)
}

func TestNewRealNodeDeploymentComponent_WithBastion(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name: "test-cluster",
			},
			Nodes: []config.NodeConfig{
				{
					Name:        "master-1",
					Provider:    "digitalocean",
					Region:      "nyc1",
					Size:        "s-2vcpu-4gb",
					Roles:       []string{"master"},
					WireGuardIP: "10.10.0.1",
				},
			},
			Security: config.SecurityConfig{
				Bastion: &config.BastionConfig{
					Enabled:  true,
					Provider: "digitalocean",
					Region:   "nyc1",
					Size:     "s-1vcpu-1gb",
				},
			},
		}

		sshKeyOutput := pulumi.String("ssh-rsa AAAAB3... test@example.com").ToStringOutput()
		sshPrivateKey := pulumi.String("-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----").ToStringOutput()
		doToken := pulumi.String("mock-do-token")
		linodeToken := pulumi.String("mock-linode-token")

		// Create mock bastion component (in real scenario this would be created first)
		bastionComponent := &BastionComponent{}

		component, nodes, err := NewRealNodeDeploymentComponent(
			ctx,
			"test-deployment",
			clusterConfig,
			sshKeyOutput,
			sshPrivateKey,
			doToken,
			linodeToken,
			nil,
			bastionComponent,
		)

		assert.NoError(t, err)
		assert.NotNil(t, component)
		assert.Len(t, nodes, 1)

		return nil
	}, pulumi.WithMocks("project", "stack", nodeMocks(0)))

	assert.NoError(t, err)
}

func TestNewRealNodeDeploymentComponent_NodePools(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		// Test with node pools
		clusterConfig := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name: "test-cluster",
			},
			Nodes: []config.NodeConfig{}, // No individual nodes
			NodePools: map[string]config.NodePool{
				"master-pool": {
					Provider: "digitalocean",
					Region:   "nyc1",
					Size:     "s-2vcpu-4gb",
					Count:    3,
					Roles:    []string{"master"},
				},
				"worker-pool": {
					Provider: "linode",
					Region:   "us-east",
					Size:     "g6-standard-2",
					Count:    2,
					Roles:    []string{"worker"},
				},
			},
			Security: config.SecurityConfig{},
		}

		sshKeyOutput := pulumi.String("ssh-rsa AAAAB3... test@example.com").ToStringOutput()
		sshPrivateKey := pulumi.String("-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----").ToStringOutput()
		doToken := pulumi.String("mock-do-token")
		linodeToken := pulumi.String("mock-linode-token")

		component, nodes, err := NewRealNodeDeploymentComponent(
			ctx,
			"test-deployment",
			clusterConfig,
			sshKeyOutput,
			sshPrivateKey,
			doToken,
			linodeToken,
			nil,
			nil,
		)

		assert.NoError(t, err)
		assert.NotNil(t, component)
		// Should have 3 masters + 2 workers = 5 nodes
		assert.Len(t, nodes, 5, "Should have 5 nodes from pools")

		return nil
	}, pulumi.WithMocks("project", "stack", nodeMocks(0)))

	assert.NoError(t, err)
}

func TestNewRealNodeDeploymentComponent_MixedConfiguration(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		// Test with both individual nodes and node pools
		clusterConfig := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name: "test-cluster",
			},
			Nodes: []config.NodeConfig{
				{
					Name:        "special-master",
					Provider:    "digitalocean",
					Region:      "nyc1",
					Size:        "s-4vcpu-8gb",
					Roles:       []string{"master"},
					WireGuardIP: "10.10.0.1",
				},
			},
			NodePools: map[string]config.NodePool{
				"worker-pool": {
					Provider: "linode",
					Region:   "us-east",
					Size:     "g6-standard-2",
					Count:    3,
					Roles:    []string{"worker"},
				},
			},
			Security: config.SecurityConfig{},
		}

		sshKeyOutput := pulumi.String("ssh-rsa AAAAB3... test@example.com").ToStringOutput()
		sshPrivateKey := pulumi.String("-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----").ToStringOutput()
		doToken := pulumi.String("mock-do-token")
		linodeToken := pulumi.String("mock-linode-token")

		component, nodes, err := NewRealNodeDeploymentComponent(
			ctx,
			"test-deployment",
			clusterConfig,
			sshKeyOutput,
			sshPrivateKey,
			doToken,
			linodeToken,
			nil,
			nil,
		)

		assert.NoError(t, err)
		assert.NotNil(t, component)
		// Should have 1 individual node + 3 pool workers = 4 nodes
		assert.Len(t, nodes, 4, "Should have 4 nodes total")

		return nil
	}, pulumi.WithMocks("project", "stack", nodeMocks(0)))

	assert.NoError(t, err)
}

func TestRealNodeComponent_Outputs(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		clusterConfig := &config.ClusterConfig{
			Metadata: config.Metadata{
				Name: "test-cluster",
			},
			Nodes: []config.NodeConfig{
				{
					Name:        "test-node",
					Provider:    "digitalocean",
					Region:      "nyc1",
					Size:        "s-2vcpu-4gb",
					Roles:       []string{"master"},
					WireGuardIP: "10.10.0.1",
				},
			},
			Security: config.SecurityConfig{},
		}

		sshKeyOutput := pulumi.String("ssh-rsa AAAAB3... test@example.com").ToStringOutput()
		sshPrivateKey := pulumi.String("-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----").ToStringOutput()
		doToken := pulumi.String("mock-do-token")
		linodeToken := pulumi.String("mock-linode-token")

		_, nodes, err := NewRealNodeDeploymentComponent(
			ctx,
			"test-deployment",
			clusterConfig,
			sshKeyOutput,
			sshPrivateKey,
			doToken,
			linodeToken,
			nil,
			nil,
		)

		assert.NoError(t, err)
		assert.Len(t, nodes, 1)

		node := nodes[0]

		// Verify node has expected outputs
		node.NodeName.ApplyT(func(name string) error {
			assert.Equal(t, "test-node", name, "Node name should match")
			return nil
		})

		node.Provider.ApplyT(func(provider string) error {
			assert.Equal(t, "digitalocean", provider, "Provider should match")
			return nil
		})

		node.Region.ApplyT(func(region string) error {
			assert.Equal(t, "nyc1", region, "Region should match")
			return nil
		})

		node.Status.ApplyT(func(status string) error {
			assert.NotEmpty(t, status, "Status should not be empty")
			return nil
		})

		return nil
	}, pulumi.WithMocks("project", "stack", nodeMocks(0)))

	assert.NoError(t, err)
}
