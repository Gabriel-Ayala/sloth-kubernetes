package addons

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
)

// ApplyArgoCDDefaults applies default values to ArgoCD configuration
func ApplyArgoCDDefaults(argocdConfig *config.ArgoCDConfig) {
	if argocdConfig == nil {
		return
	}
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
}

// InstallArgoCD installs ArgoCD and applies GitOps applications
func InstallArgoCD(cfg *config.ClusterConfig, masterNodeIP string, sshPrivateKey string) error {
	if cfg.Addons.ArgoCD == nil || !cfg.Addons.ArgoCD.Enabled {
		return nil // ArgoCD not enabled, skip
	}

	argocdConfig := cfg.Addons.ArgoCD

	// Set defaults
	ApplyArgoCDDefaults(argocdConfig)

	fmt.Println()
	fmt.Println("════════════════════════════════════════════════════════════")
	fmt.Println("🚀 Installing ArgoCD GitOps")
	fmt.Println("════════════════════════════════════════════════════════════")
	fmt.Println()

	// Step 1: Install ArgoCD
	fmt.Println("📦 Step 1: Installing ArgoCD...")
	if err := installArgoCDManifests(masterNodeIP, sshPrivateKey, argocdConfig); err != nil {
		return fmt.Errorf("failed to install ArgoCD: %w", err)
	}

	// Step 2: Wait for ArgoCD to be ready
	fmt.Println("⏳ Step 2: Waiting for ArgoCD to be ready...")
	if err := waitForArgoCDReady(masterNodeIP, sshPrivateKey, argocdConfig.Namespace); err != nil {
		return fmt.Errorf("failed to wait for ArgoCD: %w", err)
	}

	// Step 3: Clone GitOps repo and apply applications
	fmt.Println("📂 Step 3: Applying GitOps applications from repository...")
	if err := applyGitOpsApplications(masterNodeIP, sshPrivateKey, argocdConfig); err != nil {
		return fmt.Errorf("failed to apply GitOps applications: %w", err)
	}

	// Step 4: Get ArgoCD admin password
	fmt.Println()
	fmt.Println("✅ ArgoCD installation completed successfully!")
	fmt.Println()

	if argocdConfig.AdminPassword == "" {
		password, err := getArgoCDAdminPassword(masterNodeIP, sshPrivateKey, argocdConfig.Namespace)
		if err != nil {
			fmt.Printf("⚠️  Warning: Could not retrieve ArgoCD admin password: %v\n", err)
		} else {
			fmt.Println("═══════════════════════════════════════════════════════")
			fmt.Println("🔐 ArgoCD Access Information")
			fmt.Println("═══════════════════════════════════════════════════════")
			fmt.Printf("  Username: admin\n")
			fmt.Printf("  Password: %s\n", password)
			fmt.Println()
			fmt.Println("  To access ArgoCD UI, port-forward:")
			fmt.Printf("  kubectl port-forward svc/argocd-server -n %s 8080:443\n", argocdConfig.Namespace)
			fmt.Println("  Then access: https://localhost:8080")
			fmt.Println("═══════════════════════════════════════════════════════")
		}
	}

	fmt.Println()
	fmt.Printf("📋 GitOps Repository: %s\n", argocdConfig.GitOpsRepoURL)
	fmt.Printf("📂 Applications Path: %s\n", argocdConfig.AppsPath)
	fmt.Println()

	return nil
}

// installArgoCDManifests installs ArgoCD using official manifests
func installArgoCDManifests(masterNodeIP string, sshPrivateKey string, argocdConfig *config.ArgoCDConfig) error {
	installScript := fmt.Sprintf(`
set -e

# Set kubeconfig for RKE2
export KUBECONFIG=/etc/rancher/rke2/rke2.yaml

# Install kubectl if not present (use RKE2's kubectl if available)
if [ -f /var/lib/rancher/rke2/bin/kubectl ]; then
    alias kubectl='/var/lib/rancher/rke2/bin/kubectl'
    echo "Using RKE2 kubectl"
elif ! command -v kubectl &> /dev/null; then
    echo "Installing kubectl..."
    curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
    install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl
    rm kubectl
    echo "kubectl installed successfully"
else
    echo "kubectl already installed"
fi

# Use the correct kubectl path
KUBECTL="/var/lib/rancher/rke2/bin/kubectl"
if [ ! -f "$KUBECTL" ]; then
    KUBECTL="kubectl"
fi

# Create ArgoCD namespace
$KUBECTL --kubeconfig=/etc/rancher/rke2/rke2.yaml create namespace %s --dry-run=client -o yaml | $KUBECTL --kubeconfig=/etc/rancher/rke2/rke2.yaml apply -f -

# Install ArgoCD
$KUBECTL --kubeconfig=/etc/rancher/rke2/rke2.yaml apply -n %s -f https://raw.githubusercontent.com/argoproj/argo-cd/%s/manifests/install.yaml

echo "ArgoCD installed successfully"
`, argocdConfig.Namespace, argocdConfig.Namespace, argocdConfig.Version)

	return runSSHCommand(masterNodeIP, sshPrivateKey, installScript)
}

// waitForArgoCDReady waits for ArgoCD pods to be ready
func waitForArgoCDReady(masterNodeIP string, sshPrivateKey string, namespace string) error {
	waitScript := fmt.Sprintf(`
set -e

export KUBECONFIG=/etc/rancher/rke2/rke2.yaml
KUBECTL="/var/lib/rancher/rke2/bin/kubectl"
if [ ! -f "$KUBECTL" ]; then
    KUBECTL="kubectl"
fi

echo "Waiting for ArgoCD pods to be ready..."
$KUBECTL --kubeconfig=/etc/rancher/rke2/rke2.yaml wait --for=condition=Ready pods --all -n %s --timeout=300s

echo "ArgoCD is ready"
`, namespace)

	return runSSHCommand(masterNodeIP, sshPrivateKey, waitScript)
}

// applyGitOpsApplications clones the GitOps repo and applies application manifests
func applyGitOpsApplications(masterNodeIP string, sshPrivateKey string, argocdConfig *config.ArgoCDConfig) error {
	applyScript := fmt.Sprintf(`
set -e

export KUBECONFIG=/etc/rancher/rke2/rke2.yaml
KUBECTL="/var/lib/rancher/rke2/bin/kubectl"
if [ ! -f "$KUBECTL" ]; then
    KUBECTL="kubectl"
fi

# Create temporary directory for GitOps repo
TEMP_DIR=$(mktemp -d)

# Ensure cleanup happens even on error
trap "cd / && rm -rf $TEMP_DIR" EXIT

cd $TEMP_DIR

# Clone the GitOps repository
echo "Cloning GitOps repository: %s"
git clone -b %s %s gitops-repo

# Apply all YAML files from the apps path
if [ -d "gitops-repo/%s" ]; then
	echo "Applying applications from %s..."
	$KUBECTL --kubeconfig=/etc/rancher/rke2/rke2.yaml apply -f gitops-repo/%s/ -n %s
	echo "Applications applied successfully"
else
	echo "Warning: Apps path 'gitops-repo/%s' not found"
fi

echo "GitOps applications deployed"
echo "Cleaning up temporary directory..."
`, argocdConfig.GitOpsRepoURL, argocdConfig.GitOpsRepoBranch, argocdConfig.GitOpsRepoURL,
		argocdConfig.AppsPath, argocdConfig.AppsPath, argocdConfig.AppsPath, argocdConfig.Namespace,
		argocdConfig.AppsPath)

	return runSSHCommand(masterNodeIP, sshPrivateKey, applyScript)
}

// getArgoCDAdminPassword retrieves the ArgoCD admin password
func getArgoCDAdminPassword(masterNodeIP string, sshPrivateKey string, namespace string) (string, error) {
	getPasswordScript := fmt.Sprintf(`
set -e
export KUBECONFIG=/etc/rancher/rke2/rke2.yaml
KUBECTL="/var/lib/rancher/rke2/bin/kubectl"
if [ ! -f "$KUBECTL" ]; then
    KUBECTL="kubectl"
fi
$KUBECTL --kubeconfig=/etc/rancher/rke2/rke2.yaml -n %s get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d
`, namespace)

	output, err := runSSHCommandWithOutput(masterNodeIP, sshPrivateKey, getPasswordScript)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(output), nil
}

// runSSHCommand executes a command on the remote node via SSH
func runSSHCommand(host string, privateKey string, command string) error {
	_, err := runSSHCommandWithOutput(host, privateKey, command)
	return err
}

// runSSHCommandWithOutput executes a command on the remote node via SSH and returns output
func runSSHCommandWithOutput(host string, privateKey string, command string) (string, error) {
	// Determine SSH user from environment or use default
	// AWS uses "ubuntu", DigitalOcean/Linode use "root"
	sshUser := os.Getenv("SSH_USER")
	if sshUser == "" {
		sshUser = "ubuntu" // Default to ubuntu for AWS compatibility
	}

	// Save private key to temporary file
	tmpKeyFile := fmt.Sprintf("/tmp/ssh-key-%d", time.Now().UnixNano())
	if err := exec.Command("bash", "-c", fmt.Sprintf("echo '%s' > %s && chmod 600 %s", privateKey, tmpKeyFile, tmpKeyFile)).Run(); err != nil {
		return "", fmt.Errorf("failed to save SSH key: %w", err)
	}
	defer exec.Command("rm", "-f", tmpKeyFile).Run()

	// Execute SSH command with connection timeout
	// Use sudo for non-root users to ensure commands run with proper permissions
	actualCommand := command
	if sshUser != "root" {
		actualCommand = fmt.Sprintf("sudo bash -c '%s'", strings.ReplaceAll(command, "'", "'\\''"))
	}
	sshCmd := fmt.Sprintf(`ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=30 -i %s %s@%s '%s'`, tmpKeyFile, sshUser, host, strings.ReplaceAll(actualCommand, "'", "'\\''"))

	cmd := exec.Command("bash", "-c", sshCmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("SSH command failed: %w\nOutput: %s", err, string(output))
	}

	return string(output), nil
}

// AppOfAppsConfig defines configuration for App of Apps pattern
type AppOfAppsConfig struct {
	Name       string
	Namespace  string
	RepoURL    string
	Branch     string
	Path       string
	SyncPolicy string // manual, automated
}

// SetupAppOfApps creates an App of Apps Application resource
func SetupAppOfApps(masterNodeIP string, sshPrivateKey string, cfg *AppOfAppsConfig) error {
	syncPolicy := ""
	if cfg.SyncPolicy == "automated" {
		syncPolicy = `
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
    - CreateNamespace=true`
	}

	appOfAppsManifest := fmt.Sprintf(`
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: %s
  namespace: %s
  finalizers:
    - resources-finalizer.argocd.argoproj.io
spec:
  project: default
  source:
    repoURL: %s
    targetRevision: %s
    path: %s
  destination:
    server: https://kubernetes.default.svc
    namespace: %s%s
`, cfg.Name, cfg.Namespace, cfg.RepoURL, cfg.Branch, cfg.Path, cfg.Namespace, syncPolicy)

	applyScript := fmt.Sprintf(`
set -e
export KUBECONFIG=/etc/rancher/rke2/rke2.yaml
KUBECTL="/var/lib/rancher/rke2/bin/kubectl"
if [ ! -f "$KUBECTL" ]; then
    KUBECTL="kubectl"
fi
cat <<'EOF' | $KUBECTL --kubeconfig=/etc/rancher/rke2/rke2.yaml apply -f -
%s
EOF
echo "App of Apps created successfully"
`, appOfAppsManifest)

	return runSSHCommand(masterNodeIP, sshPrivateKey, applyScript)
}

// ArgoCDStatus represents the status of ArgoCD installation
type ArgoCDStatus struct {
	Healthy       bool
	Pods          []PodStatus
	AppsTotal     int
	AppsSynced    int
	AppsOutOfSync int
}

// PodStatus represents a pod's status
type PodStatus struct {
	Name   string
	Status string
	Ready  string
}

// GetArgoCDStatus retrieves the status of ArgoCD installation
func GetArgoCDStatus(masterNodeIP string, sshPrivateKey string, namespace string) (*ArgoCDStatus, error) {
	statusScript := fmt.Sprintf(`
set -e
export KUBECONFIG=/etc/rancher/rke2/rke2.yaml
KUBECTL="/var/lib/rancher/rke2/bin/kubectl"
if [ ! -f "$KUBECTL" ]; then
    KUBECTL="kubectl"
fi
$KUBECTL --kubeconfig=/etc/rancher/rke2/rke2.yaml get pods -n %s -o json 2>/dev/null || echo '{"items":[]}'
`, namespace)

	output, err := runSSHCommandWithOutput(masterNodeIP, sshPrivateKey, statusScript)
	if err != nil {
		return nil, err
	}

	status := &ArgoCDStatus{
		Healthy: true,
		Pods:    []PodStatus{},
	}

	// Parse pod status from output (simplified parsing)
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "argocd-") && strings.Contains(line, "Running") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				status.Pods = append(status.Pods, PodStatus{
					Name:   parts[0],
					Status: parts[2],
					Ready:  parts[1],
				})
			}
		}
		if strings.Contains(line, "argocd-") && !strings.Contains(line, "Running") && !strings.Contains(line, "NAME") {
			status.Healthy = false
		}
	}

	// Get simple pod list
	podListScript := fmt.Sprintf(`
export KUBECONFIG=/etc/rancher/rke2/rke2.yaml
KUBECTL="/var/lib/rancher/rke2/bin/kubectl"
if [ ! -f "$KUBECTL" ]; then KUBECTL="kubectl"; fi
$KUBECTL --kubeconfig=/etc/rancher/rke2/rke2.yaml get pods -n %s --no-headers 2>/dev/null || true`, namespace)
	podOutput, _ := runSSHCommandWithOutput(masterNodeIP, sshPrivateKey, podListScript)

	status.Pods = []PodStatus{}
	for _, line := range strings.Split(podOutput, "\n") {
		parts := strings.Fields(line)
		if len(parts) >= 3 && strings.HasPrefix(parts[0], "argocd-") {
			status.Pods = append(status.Pods, PodStatus{
				Name:   parts[0],
				Ready:  parts[1],
				Status: parts[2],
			})
			if parts[2] != "Running" {
				status.Healthy = false
			}
		}
	}

	// Get application count
	appCountScript := fmt.Sprintf(`
export KUBECONFIG=/etc/rancher/rke2/rke2.yaml
KUBECTL="/var/lib/rancher/rke2/bin/kubectl"
if [ ! -f "$KUBECTL" ]; then KUBECTL="kubectl"; fi
$KUBECTL --kubeconfig=/etc/rancher/rke2/rke2.yaml get applications -n %s --no-headers 2>/dev/null | wc -l || echo "0"`, namespace)
	countOutput, _ := runSSHCommandWithOutput(masterNodeIP, sshPrivateKey, appCountScript)
	fmt.Sscanf(strings.TrimSpace(countOutput), "%d", &status.AppsTotal)

	return status, nil
}

// GetArgoCDPassword retrieves the ArgoCD admin password
func GetArgoCDPassword(masterNodeIP string, sshPrivateKey string, namespace string) (string, error) {
	return getArgoCDAdminPassword(masterNodeIP, sshPrivateKey, namespace)
}

// ArgoCDApp represents an ArgoCD application
type ArgoCDApp struct {
	Name       string
	Namespace  string
	SyncStatus string
	Health     string
	RepoURL    string
	Path       string
}

// ListArgoCDApps lists all ArgoCD applications
func ListArgoCDApps(masterNodeIP string, sshPrivateKey string, namespace string) ([]ArgoCDApp, error) {
	listScript := fmt.Sprintf(`
export KUBECONFIG=/etc/rancher/rke2/rke2.yaml
KUBECTL="/var/lib/rancher/rke2/bin/kubectl"
if [ ! -f "$KUBECTL" ]; then KUBECTL="kubectl"; fi
$KUBECTL --kubeconfig=/etc/rancher/rke2/rke2.yaml get applications -n %s -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.sync.status}{"\t"}{.status.health.status}{"\t"}{.spec.source.repoURL}{"\t"}{.spec.source.path}{"\n"}{end}' 2>/dev/null || true
`, namespace)

	output, err := runSSHCommandWithOutput(masterNodeIP, sshPrivateKey, listScript)
	if err != nil {
		return nil, err
	}

	var apps []ArgoCDApp
	for _, line := range strings.Split(output, "\n") {
		parts := strings.Split(line, "\t")
		if len(parts) >= 4 && parts[0] != "" {
			app := ArgoCDApp{
				Name:       parts[0],
				SyncStatus: parts[1],
				Health:     parts[2],
				RepoURL:    parts[3],
			}
			if len(parts) >= 5 {
				app.Path = parts[4]
			}
			apps = append(apps, app)
		}
	}

	return apps, nil
}

// SyncAllApps triggers sync for all ArgoCD applications
func SyncAllApps(masterNodeIP string, sshPrivateKey string, namespace string) error {
	syncScript := fmt.Sprintf(`
set -e
export KUBECONFIG=/etc/rancher/rke2/rke2.yaml
KUBECTL="/var/lib/rancher/rke2/bin/kubectl"
if [ ! -f "$KUBECTL" ]; then KUBECTL="kubectl"; fi
for app in $($KUBECTL --kubeconfig=/etc/rancher/rke2/rke2.yaml get applications -n %s -o jsonpath='{.items[*].metadata.name}'); do
    echo "Syncing $app..."
    $KUBECTL --kubeconfig=/etc/rancher/rke2/rke2.yaml patch application $app -n %s --type merge -p '{"operation":{"initiatedBy":{"username":"admin"},"sync":{"revision":"HEAD"}}}'
done
echo "All applications sync triggered"
`, namespace, namespace)

	return runSSHCommand(masterNodeIP, sshPrivateKey, syncScript)
}

// SyncApp triggers sync for a specific ArgoCD application
func SyncApp(masterNodeIP string, sshPrivateKey string, namespace string, appName string) error {
	syncScript := fmt.Sprintf(`
set -e
export KUBECONFIG=/etc/rancher/rke2/rke2.yaml
KUBECTL="/var/lib/rancher/rke2/bin/kubectl"
if [ ! -f "$KUBECTL" ]; then KUBECTL="kubectl"; fi
$KUBECTL --kubeconfig=/etc/rancher/rke2/rke2.yaml patch application %s -n %s --type merge -p '{"operation":{"initiatedBy":{"username":"admin"},"sync":{"revision":"HEAD"}}}'
echo "Application sync triggered"
`, appName, namespace)

	return runSSHCommand(masterNodeIP, sshPrivateKey, syncScript)
}
