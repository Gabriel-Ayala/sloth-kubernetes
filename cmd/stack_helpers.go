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

	return result, nil
}

// RequireStackArg validates that a stack name is provided either as argument or flag
func RequireStackArg(args []string) (string, error) {
	targetStack := stackName
	if len(args) > 0 {
		targetStack = args[0]
	}

	if targetStack == "" || targetStack == "production" {
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
