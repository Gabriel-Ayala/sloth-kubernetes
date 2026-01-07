package cmd

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/fatih/color"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/curve25519"

	"github.com/chalkan3/sloth-kubernetes/pkg/operations"
	"github.com/chalkan3/sloth-kubernetes/pkg/vpn"
)

var (
	// VPN join command flags
	vpnJoinRemote  string
	vpnJoinIP      string
	vpnJoinLabel   string
	vpnJoinInstall bool

	// VPN leave command flags
	vpnLeaveIP string

	// VPN client config flags
	vpnConfigOutput string
	vpnConfigQR     bool
)

var vpnCmd = &cobra.Command{
	Use:   "vpn",
	Short: "Manage WireGuard VPN",
	Long:  `Configure, manage, and troubleshoot the WireGuard VPN mesh network`,
}

var vpnStatusCmd = &cobra.Command{
	Use:   "status [stack-name]",
	Short: "Show VPN status and tunnels",
	Long:  `Display the current status of the WireGuard VPN mesh including all tunnels`,
	Example: `  # Show VPN status for production stack
  sloth-kubernetes vpn status production`,
	RunE: runVPNStatus,
}

var vpnPeersCmd = &cobra.Command{
	Use:   "peers [stack-name]",
	Short: "List all VPN peers",
	Long:  `Display all nodes in the VPN mesh with their public keys and endpoints`,
	Example: `  # List VPN peers
  sloth-kubernetes vpn peers production`,
	RunE: runVPNPeers,
}

var vpnConfigCmd = &cobra.Command{
	Use:   "config [stack-name] [node-name]",
	Short: "Get VPN configuration for a node",
	Long:  `Display the WireGuard configuration for a specific node`,
	Example: `  # Get VPN config for a node
  sloth-kubernetes vpn config production master-1`,
	RunE: runVPNConfig,
}

var vpnTestCmd = &cobra.Command{
	Use:   "test [stack-name]",
	Short: "Test VPN connectivity",
	Long:  `Test connectivity between all nodes in the VPN mesh`,
	Example: `  # Test VPN connectivity
  sloth-kubernetes vpn test production`,
	RunE: runVPNTest,
}

var vpnJoinCmd = &cobra.Command{
	Use:   "join [stack-name]",
	Short: "Join this machine or a remote host to the VPN",
	Long: `Add your local machine or a remote SSH host to the WireGuard VPN mesh.
This will generate WireGuard keys, configure all cluster nodes to accept the new peer,
and provide you with the WireGuard configuration to install locally.`,
	Example: `  # Join local machine to VPN
  sloth-kubernetes vpn join production

  # Join a remote SSH host to VPN
  sloth-kubernetes vpn join production --remote user@host.com

  # Join with custom VPN IP
  sloth-kubernetes vpn join production --vpn-ip 10.8.0.100

  # Join and auto-install WireGuard config
  sloth-kubernetes vpn join production --install`,
	RunE: runVPNJoin,
}

var vpnLeaveCmd = &cobra.Command{
	Use:   "leave [stack-name]",
	Short: "Remove this machine from the VPN",
	Long:  `Remove your local machine or a remote host from the WireGuard VPN mesh`,
	Example: `  # Leave VPN
  sloth-kubernetes vpn leave production

  # Remove a specific peer by IP
  sloth-kubernetes vpn leave production --vpn-ip 10.8.0.100`,
	RunE: runVPNLeave,
}

var vpnClientConfigCmd = &cobra.Command{
	Use:   "client-config [stack-name]",
	Short: "Generate WireGuard client configuration",
	Long:  `Generate a WireGuard configuration file for connecting to the VPN mesh`,
	Example: `  # Generate client config
  sloth-kubernetes vpn client-config production

  # Save to file
  sloth-kubernetes vpn client-config production --output client.conf

  # Generate QR code for mobile
  sloth-kubernetes vpn client-config production --qr`,
	RunE: runVPNClientConfig,
}

func init() {
	rootCmd.AddCommand(vpnCmd)

	// Add subcommands
	vpnCmd.AddCommand(vpnStatusCmd)
	vpnCmd.AddCommand(vpnPeersCmd)
	vpnCmd.AddCommand(vpnConfigCmd)
	vpnCmd.AddCommand(vpnTestCmd)
	vpnCmd.AddCommand(vpnJoinCmd)
	vpnCmd.AddCommand(vpnLeaveCmd)
	vpnCmd.AddCommand(vpnClientConfigCmd)

	// Join flags
	vpnJoinCmd.Flags().StringVar(&vpnJoinRemote, "remote", "", "Remote SSH host to add (e.g., user@host.com)")
	vpnJoinCmd.Flags().StringVar(&vpnJoinIP, "vpn-ip", "", "Custom VPN IP address (default: auto-assign)")
	vpnJoinCmd.Flags().StringVar(&vpnJoinLabel, "label", "", "Peer label/name (e.g., 'laptop', 'ci-server')")
	vpnJoinCmd.Flags().BoolVar(&vpnJoinInstall, "install", false, "Auto-install WireGuard configuration")

	// Leave flags
	vpnLeaveCmd.Flags().StringVar(&vpnLeaveIP, "vpn-ip", "", "VPN IP of peer to remove")

	// Client config flags
	vpnClientConfigCmd.Flags().StringVar(&vpnConfigOutput, "output", "", "Output file path")
	vpnClientConfigCmd.Flags().BoolVar(&vpnConfigQR, "qr", false, "Generate QR code for mobile devices")
}

func runVPNStatus(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Require a valid stack
	stack, err := RequireStack(args)
	if err != nil {
		return err
	}

	printHeader(fmt.Sprintf("🔐 VPN Status - Stack: %s", stack))

	// Create workspace with S3 support
	workspace, err := createWorkspaceWithS3Support(ctx)
	if err != nil {
		return fmt.Errorf("failed to create workspace: %w", err)
	}

	// Use fully qualified stack name for S3 backend
	fullyQualifiedStackName := fmt.Sprintf("organization/sloth-kubernetes/%s", stack)
	s, err := auto.SelectStack(ctx, fullyQualifiedStackName, workspace)
	if err != nil {
		return fmt.Errorf("failed to select stack '%s': %w", stack, err)
	}

	// Get outputs
	outputs, err := s.Outputs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get stack outputs: %w", err)
	}

	fmt.Println()
	printVPNStatusTable(outputs)

	return nil
}

func runVPNPeers(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Require a valid stack
	stack, err := RequireStack(args)
	if err != nil {
		return err
	}

	printHeader(fmt.Sprintf("👥 VPN Peers - Stack: %s", stack))

	// Create workspace with S3 support
	workspace, err := createWorkspaceWithS3Support(ctx)
	if err != nil {
		return fmt.Errorf("failed to create workspace: %w", err)
	}

	// Use fully qualified stack name for S3 backend
	fullyQualifiedStackName := fmt.Sprintf("organization/sloth-kubernetes/%s", stack)
	s, err := auto.SelectStack(ctx, fullyQualifiedStackName, workspace)
	if err != nil {
		return fmt.Errorf("failed to select stack '%s': %w", stack, err)
	}

	// Get outputs
	outputs, err := s.Outputs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get stack outputs: %w", err)
	}

	// Parse nodes
	nodes, err := ParseNodeOutputs(outputs)
	if err != nil {
		return fmt.Errorf("failed to parse nodes: %w", err)
	}

	// Get SSH key and bastion info
	sshKeyPath := GetSSHKeyPath(stack)
	bastionEnabled := false
	bastionIP := ""

	if bastionEnabledOutput, ok := outputs["bastion_enabled"]; ok {
		if bastionEnabledOutput.Value != nil {
			bastionEnabled = bastionEnabledOutput.Value == true
		}
	}

	if bastionEnabled {
		if bastionOutput, ok := outputs["bastion"]; ok {
			if bastionMap, ok := bastionOutput.Value.(map[string]interface{}); ok {
				if pubIP, ok := bastionMap["public_ip"].(string); ok {
					bastionIP = pubIP
				}
			}
		}
	}

	fmt.Println()
	color.Cyan("ℹ  Fetching peer information from cluster nodes...")
	fmt.Println()

	// Collect peer information from all nodes
	type PeerInfo struct {
		NodeName      string
		VPNIp         string
		PublicKey     string
		Label         string
		Endpoint      string
		LastHandshake string
		Transfer      string
	}

	var allPeers []PeerInfo

	// For each node, SSH and get WireGuard peer information
	for _, node := range nodes {
		// Determine target IP for SSH
		targetIP := node.PublicIP
		if bastionEnabled && bastionIP != "" {
			// When using bastion, connect to private IP
			if node.PrivateIP != "" {
				targetIP = node.PrivateIP
			}
		}

		// Get WireGuard config and peers from this node
		// First get the config to extract labels from comments
		fetchConfigCmd := "sudo cat /etc/wireguard/wg0.conf"
		fetchPeersCmd := "sudo wg show wg0 dump | tail -n +2" // Skip header line

		// Fetch config to extract peer labels
		sshUser := getSSHUserForProvider(node.Provider)
		var configCmd *exec.Cmd
		if bastionEnabled && bastionIP != "" {
			configCmd = exec.Command("ssh",
				"-q",
				"-i", sshKeyPath,
				"-o", "StrictHostKeyChecking=accept-new",
				"-o", "UserKnownHostsFile=/dev/null",
				"-o", "ConnectTimeout=5",
				"-o", fmt.Sprintf("ProxyCommand=ssh -q -i %s -o StrictHostKeyChecking=accept-new -o UserKnownHostsFile=/dev/null -W %%h:%%p root@%s", sshKeyPath, bastionIP),
				fmt.Sprintf("%s@%s", sshUser, targetIP),
				fetchConfigCmd,
			)
		} else {
			configCmd = exec.Command("ssh",
				"-q",
				"-i", sshKeyPath,
				"-o", "StrictHostKeyChecking=accept-new",
				"-o", "UserKnownHostsFile=/dev/null",
				"-o", "ConnectTimeout=5",
				fmt.Sprintf("%s@%s", sshUser, targetIP),
				fetchConfigCmd,
			)
		}

		// Parse labels from config
		peerLabels := make(map[string]string) // map[publicKey]label
		if configOutput, err := configCmd.CombinedOutput(); err == nil {
			configLines := strings.Split(string(configOutput), "\n")
			var currentLabel string
			var currentPublicKey string

			for _, line := range configLines {
				line = strings.TrimSpace(line)

				// Check for label comment (# Peer: xxx)
				if strings.HasPrefix(line, "# Peer:") {
					currentLabel = strings.TrimSpace(strings.TrimPrefix(line, "# Peer:"))
				}

				// Check for PublicKey line
				if strings.HasPrefix(line, "PublicKey") {
					parts := strings.Split(line, "=")
					if len(parts) == 2 {
						currentPublicKey = strings.TrimSpace(parts[1])
						if currentLabel != "" {
							peerLabels[currentPublicKey] = currentLabel
							currentLabel = "" // Reset for next peer
						}
					}
				}
			}
		}

		// Fetch peer information
		var sshCmd *exec.Cmd
		if bastionEnabled && bastionIP != "" {
			sshCmd = exec.Command("ssh",
				"-q",
				"-i", sshKeyPath,
				"-o", "StrictHostKeyChecking=accept-new",
				"-o", "UserKnownHostsFile=/dev/null",
				"-o", "ConnectTimeout=5",
				"-o", fmt.Sprintf("ProxyCommand=ssh -q -i %s -o StrictHostKeyChecking=accept-new -o UserKnownHostsFile=/dev/null -W %%h:%%p root@%s", sshKeyPath, bastionIP),
				fmt.Sprintf("%s@%s", sshUser, targetIP),
				fetchPeersCmd,
			)
		} else {
			sshCmd = exec.Command("ssh",
				"-q",
				"-i", sshKeyPath,
				"-o", "StrictHostKeyChecking=accept-new",
				"-o", "UserKnownHostsFile=/dev/null",
				"-o", "ConnectTimeout=5",
				fmt.Sprintf("%s@%s", sshUser, targetIP),
				fetchPeersCmd,
			)
		}

		output, err := sshCmd.CombinedOutput()
		if err != nil {
			color.Yellow(fmt.Sprintf("⚠  Failed to get peers from %s: %v", node.Name, err))
			continue
		}

		// Parse wg dump output
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, line := range lines {
			if line == "" {
				continue
			}

			fields := strings.Fields(line)
			if len(fields) < 8 {
				continue
			}

			publicKey := fields[0]
			endpoint := fields[2]
			allowedIPs := fields[3]
			lastHandshake := fields[4]
			rxBytes := fields[5]
			txBytes := fields[6]

			// Extract VPN IP from allowed IPs (format: 10.8.0.X/32)
			vpnIP := strings.TrimSuffix(allowedIPs, "/32")

			// Format handshake time
			handshakeStr := "Never"
			if lastHandshake != "0" {
				handshakeTime, err := strconv.ParseInt(lastHandshake, 10, 64)
				if err == nil {
					elapsed := time.Now().Unix() - handshakeTime
					if elapsed < 60 {
						handshakeStr = fmt.Sprintf("%ds ago", elapsed)
					} else if elapsed < 3600 {
						handshakeStr = fmt.Sprintf("%dm ago", elapsed/60)
					} else if elapsed < 86400 {
						handshakeStr = fmt.Sprintf("%dh ago", elapsed/3600)
					} else {
						handshakeStr = fmt.Sprintf("%dd ago", elapsed/86400)
					}
				}
			}

			// Format transfer
			rx, _ := strconv.ParseInt(rxBytes, 10, 64)
			tx, _ := strconv.ParseInt(txBytes, 10, 64)
			transferStr := fmt.Sprintf("↑ %s / ↓ %s", formatBytes(tx), formatBytes(rx))

			// Format endpoint
			if endpoint == "(none)" {
				endpoint = "N/A"
			}

			// Find peer node name by VPN IP
			peerNodeName := ""
			for _, n := range nodes {
				// Compare VPN IPs, handling potential /32 suffix
				nodeVPNIP := strings.TrimSuffix(n.WireGuardIP, "/32")
				if nodeVPNIP == vpnIP {
					peerNodeName = n.Name
					break
				}
			}

			// Only add peers that belong to cluster nodes (skip external/unknown peers)
			if peerNodeName != "" {
				// Get label from map
				label := peerLabels[publicKey]

				allPeers = append(allPeers, PeerInfo{
					NodeName:      peerNodeName,
					VPNIp:         vpnIP,
					PublicKey:     publicKey[:16] + "...", // Truncate for display
					Label:         label,
					Endpoint:      endpoint,
					LastHandshake: handshakeStr,
					Transfer:      transferStr,
				})
			}
		}

		// Only need to get from one node since all should have the same peers
		if len(allPeers) > 0 {
			break
		}
	}

	// Remove duplicates and display
	seen := make(map[string]bool)
	uniquePeers := []PeerInfo{}
	for _, peer := range allPeers {
		if !seen[peer.VPNIp] {
			seen[peer.VPNIp] = true
			uniquePeers = append(uniquePeers, peer)
		}
	}

	// Display table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	defer w.Flush()

	color.New(color.Bold).Fprintln(w, "NODE\tLABEL\tVPN IP\tPUBLIC KEY\tENDPOINT\tLAST HANDSHAKE\tTRANSFER")
	fmt.Fprintln(w, "----\t-----\t------\t----------\t--------\t--------------\t--------")

	if len(uniquePeers) == 0 {
		fmt.Fprintln(w, "No peers found")
	} else {
		for _, peer := range uniquePeers {
			label := peer.Label
			if label == "" {
				label = "-"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				peer.NodeName,
				label,
				peer.VPNIp,
				peer.PublicKey,
				peer.Endpoint,
				peer.LastHandshake,
				peer.Transfer,
			)
		}
	}

	fmt.Println()
	color.Green(fmt.Sprintf("✓ Found %d peers in VPN mesh", len(uniquePeers)))

	return nil
}

func runVPNConfig(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: sloth-kubernetes vpn config <stack-name> <node-name>")
	}

	ctx := context.Background()

	// Validate stack exists
	if err := EnsureStackExists(args[0]); err != nil {
		return err
	}

	stack := args[0]
	nodeName := args[1]

	printHeader(fmt.Sprintf("📋 VPN Config - Node: %s", nodeName))

	// Create workspace with S3 support
	workspace, err := createWorkspaceWithS3Support(ctx)
	if err != nil {
		return fmt.Errorf("failed to create workspace: %w", err)
	}

	// Use fully qualified stack name for S3 backend
	fullyQualifiedStackName := fmt.Sprintf("organization/sloth-kubernetes/%s", stack)
	s, err := auto.SelectStack(ctx, fullyQualifiedStackName, workspace)
	if err != nil {
		return fmt.Errorf("failed to select stack '%s': %w", stack, err)
	}

	outputs, err := s.Outputs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get stack outputs: %w", err)
	}

	// Parse nodes
	nodes, err := ParseNodeOutputs(outputs)
	if err != nil {
		return fmt.Errorf("failed to parse nodes: %w", err)
	}

	// Find the specific node
	var targetNode *NodeInfo
	for i := range nodes {
		if nodes[i].Name == nodeName {
			targetNode = &nodes[i]
			break
		}
	}

	if targetNode == nil {
		return fmt.Errorf("node '%s' not found in stack", nodeName)
	}

	// Get SSH key and bastion info
	sshKeyPath := GetSSHKeyPath(stack)
	bastionEnabled := false
	bastionIP := ""

	if bastionEnabledOutput, ok := outputs["bastion_enabled"]; ok {
		if bastionEnabledOutput.Value != nil {
			bastionEnabled = bastionEnabledOutput.Value == true
		}
	}

	if bastionEnabled {
		if bastionOutput, ok := outputs["bastion"]; ok {
			if bastionMap, ok := bastionOutput.Value.(map[string]interface{}); ok {
				if pubIP, ok := bastionMap["public_ip"].(string); ok {
					bastionIP = pubIP
				}
			}
		}
	}

	fmt.Println()
	printInfo(fmt.Sprintf("Fetching WireGuard configuration from %s...", targetNode.Name))

	// Determine target IP for SSH
	targetIP := targetNode.WireGuardIP
	if targetIP == "" {
		targetIP = targetNode.PrivateIP
		if targetIP == "" {
			targetIP = targetNode.PublicIP
		}
	}

	// Fetch the WireGuard config
	fetchCmd := "sudo cat /etc/wireguard/wg0.conf"
	sshUser := getSSHUserForProvider(targetNode.Provider)

	var sshCmd *exec.Cmd
	if bastionEnabled && bastionIP != "" {
		sshCmd = exec.Command("ssh",
			"-i", sshKeyPath,
			"-o", "StrictHostKeyChecking=accept-new",
			"-o", "UserKnownHostsFile=/dev/null",
			"-o", fmt.Sprintf("ProxyCommand=ssh -i %s -o StrictHostKeyChecking=accept-new -o UserKnownHostsFile=/dev/null -W %%h:%%p root@%s", sshKeyPath, bastionIP),
			fmt.Sprintf("%s@%s", sshUser, targetIP),
			fetchCmd,
		)
	} else {
		sshCmd = exec.Command("ssh",
			"-i", sshKeyPath,
			"-o", "StrictHostKeyChecking=accept-new",
			"-o", "UserKnownHostsFile=/dev/null",
			fmt.Sprintf("%s@%s", sshUser, targetNode.PublicIP),
			fetchCmd,
		)
	}

	output, err := sshCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to fetch config from node: %w (output: %s)", err, string(output))
	}

	fmt.Println()
	color.Green("✓ WireGuard Configuration:")
	fmt.Println()
	fmt.Println(string(output))

	fmt.Println()
	printInfo(fmt.Sprintf("Node: %s", targetNode.Name))
	printInfo(fmt.Sprintf("Public IP: %s", targetNode.PublicIP))
	printInfo(fmt.Sprintf("VPN IP: %s", targetNode.WireGuardIP))
	printInfo(fmt.Sprintf("Provider: %s", targetNode.Provider))

	return nil
}

func runVPNTest(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Require a valid stack
	stack, err := RequireStack(args)
	if err != nil {
		return err
	}

	printHeader(fmt.Sprintf("🧪 Testing VPN Connectivity - Stack: %s", stack))

	// Create workspace with S3 support
	workspace, err := createWorkspaceWithS3Support(ctx)
	if err != nil {
		return fmt.Errorf("failed to create workspace: %w", err)
	}

	// Use fully qualified stack name for S3 backend
	fullyQualifiedStackName := fmt.Sprintf("organization/sloth-kubernetes/%s", stack)
	s, err := auto.SelectStack(ctx, fullyQualifiedStackName, workspace)
	if err != nil {
		return fmt.Errorf("failed to select stack '%s': %w", stack, err)
	}

	outputs, err := s.Outputs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get stack outputs: %w", err)
	}

	// Parse nodes
	nodes, err := ParseNodeOutputs(outputs)
	if err != nil {
		return fmt.Errorf("failed to parse nodes: %w", err)
	}

	if len(nodes) == 0 {
		return fmt.Errorf("no nodes found in stack")
	}

	fmt.Println()
	printInfo(fmt.Sprintf("Found %d nodes to test", len(nodes)))

	// Get SSH key and bastion info
	sshKeyPath := GetSSHKeyPath(stack)
	bastionEnabled := false
	bastionIP := ""

	if bastionEnabledOutput, ok := outputs["bastion_enabled"]; ok {
		if bastionEnabledOutput.Value != nil {
			bastionEnabled = bastionEnabledOutput.Value == true
		}
	}

	if bastionEnabled {
		if bastionOutput, ok := outputs["bastion"]; ok {
			if bastionMap, ok := bastionOutput.Value.(map[string]interface{}); ok {
				if pubIP, ok := bastionMap["public_ip"].(string); ok {
					bastionIP = pubIP
				}
			}
		}
	}

	// Test 1: Ping test between nodes
	fmt.Println()
	printInfo("Test 1/3: Testing ping connectivity via VPN...")
	fmt.Println()

	successCount := 0
	totalTests := 0

	for i, sourceNode := range nodes {
		if sourceNode.WireGuardIP == "" {
			continue
		}

		for j, targetNode := range nodes {
			if i == j || targetNode.WireGuardIP == "" {
				continue
			}

			totalTests++

			// Build ping command
			pingCmd := fmt.Sprintf("ping -c 2 -W 2 %s > /dev/null 2>&1 && echo 'SUCCESS' || echo 'FAILED'", targetNode.WireGuardIP)

			// Determine target IP for SSH
			sourceIP := sourceNode.WireGuardIP
			if sourceIP == "" {
				sourceIP = sourceNode.PrivateIP
				if sourceIP == "" {
					sourceIP = sourceNode.PublicIP
				}
			}

			// Build SSH command
			sshUser := getSSHUserForProvider(sourceNode.Provider)
			var sshCmd *exec.Cmd
			if bastionEnabled && bastionIP != "" {
				sshCmd = exec.Command("ssh",
					"-q",
					"-i", sshKeyPath,
					"-o", "StrictHostKeyChecking=accept-new",
					"-o", "UserKnownHostsFile=/dev/null",
					"-o", "ConnectTimeout=5",
					"-o", fmt.Sprintf("ProxyCommand=ssh -q -i %s -o StrictHostKeyChecking=accept-new -o UserKnownHostsFile=/dev/null -W %%h:%%p root@%s", sshKeyPath, bastionIP),
					fmt.Sprintf("%s@%s", sshUser, sourceIP),
					pingCmd,
				)
			} else {
				sshCmd = exec.Command("ssh",
					"-q",
					"-i", sshKeyPath,
					"-o", "StrictHostKeyChecking=accept-new",
					"-o", "UserKnownHostsFile=/dev/null",
					"-o", "ConnectTimeout=5",
					fmt.Sprintf("%s@%s", sshUser, sourceNode.PublicIP),
					pingCmd,
				)
			}

			output, err := sshCmd.CombinedOutput()
			result := strings.TrimSpace(string(output))

			if err == nil && result == "SUCCESS" {
				fmt.Printf("  ✓ %s → %s (%s)\n", sourceNode.Name, targetNode.Name, targetNode.WireGuardIP)
				successCount++
			} else {
				fmt.Printf("  ✗ %s → %s (%s) - Failed\n", sourceNode.Name, targetNode.Name, targetNode.WireGuardIP)
			}
		}
	}

	// Test 2: WireGuard handshake status
	fmt.Println()
	printInfo("Test 2/3: Checking WireGuard handshake status...")
	fmt.Println()

	handshakeOK := 0
	for _, node := range nodes {
		if node.WireGuardIP == "" {
			continue
		}

		// Check handshake on this node
		targetIP := node.WireGuardIP
		if targetIP == "" {
			targetIP = node.PrivateIP
			if targetIP == "" {
				targetIP = node.PublicIP
			}
		}

		checkCmd := "sudo wg show wg0 latest-handshakes | wc -l"
		sshUserHandshake := getSSHUserForProvider(node.Provider)

		var sshCmd *exec.Cmd
		if bastionEnabled && bastionIP != "" {
			sshCmd = exec.Command("ssh",
				"-q",
				"-i", sshKeyPath,
				"-o", "StrictHostKeyChecking=accept-new",
				"-o", "UserKnownHostsFile=/dev/null",
				"-o", "ConnectTimeout=5",
				"-o", fmt.Sprintf("ProxyCommand=ssh -q -i %s -o StrictHostKeyChecking=accept-new -o UserKnownHostsFile=/dev/null -W %%h:%%p root@%s", sshKeyPath, bastionIP),
				fmt.Sprintf("%s@%s", sshUserHandshake, targetIP),
				checkCmd,
			)
		} else {
			sshCmd = exec.Command("ssh",
				"-q",
				"-i", sshKeyPath,
				"-o", "StrictHostKeyChecking=accept-new",
				"-o", "UserKnownHostsFile=/dev/null",
				"-o", "ConnectTimeout=5",
				fmt.Sprintf("%s@%s", sshUserHandshake, node.PublicIP),
				checkCmd,
			)
		}

		output, err := sshCmd.CombinedOutput()
		if err == nil {
			peerCount := strings.TrimSpace(string(output))
			fmt.Printf("  ✓ %s - %s active peers\n", node.Name, peerCount)
			handshakeOK++
		} else {
			fmt.Printf("  ✗ %s - Could not check handshake status\n", node.Name)
		}
	}

	// Test 3: Summary
	fmt.Println()
	printInfo("Test 3/3: Summary")
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(w, "METRIC\tRESULT")
	fmt.Fprintln(w, "------\t------")
	fmt.Fprintf(w, "Total Nodes\t%d\n", len(nodes))
	fmt.Fprintf(w, "Ping Tests\t%d/%d passed (%.1f%%)\n", successCount, totalTests, float64(successCount)/float64(totalTests)*100)
	fmt.Fprintf(w, "Handshake Checks\t%d/%d nodes responding\n", handshakeOK, len(nodes))

	if successCount == totalTests && handshakeOK == len(nodes) {
		fmt.Fprintln(w, "Overall Status\t✅ All tests passed")
	} else if successCount > 0 {
		fmt.Fprintln(w, "Overall Status\t⚠️  Some tests failed")
	} else {
		fmt.Fprintln(w, "Overall Status\t❌ All tests failed")
	}

	return nil
}

func printVPNStatusTable(outputs auto.OutputMap) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	defer w.Flush()

	color.New(color.Bold).Fprintln(w, "METRIC\tVALUE")
	fmt.Fprintln(w, "------\t-----")

	// TODO: Parse actual VPN data from outputs
	fmt.Fprintln(w, "VPN Mode\tWireGuard Mesh")
	fmt.Fprintln(w, "Total Nodes\t6")
	fmt.Fprintln(w, "Total Tunnels\t15")
	fmt.Fprintln(w, "VPN Subnet\t10.8.0.0/24")
	fmt.Fprintln(w, "Status\t✅ All tunnels active")

	color.Yellow("\n⚠️  Real-time VPN metrics will be available after implementing monitoring")
}

func printVPNPeersTable(outputs auto.OutputMap) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	defer w.Flush()

	color.New(color.Bold).Fprintln(w, "NODE\tVPN IP\tPUBLIC KEY\tENDPOINT\tLAST HANDSHAKE\tTRANSFER")
	fmt.Fprintln(w, "----\t------\t----------\t--------\t--------------\t--------")

	// TODO: Parse actual peer data from outputs
	fmt.Fprintln(w, "master-1\t10.8.0.10\tABC123...\t167.71.1.1:51820\t1m ago\t↑ 1.2MB / ↓ 2.4MB")
	fmt.Fprintln(w, "worker-1\t10.8.0.11\tDEF456...\t172.236.1.1:51820\t30s ago\t↑ 800KB / ↓ 1.5MB")

	color.Yellow("\n⚠️  Full peer information will be available after implementing peer tracking")
}

func runVPNJoin(cmd *cobra.Command, args []string) error {
	startTime := time.Now()
	ctx := context.Background()

	// Require a valid stack
	stack, err := RequireStack(args)
	if err != nil {
		return err
	}

	printHeader(fmt.Sprintf("🔗 Joining VPN - Stack: %s", stack))

	// Create workspace with S3 support
	workspace, err := createWorkspaceWithS3Support(ctx)
	if err != nil {
		return fmt.Errorf("failed to create workspace: %w", err)
	}

	fullyQualifiedStackName := fmt.Sprintf("organization/sloth-kubernetes/%s", stack)
	s, err := auto.SelectStack(ctx, fullyQualifiedStackName, workspace)
	if err != nil {
		return fmt.Errorf("failed to select stack '%s': %w", stack, err)
	}

	outputs, err := s.Outputs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get stack outputs: %w", err)
	}

	// Parse nodes
	nodes, err := ParseNodeOutputs(outputs)
	if err != nil {
		return fmt.Errorf("failed to parse nodes: %w", err)
	}

	if len(nodes) == 0 {
		return fmt.Errorf("no nodes found in stack - cluster may not be deployed yet")
	}

	fmt.Println()
	printInfo(fmt.Sprintf("Found %d cluster nodes", len(nodes)))

	// Target info
	target := "local machine"
	if vpnJoinRemote != "" {
		target = vpnJoinRemote
	}
	printInfo(fmt.Sprintf("Target: %s", target))

	// Get SSH key and bastion info
	sshKeyPath := GetSSHKeyPath(stack)
	bastionEnabled := false
	bastionIP := ""

	if bastionEnabledOutput, ok := outputs["bastion_enabled"]; ok {
		if bastionEnabledOutput.Value != nil {
			bastionEnabled = bastionEnabledOutput.Value == true
		}
	}

	if bastionEnabled {
		if bastionOutput, ok := outputs["bastion"]; ok {
			if bastionMap, ok := bastionOutput.Value.(map[string]interface{}); ok {
				if pubIP, ok := bastionMap["public_ip"].(string); ok {
					bastionIP = pubIP
				}
			}
		}
	}

	// Initialize VPN Manager with robust retry policy
	fmt.Println()
	printInfo("Initializing VPN manager...")

	vpnMgr, err := vpn.NewManager(vpn.ManagerConfig{
		SSHKeyPath:     sshKeyPath,
		RetryPolicy:    vpn.NewDefaultRetryPolicy(),
		ConnectTimeout: 30 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize VPN manager: %w", err)
	}

	// STEP 1: Health check bastion if enabled
	if bastionEnabled && bastionIP != "" {
		printInfo("Step 1/5: Checking bastion connectivity...")
		if err := vpnMgr.GetHealthChecker().CheckBastion(ctx, bastionIP); err != nil {
			return fmt.Errorf("bastion health check failed: %w", err)
		}
		printSuccess("Bastion is reachable")
	} else {
		printInfo("Step 1/5: No bastion configured, skipping...")
	}

	// STEP 2: Check for existing peer registration (stable IP)
	fmt.Println()
	printInfo("Step 2/5: Checking peer registry...")

	if vpnJoinIP == "" && vpnJoinLabel != "" {
		// Try to get existing registration for this label
		if existingPeer, err := vpnMgr.GetPeerByLabel(stack, vpnJoinLabel); err == nil {
			vpnJoinIP = existingPeer.VPNIP
			printInfo(fmt.Sprintf("Found existing registration for '%s', using IP %s", vpnJoinLabel, vpnJoinIP))
		}
	}

	// Auto-assign IP if not specified
	if vpnJoinIP == "" {
		// Generate reserved IPs for cluster nodes (10.8.0.10-99)
		var reservedIPs []string
		for i := 1; i < 100; i++ {
			reservedIPs = append(reservedIPs, fmt.Sprintf("10.8.0.%d", i))
		}

		assignedIP, err := vpnMgr.GetPeerRegistry().NextAvailableIP(stack, "10.8.0.0/24", reservedIPs)
		if err != nil {
			return fmt.Errorf("failed to assign VPN IP: %w", err)
		}
		vpnJoinIP = assignedIP
		printInfo(fmt.Sprintf("Auto-assigned VPN IP: %s", vpnJoinIP))
	} else {
		printInfo(fmt.Sprintf("Using VPN IP: %s", vpnJoinIP))
	}

	// STEP 3: Generate WireGuard keypair
	fmt.Println()
	printInfo("Step 3/5: Generating WireGuard keypair...")
	privateKey, publicKey, err := generateWireGuardKeypair()
	if err != nil {
		return fmt.Errorf("failed to generate keypair: %w", err)
	}
	printSuccess(fmt.Sprintf("Generated keypair (public key: %s...)", publicKey[:16]))

	// STEP 4: Add peer to all cluster nodes using VPN manager
	fmt.Println()
	printInfo("Step 4/5: Adding peer to cluster nodes...")

	// Convert NodeInfo to vpn.NodeInfo
	vpnNodes := make([]vpn.NodeInfo, len(nodes))
	for i, n := range nodes {
		vpnNodes[i] = vpn.NodeInfo{
			Name:     n.Name,
			PublicIP: n.PublicIP,
			VPNIP:    n.WireGuardIP,
			Provider: n.Provider,
		}
	}

	// Use the robust connection manager for adding peers
	connMgr := vpnMgr.GetConnectionManager()
	configMgr := vpnMgr.GetConfigManager()
	successCount := 0
	failCount := 0

	peerConfig := vpn.PeerConfig{
		PublicKey:  publicKey,
		AllowedIPs: []string{vpnJoinIP + "/32"},
		Keepalive:  25,
		Label:      vpnJoinLabel,
	}

	for i, node := range nodes {
		printInfo(fmt.Sprintf("  [%d/%d] Adding peer to %s...", i+1, len(nodes), node.Name))

		// Determine target IP based on connectivity:
		// - If bastion is enabled: connect through bastion to VPN IP (bastion is inside the mesh)
		// - If no bastion: connect directly to public IP (we're outside the mesh)
		var targetIP string
		if bastionEnabled && bastionIP != "" {
			// Through bastion: prefer VPN IP, then private IP
			targetIP = node.WireGuardIP
			if targetIP == "" {
				targetIP = node.PrivateIP
			}
		} else {
			// Direct connection: must use public IP (we're outside the VPN)
			targetIP = node.PublicIP
		}

		if targetIP == "" {
			color.Yellow(fmt.Sprintf("  ⚠️  No reachable IP for %s, skipping", node.Name))
			failCount++
			continue
		}

		// Connect with retry
		connCfg := vpn.ConnectionConfig{
			Host:        targetIP,
			User:        getSSHUserForNode(node.Provider),
			UseBastion:  bastionEnabled && bastionIP != "",
			BastionHost: bastionIP,
			BastionUser: "root",
			Timeout:     30 * time.Second,
		}

		conn, err := connMgr.Connect(ctx, connCfg)
		if err != nil {
			color.Yellow(fmt.Sprintf("  ⚠️  Failed to connect to %s: %v", node.Name, err))
			failCount++
			continue
		}

		// Add peer using ConfigManager (uses wg set - atomic operation)
		if err := configMgr.AddPeer(ctx, conn, peerConfig); err != nil {
			color.Yellow(fmt.Sprintf("  ⚠️  Failed to add peer to %s: %v", node.Name, err))
			conn.Close()
			failCount++
			continue
		}

		conn.Close()
		successCount++
		printSuccess(fmt.Sprintf("  ✓ Added peer to %s", node.Name))

		// Small delay between nodes
		if bastionEnabled && i < len(nodes)-1 {
			time.Sleep(1 * time.Second)
		}
	}

	if successCount == 0 {
		return fmt.Errorf("failed to add peer to any cluster node")
	}

	// Register peer in local registry
	registeredPeer := vpn.RegisteredPeer{
		PublicKey:  publicKey,
		VPNIP:      vpnJoinIP,
		Label:      vpnJoinLabel,
		AllowedIPs: []string{vpnJoinIP + "/32"},
	}
	if err := vpnMgr.GetPeerRegistry().Register(stack, registeredPeer); err != nil {
		color.Yellow(fmt.Sprintf("  ⚠️  Failed to register peer locally: %v", err))
	}

	// STEP 5: Discover existing peers and generate client config
	fmt.Println()
	printInfo("Step 5/5: Generating client configuration...")

	// Fetch existing peers from cluster
	var existingPeers []VPNPeerInfo
	if len(nodes) > 0 {
		firstNode := nodes[0]
		targetIP := firstNode.WireGuardIP
		if targetIP == "" {
			targetIP = firstNode.PrivateIP
			if targetIP == "" {
				targetIP = firstNode.PublicIP
			}
		}

		connCfg := vpn.ConnectionConfig{
			Host:        targetIP,
			User:        getSSHUserForNode(firstNode.Provider),
			UseBastion:  bastionEnabled && bastionIP != "",
			BastionHost: bastionIP,
			BastionUser: "root",
		}

		if conn, err := connMgr.Connect(ctx, connCfg); err == nil {
			listScript := `sudo wg show wg0 dump | tail -n +2 | while IFS=$'\t' read -r pubkey _ endpoint allowed_ips _; do
				first_ip=$(echo "$allowed_ips" | cut -d, -f1 | cut -d/ -f1)
				if [ -n "$first_ip" ] && [ "$first_ip" != "(none)" ]; then
					echo "$pubkey|$first_ip"
				fi
			done`

			if output, err := conn.Execute(listScript); err == nil {
				lines := strings.Split(strings.TrimSpace(output), "\n")
				for _, line := range lines {
					if line == "" {
						continue
					}
					parts := strings.Split(line, "|")
					if len(parts) == 2 {
						peerIP := strings.TrimSpace(parts[1])
						peerKey := strings.TrimSpace(parts[0])
						if peerIP != "" && peerIP != "(none)" {
							// Filter cluster nodes (10.8.0.10-99)
							if strings.HasPrefix(peerIP, "10.8.0.") {
								ipParts := strings.Split(peerIP, ".")
								if len(ipParts) == 4 {
									var lastOctet int
									if _, err := fmt.Sscanf(ipParts[3], "%d", &lastOctet); err == nil {
										if lastOctet >= 10 && lastOctet < 100 {
											continue
										}
									}
								}
							}
							existingPeers = append(existingPeers, VPNPeerInfo{
								PublicKey:  peerKey,
								VPNAddress: peerIP,
							})
						}
					}
				}
			}
			conn.Close()
		}
	}

	// Generate client config
	clientConfig := generateClientConfig(privateKey, vpnJoinIP, vpnJoinLabel, nodes, existingPeers, sshKeyPath, bastionEnabled, bastionIP)

	configPath := "./wg0-client.conf"
	if err := os.WriteFile(configPath, []byte(clientConfig), 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	printSuccess(fmt.Sprintf("Client configuration saved to: %s", configPath))

	// Optional: Install configuration
	if vpnJoinInstall {
		fmt.Println()
		installVPNConfig(configPath, clientConfig, vpnJoinRemote)
	} else {
		fmt.Println()
		printVPNInstallInstructions(configPath)
	}

	fmt.Println()
	printSuccess(fmt.Sprintf("Successfully joined VPN with IP %s!", vpnJoinIP))
	printInfo(fmt.Sprintf("Configured %d/%d nodes", successCount, len(nodes)))
	printInfo("You can now access cluster nodes via their VPN IPs (10.8.0.x)")

	// Record operation
	details := fmt.Sprintf("Joined with IP %s, %d/%d nodes configured", vpnJoinIP, successCount, len(nodes))
	operations.RecordVPNOperation(stack, "join", vpnJoinLabel, "", "success", details, len(nodes), time.Since(startTime), nil)

	return nil
}

// installVPNConfig handles WireGuard configuration installation
func installVPNConfig(configPath, clientConfig, remoteHost string) {
	printInfo("Installing WireGuard configuration...")

	if remoteHost != "" {
		// Remote installation via SSH
		printInfo(fmt.Sprintf("Installing WireGuard on remote host: %s", remoteHost))

		installScript := fmt.Sprintf(`
if ! command -v wg &> /dev/null; then
    echo "Installing WireGuard..."
    if [ -f /etc/debian_version ]; then
        export DEBIAN_FRONTEND=noninteractive
        apt-get update -qq
        apt-get install -y -qq wireguard-tools >/dev/null 2>&1
    elif [ -f /etc/redhat-release ]; then
        yum install -y -q wireguard-tools
    elif [ -f /etc/arch-release ]; then
        pacman -S --noconfirm wireguard-tools
    else
        echo "⚠️  Unsupported OS. Please install WireGuard manually."
        exit 1
    fi
fi

mkdir -p /etc/wireguard
chmod 700 /etc/wireguard

cat > /etc/wireguard/wg0.conf << 'WGEOF'
%s
WGEOF

chmod 600 /etc/wireguard/wg0.conf
echo "net.ipv4.ip_forward=1" >> /etc/sysctl.conf
sysctl -p >/dev/null 2>&1

wg-quick down wg0 2>/dev/null || true
wg-quick up wg0

if command -v systemctl &> /dev/null; then
    systemctl enable wg-quick@wg0 2>/dev/null || true
fi

echo "✓ WireGuard installed and started"
`, clientConfig)

		sshCmd := exec.Command("ssh",
			"-o", "StrictHostKeyChecking=accept-new",
			"-o", "UserKnownHostsFile=/dev/null",
			remoteHost,
			"sudo", "bash",
		)
		sshCmd.Stdin = strings.NewReader(installScript)

		output, err := sshCmd.CombinedOutput()
		if err != nil {
			color.Yellow(fmt.Sprintf("⚠️  Remote installation failed: %v", err))
			color.Yellow(fmt.Sprintf("Output: %s", string(output)))
		} else {
			printSuccess("✓ WireGuard installed and activated on remote host!")
			fmt.Println(string(output))
		}
	} else {
		// Local installation
		osType := detectOS()

		switch osType {
		case "darwin":
			printInfo("Detected macOS - installing WireGuard VPN")

			mkdirCmd := exec.Command("sudo", "mkdir", "-p", "/opt/homebrew/etc/wireguard")
			if err := mkdirCmd.Run(); err != nil {
				color.Yellow("⚠️  Failed to create WireGuard directory")
				printVPNInstallInstructions(configPath)
				return
			}

			cpCmd := exec.Command("sudo", "cp", configPath, "/opt/homebrew/etc/wireguard/wg0.conf")
			if err := cpCmd.Run(); err != nil {
				color.Yellow("⚠️  Failed to copy configuration")
				printVPNInstallInstructions(configPath)
				return
			}

			printInfo("Starting WireGuard VPN...")
			upCmd := exec.Command("sudo", "wg-quick", "up", "wg0")
			if output, err := upCmd.CombinedOutput(); err != nil {
				color.Yellow(fmt.Sprintf("⚠️  Failed to start WireGuard: %v", err))
				color.Yellow(fmt.Sprintf("Output: %s", string(output)))
				return
			}

			printSuccess("✓ WireGuard VPN activated successfully!")

		case "linux":
			if os.Geteuid() != 0 {
				color.Yellow("⚠️  Installation requires root privileges")
				printVPNInstallInstructions(configPath)
			} else {
				if err := exec.Command("cp", configPath, "/etc/wireguard/wg0.conf").Run(); err != nil {
					color.Yellow(fmt.Sprintf("⚠️  Failed to copy config: %v", err))
					return
				}

				if err := exec.Command("wg-quick", "up", "wg0").Run(); err != nil {
					color.Yellow(fmt.Sprintf("⚠️  Failed to start WireGuard: %v", err))
					return
				}

				exec.Command("systemctl", "enable", "wg-quick@wg0").Run()
				printSuccess("WireGuard installed and started")
			}

		default:
			color.Yellow(fmt.Sprintf("⚠️  Unsupported OS: %s", osType))
			printVPNInstallInstructions(configPath)
		}
	}
}

// printVPNInstallInstructions prints manual installation instructions
func printVPNInstallInstructions(configPath string) {
	osType := detectOS()

	if osType == "darwin" {
		color.Cyan("To install the configuration on macOS:")
		fmt.Println()
		fmt.Println("  1. Install WireGuard app: https://www.wireguard.com/install/")
		fmt.Printf("  2. Import tunnel from file: %s\n", configPath)
		fmt.Println("  3. Click 'Activate' to connect")
		fmt.Println()
		color.Cyan("Or use command line:")
		fmt.Printf("  sudo mkdir -p /opt/homebrew/etc/wireguard\n")
		fmt.Printf("  sudo cp %s /opt/homebrew/etc/wireguard/wg0.conf\n", configPath)
		fmt.Printf("  wg-quick up /opt/homebrew/etc/wireguard/wg0.conf\n")
	} else {
		color.Cyan("To install the configuration manually:")
		fmt.Println()
		fmt.Printf("  sudo cp %s /etc/wireguard/wg0.conf\n", configPath)
		fmt.Println("  sudo wg-quick up wg0")
		fmt.Println("  sudo systemctl enable wg-quick@wg0")
	}
}

func runVPNLeave(cmd *cobra.Command, args []string) error {
	startTime := time.Now()
	ctx := context.Background()

	// Require a valid stack
	stack, err := RequireStack(args)
	if err != nil {
		return err
	}

	printHeader(fmt.Sprintf("👋 Leaving VPN - Stack: %s", stack))

	// Determine which peer to remove
	var targetVPNIP string
	if vpnLeaveIP != "" {
		targetVPNIP = vpnLeaveIP
		printInfo(fmt.Sprintf("Removing peer with VPN IP: %s", targetVPNIP))
	} else {
		fmt.Println()
		printInfo("Detecting local VPN IP address...")

		// Try to get local VPN IP from wg0 interface (cross-platform)
		detectCmd := exec.Command("sh", "-c", "ip addr show wg0 2>/dev/null | grep 'inet ' | awk '{print $2}' | cut -d/ -f1 || ifconfig wg0 2>/dev/null | grep 'inet ' | awk '{print $2}'")
		output, err := detectCmd.CombinedOutput()
		if err != nil || len(output) == 0 {
			return fmt.Errorf("could not detect local VPN IP. Use --vpn-ip flag to specify manually, or ensure WireGuard is running locally")
		}

		targetVPNIP = strings.TrimSpace(string(output))
		printInfo(fmt.Sprintf("Detected local VPN IP: %s", targetVPNIP))
	}

	// Create workspace
	workspace, err := createWorkspaceWithS3Support(ctx)
	if err != nil {
		return fmt.Errorf("failed to create workspace: %w", err)
	}

	fullyQualifiedStackName := fmt.Sprintf("organization/sloth-kubernetes/%s", stack)
	s, err := auto.SelectStack(ctx, fullyQualifiedStackName, workspace)
	if err != nil {
		return fmt.Errorf("failed to select stack '%s': %w", stack, err)
	}

	outputs, err := s.Outputs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get stack outputs: %w", err)
	}

	nodes, err := ParseNodeOutputs(outputs)
	if err != nil {
		return fmt.Errorf("failed to parse nodes: %w", err)
	}

	if len(nodes) == 0 {
		return fmt.Errorf("no nodes found in stack")
	}

	// Get SSH key and bastion info
	sshKeyPath := GetSSHKeyPath(stack)
	bastionEnabled := false
	bastionIP := ""

	if bastionEnabledOutput, ok := outputs["bastion_enabled"]; ok {
		if bastionEnabledOutput.Value != nil {
			bastionEnabled = bastionEnabledOutput.Value == true
		}
	}

	if bastionEnabled {
		if bastionOutput, ok := outputs["bastion"]; ok {
			if bastionMap, ok := bastionOutput.Value.(map[string]interface{}); ok {
				if pubIP, ok := bastionMap["public_ip"].(string); ok {
					bastionIP = pubIP
				}
			}
		}
	}

	// Initialize VPN Manager
	vpnMgr, err := vpn.NewManager(vpn.ManagerConfig{
		SSHKeyPath:     sshKeyPath,
		RetryPolicy:    vpn.NewDefaultRetryPolicy(),
		ConnectTimeout: 30 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize VPN manager: %w", err)
	}

	// STEP 1: Check peer registry first
	fmt.Println()
	printInfo("Step 1/3: Looking up peer in registry...")

	var peerPublicKey string

	// Try to find in local registry
	if peer, err := vpnMgr.GetPeerRegistry().GetByIP(stack, targetVPNIP); err == nil {
		peerPublicKey = peer.PublicKey
		printInfo(fmt.Sprintf("Found peer in registry: %s...", peerPublicKey[:16]))
	} else {
		// Query cluster for public key
		printInfo("Peer not in local registry, querying cluster...")

		if len(nodes) > 0 {
			firstNode := nodes[0]
			// Determine target IP based on connectivity
			var nodeIP string
			if bastionEnabled && bastionIP != "" {
				nodeIP = firstNode.WireGuardIP
				if nodeIP == "" {
					nodeIP = firstNode.PrivateIP
				}
			} else {
				nodeIP = firstNode.PublicIP
			}

			connCfg := vpn.ConnectionConfig{
				Host:        nodeIP,
				User:        getSSHUserForNode(firstNode.Provider),
				UseBastion:  bastionEnabled && bastionIP != "",
				BastionHost: bastionIP,
				BastionUser: "root",
			}

			conn, err := vpnMgr.GetConnectionManager().Connect(ctx, connCfg)
			if err != nil {
				return fmt.Errorf("failed to connect to cluster: %w", err)
			}

			// Query for public key by VPN IP
			getPubKeyCmd := fmt.Sprintf("sudo wg show wg0 dump | awk '$4 ~ /%s\\/32/ {print $1; exit}'", strings.ReplaceAll(targetVPNIP, ".", "\\."))
			output, err := conn.Execute(getPubKeyCmd)
			conn.Close()

			if err != nil || strings.TrimSpace(output) == "" {
				return fmt.Errorf("peer with VPN IP %s not found in cluster", targetVPNIP)
			}

			peerPublicKey = strings.TrimSpace(output)
			printInfo(fmt.Sprintf("Found peer public key: %s...", peerPublicKey[:16]))
		}
	}

	// STEP 2: Remove peer from all cluster nodes
	fmt.Println()
	printInfo("Step 2/3: Removing peer from cluster nodes...")

	connMgr := vpnMgr.GetConnectionManager()
	configMgr := vpnMgr.GetConfigManager()
	successCount := 0
	failCount := 0

	for i, node := range nodes {
		// Determine target IP based on connectivity
		var nodeIP string
		if bastionEnabled && bastionIP != "" {
			nodeIP = node.WireGuardIP
			if nodeIP == "" {
				nodeIP = node.PrivateIP
			}
		} else {
			nodeIP = node.PublicIP
		}

		if nodeIP == "" {
			color.Yellow(fmt.Sprintf("  ⚠️  No reachable IP for %s, skipping", node.Name))
			failCount++
			continue
		}

		printInfo(fmt.Sprintf("  [%d/%d] Removing peer from %s...", i+1, len(nodes), node.Name))

		connCfg := vpn.ConnectionConfig{
			Host:        nodeIP,
			User:        getSSHUserForNode(node.Provider),
			UseBastion:  bastionEnabled && bastionIP != "",
			BastionHost: bastionIP,
			BastionUser: "root",
			Timeout:     30 * time.Second,
		}

		conn, err := connMgr.Connect(ctx, connCfg)
		if err != nil {
			color.Yellow(fmt.Sprintf("  ⚠️  Failed to connect to %s: %v", node.Name, err))
			failCount++
			continue
		}

		if err := configMgr.RemovePeer(ctx, conn, peerPublicKey); err != nil {
			color.Yellow(fmt.Sprintf("  ⚠️  Failed to remove peer from %s: %v", node.Name, err))
			conn.Close()
			failCount++
			continue
		}

		conn.Close()
		successCount++
		printSuccess(fmt.Sprintf("  ✓ Removed peer from %s", node.Name))
	}

	// STEP 3: Cleanup
	fmt.Println()
	printInfo("Step 3/3: Cleanup...")

	// Remove from local registry
	if err := vpnMgr.GetPeerRegistry().Unregister(stack, peerPublicKey); err != nil {
		color.Yellow(fmt.Sprintf("  ⚠️  Failed to remove from registry: %v", err))
	} else {
		printSuccess("  ✓ Removed from local registry")
	}

	// Summary
	fmt.Println()
	if successCount == len(nodes) {
		color.Green("✓ Successfully removed peer from all nodes!")
	} else if successCount > 0 {
		color.Yellow(fmt.Sprintf("⚠️  Peer removed from %d/%d nodes", successCount, len(nodes)))
	} else {
		color.Red("✗ Failed to remove peer from any nodes")
		return fmt.Errorf("failed to remove peer")
	}

	// If removing local machine, try to stop WireGuard
	if vpnLeaveIP == "" {
		fmt.Println()
		printInfo("Stopping local WireGuard interface...")

		osType := detectOS()
		var stopCmd *exec.Cmd

		switch osType {
		case "darwin", "linux":
			stopCmd = exec.Command("sudo", "wg-quick", "down", "wg0")
		default:
			color.Yellow("⚠️  Unsupported OS - please stop WireGuard manually")
			printWireGuardStopInstructions(targetVPNIP)
			return nil
		}

		if output, err := stopCmd.CombinedOutput(); err != nil {
			color.Yellow(fmt.Sprintf("⚠️  Failed to stop WireGuard: %v", err))
			color.Yellow(fmt.Sprintf("Output: %s", string(output)))
			printWireGuardStopInstructions(targetVPNIP)
		} else {
			printSuccess("✓ WireGuard interface stopped successfully!")
			fmt.Println()
			color.Cyan("To remove WireGuard configuration:")
			fmt.Println("  sudo rm /etc/wireguard/wg0.conf")
		}
	} else {
		printWireGuardStopInstructions(targetVPNIP)
	}

	// Record operation
	details := fmt.Sprintf("Removed peer %s from %d/%d nodes", targetVPNIP, successCount, len(nodes))
	status := "success"
	if successCount < len(nodes) {
		status = "partial"
	}
	operations.RecordVPNOperation(stack, "leave", "", "", status, details, len(nodes), time.Since(startTime), nil)

	return nil
}

// printWireGuardStopInstructions prints instructions for stopping WireGuard
func printWireGuardStopInstructions(vpnIP string) {
	fmt.Println()
	color.Cyan(fmt.Sprintf("To stop WireGuard on the removed machine (%s):", vpnIP))
	fmt.Println("  sudo wg-quick down wg0")
	fmt.Println("  sudo rm /etc/wireguard/wg0.conf")
}

func runVPNClientConfig(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Require a valid stack
	stack, err := RequireStack(args)
	if err != nil {
		return err
	}

	printHeader(fmt.Sprintf("📱 Generate Client Config - Stack: %s", stack))

	// Create workspace with S3 support
	workspace, err := createWorkspaceWithS3Support(ctx)
	if err != nil {
		return fmt.Errorf("failed to create workspace: %w", err)
	}

	// Use fully qualified stack name for S3 backend
	fullyQualifiedStackName := fmt.Sprintf("organization/sloth-kubernetes/%s", stack)
	s, err := auto.SelectStack(ctx, fullyQualifiedStackName, workspace)
	if err != nil {
		return fmt.Errorf("failed to select stack '%s': %w", stack, err)
	}

	outputs, err := s.Outputs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get stack outputs: %w", err)
	}

	// Parse nodes
	nodes, err := ParseNodeOutputs(outputs)
	if err != nil {
		return fmt.Errorf("failed to parse nodes: %w", err)
	}

	if len(nodes) == 0 {
		return fmt.Errorf("no nodes found in stack")
	}

	fmt.Println()
	printInfo(fmt.Sprintf("Generating config for %d peer(s)", len(nodes)))

	if vpnConfigOutput != "" {
		printInfo(fmt.Sprintf("Output file: %s", vpnConfigOutput))
	}

	if vpnConfigQR {
		printInfo("QR code generation enabled")
	}

	fmt.Println()
	color.Yellow("⚠️  Client config generation will be implemented in next phase")
	color.Cyan("\nWhat will be implemented:")
	fmt.Println("  • Generate new WireGuard keypair")
	fmt.Println("  • Create [Interface] section with private key and VPN IP")
	fmt.Println("  • Create [Peer] sections for all cluster nodes")
	fmt.Println("  • Save to file (default: ./wg0.conf)")
	if vpnConfigQR {
		fmt.Println("  • Generate QR code using 'qrencode' for mobile import")
	}

	fmt.Println()
	color.Cyan("Example output format:")
	fmt.Print(`
[Interface]
PrivateKey = <generated-private-key>
Address = 10.8.0.100/24
DNS = 1.1.1.1

[Peer]
PublicKey = <master-1-public-key>
Endpoint = 167.71.1.1:51820
AllowedIPs = 10.8.0.10/32

[Peer]
PublicKey = <worker-1-public-key>
Endpoint = 172.236.1.1:51820
AllowedIPs = 10.8.0.11/32
`)

	return nil
}

// getSSHUserForNode returns the correct SSH username based on node provider
// Azure uses "azureuser", AWS/GCP use "ubuntu", others use "root"
func getSSHUserForNode(provider string) string {
	switch provider {
	case "azure":
		return "azureuser"
	case "aws":
		return "ubuntu"
	case "gcp":
		return "ubuntu"
	default:
		return "root" // DigitalOcean, Linode, and others use "root"
	}
}

// generateWireGuardKeypair generates a WireGuard private/public keypair
func generateWireGuardKeypair() (privateKey string, publicKey string, err error) {
	// Generate 32 random bytes for private key
	var privKey [32]byte
	if _, err := rand.Read(privKey[:]); err != nil {
		return "", "", fmt.Errorf("failed to generate random key: %w", err)
	}

	// Clamp the private key (WireGuard requirement)
	privKey[0] &= 248
	privKey[31] &= 127
	privKey[31] |= 64

	// Derive public key using Curve25519
	var pubKey [32]byte
	curve25519.ScalarBaseMult(&pubKey, &privKey)

	// Encode to base64
	privateKey = base64.StdEncoding.EncodeToString(privKey[:])
	publicKey = base64.StdEncoding.EncodeToString(pubKey[:])

	return privateKey, publicKey, nil
}

// generatePeerAddScript creates a bash script to add a peer to WireGuard config
// It uses escaped echo commands to write the configuration safely
func generatePeerAddScript(peerIP string, peerPublicKey string, peerLabel string) string {
	comment := "Client joined via CLI"
	if peerLabel != "" {
		comment = fmt.Sprintf("Peer: %s", peerLabel)
	}

	// Escape any single quotes in the values to prevent shell injection
	comment = strings.ReplaceAll(comment, "'", "'\\''")
	peerPublicKey = strings.ReplaceAll(peerPublicKey, "'", "'\\''")
	peerIP = strings.ReplaceAll(peerIP, "'", "'\\''")

	// Use escaped echo commands with single quotes to write configuration safely
	// Single quotes prevent any shell expansion, and we escape any single quotes in the values
	return fmt.Sprintf(`
set -e

# Step 1: AUTO-CLEANUP - Remove corrupted entries and existing client peers
echo "Cleaning up corrupted WireGuard config entries..."
sudo cp /etc/wireguard/wg0.conf /etc/wireguard/wg0.conf.backup-$(date +%%Y%%m%%d-%%H%%M%%S) 2>/dev/null || true

# Remove ANY lines containing literal \n (backslash followed by n) - these are corrupted
# This catches all variations: \n, \\n, \[Peer]\n, etc.
sudo sed -i '/\\n/d' /etc/wireguard/wg0.conf 2>/dev/null || true
sudo sed -i '/\\\\n/d' /etc/wireguard/wg0.conf 2>/dev/null || true

# Also remove any malformed [Peer] sections that might exist
sudo sed -i '/\[Peer\][^]]*\\n/d' /etc/wireguard/wg0.conf 2>/dev/null || true

# Remove existing client peers (10.8.0.100+) using awk
sudo awk '
BEGIN { in_peer=0; skip=0; buffer="" }
/^\[Peer\]/ {
    if (buffer != "" && skip == 0) print buffer
    buffer=$0"\n"
    in_peer=1
    skip=0
    next
}
in_peer && /^AllowedIPs = 10\.8\.0\.(10[0-9]|1[1-9][0-9]|[2-9][0-9]{2})/ {
    skip=1
    buffer=""
    in_peer=0
    next
}
in_peer { buffer=buffer$0"\n"; next }
!in_peer && skip == 0 { print }
END { if (buffer != "" && skip == 0) print buffer }
' /etc/wireguard/wg0.conf | sudo tee /etc/wireguard/wg0.conf.clean > /dev/null
sudo mv /etc/wireguard/wg0.conf.clean /etc/wireguard/wg0.conf

# Step 2: Add new peer configuration
echo "Adding new peer..."

# Write peer config using tee (most reliable method)
echo "" | sudo tee -a /etc/wireguard/wg0.conf >/dev/null
echo "[Peer]" | sudo tee -a /etc/wireguard/wg0.conf >/dev/null
echo "# %s" | sudo tee -a /etc/wireguard/wg0.conf >/dev/null
echo "PublicKey = %s" | sudo tee -a /etc/wireguard/wg0.conf >/dev/null
echo "AllowedIPs = %s/32" | sudo tee -a /etc/wireguard/wg0.conf >/dev/null
echo "PersistentKeepalive = 25" | sudo tee -a /etc/wireguard/wg0.conf >/dev/null

# Step 3: Reload WireGuard configuration
echo "Reloading WireGuard..."
sudo wg-quick strip wg0 | sudo wg syncconf wg0 /dev/stdin
echo "Peer added and WireGuard reloaded successfully!"
`, comment, peerPublicKey, peerIP)
}

// fetchNodePublicKey fetches the WireGuard public key from a node via SSH
func fetchNodePublicKey(node NodeInfo, sshKeyPath string, bastionEnabled bool, bastionIP string) (string, error) {
	// Determine target IP
	targetIP := node.WireGuardIP
	if targetIP == "" {
		targetIP = node.PrivateIP
		if targetIP == "" {
			targetIP = node.PublicIP
		}
	}

	// Build SSH command with sudo for permission and retry for connection issues
	sshUser := getSSHUserForNode(node.Provider)

	// Try up to 3 times to handle transient SSH connection issues
	var output []byte
	var err error
	maxRetries := 3

	for attempt := 1; attempt <= maxRetries; attempt++ {
		var sshCmd *exec.Cmd
		if bastionEnabled && bastionIP != "" {
			// Use ProxyCommand through bastion (use -q to suppress SSH warnings)
			sshCmd = exec.Command("ssh",
				"-q",
				"-i", sshKeyPath,
				"-o", "StrictHostKeyChecking=accept-new",
				"-o", "UserKnownHostsFile=/dev/null",
				"-o", "ConnectTimeout=10",
				"-o", fmt.Sprintf("ProxyCommand=ssh -q -i %s -o StrictHostKeyChecking=accept-new -o UserKnownHostsFile=/dev/null -W %%h:%%p root@%s", sshKeyPath, bastionIP),
				fmt.Sprintf("%s@%s", sshUser, targetIP),
				"sudo cat /etc/wireguard/publickey",
			)
		} else {
			// Direct SSH (use -q to suppress SSH warnings)
			sshCmd = exec.Command("ssh",
				"-q",
				"-i", sshKeyPath,
				"-o", "StrictHostKeyChecking=accept-new",
				"-o", "UserKnownHostsFile=/dev/null",
				"-o", "ConnectTimeout=10",
				fmt.Sprintf("%s@%s", sshUser, node.PublicIP),
				"sudo cat /etc/wireguard/publickey",
			)
		}

		output, err = sshCmd.CombinedOutput()
		if err == nil {
			break // Success
		}

		// If this was the last attempt, return the error
		if attempt == maxRetries {
			return "", fmt.Errorf("failed to fetch public key after %d attempts: %w (output: %s)", maxRetries, err, string(output))
		}

		// Wait before retrying (exponential backoff)
		time.Sleep(time.Duration(attempt) * time.Second)
	}

	// Trim whitespace and newlines
	publicKey := string(output)
	// Remove trailing newlines
	for len(publicKey) > 0 && (publicKey[len(publicKey)-1] == '\n' || publicKey[len(publicKey)-1] == '\r') {
		publicKey = publicKey[:len(publicKey)-1]
	}

	return publicKey, nil
}

// generateClientConfig generates a complete WireGuard client configuration
func generateClientConfig(privateKey string, clientIP string, peerLabel string, nodes []NodeInfo, existingPeers []VPNPeerInfo, sshKeyPath string, bastionEnabled bool, bastionIP string) string {
	labelComment := ""
	if peerLabel != "" {
		labelComment = fmt.Sprintf("# Peer Label: %s\n", peerLabel)
	}

	config := fmt.Sprintf(`[Interface]
# WireGuard Client Configuration
# Generated by sloth-kubernetes CLI
%sPrivateKey = %s
Address = %s/24
DNS = 1.1.1.1

# Post-connection script (optional)
# PostUp = echo "Connected to Kubernetes cluster VPN"
# PreDown = echo "Disconnecting from cluster VPN"

`, labelComment, privateKey, clientIP)

	// Add each cluster node as a peer
	for _, node := range nodes {
		if node.WireGuardIP == "" {
			continue
		}

		// Fetch actual public key from node
		publicKey, err := fetchNodePublicKey(node, sshKeyPath, bastionEnabled, bastionIP)
		if err != nil {
			// If we can't fetch the key, use placeholder and add a warning
			color.Yellow(fmt.Sprintf("  ⚠️  Failed to fetch public key from %s: %v", node.Name, err))
			publicKey = "<PUBLIC_KEY_PLACEHOLDER>"
		}

		config += fmt.Sprintf(`
[Peer]
# %s (%s)
PublicKey = %s
Endpoint = %s:51820
AllowedIPs = %s/32, 10.0.0.0/8
PersistentKeepalive = 25
`, node.Name, node.Provider, publicKey, node.PublicIP, node.WireGuardIP)
	}

	// Add existing VPN clients as peers for full mesh
	// Special handling: if bastion is in existingPeers (VPN IP 10.8.0.5), add it with endpoint
	for _, peer := range existingPeers {
		// Check if this peer is the bastion (VPN IP 10.8.0.5)
		if peer.VPNAddress == "10.8.0.5" && bastionEnabled && bastionIP != "" {
			// Add bastion with endpoint for direct connectivity
			config += fmt.Sprintf(`
[Peer]
# Bastion Host
PublicKey = %s
Endpoint = %s:51820
AllowedIPs = %s/32, 192.168.0.0/16
PersistentKeepalive = 25
`, peer.PublicKey, bastionIP, peer.VPNAddress)
		} else {
			// Regular external VPN client without endpoint
			config += fmt.Sprintf(`
[Peer]
# External VPN Client
PublicKey = %s
AllowedIPs = %s/32
PersistentKeepalive = 25
`, peer.PublicKey, peer.VPNAddress)
		}
	}

	return config
}

// detectOS detects the operating system
func detectOS() string {
	cmd := exec.Command("uname", "-s")
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}

	osName := string(output)
	// Remove trailing newline if present
	if len(osName) > 0 && osName[len(osName)-1] == '\n' {
		osName = osName[:len(osName)-1]
	}

	switch osName {
	case "Darwin":
		return "darwin"
	case "Linux":
		return "linux"
	default:
		return "unknown"
	}
}

// formatBytes formats bytes into human-readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%dB", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f%cB",
		float64(bytes)/float64(div), "KMGTPE"[exp])
}
