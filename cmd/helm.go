package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var helmCmd = &cobra.Command{
	Use:   "helm [stack-name] [helm-args...]",
	Short: "Execute Helm commands using kubeconfig from stack",
	Long: `Execute Helm commands by calling the helm binary.

This command requires the 'helm' binary to be installed and available in your PATH.
You can install Helm from: https://helm.sh/docs/intro/install/

The kubeconfig is automatically retrieved from the specified Pulumi stack,
so you don't need to manage kubeconfig files manually.

All standard Helm v3 commands and flags are supported after the stack name.`,
	Example: `  # List all releases
  sloth-kubernetes helm my-cluster list

  # List releases in all namespaces
  sloth-kubernetes helm my-cluster list -A

  # Install a chart
  sloth-kubernetes helm my-cluster install myapp bitnami/nginx

  # Upgrade a release
  sloth-kubernetes helm my-cluster upgrade myapp bitnami/nginx

  # Add a repository
  sloth-kubernetes helm my-cluster repo add bitnami https://charts.bitnami.com/bitnami

  # Search for charts
  sloth-kubernetes helm my-cluster search repo nginx

  # Get release status
  sloth-kubernetes helm my-cluster status myapp

  # Uninstall a release
  sloth-kubernetes helm my-cluster uninstall myapp`,
	DisableFlagParsing: true,
	RunE:               runHelm,
}

func init() {
	rootCmd.AddCommand(helmCmd)
}

func runHelm(cmd *cobra.Command, args []string) error {
	// Check if helm is available in PATH
	helmBinary, err := exec.LookPath("helm")
	if err != nil {
		return fmt.Errorf("helm binary not found in PATH. Please install Helm from https://helm.sh/docs/intro/install/")
	}

	// Parse stack name from args
	if len(args) == 0 {
		return fmt.Errorf("stack name is required\nUsage: sloth-kubernetes helm <stack-name> [helm-args...]")
	}

	// Check if first arg is a flag (starts with -)
	targetStack := ""
	helmArgs := args

	if args[0][0] != '-' {
		targetStack = args[0]
		helmArgs = args[1:]
	} else {
		targetStack = stackName
	}

	if targetStack == "" {
		return fmt.Errorf("stack name is required\nUsage: sloth-kubernetes helm <stack-name> [helm-args...]")
	}

	// Validate stack exists
	if err := EnsureStackExists(targetStack); err != nil {
		return err
	}

	// Get kubeconfig from stack
	kubeconfigPath, err := GetKubeconfigFromStack(targetStack)
	if err != nil {
		return fmt.Errorf("failed to get kubeconfig from stack '%s': %w", targetStack, err)
	}

	// Set KUBECONFIG environment
	os.Setenv("KUBECONFIG", kubeconfigPath)

	// Create and execute helm command
	helmExec := exec.Command(helmBinary, helmArgs...)
	helmExec.Stdin = os.Stdin
	helmExec.Stdout = os.Stdout
	helmExec.Stderr = os.Stderr
	helmExec.Env = os.Environ()

	return helmExec.Run()
}
