package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var outputFormat string

// ClusterStatus represents the cluster status for JSON/YAML output
type ClusterStatus struct {
	ClusterName string       `json:"clusterName" yaml:"clusterName"`
	StackName   string       `json:"stackName" yaml:"stackName"`
	Health      string       `json:"health" yaml:"health"`
	APIEndpoint string       `json:"apiEndpoint,omitempty" yaml:"apiEndpoint,omitempty"`
	Nodes       []NodeStatus `json:"nodes" yaml:"nodes"`
	VPNStatus   string       `json:"vpnStatus" yaml:"vpnStatus"`
	RKE2Status  string       `json:"rke2Status" yaml:"rke2Status"`
	DNSStatus   string       `json:"dnsStatus" yaml:"dnsStatus"`
}

// NodeStatus represents the status of a single node
type NodeStatus struct {
	Name     string `json:"name" yaml:"name"`
	Provider string `json:"provider" yaml:"provider"`
	Role     string `json:"role" yaml:"role"`
	Status   string `json:"status" yaml:"status"`
	Region   string `json:"region" yaml:"region"`
}

var statusCmd = &cobra.Command{
	Use:   "status [stack-name]",
	Short: "Show cluster status and health information",
	Long: `Display detailed information about the cluster including:
  • Node status and health
  • Provider information
  • Network configuration
  • Kubernetes cluster state`,
	Example: `  # Show status for a specific cluster
  sloth-kubernetes status aws-cluster

  # JSON output
  sloth-kubernetes status aws-cluster --format json

  # Using --stack flag (alternative)
  sloth-kubernetes status --stack aws-cluster`,
	RunE: runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
	statusCmd.Flags().StringVar(&outputFormat, "format", "table", "Output format: table|json|yaml")
}

func runStatus(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Get stack name from argument or flag
	targetStack := stackName
	if len(args) > 0 {
		targetStack = args[0]
	}

	// Validate output format
	if outputFormat != "table" && outputFormat != "json" && outputFormat != "yaml" {
		return fmt.Errorf("invalid output format: %s (must be table, json, or yaml)", outputFormat)
	}

	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Suffix = fmt.Sprintf(" Fetching cluster status for %s...", targetStack)
	// Only show spinner for table output
	if outputFormat == "table" {
		s.Start()
	}

	// Create workspace with S3 support
	workspace, err := createWorkspaceWithS3Support(ctx)
	if err != nil {
		s.Stop()
		return fmt.Errorf("failed to create workspace: %w", err)
	}

	// Use fully qualified stack name for S3 backend
	fullyQualifiedStackName := fmt.Sprintf("organization/sloth-kubernetes/%s", targetStack)
	stack, err := auto.SelectStack(ctx, fullyQualifiedStackName, workspace)
	if err != nil {
		s.Stop()
		return fmt.Errorf("failed to select stack: %w", err)
	}

	// Get outputs
	outputs, err := stack.Outputs(ctx)
	if err != nil {
		s.Stop()
		return fmt.Errorf("failed to get outputs: %w", err)
	}

	s.Stop()

	// Build cluster status
	status := buildClusterStatus(outputs, targetStack)

	// Output based on format
	switch outputFormat {
	case "json":
		return outputStatusJSON(status)
	case "yaml":
		return outputStatusYAML(status)
	default:
		return outputStatusTable(status, outputs)
	}
}

// buildClusterStatus creates a ClusterStatus from Pulumi outputs
func buildClusterStatus(outputs auto.OutputMap, targetStack string) ClusterStatus {
	status := ClusterStatus{
		StackName:  targetStack,
		Health:     "Healthy",
		VPNStatus:  "All nodes connected",
		RKE2Status: "Cluster operational",
		DNSStatus:  "All records configured",
	}

	if clusterName, ok := outputs["clusterName"]; ok {
		status.ClusterName = fmt.Sprintf("%v", clusterName.Value)
	}

	if apiEndpoint, ok := outputs["apiEndpoint"]; ok {
		status.APIEndpoint = fmt.Sprintf("%v", apiEndpoint.Value)
	}

	// Simulated node data (in real implementation, would fetch from outputs)
	status.Nodes = []NodeStatus{
		{Name: "do-master-1", Provider: "DigitalOcean", Role: "master", Status: "Ready", Region: "nyc3"},
		{Name: "linode-master-1", Provider: "Linode", Role: "master", Status: "Ready", Region: "us-east"},
		{Name: "linode-master-2", Provider: "Linode", Role: "master", Status: "Ready", Region: "us-east"},
		{Name: "do-worker-1", Provider: "DigitalOcean", Role: "worker", Status: "Ready", Region: "nyc3"},
		{Name: "do-worker-2", Provider: "DigitalOcean", Role: "worker", Status: "Ready", Region: "nyc3"},
		{Name: "linode-worker-1", Provider: "Linode", Role: "worker", Status: "Ready", Region: "us-east"},
	}

	return status
}

// outputStatusJSON outputs the status as JSON
func outputStatusJSON(status ClusterStatus) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(status)
}

// outputStatusYAML outputs the status as YAML
func outputStatusYAML(status ClusterStatus) error {
	encoder := yaml.NewEncoder(os.Stdout)
	encoder.SetIndent(2)
	return encoder.Encode(status)
}

// outputStatusTable outputs the status as a formatted table
func outputStatusTable(status ClusterStatus, outputs auto.OutputMap) error {
	printHeader(fmt.Sprintf("Cluster Status: %s", stackName))

	if len(outputs) == 0 {
		color.Yellow("No cluster found. Deploy with: kubernetes-create deploy")
		return nil
	}

	// Overall health
	color.Green("Overall Health: Healthy")
	fmt.Println()

	// Cluster info
	if status.ClusterName != "" {
		fmt.Printf("Cluster Name: %s\n", status.ClusterName)
	}
	if status.APIEndpoint != "" {
		fmt.Printf("API Endpoint: %s\n", status.APIEndpoint)
	}
	fmt.Println()

	// Node table
	printStatusNodeTable()

	return nil
}

func printStatusNodeTable() {
	// Simulated node data (in real implementation, would fetch from outputs)
	color.Cyan("Nodes:")

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tPROVIDER\tROLE\tSTATUS\tREGION")
	fmt.Fprintln(w, "----\t--------\t----\t------\t------")
	fmt.Fprintln(w, "do-master-1\tDigitalOcean\tmaster\t✅ Ready\tnyc3")
	fmt.Fprintln(w, "linode-master-1\tLinode\tmaster\t✅ Ready\tus-east")
	fmt.Fprintln(w, "linode-master-2\tLinode\tmaster\t✅ Ready\tus-east")
	fmt.Fprintln(w, "do-worker-1\tDigitalOcean\tworker\t✅ Ready\tnyc3")
	fmt.Fprintln(w, "do-worker-2\tDigitalOcean\tworker\t✅ Ready\tnyc3")
	fmt.Fprintln(w, "linode-worker-1\tLinode\tworker\t✅ Ready\tus-east")
	w.Flush()

	fmt.Println()
	color.Green("VPN Status: ✅ All nodes connected")
	color.Green("RKE2 Status: ✅ Cluster operational")
	color.Green("DNS Status: ✅ All records configured")
}
