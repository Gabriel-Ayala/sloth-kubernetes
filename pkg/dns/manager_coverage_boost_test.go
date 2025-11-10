package dns

import (
	"testing"

	"github.com/chalkan3/sloth-kubernetes/pkg/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

// TEST 1: Default node type (not master/worker) - COVERS LINES 74-78
func TestCreateNodeRecords_DefaultNodeType(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		manager := NewManager(ctx, "example.com")

		nodes := map[string][]*providers.NodeOutput{
			"digitalocean": {
				{
					Name:      "loadbalancer-1",
					PublicIP:  pulumi.String("203.0.113.30").ToStringOutput(),
					PrivateIP: pulumi.String("10.10.0.30").ToStringOutput(),
					Labels: map[string]string{
						"role": "loadbalancer", // Not master/worker - uses default case
					},
				},
			},
		}

		err := manager.CreateNodeRecords(nodes)
		assert.NoError(t, err)

		return nil
	}, pulumi.WithMocks("test-project", "test-stack", &DNSMocks{}))

	assert.NoError(t, err)
}

// TEST 2: Node with no labels (nil labels) - DEFAULT case
func TestCreateNodeRecords_NoLabelsDefaultCase(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		manager := NewManager(ctx, "example.com")

		nodes := map[string][]*providers.NodeOutput{
			"digitalocean": {
				{
					Name:      "generic-node-1",
					PublicIP:  pulumi.String("203.0.113.40").ToStringOutput(),
					PrivateIP: pulumi.String("10.10.0.40").ToStringOutput(),
					Labels:    nil, // No labels - will use default case
				},
			},
		}

		err := manager.CreateNodeRecords(nodes)
		assert.NoError(t, err)

		return nil
	}, pulumi.WithMocks("test-project", "test-stack", &DNSMocks{}))

	assert.NoError(t, err)
}

// TEST 3: Database node - another default case
func TestCreateNodeRecords_DatabaseNode(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		manager := NewManager(ctx, "example.com")

		nodes := map[string][]*providers.NodeOutput{
			"digitalocean": {
				{
					Name:      "postgres-1",
					PublicIP:  pulumi.String("203.0.113.50").ToStringOutput(),
					PrivateIP: pulumi.String("10.10.0.50").ToStringOutput(),
					Labels: map[string]string{
						"role": "database",
					},
				},
			},
		}

		err := manager.CreateNodeRecords(nodes)
		assert.NoError(t, err)

		return nil
	}, pulumi.WithMocks("test-project", "test-stack", &DNSMocks{}))

	assert.NoError(t, err)
}

// TEST 4: WireGuard DNS for worker nodes - COVERS LINES 108-111
func TestCreateNodeRecords_WireGuardWorker(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		manager := NewManager(ctx, "example.com")

		nodes := map[string][]*providers.NodeOutput{
			"digitalocean": {
				{
					Name:        "worker-1",
					PublicIP:    pulumi.String("203.0.113.20").ToStringOutput(),
					PrivateIP:   pulumi.String("10.10.0.20").ToStringOutput(),
					WireGuardIP: "10.8.0.2",
					Labels: map[string]string{
						"role": "worker",
					},
				},
			},
		}

		err := manager.CreateNodeRecords(nodes)
		assert.NoError(t, err)

		return nil
	}, pulumi.WithMocks("test-project", "test-stack", &DNSMocks{}))

	assert.NoError(t, err)
}

// TEST 5: WireGuard DNS for controlplane nodes - COVERS LINES 104-107
func TestCreateNodeRecords_WireGuardControlplane(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		manager := NewManager(ctx, "example.com")

		nodes := map[string][]*providers.NodeOutput{
			"digitalocean": {
				{
					Name:        "controlplane-1",
					PublicIP:    pulumi.String("203.0.113.10").ToStringOutput(),
					PrivateIP:   pulumi.String("10.10.0.10").ToStringOutput(),
					WireGuardIP: "10.8.0.1",
					Labels: map[string]string{
						"role": "controlplane",
					},
				},
			},
		}

		err := manager.CreateNodeRecords(nodes)
		assert.NoError(t, err)

		return nil
	}, pulumi.WithMocks("test-project", "test-stack", &DNSMocks{}))

	assert.NoError(t, err)
}

// TEST 6: Multiple masters with WireGuard - comprehensive coverage
func TestCreateNodeRecords_MultiMastersWithWireGuard(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		manager := NewManager(ctx, "example.com")

		nodes := map[string][]*providers.NodeOutput{
			"digitalocean": {
				{
					Name:        "master-1",
					PublicIP:    pulumi.String("203.0.113.10").ToStringOutput(),
					PrivateIP:   pulumi.String("10.10.0.10").ToStringOutput(),
					WireGuardIP: "10.8.0.1",
					Labels: map[string]string{
						"role": "master",
					},
				},
				{
					Name:        "master-2",
					PublicIP:    pulumi.String("203.0.113.11").ToStringOutput(),
					PrivateIP:   pulumi.String("10.10.0.11").ToStringOutput(),
					WireGuardIP: "10.8.0.2",
					Labels: map[string]string{
						"role": "master",
					},
				},
				{
					Name:        "master-3",
					PublicIP:    pulumi.String("203.0.113.12").ToStringOutput(),
					PrivateIP:   pulumi.String("10.10.0.12").ToStringOutput(),
					WireGuardIP: "10.8.0.3",
					Labels: map[string]string{
						"role": "master",
					},
				},
			},
		}

		err := manager.CreateNodeRecords(nodes)
		assert.NoError(t, err)

		return nil
	}, pulumi.WithMocks("test-project", "test-stack", &DNSMocks{}))

	assert.NoError(t, err)
}

// TEST 7: Multiple workers with WireGuard - comprehensive coverage
func TestCreateNodeRecords_MultiWorkersWithWireGuard(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		manager := NewManager(ctx, "example.com")

		nodes := map[string][]*providers.NodeOutput{
			"digitalocean": {
				{
					Name:        "worker-1",
					PublicIP:    pulumi.String("203.0.113.20").ToStringOutput(),
					PrivateIP:   pulumi.String("10.10.0.20").ToStringOutput(),
					WireGuardIP: "10.8.0.10",
					Labels: map[string]string{
						"role": "worker",
					},
				},
				{
					Name:        "worker-2",
					PublicIP:    pulumi.String("203.0.113.21").ToStringOutput(),
					PrivateIP:   pulumi.String("10.10.0.21").ToStringOutput(),
					WireGuardIP: "10.8.0.11",
					Labels: map[string]string{
						"role": "worker",
					},
				},
				{
					Name:        "worker-3",
					PublicIP:    pulumi.String("203.0.113.22").ToStringOutput(),
					PrivateIP:   pulumi.String("10.10.0.22").ToStringOutput(),
					WireGuardIP: "10.8.0.12",
					Labels: map[string]string{
						"role": "worker",
					},
				},
			},
		}

		err := manager.CreateNodeRecords(nodes)
		assert.NoError(t, err)

		return nil
	}, pulumi.WithMocks("test-project", "test-stack", &DNSMocks{}))

	assert.NoError(t, err)
}

// TEST 8: Mixed node types with default nodes
func TestCreateNodeRecords_MixedTypesIncludingDefault(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		manager := NewManager(ctx, "example.com")

		nodes := map[string][]*providers.NodeOutput{
			"digitalocean": {
				{
					Name:      "master-1",
					PublicIP:  pulumi.String("203.0.113.10").ToStringOutput(),
					PrivateIP: pulumi.String("10.10.0.10").ToStringOutput(),
					Labels: map[string]string{
						"role": "master",
					},
				},
				{
					Name:      "worker-1",
					PublicIP:  pulumi.String("203.0.113.20").ToStringOutput(),
					PrivateIP: pulumi.String("10.10.0.20").ToStringOutput(),
					Labels: map[string]string{
						"role": "worker",
					},
				},
				{
					Name:      "database-1",
					PublicIP:  pulumi.String("203.0.113.30").ToStringOutput(),
					PrivateIP: pulumi.String("10.10.0.30").ToStringOutput(),
					Labels: map[string]string{
						"role": "database", // Default case
					},
				},
				{
					Name:      "cache-1",
					PublicIP:  pulumi.String("203.0.113.40").ToStringOutput(),
					PrivateIP: pulumi.String("10.10.0.40").ToStringOutput(),
					Labels: map[string]string{
						"role": "cache", // Another default case
					},
				},
			},
		}

		err := manager.CreateNodeRecords(nodes)
		assert.NoError(t, err)

		return nil
	}, pulumi.WithMocks("test-project", "test-stack", &DNSMocks{}))

	assert.NoError(t, err)
}

// TEST 9: Wildcard record with only masters (no workers) - COVERS LINES 167-169
func TestCreateWildcardRecord_OnlyMasters(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		manager := NewManager(ctx, "example.com")

		// Add only master nodes (no workers)
		manager.nodes = []*providers.NodeOutput{
			{
				Name:      "master-1",
				PublicIP:  pulumi.String("203.0.113.10").ToStringOutput(),
				PrivateIP: pulumi.String("10.10.0.10").ToStringOutput(),
				Labels: map[string]string{
					"role": "master",
				},
			},
			{
				Name:      "master-2",
				PublicIP:  pulumi.String("203.0.113.11").ToStringOutput(),
				PrivateIP: pulumi.String("10.10.0.11").ToStringOutput(),
				Labels: map[string]string{
					"role": "master",
				},
			},
		}

		err := manager.createWildcardRecord()
		assert.NoError(t, err)

		return nil
	}, pulumi.WithMocks("test-project", "test-stack", &DNSMocks{}))

	assert.NoError(t, err)
}

// TEST 10: UpdateIngressRecord success path
func TestUpdateIngressRecord_Success(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		manager := NewManager(ctx, "example.com")

		ingressIP := pulumi.String("203.0.113.100").ToStringOutput()
		err := manager.UpdateIngressRecord(ingressIP)
		assert.NoError(t, err)

		return nil
	}, pulumi.WithMocks("test-project", "test-stack", &DNSMocks{}))

	assert.NoError(t, err)
}

// TEST 11: CreateClusterRecords success path
func TestCreateClusterRecords_Success(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		manager := NewManager(ctx, "example.com")

		err := manager.CreateClusterRecords()
		assert.NoError(t, err)

		return nil
	}, pulumi.WithMocks("test-project", "test-stack", &DNSMocks{}))

	assert.NoError(t, err)
}

// TEST 12: Node with empty role label string
func TestCreateNodeRecords_EmptyRoleLabel(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		manager := NewManager(ctx, "example.com")

		nodes := map[string][]*providers.NodeOutput{
			"digitalocean": {
				{
					Name:      "unknown-1",
					PublicIP:  pulumi.String("203.0.113.60").ToStringOutput(),
					PrivateIP: pulumi.String("10.10.0.60").ToStringOutput(),
					Labels: map[string]string{
						"role": "", // Empty role - uses default
					},
				},
			},
		}

		err := manager.CreateNodeRecords(nodes)
		assert.NoError(t, err)

		return nil
	}, pulumi.WithMocks("test-project", "test-stack", &DNSMocks{}))

	assert.NoError(t, err)
}

// TEST 13: Ingress and storage nodes (more default cases)
func TestCreateNodeRecords_IngressAndStorageNodes(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		manager := NewManager(ctx, "example.com")

		nodes := map[string][]*providers.NodeOutput{
			"digitalocean": {
				{
					Name:      "ingress-1",
					PublicIP:  pulumi.String("203.0.113.70").ToStringOutput(),
					PrivateIP: pulumi.String("10.10.0.70").ToStringOutput(),
					Labels: map[string]string{
						"role": "ingress",
					},
				},
				{
					Name:      "storage-1",
					PublicIP:  pulumi.String("203.0.113.80").ToStringOutput(),
					PrivateIP: pulumi.String("10.10.0.80").ToStringOutput(),
					Labels: map[string]string{
						"role": "storage",
					},
				},
			},
		}

		err := manager.CreateNodeRecords(nodes)
		assert.NoError(t, err)

		return nil
	}, pulumi.WithMocks("test-project", "test-stack", &DNSMocks{}))

	assert.NoError(t, err)
}

// TEST 14: WireGuard with default node type
func TestCreateNodeRecords_WireGuardWithDefaultType(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		manager := NewManager(ctx, "example.com")

		nodes := map[string][]*providers.NodeOutput{
			"digitalocean": {
				{
					Name:        "vpn-gateway-1",
					PublicIP:    pulumi.String("203.0.113.90").ToStringOutput(),
					PrivateIP:   pulumi.String("10.10.0.90").ToStringOutput(),
					WireGuardIP: "10.8.0.99",
					Labels: map[string]string{
						"role": "vpn-gateway", // Default type with WireGuard
					},
				},
			},
		}

		err := manager.CreateNodeRecords(nodes)
		assert.NoError(t, err)

		return nil
	}, pulumi.WithMocks("test-project", "test-stack", &DNSMocks{}))

	assert.NoError(t, err)
}

// TEST 15: Large cluster with many node types
func TestCreateNodeRecords_LargeClusterManyTypes(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		manager := NewManager(ctx, "example.com")

		nodes := map[string][]*providers.NodeOutput{
			"digitalocean": {
				// Masters
				{
					Name:        "master-1",
					PublicIP:    pulumi.String("203.0.113.1").ToStringOutput(),
					PrivateIP:   pulumi.String("10.10.0.1").ToStringOutput(),
					WireGuardIP: "10.8.0.1",
					Labels:      map[string]string{"role": "master"},
				},
				// Workers
				{
					Name:        "worker-1",
					PublicIP:    pulumi.String("203.0.113.10").ToStringOutput(),
					PrivateIP:   pulumi.String("10.10.0.10").ToStringOutput(),
					WireGuardIP: "10.8.0.10",
					Labels:      map[string]string{"role": "worker"},
				},
				{
					Name:        "worker-2",
					PublicIP:    pulumi.String("203.0.113.11").ToStringOutput(),
					PrivateIP:   pulumi.String("10.10.0.11").ToStringOutput(),
					WireGuardIP: "10.8.0.11",
					Labels:      map[string]string{"role": "worker"},
				},
				// Default types
				{
					Name:      "lb-1",
					PublicIP:  pulumi.String("203.0.113.20").ToStringOutput(),
					PrivateIP: pulumi.String("10.10.0.20").ToStringOutput(),
					Labels:    map[string]string{"role": "loadbalancer"},
				},
				{
					Name:      "db-1",
					PublicIP:  pulumi.String("203.0.113.30").ToStringOutput(),
					PrivateIP: pulumi.String("10.10.0.30").ToStringOutput(),
					Labels:    map[string]string{"role": "database"},
				},
				{
					Name:      "monitor-1",
					PublicIP:  pulumi.String("203.0.113.40").ToStringOutput(),
					PrivateIP: pulumi.String("10.10.0.40").ToStringOutput(),
					Labels:    map[string]string{"role": "monitoring"},
				},
			},
		}

		err := manager.CreateNodeRecords(nodes)
		assert.NoError(t, err)

		return nil
	}, pulumi.WithMocks("test-project", "test-stack", &DNSMocks{}))

	assert.NoError(t, err)
}
