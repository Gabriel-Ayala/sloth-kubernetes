package orchestrator

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/chalkan3/sloth-kubernetes/pkg/versioning"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==================== sanitizeConfigForStorage Tests ====================

func TestSanitizeConfigForStorage_AllProvidersAtOnce(t *testing.T) {
	cfg := &config.ClusterConfig{
		Metadata: config.Metadata{
			Name:        "production-cluster",
			Environment: "prod",
			Version:     "2.1.0",
		},
		Providers: config.ProvidersConfig{
			DigitalOcean: &config.DigitalOceanProvider{
				Enabled: true,
				Token:   "dop_v1_real_token_abc123",
				Region:  "nyc3",
			},
			Linode: &config.LinodeProvider{
				Enabled:      true,
				Token:        "linode_pat_real_token",
				RootPassword: "P@ssw0rd!Secure",
				Region:       "us-east",
			},
			AWS: &config.AWSProvider{
				Enabled:         true,
				AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
				SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCY",
				Region:          "us-west-2",
			},
			Azure: &config.AzureProvider{
				Enabled:        true,
				ClientID:       "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
				ClientSecret:   "azure~secret~value",
				TenantID:       "t1e2n3a4-n5t6-7890-abcd-ef1234567890",
				SubscriptionID: "s1u2b3s4-c5r6-7890-abcd-ef1234567890",
			},
			GCP: &config.GCPProvider{
				Enabled:     true,
				Credentials: `{"type":"service_account","private_key":"-----BEGIN RSA PRIVATE KEY-----\nMIIEo..."}`,
				ProjectID:   "my-gcp-project",
			},
		},
		Network: config.NetworkConfig{
			CIDR:        "10.0.0.0/16",
			PodCIDR:     "10.244.0.0/16",
			ServiceCIDR: "10.96.0.0/12",
			DNS: config.DNSConfig{
				Domain: "prod.example.com",
			},
		},
		Nodes: []config.NodeConfig{
			{Name: "master-1", Provider: "digitalocean", Region: "nyc3"},
			{Name: "worker-1", Provider: "linode", Region: "us-east"},
		},
	}

	result := sanitizeConfigForStorage(cfg)

	// All tokens sanitized
	assert.Equal(t, "${DIGITALOCEAN_TOKEN}", result.Providers.DigitalOcean.Token)
	assert.Equal(t, "${LINODE_TOKEN}", result.Providers.Linode.Token)
	assert.Equal(t, "${LINODE_ROOT_PASSWORD}", result.Providers.Linode.RootPassword)
	assert.Equal(t, "${AWS_ACCESS_KEY_ID}", result.Providers.AWS.AccessKeyID)
	assert.Equal(t, "${AWS_SECRET_ACCESS_KEY}", result.Providers.AWS.SecretAccessKey)
	assert.Equal(t, "${AZURE_CLIENT_ID}", result.Providers.Azure.ClientID)
	assert.Equal(t, "${AZURE_CLIENT_SECRET}", result.Providers.Azure.ClientSecret)
	assert.Equal(t, "${AZURE_TENANT_ID}", result.Providers.Azure.TenantID)
	assert.Equal(t, "${AZURE_SUBSCRIPTION_ID}", result.Providers.Azure.SubscriptionID)
	assert.Equal(t, "${GCP_CREDENTIALS}", result.Providers.GCP.Credentials)

	// Non-sensitive fields preserved exactly
	assert.Equal(t, "production-cluster", result.Metadata.Name)
	assert.Equal(t, "prod", result.Metadata.Environment)
	assert.Equal(t, "2.1.0", result.Metadata.Version)
	assert.Equal(t, "nyc3", result.Providers.DigitalOcean.Region)
	assert.Equal(t, "us-east", result.Providers.Linode.Region)
	assert.Equal(t, "us-west-2", result.Providers.AWS.Region)
	assert.Equal(t, "my-gcp-project", result.Providers.GCP.ProjectID)
	assert.Equal(t, "10.0.0.0/16", result.Network.CIDR)
	assert.Equal(t, "prod.example.com", result.Network.DNS.Domain)
	assert.True(t, result.Providers.DigitalOcean.Enabled)
	assert.True(t, result.Providers.Linode.Enabled)

	// Nodes preserved
	require.Len(t, result.Nodes, 2)
	assert.Equal(t, "master-1", result.Nodes[0].Name)
	assert.Equal(t, "worker-1", result.Nodes[1].Name)
}

func TestSanitizeConfigForStorage_OriginalConfigNeverModified(t *testing.T) {
	originalDOToken := "dop_v1_real_do_token"
	originalLinodeToken := "linode_real_token"
	originalLinodePwd := "linode_root_pwd"
	originalAWSKey := "AKIAIOSFODNN7REAL"
	originalAWSSecret := "real_aws_secret_key"
	originalAzureClient := "azure-real-client"
	originalAzureSecret := "azure-real-secret"
	originalAzureTenant := "azure-real-tenant"
	originalAzureSub := "azure-real-sub"
	originalGCPCreds := `{"real":"credentials"}`

	cfg := &config.ClusterConfig{
		Providers: config.ProvidersConfig{
			DigitalOcean: &config.DigitalOceanProvider{Token: originalDOToken},
			Linode:       &config.LinodeProvider{Token: originalLinodeToken, RootPassword: originalLinodePwd},
			AWS:          &config.AWSProvider{AccessKeyID: originalAWSKey, SecretAccessKey: originalAWSSecret},
			Azure:        &config.AzureProvider{ClientID: originalAzureClient, ClientSecret: originalAzureSecret, TenantID: originalAzureTenant, SubscriptionID: originalAzureSub},
			GCP:          &config.GCPProvider{Credentials: originalGCPCreds},
		},
	}

	_ = sanitizeConfigForStorage(cfg)

	// Original must be completely untouched
	assert.Equal(t, originalDOToken, cfg.Providers.DigitalOcean.Token)
	assert.Equal(t, originalLinodeToken, cfg.Providers.Linode.Token)
	assert.Equal(t, originalLinodePwd, cfg.Providers.Linode.RootPassword)
	assert.Equal(t, originalAWSKey, cfg.Providers.AWS.AccessKeyID)
	assert.Equal(t, originalAWSSecret, cfg.Providers.AWS.SecretAccessKey)
	assert.Equal(t, originalAzureClient, cfg.Providers.Azure.ClientID)
	assert.Equal(t, originalAzureSecret, cfg.Providers.Azure.ClientSecret)
	assert.Equal(t, originalAzureTenant, cfg.Providers.Azure.TenantID)
	assert.Equal(t, originalAzureSub, cfg.Providers.Azure.SubscriptionID)
	assert.Equal(t, originalGCPCreds, cfg.Providers.GCP.Credentials)
}

func TestSanitizeConfigForStorage_NilProviders_NoPanic(t *testing.T) {
	cfg := &config.ClusterConfig{
		Metadata:  config.Metadata{Name: "safe-cluster"},
		Providers: config.ProvidersConfig{},
	}

	// Must not panic when all provider fields are nil
	result := sanitizeConfigForStorage(cfg)

	assert.NotNil(t, result)
	assert.Equal(t, "safe-cluster", result.Metadata.Name)
	assert.Nil(t, result.Providers.DigitalOcean)
	assert.Nil(t, result.Providers.Linode)
	assert.Nil(t, result.Providers.AWS)
	assert.Nil(t, result.Providers.Azure)
	assert.Nil(t, result.Providers.GCP)
}

func TestSanitizeConfigForStorage_ResultIsDeepCopy(t *testing.T) {
	cfg := &config.ClusterConfig{
		Metadata: config.Metadata{Name: "original"},
		Providers: config.ProvidersConfig{
			DigitalOcean: &config.DigitalOceanProvider{
				Token:  "secret",
				Region: "nyc3",
			},
		},
	}

	result := sanitizeConfigForStorage(cfg)

	// Mutate the result - should NOT affect original
	result.Metadata.Name = "mutated"
	result.Providers.DigitalOcean.Region = "sfo1"

	assert.Equal(t, "original", cfg.Metadata.Name)
	assert.Equal(t, "nyc3", cfg.Providers.DigitalOcean.Region)
}

func TestSanitizeConfigForStorage_SanitizedOutputIsValidJSON(t *testing.T) {
	cfg := &config.ClusterConfig{
		Metadata: config.Metadata{Name: "json-test"},
		Providers: config.ProvidersConfig{
			DigitalOcean: &config.DigitalOceanProvider{Token: "secret-token"},
			AWS:          &config.AWSProvider{AccessKeyID: "AKIA...", SecretAccessKey: "secret"},
		},
	}

	result := sanitizeConfigForStorage(cfg)

	// The sanitized config must be JSON-serializable
	data, err := json.Marshal(result)
	require.NoError(t, err)

	// Verify the JSON doesn't contain the original secret
	assert.NotContains(t, string(data), "secret-token")
	assert.Contains(t, string(data), "${DIGITALOCEAN_TOKEN}")
}

func TestSanitizeConfigForStorage_WithNodePoolsAndKubernetes(t *testing.T) {
	cfg := &config.ClusterConfig{
		Metadata: config.Metadata{Name: "full-cluster"},
		Providers: config.ProvidersConfig{
			DigitalOcean: &config.DigitalOceanProvider{Token: "token123"},
		},
		NodePools: map[string]config.NodePool{
			"masters": {Name: "masters", Count: 3, Provider: "digitalocean", Roles: []string{"master"}},
			"workers": {Name: "workers", Count: 5, Provider: "digitalocean", Roles: []string{"worker"}},
		},
		Kubernetes: config.KubernetesConfig{
			Distribution:  "rke2",
			Version:       "v1.28.0",
			NetworkPlugin: "calico",
		},
	}

	result := sanitizeConfigForStorage(cfg)

	// NodePools preserved
	require.Len(t, result.NodePools, 2)
	assert.Equal(t, 3, result.NodePools["masters"].Count)
	assert.Equal(t, 5, result.NodePools["workers"].Count)
	assert.Equal(t, []string{"master"}, result.NodePools["masters"].Roles)

	// Kubernetes config preserved
	assert.Equal(t, "rke2", result.Kubernetes.Distribution)
	assert.Equal(t, "v1.28.0", result.Kubernetes.Version)
}

// ==================== generateSHA256Checksum Tests ====================

func TestGenerateSHA256Checksum_KnownValues(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		{"hello", "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"},
		{"hello world", "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("input=%q", tt.input), func(t *testing.T) {
			result := generateSHA256Checksum(tt.input)
			assert.Equal(t, tt.expected, result)
			assert.Len(t, result, 64)
		})
	}
}

func TestGenerateSHA256Checksum_AlwaysProduces64HexChars(t *testing.T) {
	inputs := []string{
		"",
		"a",
		"short",
		strings.Repeat("x", 10000),
		"special chars: !@#$%^&*()\n\t",
		"\x00\x01\x02binary",
	}

	for _, input := range inputs {
		result := generateSHA256Checksum(input)
		assert.Len(t, result, 64, "input: %q", input)
		// Verify it's valid hex
		for _, c := range result {
			assert.True(t, (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'),
				"non-hex char %c in result for input %q", c, input)
		}
	}
}

func TestGenerateSHA256Checksum_Deterministic_MultipleRuns(t *testing.T) {
	input := "kubernetes manifest content with YAML:\napiVersion: v1\nkind: Pod"
	first := generateSHA256Checksum(input)
	for i := 0; i < 100; i++ {
		assert.Equal(t, first, generateSHA256Checksum(input))
	}
}

func TestGenerateSHA256Checksum_SingleBitDifference(t *testing.T) {
	// Even a single character difference should produce completely different hashes
	hash1 := generateSHA256Checksum("test input A")
	hash2 := generateSHA256Checksum("test input B")

	assert.NotEqual(t, hash1, hash2)

	// Count differing characters - should be many (avalanche effect)
	diffs := 0
	for i := range hash1 {
		if hash1[i] != hash2[i] {
			diffs++
		}
	}
	assert.Greater(t, diffs, 20, "SHA256 avalanche effect: expected many different chars")
}

// ==================== generateChecksum Tests ====================

func TestGenerateChecksum_Deterministic(t *testing.T) {
	input := "test content for checksum"
	cs1 := generateChecksum(input)
	cs2 := generateChecksum(input)
	assert.Equal(t, cs1, cs2)
}

func TestGenerateChecksum_DifferentInputs(t *testing.T) {
	inputs := []string{"alpha", "beta", "gamma", "delta"}
	checksums := make(map[string]bool)

	for _, input := range inputs {
		cs := generateChecksum(input)
		assert.False(t, checksums[cs], "collision detected for input %q", input)
		checksums[cs] = true
	}
}

func TestGenerateChecksum_EmptyInput(t *testing.T) {
	cs := generateChecksum("")
	assert.Equal(t, "0", cs)
}

func TestGenerateChecksum_PositionSensitive(t *testing.T) {
	// The checksum algorithm includes position (i) in the calculation
	cs1 := generateChecksum("ab")
	cs2 := generateChecksum("ba")
	assert.NotEqual(t, cs1, cs2, "checksum should be position-sensitive")
}

// ==================== detectPoolChanges Tests ====================

func TestDetectPoolChanges_AlwaysReturnsNil(t *testing.T) {
	// Current implementation is a stub that always returns nil
	tests := []struct {
		name string
		cfg  *config.ClusterConfig
		prev *DeploymentMetadata
	}{
		{
			name: "with pools",
			cfg: &config.ClusterConfig{
				NodePools: map[string]config.NodePool{
					"new-pool": {Count: 3},
				},
			},
			prev: &DeploymentMetadata{CurrentNodeCount: 5},
		},
		{
			name: "empty pools",
			cfg:  &config.ClusterConfig{},
			prev: &DeploymentMetadata{},
		},
		{
			name: "multiple pools",
			cfg: &config.ClusterConfig{
				NodePools: map[string]config.NodePool{
					"masters": {Count: 3},
					"workers": {Count: 10},
					"gpu":     {Count: 2},
				},
			},
			prev: &DeploymentMetadata{
				CurrentNodeCount: 8,
				NodePoolsAdded:   []string{"masters", "workers"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			added, removed, scaled := detectPoolChanges(tt.cfg, tt.prev)
			assert.Nil(t, added)
			assert.Nil(t, removed)
			assert.Nil(t, scaled)
		})
	}
}

// ==================== generateDeploymentMetadata Tests ====================

func TestGenerateDeploymentMetadata_InitialDeployment_CompleteFieldVerification(t *testing.T) {
	cfg := &config.ClusterConfig{
		Metadata:  config.Metadata{Name: "prod-cluster"},
		NodePools: map[string]config.NodePool{},
	}

	manifest := "apiVersion: v1\nkind: ConfigMap"
	meta := generateDeploymentMetadata(cfg, "", 3, manifest)

	// Timestamps
	assert.NotEmpty(t, meta.CreatedAt)
	assert.NotEmpty(t, meta.UpdatedAt)
	assert.NotEmpty(t, meta.LastDeployedAt)
	assert.Equal(t, meta.CreatedAt, meta.UpdatedAt, "initial deployment: CreatedAt == UpdatedAt")
	assert.Equal(t, meta.CreatedAt, meta.LastDeployedAt)

	// Deployment tracking
	assert.Equal(t, 1, meta.DeploymentCount)
	assert.Contains(t, meta.DeploymentID, "deploy-1-")
	assert.Len(t, meta.DeploymentID, len("deploy-1-2025-01-01"))

	// Scale tracking
	assert.Equal(t, "initial", meta.LastScaleOperation)
	assert.Equal(t, 0, meta.PreviousNodeCount)
	assert.Equal(t, 3, meta.CurrentNodeCount)

	// Version info
	assert.Equal(t, "1.0.0", meta.SlothVersion)
	assert.Equal(t, "3.x", meta.PulumiVersion)
	assert.Equal(t, string(versioning.CurrentSchema), meta.SchemaVersion)

	// Checksum matches content
	expectedChecksum := generateSHA256Checksum(manifest)
	assert.Equal(t, expectedChecksum, meta.ConfigChecksum)

	// State snapshot ID includes deployment ID and checksum prefix
	assert.Contains(t, meta.StateSnapshotID, "state-deploy-1-")
	assert.Contains(t, meta.StateSnapshotID, expectedChecksum[:8])

	// ConfigVersion should be created
	assert.NotNil(t, meta.ConfigVersion)

	// ManifestHashes initialized but empty
	assert.NotNil(t, meta.ManifestHashes)
	assert.Empty(t, meta.ManifestHashes)

	// ChangeLog should have cluster_created entry
	require.NotEmpty(t, meta.ChangeLog)
	assert.Equal(t, "cluster_created", meta.ChangeLog[0].ChangeType)
	assert.Equal(t, "prod-cluster", meta.ChangeLog[0].ResourceID)
	assert.Contains(t, meta.ChangeLog[0].Description, "3 nodes")
	assert.Equal(t, "sloth-kubernetes", meta.ChangeLog[0].Actor)
	assert.Equal(t, "3", meta.ChangeLog[0].NewValue)

	// No parent state for initial
	assert.Empty(t, meta.ParentStateID)

	// No previous deployments
	assert.Empty(t, meta.PreviousDeployments)
}

func TestGenerateDeploymentMetadata_InitialWithPools_AllPoolsAdded(t *testing.T) {
	cfg := &config.ClusterConfig{
		Metadata: config.Metadata{Name: "pool-cluster"},
		NodePools: map[string]config.NodePool{
			"masters": {Count: 3, Provider: "do", Roles: []string{"master"}},
			"workers": {Count: 5, Provider: "linode", Roles: []string{"worker"}},
			"gpu":     {Count: 2, Provider: "aws", Roles: []string{"worker"}},
		},
	}

	meta := generateDeploymentMetadata(cfg, "", 10, "manifest")

	// All 3 pools should be in NodePoolsAdded
	assert.Len(t, meta.NodePoolsAdded, 3)
	assert.Contains(t, meta.NodePoolsAdded, "masters")
	assert.Contains(t, meta.NodePoolsAdded, "workers")
	assert.Contains(t, meta.NodePoolsAdded, "gpu")

	// ChangeLog should have pool_added entries for each pool + cluster_created
	poolAddedCount := 0
	for _, entry := range meta.ChangeLog {
		if entry.ChangeType == "pool_added" {
			poolAddedCount++
			assert.Contains(t, entry.Description, "Initial node pool")
			assert.Equal(t, "sloth-kubernetes", entry.Actor)
		}
	}
	assert.Equal(t, 3, poolAddedCount)

	// cluster_created should be first
	assert.Equal(t, "cluster_created", meta.ChangeLog[0].ChangeType)
}

func TestGenerateDeploymentMetadata_ScaleUp_FullVerification(t *testing.T) {
	prevMeta := &DeploymentMetadata{
		CreatedAt:        "2025-06-01T10:00:00Z",
		DeploymentCount:  2,
		DeploymentID:     "deploy-2-2025-06-15",
		CurrentNodeCount: 3,
		ConfigChecksum:   generateSHA256Checksum("same-manifest"),
		LastDeployedAt:   "2025-06-15T10:00:00Z",
		SchemaVersion:    "2.0",
		StateSnapshotID:  "state-deploy-2-2025-06-15-abcd1234",
	}
	prevJSON, err := json.Marshal(prevMeta)
	require.NoError(t, err)

	cfg := &config.ClusterConfig{
		Metadata: config.Metadata{Name: "scale-cluster"},
	}

	meta := generateDeploymentMetadata(cfg, string(prevJSON), 7, "same-manifest")

	// Preserved from previous
	assert.Equal(t, "2025-06-01T10:00:00Z", meta.CreatedAt)
	assert.Equal(t, 3, meta.DeploymentCount) // prev was 2, now 3

	// Scale operation
	assert.Equal(t, "scale-up", meta.LastScaleOperation)
	assert.Equal(t, 3, meta.PreviousNodeCount)
	assert.Equal(t, 7, meta.CurrentNodeCount)

	// Parent state linked
	assert.Equal(t, "state-deploy-2-2025-06-15-abcd1234", meta.ParentStateID)

	// ChangeLog has scale_up entry
	found := false
	for _, entry := range meta.ChangeLog {
		if entry.ChangeType == "scale_up" {
			found = true
			assert.Equal(t, "scale-cluster", entry.ResourceID)
			assert.Equal(t, "3", entry.OldValue)
			assert.Equal(t, "7", entry.NewValue)
			assert.Contains(t, entry.Description, "3 to 7")
			assert.Equal(t, "sloth-kubernetes", entry.Actor)
			assert.NotEmpty(t, entry.Timestamp)
		}
	}
	assert.True(t, found, "missing scale_up changelog entry")

	// No config_updated entry (same manifest)
	for _, entry := range meta.ChangeLog {
		assert.NotEqual(t, "config_updated", entry.ChangeType)
	}

	// Deployment history has previous entry
	require.Len(t, meta.PreviousDeployments, 1)
	assert.Equal(t, "deploy-2-2025-06-15", meta.PreviousDeployments[0].DeploymentID)
	assert.Equal(t, 3, meta.PreviousDeployments[0].NodeCount)
	assert.True(t, meta.PreviousDeployments[0].Success)
	assert.Equal(t, "2.0", meta.PreviousDeployments[0].SchemaVersion)
}

func TestGenerateDeploymentMetadata_ScaleDown_FullVerification(t *testing.T) {
	prevMeta := &DeploymentMetadata{
		CreatedAt:        "2025-01-01T00:00:00Z",
		DeploymentCount:  5,
		DeploymentID:     "deploy-5-2025-03-01",
		CurrentNodeCount: 10,
		ConfigChecksum:   generateSHA256Checksum("old-manifest"),
		LastDeployedAt:   "2025-03-01T12:00:00Z",
	}
	prevJSON, _ := json.Marshal(prevMeta)

	cfg := &config.ClusterConfig{
		Metadata: config.Metadata{Name: "shrinking-cluster"},
	}

	meta := generateDeploymentMetadata(cfg, string(prevJSON), 4, "old-manifest")

	assert.Equal(t, "scale-down", meta.LastScaleOperation)
	assert.Equal(t, 10, meta.PreviousNodeCount)
	assert.Equal(t, 4, meta.CurrentNodeCount)
	assert.Equal(t, 6, meta.DeploymentCount)

	// Verify scale_down changelog entry format
	found := false
	for _, entry := range meta.ChangeLog {
		if entry.ChangeType == "scale_down" {
			found = true
			assert.Equal(t, "shrinking-cluster", entry.ResourceID)
			assert.Equal(t, "10", entry.OldValue)
			assert.Equal(t, "4", entry.NewValue)
			assert.Contains(t, entry.Description, "10 to 4")
			assert.Equal(t, "sloth-kubernetes", entry.Actor)
		}
	}
	assert.True(t, found, "missing scale_down changelog entry")
}

func TestGenerateDeploymentMetadata_ScaleAndConfigChangeTogether(t *testing.T) {
	prevMeta := &DeploymentMetadata{
		CreatedAt:        "2025-01-01T00:00:00Z",
		DeploymentCount:  1,
		DeploymentID:     "deploy-1-2025-01-01",
		CurrentNodeCount: 3,
		ConfigChecksum:   "old-checksum-will-not-match",
		LastDeployedAt:   "2025-01-01T00:00:00Z",
	}
	prevJSON, _ := json.Marshal(prevMeta)

	cfg := &config.ClusterConfig{
		Metadata: config.Metadata{Name: "evolving-cluster"},
	}

	// Different manifest (config change) AND different node count (scale-up)
	meta := generateDeploymentMetadata(cfg, string(prevJSON), 6, "completely-new-manifest-v2")

	assert.Equal(t, "scale-up", meta.LastScaleOperation)
	assert.Equal(t, 6, meta.CurrentNodeCount)

	// Both scale_up AND config_updated should be in changelog
	scaleFound := false
	configFound := false
	for _, entry := range meta.ChangeLog {
		if entry.ChangeType == "scale_up" {
			scaleFound = true
		}
		if entry.ChangeType == "config_updated" {
			configFound = true
			assert.Equal(t, "old-checksum-will-not-match", entry.OldValue)
			newChecksum := generateSHA256Checksum("completely-new-manifest-v2")
			assert.Equal(t, newChecksum, entry.NewValue)
		}
	}
	assert.True(t, scaleFound, "missing scale_up when both scale and config change")
	assert.True(t, configFound, "missing config_updated when both scale and config change")
}

func TestGenerateDeploymentMetadata_UpdateNoCountChange(t *testing.T) {
	manifest := "same-content-for-both"
	checksum := generateSHA256Checksum(manifest)

	prevMeta := &DeploymentMetadata{
		CreatedAt:        "2025-01-01T00:00:00Z",
		DeploymentCount:  3,
		DeploymentID:     "deploy-3-2025-02-01",
		CurrentNodeCount: 5,
		ConfigChecksum:   checksum,
		LastDeployedAt:   "2025-02-01T00:00:00Z",
	}
	prevJSON, _ := json.Marshal(prevMeta)

	cfg := &config.ClusterConfig{Metadata: config.Metadata{Name: "stable"}}

	meta := generateDeploymentMetadata(cfg, string(prevJSON), 5, manifest)

	assert.Equal(t, "update", meta.LastScaleOperation)
	assert.Equal(t, 5, meta.PreviousNodeCount)
	assert.Equal(t, 5, meta.CurrentNodeCount)
	assert.Equal(t, 4, meta.DeploymentCount)

	// No scale or config changelog entries (no actual changes)
	for _, entry := range meta.ChangeLog {
		assert.NotEqual(t, "scale_up", entry.ChangeType)
		assert.NotEqual(t, "scale_down", entry.ChangeType)
		assert.NotEqual(t, "config_updated", entry.ChangeType)
	}
}

func TestGenerateDeploymentMetadata_ConfigVersionParentHashLinking(t *testing.T) {
	prevMeta := &DeploymentMetadata{
		CreatedAt:        "2025-01-01T00:00:00Z",
		DeploymentCount:  1,
		DeploymentID:     "deploy-1-2025-01-01",
		CurrentNodeCount: 3,
		ConfigChecksum:   "prev-checksum",
		LastDeployedAt:   "2025-01-01T00:00:00Z",
		ConfigVersion: &versioning.ConfigVersion{
			ConfigHash: "parent-config-hash-abc123",
		},
	}
	prevJSON, _ := json.Marshal(prevMeta)

	cfg := &config.ClusterConfig{Metadata: config.Metadata{Name: "versioned"}}

	meta := generateDeploymentMetadata(cfg, string(prevJSON), 3, "manifest-v2")

	// ConfigVersion should exist with ParentHash linked
	require.NotNil(t, meta.ConfigVersion)
	assert.Equal(t, "parent-config-hash-abc123", meta.ConfigVersion.ParentHash)
}

func TestGenerateDeploymentMetadata_ConfigVersionNilInPrevious(t *testing.T) {
	prevMeta := &DeploymentMetadata{
		CreatedAt:        "2025-01-01T00:00:00Z",
		DeploymentCount:  1,
		DeploymentID:     "deploy-1-2025-01-01",
		CurrentNodeCount: 3,
		ConfigChecksum:   "check",
		LastDeployedAt:   "2025-01-01T00:00:00Z",
		ConfigVersion:    nil, // No config version in previous
	}
	prevJSON, _ := json.Marshal(prevMeta)

	cfg := &config.ClusterConfig{Metadata: config.Metadata{Name: "no-prev-version"}}

	meta := generateDeploymentMetadata(cfg, string(prevJSON), 3, "manifest")

	// Should still work - ConfigVersion created, but no ParentHash
	assert.NotNil(t, meta.ConfigVersion)
	assert.Empty(t, meta.ConfigVersion.ParentHash)
}

func TestGenerateDeploymentMetadata_HistoryBoundaryExactly10(t *testing.T) {
	// Previous meta already has 9 entries (+ current prev = 10 total after prepend)
	prevHistory := make([]DeploymentHistoryEntry, 9)
	for i := 0; i < 9; i++ {
		prevHistory[i] = DeploymentHistoryEntry{
			DeploymentID: fmt.Sprintf("deploy-%d", i+1),
			Timestamp:    fmt.Sprintf("2025-01-%02dT00:00:00Z", i+1),
			NodeCount:    i + 1,
			Success:      true,
		}
	}

	prevMeta := &DeploymentMetadata{
		CreatedAt:           "2025-01-01T00:00:00Z",
		DeploymentCount:     10,
		DeploymentID:        "deploy-10-2025-01-10",
		CurrentNodeCount:    3,
		ConfigChecksum:      "cs",
		LastDeployedAt:      "2025-01-10T00:00:00Z",
		PreviousDeployments: prevHistory,
	}
	prevJSON, _ := json.Marshal(prevMeta)

	cfg := &config.ClusterConfig{Metadata: config.Metadata{Name: "t"}}

	meta := generateDeploymentMetadata(cfg, string(prevJSON), 3, "m")

	// 9 existing + 1 new prepended = 10, should be exactly 10
	assert.Len(t, meta.PreviousDeployments, 10)
	// First entry should be the most recent previous
	assert.Equal(t, "deploy-10-2025-01-10", meta.PreviousDeployments[0].DeploymentID)
}

func TestGenerateDeploymentMetadata_HistoryTruncation11To10(t *testing.T) {
	prevHistory := make([]DeploymentHistoryEntry, 10)
	for i := 0; i < 10; i++ {
		prevHistory[i] = DeploymentHistoryEntry{
			DeploymentID: fmt.Sprintf("deploy-%d", i+1),
			NodeCount:    i + 1,
			Success:      true,
		}
	}

	prevMeta := &DeploymentMetadata{
		CreatedAt:           "2025-01-01T00:00:00Z",
		DeploymentCount:     11,
		DeploymentID:        "deploy-11-2025-01-11",
		CurrentNodeCount:    3,
		ConfigChecksum:      "cs",
		LastDeployedAt:      "2025-01-11T00:00:00Z",
		PreviousDeployments: prevHistory,
	}
	prevJSON, _ := json.Marshal(prevMeta)

	cfg := &config.ClusterConfig{Metadata: config.Metadata{Name: "t"}}

	meta := generateDeploymentMetadata(cfg, string(prevJSON), 3, "m")

	// 10 existing + 1 new = 11, truncated to 10
	assert.Len(t, meta.PreviousDeployments, 10)
	// Newest entry (prepended) is the previous deployment
	assert.Equal(t, "deploy-11-2025-01-11", meta.PreviousDeployments[0].DeploymentID)
	// Tail entry is deploy-9 (deploy-10 was truncated off the end)
	assert.Equal(t, "deploy-9", meta.PreviousDeployments[9].DeploymentID)
}

func TestGenerateDeploymentMetadata_InvalidJSON_FallbackToInitial(t *testing.T) {
	invalidInputs := []string{
		"",
		"not-json",
		"{incomplete",
		"null",
		"[]",    // valid JSON but wrong type
		"12345", // valid JSON but wrong type
	}

	cfg := &config.ClusterConfig{
		Metadata:  config.Metadata{Name: "fallback"},
		NodePools: map[string]config.NodePool{},
	}

	for _, input := range invalidInputs {
		t.Run(fmt.Sprintf("input=%q", input), func(t *testing.T) {
			meta := generateDeploymentMetadata(cfg, input, 2, "manifest")

			// All invalid inputs should result in initial deployment behavior
			assert.Equal(t, "initial", meta.LastScaleOperation)
			assert.Equal(t, 1, meta.DeploymentCount)
			assert.Equal(t, 0, meta.PreviousNodeCount)
			assert.Equal(t, 2, meta.CurrentNodeCount)
			assert.NotEmpty(t, meta.StateSnapshotID)
		})
	}
}

func TestGenerateDeploymentMetadata_StateSnapshotIDFormat(t *testing.T) {
	cfg := &config.ClusterConfig{
		Metadata:  config.Metadata{Name: "snapshot-test"},
		NodePools: map[string]config.NodePool{},
	}

	manifest := "test manifest content"
	meta := generateDeploymentMetadata(cfg, "", 3, manifest)

	checksum := generateSHA256Checksum(manifest)
	expectedPrefix := fmt.Sprintf("state-%s-", meta.DeploymentID)
	assert.True(t, strings.HasPrefix(meta.StateSnapshotID, expectedPrefix),
		"StateSnapshotID %q should start with %q", meta.StateSnapshotID, expectedPrefix)
	assert.True(t, strings.HasSuffix(meta.StateSnapshotID, checksum[:8]),
		"StateSnapshotID %q should end with checksum prefix %q", meta.StateSnapshotID, checksum[:8])
}

func TestGenerateDeploymentMetadata_ParentStateLinked(t *testing.T) {
	prevMeta := &DeploymentMetadata{
		CreatedAt:        "2025-01-01T00:00:00Z",
		DeploymentCount:  1,
		DeploymentID:     "deploy-1-2025-01-01",
		CurrentNodeCount: 3,
		ConfigChecksum:   "cs",
		LastDeployedAt:   "2025-01-01T00:00:00Z",
		StateSnapshotID:  "state-deploy-1-2025-01-01-abcdef12",
	}
	prevJSON, _ := json.Marshal(prevMeta)

	cfg := &config.ClusterConfig{Metadata: config.Metadata{Name: "t"}}
	meta := generateDeploymentMetadata(cfg, string(prevJSON), 3, "m")

	assert.Equal(t, "state-deploy-1-2025-01-01-abcdef12", meta.ParentStateID)
}

func TestGenerateDeploymentMetadata_NoParentStateWhenEmpty(t *testing.T) {
	prevMeta := &DeploymentMetadata{
		CreatedAt:        "2025-01-01T00:00:00Z",
		DeploymentCount:  1,
		DeploymentID:     "deploy-1-2025-01-01",
		CurrentNodeCount: 3,
		ConfigChecksum:   "cs",
		LastDeployedAt:   "2025-01-01T00:00:00Z",
		StateSnapshotID:  "", // Empty - no snapshot
	}
	prevJSON, _ := json.Marshal(prevMeta)

	cfg := &config.ClusterConfig{Metadata: config.Metadata{Name: "t"}}
	meta := generateDeploymentMetadata(cfg, string(prevJSON), 3, "m")

	assert.Empty(t, meta.ParentStateID)
}

func TestGenerateDeploymentMetadata_FullLifecycle(t *testing.T) {
	cfg := &config.ClusterConfig{
		Metadata: config.Metadata{Name: "lifecycle-cluster"},
		NodePools: map[string]config.NodePool{
			"workers": {Count: 3},
		},
	}

	// Step 1: Initial deployment
	meta1 := generateDeploymentMetadata(cfg, "", 3, "manifest-v1")
	assert.Equal(t, "initial", meta1.LastScaleOperation)
	assert.Equal(t, 1, meta1.DeploymentCount)
	assert.Equal(t, 3, meta1.CurrentNodeCount)
	meta1JSON, _ := json.Marshal(meta1)

	// Step 2: Scale up
	meta2 := generateDeploymentMetadata(cfg, string(meta1JSON), 5, "manifest-v1")
	assert.Equal(t, "scale-up", meta2.LastScaleOperation)
	assert.Equal(t, 2, meta2.DeploymentCount)
	assert.Equal(t, 3, meta2.PreviousNodeCount)
	assert.Equal(t, 5, meta2.CurrentNodeCount)
	assert.Equal(t, meta1.CreatedAt, meta2.CreatedAt, "CreatedAt must be preserved")
	assert.Equal(t, meta1.StateSnapshotID, meta2.ParentStateID)
	meta2JSON, _ := json.Marshal(meta2)

	// Step 3: Config change (same count)
	meta3 := generateDeploymentMetadata(cfg, string(meta2JSON), 5, "manifest-v2-changed")
	assert.Equal(t, "update", meta3.LastScaleOperation)
	assert.Equal(t, 3, meta3.DeploymentCount)
	assert.Equal(t, 5, meta3.PreviousNodeCount)
	assert.Equal(t, 5, meta3.CurrentNodeCount)
	assert.Equal(t, meta1.CreatedAt, meta3.CreatedAt)
	assert.Equal(t, meta2.StateSnapshotID, meta3.ParentStateID)
	meta3JSON, _ := json.Marshal(meta3)

	// Step 4: Scale down
	meta4 := generateDeploymentMetadata(cfg, string(meta3JSON), 2, "manifest-v2-changed")
	assert.Equal(t, "scale-down", meta4.LastScaleOperation)
	assert.Equal(t, 4, meta4.DeploymentCount)
	assert.Equal(t, 5, meta4.PreviousNodeCount)
	assert.Equal(t, 2, meta4.CurrentNodeCount)
	assert.Equal(t, meta1.CreatedAt, meta4.CreatedAt)

	// History should have 3 entries (meta1, meta2, meta3)
	assert.Len(t, meta4.PreviousDeployments, 3)
	assert.Equal(t, meta3.DeploymentID, meta4.PreviousDeployments[0].DeploymentID)
	assert.Equal(t, meta2.DeploymentID, meta4.PreviousDeployments[1].DeploymentID)
	assert.Equal(t, meta1.DeploymentID, meta4.PreviousDeployments[2].DeploymentID)
}

func TestGenerateDeploymentMetadata_DeploymentIDFormat(t *testing.T) {
	prevMeta := &DeploymentMetadata{
		CreatedAt:        "2025-01-01T00:00:00Z",
		DeploymentCount:  42,
		DeploymentID:     "deploy-42-2025-01-01",
		CurrentNodeCount: 3,
		ConfigChecksum:   "cs",
		LastDeployedAt:   "2025-01-01T00:00:00Z",
	}
	prevJSON, _ := json.Marshal(prevMeta)

	cfg := &config.ClusterConfig{Metadata: config.Metadata{Name: "t"}}
	meta := generateDeploymentMetadata(cfg, string(prevJSON), 3, "m")

	// DeploymentID format: "deploy-{count}-{date}"
	assert.True(t, strings.HasPrefix(meta.DeploymentID, "deploy-43-"),
		"expected deploy-43-*, got %s", meta.DeploymentID)
	// Date portion should be 10 chars (YYYY-MM-DD)
	parts := strings.SplitN(meta.DeploymentID, "-", 3)
	require.Len(t, parts, 3)
	assert.Len(t, parts[2], 10) // "2025-01-24"
}

func TestGenerateDeploymentMetadata_HistoryEntryFields(t *testing.T) {
	prevMeta := &DeploymentMetadata{
		CreatedAt:        "2025-03-15T08:30:00Z",
		DeploymentCount:  5,
		DeploymentID:     "deploy-5-2025-03-15",
		CurrentNodeCount: 7,
		ConfigChecksum:   "prev-hash-xyz",
		LastDeployedAt:   "2025-03-15T08:30:00Z",
		SchemaVersion:    "2.0",
	}
	prevJSON, _ := json.Marshal(prevMeta)

	cfg := &config.ClusterConfig{Metadata: config.Metadata{Name: "t"}}
	meta := generateDeploymentMetadata(cfg, string(prevJSON), 7, "m")

	require.NotEmpty(t, meta.PreviousDeployments)
	entry := meta.PreviousDeployments[0]

	assert.Equal(t, "deploy-5-2025-03-15", entry.DeploymentID)
	assert.Equal(t, "2025-03-15T08:30:00Z", entry.Timestamp)
	assert.Equal(t, 7, entry.NodeCount)
	assert.Equal(t, "prev-hash-xyz", entry.ConfigChecksum)
	assert.Equal(t, "2.0", entry.SchemaVersion)
	assert.True(t, entry.Success)
	assert.Empty(t, entry.ErrorMessage)
}

func TestGenerateDeploymentMetadata_LargeManifest(t *testing.T) {
	// Realistic large manifest
	largeManifest := strings.Repeat("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\n---\n", 1000)

	cfg := &config.ClusterConfig{
		Metadata:  config.Metadata{Name: "big-cluster"},
		NodePools: map[string]config.NodePool{},
	}

	meta := generateDeploymentMetadata(cfg, "", 50, largeManifest)

	assert.Equal(t, 50, meta.CurrentNodeCount)
	assert.Len(t, meta.ConfigChecksum, 64) // Still valid SHA256
	assert.NotEmpty(t, meta.StateSnapshotID)
}

// ==================== Additional Sanitization Tests ====================

func TestSanitizeConfigForStorage_EmptyConfig(t *testing.T) {
	cfg := &config.ClusterConfig{}
	result := sanitizeConfigForStorage(cfg)
	assert.NotNil(t, result)
}

func TestSanitizeConfigForStorage_OnlyDigitalOcean(t *testing.T) {
	cfg := &config.ClusterConfig{
		Providers: config.ProvidersConfig{
			DigitalOcean: &config.DigitalOceanProvider{
				Token:   "super-secret-token",
				Enabled: true,
				Region:  "nyc1",
			},
		},
	}

	result := sanitizeConfigForStorage(cfg)

	assert.Equal(t, "${DIGITALOCEAN_TOKEN}", result.Providers.DigitalOcean.Token)
	assert.True(t, result.Providers.DigitalOcean.Enabled)
	assert.Equal(t, "nyc1", result.Providers.DigitalOcean.Region)
}

func TestSanitizeConfigForStorage_OnlyLinode(t *testing.T) {
	cfg := &config.ClusterConfig{
		Providers: config.ProvidersConfig{
			Linode: &config.LinodeProvider{
				Token:        "linode-api-token-12345",
				RootPassword: "super-secret-password",
				Enabled:      true,
				Region:       "us-east",
			},
		},
	}

	result := sanitizeConfigForStorage(cfg)

	assert.Equal(t, "${LINODE_TOKEN}", result.Providers.Linode.Token)
	assert.Equal(t, "${LINODE_ROOT_PASSWORD}", result.Providers.Linode.RootPassword)
	assert.True(t, result.Providers.Linode.Enabled)
	assert.Equal(t, "us-east", result.Providers.Linode.Region)
}

func TestSanitizeConfigForStorage_OnlyAWS(t *testing.T) {
	cfg := &config.ClusterConfig{
		Providers: config.ProvidersConfig{
			AWS: &config.AWSProvider{
				AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
				SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
				Enabled:         true,
				Region:          "us-east-1",
			},
		},
	}

	result := sanitizeConfigForStorage(cfg)

	assert.Equal(t, "${AWS_ACCESS_KEY_ID}", result.Providers.AWS.AccessKeyID)
	assert.Equal(t, "${AWS_SECRET_ACCESS_KEY}", result.Providers.AWS.SecretAccessKey)
	assert.True(t, result.Providers.AWS.Enabled)
	assert.Equal(t, "us-east-1", result.Providers.AWS.Region)
}

func TestSanitizeConfigForStorage_OnlyAzure(t *testing.T) {
	cfg := &config.ClusterConfig{
		Providers: config.ProvidersConfig{
			Azure: &config.AzureProvider{
				SubscriptionID: "11111111-1111-1111-1111-111111111111",
				ClientID:       "22222222-2222-2222-2222-222222222222",
				ClientSecret:   "azure-client-secret-value",
				TenantID:       "33333333-3333-3333-3333-333333333333",
				Enabled:        true,
			},
		},
	}

	result := sanitizeConfigForStorage(cfg)

	assert.Equal(t, "${AZURE_SUBSCRIPTION_ID}", result.Providers.Azure.SubscriptionID)
	assert.Equal(t, "${AZURE_CLIENT_ID}", result.Providers.Azure.ClientID)
	assert.Equal(t, "${AZURE_CLIENT_SECRET}", result.Providers.Azure.ClientSecret)
	assert.Equal(t, "${AZURE_TENANT_ID}", result.Providers.Azure.TenantID)
	assert.True(t, result.Providers.Azure.Enabled)
}

func TestSanitizeConfigForStorage_OnlyGCP(t *testing.T) {
	cfg := &config.ClusterConfig{
		Providers: config.ProvidersConfig{
			GCP: &config.GCPProvider{
				ProjectID:   "my-project-id",
				Credentials: `{"type": "service_account", "private_key": "-----BEGIN RSA PRIVATE KEY-----..."}`,
				Enabled:     true,
				Region:      "us-central1",
			},
		},
	}

	result := sanitizeConfigForStorage(cfg)

	// ProjectID is NOT sanitized, only Credentials
	assert.Equal(t, "my-project-id", result.Providers.GCP.ProjectID)
	assert.Equal(t, "${GCP_CREDENTIALS}", result.Providers.GCP.Credentials)
	assert.True(t, result.Providers.GCP.Enabled)
	assert.Equal(t, "us-central1", result.Providers.GCP.Region)
}

func TestSanitizeConfigForStorage_PreservesMetadata(t *testing.T) {
	cfg := &config.ClusterConfig{
		Metadata: config.Metadata{
			Name:        "production-cluster",
			Environment: "prod",
			Version:     "v1.28.0",
		},
	}

	result := sanitizeConfigForStorage(cfg)

	assert.Equal(t, "production-cluster", result.Metadata.Name)
	assert.Equal(t, "prod", result.Metadata.Environment)
	assert.Equal(t, "v1.28.0", result.Metadata.Version)
}

func TestSanitizeConfigForStorage_PreservesKubernetesConfig(t *testing.T) {
	cfg := &config.ClusterConfig{
		Kubernetes: config.KubernetesConfig{
			Version:       "v1.28.4",
			ClusterDomain: "cluster.local",
			ServiceCIDR:   "10.96.0.0/12",
			PodCIDR:       "10.244.0.0/16",
		},
	}

	result := sanitizeConfigForStorage(cfg)

	assert.Equal(t, "v1.28.4", result.Kubernetes.Version)
	assert.Equal(t, "cluster.local", result.Kubernetes.ClusterDomain)
	assert.Equal(t, "10.96.0.0/12", result.Kubernetes.ServiceCIDR)
	assert.Equal(t, "10.244.0.0/16", result.Kubernetes.PodCIDR)
}

func TestSanitizeConfigForStorage_PreservesNetworkConfig(t *testing.T) {
	cfg := &config.ClusterConfig{
		Network: config.NetworkConfig{
			CIDR: "10.0.0.0/16",
			WireGuard: &config.WireGuardConfig{
				Enabled: true,
				Port:    51820,
			},
		},
	}

	result := sanitizeConfigForStorage(cfg)

	assert.Equal(t, "10.0.0.0/16", result.Network.CIDR)
	assert.NotNil(t, result.Network.WireGuard)
	assert.True(t, result.Network.WireGuard.Enabled)
	assert.Equal(t, 51820, result.Network.WireGuard.Port)
}

func TestSanitizeConfigForStorage_MultipleProvidersPartiallyConfigured(t *testing.T) {
	cfg := &config.ClusterConfig{
		Providers: config.ProvidersConfig{
			DigitalOcean: &config.DigitalOceanProvider{Token: "do-token", Enabled: true},
			Linode:       nil, // Not configured
			AWS:          &config.AWSProvider{AccessKeyID: "aws-key", SecretAccessKey: "aws-secret", Enabled: false},
			Azure:        nil, // Not configured
			GCP:          &config.GCPProvider{ProjectID: "gcp-proj", Credentials: "gcp-creds", Enabled: true},
		},
	}

	result := sanitizeConfigForStorage(cfg)

	assert.Equal(t, "${DIGITALOCEAN_TOKEN}", result.Providers.DigitalOcean.Token)
	assert.Nil(t, result.Providers.Linode)
	assert.Equal(t, "${AWS_ACCESS_KEY_ID}", result.Providers.AWS.AccessKeyID)
	assert.Nil(t, result.Providers.Azure)
	// GCP ProjectID is NOT sanitized, only Credentials
	assert.Equal(t, "gcp-proj", result.Providers.GCP.ProjectID)
	assert.Equal(t, "${GCP_CREDENTIALS}", result.Providers.GCP.Credentials)
}

// ==================== Checksum Edge Cases ====================

func TestGenerateChecksum_VeryLongInput(t *testing.T) {
	// 1MB string
	longInput := strings.Repeat("a", 1024*1024)

	checksum := generateChecksum(longInput)

	assert.NotEmpty(t, checksum)
	assert.NotEqual(t, "0", checksum)
}

func TestGenerateChecksum_BinaryData(t *testing.T) {
	// String with null bytes and special characters
	binaryLike := string([]byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD})

	checksum := generateChecksum(binaryLike)

	assert.NotEmpty(t, checksum)
	assert.NotEqual(t, "0", checksum)
}

func TestGenerateChecksum_UnicodeInput(t *testing.T) {
	unicodeInput := "こんにちは世界 🌍 مرحبا 你好 Привет"

	checksum := generateChecksum(unicodeInput)

	assert.NotEmpty(t, checksum)
	assert.NotEqual(t, "0", checksum)

	// Verify determinism with unicode
	checksum2 := generateChecksum(unicodeInput)
	assert.Equal(t, checksum, checksum2)
}

func TestGenerateSHA256Checksum_EmptyVsWhitespace(t *testing.T) {
	empty := generateSHA256Checksum("")
	space := generateSHA256Checksum(" ")
	tab := generateSHA256Checksum("\t")
	newline := generateSHA256Checksum("\n")

	// All should be different
	assert.NotEqual(t, empty, space)
	assert.NotEqual(t, empty, tab)
	assert.NotEqual(t, empty, newline)
	assert.NotEqual(t, space, tab)
	assert.NotEqual(t, space, newline)
	assert.NotEqual(t, tab, newline)
}

func TestGenerateSHA256Checksum_LeadingTrailingWhitespace(t *testing.T) {
	base := generateSHA256Checksum("test")
	leading := generateSHA256Checksum(" test")
	trailing := generateSHA256Checksum("test ")
	both := generateSHA256Checksum(" test ")

	// All should be different (whitespace matters)
	assert.NotEqual(t, base, leading)
	assert.NotEqual(t, base, trailing)
	assert.NotEqual(t, base, both)
	assert.NotEqual(t, leading, trailing)
}

// ==================== Deployment Metadata Edge Cases ====================

func TestGenerateDeploymentMetadata_ZeroNodeCount(t *testing.T) {
	cfg := &config.ClusterConfig{
		Metadata:  config.Metadata{Name: "empty-cluster"},
		NodePools: map[string]config.NodePool{},
	}

	meta := generateDeploymentMetadata(cfg, "", 0, "manifest")

	assert.Equal(t, 0, meta.CurrentNodeCount)
	assert.Equal(t, 0, meta.PreviousNodeCount)
	assert.Equal(t, "initial", meta.LastScaleOperation)
}

func TestGenerateDeploymentMetadata_NegativeNodeCount(t *testing.T) {
	cfg := &config.ClusterConfig{
		Metadata: config.Metadata{Name: "test"},
	}

	// The function should handle this gracefully
	meta := generateDeploymentMetadata(cfg, "", -5, "manifest")

	assert.Equal(t, -5, meta.CurrentNodeCount)
	// The function doesn't validate, just stores
}

func TestGenerateDeploymentMetadata_ScaleOperationDetection(t *testing.T) {
	testCases := []struct {
		name              string
		prevCount         int
		currentCount      int
		expectedOperation string
	}{
		{"scale up by 1", 5, 6, "scale-up"},
		{"scale up large", 10, 100, "scale-up"},
		{"scale down by 1", 6, 5, "scale-down"},
		{"scale down large", 100, 10, "scale-down"},
		{"no change", 10, 10, "update"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			prevMeta := &DeploymentMetadata{
				CreatedAt:        "2025-01-01T00:00:00Z", // Must have CreatedAt to not fall into initial
				CurrentNodeCount: tc.prevCount,
				DeploymentID:     "prev-deploy",
				ConfigChecksum:   "prev-checksum",
				LastDeployedAt:   "2025-01-01T00:00:00Z",
			}
			prevJSON, _ := json.Marshal(prevMeta)

			cfg := &config.ClusterConfig{Metadata: config.Metadata{Name: "test"}}
			meta := generateDeploymentMetadata(cfg, string(prevJSON), tc.currentCount, "manifest")

			assert.Equal(t, tc.expectedOperation, meta.LastScaleOperation)
			assert.Equal(t, tc.prevCount, meta.PreviousNodeCount)
			assert.Equal(t, tc.currentCount, meta.CurrentNodeCount)
		})
	}
}

func TestGenerateDeploymentMetadata_VersionFieldsPopulated(t *testing.T) {
	cfg := &config.ClusterConfig{
		Metadata: config.Metadata{Name: "test"},
	}

	meta := generateDeploymentMetadata(cfg, "", 5, "manifest")

	// These should have default values
	assert.NotEmpty(t, meta.SlothVersion)
	assert.NotEmpty(t, meta.PulumiVersion)
}

func TestGenerateDeploymentMetadata_DeploymentCountIncreases(t *testing.T) {
	cfg := &config.ClusterConfig{Metadata: config.Metadata{Name: "test"}}

	// First deployment
	meta1 := generateDeploymentMetadata(cfg, "", 3, "m")
	assert.Equal(t, 1, meta1.DeploymentCount)

	// Subsequent deployments
	for i := 2; i <= 5; i++ {
		prevJSON, _ := json.Marshal(meta1)
		meta1 = generateDeploymentMetadata(cfg, string(prevJSON), 3, "m")
		assert.Equal(t, i, meta1.DeploymentCount)
	}
}

func TestGenerateDeploymentMetadata_TimestampsAreRFC3339(t *testing.T) {
	cfg := &config.ClusterConfig{Metadata: config.Metadata{Name: "test"}}

	meta := generateDeploymentMetadata(cfg, "", 1, "m")

	// Parse should succeed for RFC3339
	_, err := time.Parse(time.RFC3339, meta.CreatedAt)
	assert.NoError(t, err, "CreatedAt should be valid RFC3339")

	_, err = time.Parse(time.RFC3339, meta.UpdatedAt)
	assert.NoError(t, err, "UpdatedAt should be valid RFC3339")

	_, err = time.Parse(time.RFC3339, meta.LastDeployedAt)
	assert.NoError(t, err, "LastDeployedAt should be valid RFC3339")
}

func TestGenerateDeploymentMetadata_ChangeLogEntries(t *testing.T) {
	// Initial deployment should have no changelog
	cfg := &config.ClusterConfig{
		Metadata:  config.Metadata{Name: "test"},
		NodePools: map[string]config.NodePool{"pool1": {Count: 3}},
	}

	meta1 := generateDeploymentMetadata(cfg, "", 3, "m")
	// Initial has NodePoolsAdded not ChangeLog
	assert.NotEmpty(t, meta1.NodePoolsAdded)

	// Scale up should create changelog entry
	prevJSON, _ := json.Marshal(&DeploymentMetadata{
		CreatedAt:        "2025-01-01T00:00:00Z", // Must have CreatedAt to not fall into initial
		CurrentNodeCount: 3,
		DeploymentID:     "deploy-1",
		ConfigChecksum:   "cs1",
		LastDeployedAt:   "2025-01-01T00:00:00Z",
	})

	meta2 := generateDeploymentMetadata(cfg, string(prevJSON), 5, "m")
	assert.NotEmpty(t, meta2.ChangeLog)
	// scale-up is stored in "Description" with "Scaled cluster from X to Y nodes"
	found := false
	for _, entry := range meta2.ChangeLog {
		if strings.Contains(entry.Description, "Scaled cluster from") {
			found = true
			break
		}
	}
	assert.True(t, found, "ChangeLog should contain scale operation")
}

func TestGenerateDeploymentMetadata_MultiplePoolsTracked(t *testing.T) {
	cfg := &config.ClusterConfig{
		Metadata: config.Metadata{Name: "test"},
		NodePools: map[string]config.NodePool{
			"masters": {Count: 3},
			"workers": {Count: 10},
			"gpu":     {Count: 2},
		},
	}

	meta := generateDeploymentMetadata(cfg, "", 15, "m")

	assert.Len(t, meta.NodePoolsAdded, 3)
	assert.Contains(t, meta.NodePoolsAdded, "masters")
	assert.Contains(t, meta.NodePoolsAdded, "workers")
	assert.Contains(t, meta.NodePoolsAdded, "gpu")
}

// ==================== Pool Changes Detection ====================

func TestDetectPoolChanges_EmptyConfig(t *testing.T) {
	cfg := &config.ClusterConfig{
		NodePools: map[string]config.NodePool{},
	}
	prevMeta := &DeploymentMetadata{}

	added, removed, scaled := detectPoolChanges(cfg, prevMeta)

	assert.Nil(t, added)
	assert.Nil(t, removed)
	assert.Nil(t, scaled)
}

func TestDetectPoolChanges_NilInputs(t *testing.T) {
	added, removed, scaled := detectPoolChanges(nil, nil)

	assert.Nil(t, added)
	assert.Nil(t, removed)
	assert.Nil(t, scaled)
}

func TestDetectPoolChanges_WithPools(t *testing.T) {
	cfg := &config.ClusterConfig{
		NodePools: map[string]config.NodePool{
			"workers": {Count: 5, Roles: []string{"worker"}},
		},
	}
	prevMeta := &DeploymentMetadata{
		CurrentNodeCount: 5,
	}

	added, removed, scaled := detectPoolChanges(cfg, prevMeta)

	// Current implementation returns nil for all - stub behavior
	assert.Nil(t, added)
	assert.Nil(t, removed)
	assert.Nil(t, scaled)
}

// ==================== Concurrency Tests ====================

func TestGenerateSHA256Checksum_ConcurrentCalls(t *testing.T) {
	var wg sync.WaitGroup
	results := make([]string, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx] = generateSHA256Checksum("concurrent-test-input")
		}(i)
	}

	wg.Wait()

	// All results should be identical
	for i := 1; i < 100; i++ {
		assert.Equal(t, results[0], results[i], "Concurrent calls should produce identical results")
	}
}

func TestGenerateDeploymentMetadata_ConcurrentCalls(t *testing.T) {
	var wg sync.WaitGroup
	results := make([]*DeploymentMetadata, 50)

	cfg := &config.ClusterConfig{
		Metadata:  config.Metadata{Name: "concurrent-test"},
		NodePools: map[string]config.NodePool{"workers": {Count: 5}},
	}

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx] = generateDeploymentMetadata(cfg, "", 5, "manifest")
		}(i)
	}

	wg.Wait()

	// All should have valid deployment IDs and checksums
	for i, meta := range results {
		assert.NotEmpty(t, meta.DeploymentID, "Index %d should have DeploymentID", i)
		assert.NotEmpty(t, meta.ConfigChecksum, "Index %d should have ConfigChecksum", i)
		assert.Equal(t, 5, meta.CurrentNodeCount, "Index %d should have correct node count", i)
	}
}

func TestSanitizeConfigForStorage_ConcurrentCalls(t *testing.T) {
	var wg sync.WaitGroup

	cfg := &config.ClusterConfig{
		Providers: config.ProvidersConfig{
			DigitalOcean: &config.DigitalOceanProvider{Token: "secret-token"},
			AWS:          &config.AWSProvider{AccessKeyID: "key", SecretAccessKey: "secret"},
		},
	}

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := sanitizeConfigForStorage(cfg)
			// Verify sanitization worked
			assert.Equal(t, "${DIGITALOCEAN_TOKEN}", result.Providers.DigitalOcean.Token)
			assert.Equal(t, "${AWS_ACCESS_KEY_ID}", result.Providers.AWS.AccessKeyID)
		}()
	}

	wg.Wait()

	// Original should still have real values
	assert.Equal(t, "secret-token", cfg.Providers.DigitalOcean.Token)
	assert.Equal(t, "key", cfg.Providers.AWS.AccessKeyID)
}

// ==================== REAL PRODUCTION SCENARIO TESTS ====================
// These tests are based on actual configuration files from examples/

// TestRealScenario_MinimalCluster tests deployment metadata for a minimal cluster
// based on examples/cluster-minimal.lisp: 1 master + 2 workers = 3 nodes
func TestRealScenario_MinimalCluster_InitialDeployment(t *testing.T) {
	// Configuration matching cluster-minimal.lisp
	cfg := &config.ClusterConfig{
		Metadata: config.Metadata{
			Name:        "minimal-cluster",
			Environment: "development",
		},
		Providers: config.ProvidersConfig{
			DigitalOcean: &config.DigitalOceanProvider{
				Enabled: true,
				Token:   "dop_v1_xxxxx", // Will be sanitized
				Region:  "nyc3",
			},
		},
		Network: config.NetworkConfig{
			CIDR: "10.8.0.0/24",
			WireGuard: &config.WireGuardConfig{
				Enabled:        true,
				MeshNetworking: true,
			},
		},
		NodePools: map[string]config.NodePool{
			"masters": {
				Name:     "masters",
				Provider: "digitalocean",
				Count:    1,
				Roles:    []string{"master", "etcd"},
				Size:     "s-2vcpu-4gb",
				Region:   "nyc3",
			},
			"workers": {
				Name:     "workers",
				Provider: "digitalocean",
				Count:    2,
				Roles:    []string{"worker"},
				Size:     "s-2vcpu-4gb",
				Region:   "nyc3",
			},
		},
		Kubernetes: config.KubernetesConfig{
			Distribution: "k3s",
			Version:      "v1.29.0",
		},
	}

	lispManifest := `(cluster
  (metadata (name "minimal-cluster") (environment "development"))
  (providers (digitalocean (enabled true) (region "nyc3")))
  (node-pools
    (masters (name "masters") (count 1) (roles master etcd))
    (workers (name "workers") (count 2) (roles worker))))`

	// Initial deployment: 1 master + 2 workers = 3 nodes
	meta := generateDeploymentMetadata(cfg, "", 3, lispManifest)

	// Verify initial deployment metadata
	assert.Equal(t, "initial", meta.LastScaleOperation)
	assert.Equal(t, 1, meta.DeploymentCount)
	assert.Equal(t, 0, meta.PreviousNodeCount)
	assert.Equal(t, 3, meta.CurrentNodeCount)
	assert.NotEmpty(t, meta.CreatedAt)
	assert.NotEmpty(t, meta.ConfigChecksum)
	assert.Equal(t, string(versioning.CurrentSchema), meta.SchemaVersion)

	// Verify node pools added
	assert.Contains(t, meta.NodePoolsAdded, "masters")
	assert.Contains(t, meta.NodePoolsAdded, "workers")

	// Verify changelog has cluster_created entry
	var foundClusterCreated bool
	for _, entry := range meta.ChangeLog {
		if entry.ChangeType == "cluster_created" {
			foundClusterCreated = true
			assert.Contains(t, entry.Description, "3 nodes")
			assert.Equal(t, "minimal-cluster", entry.ResourceID)
		}
	}
	assert.True(t, foundClusterCreated, "Should have cluster_created changelog entry")
}

// TestRealScenario_MultiCloudCluster tests deployment for a multi-cloud setup
// based on examples/cluster-multi-cloud.lisp: AWS masters + AWS/Azure/DO workers
func TestRealScenario_MultiCloudCluster_InitialDeployment(t *testing.T) {
	// Configuration matching cluster-multi-cloud.lisp
	cfg := &config.ClusterConfig{
		Metadata: config.Metadata{
			Name:        "multi-cloud-cluster",
			Environment: "production",
			Labels: map[string]string{
				"project": "sloth-kubernetes",
				"type":    "multi-cloud",
			},
		},
		Providers: config.ProvidersConfig{
			AWS: &config.AWSProvider{
				Enabled: true,
				Region:  "us-east-1",
			},
			Azure: &config.AzureProvider{
				Enabled:        true,
				SubscriptionID: "sub-xxx",
				TenantID:       "tenant-xxx",
				ClientID:       "client-xxx",
				ClientSecret:   "secret-xxx",
				ResourceGroup:  "sloth-k8s-rg",
				Location:       "eastus",
			},
			DigitalOcean: &config.DigitalOceanProvider{
				Enabled: true,
				Token:   "dop_v1_xxx",
				Region:  "nyc3",
			},
		},
		Network: config.NetworkConfig{
			PodCIDR:     "10.42.0.0/16",
			ServiceCIDR: "10.43.0.0/16",
			WireGuard: &config.WireGuardConfig{
				Enabled:        true,
				SubnetCIDR:     "10.8.0.0/24",
				Port:           51820,
				MeshNetworking: true,
			},
		},
		NodePools: map[string]config.NodePool{
			"aws-masters": {
				Name:     "aws-masters",
				Provider: "aws",
				Region:   "us-east-1",
				Count:    3, // HA control plane
				Roles:    []string{"master", "etcd"},
				Size:     "t3.medium",
				Labels:   map[string]string{"cloud": "aws", "role": "control-plane"},
			},
			"aws-workers": {
				Name:         "aws-workers",
				Provider:     "aws",
				Region:       "us-east-1",
				Count:        3,
				Roles:        []string{"worker"},
				Size:         "t3.large",
				SpotInstance: true,
				Labels:       map[string]string{"cloud": "aws", "role": "worker"},
			},
			"azure-workers": {
				Name:     "azure-workers",
				Provider: "azure",
				Region:   "eastus",
				Count:    2,
				Roles:    []string{"worker"},
				Size:     "Standard_D2s_v3",
				Labels:   map[string]string{"cloud": "azure", "role": "worker"},
			},
			"do-workers": {
				Name:     "do-workers",
				Provider: "digitalocean",
				Region:   "nyc3",
				Count:    2,
				Roles:    []string{"worker"},
				Size:     "s-4vcpu-8gb",
				Labels:   map[string]string{"cloud": "digitalocean", "role": "worker"},
			},
		},
		Kubernetes: config.KubernetesConfig{
			Distribution:  "rke2",
			Version:       "v1.29.0",
			NetworkPlugin: "canal",
		},
	}

	lispManifest := `(cluster
  (metadata (name "multi-cloud-cluster") (environment "production"))
  (providers
    (aws (enabled true) (region "us-east-1"))
    (azure (enabled true) (location "eastus"))
    (digitalocean (enabled true) (region "nyc3")))
  (node-pools
    (aws-masters (count 3) (roles master etcd))
    (aws-workers (count 3) (roles worker) (spot-instance true))
    (azure-workers (count 2) (roles worker))
    (do-workers (count 2) (roles worker))))`

	// Total: 3 masters + 3 AWS workers + 2 Azure workers + 2 DO workers = 10 nodes
	meta := generateDeploymentMetadata(cfg, "", 10, lispManifest)

	assert.Equal(t, "initial", meta.LastScaleOperation)
	assert.Equal(t, 1, meta.DeploymentCount)
	assert.Equal(t, 10, meta.CurrentNodeCount)
	assert.Equal(t, 0, meta.PreviousNodeCount)

	// All 4 pools should be marked as added
	assert.Len(t, meta.NodePoolsAdded, 4)
	assert.Contains(t, meta.NodePoolsAdded, "aws-masters")
	assert.Contains(t, meta.NodePoolsAdded, "aws-workers")
	assert.Contains(t, meta.NodePoolsAdded, "azure-workers")
	assert.Contains(t, meta.NodePoolsAdded, "do-workers")
}

// TestRealScenario_ScaleUp_AddWorkers tests scaling a minimal cluster by adding workers
func TestRealScenario_ScaleUp_AddWorkers(t *testing.T) {
	cfg := &config.ClusterConfig{
		Metadata: config.Metadata{Name: "minimal-cluster"},
		NodePools: map[string]config.NodePool{
			"masters": {Name: "masters", Count: 1},
			"workers": {Name: "workers", Count: 5}, // Scaled from 2 to 5
		},
	}

	// Previous state: 1 master + 2 workers = 3 nodes
	prevMeta := DeploymentMetadata{
		CreatedAt:        "2025-01-01T00:00:00Z",
		LastDeployedAt:   "2025-01-15T00:00:00Z",
		DeploymentID:     "deploy-1-2025-01-01",
		DeploymentCount:  1,
		CurrentNodeCount: 3,
		ConfigChecksum:   "abc123",
	}
	prevJSON, _ := json.Marshal(prevMeta)

	// New state: 1 master + 5 workers = 6 nodes
	newLisp := `(cluster (node-pools (masters (count 1)) (workers (count 5))))`
	meta := generateDeploymentMetadata(cfg, string(prevJSON), 6, newLisp)

	assert.Equal(t, "scale-up", meta.LastScaleOperation)
	assert.Equal(t, 2, meta.DeploymentCount)
	assert.Equal(t, 3, meta.PreviousNodeCount)
	assert.Equal(t, 6, meta.CurrentNodeCount)
	assert.Equal(t, "2025-01-01T00:00:00Z", meta.CreatedAt) // Preserved

	// Verify scale-up changelog
	var foundScaleUp bool
	for _, entry := range meta.ChangeLog {
		if entry.ChangeType == "scale_up" {
			foundScaleUp = true
			assert.Contains(t, entry.Description, "3 to 6")
			assert.Equal(t, "3", entry.OldValue)
			assert.Equal(t, "6", entry.NewValue)
		}
	}
	assert.True(t, foundScaleUp, "Should have scale_up changelog entry")
}

// TestRealScenario_ScaleDown_RemoveWorkers tests scaling down by removing workers
func TestRealScenario_ScaleDown_RemoveWorkers(t *testing.T) {
	cfg := &config.ClusterConfig{
		Metadata: config.Metadata{Name: "production-cluster"},
		NodePools: map[string]config.NodePool{
			"masters": {Name: "masters", Count: 3},
			"workers": {Name: "workers", Count: 2}, // Scaled from 5 to 2
		},
	}

	// Previous: 3 masters + 5 workers = 8 nodes
	prevMeta := DeploymentMetadata{
		CreatedAt:        "2024-06-01T00:00:00Z",
		LastDeployedAt:   "2025-01-01T00:00:00Z",
		DeploymentID:     "deploy-5-2025-01-01",
		DeploymentCount:  5,
		CurrentNodeCount: 8,
		ConfigChecksum:   "previous-checksum",
	}
	prevJSON, _ := json.Marshal(prevMeta)

	// New: 3 masters + 2 workers = 5 nodes (cost optimization)
	newLisp := `(cluster (node-pools (masters (count 3)) (workers (count 2))))`
	meta := generateDeploymentMetadata(cfg, string(prevJSON), 5, newLisp)

	assert.Equal(t, "scale-down", meta.LastScaleOperation)
	assert.Equal(t, 6, meta.DeploymentCount)
	assert.Equal(t, 8, meta.PreviousNodeCount)
	assert.Equal(t, 5, meta.CurrentNodeCount)

	// Verify scale-down changelog
	var foundScaleDown bool
	for _, entry := range meta.ChangeLog {
		if entry.ChangeType == "scale_down" {
			foundScaleDown = true
			assert.Contains(t, entry.Description, "8 to 5")
		}
	}
	assert.True(t, foundScaleDown, "Should have scale_down changelog entry")
}

// TestRealScenario_ConfigUpdate_NoScaleChange tests updating config without node changes
func TestRealScenario_ConfigUpdate_NoScaleChange(t *testing.T) {
	cfg := &config.ClusterConfig{
		Metadata: config.Metadata{Name: "production-cluster"},
		Kubernetes: config.KubernetesConfig{
			Version: "v1.30.0", // Upgraded from v1.29.0
		},
	}

	prevMeta := DeploymentMetadata{
		CreatedAt:        "2024-01-01T00:00:00Z",
		DeploymentCount:  10,
		CurrentNodeCount: 5,
		ConfigChecksum:   "old-config-checksum",
	}
	prevJSON, _ := json.Marshal(prevMeta)

	// Same node count, different config
	newLisp := `(cluster (kubernetes (version "v1.30.0")))`
	meta := generateDeploymentMetadata(cfg, string(prevJSON), 5, newLisp)

	assert.Equal(t, "update", meta.LastScaleOperation)
	assert.Equal(t, 11, meta.DeploymentCount)
	assert.Equal(t, 5, meta.PreviousNodeCount)
	assert.Equal(t, 5, meta.CurrentNodeCount)

	// Should have config_updated changelog since checksum changed
	var foundConfigUpdate bool
	for _, entry := range meta.ChangeLog {
		if entry.ChangeType == "config_updated" {
			foundConfigUpdate = true
		}
	}
	assert.True(t, foundConfigUpdate, "Should detect config change via checksum")
}

// TestRealScenario_SanitizeMinimalCluster tests sanitization of minimal cluster config
func TestRealScenario_SanitizeMinimalCluster(t *testing.T) {
	// Real config based on cluster-minimal.lisp
	cfg := &config.ClusterConfig{
		Metadata: config.Metadata{
			Name:        "minimal-cluster",
			Environment: "development",
		},
		Providers: config.ProvidersConfig{
			DigitalOcean: &config.DigitalOceanProvider{
				Enabled: true,
				Token:   "dop_v1_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0", // Real token format
				Region:  "nyc3",
			},
		},
		Network: config.NetworkConfig{
			WireGuard: &config.WireGuardConfig{
				Enabled:        true,
				MeshNetworking: true,
			},
		},
		NodePools: map[string]config.NodePool{
			"masters": {Name: "masters", Provider: "digitalocean", Count: 1, Size: "s-2vcpu-4gb"},
			"workers": {Name: "workers", Provider: "digitalocean", Count: 2, Size: "s-2vcpu-4gb"},
		},
	}

	sanitized := sanitizeConfigForStorage(cfg)

	// Token replaced
	assert.Equal(t, "${DIGITALOCEAN_TOKEN}", sanitized.Providers.DigitalOcean.Token)

	// All other config preserved exactly
	assert.Equal(t, "minimal-cluster", sanitized.Metadata.Name)
	assert.Equal(t, "development", sanitized.Metadata.Environment)
	assert.Equal(t, "nyc3", sanitized.Providers.DigitalOcean.Region)
	assert.True(t, sanitized.Providers.DigitalOcean.Enabled)
	assert.True(t, sanitized.Network.WireGuard.Enabled)
	assert.True(t, sanitized.Network.WireGuard.MeshNetworking)
	assert.Len(t, sanitized.NodePools, 2)
	assert.Equal(t, 1, sanitized.NodePools["masters"].Count)
	assert.Equal(t, 2, sanitized.NodePools["workers"].Count)

	// Original not mutated
	assert.Contains(t, cfg.Providers.DigitalOcean.Token, "dop_v1_")
}

// TestRealScenario_SanitizeMultiCloudCluster tests sanitization of multi-cloud config
func TestRealScenario_SanitizeMultiCloudCluster(t *testing.T) {
	cfg := &config.ClusterConfig{
		Metadata: config.Metadata{
			Name:        "multi-cloud-cluster",
			Environment: "production",
		},
		Providers: config.ProvidersConfig{
			AWS: &config.AWSProvider{
				Enabled:         true,
				Region:          "us-east-1",
				AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
				SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			},
			Azure: &config.AzureProvider{
				Enabled:        true,
				Location:       "eastus",
				SubscriptionID: "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
				TenantID:       "t1e2n3a4-n5t6-7890-abcd-ef1234567890",
				ClientID:       "c1l2i3e4-n5t6-7890-abcd-ef1234567890",
				ClientSecret:   "azure~secret~P@ssw0rd!",
				ResourceGroup:  "sloth-k8s-rg",
			},
			DigitalOcean: &config.DigitalOceanProvider{
				Enabled: true,
				Token:   "dop_v1_production_token_value",
				Region:  "nyc3",
			},
		},
		NodePools: map[string]config.NodePool{
			"aws-masters":   {Provider: "aws", Count: 3},
			"aws-workers":   {Provider: "aws", Count: 3, SpotInstance: true},
			"azure-workers": {Provider: "azure", Count: 2},
			"do-workers":    {Provider: "digitalocean", Count: 2},
		},
	}

	sanitized := sanitizeConfigForStorage(cfg)

	// ALL sensitive tokens replaced
	assert.Equal(t, "${AWS_ACCESS_KEY_ID}", sanitized.Providers.AWS.AccessKeyID)
	assert.Equal(t, "${AWS_SECRET_ACCESS_KEY}", sanitized.Providers.AWS.SecretAccessKey)
	assert.Equal(t, "${AZURE_SUBSCRIPTION_ID}", sanitized.Providers.Azure.SubscriptionID)
	assert.Equal(t, "${AZURE_TENANT_ID}", sanitized.Providers.Azure.TenantID)
	assert.Equal(t, "${AZURE_CLIENT_ID}", sanitized.Providers.Azure.ClientID)
	assert.Equal(t, "${AZURE_CLIENT_SECRET}", sanitized.Providers.Azure.ClientSecret)
	assert.Equal(t, "${DIGITALOCEAN_TOKEN}", sanitized.Providers.DigitalOcean.Token)

	// Non-sensitive preserved
	assert.Equal(t, "us-east-1", sanitized.Providers.AWS.Region)
	assert.Equal(t, "eastus", sanitized.Providers.Azure.Location)
	assert.Equal(t, "sloth-k8s-rg", sanitized.Providers.Azure.ResourceGroup)
	assert.Equal(t, "nyc3", sanitized.Providers.DigitalOcean.Region)

	// Node pools preserved with spot instance config
	assert.True(t, sanitized.NodePools["aws-workers"].SpotInstance)
	assert.Equal(t, 3, sanitized.NodePools["aws-masters"].Count)
}

// TestRealScenario_DeploymentHistory_Production tests deployment history tracking
// Simulates a production cluster with multiple deployments over time
func TestRealScenario_DeploymentHistory_Production(t *testing.T) {
	cfg := &config.ClusterConfig{
		Metadata: config.Metadata{Name: "prod-cluster"},
	}

	// Simulate 5 previous deployments (typical production cluster)
	prevMeta := DeploymentMetadata{
		CreatedAt:        "2024-01-15T10:00:00Z", // Cluster created Jan 15
		LastDeployedAt:   "2025-01-20T14:30:00Z", // Last deploy Jan 20
		DeploymentID:     "deploy-5-2025-01-20",
		DeploymentCount:  5,
		CurrentNodeCount: 8,
		ConfigChecksum:   "checksum-v5",
		SchemaVersion:    "2.0",
		PreviousDeployments: []DeploymentHistoryEntry{
			{DeploymentID: "deploy-4-2025-01-10", Timestamp: "2025-01-10T09:00:00Z", NodeCount: 6, Success: true},
			{DeploymentID: "deploy-3-2024-12-01", Timestamp: "2024-12-01T11:00:00Z", NodeCount: 5, Success: true},
			{DeploymentID: "deploy-2-2024-06-15", Timestamp: "2024-06-15T08:00:00Z", NodeCount: 3, Success: true},
			{DeploymentID: "deploy-1-2024-01-15", Timestamp: "2024-01-15T10:00:00Z", NodeCount: 3, Success: true},
		},
	}
	prevJSON, _ := json.Marshal(prevMeta)

	// New deployment: scale to 10 nodes
	meta := generateDeploymentMetadata(cfg, string(prevJSON), 10, "(cluster)")

	// Should be deployment #6
	assert.Equal(t, 6, meta.DeploymentCount)
	assert.Equal(t, "scale-up", meta.LastScaleOperation)

	// History should have 5 entries (current prev becomes first in history)
	assert.Len(t, meta.PreviousDeployments, 5)
	assert.Equal(t, "deploy-5-2025-01-20", meta.PreviousDeployments[0].DeploymentID)
	assert.Equal(t, 8, meta.PreviousDeployments[0].NodeCount)

	// Original creation date preserved
	assert.Equal(t, "2024-01-15T10:00:00Z", meta.CreatedAt)
}

// TestRealScenario_StateSnapshot_Linking tests state snapshot ID linking
func TestRealScenario_StateSnapshot_Linking(t *testing.T) {
	cfg := &config.ClusterConfig{
		Metadata: config.Metadata{Name: "stateful-cluster"},
	}

	// First deployment
	meta1 := generateDeploymentMetadata(cfg, "", 3, "(cluster v1)")
	assert.NotEmpty(t, meta1.StateSnapshotID)
	assert.Empty(t, meta1.ParentStateID)
	assert.Contains(t, meta1.StateSnapshotID, "state-deploy-1")

	// Second deployment - should link to first
	meta1JSON, _ := json.Marshal(meta1)
	meta2 := generateDeploymentMetadata(cfg, string(meta1JSON), 5, "(cluster v2)")

	assert.NotEmpty(t, meta2.StateSnapshotID)
	assert.Equal(t, meta1.StateSnapshotID, meta2.ParentStateID)
	assert.Contains(t, meta2.StateSnapshotID, "state-deploy-2")

	// Third deployment - should link to second
	meta2JSON, _ := json.Marshal(meta2)
	meta3 := generateDeploymentMetadata(cfg, string(meta2JSON), 5, "(cluster v3)")

	assert.Equal(t, meta2.StateSnapshotID, meta3.ParentStateID)
}

// TestRealScenario_ChecksumDeterminism tests that same config produces same checksum
func TestRealScenario_ChecksumDeterminism(t *testing.T) {
	// Real Lisp manifest content
	lispManifest := `(cluster
  (metadata
    (name "test-cluster")
    (environment "production"))
  (providers
    (aws (enabled true) (region "us-east-1"))
    (digitalocean (enabled true) (region "nyc3")))
  (node-pools
    (masters (count 3) (roles master etcd))
    (workers (count 5) (roles worker))))`

	checksum1 := generateSHA256Checksum(lispManifest)
	checksum2 := generateSHA256Checksum(lispManifest)
	checksum3 := generateSHA256Checksum(lispManifest)

	// Same input always produces same checksum
	assert.Equal(t, checksum1, checksum2)
	assert.Equal(t, checksum2, checksum3)

	// Different input produces different checksum
	modifiedManifest := strings.Replace(lispManifest, "count 5", "count 6", 1)
	checksumModified := generateSHA256Checksum(modifiedManifest)
	assert.NotEqual(t, checksum1, checksumModified)

	// Checksum format: 64 hex characters (SHA256)
	assert.Len(t, checksum1, 64)
	assert.Regexp(t, "^[a-f0-9]{64}$", checksum1)
}

// TestRealScenario_ConfigVersionTracking tests config version with parent hash
func TestRealScenario_ConfigVersionTracking(t *testing.T) {
	cfg := &config.ClusterConfig{
		Metadata: config.Metadata{Name: "versioned-cluster"},
	}

	// Initial deployment
	meta1 := generateDeploymentMetadata(cfg, "", 3, "(cluster initial)")
	require.NotNil(t, meta1.ConfigVersion)
	assert.NotEmpty(t, meta1.ConfigVersion.ConfigHash)
	assert.Empty(t, meta1.ConfigVersion.ParentHash) // No parent for initial

	// Second deployment
	meta1JSON, _ := json.Marshal(meta1)
	meta2 := generateDeploymentMetadata(cfg, string(meta1JSON), 5, "(cluster updated)")

	require.NotNil(t, meta2.ConfigVersion)
	assert.NotEmpty(t, meta2.ConfigVersion.ConfigHash)
	assert.Equal(t, meta1.ConfigVersion.ConfigHash, meta2.ConfigVersion.ParentHash)
}

// TestRealScenario_ChangeLogEntries_AllTypes tests that all change types are properly logged
func TestRealScenario_ChangeLogEntries_AllTypes(t *testing.T) {
	cfg := &config.ClusterConfig{
		Metadata: config.Metadata{Name: "changelog-cluster"},
		NodePools: map[string]config.NodePool{
			"masters": {Count: 3},
			"workers": {Count: 5},
		},
	}

	// Previous state with different config
	prevMeta := DeploymentMetadata{
		CreatedAt:        "2024-01-01T00:00:00Z",
		DeploymentCount:  3,
		CurrentNodeCount: 5, // Will scale to 8
		ConfigChecksum:   "old-checksum",
	}
	prevJSON, _ := json.Marshal(prevMeta)

	meta := generateDeploymentMetadata(cfg, string(prevJSON), 8, "(cluster new)")

	// Should have multiple changelog entries
	changeTypes := make(map[string]bool)
	for _, entry := range meta.ChangeLog {
		changeTypes[entry.ChangeType] = true
		// All entries should have timestamp and actor
		assert.NotEmpty(t, entry.Timestamp)
		assert.Equal(t, "sloth-kubernetes", entry.Actor)
	}

	// Should have scale_up (5->8) and config_updated (checksum changed)
	assert.True(t, changeTypes["scale_up"], "Should have scale_up entry")
	assert.True(t, changeTypes["config_updated"], "Should have config_updated entry")
}
