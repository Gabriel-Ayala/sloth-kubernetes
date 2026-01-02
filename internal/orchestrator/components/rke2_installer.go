package components

import (
	"fmt"

	"github.com/pulumi/pulumi-command/sdk/go/command/remote"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
)

// getSSHUserForProviderRKE2 returns the correct SSH username for the given cloud provider
func getSSHUserForProviderRKE2(provider pulumi.StringOutput) pulumi.StringOutput {
	return provider.ApplyT(func(p string) string {
		switch p {
		case "azure":
			return "azureuser"
		case "aws":
			return "ubuntu"
		case "gcp":
			return "ubuntu"
		default:
			return "root"
		}
	}).(pulumi.StringOutput)
}

// RKE2RealComponent represents a real RKE2 Kubernetes cluster
type RKE2RealComponent struct {
	pulumi.ResourceState

	Status        pulumi.StringOutput `pulumi:"status"`
	KubeConfig    pulumi.StringOutput `pulumi:"kubeConfig"`
	MasterCount   pulumi.IntOutput    `pulumi:"masterCount"`
	WorkerCount   pulumi.IntOutput    `pulumi:"workerCount"`
	ClusterToken  pulumi.StringOutput `pulumi:"clusterToken"`
	FirstMasterIP pulumi.StringOutput `pulumi:"firstMasterIP"`
}

// NewRKE2RealComponent deploys a REAL RKE2 cluster
func NewRKE2RealComponent(ctx *pulumi.Context, name string, nodes []*RealNodeComponent, sshPrivateKey pulumi.StringOutput, cfg *config.ClusterConfig, bastionComponent *BastionComponent, opts ...pulumi.ResourceOption) (*RKE2RealComponent, error) {
	component := &RKE2RealComponent{}
	err := ctx.RegisterComponentResource("kubernetes-create:cluster:RKE2Real", name, component, opts...)
	if err != nil {
		return nil, err
	}

	// Separate nodes into masters and workers
	var masters []*RealNodeComponent
	var workers []*RealNodeComponent

	ctx.Log.Info("🔍 Separating nodes for RKE2 (first 3 = masters, rest = workers)...", nil)

	for i, node := range nodes {
		if i < 3 {
			masters = append(masters, node)
		} else {
			workers = append(workers, node)
		}
	}

	ctx.Log.Info(fmt.Sprintf("🚀 Installing RKE2: %d masters, %d workers", len(masters), len(workers)), nil)

	// Get RKE2 version from config
	rke2Version := "stable"
	if cfg.Kubernetes.RKE2 != nil && cfg.Kubernetes.RKE2.Version != "" {
		rke2Version = cfg.Kubernetes.RKE2.Version
	}

	// Cluster token
	clusterToken := "rke2-super-secret-cluster-token-2025"
	if cfg.Kubernetes.RKE2 != nil && cfg.Kubernetes.RKE2.ClusterToken != "" {
		clusterToken = cfg.Kubernetes.RKE2.ClusterToken
	}
	clusterTokenOutput := pulumi.String(clusterToken).ToStringOutput()

	// STEP 1: Install RKE2 on first master node
	firstMaster := masters[0]
	ctx.Log.Info("📦 Installing RKE2 on first master (cluster init)...", nil)

	firstMasterSSHUser := getSSHUserForProviderRKE2(firstMaster.Provider)

	firstMasterConnArgs := remote.ConnectionArgs{
		Host:           firstMaster.PublicIP,
		User:           firstMasterSSHUser,
		PrivateKey:     sshPrivateKey,
		DialErrorLimit: pulumi.Int(30),
	}
	if bastionComponent != nil {
		firstMasterConnArgs.Proxy = &remote.ProxyConnectionArgs{
			Host:       bastionComponent.PublicIP,
			User:       getSSHUserForProvider(bastionComponent.Provider),
			PrivateKey: sshPrivateKey,
		}
	}

	firstMasterInstall, err := remote.NewCommand(ctx, fmt.Sprintf("%s-master-0-install", name), &remote.CommandArgs{
		Connection: firstMasterConnArgs,
		Create: pulumi.All(firstMaster.WireGuardIP, firstMaster.PublicIP, clusterTokenOutput).ApplyT(func(args []interface{}) string {
			wgIP := args[0].(string)
			publicIP := args[1].(string)
			token := args[2].(string)

			return fmt.Sprintf(`#!/bin/bash
set -e

echo "🔧 Installing RKE2 on first master..."

# Wait for WireGuard to be ready (optimized: 20s timeout, 1s polling)
echo "⏳ Waiting for WireGuard VPN interface (wg0)..."
timeout=20
elapsed=0
while [ $elapsed -lt $timeout ]; do
  if ip addr show wg0 &>/dev/null && ip addr show wg0 | grep -q "%s"; then
    break
  fi
  sleep 1
  elapsed=$((elapsed + 1))
done

echo "✅ WireGuard ready (IP: %s)"

# Create RKE2 config directory
sudo mkdir -p /etc/rancher/rke2

# Create RKE2 config
cat <<EOF | sudo tee /etc/rancher/rke2/config.yaml
node-ip: %s
node-external-ip: %s
advertise-address: %s
tls-san:
  - %s
  - %s
  - 127.0.0.1
token: %s
cni: calico
disable:
  - rke2-ingress-nginx
write-kubeconfig-mode: "0644"
EOF

echo "📥 Downloading RKE2 installer..."
curl -sfL https://get.rke2.io | INSTALL_RKE2_CHANNEL=%s sudo sh -

echo "🚀 Starting RKE2 server..."
sudo systemctl enable rke2-server.service
sudo systemctl start rke2-server.service

# Wait for RKE2 to be ready
echo "⏳ Waiting for RKE2 server to be ready..."
timeout=180
elapsed=0
while [ $elapsed -lt $timeout ]; do
  if sudo /var/lib/rancher/rke2/bin/kubectl --kubeconfig /etc/rancher/rke2/rke2.yaml get nodes &>/dev/null; then
    break
  fi
  sleep 5
  elapsed=$((elapsed + 5))
  echo "  Still waiting... (${elapsed}s)"
done

if [ $elapsed -ge $timeout ]; then
  echo "❌ RKE2 server failed to start in time"
  sudo journalctl -u rke2-server -n 50 --no-pager
  exit 1
fi

echo "✅ RKE2 server is ready!"

# Set up kubectl
sudo ln -sf /var/lib/rancher/rke2/bin/kubectl /usr/local/bin/kubectl
mkdir -p ~/.kube
sudo cp /etc/rancher/rke2/rke2.yaml ~/.kube/config
sudo chown $(id -u):$(id -g) ~/.kube/config

# Update kubeconfig to use VPN IP
sudo sed -i 's/127.0.0.1/%s/g' /etc/rancher/rke2/rke2.yaml
sed -i 's/127.0.0.1/%s/g' ~/.kube/config

echo "✅ Kubeconfig updated to use VPN IP %s"

# Show nodes
echo "📋 Cluster nodes:"
kubectl get nodes -o wide

# Output kubeconfig
echo "---KUBECONFIG_START---"
cat /etc/rancher/rke2/rke2.yaml
echo "---KUBECONFIG_END---"
`, wgIP, wgIP, wgIP, publicIP, wgIP, wgIP, publicIP, token, rke2Version, wgIP, wgIP, wgIP)
		}).(pulumi.StringOutput),
	}, pulumi.Parent(component), pulumi.Timeouts(&pulumi.CustomTimeouts{
		Create: "15m",
	}))
	if err != nil {
		return nil, fmt.Errorf("failed to install RKE2 on first master: %w", err)
	}

	// Extract kubeconfig from first master output
	kubeConfig := firstMasterInstall.Stdout.ApplyT(func(output string) string {
		start := "---KUBECONFIG_START---"
		end := "---KUBECONFIG_END---"
		startIdx := len(output) - len(end)
		endIdx := 0
		for i := 0; i < len(output)-len(start); i++ {
			if output[i:i+len(start)] == start {
				startIdx = i + len(start) + 1
				break
			}
		}
		for i := startIdx; i < len(output)-len(end); i++ {
			if output[i:i+len(end)] == end {
				endIdx = i
				break
			}
		}
		if endIdx > startIdx {
			return output[startIdx:endIdx]
		}
		return output
	}).(pulumi.StringOutput)

	// Get join token
	ctx.Log.Info("🔑 Fetching RKE2 join token from first master...", nil)

	fetchToken, err := remote.NewCommand(ctx, fmt.Sprintf("%s-fetch-token", name), &remote.CommandArgs{
		Connection: firstMasterConnArgs,
		Create:     pulumi.String("sudo cat /var/lib/rancher/rke2/server/node-token"),
	}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{firstMasterInstall}))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch join token: %w", err)
	}

	joinToken := fetchToken.Stdout

	// STEP 2: Install RKE2 on additional masters (parallel)
	ctx.Log.Info("🚀 Installing RKE2 on additional masters (if any)...", nil)
	var additionalMasterCmds []pulumi.Resource

	for i := 1; i < len(masters); i++ {
		master := masters[i]
		masterSSHUser := getSSHUserForProviderRKE2(master.Provider)

		masterConnArgs := remote.ConnectionArgs{
			Host:           master.PublicIP,
			User:           masterSSHUser,
			PrivateKey:     sshPrivateKey,
			DialErrorLimit: pulumi.Int(30),
		}
		if bastionComponent != nil {
			masterConnArgs.Proxy = &remote.ProxyConnectionArgs{
				Host:       bastionComponent.PublicIP,
				User:       getSSHUserForProvider(bastionComponent.Provider),
				PrivateKey: sshPrivateKey,
			}
		}

		masterCmd, err := remote.NewCommand(ctx, fmt.Sprintf("%s-master-%d-join", name, i), &remote.CommandArgs{
			Connection: masterConnArgs,
			Create: pulumi.All(master.WireGuardIP, master.PublicIP, firstMaster.WireGuardIP, joinToken).ApplyT(func(args []interface{}) string {
				wgIP := args[0].(string)
				publicIP := args[1].(string)
				firstMasterWgIP := args[2].(string)
				token := args[3].(string)

				return fmt.Sprintf(`#!/bin/bash
set -e

echo "🔧 Installing RKE2 on additional master..."

# Wait for WireGuard (optimized: 20s timeout, 1s polling)
echo "⏳ Waiting for WireGuard..."
timeout=20
elapsed=0
while [ $elapsed -lt $timeout ]; do
  if ip addr show wg0 &>/dev/null; then
    break
  fi
  sleep 1
  elapsed=$((elapsed + 1))
done

# Create RKE2 config
sudo mkdir -p /etc/rancher/rke2
cat <<EOF | sudo tee /etc/rancher/rke2/config.yaml
server: https://%s:9345
token: %s
node-ip: %s
node-external-ip: %s
cni: calico
write-kubeconfig-mode: "0644"
EOF

# Install RKE2
curl -sfL https://get.rke2.io | INSTALL_RKE2_CHANNEL=%s sudo sh -

# Start RKE2 server
sudo systemctl enable rke2-server.service
sudo systemctl start rke2-server.service

# Wait for node to join (poll instead of hard sleep)
echo "⏳ Waiting for node to join cluster..."
timeout=30
elapsed=0
while [ $elapsed -lt $timeout ]; do
  if sudo systemctl is-active --quiet rke2-server; then
    break
  fi
  sleep 2
  elapsed=$((elapsed + 2))
done

echo "✅ Additional master joined successfully"
`, firstMasterWgIP, token, wgIP, publicIP, rke2Version)
			}).(pulumi.StringOutput),
		}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{fetchToken}), pulumi.Timeouts(&pulumi.CustomTimeouts{
			Create: "15m",
		}))
		if err != nil {
			ctx.Log.Warn(fmt.Sprintf("Failed to join master %d: %v", i, err), nil)
			continue
		}
		additionalMasterCmds = append(additionalMasterCmds, masterCmd)
	}

	// STEP 3: Install RKE2 on workers (parallel)
	ctx.Log.Info(fmt.Sprintf("🚀 Installing RKE2 on %d workers...", len(workers)), nil)
	var workerCmds []pulumi.Resource

	for i, worker := range workers {
		workerSSHUser := getSSHUserForProviderRKE2(worker.Provider)

		workerConnArgs := remote.ConnectionArgs{
			Host:           worker.PublicIP,
			User:           workerSSHUser,
			PrivateKey:     sshPrivateKey,
			DialErrorLimit: pulumi.Int(30),
		}
		if bastionComponent != nil {
			workerConnArgs.Proxy = &remote.ProxyConnectionArgs{
				Host:       bastionComponent.PublicIP,
				User:       getSSHUserForProvider(bastionComponent.Provider),
				PrivateKey: sshPrivateKey,
			}
		}

		workerCmd, err := remote.NewCommand(ctx, fmt.Sprintf("%s-worker-%d-join", name, i), &remote.CommandArgs{
			Connection: workerConnArgs,
			Create: pulumi.All(worker.WireGuardIP, worker.PublicIP, firstMaster.WireGuardIP, joinToken).ApplyT(func(args []interface{}) string {
				wgIP := args[0].(string)
				publicIP := args[1].(string)
				firstMasterWgIP := args[2].(string)
				token := args[3].(string)

				return fmt.Sprintf(`#!/bin/bash
set -e

echo "🔧 Installing RKE2 agent on worker..."

# Wait for WireGuard (optimized: 20s timeout, 1s polling)
echo "⏳ Waiting for WireGuard..."
timeout=20
elapsed=0
while [ $elapsed -lt $timeout ]; do
  if ip addr show wg0 &>/dev/null; then
    break
  fi
  sleep 1
  elapsed=$((elapsed + 1))
done

# Create RKE2 config
sudo mkdir -p /etc/rancher/rke2
cat <<EOF | sudo tee /etc/rancher/rke2/config.yaml
server: https://%s:9345
token: %s
node-ip: %s
node-external-ip: %s
EOF

# Install RKE2 agent
curl -sfL https://get.rke2.io | INSTALL_RKE2_CHANNEL=%s INSTALL_RKE2_TYPE="agent" sudo sh -

# Start RKE2 agent
sudo systemctl enable rke2-agent.service
sudo systemctl start rke2-agent.service

echo "⏳ Waiting for agent to connect..."
timeout=20
elapsed=0
while [ $elapsed -lt $timeout ]; do
  if sudo systemctl is-active --quiet rke2-agent; then
    break
  fi
  sleep 2
  elapsed=$((elapsed + 2))
done

echo "✅ Worker joined successfully"
`, firstMasterWgIP, token, wgIP, publicIP, rke2Version)
			}).(pulumi.StringOutput),
		}, pulumi.Parent(component), pulumi.DependsOn([]pulumi.Resource{fetchToken}), pulumi.Timeouts(&pulumi.CustomTimeouts{
			Create: "15m",
		}))
		if err != nil {
			ctx.Log.Warn(fmt.Sprintf("Failed to join worker %d: %v", i, err), nil)
			continue
		}
		workerCmds = append(workerCmds, workerCmd)
	}

	ctx.Log.Info(fmt.Sprintf("✅ RKE2 cluster DEPLOYED: %d masters, %d workers", len(masters), len(workers)), nil)

	// Set component outputs
	component.Status = pulumi.Sprintf("RKE2 cluster deployed with %d masters and %d workers", len(masters), len(workers))
	component.KubeConfig = kubeConfig
	component.MasterCount = pulumi.Int(len(masters)).ToIntOutput()
	component.WorkerCount = pulumi.Int(len(workers)).ToIntOutput()
	component.ClusterToken = clusterTokenOutput
	component.FirstMasterIP = firstMaster.WireGuardIP

	if err := ctx.RegisterResourceOutputs(component, pulumi.Map{
		"status":        component.Status,
		"kubeConfig":    component.KubeConfig,
		"masterCount":   component.MasterCount,
		"workerCount":   component.WorkerCount,
		"clusterToken":  component.ClusterToken,
		"firstMasterIP": component.FirstMasterIP,
	}); err != nil {
		return nil, err
	}

	return component, nil
}
