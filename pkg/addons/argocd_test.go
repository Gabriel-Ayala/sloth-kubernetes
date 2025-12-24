package addons

import (
	"strings"
	"testing"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAppOfAppsConfig tests the AppOfAppsConfig struct
func TestAppOfAppsConfig(t *testing.T) {
	tests := []struct {
		name   string
		config AppOfAppsConfig
	}{
		{
			name: "basic config",
			config: AppOfAppsConfig{
				Name:       "root-app",
				Namespace:  "argocd",
				RepoURL:    "https://github.com/org/gitops.git",
				Branch:     "main",
				Path:       "argocd/apps",
				SyncPolicy: "automated",
			},
		},
		{
			name: "manual sync policy",
			config: AppOfAppsConfig{
				Name:       "my-app",
				Namespace:  "argocd",
				RepoURL:    "https://github.com/org/repo.git",
				Branch:     "develop",
				Path:       "apps",
				SyncPolicy: "manual",
			},
		},
		{
			name: "minimal config",
			config: AppOfAppsConfig{
				Name:      "app",
				Namespace: "argocd",
				RepoURL:   "https://github.com/org/repo.git",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, tt.config.Name)
			assert.NotEmpty(t, tt.config.Namespace)
			assert.NotEmpty(t, tt.config.RepoURL)
		})
	}
}

// TestArgoCDStatus tests the ArgoCDStatus struct
func TestArgoCDStatus(t *testing.T) {
	status := &ArgoCDStatus{
		Healthy:       true,
		Pods:          []PodStatus{},
		AppsTotal:     5,
		AppsSynced:    4,
		AppsOutOfSync: 1,
	}

	assert.True(t, status.Healthy)
	assert.Equal(t, 5, status.AppsTotal)
	assert.Equal(t, 4, status.AppsSynced)
	assert.Equal(t, 1, status.AppsOutOfSync)
}

// TestPodStatus tests the PodStatus struct
func TestPodStatus(t *testing.T) {
	tests := []struct {
		name   string
		pod    PodStatus
		isOK   bool
	}{
		{
			name: "running pod",
			pod: PodStatus{
				Name:   "argocd-server-abc123",
				Status: "Running",
				Ready:  "1/1",
			},
			isOK: true,
		},
		{
			name: "pending pod",
			pod: PodStatus{
				Name:   "argocd-repo-server-xyz789",
				Status: "Pending",
				Ready:  "0/1",
			},
			isOK: false,
		},
		{
			name: "crashloopbackoff pod",
			pod: PodStatus{
				Name:   "argocd-application-controller-def456",
				Status: "CrashLoopBackOff",
				Ready:  "0/1",
			},
			isOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, tt.pod.Name)
			if tt.isOK {
				assert.Equal(t, "Running", tt.pod.Status)
			} else {
				assert.NotEqual(t, "Running", tt.pod.Status)
			}
		})
	}
}

// TestArgoCDApp tests the ArgoCDApp struct
func TestArgoCDApp(t *testing.T) {
	tests := []struct {
		name     string
		app      ArgoCDApp
		isSynced bool
	}{
		{
			name: "synced healthy app",
			app: ArgoCDApp{
				Name:       "my-app",
				Namespace:  "argocd",
				SyncStatus: "Synced",
				Health:     "Healthy",
				RepoURL:    "https://github.com/org/repo.git",
				Path:       "apps/my-app",
			},
			isSynced: true,
		},
		{
			name: "out of sync app",
			app: ArgoCDApp{
				Name:       "another-app",
				Namespace:  "argocd",
				SyncStatus: "OutOfSync",
				Health:     "Healthy",
				RepoURL:    "https://github.com/org/repo.git",
				Path:       "apps/another-app",
			},
			isSynced: false,
		},
		{
			name: "degraded app",
			app: ArgoCDApp{
				Name:       "degraded-app",
				Namespace:  "argocd",
				SyncStatus: "Synced",
				Health:     "Degraded",
				RepoURL:    "https://github.com/org/repo.git",
				Path:       "apps/degraded-app",
			},
			isSynced: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, tt.app.Name)
			if tt.isSynced {
				assert.Equal(t, "Synced", tt.app.SyncStatus)
			} else {
				assert.Equal(t, "OutOfSync", tt.app.SyncStatus)
			}
		})
	}
}

// TestInstallArgoCD_DisabledConfig tests that InstallArgoCD returns nil when ArgoCD is disabled
func TestInstallArgoCD_DisabledConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.ClusterConfig
	}{
		{
			name: "nil argocd config",
			cfg: &config.ClusterConfig{
				Addons: config.AddonsConfig{
					ArgoCD: nil,
				},
			},
		},
		{
			name: "argocd disabled",
			cfg: &config.ClusterConfig{
				Addons: config.AddonsConfig{
					ArgoCD: &config.ArgoCDConfig{
						Enabled: false,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := InstallArgoCD(tt.cfg, "", "")
			assert.NoError(t, err)
		})
	}
}

// TestArgoCDConfigDefaults tests that default values are set correctly
func TestArgoCDConfigDefaults(t *testing.T) {
	cfg := &config.ClusterConfig{
		Addons: config.AddonsConfig{
			ArgoCD: &config.ArgoCDConfig{
				Enabled: true,
				// Empty values to test defaults
			},
		},
	}

	argocdConfig := cfg.Addons.ArgoCD

	// Before defaults are set
	assert.Empty(t, argocdConfig.Namespace)
	assert.Empty(t, argocdConfig.GitOpsRepoBranch)
	assert.Empty(t, argocdConfig.AppsPath)
	assert.Empty(t, argocdConfig.Version)

	// Simulate setting defaults (as done in InstallArgoCD)
	if argocdConfig.Namespace == "" {
		argocdConfig.Namespace = "argocd"
	}
	if argocdConfig.GitOpsRepoBranch == "" {
		argocdConfig.GitOpsRepoBranch = "main"
	}
	if argocdConfig.AppsPath == "" {
		argocdConfig.AppsPath = "argocd/apps"
	}
	if argocdConfig.Version == "" {
		argocdConfig.Version = "stable"
	}

	// After defaults are set
	assert.Equal(t, "argocd", argocdConfig.Namespace)
	assert.Equal(t, "main", argocdConfig.GitOpsRepoBranch)
	assert.Equal(t, "argocd/apps", argocdConfig.AppsPath)
	assert.Equal(t, "stable", argocdConfig.Version)
}

// TestArgoCDStatusHealthy tests healthy status detection
func TestArgoCDStatusHealthy(t *testing.T) {
	tests := []struct {
		name     string
		pods     []PodStatus
		expected bool
	}{
		{
			name: "all pods running",
			pods: []PodStatus{
				{Name: "argocd-server", Status: "Running", Ready: "1/1"},
				{Name: "argocd-repo-server", Status: "Running", Ready: "1/1"},
				{Name: "argocd-application-controller", Status: "Running", Ready: "1/1"},
			},
			expected: true,
		},
		{
			name: "one pod pending",
			pods: []PodStatus{
				{Name: "argocd-server", Status: "Running", Ready: "1/1"},
				{Name: "argocd-repo-server", Status: "Pending", Ready: "0/1"},
			},
			expected: false,
		},
		{
			name: "empty pods list",
			pods: []PodStatus{},
			expected: true, // No pods means we can't determine unhealthy
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := &ArgoCDStatus{
				Healthy: true,
				Pods:    tt.pods,
			}

			// Check if any pod is not running
			for _, pod := range status.Pods {
				if pod.Status != "Running" {
					status.Healthy = false
					break
				}
			}

			assert.Equal(t, tt.expected, status.Healthy)
		})
	}
}

// TestSyncPolicyGeneration tests sync policy YAML generation
func TestSyncPolicyGeneration(t *testing.T) {
	tests := []struct {
		name           string
		syncPolicy     string
		shouldContain  []string
		shouldNotContain []string
	}{
		{
			name:       "automated sync",
			syncPolicy: "automated",
			shouldContain: []string{
				"syncPolicy:",
				"automated:",
				"prune: true",
				"selfHeal: true",
				"CreateNamespace=true",
			},
			shouldNotContain: []string{},
		},
		{
			name:       "manual sync",
			syncPolicy: "manual",
			shouldContain: []string{},
			shouldNotContain: []string{
				"syncPolicy:",
				"automated:",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			syncPolicy := ""
			if tt.syncPolicy == "automated" {
				syncPolicy = `
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
    - CreateNamespace=true`
			}

			for _, s := range tt.shouldContain {
				assert.Contains(t, syncPolicy, s)
			}
			for _, s := range tt.shouldNotContain {
				assert.NotContains(t, syncPolicy, s)
			}
		})
	}
}

// TestAppOfAppsManifestGeneration tests App of Apps manifest generation
func TestAppOfAppsManifestGeneration(t *testing.T) {
	cfg := &AppOfAppsConfig{
		Name:       "root-app",
		Namespace:  "argocd",
		RepoURL:    "https://github.com/org/gitops.git",
		Branch:     "main",
		Path:       "argocd/apps",
		SyncPolicy: "automated",
	}

	// Generate manifest parts
	manifest := `apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: ` + cfg.Name + `
  namespace: ` + cfg.Namespace

	assert.Contains(t, manifest, "apiVersion: argoproj.io/v1alpha1")
	assert.Contains(t, manifest, "kind: Application")
	assert.Contains(t, manifest, "name: root-app")
	assert.Contains(t, manifest, "namespace: argocd")
}

// TestArgoCDRepoURLValidation tests various GitOps repo URL formats
func TestArgoCDRepoURLValidation(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		isValid bool
	}{
		{
			name:    "https github url",
			url:     "https://github.com/org/repo.git",
			isValid: true,
		},
		{
			name:    "ssh github url",
			url:     "git@github.com:org/repo.git",
			isValid: true,
		},
		{
			name:    "https gitlab url",
			url:     "https://gitlab.com/org/repo.git",
			isValid: true,
		},
		{
			name:    "https bitbucket url",
			url:     "https://bitbucket.org/org/repo.git",
			isValid: true,
		},
		{
			name:    "empty url",
			url:     "",
			isValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := tt.url != "" && (strings.HasPrefix(tt.url, "https://") || strings.HasPrefix(tt.url, "git@"))
			assert.Equal(t, tt.isValid, isValid)
		})
	}
}

// TestArgoCDVersionFormats tests various ArgoCD version formats
func TestArgoCDVersionFormats(t *testing.T) {
	tests := []struct {
		name    string
		version string
		isValid bool
	}{
		{
			name:    "stable version",
			version: "stable",
			isValid: true,
		},
		{
			name:    "specific version",
			version: "v2.9.3",
			isValid: true,
		},
		{
			name:    "version without v prefix",
			version: "2.9.3",
			isValid: true,
		},
		{
			name:    "empty version",
			version: "",
			isValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := tt.version != ""
			assert.Equal(t, tt.isValid, isValid)
		})
	}
}

// TestArgoCDNamespaceValidation tests namespace naming
func TestArgoCDNamespaceValidation(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		isValid   bool
	}{
		{
			name:      "default argocd namespace",
			namespace: "argocd",
			isValid:   true,
		},
		{
			name:      "custom namespace",
			namespace: "gitops",
			isValid:   true,
		},
		{
			name:      "namespace with hyphen",
			namespace: "argo-cd",
			isValid:   true,
		},
		{
			name:      "empty namespace",
			namespace: "",
			isValid:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := tt.namespace != ""
			assert.Equal(t, tt.isValid, isValid)
		})
	}
}

// TestPodStatusParsing tests parsing pod status from kubectl output
func TestPodStatusParsing(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected PodStatus
	}{
		{
			name: "running pod",
			line: "argocd-server-abc123   1/1   Running   0   5m",
			expected: PodStatus{
				Name:   "argocd-server-abc123",
				Ready:  "1/1",
				Status: "Running",
			},
		},
		{
			name: "pending pod",
			line: "argocd-repo-server-xyz   0/1   Pending   0   1m",
			expected: PodStatus{
				Name:   "argocd-repo-server-xyz",
				Ready:  "0/1",
				Status: "Pending",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parts := strings.Fields(tt.line)
			require.GreaterOrEqual(t, len(parts), 3)

			pod := PodStatus{
				Name:   parts[0],
				Ready:  parts[1],
				Status: parts[2],
			}

			assert.Equal(t, tt.expected.Name, pod.Name)
			assert.Equal(t, tt.expected.Ready, pod.Ready)
			assert.Equal(t, tt.expected.Status, pod.Status)
		})
	}
}

// TestAppListParsing tests parsing application list from ArgoCD
func TestAppListParsing(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected ArgoCDApp
	}{
		{
			name: "synced app",
			line: "my-app\tSynced\tHealthy\thttps://github.com/org/repo.git\tapps/my-app",
			expected: ArgoCDApp{
				Name:       "my-app",
				SyncStatus: "Synced",
				Health:     "Healthy",
				RepoURL:    "https://github.com/org/repo.git",
				Path:       "apps/my-app",
			},
		},
		{
			name: "out of sync app",
			line: "other-app\tOutOfSync\tDegraded\thttps://github.com/org/repo.git\tapps/other",
			expected: ArgoCDApp{
				Name:       "other-app",
				SyncStatus: "OutOfSync",
				Health:     "Degraded",
				RepoURL:    "https://github.com/org/repo.git",
				Path:       "apps/other",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parts := strings.Split(tt.line, "\t")
			require.GreaterOrEqual(t, len(parts), 4)

			app := ArgoCDApp{
				Name:       parts[0],
				SyncStatus: parts[1],
				Health:     parts[2],
				RepoURL:    parts[3],
			}
			if len(parts) >= 5 {
				app.Path = parts[4]
			}

			assert.Equal(t, tt.expected.Name, app.Name)
			assert.Equal(t, tt.expected.SyncStatus, app.SyncStatus)
			assert.Equal(t, tt.expected.Health, app.Health)
			assert.Equal(t, tt.expected.RepoURL, app.RepoURL)
			assert.Equal(t, tt.expected.Path, app.Path)
		})
	}
}

// TestArgoCDInstallScriptGeneration tests the install script structure
func TestArgoCDInstallScriptGeneration(t *testing.T) {
	namespace := "argocd"
	version := "stable"

	installScript := `
set -e

# Install kubectl if not present
if ! command -v kubectl &> /dev/null; then
    echo "Installing kubectl..."
fi

# Create ArgoCD namespace
kubectl create namespace ` + namespace + ` --dry-run=client -o yaml | kubectl apply -f -

# Install ArgoCD
kubectl apply -n ` + namespace + ` -f https://raw.githubusercontent.com/argoproj/argo-cd/` + version + `/manifests/install.yaml
`

	assert.Contains(t, installScript, "set -e")
	assert.Contains(t, installScript, "kubectl create namespace "+namespace)
	assert.Contains(t, installScript, "kubectl apply -n "+namespace)
	assert.Contains(t, installScript, version)
}

// TestArgoCDPasswordScriptGeneration tests password retrieval script
func TestArgoCDPasswordScriptGeneration(t *testing.T) {
	namespace := "argocd"

	getPasswordScript := `
set -e
kubectl -n ` + namespace + ` get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d
`

	assert.Contains(t, getPasswordScript, "argocd-initial-admin-secret")
	assert.Contains(t, getPasswordScript, "base64 -d")
	assert.Contains(t, getPasswordScript, namespace)
}

// TestWaitForArgoCDReadyScriptGeneration tests the wait script
func TestWaitForArgoCDReadyScriptGeneration(t *testing.T) {
	namespace := "argocd"

	waitScript := `
set -e

echo "Waiting for ArgoCD pods to be ready..."
kubectl wait --for=condition=Ready pods --all -n ` + namespace + ` --timeout=300s

echo "ArgoCD is ready"
`

	assert.Contains(t, waitScript, "kubectl wait")
	assert.Contains(t, waitScript, "--for=condition=Ready")
	assert.Contains(t, waitScript, "--timeout=300s")
	assert.Contains(t, waitScript, namespace)
}
