package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/chalkan3/sloth-kubernetes/pkg/addons"
	"github.com/chalkan3/sloth-kubernetes/pkg/config"
)

var (
	argocdNamespace   string
	argocdVersion     string
	gitopsRepoURL     string
	gitopsRepoBranch  string
	gitopsAppsPath    string
	appOfAppsEnabled  bool
	appOfAppsName     string
)

var argocdCmd = &cobra.Command{
	Use:   "argocd",
	Short: "Manage ArgoCD GitOps integration",
	Long: `Manage ArgoCD GitOps integration for your Kubernetes cluster.

ArgoCD enables declarative GitOps continuous delivery with automatic
synchronization of your applications from a Git repository.

Supports App of Apps pattern for managing multiple applications.`,
}

var argocdInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install ArgoCD on the cluster",
	Long: `Install ArgoCD on your Kubernetes cluster.

This command will:
  1. Create the ArgoCD namespace
  2. Install ArgoCD from official manifests
  3. Wait for all pods to be ready
  4. Optionally configure GitOps repository
  5. Set up App of Apps pattern if enabled`,
	Example: `  # Install ArgoCD with defaults
  sloth-kubernetes argocd install --config cluster.lisp

  # Install with GitOps repo
  sloth-kubernetes argocd install -c cluster.lisp \
    --repo https://github.com/myorg/gitops.git \
    --branch main \
    --apps-path argocd/apps

  # Install with App of Apps pattern
  sloth-kubernetes argocd install -c cluster.lisp \
    --repo https://github.com/myorg/gitops.git \
    --app-of-apps \
    --app-of-apps-name root-app`,
	RunE: runArgocdInstall,
}

var argocdStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check ArgoCD status",
	Long:  `Check the status of ArgoCD installation and applications.`,
	Example: `  # Check ArgoCD status
  sloth-kubernetes argocd status --config cluster.lisp`,
	RunE: runArgocdStatus,
}

var argocdPasswordCmd = &cobra.Command{
	Use:   "password",
	Short: "Get ArgoCD admin password",
	Long:  `Retrieve the ArgoCD admin password from the cluster.`,
	Example: `  # Get ArgoCD admin password
  sloth-kubernetes argocd password --config cluster.lisp`,
	RunE: runArgocdPassword,
}

var argocdAppsCmd = &cobra.Command{
	Use:   "apps",
	Short: "List ArgoCD applications",
	Long:  `List all ArgoCD applications and their sync status.`,
	Example: `  # List all applications
  sloth-kubernetes argocd apps --config cluster.lisp`,
	RunE: runArgocdApps,
}

var argocdSyncCmd = &cobra.Command{
	Use:   "sync [app-name]",
	Short: "Sync an ArgoCD application",
	Long:  `Trigger synchronization of an ArgoCD application.`,
	Example: `  # Sync all applications
  sloth-kubernetes argocd sync --all --config cluster.lisp

  # Sync specific application
  sloth-kubernetes argocd sync my-app --config cluster.lisp`,
	RunE: runArgocdSync,
}

func init() {
	rootCmd.AddCommand(argocdCmd)
	argocdCmd.AddCommand(argocdInstallCmd)
	argocdCmd.AddCommand(argocdStatusCmd)
	argocdCmd.AddCommand(argocdPasswordCmd)
	argocdCmd.AddCommand(argocdAppsCmd)
	argocdCmd.AddCommand(argocdSyncCmd)

	// Install command flags
	argocdInstallCmd.Flags().StringVar(&argocdNamespace, "namespace", "argocd", "ArgoCD namespace")
	argocdInstallCmd.Flags().StringVar(&argocdVersion, "version", "stable", "ArgoCD version (stable, v2.9.3, etc)")
	argocdInstallCmd.Flags().StringVar(&gitopsRepoURL, "repo", "", "GitOps repository URL")
	argocdInstallCmd.Flags().StringVar(&gitopsRepoBranch, "branch", "main", "GitOps repository branch")
	argocdInstallCmd.Flags().StringVar(&gitopsAppsPath, "apps-path", "argocd/apps", "Path to applications in GitOps repo")
	argocdInstallCmd.Flags().BoolVar(&appOfAppsEnabled, "app-of-apps", false, "Enable App of Apps pattern")
	argocdInstallCmd.Flags().StringVar(&appOfAppsName, "app-of-apps-name", "root-app", "Name for App of Apps")

	// Status command flags
	argocdStatusCmd.Flags().StringVar(&argocdNamespace, "namespace", "argocd", "ArgoCD namespace")

	// Password command flags
	argocdPasswordCmd.Flags().StringVar(&argocdNamespace, "namespace", "argocd", "ArgoCD namespace")

	// Apps command flags
	argocdAppsCmd.Flags().StringVar(&argocdNamespace, "namespace", "argocd", "ArgoCD namespace")

	// Sync command flags
	argocdSyncCmd.Flags().StringVar(&argocdNamespace, "namespace", "argocd", "ArgoCD namespace")
	argocdSyncCmd.Flags().Bool("all", false, "Sync all applications")
}

func runArgocdInstall(cmd *cobra.Command, args []string) error {
	printHeader("Installing ArgoCD GitOps")

	// Load configuration
	configPath := cfgFile
	if configPath == "" {
		configPath = "./cluster-config.lisp"
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("config file not found: %s", configPath)
	}

	cfg, err := config.LoadFromLisp(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Get master node IP from stack outputs or config
	masterIP, sshKey, err := getMasterNodeCredentials(cfg)
	if err != nil {
		return fmt.Errorf("failed to get master node credentials: %w", err)
	}

	// Override config with CLI flags
	if cfg.Addons.ArgoCD == nil {
		cfg.Addons.ArgoCD = &config.ArgoCDConfig{}
	}
	cfg.Addons.ArgoCD.Enabled = true
	cfg.Addons.ArgoCD.Namespace = argocdNamespace
	cfg.Addons.ArgoCD.Version = argocdVersion

	if gitopsRepoURL != "" {
		cfg.Addons.ArgoCD.GitOpsRepoURL = gitopsRepoURL
		cfg.Addons.ArgoCD.GitOpsRepoBranch = gitopsRepoBranch
		cfg.Addons.ArgoCD.AppsPath = gitopsAppsPath
	}

	// Install ArgoCD
	if err := addons.InstallArgoCD(cfg, masterIP, sshKey); err != nil {
		return fmt.Errorf("failed to install ArgoCD: %w", err)
	}

	// Install App of Apps if enabled
	if appOfAppsEnabled && gitopsRepoURL != "" {
		fmt.Println()
		color.Cyan("Setting up App of Apps pattern...")
		if err := addons.SetupAppOfApps(masterIP, sshKey, &addons.AppOfAppsConfig{
			Name:       appOfAppsName,
			Namespace:  argocdNamespace,
			RepoURL:    gitopsRepoURL,
			Branch:     gitopsRepoBranch,
			Path:       gitopsAppsPath,
			SyncPolicy: "automated",
		}); err != nil {
			color.Yellow("Warning: Failed to setup App of Apps: %v", err)
		} else {
			color.Green("App of Apps '%s' created successfully!", appOfAppsName)
		}
	}

	fmt.Println()
	printSuccess("ArgoCD installation completed!")
	printArgocdAccessInfo(argocdNamespace)

	return nil
}

func runArgocdStatus(cmd *cobra.Command, args []string) error {
	printHeader("ArgoCD Status")

	// Load configuration
	cfg, masterIP, sshKey, err := loadClusterCredentials()
	if err != nil {
		return err
	}
	_ = cfg

	fmt.Println()
	color.Cyan("Checking ArgoCD pods...")
	fmt.Println()

	// Check pod status
	status, err := addons.GetArgoCDStatus(masterIP, sshKey, argocdNamespace)
	if err != nil {
		return fmt.Errorf("failed to get ArgoCD status: %w", err)
	}

	// Display status
	fmt.Printf("Namespace: %s\n", argocdNamespace)
	fmt.Println()

	if status.Healthy {
		color.Green("Status: Healthy")
	} else {
		color.Red("Status: Unhealthy")
	}
	fmt.Println()

	fmt.Println("Pods:")
	for _, pod := range status.Pods {
		statusIcon := "✅"
		if pod.Status != "Running" {
			statusIcon = "❌"
		}
		fmt.Printf("  %s %s: %s (%s)\n", statusIcon, pod.Name, pod.Status, pod.Ready)
	}

	fmt.Println()
	fmt.Printf("Applications: %d total, %d synced, %d out-of-sync\n",
		status.AppsTotal, status.AppsSynced, status.AppsOutOfSync)

	return nil
}

func runArgocdPassword(cmd *cobra.Command, args []string) error {
	_, masterIP, sshKey, err := loadClusterCredentials()
	if err != nil {
		return err
	}

	password, err := addons.GetArgoCDPassword(masterIP, sshKey, argocdNamespace)
	if err != nil {
		return fmt.Errorf("failed to get ArgoCD password: %w", err)
	}

	fmt.Println()
	color.Cyan("ArgoCD Admin Credentials")
	fmt.Println("========================")
	fmt.Printf("Username: admin\n")
	fmt.Printf("Password: %s\n", password)
	fmt.Println()

	return nil
}

func runArgocdApps(cmd *cobra.Command, args []string) error {
	printHeader("ArgoCD Applications")

	_, masterIP, sshKey, err := loadClusterCredentials()
	if err != nil {
		return err
	}

	apps, err := addons.ListArgoCDApps(masterIP, sshKey, argocdNamespace)
	if err != nil {
		return fmt.Errorf("failed to list applications: %w", err)
	}

	if len(apps) == 0 {
		fmt.Println("No applications found.")
		return nil
	}

	fmt.Println()
	fmt.Printf("%-30s %-15s %-15s %-20s\n", "NAME", "SYNC STATUS", "HEALTH", "REPO")
	fmt.Println(strings.Repeat("-", 80))

	for _, app := range apps {
		syncIcon := "✅"
		if app.SyncStatus != "Synced" {
			syncIcon = "⚠️"
		}
		healthIcon := "💚"
		if app.Health != "Healthy" {
			healthIcon = "❤️"
		}

		fmt.Printf("%-30s %s %-13s %s %-13s %-20s\n",
			truncate(app.Name, 30),
			syncIcon, app.SyncStatus,
			healthIcon, app.Health,
			truncate(app.RepoURL, 20))
	}

	fmt.Println()
	return nil
}

func runArgocdSync(cmd *cobra.Command, args []string) error {
	syncAll, _ := cmd.Flags().GetBool("all")

	_, masterIP, sshKey, err := loadClusterCredentials()
	if err != nil {
		return err
	}

	if syncAll {
		color.Cyan("Syncing all applications...")
		if err := addons.SyncAllApps(masterIP, sshKey, argocdNamespace); err != nil {
			return fmt.Errorf("failed to sync applications: %w", err)
		}
		printSuccess("All applications synced!")
	} else if len(args) > 0 {
		appName := args[0]
		color.Cyan("Syncing application: %s", appName)
		if err := addons.SyncApp(masterIP, sshKey, argocdNamespace, appName); err != nil {
			return fmt.Errorf("failed to sync application %s: %w", appName, err)
		}
		printSuccess(fmt.Sprintf("Application '%s' synced!", appName))
	} else {
		return fmt.Errorf("please specify an application name or use --all")
	}

	return nil
}

// Helper functions

func getMasterNodeCredentials(cfg *config.ClusterConfig) (string, string, error) {
	// Try to get from environment or saved state
	masterIP := os.Getenv("MASTER_NODE_IP")
	sshKeyPath := cfg.Security.SSHConfig.KeyPath

	if masterIP == "" {
		// Try to find first master node from config
		for _, pool := range cfg.NodePools {
			for _, role := range pool.Roles {
				if role == "master" || role == "controlplane" {
					// We need to get the actual IP from Pulumi state
					// For now, prompt user
					return "", "", fmt.Errorf("MASTER_NODE_IP environment variable not set. Please set it or use 'sloth-kubernetes status' to get node IPs")
				}
			}
		}
	}

	// Read SSH key
	sshKey, err := readSSHKey(sshKeyPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to read SSH key: %w", err)
	}

	return masterIP, sshKey, nil
}

func loadClusterCredentials() (*config.ClusterConfig, string, string, error) {
	configPath := cfgFile
	if configPath == "" {
		configPath = "./cluster-config.lisp"
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, "", "", fmt.Errorf("config file not found: %s", configPath)
	}

	cfg, err := config.LoadFromLisp(configPath)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to load config: %w", err)
	}

	masterIP, sshKey, err := getMasterNodeCredentials(cfg)
	if err != nil {
		return nil, "", "", err
	}

	return cfg, masterIP, sshKey, nil
}

func readSSHKey(path string) (string, error) {
	if path == "" {
		path = os.ExpandEnv("$HOME/.ssh/id_rsa")
	}

	// Expand ~ to home directory
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		path = home + path[1:]
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

func printArgocdAccessInfo(namespace string) {
	fmt.Println()
	color.Cyan("Access Information:")
	fmt.Println("==================")
	fmt.Println()
	fmt.Println("To access ArgoCD UI:")
	fmt.Printf("  kubectl port-forward svc/argocd-server -n %s 8080:443\n", namespace)
	fmt.Println("  Then open: https://localhost:8080")
	fmt.Println()
	fmt.Println("To get admin password:")
	fmt.Println("  sloth-kubernetes argocd password")
	fmt.Println()
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
