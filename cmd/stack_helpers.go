package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
)

// StackKubeconfig holds kubeconfig and related info from a stack
type StackKubeconfig struct {
	KubeconfigPath string
	MasterIP       string
	SSHKeyPath     string
	ClusterName    string
}

// GetKubeconfigFromStack retrieves kubeconfig from a Pulumi stack and saves it to a temp file
// Returns the path to the kubeconfig file
func GetKubeconfigFromStack(targetStack string) (string, error) {
	if targetStack == "" {
		return "", fmt.Errorf("stack name is required")
	}

	ctx := context.Background()

	// Create workspace with S3 support
	workspace, err := createWorkspaceWithS3Support(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to create workspace: %w", err)
	}

	// Use fully qualified stack name for S3 backend
	fullyQualifiedStackName := fmt.Sprintf("organization/sloth-kubernetes/%s", targetStack)
	stack, err := auto.SelectStack(ctx, fullyQualifiedStackName, workspace)
	if err != nil {
		return "", fmt.Errorf("failed to select stack '%s': %w", targetStack, err)
	}

	// Get outputs
	outputs, err := stack.Outputs(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get stack outputs: %w", err)
	}

	// Get kubeconfig from outputs
	kubeConfigOutput, ok := outputs["kubeConfig"]
	if !ok {
		return "", fmt.Errorf("kubeconfig not found in stack '%s' outputs", targetStack)
	}

	kubeConfigStr := fmt.Sprintf("%v", kubeConfigOutput.Value)
	if kubeConfigStr == "" || kubeConfigStr == "<nil>" {
		return "", fmt.Errorf("kubeconfig is empty in stack '%s'", targetStack)
	}

	// Extract kubeconfig from markers if present
	kubeConfigStr = extractKubeconfig(kubeConfigStr)

	// Save kubeconfig to a file in ~/.kube/sloth-kubernetes/
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	kubeconfigDir := filepath.Join(home, ".kube", "sloth-kubernetes")
	if err := os.MkdirAll(kubeconfigDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create kubeconfig directory: %w", err)
	}

	kubeconfigPath := filepath.Join(kubeconfigDir, fmt.Sprintf("%s.yaml", targetStack))
	if err := os.WriteFile(kubeconfigPath, []byte(kubeConfigStr), 0600); err != nil {
		return "", fmt.Errorf("failed to write kubeconfig: %w", err)
	}

	return kubeconfigPath, nil
}

// GetStackInfo retrieves comprehensive info from a stack including kubeconfig, SSH key, and master IP
func GetStackInfo(targetStack string) (*StackKubeconfig, error) {
	if targetStack == "" {
		return nil, fmt.Errorf("stack name is required")
	}

	ctx := context.Background()

	// Create workspace with S3 support
	workspace, err := createWorkspaceWithS3Support(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create workspace: %w", err)
	}

	// Use fully qualified stack name for S3 backend
	fullyQualifiedStackName := fmt.Sprintf("organization/sloth-kubernetes/%s", targetStack)
	stack, err := auto.SelectStack(ctx, fullyQualifiedStackName, workspace)
	if err != nil {
		return nil, fmt.Errorf("failed to select stack '%s': %w", targetStack, err)
	}

	// Get outputs
	outputs, err := stack.Outputs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get stack outputs: %w", err)
	}

	result := &StackKubeconfig{
		ClusterName: targetStack,
	}

	// Get kubeconfig
	if kubeConfigOutput, ok := outputs["kubeConfig"]; ok {
		kubeConfigStr := fmt.Sprintf("%v", kubeConfigOutput.Value)
		if kubeConfigStr != "" && kubeConfigStr != "<nil>" {
			kubeConfigStr = extractKubeconfig(kubeConfigStr)

			home, _ := os.UserHomeDir()
			kubeconfigDir := filepath.Join(home, ".kube", "sloth-kubernetes")
			os.MkdirAll(kubeconfigDir, 0700)

			kubeconfigPath := filepath.Join(kubeconfigDir, fmt.Sprintf("%s.yaml", targetStack))
			if err := os.WriteFile(kubeconfigPath, []byte(kubeConfigStr), 0600); err == nil {
				result.KubeconfigPath = kubeconfigPath
			}
		}
	}

	// Get SSH key path
	if sshKeyOutput, ok := outputs["ssh_private_key_path"]; ok {
		result.SSHKeyPath = fmt.Sprintf("%v", sshKeyOutput.Value)
	} else if sshKeyOutput, ok := outputs["sshPrivateKey"]; ok {
		sshKeyPath := fmt.Sprintf("%v", sshKeyOutput.Value)
		// Expand ~ to home directory
		if strings.HasPrefix(sshKeyPath, "~/") {
			home, _ := os.UserHomeDir()
			sshKeyPath = filepath.Join(home, sshKeyPath[2:])
		}
		result.SSHKeyPath = sshKeyPath
	}

	// Get master IP from nodes output
	// Try __realNodes first
	if nodesOutput, ok := outputs["__realNodes"]; ok {
		nodes, ok := nodesOutput.Value.([]interface{})
		if ok && len(nodes) > 0 {
			for _, nodeInterface := range nodes {
				node, ok := nodeInterface.(map[string]interface{})
				if !ok {
					continue
				}
				// Check if this is a master node
				rolesInterface, hasRoles := node["roles"]
				if hasRoles {
					roles, ok := rolesInterface.([]interface{})
					if ok {
						for _, role := range roles {
							if roleStr, ok := role.(string); ok && (roleStr == "master" || roleStr == "controlplane") {
								// Get wireguard IP or public IP
								if wgIP, ok := node["wireguard_ip"].(string); ok && wgIP != "" {
									result.MasterIP = wgIP
									break
								}
								if pubIP, ok := node["public_ip"].(string); ok && pubIP != "" {
									result.MasterIP = pubIP
									break
								}
							}
						}
					}
				}
				if result.MasterIP != "" {
					break
				}
			}
		}
	}

	// If not found, try the nodes map output
	if result.MasterIP == "" {
		if nodesOutput, ok := outputs["nodes"]; ok {
			nodesMap, ok := nodesOutput.Value.(map[string]interface{})
			if ok {
				for _, nodeInterface := range nodesMap {
					node, ok := nodeInterface.(map[string]interface{})
					if !ok {
						continue
					}
					// Check if this is a master node by name or role
					name, _ := node["name"].(string)
					role, _ := node["role"].(string)
					if strings.Contains(name, "master") || role == "master" || role == "controlplane" {
						// Get public IP
						if pubIP, ok := node["public_ip"].(string); ok && pubIP != "" {
							result.MasterIP = pubIP
							break
						}
					}
				}
			}
		}
	}

	return result, nil
}

// RequireStackArg validates that a stack name is provided either as argument or flag.
// Deprecated: Use RequireStack instead, which also validates the stack exists.
func RequireStackArg(args []string) (string, error) {
	targetStack := stackName
	if len(args) > 0 {
		targetStack = args[0]
	}

	if targetStack == "" {
		// Check if there's only one stack available
		ctx := context.Background()
		workspace, err := createWorkspaceWithS3Support(ctx)
		if err != nil {
			return "", fmt.Errorf("stack name is required. Use: command <stack-name> or --stack <name>")
		}

		stacks, err := workspace.ListStacks(ctx)
		if err != nil || len(stacks) == 0 {
			return "", fmt.Errorf("stack name is required. Use: command <stack-name> or --stack <name>")
		}

		if len(stacks) == 1 {
			return stacks[0].Name, nil
		}

		return "", fmt.Errorf("stack name is required. Available stacks: %v", getStackNames(stacks))
	}

	return targetStack, nil
}

func getStackNames(stacks []auto.StackSummary) []string {
	names := make([]string, len(stacks))
	for i, s := range stacks {
		names[i] = s.Name
	}
	return names
}

// EnsureStackExists verifies a stack exists and is properly configured.
// Returns nil if stack exists, error with helpful message if not.
func EnsureStackExists(targetStack string) error {
	if targetStack == "" {
		return fmt.Errorf(`no stack specified

Create an encrypted stack first:
  sloth-kubernetes stacks create <name> --password-stdin

Or with AWS KMS:
  sloth-kubernetes stacks create <name> --kms-key <arn-or-alias>

List existing stacks:
  sloth-kubernetes stacks list`)
	}

	ctx := context.Background()
	workspace, err := createWorkspaceWithS3Support(ctx)
	if err != nil {
		return fmt.Errorf("failed to access Pulumi backend: %w", err)
	}

	// Check if stack exists
	fullyQualifiedName := fmt.Sprintf("organization/sloth-kubernetes/%s", targetStack)
	_, err = auto.SelectStack(ctx, fullyQualifiedName, workspace)
	if err != nil {
		// Check if there are any stacks available
		stacks, listErr := workspace.ListStacks(ctx)
		if listErr == nil && len(stacks) > 0 {
			return fmt.Errorf(`stack '%s' not found

Available stacks: %v

Create a new encrypted stack:
  sloth-kubernetes stacks create %s --password-stdin`, targetStack, getStackNames(stacks), targetStack)
		}

		return fmt.Errorf(`stack '%s' not found

Create an encrypted stack first:
  sloth-kubernetes stacks create %s --password-stdin

Or with AWS KMS:
  sloth-kubernetes stacks create %s --kms-key <arn-or-alias>`, targetStack, targetStack, targetStack)
	}

	return nil
}

// RequireStack validates that a stack exists and returns the stack name.
// This is the main guard function that should be called at the start of commands.
// It accepts either a positional argument or the --stack flag value.
func RequireStack(args []string) (string, error) {
	targetStack := stackName // from global flag
	if len(args) > 0 {
		targetStack = args[0]
	}

	// If no stack specified, check if there's exactly one stack available
	if targetStack == "" {
		ctx := context.Background()
		workspace, err := createWorkspaceWithS3Support(ctx)
		if err != nil {
			return "", EnsureStackExists("") // Will return helpful error
		}

		stacks, err := workspace.ListStacks(ctx)
		if err != nil || len(stacks) == 0 {
			return "", EnsureStackExists("") // Will return helpful error
		}

		// If exactly one stack exists, use it automatically
		if len(stacks) == 1 {
			targetStack = stacks[0].Name
			fmt.Printf("Using stack: %s\n", targetStack)
		} else {
			return "", fmt.Errorf(`stack name required

Available stacks: %v

Specify a stack:
  command <stack-name>
  command --stack <name>`, getStackNames(stacks))
		}
	}

	// Verify the stack exists
	if err := EnsureStackExists(targetStack); err != nil {
		return "", err
	}

	return targetStack, nil
}
