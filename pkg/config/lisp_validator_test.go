package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConfigValidator(t *testing.T) {
	v := NewConfigValidator()
	assert.NotNil(t, v)
	assert.NotEmpty(t, v.AllowedProviders)
	assert.NotEmpty(t, v.AllowedDistributions)
}

func TestValidationResult_HasErrors(t *testing.T) {
	t.Run("no errors", func(t *testing.T) {
		result := &ValidationResult{
			Issues: []ValidationIssue{
				{Severity: SeverityWarning, Message: "warning"},
				{Severity: SeverityInfo, Message: "info"},
			},
		}
		assert.False(t, result.HasErrors())
	})

	t.Run("with errors", func(t *testing.T) {
		result := &ValidationResult{
			Issues: []ValidationIssue{
				{Severity: SeverityError, Message: "error"},
			},
		}
		assert.True(t, result.HasErrors())
	})
}

func TestValidationResult_HasWarnings(t *testing.T) {
	t.Run("no warnings", func(t *testing.T) {
		result := &ValidationResult{
			Issues: []ValidationIssue{
				{Severity: SeverityInfo, Message: "info"},
			},
		}
		assert.False(t, result.HasWarnings())
	})

	t.Run("with warnings", func(t *testing.T) {
		result := &ValidationResult{
			Issues: []ValidationIssue{
				{Severity: SeverityWarning, Message: "warning"},
			},
		}
		assert.True(t, result.HasWarnings())
	})
}

func TestValidationResult_Errors(t *testing.T) {
	result := &ValidationResult{
		Issues: []ValidationIssue{
			{Severity: SeverityError, Message: "error1"},
			{Severity: SeverityWarning, Message: "warning"},
			{Severity: SeverityError, Message: "error2"},
		},
	}
	errors := result.Errors()
	assert.Len(t, errors, 2)
}

func TestValidationResult_Warnings(t *testing.T) {
	result := &ValidationResult{
		Issues: []ValidationIssue{
			{Severity: SeverityError, Message: "error"},
			{Severity: SeverityWarning, Message: "warning1"},
			{Severity: SeverityWarning, Message: "warning2"},
		},
	}
	warnings := result.Warnings()
	assert.Len(t, warnings, 2)
}

func TestValidateMetadata(t *testing.T) {
	v := NewConfigValidator()

	t.Run("missing name", func(t *testing.T) {
		cfg := &ClusterConfig{
			Metadata: Metadata{},
		}
		result := v.Validate(cfg)
		assert.True(t, result.HasErrors())

		found := false
		for _, issue := range result.Issues {
			if issue.Field == "name" && issue.Severity == SeverityError {
				found = true
				break
			}
		}
		assert.True(t, found, "should have error for missing name")
	})

	t.Run("invalid name format", func(t *testing.T) {
		cfg := &ClusterConfig{
			Metadata: Metadata{
				Name: "INVALID_NAME_123",
			},
		}
		result := v.Validate(cfg)

		found := false
		for _, issue := range result.Issues {
			if issue.Field == "name" && issue.Message == "cluster name must be a valid DNS subdomain" {
				found = true
				break
			}
		}
		assert.True(t, found, "should have error for invalid name format")
	})

	t.Run("valid name", func(t *testing.T) {
		cfg := &ClusterConfig{
			Metadata: Metadata{
				Name:        "my-cluster",
				Environment: "production",
			},
			Providers: ProvidersConfig{
				AWS: &AWSProvider{
					Enabled: true,
					Region:  "us-east-1",
				},
			},
			Nodes: []NodeConfig{
				{Name: "node1", Provider: "aws", Roles: []string{"controlplane"}},
			},
		}
		result := v.Validate(cfg)

		// Should not have name-related errors
		for _, issue := range result.Errors() {
			if issue.Field == "name" {
				t.Errorf("should not have name error: %s", issue.Message)
			}
		}
	})
}

func TestValidateProviders(t *testing.T) {
	v := NewConfigValidator()

	t.Run("no provider enabled", func(t *testing.T) {
		cfg := &ClusterConfig{
			Metadata: Metadata{Name: "test"},
		}
		result := v.Validate(cfg)

		found := false
		for _, issue := range result.Issues {
			if issue.Path == "providers" && issue.Severity == SeverityError {
				found = true
				break
			}
		}
		assert.True(t, found, "should have error for no provider")
	})

	t.Run("AWS missing region", func(t *testing.T) {
		cfg := &ClusterConfig{
			Metadata: Metadata{Name: "test"},
			Providers: ProvidersConfig{
				AWS: &AWSProvider{Enabled: true},
			},
		}
		result := v.Validate(cfg)

		found := false
		for _, issue := range result.Issues {
			if issue.Path == "providers.aws" && issue.Field == "region" {
				found = true
				break
			}
		}
		assert.True(t, found, "should have error for missing AWS region")
	})

	t.Run("GCP missing project-id", func(t *testing.T) {
		cfg := &ClusterConfig{
			Metadata: Metadata{Name: "test"},
			Providers: ProvidersConfig{
				GCP: &GCPProvider{Enabled: true, Region: "us-central1"},
			},
		}
		result := v.Validate(cfg)

		found := false
		for _, issue := range result.Issues {
			if issue.Path == "providers.gcp" && issue.Field == "project-id" {
				found = true
				break
			}
		}
		assert.True(t, found, "should have error for missing GCP project-id")
	})

	t.Run("DigitalOcean missing token", func(t *testing.T) {
		cfg := &ClusterConfig{
			Metadata: Metadata{Name: "test"},
			Providers: ProvidersConfig{
				DigitalOcean: &DigitalOceanProvider{Enabled: true, Region: "nyc3"},
			},
		}
		result := v.Validate(cfg)

		found := false
		for _, issue := range result.Issues {
			if issue.Path == "providers.digitalocean" && issue.Field == "token" {
				found = true
				break
			}
		}
		assert.True(t, found, "should have error for missing DO token")
	})
}

func TestValidateNetwork(t *testing.T) {
	v := NewConfigValidator()

	t.Run("invalid pod CIDR", func(t *testing.T) {
		cfg := &ClusterConfig{
			Metadata: Metadata{Name: "test"},
			Providers: ProvidersConfig{
				AWS: &AWSProvider{Enabled: true, Region: "us-east-1"},
			},
			Network: NetworkConfig{
				PodCIDR: "invalid-cidr",
			},
			Nodes: []NodeConfig{
				{Name: "node1", Provider: "aws", Roles: []string{"controlplane"}},
			},
		}
		result := v.Validate(cfg)

		found := false
		for _, issue := range result.Issues {
			if issue.Field == "pod-cidr" && issue.Severity == SeverityError {
				found = true
				break
			}
		}
		assert.True(t, found, "should have error for invalid pod CIDR")
	})

	t.Run("valid CIDR", func(t *testing.T) {
		cfg := &ClusterConfig{
			Metadata: Metadata{Name: "test"},
			Providers: ProvidersConfig{
				AWS: &AWSProvider{Enabled: true, Region: "us-east-1"},
			},
			Network: NetworkConfig{
				PodCIDR:     "10.42.0.0/16",
				ServiceCIDR: "10.43.0.0/16",
			},
			Nodes: []NodeConfig{
				{Name: "node1", Provider: "aws", Roles: []string{"controlplane"}},
			},
		}
		result := v.Validate(cfg)

		// Should not have CIDR-related errors
		for _, issue := range result.Errors() {
			if issue.Field == "pod-cidr" || issue.Field == "service-cidr" {
				t.Errorf("should not have CIDR error: %s", issue.Message)
			}
		}
	})

	t.Run("overlapping CIDRs", func(t *testing.T) {
		cfg := &ClusterConfig{
			Metadata: Metadata{Name: "test"},
			Providers: ProvidersConfig{
				AWS: &AWSProvider{Enabled: true, Region: "us-east-1"},
			},
			Network: NetworkConfig{
				PodCIDR:     "10.0.0.0/8",
				ServiceCIDR: "10.43.0.0/16",
			},
			Nodes: []NodeConfig{
				{Name: "node1", Provider: "aws", Roles: []string{"controlplane"}},
			},
		}
		result := v.Validate(cfg)

		found := false
		for _, issue := range result.Issues {
			if issue.Message == "Pod CIDR and Service CIDR overlap" {
				found = true
				break
			}
		}
		assert.True(t, found, "should have error for overlapping CIDRs")
	})
}

func TestValidateNodes(t *testing.T) {
	v := NewConfigValidator()

	t.Run("no nodes or pools", func(t *testing.T) {
		cfg := &ClusterConfig{
			Metadata: Metadata{Name: "test"},
			Providers: ProvidersConfig{
				AWS: &AWSProvider{Enabled: true, Region: "us-east-1"},
			},
		}
		result := v.Validate(cfg)

		found := false
		for _, issue := range result.Issues {
			if issue.Path == "nodes" && issue.Message == "no nodes or node pools defined" {
				found = true
				break
			}
		}
		assert.True(t, found, "should have error for no nodes")
	})

	t.Run("no control plane", func(t *testing.T) {
		cfg := &ClusterConfig{
			Metadata: Metadata{Name: "test"},
			Providers: ProvidersConfig{
				AWS: &AWSProvider{Enabled: true, Region: "us-east-1"},
			},
			Nodes: []NodeConfig{
				{Name: "worker1", Provider: "aws", Roles: []string{"worker"}},
			},
		}
		result := v.Validate(cfg)

		found := false
		for _, issue := range result.Issues {
			if issue.Message == "no control plane nodes defined" {
				found = true
				break
			}
		}
		assert.True(t, found, "should have error for no control plane")
	})

	t.Run("duplicate node names", func(t *testing.T) {
		cfg := &ClusterConfig{
			Metadata: Metadata{Name: "test"},
			Providers: ProvidersConfig{
				AWS: &AWSProvider{Enabled: true, Region: "us-east-1"},
			},
			Nodes: []NodeConfig{
				{Name: "node1", Provider: "aws", Roles: []string{"controlplane"}},
				{Name: "node1", Provider: "aws", Roles: []string{"worker"}},
			},
		}
		result := v.Validate(cfg)

		found := false
		for _, issue := range result.Issues {
			if issue.Message == "duplicate node name" {
				found = true
				break
			}
		}
		assert.True(t, found, "should have error for duplicate names")
	})
}

func TestValidateKubernetes(t *testing.T) {
	v := NewConfigValidator()

	t.Run("invalid distribution", func(t *testing.T) {
		cfg := &ClusterConfig{
			Metadata: Metadata{Name: "test"},
			Providers: ProvidersConfig{
				AWS: &AWSProvider{Enabled: true, Region: "us-east-1"},
			},
			Kubernetes: KubernetesConfig{
				Distribution: "invalid-distro",
			},
			Nodes: []NodeConfig{
				{Name: "node1", Provider: "aws", Roles: []string{"controlplane"}},
			},
		}
		result := v.Validate(cfg)

		found := false
		for _, issue := range result.Issues {
			if issue.Field == "distribution" && issue.Severity == SeverityError {
				found = true
				break
			}
		}
		assert.True(t, found, "should have error for invalid distribution")
	})

	t.Run("valid distribution", func(t *testing.T) {
		cfg := &ClusterConfig{
			Metadata: Metadata{Name: "test"},
			Providers: ProvidersConfig{
				AWS: &AWSProvider{Enabled: true, Region: "us-east-1"},
			},
			Kubernetes: KubernetesConfig{
				Distribution: "rke2",
				Version:      "v1.28.0",
			},
			Nodes: []NodeConfig{
				{Name: "node1", Provider: "aws", Roles: []string{"controlplane"}},
			},
		}
		result := v.Validate(cfg)

		// Should not have distribution errors
		for _, issue := range result.Errors() {
			if issue.Field == "distribution" {
				t.Errorf("should not have distribution error: %s", issue.Message)
			}
		}
	})
}

func TestValidateNodePools(t *testing.T) {
	v := NewConfigValidator()

	t.Run("autoscaling min > max", func(t *testing.T) {
		cfg := &ClusterConfig{
			Metadata: Metadata{Name: "test"},
			Providers: ProvidersConfig{
				AWS: &AWSProvider{Enabled: true, Region: "us-east-1"},
			},
			NodePools: map[string]NodePool{
				"workers": {
					Provider:    "aws",
					Count:       3,
					Roles:       []string{"controlplane", "worker"},
					AutoScaling: true,
					AutoScalingConfig: &AutoScalingConfig{
						Enabled:  true,
						MinNodes: 10,
						MaxNodes: 5,
					},
				},
			},
		}
		result := v.Validate(cfg)

		found := false
		for _, issue := range result.Issues {
			if issue.Message == "min-nodes cannot be greater than max-nodes" {
				found = true
				break
			}
		}
		assert.True(t, found, "should have error for min > max")
	})

	t.Run("spot instance for control plane warning", func(t *testing.T) {
		cfg := &ClusterConfig{
			Metadata: Metadata{Name: "test"},
			Providers: ProvidersConfig{
				AWS: &AWSProvider{Enabled: true, Region: "us-east-1"},
			},
			NodePools: map[string]NodePool{
				"masters": {
					Provider:     "aws",
					Count:        3,
					Roles:        []string{"controlplane"},
					SpotInstance: true,
					SpotConfig:   &SpotConfig{Enabled: true},
				},
			},
		}
		result := v.Validate(cfg)

		found := false
		for _, issue := range result.Issues {
			if issue.Field == "spot-instance" && issue.Severity == SeverityWarning {
				found = true
				break
			}
		}
		assert.True(t, found, "should have warning for spot control plane")
	})
}

func TestValidateHighAvailability(t *testing.T) {
	v := NewConfigValidator()

	t.Run("HA with insufficient control planes", func(t *testing.T) {
		cfg := &ClusterConfig{
			Metadata: Metadata{Name: "test"},
			Cluster: ClusterSpec{
				HighAvailability: true,
			},
			Providers: ProvidersConfig{
				AWS: &AWSProvider{Enabled: true, Region: "us-east-1"},
			},
			Nodes: []NodeConfig{
				{Name: "master1", Provider: "aws", Roles: []string{"controlplane"}},
			},
		}
		result := v.Validate(cfg)

		found := false
		for _, issue := range result.Issues {
			if issue.Field == "high-availability" && issue.Severity == SeverityWarning {
				found = true
				break
			}
		}
		assert.True(t, found, "should have warning for insufficient HA nodes")
	})

	t.Run("HA with even number of control planes", func(t *testing.T) {
		cfg := &ClusterConfig{
			Metadata: Metadata{Name: "test"},
			Cluster: ClusterSpec{
				HighAvailability: true,
			},
			Providers: ProvidersConfig{
				AWS: &AWSProvider{Enabled: true, Region: "us-east-1"},
			},
			Nodes: []NodeConfig{
				{Name: "master1", Provider: "aws", Roles: []string{"controlplane"}},
				{Name: "master2", Provider: "aws", Roles: []string{"controlplane"}},
				{Name: "master3", Provider: "aws", Roles: []string{"controlplane"}},
				{Name: "master4", Provider: "aws", Roles: []string{"controlplane"}},
			},
		}
		result := v.Validate(cfg)

		found := false
		for _, issue := range result.Issues {
			if issue.Message == "even number of control plane nodes (4) may cause split-brain" {
				found = true
				break
			}
		}
		assert.True(t, found, "should have warning for even HA nodes")
	})
}

func TestValidateBackup(t *testing.T) {
	v := NewConfigValidator()

	t.Run("production without backup", func(t *testing.T) {
		cfg := &ClusterConfig{
			Metadata: Metadata{
				Name:        "test",
				Environment: "production",
			},
			Providers: ProvidersConfig{
				AWS: &AWSProvider{Enabled: true, Region: "us-east-1"},
			},
			Nodes: []NodeConfig{
				{Name: "node1", Provider: "aws", Roles: []string{"controlplane"}},
			},
		}
		result := v.Validate(cfg)

		found := false
		for _, issue := range result.Issues {
			if issue.Path == "backup" && issue.Severity == SeverityWarning {
				found = true
				break
			}
		}
		assert.True(t, found, "should have warning for production without backup")
	})

	t.Run("backup without retention", func(t *testing.T) {
		cfg := &ClusterConfig{
			Metadata: Metadata{Name: "test"},
			Providers: ProvidersConfig{
				AWS: &AWSProvider{Enabled: true, Region: "us-east-1"},
			},
			Backup: &BackupConfig{
				Enabled:  true,
				Schedule: "0 2 * * *",
			},
			Nodes: []NodeConfig{
				{Name: "node1", Provider: "aws", Roles: []string{"controlplane"}},
			},
		}
		result := v.Validate(cfg)

		found := false
		for _, issue := range result.Issues {
			if issue.Field == "retention" {
				found = true
				break
			}
		}
		assert.True(t, found, "should have warning for missing retention")
	})
}

func TestValidateCostControl(t *testing.T) {
	v := NewConfigValidator()

	t.Run("negative budget", func(t *testing.T) {
		cfg := &ClusterConfig{
			Metadata: Metadata{Name: "test"},
			Providers: ProvidersConfig{
				AWS: &AWSProvider{Enabled: true, Region: "us-east-1"},
			},
			CostControl: &CostControlConfig{
				MonthlyBudget: -100,
			},
			Nodes: []NodeConfig{
				{Name: "node1", Provider: "aws", Roles: []string{"controlplane"}},
			},
		}
		result := v.Validate(cfg)

		found := false
		for _, issue := range result.Issues {
			if issue.Field == "monthly-budget" && issue.Severity == SeverityError {
				found = true
				break
			}
		}
		assert.True(t, found, "should have error for negative budget")
	})

	t.Run("invalid alert threshold", func(t *testing.T) {
		cfg := &ClusterConfig{
			Metadata: Metadata{Name: "test"},
			Providers: ProvidersConfig{
				AWS: &AWSProvider{Enabled: true, Region: "us-east-1"},
			},
			CostControl: &CostControlConfig{
				AlertThreshold: 150,
			},
			Nodes: []NodeConfig{
				{Name: "node1", Provider: "aws", Roles: []string{"controlplane"}},
			},
		}
		result := v.Validate(cfg)

		found := false
		for _, issue := range result.Issues {
			if issue.Field == "alert-threshold" && issue.Severity == SeverityError {
				found = true
				break
			}
		}
		assert.True(t, found, "should have error for invalid threshold")
	})
}

func TestValidationIssue_String(t *testing.T) {
	t.Run("error with hint", func(t *testing.T) {
		issue := ValidationIssue{
			Severity: SeverityError,
			Path:     "metadata",
			Field:    "name",
			Message:  "name is required",
			Hint:     "add (name \"my-cluster\")",
		}
		str := issue.String()
		assert.Contains(t, str, "[ERROR]")
		assert.Contains(t, str, "metadata.name")
		assert.Contains(t, str, "name is required")
		assert.Contains(t, str, "(hint:")
	})

	t.Run("warning without hint", func(t *testing.T) {
		issue := ValidationIssue{
			Severity: SeverityWarning,
			Path:     "backup",
			Message:  "backup not enabled",
		}
		str := issue.String()
		assert.Contains(t, str, "[WARN]")
		assert.NotContains(t, str, "(hint:")
	})
}

func TestCustomValidator(t *testing.T) {
	v := NewConfigValidator()

	// Add custom validator
	v.CustomValidators = append(v.CustomValidators, func(cfg *ClusterConfig) []ValidationIssue {
		var issues []ValidationIssue
		if cfg.Metadata.Owner == "" {
			issues = append(issues, ValidationIssue{
				Severity: SeverityWarning,
				Path:     "metadata",
				Field:    "owner",
				Message:  "owner is not set",
			})
		}
		return issues
	})

	cfg := &ClusterConfig{
		Metadata: Metadata{Name: "test"},
		Providers: ProvidersConfig{
			AWS: &AWSProvider{Enabled: true, Region: "us-east-1"},
		},
		Nodes: []NodeConfig{
			{Name: "node1", Provider: "aws", Roles: []string{"controlplane"}},
		},
	}

	result := v.Validate(cfg)

	found := false
	for _, issue := range result.Issues {
		if issue.Field == "owner" && issue.Message == "owner is not set" {
			found = true
			break
		}
	}
	assert.True(t, found, "custom validator should add issue")
}

func TestValidateConfig_Convenience(t *testing.T) {
	cfg := &ClusterConfig{
		Metadata: Metadata{Name: "test-cluster"},
		Providers: ProvidersConfig{
			AWS: &AWSProvider{
				Enabled: true,
				Region:  "us-east-1",
			},
		},
		Nodes: []NodeConfig{
			{Name: "master1", Provider: "aws", Roles: []string{"controlplane"}},
			{Name: "master2", Provider: "aws", Roles: []string{"controlplane"}},
			{Name: "master3", Provider: "aws", Roles: []string{"controlplane"}},
		},
	}

	result := ValidateConfig(cfg)
	require.NotNil(t, result)
	assert.True(t, result.Valid)
}

func TestHelperFunctions(t *testing.T) {
	t.Run("isValidKubernetesName", func(t *testing.T) {
		assert.True(t, isValidKubernetesName("my-cluster"))
		assert.True(t, isValidKubernetesName("cluster1"))
		assert.False(t, isValidKubernetesName("My-Cluster"))
		assert.False(t, isValidKubernetesName("-cluster"))
		assert.False(t, isValidKubernetesName("cluster-"))
	})

	t.Run("isValidKubernetesVersion", func(t *testing.T) {
		assert.True(t, isValidKubernetesVersion("v1.28.0"))
		assert.True(t, isValidKubernetesVersion("1.28.0"))
		assert.True(t, isValidKubernetesVersion("v1.28.0-rc1"))
		assert.False(t, isValidKubernetesVersion("1.28"))
		assert.False(t, isValidKubernetesVersion("latest"))
	})

	t.Run("isValidCIDR", func(t *testing.T) {
		assert.True(t, isValidCIDR("10.0.0.0/8"))
		assert.True(t, isValidCIDR("192.168.1.0/24"))
		assert.False(t, isValidCIDR("invalid"))
		assert.False(t, isValidCIDR("10.0.0.0"))
	})

	t.Run("isValidIP", func(t *testing.T) {
		assert.True(t, isValidIP("192.168.1.1"))
		assert.True(t, isValidIP("10.0.0.1"))
		assert.False(t, isValidIP("invalid"))
		assert.False(t, isValidIP("256.1.1.1"))
	})

	t.Run("isValidPortSpec", func(t *testing.T) {
		assert.True(t, isValidPortSpec("80"))
		assert.True(t, isValidPortSpec("80-443"))
		assert.True(t, isValidPortSpec("all"))
		assert.False(t, isValidPortSpec("invalid"))
	})

	t.Run("isValidURL", func(t *testing.T) {
		assert.True(t, isValidURL("https://github.com/repo"))
		assert.True(t, isValidURL("http://example.com"))
		assert.True(t, isValidURL("git://github.com/repo"))
		assert.False(t, isValidURL("invalid"))
	})

	t.Run("cidrsOverlap", func(t *testing.T) {
		assert.True(t, cidrsOverlap("10.0.0.0/8", "10.42.0.0/16"))
		assert.False(t, cidrsOverlap("10.0.0.0/8", "192.168.0.0/16"))
	})
}
