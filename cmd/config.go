package cmd

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	outputPath string
	format     string
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage cluster configuration",
	Long: `Manage cluster configuration files.

Generate example configuration files in Lisp S-expression format.`,
}

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate example configuration file",
	Long: `Generate an example cluster configuration file in Lisp S-expression format.

The generated file uses S-expressions with:
  - (cluster ...)  - Root element
  - (metadata ...) - Cluster name, labels, annotations
  - (providers ..) - Cloud provider configurations
  - (node-pools .) - Node pool definitions

You can use environment variables using ${VAR_NAME} syntax.`,
	Example: `  # Generate example config
  sloth-kubernetes config generate

  # Generate to specific file
  sloth-kubernetes config generate -o cluster.lisp

  # Generate minimal config
  sloth-kubernetes config generate --format minimal`,
	RunE: runGenerate,
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(generateCmd)

	generateCmd.Flags().StringVarP(&outputPath, "output", "o", "cluster-config.lisp", "Output file path")
	generateCmd.Flags().StringVar(&format, "format", "full", "Config format: full|minimal")
}

func runGenerate(cmd *cobra.Command, args []string) error {
	printHeader("Generating Configuration File")

	var content string

	switch format {
	case "minimal":
		content = generateMinimalLispConfig()
	case "full":
		content = generateFullLispConfig()
	default:
		return fmt.Errorf("unknown format: %s (use 'full' or 'minimal')", format)
	}

	// Save to file
	if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	printSuccess(fmt.Sprintf("Configuration saved to %s", outputPath))
	fmt.Println()

	// Print usage instructions
	printUsageInstructions(outputPath)

	return nil
}

func generateMinimalLispConfig() string {
	return `; Sloth Kubernetes - Minimal Cluster Configuration
; Edit this file and set your credentials before deploying

(cluster
  (metadata
    (name "my-cluster")
    (environment "production"))

  (providers
    (digitalocean
      (enabled true)
      (token "${DIGITALOCEAN_TOKEN}")
      (region "nyc3")))

  (network
    (mode "wireguard")
    (wireguard
      (enabled true)
      (create true)
      (mesh-networking true)))

  (node-pools
    (masters
      (name "masters")
      (provider "digitalocean")
      (count 1)
      (roles master etcd)
      (size "s-2vcpu-4gb")
      (region "nyc3"))
    (workers
      (name "workers")
      (provider "digitalocean")
      (count 2)
      (roles worker)
      (size "s-2vcpu-4gb")
      (region "nyc3")))

  (kubernetes
    (distribution "rke2")
    (version "v1.29.0+rke2r1")))
`
}

func generateFullLispConfig() string {
	return `; Sloth Kubernetes - Full Cluster Configuration
; Complete example with all available options

(cluster
  ; Cluster metadata
  (metadata
    (name "production-cluster")
    (environment "production")
    (description "Production Kubernetes cluster")
    (owner "platform-team")
    (labels
      (project "my-project")
      (team "platform")))

  ; Cluster specification
  (cluster
    (type "rke2")
    (version "v1.29.0+rke2r1")
    (high-availability true))

  ; Cloud providers - enable the ones you need
  (providers
    (digitalocean
      (enabled true)
      (token "${DIGITALOCEAN_TOKEN}")
      (region "nyc3")
      (monitoring true))

    (aws
      (enabled false)
      (region "us-east-1"))

    (azure
      (enabled false)
      (subscription-id "${AZURE_SUBSCRIPTION_ID}")
      (tenant-id "${AZURE_TENANT_ID}")
      (client-id "${AZURE_CLIENT_ID}")
      (client-secret "${AZURE_CLIENT_SECRET}")
      (resource-group "kubernetes-rg")
      (location "eastus"))

    (linode
      (enabled false)
      (token "${LINODE_TOKEN}")
      (region "us-east")
      (root-password "${LINODE_ROOT_PASSWORD}")))

  ; Network configuration
  (network
    (mode "wireguard")
    (pod-cidr "10.42.0.0/16")
    (service-cidr "10.43.0.0/16")

    (wireguard
      (enabled true)
      (create true)
      (provider "digitalocean")
      (region "nyc3")
      (subnet-cidr "10.8.0.0/24")
      (port 51820)
      (mesh-networking true)
      (auto-config true))

    (dns
      (domain "example.com")
      (provider "digitalocean")))

  ; Security configuration
  (security
    (ssh
      (key-path "~/.ssh/id_rsa")
      (public-key-path "~/.ssh/id_rsa.pub")
      (port 22))

    (bastion
      (enabled true)
      (provider "digitalocean")
      (region "nyc3")
      (size "s-1vcpu-1gb")
      (name "bastion")
      (ssh-port 22)
      (allowed-cidrs "0.0.0.0/0")
      (enable-audit-log true)
      (idle-timeout 30)
      (max-sessions 10)))

  ; Node pools
  (node-pools
    (masters
      (name "masters")
      (provider "digitalocean")
      (region "nyc3")
      (count 3)
      (roles master etcd)
      (size "s-4vcpu-8gb")
      (labels
        (role "control-plane")
        (tier "master")))

    (workers
      (name "workers")
      (provider "digitalocean")
      (region "nyc3")
      (count 5)
      (roles worker)
      (size "s-4vcpu-8gb")
      (labels
        (role "worker")
        (tier "application"))))

  ; Kubernetes configuration
  (kubernetes
    (version "v1.29.0")
    (distribution "rke2")
    (network-plugin "canal")
    (pod-cidr "10.42.0.0/16")
    (service-cidr "10.43.0.0/16")
    (cluster-domain "cluster.local")

    (rke2
      (version "v1.29.0+rke2r1")
      (channel "stable")
      (disable-components "rke2-ingress-nginx")
      (snapshot-schedule-cron "0 */6 * * *")
      (snapshot-retention 5)))

  ; Monitoring
  (monitoring
    (enabled true)
    (prometheus
      (enabled true)
      (retention "15d"))
    (grafana
      (enabled true))))
`
}

func printUsageInstructions(filePath string) {
	color.Cyan("Next Steps:")
	fmt.Println()
	fmt.Println("1. Edit the configuration file:")
	fmt.Printf("   vim %s\n", filePath)
	fmt.Println()
	fmt.Println("2. Set your credentials:")
	fmt.Println("   export DIGITALOCEAN_TOKEN=\"your-token\"")
	fmt.Println("   # Or for other providers:")
	fmt.Println("   export AWS_ACCESS_KEY_ID=\"...\"")
	fmt.Println("   export AWS_SECRET_ACCESS_KEY=\"...\"")
	fmt.Println()
	fmt.Println("3. Deploy the cluster:")
	fmt.Printf("   sloth-kubernetes deploy --config %s\n", filePath)
	fmt.Println()
	color.Yellow("Tip: Use 'sloth-kubernetes validate --config %s' to check your configuration", filePath)
}
