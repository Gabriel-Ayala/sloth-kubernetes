// Package config provides Lisp-based configuration loading
package config

import (
	"fmt"
)

// LoadFromLisp loads cluster configuration from a Lisp file
func LoadFromLisp(filePath string) (*ClusterConfig, error) {
	expr, err := ParseLispFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Lisp file: %w", err)
	}

	list, ok := expr.(*List)
	if !ok {
		return nil, fmt.Errorf("expected a list at root level")
	}

	head := list.Head()
	if head == nil || head.AsString() != "cluster" {
		return nil, fmt.Errorf("expected (cluster ...) at root level, got: %v", head)
	}

	cfg := &ClusterConfig{
		NodePools: make(map[string]NodePool),
	}

	// Parse each section
	for _, item := range list.Tail() {
		if section, ok := item.(*List); ok {
			if sectionHead := section.Head(); sectionHead != nil {
				switch sectionHead.AsString() {
				case "metadata":
					cfg.Metadata = parseMetadata(section)
				case "cluster":
					cfg.Cluster = parseClusterSpec(section)
				case "providers":
					cfg.Providers = parseProviders(section)
				case "network":
					cfg.Network = parseNetwork(section)
				case "security":
					cfg.Security = parseSecurity(section)
				case "nodes":
					cfg.Nodes = parseNodes(section)
				case "node-pools", "nodePools":
					cfg.NodePools = parseNodePools(section)
				case "kubernetes":
					cfg.Kubernetes = parseKubernetes(section)
				case "monitoring":
					cfg.Monitoring = parseMonitoring(section)
				case "storage":
					cfg.Storage = parseStorage(section)
				case "load-balancer", "loadBalancer":
					cfg.LoadBalancer = parseLoadBalancer(section)
				case "addons":
					cfg.Addons = parseAddons(section)
				}
			}
		}
	}

	// Apply defaults
	applyDefaults(cfg)

	return cfg, nil
}

func parseMetadata(l *List) Metadata {
	return Metadata{
		Name:        l.GetString("name"),
		Environment: l.GetString("environment"),
		Version:     l.GetString("version"),
		Description: l.GetString("description"),
		Owner:       l.GetString("owner"),
		Team:        l.GetString("team"),
		Labels:      l.GetMap("labels"),
		Annotations: l.GetMap("annotations"),
	}
}

func parseClusterSpec(l *List) ClusterSpec {
	return ClusterSpec{
		Type:             l.GetString("type"),
		Version:          l.GetString("version"),
		Distribution:     l.GetString("distribution"),
		HighAvailability: l.GetBool("high-availability"),
		MultiCloud:       l.GetBool("multi-cloud"),
	}
}

func parseProviders(l *List) ProvidersConfig {
	cfg := ProvidersConfig{}

	for _, item := range l.Tail() {
		if provider, ok := item.(*List); ok {
			if head := provider.Head(); head != nil {
				switch head.AsString() {
				case "digitalocean", "do":
					cfg.DigitalOcean = parseDigitalOceanProvider(provider)
				case "linode":
					cfg.Linode = parseLinodeProvider(provider)
				case "aws":
					cfg.AWS = parseAWSProvider(provider)
				case "azure":
					cfg.Azure = parseAzureProvider(provider)
				case "gcp":
					cfg.GCP = parseGCPProvider(provider)
				}
			}
		}
	}

	return cfg
}

func parseDigitalOceanProvider(l *List) *DigitalOceanProvider {
	return &DigitalOceanProvider{
		Enabled:    l.GetBool("enabled"),
		Token:      l.GetString("token"),
		Region:     l.GetString("region"),
		SSHKeys:    l.GetStringSlice("ssh-keys"),
		Tags:       l.GetStringSlice("tags"),
		Monitoring: l.GetBool("monitoring"),
		IPv6:       l.GetBool("ipv6"),
		VPC:        parseVPCConfig(l.GetList("vpc")),
	}
}

func parseLinodeProvider(l *List) *LinodeProvider {
	return &LinodeProvider{
		Enabled:        l.GetBool("enabled"),
		Token:          l.GetString("token"),
		Region:         l.GetString("region"),
		RootPassword:   l.GetString("root-password"),
		PrivateIP:      l.GetBool("private-ip"),
		AuthorizedKeys: l.GetStringSlice("authorized-keys"),
		Tags:           l.GetStringSlice("tags"),
		VPC:            parseVPCConfig(l.GetList("vpc")),
	}
}

func parseAWSProvider(l *List) *AWSProvider {
	return &AWSProvider{
		Enabled:         l.GetBool("enabled"),
		AccessKeyID:     l.GetString("access-key-id"),
		SecretAccessKey: l.GetString("secret-access-key"),
		Region:          l.GetString("region"),
		SecurityGroups:  l.GetStringSlice("security-groups"),
		KeyPair:         l.GetString("key-pair"),
		IAMRole:         l.GetString("iam-role"),
		VPC:             parseVPCConfig(l.GetList("vpc")),
	}
}

func parseAzureProvider(l *List) *AzureProvider {
	return &AzureProvider{
		Enabled:        l.GetBool("enabled"),
		SubscriptionID: l.GetString("subscription-id"),
		TenantID:       l.GetString("tenant-id"),
		ClientID:       l.GetString("client-id"),
		ClientSecret:   l.GetString("client-secret"),
		ResourceGroup:  l.GetString("resource-group"),
		Location:       l.GetString("location"),
	}
}

func parseGCPProvider(l *List) *GCPProvider {
	return &GCPProvider{
		Enabled:     l.GetBool("enabled"),
		ProjectID:   l.GetString("project-id"),
		Credentials: l.GetString("credentials"),
		Region:      l.GetString("region"),
		Zone:        l.GetString("zone"),
	}
}

func parseVPCConfig(l *List) *VPCConfig {
	if l == nil {
		return nil
	}
	return &VPCConfig{
		Create:            l.GetBool("create"),
		ID:                l.GetString("id"),
		Name:              l.GetString("name"),
		CIDR:              l.GetString("cidr"),
		Region:            l.GetString("region"),
		Private:           l.GetBool("private"),
		EnableDNS:         l.GetBool("enable-dns"),
		EnableDNSHostname: l.GetBool("enable-dns-hostname"),
		InternetGateway:   l.GetBool("internet-gateway"),
		NATGateway:        l.GetBool("nat-gateway"),
	}
}

func parseNetwork(l *List) NetworkConfig {
	cfg := NetworkConfig{
		Mode:        l.GetString("mode"),
		CIDR:        l.GetString("cidr"),
		PodCIDR:     l.GetString("pod-cidr"),
		ServiceCIDR: l.GetString("service-cidr"),
		DNSServers:  l.GetStringSlice("dns-servers"),
	}

	if wg := l.GetList("wireguard"); wg != nil {
		cfg.WireGuard = parseWireGuard(wg)
	}

	if dns := l.GetList("dns"); dns != nil {
		cfg.DNS = parseDNS(dns)
	}

	if fw := l.GetList("firewall"); fw != nil {
		cfg.Firewall = parseFirewall(fw)
	}

	return cfg
}

func parseWireGuard(l *List) *WireGuardConfig {
	return &WireGuardConfig{
		Enabled:             l.GetBool("enabled"),
		Create:              l.GetBool("create"),
		Provider:            l.GetString("provider"),
		Region:              l.GetString("region"),
		Size:                l.GetString("size"),
		Image:               l.GetString("image"),
		Name:                l.GetString("name"),
		ServerEndpoint:      l.GetString("server-endpoint"),
		ServerPublicKey:     l.GetString("server-public-key"),
		ClientIPBase:        l.GetString("client-ip-base"),
		Port:                l.GetInt("port"),
		MTU:                 l.GetInt("mtu"),
		PersistentKeepalive: l.GetInt("persistent-keepalive"),
		AutoConfig:          l.GetBool("auto-config"),
		MeshNetworking:      l.GetBool("mesh-networking"),
		SubnetCIDR:          l.GetString("subnet-cidr"),
	}
}

func parseDNS(l *List) DNSConfig {
	return DNSConfig{
		Domain:      l.GetString("domain"),
		Servers:     l.GetStringSlice("servers"),
		Searches:    l.GetStringSlice("searches"),
		ExternalDNS: l.GetBool("external-dns"),
		Provider:    l.GetString("provider"),
	}
}

func parseFirewall(l *List) *FirewallConfig {
	cfg := &FirewallConfig{
		Name:          l.GetString("name"),
		DefaultAction: l.GetString("default-action"),
	}

	if inbound := l.GetList("inbound-rules"); inbound != nil {
		cfg.InboundRules = parseFirewallRules(inbound)
	}
	if outbound := l.GetList("outbound-rules"); outbound != nil {
		cfg.OutboundRules = parseFirewallRules(outbound)
	}

	return cfg
}

func parseFirewallRules(l *List) []FirewallRule {
	var rules []FirewallRule
	for _, item := range l.Items {
		if rule, ok := item.(*List); ok {
			rules = append(rules, FirewallRule{
				Protocol:    rule.GetString("protocol"),
				Port:        rule.GetString("port"),
				Source:      rule.GetStringSlice("source"),
				Target:      rule.GetStringSlice("target"),
				Action:      rule.GetString("action"),
				Description: rule.GetString("description"),
			})
		}
	}
	return rules
}

func parseSecurity(l *List) SecurityConfig {
	cfg := SecurityConfig{}

	if ssh := l.GetList("ssh"); ssh != nil {
		cfg.SSHConfig = parseSSHConfig(ssh)
	}

	if bastion := l.GetList("bastion"); bastion != nil {
		cfg.Bastion = parseBastionConfig(bastion)
	}

	return cfg
}

func parseSSHConfig(l *List) SSHConfig {
	return SSHConfig{
		KeyPath:           l.GetString("key-path"),
		PublicKeyPath:     l.GetString("public-key-path"),
		AuthorizedKeys:    l.GetStringSlice("authorized-keys"),
		AllowPasswordAuth: l.GetBool("allow-password-auth"),
		Port:              l.GetInt("port"),
		AllowedUsers:      l.GetStringSlice("allowed-users"),
	}
}

func parseBastionConfig(l *List) *BastionConfig {
	return &BastionConfig{
		Enabled:        l.GetBool("enabled"),
		Provider:       l.GetString("provider"),
		Region:         l.GetString("region"),
		Size:           l.GetString("size"),
		Image:          l.GetString("image"),
		Name:           l.GetString("name"),
		VPNOnly:        l.GetBool("vpn-only"),
		AllowedCIDRs:   l.GetStringSlice("allowed-cidrs"),
		SSHPort:        l.GetInt("ssh-port"),
		IdleTimeout:    l.GetInt("idle-timeout"),
		MaxSessions:    l.GetInt("max-sessions"),
		EnableAuditLog: l.GetBool("enable-audit-log"),
		EnableMFA:      l.GetBool("enable-mfa"),
	}
}

func parseNodes(l *List) []NodeConfig {
	var nodes []NodeConfig
	for _, item := range l.Tail() {
		if node, ok := item.(*List); ok {
			nodes = append(nodes, NodeConfig{
				Name:         node.GetString("name"),
				Provider:     node.GetString("provider"),
				Pool:         node.GetString("pool"),
				Roles:        node.GetStringSlice("roles"),
				Size:         node.GetString("size"),
				Image:        node.GetString("image"),
				Region:       node.GetString("region"),
				Zone:         node.GetString("zone"),
				Labels:       node.GetMap("labels"),
				SpotInstance: node.GetBool("spot-instance"),
				SpotMaxPrice: node.GetString("spot-max-price"),
			})
		}
	}
	return nodes
}

func parseNodePools(l *List) map[string]NodePool {
	pools := make(map[string]NodePool)

	for _, item := range l.Tail() {
		if pool, ok := item.(*List); ok {
			if head := pool.Head(); head != nil {
				name := head.AsString()
				pools[name] = NodePool{
					Name:         pool.GetString("name"),
					Provider:     pool.GetString("provider"),
					Count:        pool.GetInt("count"),
					MinCount:     pool.GetInt("min-count"),
					MaxCount:     pool.GetInt("max-count"),
					Roles:        pool.GetStringSlice("roles"),
					Size:         pool.GetString("size"),
					Image:        pool.GetString("image"),
					Region:       pool.GetString("region"),
					Zones:        pool.GetStringSlice("zones"),
					Labels:       pool.GetMap("labels"),
					AutoScaling:  pool.GetBool("auto-scaling"),
					SpotInstance: pool.GetBool("spot-instance"),
					Preemptible:  pool.GetBool("preemptible"),
					UserData:     pool.GetString("user-data"),
				}
			}
		}
	}

	return pools
}

func parseKubernetes(l *List) KubernetesConfig {
	cfg := KubernetesConfig{
		Version:       l.GetString("version"),
		Distribution:  l.GetString("distribution"),
		NetworkPlugin: l.GetString("network-plugin"),
		PodCIDR:       l.GetString("pod-cidr"),
		ServiceCIDR:   l.GetString("service-cidr"),
		ClusterDNS:    l.GetString("cluster-dns"),
		ClusterDomain: l.GetString("cluster-domain"),
	}

	if rke2 := l.GetList("rke2"); rke2 != nil {
		cfg.RKE2 = parseRKE2Config(rke2)
	}

	return cfg
}

func parseRKE2Config(l *List) *RKE2Config {
	return &RKE2Config{
		Version:              l.GetString("version"),
		Channel:              l.GetString("channel"),
		ClusterToken:         l.GetString("cluster-token"),
		TLSSan:               l.GetStringSlice("tls-san"),
		DisableComponents:    l.GetStringSlice("disable-components"),
		DataDir:              l.GetString("data-dir"),
		SnapshotScheduleCron: l.GetString("snapshot-schedule-cron"),
		SnapshotRetention:    l.GetInt("snapshot-retention"),
		SeLinux:              l.GetBool("selinux"),
		SecretsEncryption:    l.GetBool("secrets-encryption"),
	}
}

func parseMonitoring(l *List) MonitoringConfig {
	cfg := MonitoringConfig{
		Enabled:  l.GetBool("enabled"),
		Provider: l.GetString("provider"),
	}

	if prom := l.GetList("prometheus"); prom != nil {
		cfg.Prometheus = &PrometheusConfig{
			Enabled:        prom.GetBool("enabled"),
			Retention:      prom.GetString("retention"),
			StorageSize:    prom.GetString("storage-size"),
			Replicas:       prom.GetInt("replicas"),
			ScrapeInterval: prom.GetString("scrape-interval"),
		}
	}

	if grafana := l.GetList("grafana"); grafana != nil {
		cfg.Grafana = &GrafanaConfig{
			Enabled:       grafana.GetBool("enabled"),
			AdminPassword: grafana.GetString("admin-password"),
			Ingress:       grafana.GetBool("ingress"),
			Domain:        grafana.GetString("domain"),
		}
	}

	return cfg
}

func parseStorage(l *List) StorageConfig {
	return StorageConfig{
		DefaultClass: l.GetString("default-class"),
	}
}

func parseLoadBalancer(l *List) LoadBalancerConfig {
	return LoadBalancerConfig{
		Name:     l.GetString("name"),
		Type:     l.GetString("type"),
		Provider: l.GetString("provider"),
	}
}

func parseAddons(l *List) AddonsConfig {
	cfg := AddonsConfig{}

	if argocd := l.GetList("argocd"); argocd != nil {
		cfg.ArgoCD = &ArgoCDConfig{
			Enabled:          argocd.GetBool("enabled"),
			Version:          argocd.GetString("version"),
			GitOpsRepoURL:    argocd.GetString("gitops-repo-url"),
			GitOpsRepoBranch: argocd.GetString("gitops-repo-branch"),
			AppsPath:         argocd.GetString("apps-path"),
			Namespace:        argocd.GetString("namespace"),
		}
	}

	return cfg
}

// LoadConfig loads configuration from a Lisp file
func LoadConfig(filePath string) (*ClusterConfig, error) {
	return LoadFromLisp(filePath)
}
