package components

import (
	"fmt"
	"testing"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

// Massive test suite for Components - 100 tests

type componentMocks int

func (componentMocks) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	outputs := args.Inputs.Copy()

	switch args.TypeToken {
	case "kubernetes-create:network:VPC":
		outputs["vpcId"] = resource.NewStringProperty("vpc-" + args.Name)
		outputs["vpcName"] = resource.NewStringProperty(args.Name)
		outputs["region"] = resource.NewStringProperty("nyc1")
		outputs["ipRange"] = resource.NewStringProperty("10.0.0.0/16")
	case "digitalocean:index/vpc:Vpc":
		outputs["id"] = resource.NewStringProperty("vpc-mock-" + args.Name)
		outputs["name"] = args.Inputs["name"]
		outputs["region"] = args.Inputs["region"]
		outputs["ipRange"] = args.Inputs["ipRange"]
	case "kubernetes-create:security:Bastion":
		outputs["bastionName"] = resource.NewStringProperty("bastion")
		outputs["publicIP"] = resource.NewStringProperty("1.2.3.4")
		outputs["status"] = resource.NewStringProperty("active")
	case "digitalocean:index/droplet:Droplet", "linode:index/instance:Instance":
		outputs["id"] = resource.NewStringProperty("instance-" + args.Name)
		outputs["ipAddress"] = resource.NewStringProperty("1.2.3.4")
		outputs["status"] = resource.NewStringProperty("active")
	case "digitalocean:index/sshKey:SshKey":
		outputs["id"] = resource.NewStringProperty("ssh-" + args.Name)
		outputs["fingerprint"] = resource.NewStringProperty("ab:cd:ef:12")
	case "command:remote:Command":
		outputs["stdout"] = resource.NewStringProperty("success")
	}

	return args.Name + "_id", outputs, nil
}

func (componentMocks) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return resource.PropertyMap{}, nil
}

func TestVPCComponent_Regions(t *testing.T) {
	regions := []string{
		"nyc1", "nyc3", "sfo1", "sfo2", "sfo3",
		"ams2", "ams3", "sgp1", "lon1", "fra1",
		"tor1", "blr1",
	}

	for _, region := range regions {
		t.Run("Region_"+region, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				vpc, err := NewVPCComponent(ctx, "test-vpc", region, "10.0.0.0/16")
				assert.NoError(t, err)
				assert.NotNil(t, vpc)

				vpc.Region.ApplyT(func(r string) error {
					assert.Equal(t, region, r)
					return nil
				})

				return nil
			}, pulumi.WithMocks("project", "stack", componentMocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestVPCComponent_IPRanges(t *testing.T) {
	ipRanges := []string{
		"10.0.0.0/8", "10.0.0.0/12", "10.0.0.0/16", "10.0.0.0/20", "10.0.0.0/24",
		"172.16.0.0/12", "172.16.0.0/16", "172.16.0.0/20", "172.16.0.0/24",
		"192.168.0.0/16", "192.168.0.0/20", "192.168.0.0/24",
	}

	for _, ipRange := range ipRanges {
		t.Run("IPRange_"+ipRange, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				vpc, err := NewVPCComponent(ctx, "test-vpc", "nyc1", ipRange)
				assert.NoError(t, err)
				assert.NotNil(t, vpc)

				vpc.IPRange.ApplyT(func(ip string) error {
					assert.Equal(t, ipRange, ip)
					return nil
				})

				return nil
			}, pulumi.WithMocks("project", "stack", componentMocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestBastionComponent_SSHPorts(t *testing.T) {
	ports := []int{22, 2222, 2200, 22000, 50000, 60022}

	for _, port := range ports {
		t.Run(fmt.Sprintf("SSHPort_%d", port), func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				bastionConfig := &config.BastionConfig{
					Enabled:  true,
					Provider: "digitalocean",
					Region:   "nyc1",
					Size:     "s-1vcpu-1gb",
					SSHPort:  port,
				}

				sshKey := pulumi.String("ssh-rsa AAAAB3...").ToStringOutput()
				sshPrivateKey := pulumi.String("-----BEGIN...").ToStringOutput()

				component, err := NewBastionComponent(
					ctx,
					"test-bastion",
					bastionConfig,
					sshKey,
					sshPrivateKey,
					pulumi.String("token"),
					pulumi.String("token"),
				)

				assert.NoError(t, err)
				assert.NotNil(t, component)

				component.SSHPort.ApplyT(func(p int) error {
					assert.Equal(t, port, p)
					return nil
				})

				return nil
			}, pulumi.WithMocks("project", "stack", componentMocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestBastionComponent_Providers(t *testing.T) {
	providers := []string{"digitalocean", "linode", "azure"}

	for _, provider := range providers {
		t.Run("Provider_"+provider, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				bastionConfig := &config.BastionConfig{
					Enabled:  true,
					Provider: provider,
					Region:   "nyc1",
					Size:     "s-1vcpu-1gb",
				}

				sshKey := pulumi.String("ssh-rsa AAAAB3...").ToStringOutput()
				sshPrivateKey := pulumi.String("-----BEGIN...").ToStringOutput()

				component, err := NewBastionComponent(
					ctx,
					"test-bastion",
					bastionConfig,
					sshKey,
					sshPrivateKey,
					pulumi.String("token"),
					pulumi.String("token"),
				)

				assert.NoError(t, err)
				assert.NotNil(t, component)

				component.Provider.ApplyT(func(p string) error {
					assert.Equal(t, provider, p)
					return nil
				})

				return nil
			}, pulumi.WithMocks("project", "stack", componentMocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestBastionComponent_AllowedCIDRs(t *testing.T) {
	cidrSets := [][]string{
		{"0.0.0.0/0"},
		{"10.0.0.0/8"},
		{"192.168.1.0/24"},
		{"10.0.0.0/8", "172.16.0.0/12"},
		{"192.168.1.0/24", "192.168.2.0/24", "192.168.3.0/24"},
		{"10.0.0.0/16", "10.1.0.0/16", "10.2.0.0/16"},
	}

	for i, cidrs := range cidrSets {
		t.Run(fmt.Sprintf("CIDRs_%d", i), func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				bastionConfig := &config.BastionConfig{
					Enabled:      true,
					Provider:     "digitalocean",
					Region:       "nyc1",
					Size:         "s-1vcpu-1gb",
					AllowedCIDRs: cidrs,
				}

				sshKey := pulumi.String("ssh-rsa AAAAB3...").ToStringOutput()
				sshPrivateKey := pulumi.String("-----BEGIN...").ToStringOutput()

				component, err := NewBastionComponent(
					ctx,
					"test-bastion",
					bastionConfig,
					sshKey,
					sshPrivateKey,
					pulumi.String("token"),
					pulumi.String("token"),
				)

				assert.NoError(t, err)
				assert.NotNil(t, component)

				return nil
			}, pulumi.WithMocks("project", "stack", componentMocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestNodeDeployment_ClusterSizes(t *testing.T) {
	clusterSizes := []struct {
		name    string
		masters int
		workers int
	}{
		{"Single_Node", 1, 0},
		{"HA_3Masters", 3, 0},
		{"HA_5Masters", 5, 0},
		{"Small_1M_2W", 1, 2},
		{"Medium_3M_3W", 3, 3},
		{"Large_3M_10W", 3, 10},
		{"XLarge_5M_20W", 5, 20},
	}

	for _, tc := range clusterSizes {
		t.Run(tc.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				nodes := []config.NodeConfig{}

				// Add masters
				for i := 0; i < tc.masters; i++ {
					nodes = append(nodes, config.NodeConfig{
						Name:        fmt.Sprintf("master-%d", i+1),
						Provider:    "digitalocean",
						Region:      "nyc1",
						Size:        "s-2vcpu-4gb",
						Roles:       []string{"master"},
						WireGuardIP: fmt.Sprintf("10.10.0.%d", 10+i),
					})
				}

				// Add workers
				for i := 0; i < tc.workers; i++ {
					nodes = append(nodes, config.NodeConfig{
						Name:        fmt.Sprintf("worker-%d", i+1),
						Provider:    "digitalocean",
						Region:      "nyc1",
						Size:        "s-2vcpu-4gb",
						Roles:       []string{"worker"},
						WireGuardIP: fmt.Sprintf("10.10.0.%d", 20+i),
					})
				}

				clusterConfig := &config.ClusterConfig{
					Metadata: config.Metadata{
						Name: "test-cluster",
					},
					Nodes:    nodes,
					Security: config.SecurityConfig{},
				}

				sshKey := pulumi.String("ssh-rsa AAAAB3...").ToStringOutput()
				sshPrivateKey := pulumi.String("-----BEGIN...").ToStringOutput()

				component, deployedNodes, err := NewRealNodeDeploymentComponent(
					ctx,
					"test-deployment",
					clusterConfig,
					sshKey,
					sshPrivateKey,
					pulumi.String("token"),
					pulumi.String("token"),
					nil,
					nil,
				)

				assert.NoError(t, err)
				assert.NotNil(t, component)
				assert.Len(t, deployedNodes, tc.masters+tc.workers)

				return nil
			}, pulumi.WithMocks("project", "stack", nodeMocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestNodeDeployment_MultiCloud(t *testing.T) {
	providerCombos := []struct {
		name      string
		providers []string
	}{
		{"DO_Only", []string{"digitalocean", "digitalocean", "digitalocean"}},
		{"Linode_Only", []string{"linode", "linode", "linode"}},
		{"DO_Linode", []string{"digitalocean", "linode", "linode"}},
		{"Mixed_3", []string{"digitalocean", "linode", "digitalocean"}},
	}

	for _, tc := range providerCombos {
		t.Run(tc.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				nodes := []config.NodeConfig{}

				for i, provider := range tc.providers {
					region := "nyc1"
					if provider == "linode" {
						region = "us-east"
					}

					nodes = append(nodes, config.NodeConfig{
						Name:        fmt.Sprintf("node-%d", i+1),
						Provider:    provider,
						Region:      region,
						Size:        "s-2vcpu-4gb",
						Roles:       []string{"worker"},
						WireGuardIP: fmt.Sprintf("10.10.0.%d", 10+i),
					})
				}

				clusterConfig := &config.ClusterConfig{
					Metadata: config.Metadata{
						Name: "test-cluster",
					},
					Nodes:    nodes,
					Security: config.SecurityConfig{},
				}

				sshKey := pulumi.String("ssh-rsa AAAAB3...").ToStringOutput()
				sshPrivateKey := pulumi.String("-----BEGIN...").ToStringOutput()

				component, deployedNodes, err := NewRealNodeDeploymentComponent(
					ctx,
					"test-deployment",
					clusterConfig,
					sshKey,
					sshPrivateKey,
					pulumi.String("token"),
					pulumi.String("token"),
					nil,
					nil,
				)

				assert.NoError(t, err)
				assert.NotNil(t, component)
				assert.Len(t, deployedNodes, len(tc.providers))

				return nil
			}, pulumi.WithMocks("project", "stack", nodeMocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestVPCComponent_StackNames(t *testing.T) {
	stackNames := []string{
		"dev", "staging", "production", "test",
		"cluster-1", "cluster-2", "k8s-prod", "k8s-staging",
	}

	for _, stackName := range stackNames {
		t.Run("Stack_"+stackName, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				vpc, err := NewVPCComponent(ctx, "test-vpc", "nyc1", "10.0.0.0/16")
				assert.NoError(t, err)
				assert.NotNil(t, vpc)

				vpc.VPCName.ApplyT(func(name string) error {
					assert.Contains(t, name, stackName)
					return nil
				})

				return nil
			}, pulumi.WithMocks("project", stackName, componentMocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestBastionComponent_Sizes(t *testing.T) {
	bastionSizes := []struct {
		provider string
		size     string
	}{
		{"digitalocean", "s-1vcpu-1gb"},
		{"digitalocean", "s-1vcpu-2gb"},
		{"digitalocean", "s-2vcpu-2gb"},
		{"linode", "g6-nanode-1"},
		{"linode", "g6-standard-1"},
		{"linode", "g6-standard-2"},
		{"azure", "Standard_B1s"},
		{"azure", "Standard_B1ms"},
		{"azure", "Standard_B2s"},
	}

	for _, tc := range bastionSizes {
		t.Run(fmt.Sprintf("%s_%s", tc.provider, tc.size), func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				bastionConfig := &config.BastionConfig{
					Enabled:  true,
					Provider: tc.provider,
					Region:   "nyc1",
					Size:     tc.size,
				}

				sshKey := pulumi.String("ssh-rsa AAAAB3...").ToStringOutput()
				sshPrivateKey := pulumi.String("-----BEGIN...").ToStringOutput()

				component, err := NewBastionComponent(
					ctx,
					"test-bastion",
					bastionConfig,
					sshKey,
					sshPrivateKey,
					pulumi.String("token"),
					pulumi.String("token"),
				)

				assert.NoError(t, err)
				assert.NotNil(t, component)

				return nil
			}, pulumi.WithMocks("project", "stack", componentMocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestBastionComponent_IdleTimeouts(t *testing.T) {
	timeouts := []int{5, 10, 15, 30, 60, 120, 300}

	for _, timeout := range timeouts {
		t.Run(fmt.Sprintf("Timeout_%dmin", timeout), func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				bastionConfig := &config.BastionConfig{
					Enabled:     true,
					Provider:    "digitalocean",
					Region:      "nyc1",
					Size:        "s-1vcpu-1gb",
					IdleTimeout: timeout,
				}

				sshKey := pulumi.String("ssh-rsa AAAAB3...").ToStringOutput()
				sshPrivateKey := pulumi.String("-----BEGIN...").ToStringOutput()

				component, err := NewBastionComponent(
					ctx,
					"test-bastion",
					bastionConfig,
					sshKey,
					sshPrivateKey,
					pulumi.String("token"),
					pulumi.String("token"),
				)

				assert.NoError(t, err)
				assert.NotNil(t, component)

				return nil
			}, pulumi.WithMocks("project", "stack", componentMocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestBastionComponent_MaxSessions(t *testing.T) {
	maxSessions := []int{1, 5, 10, 20, 50, 100}

	for _, max := range maxSessions {
		t.Run(fmt.Sprintf("MaxSessions_%d", max), func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				bastionConfig := &config.BastionConfig{
					Enabled:     true,
					Provider:    "digitalocean",
					Region:      "nyc1",
					Size:        "s-1vcpu-1gb",
					MaxSessions: max,
				}

				sshKey := pulumi.String("ssh-rsa AAAAB3...").ToStringOutput()
				sshPrivateKey := pulumi.String("-----BEGIN...").ToStringOutput()

				component, err := NewBastionComponent(
					ctx,
					"test-bastion",
					bastionConfig,
					sshKey,
					sshPrivateKey,
					pulumi.String("token"),
					pulumi.String("token"),
				)

				assert.NoError(t, err)
				assert.NotNil(t, component)

				return nil
			}, pulumi.WithMocks("project", "stack", componentMocks(0)))

			assert.NoError(t, err)
		})
	}
}

func TestBastionComponent_Features(t *testing.T) {
	features := []struct {
		name     string
		vpnOnly  bool
		auditLog bool
		mfa      bool
	}{
		{"Basic", false, false, false},
		{"VPNOnly", true, false, false},
		{"AuditLog", false, true, false},
		{"MFA", false, false, true},
		{"VPN_Audit", true, true, false},
		{"VPN_MFA", true, false, true},
		{"Audit_MFA", false, true, true},
		{"All", true, true, true},
	}

	for _, tc := range features {
		t.Run(tc.name, func(t *testing.T) {
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				bastionConfig := &config.BastionConfig{
					Enabled:        true,
					Provider:       "digitalocean",
					Region:         "nyc1",
					Size:           "s-1vcpu-1gb",
					VPNOnly:        tc.vpnOnly,
					EnableAuditLog: tc.auditLog,
					EnableMFA:      tc.mfa,
				}

				sshKey := pulumi.String("ssh-rsa AAAAB3...").ToStringOutput()
				sshPrivateKey := pulumi.String("-----BEGIN...").ToStringOutput()

				component, err := NewBastionComponent(
					ctx,
					"test-bastion",
					bastionConfig,
					sshKey,
					sshPrivateKey,
					pulumi.String("token"),
					pulumi.String("token"),
				)

				assert.NoError(t, err)
				assert.NotNil(t, component)

				return nil
			}, pulumi.WithMocks("project", "stack", componentMocks(0)))

			assert.NoError(t, err)
		})
	}
}
