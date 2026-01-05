// Package config provides configuration validation for Lisp-based cluster configs
package config

import (
	"fmt"
	"net"
	"regexp"
	"strings"
)

// ValidationSeverity represents the severity of a validation issue
type ValidationSeverity string

const (
	// SeverityError represents a validation error that must be fixed
	SeverityError ValidationSeverity = "error"
	// SeverityWarning represents a validation warning that should be reviewed
	SeverityWarning ValidationSeverity = "warning"
	// SeverityInfo represents informational validation feedback
	SeverityInfo ValidationSeverity = "info"
)

// ValidationIssue represents a single validation issue
type ValidationIssue struct {
	Severity ValidationSeverity `json:"severity"`
	Path     string             `json:"path"`
	Field    string             `json:"field"`
	Message  string             `json:"message"`
	Value    interface{}        `json:"value,omitempty"`
	Hint     string             `json:"hint,omitempty"`
}

func (v ValidationIssue) String() string {
	prefix := "[ERROR]"
	switch v.Severity {
	case SeverityWarning:
		prefix = "[WARN]"
	case SeverityInfo:
		prefix = "[INFO]"
	}
	path := v.Path
	if v.Field != "" {
		path = path + "." + v.Field
	}
	result := fmt.Sprintf("%s %s: %s", prefix, path, v.Message)
	if v.Hint != "" {
		result += fmt.Sprintf(" (hint: %s)", v.Hint)
	}
	return result
}

// ValidationResult holds the result of configuration validation
type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Issues []ValidationIssue `json:"issues"`
}

// HasErrors returns true if there are any error-level issues
func (r *ValidationResult) HasErrors() bool {
	for _, issue := range r.Issues {
		if issue.Severity == SeverityError {
			return true
		}
	}
	return false
}

// HasWarnings returns true if there are any warning-level issues
func (r *ValidationResult) HasWarnings() bool {
	for _, issue := range r.Issues {
		if issue.Severity == SeverityWarning {
			return true
		}
	}
	return false
}

// Errors returns only error-level issues
func (r *ValidationResult) Errors() []ValidationIssue {
	var errors []ValidationIssue
	for _, issue := range r.Issues {
		if issue.Severity == SeverityError {
			errors = append(errors, issue)
		}
	}
	return errors
}

// Warnings returns only warning-level issues
func (r *ValidationResult) Warnings() []ValidationIssue {
	var warnings []ValidationIssue
	for _, issue := range r.Issues {
		if issue.Severity == SeverityWarning {
			warnings = append(warnings, issue)
		}
	}
	return warnings
}

// Summary returns a summary of the validation result
func (r *ValidationResult) Summary() string {
	errors := len(r.Errors())
	warnings := len(r.Warnings())
	if errors == 0 && warnings == 0 {
		return "Configuration is valid"
	}
	return fmt.Sprintf("Found %d error(s) and %d warning(s)", errors, warnings)
}

// ConfigValidator validates cluster configurations
type ConfigValidator struct {
	// StrictMode enables additional strict validation rules
	StrictMode bool
	// AllowedProviders is the list of allowed cloud providers
	AllowedProviders []string
	// AllowedDistributions is the list of allowed Kubernetes distributions
	AllowedDistributions []string
	// AllowedRegions is a map of provider to allowed regions (optional)
	AllowedRegions map[string][]string
	// CustomValidators allows adding custom validation functions
	CustomValidators []CustomValidator
}

// CustomValidator is a function that performs custom validation
type CustomValidator func(cfg *ClusterConfig) []ValidationIssue

// NewConfigValidator creates a new validator with default settings
func NewConfigValidator() *ConfigValidator {
	return &ConfigValidator{
		AllowedProviders:     []string{"aws", "gcp", "azure", "digitalocean", "linode", "hetzner"},
		AllowedDistributions: []string{"rke2", "k3s", "rke"},
		AllowedRegions:       make(map[string][]string),
		CustomValidators:     make([]CustomValidator, 0),
	}
}

// Validate validates a cluster configuration
func (v *ConfigValidator) Validate(cfg *ClusterConfig) *ValidationResult {
	result := &ValidationResult{
		Valid:  true,
		Issues: make([]ValidationIssue, 0),
	}

	// Validate each section
	v.validateMetadata(cfg, result)
	v.validateClusterSpec(cfg, result)
	v.validateProviders(cfg, result)
	v.validateNetwork(cfg, result)
	v.validateSecurity(cfg, result)
	v.validateNodes(cfg, result)
	v.validateNodePools(cfg, result)
	v.validateKubernetes(cfg, result)
	v.validateAddons(cfg, result)
	v.validateMonitoring(cfg, result)
	v.validateBackup(cfg, result)
	v.validateCostControl(cfg, result)

	// Cross-field validations
	v.validateCrossFields(cfg, result)

	// Run custom validators
	for _, validator := range v.CustomValidators {
		issues := validator(cfg)
		result.Issues = append(result.Issues, issues...)
	}

	// Set valid flag based on errors
	result.Valid = !result.HasErrors()

	return result
}

// addError adds an error-level issue
func (v *ConfigValidator) addError(result *ValidationResult, path, field, message string, value interface{}, hint string) {
	result.Issues = append(result.Issues, ValidationIssue{
		Severity: SeverityError,
		Path:     path,
		Field:    field,
		Message:  message,
		Value:    value,
		Hint:     hint,
	})
}

// addWarning adds a warning-level issue
func (v *ConfigValidator) addWarning(result *ValidationResult, path, field, message string, value interface{}, hint string) {
	result.Issues = append(result.Issues, ValidationIssue{
		Severity: SeverityWarning,
		Path:     path,
		Field:    field,
		Message:  message,
		Value:    value,
		Hint:     hint,
	})
}

// addInfo adds an info-level issue
func (v *ConfigValidator) addInfo(result *ValidationResult, path, field, message string, value interface{}, hint string) {
	result.Issues = append(result.Issues, ValidationIssue{
		Severity: SeverityInfo,
		Path:     path,
		Field:    field,
		Message:  message,
		Value:    value,
		Hint:     hint,
	})
}

// validateMetadata validates the metadata section
func (v *ConfigValidator) validateMetadata(cfg *ClusterConfig, result *ValidationResult) {
	path := "metadata"

	// Required: name
	if cfg.Metadata.Name == "" {
		v.addError(result, path, "name", "cluster name is required", nil, "add (name \"my-cluster\") to metadata section")
	} else {
		// Validate name format
		if !isValidKubernetesName(cfg.Metadata.Name) {
			v.addError(result, path, "name", "cluster name must be a valid DNS subdomain", cfg.Metadata.Name,
				"use lowercase letters, numbers, and hyphens; must start with letter")
		}
	}

	// Recommended: environment
	if cfg.Metadata.Environment == "" {
		v.addWarning(result, path, "environment", "environment is not set", nil,
			"add (environment \"production\") for better organization")
	} else {
		validEnvs := []string{"development", "staging", "production", "testing", "dev", "stg", "prod", "test"}
		if !sliceContains(validEnvs, strings.ToLower(cfg.Metadata.Environment)) {
			v.addInfo(result, path, "environment", "non-standard environment value", cfg.Metadata.Environment,
				"consider using: development, staging, production, testing")
		}
	}

	// Recommended: version
	if cfg.Metadata.Version == "" {
		v.addInfo(result, path, "version", "configuration version is not set", nil,
			"add (version \"1.0.0\") for versioning")
	}
}

// validateClusterSpec validates the cluster specification
func (v *ConfigValidator) validateClusterSpec(cfg *ClusterConfig, result *ValidationResult) {
	path := "cluster"

	// Validate distribution
	if cfg.Cluster.Distribution != "" {
		if !sliceContains(v.AllowedDistributions, cfg.Cluster.Distribution) {
			v.addError(result, path, "distribution", "unsupported Kubernetes distribution", cfg.Cluster.Distribution,
				fmt.Sprintf("use one of: %s", strings.Join(v.AllowedDistributions, ", ")))
		}
	}

	// Validate Kubernetes version format
	if cfg.Cluster.Version != "" {
		if !isValidKubernetesVersion(cfg.Cluster.Version) {
			v.addWarning(result, path, "version", "Kubernetes version format may be invalid", cfg.Cluster.Version,
				"use format: v1.28.0 or 1.28.0")
		}
	}

	// HA recommendation
	if cfg.Cluster.HighAvailability {
		v.addInfo(result, path, "high-availability", "HA mode enabled - ensure you have at least 3 control plane nodes", nil, "")
	}
}

// validateProviders validates provider configurations
func (v *ConfigValidator) validateProviders(cfg *ClusterConfig, result *ValidationResult) {
	path := "providers"
	hasEnabledProvider := false

	// AWS
	if cfg.Providers.AWS != nil && cfg.Providers.AWS.Enabled {
		hasEnabledProvider = true
		v.validateAWSProvider(cfg.Providers.AWS, result)
	}

	// GCP
	if cfg.Providers.GCP != nil && cfg.Providers.GCP.Enabled {
		hasEnabledProvider = true
		v.validateGCPProvider(cfg.Providers.GCP, result)
	}

	// Azure
	if cfg.Providers.Azure != nil && cfg.Providers.Azure.Enabled {
		hasEnabledProvider = true
		v.validateAzureProvider(cfg.Providers.Azure, result)
	}

	// DigitalOcean
	if cfg.Providers.DigitalOcean != nil && cfg.Providers.DigitalOcean.Enabled {
		hasEnabledProvider = true
		v.validateDigitalOceanProvider(cfg.Providers.DigitalOcean, result)
	}

	// Linode
	if cfg.Providers.Linode != nil && cfg.Providers.Linode.Enabled {
		hasEnabledProvider = true
		v.validateLinodeProvider(cfg.Providers.Linode, result)
	}

	if !hasEnabledProvider {
		v.addError(result, path, "", "at least one cloud provider must be enabled", nil,
			"enable a provider like (aws (enabled true) ...)")
	}
}

func (v *ConfigValidator) validateAWSProvider(p *AWSProvider, result *ValidationResult) {
	path := "providers.aws"

	if p.Region == "" {
		v.addError(result, path, "region", "AWS region is required", nil, "add (region \"us-east-1\")")
	} else if !isValidAWSRegion(p.Region) {
		v.addWarning(result, path, "region", "AWS region may be invalid", p.Region, "")
	}

	// Check for credentials (warning, not error - might use IAM roles)
	if p.AccessKeyID == "" && p.IAMRole == "" {
		v.addWarning(result, path, "access-key-id", "no AWS credentials or IAM role specified", nil,
			"provide credentials or ensure IAM role is configured")
	}

	// VPC validation
	if p.VPC != nil {
		if p.VPC.Create && p.VPC.ID != "" {
			v.addError(result, path+".vpc", "", "cannot both create VPC and specify existing VPC ID", nil,
				"set either (create true) or (id \"vpc-xxx\"), not both")
		}
		if p.VPC.CIDR != "" && !isValidCIDR(p.VPC.CIDR) {
			v.addError(result, path+".vpc", "cidr", "invalid CIDR format", p.VPC.CIDR,
				"use format: 10.0.0.0/16")
		}
	}
}

func (v *ConfigValidator) validateGCPProvider(p *GCPProvider, result *ValidationResult) {
	path := "providers.gcp"

	if p.ProjectID == "" {
		v.addError(result, path, "project-id", "GCP project ID is required", nil, "add (project-id \"my-project\")")
	}

	if p.Region == "" && p.Zone == "" {
		v.addError(result, path, "region", "GCP region or zone is required", nil, "add (region \"us-central1\")")
	}

	if p.Credentials == "" {
		v.addWarning(result, path, "credentials", "GCP credentials not specified", nil,
			"provide credentials file path or ensure GOOGLE_APPLICATION_CREDENTIALS is set")
	}
}

func (v *ConfigValidator) validateAzureProvider(p *AzureProvider, result *ValidationResult) {
	path := "providers.azure"

	if p.SubscriptionID == "" {
		v.addError(result, path, "subscription-id", "Azure subscription ID is required", nil, "")
	}

	if p.ResourceGroup == "" {
		v.addError(result, path, "resource-group", "Azure resource group is required", nil, "")
	}

	if p.Location == "" {
		v.addError(result, path, "location", "Azure location is required", nil, "add (location \"eastus\")")
	}

	// Validate authentication
	if p.ClientID == "" || p.ClientSecret == "" || p.TenantID == "" {
		v.addWarning(result, path, "", "Azure service principal credentials may be incomplete", nil,
			"provide client-id, client-secret, and tenant-id, or use managed identity")
	}
}

func (v *ConfigValidator) validateDigitalOceanProvider(p *DigitalOceanProvider, result *ValidationResult) {
	path := "providers.digitalocean"

	if p.Token == "" {
		v.addError(result, path, "token", "DigitalOcean API token is required", nil,
			"add (token (env \"DIGITALOCEAN_TOKEN\"))")
	}

	if p.Region == "" {
		v.addError(result, path, "region", "DigitalOcean region is required", nil, "add (region \"nyc3\")")
	}
}

func (v *ConfigValidator) validateLinodeProvider(p *LinodeProvider, result *ValidationResult) {
	path := "providers.linode"

	if p.Token == "" {
		v.addError(result, path, "token", "Linode API token is required", nil,
			"add (token (env \"LINODE_TOKEN\"))")
	}

	if p.Region == "" {
		v.addError(result, path, "region", "Linode region is required", nil, "add (region \"us-east\")")
	}
}

// validateNetwork validates network configuration
func (v *ConfigValidator) validateNetwork(cfg *ClusterConfig, result *ValidationResult) {
	path := "network"

	// Validate CIDRs
	if cfg.Network.PodCIDR != "" && !isValidCIDR(cfg.Network.PodCIDR) {
		v.addError(result, path, "pod-cidr", "invalid Pod CIDR format", cfg.Network.PodCIDR, "use format: 10.42.0.0/16")
	}

	if cfg.Network.ServiceCIDR != "" && !isValidCIDR(cfg.Network.ServiceCIDR) {
		v.addError(result, path, "service-cidr", "invalid Service CIDR format", cfg.Network.ServiceCIDR, "use format: 10.43.0.0/16")
	}

	// Check for CIDR overlap
	if cfg.Network.PodCIDR != "" && cfg.Network.ServiceCIDR != "" {
		if cidrsOverlap(cfg.Network.PodCIDR, cfg.Network.ServiceCIDR) {
			v.addError(result, path, "", "Pod CIDR and Service CIDR overlap", nil,
				"ensure Pod and Service CIDRs do not overlap")
		}
	}

	// WireGuard validation
	if cfg.Network.WireGuard != nil && cfg.Network.WireGuard.Enabled {
		v.validateWireGuard(cfg.Network.WireGuard, result)
	}

	// Firewall validation
	if cfg.Network.Firewall != nil {
		v.validateFirewall(cfg.Network.Firewall, result)
	}
}

func (v *ConfigValidator) validateWireGuard(wg *WireGuardConfig, result *ValidationResult) {
	path := "network.wireguard"

	if wg.Port != 0 && (wg.Port < 1 || wg.Port > 65535) {
		v.addError(result, path, "port", "invalid port number", wg.Port, "use port between 1-65535")
	}

	if wg.SubnetCIDR != "" && !isValidCIDR(wg.SubnetCIDR) {
		v.addError(result, path, "subnet-cidr", "invalid WireGuard subnet CIDR", wg.SubnetCIDR, "")
	}

	if wg.MTU != 0 && (wg.MTU < 576 || wg.MTU > 9000) {
		v.addWarning(result, path, "mtu", "MTU value may be suboptimal", wg.MTU, "typical values: 1280-1500")
	}
}

func (v *ConfigValidator) validateFirewall(fw *FirewallConfig, result *ValidationResult) {
	path := "network.firewall"

	// Validate inbound rules
	for i, rule := range fw.InboundRules {
		rulePath := fmt.Sprintf("%s.inbound-rules[%d]", path, i)
		v.validateFirewallRule(rule, rulePath, result)
	}

	// Validate outbound rules
	for i, rule := range fw.OutboundRules {
		rulePath := fmt.Sprintf("%s.outbound-rules[%d]", path, i)
		v.validateFirewallRule(rule, rulePath, result)
	}
}

func (v *ConfigValidator) validateFirewallRule(rule FirewallRule, path string, result *ValidationResult) {
	// Validate protocol
	validProtocols := []string{"tcp", "udp", "icmp", "all"}
	if rule.Protocol != "" && !sliceContains(validProtocols, strings.ToLower(rule.Protocol)) {
		v.addError(result, path, "protocol", "invalid protocol", rule.Protocol,
			fmt.Sprintf("use one of: %s", strings.Join(validProtocols, ", ")))
	}

	// Validate port
	if rule.Port != "" && !isValidPortSpec(rule.Port) {
		v.addWarning(result, path, "port", "port specification may be invalid", rule.Port,
			"use format: 80, 80-443, or all")
	}

	// Validate source CIDRs
	for _, source := range rule.Source {
		if source != "0.0.0.0/0" && !isValidCIDR(source) && !isValidIP(source) {
			v.addWarning(result, path, "source", "source may be invalid", source, "use CIDR or IP address")
		}
	}
}

// validateSecurity validates security configuration
func (v *ConfigValidator) validateSecurity(cfg *ClusterConfig, result *ValidationResult) {
	path := "security"

	// SSH validation
	if cfg.Security.SSHConfig.KeyPath != "" {
		// Warn about plaintext keys
		if strings.Contains(cfg.Security.SSHConfig.KeyPath, "-----BEGIN") {
			v.addWarning(result, path+".ssh", "key-path", "SSH key appears to be inline - consider using file path", nil,
				"use (key-path \"~/.ssh/id_rsa\") instead")
		}
	}

	if cfg.Security.SSHConfig.AllowPasswordAuth {
		v.addWarning(result, path+".ssh", "allow-password-auth", "password authentication is enabled - consider key-only", nil,
			"set (allow-password-auth false) for better security")
	}

	// Bastion validation
	if cfg.Security.Bastion != nil && cfg.Security.Bastion.Enabled {
		if len(cfg.Security.Bastion.AllowedCIDRs) == 0 {
			v.addWarning(result, path+".bastion", "allowed-cidrs", "bastion has no CIDR restrictions", nil,
				"add allowed-cidrs to restrict bastion access")
		}
	}
}

// validateNodes validates individual node configurations
func (v *ConfigValidator) validateNodes(cfg *ClusterConfig, result *ValidationResult) {
	path := "nodes"

	if len(cfg.Nodes) == 0 && len(cfg.NodePools) == 0 {
		v.addError(result, path, "", "no nodes or node pools defined", nil,
			"define nodes in (nodes ...) or node pools in (node-pools ...)")
		return
	}

	hasControlPlane := false
	nodeNames := make(map[string]bool)

	for i, node := range cfg.Nodes {
		nodePath := fmt.Sprintf("%s[%d]", path, i)

		// Check for duplicate names
		if node.Name != "" {
			if nodeNames[node.Name] {
				v.addError(result, nodePath, "name", "duplicate node name", node.Name, "each node must have unique name")
			}
			nodeNames[node.Name] = true
		} else {
			v.addError(result, nodePath, "name", "node name is required", nil, "")
		}

		// Check for control plane role
		for _, role := range node.Roles {
			if role == "controlplane" || role == "master" || role == "server" {
				hasControlPlane = true
			}
		}

		// Validate node provider
		if node.Provider == "" {
			v.addError(result, nodePath, "provider", "node provider is required", nil, "")
		}
	}

	// Check for control plane nodes (only if not using node pools)
	if len(cfg.NodePools) == 0 && !hasControlPlane {
		v.addError(result, path, "", "no control plane nodes defined", nil,
			"add at least one node with (roles (\"controlplane\"))")
	}
}

// validateNodePools validates node pool configurations
func (v *ConfigValidator) validateNodePools(cfg *ClusterConfig, result *ValidationResult) {
	if len(cfg.NodePools) == 0 {
		return
	}

	path := "node-pools"
	hasControlPlane := false

	for name, pool := range cfg.NodePools {
		poolPath := fmt.Sprintf("%s.%s", path, name)

		// Check for control plane role
		for _, role := range pool.Roles {
			if role == "controlplane" || role == "master" || role == "server" {
				hasControlPlane = true
			}
		}

		// Validate count
		if pool.Count < 0 {
			v.addError(result, poolPath, "count", "node count cannot be negative", pool.Count, "")
		}

		// Validate autoscaling configuration
		if pool.AutoScaling || pool.AutoScalingConfig != nil {
			if pool.AutoScalingConfig != nil {
				if pool.AutoScalingConfig.MinNodes > pool.AutoScalingConfig.MaxNodes {
					v.addError(result, poolPath+".autoscaling", "", "min-nodes cannot be greater than max-nodes",
						nil, "")
				}
				if pool.AutoScalingConfig.MaxNodes < pool.Count {
					v.addWarning(result, poolPath+".autoscaling", "max-nodes",
						"max-nodes is less than current count", nil, "")
				}
			}
		}

		// Validate spot instance configuration
		if pool.SpotInstance && pool.SpotConfig != nil {
			for _, role := range pool.Roles {
				if role == "controlplane" || role == "master" {
					v.addWarning(result, poolPath, "spot-instance",
						"using spot instances for control plane is risky", nil,
						"consider using on-demand for control plane nodes")
				}
			}
		}

		// Validate provider
		if pool.Provider == "" {
			v.addError(result, poolPath, "provider", "pool provider is required", nil, "")
		}
	}

	// Check for control plane
	if !hasControlPlane && len(cfg.Nodes) == 0 {
		v.addError(result, path, "", "no control plane node pool defined", nil,
			"add a pool with (roles (\"controlplane\"))")
	}
}

// validateKubernetes validates Kubernetes configuration
func (v *ConfigValidator) validateKubernetes(cfg *ClusterConfig, result *ValidationResult) {
	path := "kubernetes"

	// Validate version format
	if cfg.Kubernetes.Version != "" && !isValidKubernetesVersion(cfg.Kubernetes.Version) {
		v.addWarning(result, path, "version", "Kubernetes version format may be invalid",
			cfg.Kubernetes.Version, "use format: v1.28.0 or 1.28.0")
	}

	// Validate distribution
	if cfg.Kubernetes.Distribution != "" {
		if !sliceContains(v.AllowedDistributions, cfg.Kubernetes.Distribution) {
			v.addError(result, path, "distribution", "unsupported distribution",
				cfg.Kubernetes.Distribution, fmt.Sprintf("use one of: %s", strings.Join(v.AllowedDistributions, ", ")))
		}
	}

	// Validate network plugin
	validPlugins := []string{"calico", "canal", "flannel", "cilium", "weave", "none"}
	if cfg.Kubernetes.NetworkPlugin != "" && !sliceContains(validPlugins, cfg.Kubernetes.NetworkPlugin) {
		v.addWarning(result, path, "network-plugin", "network plugin may not be supported",
			cfg.Kubernetes.NetworkPlugin, fmt.Sprintf("common plugins: %s", strings.Join(validPlugins, ", ")))
	}

	// RKE2 specific validation
	if cfg.Kubernetes.RKE2 != nil {
		v.validateRKE2Config(cfg.Kubernetes.RKE2, result)
	}
}

func (v *ConfigValidator) validateRKE2Config(rke2 *RKE2Config, result *ValidationResult) {
	path := "kubernetes.rke2"

	// Validate channel
	validChannels := []string{"stable", "latest", "testing"}
	if rke2.Channel != "" && !sliceContains(validChannels, rke2.Channel) {
		v.addWarning(result, path, "channel", "RKE2 channel may be invalid", rke2.Channel,
			fmt.Sprintf("use one of: %s", strings.Join(validChannels, ", ")))
	}

	// Validate snapshot retention
	if rke2.SnapshotRetention < 0 {
		v.addError(result, path, "snapshot-retention", "snapshot retention cannot be negative", rke2.SnapshotRetention, "")
	}

	// Security recommendations
	if !rke2.SecretsEncryption {
		v.addInfo(result, path, "secrets-encryption", "secrets encryption is not enabled", nil,
			"consider enabling for production: (secrets-encryption true)")
	}
}

// validateAddons validates addon configurations
func (v *ConfigValidator) validateAddons(cfg *ClusterConfig, result *ValidationResult) {
	path := "addons"

	// ArgoCD validation
	if cfg.Addons.ArgoCD != nil && cfg.Addons.ArgoCD.Enabled {
		argoPath := path + ".argocd"
		if cfg.Addons.ArgoCD.GitOpsRepoURL == "" {
			v.addWarning(result, argoPath, "gitops-repo-url", "GitOps repo URL not set", nil,
				"add (gitops-repo-url \"https://github.com/...\")")
		} else if !isValidURL(cfg.Addons.ArgoCD.GitOpsRepoURL) {
			v.addError(result, argoPath, "gitops-repo-url", "invalid URL format", cfg.Addons.ArgoCD.GitOpsRepoURL, "")
		}
	}

	// Salt validation
	if cfg.Addons.Salt != nil && cfg.Addons.Salt.Enabled {
		saltPath := path + ".salt"
		if cfg.Addons.Salt.MasterNode == "" {
			v.addWarning(result, saltPath, "master-node", "Salt master node not specified", nil,
				"add (master-node \"node-name\") to designate Salt master")
		}

		// API security
		if cfg.Addons.Salt.APIEnabled && cfg.Addons.Salt.APIPassword == "" {
			v.addWarning(result, saltPath, "api-password", "Salt API password not set - will be auto-generated", nil,
				"consider setting explicit password for production")
		}
	}
}

// validateMonitoring validates monitoring configuration
func (v *ConfigValidator) validateMonitoring(cfg *ClusterConfig, result *ValidationResult) {
	if !cfg.Monitoring.Enabled {
		return
	}

	path := "monitoring"

	// Prometheus validation
	if cfg.Monitoring.Prometheus != nil && cfg.Monitoring.Prometheus.Enabled {
		promPath := path + ".prometheus"
		if cfg.Monitoring.Prometheus.StorageSize == "" {
			v.addWarning(result, promPath, "storage-size", "Prometheus storage size not set", nil,
				"add (storage-size \"50Gi\") to configure retention storage")
		}
	}

	// Grafana validation
	if cfg.Monitoring.Grafana != nil && cfg.Monitoring.Grafana.Enabled {
		grafanaPath := path + ".grafana"
		if cfg.Monitoring.Grafana.AdminPassword == "" {
			v.addWarning(result, grafanaPath, "admin-password", "Grafana admin password not set", nil,
				"set (admin-password (env \"GRAFANA_PASSWORD\")) for security")
		}
		if cfg.Monitoring.Grafana.Ingress && cfg.Monitoring.Grafana.Domain == "" {
			v.addError(result, grafanaPath, "domain", "domain required when ingress is enabled", nil,
				"add (domain \"grafana.example.com\")")
		}
	}
}

// validateBackup validates backup configuration
func (v *ConfigValidator) validateBackup(cfg *ClusterConfig, result *ValidationResult) {
	if cfg.Backup == nil || !cfg.Backup.Enabled {
		// Recommend backup for production
		if cfg.Metadata.Environment == "production" || cfg.Metadata.Environment == "prod" {
			v.addWarning(result, "backup", "enabled", "backup not enabled for production environment", nil,
				"consider enabling backup: (backup (enabled true) ...)")
		}
		return
	}

	path := "backup"

	// Validate schedule (cron format)
	if cfg.Backup.Schedule != "" && !isValidCronSchedule(cfg.Backup.Schedule) {
		v.addWarning(result, path, "schedule", "backup schedule may be invalid cron format",
			cfg.Backup.Schedule, "use format: \"0 2 * * *\" (daily at 2am)")
	}

	// Validate retention
	if cfg.Backup.Retention <= 0 && cfg.Backup.RetentionDays <= 0 {
		v.addWarning(result, path, "retention", "backup retention not set", nil,
			"add (retention 7) or (retention-days 30)")
	}

	// Storage validation
	if cfg.Backup.Storage != nil {
		storagePath := path + ".storage"
		validTypes := []string{"s3", "gcs", "azure-blob", "local"}
		if cfg.Backup.Storage.Type != "" && !sliceContains(validTypes, cfg.Backup.Storage.Type) {
			v.addWarning(result, storagePath, "type", "unknown storage type", cfg.Backup.Storage.Type,
				fmt.Sprintf("use one of: %s", strings.Join(validTypes, ", ")))
		}
		if cfg.Backup.Storage.Bucket == "" && cfg.Backup.Storage.Type != "local" {
			v.addError(result, storagePath, "bucket", "bucket is required for cloud storage", nil, "")
		}
	}
}

// validateCostControl validates cost control configuration
func (v *ConfigValidator) validateCostControl(cfg *ClusterConfig, result *ValidationResult) {
	if cfg.CostControl == nil {
		return
	}

	path := "cost-control"

	if cfg.CostControl.MonthlyBudget < 0 {
		v.addError(result, path, "monthly-budget", "monthly budget cannot be negative", cfg.CostControl.MonthlyBudget, "")
	}

	if cfg.CostControl.AlertThreshold < 0 || cfg.CostControl.AlertThreshold > 100 {
		v.addError(result, path, "alert-threshold", "alert threshold must be between 0 and 100",
			cfg.CostControl.AlertThreshold, "use percentage value like 80 for 80%")
	}

	if cfg.CostControl.AlertThreshold > 0 && cfg.CostControl.NotifyEmail == "" {
		v.addWarning(result, path, "notify", "cost alert threshold set but no notification email", nil,
			"add (notify \"team@example.com\")")
	}
}

// validateCrossFields performs cross-field validations
func (v *ConfigValidator) validateCrossFields(cfg *ClusterConfig, result *ValidationResult) {
	// HA validation
	if cfg.Cluster.HighAvailability {
		controlPlaneCount := 0
		for _, node := range cfg.Nodes {
			for _, role := range node.Roles {
				if role == "controlplane" || role == "master" || role == "server" {
					controlPlaneCount++
				}
			}
		}
		for _, pool := range cfg.NodePools {
			for _, role := range pool.Roles {
				if role == "controlplane" || role == "master" || role == "server" {
					controlPlaneCount += pool.Count
				}
			}
		}

		if controlPlaneCount < 3 {
			v.addWarning(result, "cluster", "high-availability",
				fmt.Sprintf("HA enabled but only %d control plane node(s) - recommend 3 or more", controlPlaneCount),
				nil, "add more control plane nodes for proper HA")
		}

		if controlPlaneCount%2 == 0 {
			v.addWarning(result, "cluster", "high-availability",
				fmt.Sprintf("even number of control plane nodes (%d) may cause split-brain", controlPlaneCount),
				nil, "use odd number (3, 5, 7) for etcd quorum")
		}
	}

	// Provider-region consistency
	for _, node := range cfg.Nodes {
		if node.Provider == "aws" && cfg.Providers.AWS != nil {
			if node.Region != "" && node.Region != cfg.Providers.AWS.Region {
				v.addWarning(result, "nodes", "region",
					fmt.Sprintf("node %s region differs from AWS provider region", node.Name),
					nil, "ensure multi-region is intentional")
			}
		}
	}

	// Node pool provider validation
	for name, pool := range cfg.NodePools {
		switch pool.Provider {
		case "aws":
			if cfg.Providers.AWS == nil || !cfg.Providers.AWS.Enabled {
				v.addError(result, fmt.Sprintf("node-pools.%s", name), "provider",
					"pool uses AWS but AWS provider is not enabled", nil, "")
			}
		case "gcp":
			if cfg.Providers.GCP == nil || !cfg.Providers.GCP.Enabled {
				v.addError(result, fmt.Sprintf("node-pools.%s", name), "provider",
					"pool uses GCP but GCP provider is not enabled", nil, "")
			}
		case "digitalocean", "do":
			if cfg.Providers.DigitalOcean == nil || !cfg.Providers.DigitalOcean.Enabled {
				v.addError(result, fmt.Sprintf("node-pools.%s", name), "provider",
					"pool uses DigitalOcean but DigitalOcean provider is not enabled", nil, "")
			}
		}
	}
}

// =============================================================================
// Helper validation functions
// =============================================================================

func isValidKubernetesName(name string) bool {
	// DNS subdomain name rules
	if len(name) > 253 {
		return false
	}
	pattern := `^[a-z]([-a-z0-9]*[a-z0-9])?$`
	matched, _ := regexp.MatchString(pattern, name)
	return matched
}

func isValidKubernetesVersion(version string) bool {
	// Match v1.28.0 or 1.28.0 format
	pattern := `^v?[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9]+)?$`
	matched, _ := regexp.MatchString(pattern, version)
	return matched
}

func isValidCIDR(cidr string) bool {
	_, _, err := net.ParseCIDR(cidr)
	return err == nil
}

func isValidIP(ip string) bool {
	return net.ParseIP(ip) != nil
}

func cidrsOverlap(cidr1, cidr2 string) bool {
	_, net1, err1 := net.ParseCIDR(cidr1)
	_, net2, err2 := net.ParseCIDR(cidr2)
	if err1 != nil || err2 != nil {
		return false
	}
	return net1.Contains(net2.IP) || net2.Contains(net1.IP)
}

func isValidPortSpec(port string) bool {
	if port == "all" || port == "*" {
		return true
	}
	// Single port or range
	pattern := `^[0-9]+(-[0-9]+)?$`
	matched, _ := regexp.MatchString(pattern, port)
	return matched
}

func isValidURL(url string) bool {
	pattern := `^(https?|git)://[^\s]+$`
	matched, _ := regexp.MatchString(pattern, url)
	return matched
}

func isValidAWSRegion(region string) bool {
	validRegions := []string{
		"us-east-1", "us-east-2", "us-west-1", "us-west-2",
		"eu-west-1", "eu-west-2", "eu-west-3", "eu-central-1", "eu-north-1",
		"ap-south-1", "ap-northeast-1", "ap-northeast-2", "ap-northeast-3",
		"ap-southeast-1", "ap-southeast-2",
		"sa-east-1", "ca-central-1", "me-south-1", "af-south-1",
	}
	return sliceContains(validRegions, region)
}

func isValidCronSchedule(schedule string) bool {
	// Basic cron validation (5 or 6 fields)
	parts := strings.Fields(schedule)
	return len(parts) >= 5 && len(parts) <= 6
}

func sliceContains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// ValidateConfig is a convenience function to validate a ClusterConfig
func ValidateConfig(cfg *ClusterConfig) *ValidationResult {
	validator := NewConfigValidator()
	return validator.Validate(cfg)
}

// ValidateLispFile validates a Lisp configuration file
func ValidateLispFile(filePath string) (*ValidationResult, error) {
	cfg, err := LoadFromLisp(filePath)
	if err != nil {
		return &ValidationResult{
			Valid: false,
			Issues: []ValidationIssue{{
				Severity: SeverityError,
				Path:     filePath,
				Message:  fmt.Sprintf("failed to parse Lisp file: %v", err),
			}},
		}, err
	}

	return ValidateConfig(cfg), nil
}
