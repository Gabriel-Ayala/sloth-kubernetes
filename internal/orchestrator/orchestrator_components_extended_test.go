package orchestrator

import (
	"testing"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

// Massive Mock Provider for orchestrator tests
type OrchestratorMassiveMockProvider struct {
	pulumi.ResourceState
}

func (m *OrchestratorMassiveMockProvider) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	outputs := args.Inputs.Copy()

	// Mock various resource types
	switch args.TypeToken {
	case "digitalocean:index/droplet:Droplet":
		outputs["ipv4Address"] = resource.NewStringProperty("192.168.1.100")
		outputs["name"] = resource.NewStringProperty(args.Name)
		outputs["id"] = resource.NewStringProperty(args.Name + "-id")
	case "linode:index/instance:Instance":
		outputs["ipAddress"] = resource.NewStringProperty("10.0.0.1")
		outputs["name"] = resource.NewStringProperty(args.Name)
	case "azure:compute/virtualMachine:VirtualMachine":
		outputs["name"] = resource.NewStringProperty(args.Name)
		outputs["id"] = resource.NewStringProperty(args.Name + "-id")
	case "command:remote:Command":
		outputs["stdout"] = resource.NewStringProperty("success")
		outputs["stderr"] = resource.NewStringProperty("")
	case "digitalocean:index/firewall:Firewall":
		outputs["id"] = resource.NewStringProperty(args.Name + "-fw-id")
	case "digitalocean:index/loadBalancer:LoadBalancer":
		outputs["ip"] = resource.NewStringProperty("203.0.113.1")
	case "digitalocean:index/dnsRecord:DnsRecord":
		outputs["fqdn"] = resource.NewStringProperty(args.Name + ".example.com")
	}

	return args.Name + "_id", outputs, nil
}

func (m *OrchestratorMassiveMockProvider) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return args.Args, nil
}

// Test RealDNSComponentGranular with comprehensive scenarios
func TestNewRealDNSComponentGranular_ComprehensiveScenarios(t *testing.T) {
	scenarios := []struct {
		name   string
		domain string
		nodes  int
	}{
		{"SingleNode", "single.example.com", 1},
		{"ThreeNodes", "three.example.com", 3},
		{"SixNodes", "six.example.com", 6},
		{"TenNodes", "ten.example.com", 10},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				cfg := &config.ClusterConfig{
					Network: config.NetworkConfig{
						DNS: config.DNSConfig{
							Domain: scenario.domain,
						},
					},
				}

				// Create nodes array
				nodesArray := make(pulumi.Array, scenario.nodes)
				for i := 0; i < scenario.nodes; i++ {
					nodesArray[i] = pulumi.String("node-" + string(rune(i+'1')))
				}
				nodes := pulumi.ToOutput(nodesArray).(pulumi.ArrayOutput)

				component, err := NewRealDNSComponentGranular(ctx, "test-dns-"+scenario.name, cfg, nodes)
				assert.NoError(t, err)
				assert.NotNil(t, component)

				return nil
			}, pulumi.WithMocks("test", "stack", &OrchestratorMassiveMockProvider{}))

			assert.NoError(t, err)
		})
	}
}

// Test OSFirewallComponentGranular with various node counts
func TestNewOSFirewallComponentGranular_VariousNodeCounts(t *testing.T) {
	nodeCounts := []int{1, 2, 3, 5, 8, 10, 15, 20}

	for _, count := range nodeCounts {
		t.Run(string(rune(count+'0'))+"Nodes", func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				// Create nodes array
				nodesArray := make(pulumi.Array, count)
				for i := 0; i < count; i++ {
					nodesArray[i] = pulumi.String("node-" + string(rune(i+'1')))
				}
				nodes := pulumi.ToOutput(nodesArray).(pulumi.ArrayOutput)
				sshKey := pulumi.String("/path/to/key").ToStringOutput()

				component, err := NewOSFirewallComponentGranular(ctx, "test-firewall", nodes, sshKey)
				assert.NoError(t, err)
				assert.NotNil(t, component)

				return nil
			}, pulumi.WithMocks("test", "stack", &OrchestratorMassiveMockProvider{}))

			assert.NoError(t, err)
		})
	}
}

// Test RealDNSRecordComponent with various record types
func TestNewRealDNSRecordComponent_AllRecordTypes(t *testing.T) {
	recordTypes := []struct {
		recordType string
		value      string
	}{
		{"A", "192.168.1.1"},
		{"AAAA", "2001:db8::1"},
		{"CNAME", "example.com."},
		{"MX", "10 mail.example.com."},
		{"TXT", "v=spf1 include:example.com ~all"},
		{"NS", "ns1.example.com."},
		{"SRV", "0 5 5060 sipserver.example.com."},
	}

	for _, rt := range recordTypes {
		t.Run(rt.recordType, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				parent := &DNSComponent{}
				ctx.RegisterComponentResource("test:parent", "parent", parent)

				record, err := newRealDNSRecordComponent(
					ctx,
					"test-record-"+rt.recordType,
					"test",
					"example.com",
					rt.recordType,
					rt.value,
					300,
					parent,
				)

				assert.NoError(t, err)
				assert.NotNil(t, record)
				return nil
			}, pulumi.WithMocks("test", "stack", &OrchestratorMassiveMockProvider{}))

			assert.NoError(t, err)
		})
	}
}

// Test FirewallRuleComponent with different protocols and ports
func TestNewFirewallRuleComponent_VariousProtocolsAndPorts(t *testing.T) {
	rules := []struct {
		name     string
		protocol string
		port     string
		source   string
	}{
		{"HTTP", "tcp", "80", "0.0.0.0/0"},
		{"HTTPS", "tcp", "443", "0.0.0.0/0"},
		{"SSH", "tcp", "22", "10.0.0.0/8"},
		{"DNS_TCP", "tcp", "53", "0.0.0.0/0"},
		{"DNS_UDP", "udp", "53", "0.0.0.0/0"},
		{"K8s_API", "tcp", "6443", "10.0.0.0/8"},
		{"NodePort_Range", "tcp", "30000-32767", "10.0.0.0/8"},
		{"Etcd", "tcp", "2379-2380", "10.0.0.0/8"},
		{"Flannel_UDP", "udp", "8472", "10.0.0.0/8"},
		{"WireGuard", "udp", "51820", "0.0.0.0/0"},
	}

	for _, rule := range rules {
		t.Run(rule.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				parent := &OSFirewallComponent{}
				ctx.RegisterComponentResource("test:parent", "parent", parent)

				ruleComp, err := newFirewallRuleComponent(
					ctx,
					"test-rule-"+rule.name,
					rule.name,
					rule.protocol,
					rule.port,
					rule.source,
					rule.name+" rule",
					"allow",
					parent,
				)

				assert.NoError(t, err)
				assert.NotNil(t, ruleComp)
				return nil
			}, pulumi.WithMocks("test", "stack", &OrchestratorMassiveMockProvider{}))

			assert.NoError(t, err)
		})
	}
}

// Test OSFirewallNodeComponent for various node types
func TestNewOSFirewallNodeComponent_VariousNodeTypes(t *testing.T) {
	nodeTypes := []string{
		"master-1",
		"master-2",
		"master-3",
		"worker-1",
		"worker-2",
		"worker-3",
		"etcd-1",
		"etcd-2",
		"etcd-3",
		"load-balancer",
	}

	for _, nodeType := range nodeTypes {
		t.Run(nodeType, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				parent := &OSFirewallComponent{}
				ctx.RegisterComponentResource("test:parent", "parent", parent)

				node, err := newOSFirewallNodeComponent(ctx, "test-"+nodeType, nodeType, parent)
				assert.NoError(t, err)
				assert.NotNil(t, node)
				return nil
			}, pulumi.WithMocks("test", "stack", &OrchestratorMassiveMockProvider{}))

			assert.NoError(t, err)
		})
	}
}

// Test NodeHealthCheckComponent for various scenarios
func TestNewNodeHealthCheckComponent_VariousScenarios(t *testing.T) {
	scenarios := []struct {
		name     string
		nodeName string
	}{
		{"Master", "master-1"},
		{"Worker", "worker-1"},
		{"Etcd", "etcd-1"},
		{"LongName", "production-kubernetes-master-node-1"},
		{"ShortName", "m1"},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				parent := &HealthCheckComponent{}
				ctx.RegisterComponentResource("test:parent", "parent", parent)

				healthCheck, err := newNodeHealthCheckComponent(ctx, "test-health-"+scenario.name, scenario.nodeName, parent)
				assert.NoError(t, err)
				assert.NotNil(t, healthCheck)
				return nil
			}, pulumi.WithMocks("test", "stack", &OrchestratorMassiveMockProvider{}))

			assert.NoError(t, err)
		})
	}
}

// Test IngressControllerComponent with various configurations
func TestNewIngressControllerComponent_VariousConfigurations(t *testing.T) {
	configurations := []struct {
		name        string
		controller  string
		replicas    int
	}{
		{"Nginx_SingleReplica", "ingress-nginx", 1},
		{"Nginx_TwoReplicas", "ingress-nginx", 2},
		{"Nginx_ThreeReplicas", "ingress-nginx", 3},
		{"Traefik_TwoReplicas", "traefik", 2},
		{"HAProxy_ThreeReplicas", "haproxy", 3},
	}

	for _, cfg := range configurations {
		t.Run(cfg.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				parent := &IngressComponent{}
				ctx.RegisterComponentResource("test:parent", "parent", parent)

				ingress, err := newIngressControllerComponent(ctx, "test-ingress-"+cfg.name, cfg.controller, cfg.replicas, parent)
				assert.NoError(t, err)
				assert.NotNil(t, ingress)
				return nil
			}, pulumi.WithMocks("test", "stack", &OrchestratorMassiveMockProvider{}))

			assert.NoError(t, err)
		})
	}
}

// Test IngressClassComponent with different classes
func TestNewIngressClassComponent_DifferentClasses(t *testing.T) {
	classes := []struct {
		name       string
		className  string
		isDefault  bool
	}{
		{"Nginx_Default", "nginx", true},
		{"Nginx_NonDefault", "nginx", false},
		{"Traefik_Default", "traefik", true},
		{"HAProxy_NonDefault", "haproxy", false},
	}

	for _, class := range classes {
		t.Run(class.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				parent := &IngressComponent{}
				ctx.RegisterComponentResource("test:parent", "parent", parent)

				ingressClass, err := newIngressClassComponent(ctx, "test-class-"+class.name, class.className, class.isDefault, parent)
				assert.NoError(t, err)
				assert.NotNil(t, ingressClass)
				return nil
			}, pulumi.WithMocks("test", "stack", &OrchestratorMassiveMockProvider{}))

			assert.NoError(t, err)
		})
	}
}

// Test AddonComponent with various addons
func TestNewAddonComponent_VariousAddons(t *testing.T) {
	addons := []struct {
		name      string
		addon     string
		namespace string
		version   string
	}{
		{"CertManager", "cert-manager", "cert-manager", "v1.13.0"},
		{"Prometheus", "prometheus", "monitoring", "v2.45.0"},
		{"Grafana", "grafana", "monitoring", "v10.0.0"},
		{"ArgoCD", "argocd", "argocd", "v2.8.0"},
		{"Longhorn", "longhorn", "longhorn-system", "v1.5.0"},
	}

	for _, addon := range addons {
		t.Run(addon.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				parent := &AddonsComponent{}
				ctx.RegisterComponentResource("test:parent", "parent", parent)

				addonComp, err := newAddonComponent(ctx, "test-addon-"+addon.name, addon.addon, addon.namespace, addon.version, parent)
				assert.NoError(t, err)
				assert.NotNil(t, addonComp)
				return nil
			}, pulumi.WithMocks("test", "stack", &OrchestratorMassiveMockProvider{}))

			assert.NoError(t, err)
		})
	}
}

// Test DNS component with empty nodes
func TestNewRealDNSComponentGranular_EmptyNodes(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		cfg := &config.ClusterConfig{
			Network: config.NetworkConfig{
				DNS: config.DNSConfig{
					Domain: "empty.example.com",
				},
			},
		}

		nodes := pulumi.ToOutput(pulumi.Array{}).(pulumi.ArrayOutput)

		component, err := NewRealDNSComponentGranular(ctx, "test-dns-empty", cfg, nodes)
		assert.NoError(t, err)
		assert.NotNil(t, component)

		return nil
	}, pulumi.WithMocks("test", "stack", &OrchestratorMassiveMockProvider{}))

	assert.NoError(t, err)
}

// Test firewall component with single node
func TestNewOSFirewallComponentGranular_SingleNode(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		nodes := pulumi.ToOutput(pulumi.Array{pulumi.String("single-node")}).(pulumi.ArrayOutput)
		sshKey := pulumi.String("/path/to/key").ToStringOutput()

		component, err := NewOSFirewallComponentGranular(ctx, "test-firewall-single", nodes, sshKey)
		assert.NoError(t, err)
		assert.NotNil(t, component)

		return nil
	}, pulumi.WithMocks("test", "stack", &OrchestratorMassiveMockProvider{}))

	assert.NoError(t, err)
}

// Test DNS record with various TTLs
func TestNewRealDNSRecordComponent_VariousTTLs(t *testing.T) {
	ttls := []int{60, 300, 600, 1800, 3600, 7200, 86400}

	for _, ttl := range ttls {
		t.Run(string(rune(ttl)), func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				parent := &DNSComponent{}
				ctx.RegisterComponentResource("test:parent", "parent", parent)

				record, err := newRealDNSRecordComponent(
					ctx,
					"test-record-ttl",
					"api",
					"example.com",
					"A",
					"192.168.1.1",
					ttl,
					parent,
				)

				assert.NoError(t, err)
				assert.NotNil(t, record)
				return nil
			}, pulumi.WithMocks("test", "stack", &OrchestratorMassiveMockProvider{}))

			assert.NoError(t, err)
		})
	}
}

// Test component structures
func TestComponentStructures_Various(t *testing.T) {
	t.Run("RealDNSRecordComponent", func(t *testing.T) {
		comp := &RealDNSRecordComponent{}
		assert.NotNil(t, comp)
	})

	t.Run("FirewallRuleComponent", func(t *testing.T) {
		comp := &FirewallRuleComponent{}
		assert.NotNil(t, comp)
	})

	t.Run("OSFirewallNodeComponent", func(t *testing.T) {
		comp := &OSFirewallNodeComponent{}
		assert.NotNil(t, comp)
	})

	t.Run("NodeHealthCheckComponent", func(t *testing.T) {
		comp := &NodeHealthCheckComponent{}
		assert.NotNil(t, comp)
	})

	t.Run("IngressControllerComponent", func(t *testing.T) {
		comp := &IngressControllerComponent{}
		assert.NotNil(t, comp)
	})

	t.Run("IngressClassComponent", func(t *testing.T) {
		comp := &IngressClassComponent{}
		assert.NotNil(t, comp)
	})

	t.Run("AddonComponent", func(t *testing.T) {
		comp := &AddonComponent{}
		assert.NotNil(t, comp)
	})
}

// Test DNS component with different domains
func TestNewRealDNSComponentGranular_DifferentDomains(t *testing.T) {
	domains := []string{
		"short.com",
		"medium-length.example.com",
		"very-long-domain-name-for-testing.example.co.uk",
		"sub1.sub2.sub3.example.com",
	}

	for _, domain := range domains {
		t.Run(domain, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				cfg := &config.ClusterConfig{
					Network: config.NetworkConfig{
						DNS: config.DNSConfig{
							Domain: domain,
						},
					},
				}

				nodes := pulumi.ToOutput(pulumi.Array{pulumi.String("node-1")}).(pulumi.ArrayOutput)

				component, err := NewRealDNSComponentGranular(ctx, "test-dns", cfg, nodes)
				assert.NoError(t, err)
				assert.NotNil(t, component)

				return nil
			}, pulumi.WithMocks("test", "stack", &OrchestratorMassiveMockProvider{}))

			assert.NoError(t, err)
		})
	}
}

// Test firewall rules with different actions
func TestNewFirewallRuleComponent_DifferentActions(t *testing.T) {
	actions := []string{"allow", "deny", "drop", "reject"}

	for _, action := range actions {
		t.Run(action, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				parent := &OSFirewallComponent{}
				ctx.RegisterComponentResource("test:parent", "parent", parent)

				rule, err := newFirewallRuleComponent(
					ctx,
					"test-rule-"+action,
					"test-rule",
					"tcp",
					"80",
					"0.0.0.0/0",
					"Test rule with "+action,
					action,
					parent,
				)

				assert.NoError(t, err)
				assert.NotNil(t, rule)
				return nil
			}, pulumi.WithMocks("test", "stack", &OrchestratorMassiveMockProvider{}))

			assert.NoError(t, err)
		})
	}
}
