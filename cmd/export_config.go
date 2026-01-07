package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"

	"github.com/chalkan3/sloth-kubernetes/internal/common"
	"github.com/chalkan3/sloth-kubernetes/pkg/config"
)

var (
	exportFormat     string
	exportConfigFile string
	regenerateLisp   bool
	showMeta         bool
)

var exportConfigCmd = &cobra.Command{
	Use:   "export-config [stack-name]",
	Short: "Export cluster configuration from Pulumi state",
	Long: `Export the cluster configuration stored in Pulumi state.

This command retrieves the configuration that was used to deploy the cluster,
allowing you to regenerate the Lisp manifest or view the JSON configuration.

The configuration is stored in Pulumi state during deployment, enabling:
  • Recovery of lost configuration files
  • Viewing what was actually deployed
  • Creating new deployments based on existing ones
  • Auditing cluster configurations

Formats:
  • lisp: Original Lisp S-expression format (default)
  • json: JSON format (structured, programmatic access)
  • yaml: YAML format (human-readable)
  • meta: Deployment metadata (timestamps, scale operations, etc.)`,
	Example: `  # Export config as Lisp (default)
  sloth export-config production

  # Export config as JSON
  sloth export-config production --format json

  # Export to a file
  sloth export-config production --output recovered-config.lisp

  # Regenerate Lisp from stored JSON (if original Lisp was lost)
  sloth export-config production --regenerate

  # Export deployment metadata (timestamps, scale operations)
  sloth export-config production --format meta`,
	RunE: runExportConfig,
}

func init() {
	rootCmd.AddCommand(exportConfigCmd)

	exportConfigCmd.Flags().StringVarP(&exportFormat, "format", "f", "lisp", "Output format: lisp, json, yaml, meta")
	exportConfigCmd.Flags().StringVarP(&exportConfigFile, "output", "o", "", "Output file (default: stdout)")
	exportConfigCmd.Flags().BoolVar(&regenerateLisp, "regenerate", false, "Regenerate Lisp from stored JSON config")
	exportConfigCmd.Flags().BoolVar(&showMeta, "meta", false, "Also show deployment metadata")
}

func runExportConfig(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Load saved S3 backend configuration
	_ = common.LoadSavedConfig()

	// Require a valid stack
	targetStack, err := RequireStack(args)
	if err != nil {
		return err
	}

	color.Cyan("📦 Exporting configuration from stack: %s\n", targetStack)

	// Create workspace
	projectName := "sloth-kubernetes"
	workspaceOpts := []auto.LocalWorkspaceOption{
		auto.Project(workspace.Project{
			Name:    tokens.PackageName(projectName),
			Runtime: workspace.NewProjectRuntimeInfo("go", nil),
		}),
	}

	// Collect environment variables for S3 backend
	envVars := make(map[string]string)
	awsEnvKeys := []string{
		"AWS_ACCESS_KEY_ID",
		"AWS_SECRET_ACCESS_KEY",
		"AWS_REGION",
		"AWS_S3_ENDPOINT",
		"AWS_S3_USE_PATH_STYLE",
		"AWS_S3_FORCE_PATH_STYLE",
		"PULUMI_BACKEND_URL",
		"PULUMI_CONFIG_PASSPHRASE",
	}
	for _, key := range awsEnvKeys {
		if val := os.Getenv(key); val != "" {
			envVars[key] = val
		}
	}

	if len(envVars) > 0 {
		workspaceOpts = append(workspaceOpts, auto.EnvVars(envVars))
	}

	if backendURL := os.Getenv("PULUMI_BACKEND_URL"); backendURL != "" {
		workspaceOpts = append(workspaceOpts, auto.SecretsProvider("passphrase"))
	}

	ws, err := auto.NewLocalWorkspace(ctx, workspaceOpts...)
	if err != nil {
		return fmt.Errorf("failed to create workspace: %w", err)
	}

	// Select the stack
	fullyQualifiedStackName := fmt.Sprintf("organization/%s/%s", projectName, targetStack)
	stack, err := auto.SelectStack(ctx, fullyQualifiedStackName, ws)
	if err != nil {
		return fmt.Errorf("failed to select stack '%s': %w", targetStack, err)
	}

	// Get outputs
	outputs, err := stack.Outputs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get stack outputs: %w", err)
	}

	// Get the stored configuration
	var result string

	switch exportFormat {
	case "lisp":
		if regenerateLisp {
			// Regenerate Lisp from JSON config
			result, err = regenerateLispFromJSON(outputs)
			if err != nil {
				return fmt.Errorf("failed to regenerate Lisp: %w", err)
			}
		} else {
			// Use stored Lisp manifest
			lispOutput, ok := outputs["lispManifest"]
			if !ok || lispOutput.Value == nil || lispOutput.Value.(string) == "" {
				color.Yellow("⚠️  No stored Lisp manifest found. Trying to regenerate from JSON...")
				result, err = regenerateLispFromJSON(outputs)
				if err != nil {
					return fmt.Errorf("failed to regenerate Lisp: %w", err)
				}
			} else {
				result = lispOutput.Value.(string)
			}
		}

	case "json":
		jsonOutput, ok := outputs["configJson"]
		if !ok || jsonOutput.Value == nil {
			return fmt.Errorf("no stored JSON config found in stack outputs")
		}
		result = jsonOutput.Value.(string)

	case "yaml":
		// Convert JSON to YAML
		jsonOutput, ok := outputs["configJson"]
		if !ok || jsonOutput.Value == nil {
			return fmt.Errorf("no stored JSON config found in stack outputs")
		}

		var cfg config.ClusterConfig
		if err := json.Unmarshal([]byte(jsonOutput.Value.(string)), &cfg); err != nil {
			return fmt.Errorf("failed to parse stored JSON: %w", err)
		}

		// Use JSON with indentation as pseudo-YAML (proper YAML would need yaml.v3)
		yamlLike, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to format as YAML: %w", err)
		}
		result = string(yamlLike)

	case "meta":
		// Export deployment metadata
		metaOutput, ok := outputs["deploymentMeta"]
		if !ok || metaOutput.Value == nil {
			return fmt.Errorf("no deployment metadata found in stack outputs")
		}
		// Pretty print the metadata JSON
		var meta interface{}
		if err := json.Unmarshal([]byte(metaOutput.Value.(string)), &meta); err != nil {
			result = metaOutput.Value.(string)
		} else {
			prettyMeta, err := json.MarshalIndent(meta, "", "  ")
			if err != nil {
				result = metaOutput.Value.(string)
			} else {
				result = string(prettyMeta)
			}
		}

	default:
		return fmt.Errorf("unknown format: %s (supported: lisp, json, yaml, meta)", exportFormat)
	}

	// Output the result
	if exportConfigFile != "" {
		if err := os.WriteFile(exportConfigFile, []byte(result), 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		color.Green("✅ Configuration exported to: %s", exportConfigFile)
	} else {
		fmt.Println()
		fmt.Println(result)
	}

	// Show deployment metadata if requested
	if showMeta && exportFormat != "meta" {
		metaOutput, ok := outputs["deploymentMeta"]
		if ok && metaOutput.Value != nil {
			fmt.Println()
			color.Cyan("📊 Deployment Metadata:")
			var meta interface{}
			if err := json.Unmarshal([]byte(metaOutput.Value.(string)), &meta); err == nil {
				prettyMeta, _ := json.MarshalIndent(meta, "", "  ")
				fmt.Println(string(prettyMeta))
			} else {
				fmt.Println(metaOutput.Value.(string))
			}
		}
	}

	return nil
}

// regenerateLispFromJSON regenerates Lisp config from stored JSON
func regenerateLispFromJSON(outputs auto.OutputMap) (string, error) {
	jsonOutput, ok := outputs["configJson"]
	if !ok || jsonOutput.Value == nil {
		return "", fmt.Errorf("no stored JSON config found in stack outputs")
	}

	jsonStr, ok := jsonOutput.Value.(string)
	if !ok {
		return "", fmt.Errorf("configJson is not a string")
	}

	var cfg config.ClusterConfig
	if err := json.Unmarshal([]byte(jsonStr), &cfg); err != nil {
		return "", fmt.Errorf("failed to parse stored JSON: %w", err)
	}

	// Use the Lisp generator
	return config.GenerateLisp(&cfg), nil
}
