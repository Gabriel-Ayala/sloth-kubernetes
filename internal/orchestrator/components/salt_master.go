package components

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/pulumi/pulumi-command/sdk/go/command/remote"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// SaltMasterComponent installs and configures Salt Master on a node
type SaltMasterComponent struct {
	pulumi.ResourceState

	MasterIP     pulumi.StringOutput `pulumi:"masterIP"`
	APIURL       pulumi.StringOutput `pulumi:"apiURL"`
	APIUsername  pulumi.StringOutput `pulumi:"apiUsername"`
	APIPassword  pulumi.StringOutput `pulumi:"apiPassword"`
	ClusterToken pulumi.StringOutput `pulumi:"clusterToken"` // Secure token for minion authentication
	Status       pulumi.StringOutput `pulumi:"status"`
}

// generateClusterToken creates a secure hash token for minion authentication
// The token is generated from cluster name + stack name + timestamp
func generateClusterToken(clusterName, stackName string) string {
	// Create a unique seed combining cluster info and timestamp
	seed := fmt.Sprintf("%s-%s-%d", clusterName, stackName, time.Now().UnixNano())

	// Generate SHA256 hash
	hash := sha256.Sum256([]byte(seed))

	// Return first 32 chars of hex string for manageability
	return hex.EncodeToString(hash[:])[:32]
}

// NewSaltMasterComponent creates a Salt Master on the specified node
// Uses secure hash-based authentication instead of auto-accept
func NewSaltMasterComponent(
	ctx *pulumi.Context,
	name string,
	targetNode *RealNodeComponent,
	sshPrivateKey pulumi.StringOutput,
	bastionComponent *BastionComponent,
	opts ...pulumi.ResourceOption,
) (*SaltMasterComponent, error) {
	component := &SaltMasterComponent{}
	err := ctx.RegisterComponentResource("kubernetes-create:salt:Master", name, component, opts...)
	if err != nil {
		return nil, err
	}

	ctx.Log.Info("", nil)
	ctx.Log.Info("════════════════════════════════════════════════════════════", nil)
	ctx.Log.Info("🧂 SALT MASTER INSTALLATION", nil)
	ctx.Log.Info("════════════════════════════════════════════════════════════", nil)
	ctx.Log.Info("🔐 Using secure hash-based minion authentication", nil)
	ctx.Log.Info("", nil)

	// Get SSH user for the target node
	sshUser := getSSHUserForProvider(targetNode.Provider)

	// Build connection args
	connArgs := remote.ConnectionArgs{
		Host:           targetNode.PublicIP,
		User:           sshUser,
		PrivateKey:     sshPrivateKey,
		DialErrorLimit: pulumi.Int(30),
	}
	if bastionComponent != nil {
		connArgs.Proxy = &remote.ProxyConnectionArgs{
			Host:       bastionComponent.PublicIP,
			User:       getSSHUserForProvider(bastionComponent.Provider),
			PrivateKey: sshPrivateKey,
		}
	}

	// Generate secure credentials
	clusterToken := generateClusterToken(name, ctx.Stack())
	apiPassword := fmt.Sprintf("salt-%s", clusterToken[:16])
	apiUsername := "saltapi"

	ctx.Log.Info(fmt.Sprintf("🔑 Generated cluster token: %s...", clusterToken[:8]), nil)

	// Install Salt Master with secure authentication
	installCmd, err := remote.NewCommand(ctx, fmt.Sprintf("%s-install", name), &remote.CommandArgs{
		Connection: connArgs,
		Create: pulumi.All(targetNode.WireGuardIP, targetNode.NodeName).ApplyT(func(args []interface{}) string {
			wgIP := args[0].(string)
			nodeName := args[1].(string)

			return fmt.Sprintf(`#!/bin/bash
set -e

echo "🧂 Installing Salt Master on %s..."
echo "🔐 Configuring secure hash-based authentication..."

# Check if Salt Master is already installed
if command -v salt-master &> /dev/null; then
    echo "✅ Salt Master already installed"
else
    echo "📦 Installing Salt Master..."

    # Install Salt using bootstrap script
    curl -o /tmp/bootstrap-salt.sh -L https://github.com/saltstack/salt-bootstrap/releases/latest/download/bootstrap-salt.sh
    chmod +x /tmp/bootstrap-salt.sh

    # Install Salt Master with API (-M = Master, -W = Salt API with CherryPy, -N = no minion on master)
    sudo sh /tmp/bootstrap-salt.sh -M -W -N stable

    echo "✅ Salt Master and API installed"
fi

# Ensure CherryPy dependencies are installed for Salt API
echo "📦 Ensuring Salt API dependencies..."
sudo apt-get update -qq
sudo apt-get install -y python3-cherrypy3 python3-ws4py 2>/dev/null || {
    pip3 install cherrypy ws4py 2>/dev/null || true
}

# Configure Salt Master with SECURE authentication
echo "⚙️  Configuring Salt Master with hash-based auth..."
sudo mkdir -p /etc/salt/master.d
sudo mkdir -p /etc/salt/autosign_grains

# Main master configuration - NO AUTO ACCEPT
cat <<'MASTERCONF' | sudo tee /etc/salt/master.d/master.conf
# Salt Master Configuration
# Listen on WireGuard IP for cluster-internal communication
interface: %s

# SECURITY: Disable auto-accept - use grain-based autosign
auto_accept: False

# Enable autosign based on grains (secure token validation)
autosign_grains_dir: /etc/salt/autosign_grains

# File roots
file_roots:
  base:
    - /srv/salt

# Pillar roots
pillar_roots:
  base:
    - /srv/pillar

# Enable logging
log_level: info
log_file: /var/log/salt/master

# Performance tuning
worker_threads: 5
timeout: 60

# Security: Reject minions with changed keys
open_mode: False
MASTERCONF

# Create autosign grain file for cluster_token
# Only minions presenting this exact token will be auto-accepted
echo "🔐 Configuring cluster token for secure autosign..."
cat <<'AUTOSIGN' | sudo tee /etc/salt/autosign_grains/cluster_token
%s
AUTOSIGN

# Salt API configuration with sharedsecret auth (more reliable than PAM)
echo "🔐 Configuring Salt API with sharedsecret authentication..."
cat <<'APICONF' | sudo tee /etc/salt/master.d/api.conf
# Salt API Configuration
rest_cherrypy:
  port: 8000
  host: 0.0.0.0
  disable_ssl: true

# Enable API clients
netapi_enable_clients:
  - local
  - local_async
  - runner
  - wheel

# Sharedsecret auth - password serves as the shared secret
sharedsecret: %s

external_auth:
  sharedsecret:
    %s:
      - .*
      - '@wheel'
      - '@runner'
      - '@jobs'
APICONF

echo "✅ Salt API configured with sharedsecret authentication"

# Create Salt directories
sudo mkdir -p /srv/salt /srv/pillar

# Create a basic top.sls
cat <<'TOPSLS' | sudo tee /srv/salt/top.sls
base:
  '*':
    - common
  'master*':
    - kubernetes.master
  'worker*':
    - kubernetes.worker
TOPSLS

# Create common state with security hardening
sudo mkdir -p /srv/salt/common
cat <<'COMMONSLS' | sudo tee /srv/salt/common/init.sls
# Common configuration for all nodes
timezone:
  timezone.system:
    - name: UTC

net.ipv4.ip_forward:
  sysctl.present:
    - value: 1

# Security: Disable IP source routing
net.ipv4.conf.all.accept_source_route:
  sysctl.present:
    - value: 0

net.ipv4.conf.default.accept_source_route:
  sysctl.present:
    - value: 0

# Ensure essential packages
essential_packages:
  pkg.installed:
    - pkgs:
      - curl
      - wget
      - git
      - htop
      - vim
COMMONSLS

# Create kubernetes state directories
sudo mkdir -p /srv/salt/kubernetes

# Create reactor to log key acceptance (audit trail)
sudo mkdir -p /srv/reactor
cat <<'REACTOR' | sudo tee /srv/reactor/auth_log.sls
# Log all key authentication events for audit
log_auth_event:
  local.cmd.run:
    - tgt: salt-master
    - arg:
      - 'echo "[$(date)] Key auth event: {{ data.get("id", "unknown") }} - {{ data.get("act", "unknown") }}" >> /var/log/salt/auth_audit.log'
REACTOR

# Configure reactor in master
cat <<'REACTORCONF' | sudo tee /etc/salt/master.d/reactor.conf
# Reactor configuration for audit logging
reactor:
  - 'salt/auth':
    - /srv/reactor/auth_log.sls
REACTORCONF

# Enable and start services
echo "🚀 Starting Salt Master services..."
sudo systemctl daemon-reload
sudo systemctl enable salt-master
sudo systemctl restart salt-master

# Install and start Salt API
echo "🌐 Configuring Salt API service..."

# Create salt-api systemd service file if it doesn't exist
if [ ! -f /etc/systemd/system/salt-api.service ] && [ ! -f /lib/systemd/system/salt-api.service ]; then
    echo "📝 Creating salt-api systemd service..."
    cat <<'SALTAPISERVICE' | sudo tee /lib/systemd/system/salt-api.service
[Unit]
Description=The Salt API
Documentation=man:salt-api(1) file:///usr/share/doc/salt/html/contents.html https://docs.saltproject.io/en/latest/contents.html
After=network.target salt-master.service
Wants=salt-master.service

[Service]
Type=notify
NotifyAccess=all
LimitNOFILE=100000
ExecStart=/usr/bin/salt-api
TimeoutStopSec=3

[Install]
WantedBy=multi-user.target
SALTAPISERVICE
    sudo systemctl daemon-reload
fi

# Enable and start Salt API
echo "🚀 Starting Salt API..."
sudo systemctl enable salt-api
sudo systemctl restart salt-api

# Wait for Salt Master to be ready
echo "⏳ Waiting for Salt services to be ready..."
sleep 5

# Verify services are running
if sudo systemctl is-active --quiet salt-master; then
    echo "✅ Salt Master is running"
else
    echo "❌ Salt Master failed to start"
    sudo journalctl -u salt-master -n 20 --no-pager
    exit 1
fi

# Check Salt API with retry
echo "🔍 Verifying Salt API..."
for i in 1 2 3; do
    if sudo systemctl is-active --quiet salt-api; then
        echo "✅ Salt API is running on port 8000"
        break
    else
        if [ $i -lt 3 ]; then
            echo "⏳ Waiting for Salt API to start (attempt $i/3)..."
            sleep 3
            sudo systemctl restart salt-api
        else
            echo "❌ Salt API failed to start"
            sudo journalctl -u salt-api -n 20 --no-pager
            # Don't exit - Salt API is not critical for basic operation
        fi
    fi
done

# Verify API is responding
if command -v curl &> /dev/null; then
    if curl -s -o /dev/null -w '' http://localhost:8000/ 2>/dev/null; then
        echo "✅ Salt API is responding on http://localhost:8000"
    fi
fi

# Create auth audit log
sudo touch /var/log/salt/auth_audit.log
sudo chmod 640 /var/log/salt/auth_audit.log

echo ""
echo "════════════════════════════════════════════════════════════"
echo "✅ SALT MASTER READY (SECURE MODE)"
echo "════════════════════════════════════════════════════════════"
echo "  Master IP: %s"
echo "  API URL: http://%s:8000"
echo "  API User: %s"
echo "  Auth Mode: Sharedsecret (cluster_token grain for minions)"
echo "  Audit Log: /var/log/salt/auth_audit.log"
echo "════════════════════════════════════════════════════════════"
`, nodeName, wgIP, clusterToken, apiPassword, apiUsername, wgIP, wgIP, apiUsername)
		}).(pulumi.StringOutput),
	}, pulumi.Parent(component), pulumi.Timeouts(&pulumi.CustomTimeouts{
		Create: "10m",
	}))
	if err != nil {
		return nil, fmt.Errorf("failed to install Salt Master: %w", err)
	}

	// Set component outputs
	component.MasterIP = targetNode.WireGuardIP
	component.APIURL = targetNode.WireGuardIP.ApplyT(func(ip string) string {
		return fmt.Sprintf("http://%s:8000", ip)
	}).(pulumi.StringOutput)
	component.APIUsername = pulumi.String(apiUsername).ToStringOutput()
	component.APIPassword = pulumi.String(apiPassword).ToStringOutput()
	component.ClusterToken = pulumi.String(clusterToken).ToStringOutput()
	component.Status = installCmd.Stdout.ApplyT(func(out string) string {
		return "Salt Master installed and running (secure mode)"
	}).(pulumi.StringOutput)

	ctx.Log.Info("✅ Salt Master installation initiated (secure hash-based auth)", nil)

	if err := ctx.RegisterResourceOutputs(component, pulumi.Map{
		"masterIP":     component.MasterIP,
		"apiURL":       component.APIURL,
		"apiUsername":  component.APIUsername,
		"apiPassword":  component.APIPassword,
		"clusterToken": component.ClusterToken,
		"status":       component.Status,
	}); err != nil {
		return nil, err
	}

	return component, nil
}

// SaltMinionJoinComponent configures Salt Minions on all nodes to join the master
// Uses secure cluster token for authentication
type SaltMinionJoinComponent struct {
	pulumi.ResourceState

	JoinedMinions pulumi.IntOutput    `pulumi:"joinedMinions"`
	Status        pulumi.StringOutput `pulumi:"status"`
}

// NewSaltMinionJoinComponent installs and configures Salt Minion on all nodes
// Minions are configured with the cluster token for secure authentication
func NewSaltMinionJoinComponent(
	ctx *pulumi.Context,
	name string,
	nodes []*RealNodeComponent,
	saltMaster *SaltMasterComponent,
	sshPrivateKey pulumi.StringOutput,
	bastionComponent *BastionComponent,
	opts ...pulumi.ResourceOption,
) (*SaltMinionJoinComponent, error) {
	component := &SaltMinionJoinComponent{}
	err := ctx.RegisterComponentResource("kubernetes-create:salt:MinionJoin", name, component, opts...)
	if err != nil {
		return nil, err
	}

	ctx.Log.Info("", nil)
	ctx.Log.Info("════════════════════════════════════════════════════════════", nil)
	ctx.Log.Info("🧂 SALT MINION CONFIGURATION (SECURE)", nil)
	ctx.Log.Info("════════════════════════════════════════════════════════════", nil)
	ctx.Log.Info(fmt.Sprintf("📋 Configuring %d nodes with cluster token...", len(nodes)), nil)
	ctx.Log.Info("", nil)

	var minionCmds []pulumi.Resource

	for i, node := range nodes {
		// Get SSH user for the node
		sshUser := getSSHUserForProvider(node.Provider)

		// Build connection args
		connArgs := remote.ConnectionArgs{
			Host:           node.PublicIP,
			User:           sshUser,
			PrivateKey:     sshPrivateKey,
			DialErrorLimit: pulumi.Int(30),
		}
		if bastionComponent != nil {
			connArgs.Proxy = &remote.ProxyConnectionArgs{
				Host:       bastionComponent.PublicIP,
				User:       getSSHUserForProvider(bastionComponent.Provider),
				PrivateKey: sshPrivateKey,
			}
		}

		// Install and configure Salt Minion with secure token
		minionCmd, err := remote.NewCommand(ctx, fmt.Sprintf("%s-minion-%d", name, i), &remote.CommandArgs{
			Connection: connArgs,
			Create: pulumi.All(node.NodeName, node.WireGuardIP, saltMaster.MasterIP, saltMaster.ClusterToken).ApplyT(func(args []interface{}) string {
				nodeName := args[0].(string)
				nodeWgIP := args[1].(string)
				masterIP := args[2].(string)
				clusterToken := args[3].(string)

				return fmt.Sprintf(`#!/bin/bash
set -e

echo "🧂 Configuring Salt Minion on %s..."
echo "🔐 Using secure cluster token for authentication..."

# Check if Salt Minion is already installed
if command -v salt-minion &> /dev/null; then
    echo "✅ Salt Minion already installed"
else
    echo "📦 Installing Salt Minion..."

    # Install Salt Minion using bootstrap script
    curl -o /tmp/bootstrap-salt.sh -L https://github.com/saltstack/salt-bootstrap/releases/latest/download/bootstrap-salt.sh
    chmod +x /tmp/bootstrap-salt.sh
    sudo sh /tmp/bootstrap-salt.sh stable

    echo "✅ Salt Minion installed"
fi

# Configure Salt Minion with SECURE token
echo "⚙️  Configuring Salt Minion with cluster token..."
sudo mkdir -p /etc/salt/minion.d
sudo mkdir -p /etc/salt/grains.d

# Set minion ID
echo "%s" | sudo tee /etc/salt/minion_id > /dev/null

# Configure master connection with WireGuard IP binding
cat <<MASTERCONF | sudo tee /etc/salt/minion.d/master.conf
# Salt Master configuration
master: %s

# Bind to WireGuard IP for secure VPN communication
interface: %s

# Minion identification
id: %s

# SECURITY: Verify master fingerprint on first connection
# (uncomment after first successful connection)
# master_finger: '<master_fingerprint>'
MASTERCONF

# Configure grains INCLUDING the secure cluster token
# This token MUST match what the master expects for autosign
cat <<GRAINS | sudo tee /etc/salt/grains
# Node classification grains
roles:
  - kubernetes
cluster: sloth-kubernetes
node_type: %s
wireguard_ip: %s

# SECURITY: Cluster authentication token
# This must match the master's autosign_grains/cluster_token
cluster_token: %s
GRAINS

# Create grains.d for additional dynamic grains
cat <<DYNAMICGRAINS | sudo tee /etc/salt/grains.d/dynamic.conf
# Dynamic grains that may change
hostname: %s
DYNAMICGRAINS

# Delete any existing keys to ensure fresh authentication
echo "🔑 Clearing old keys for fresh secure handshake..."
sudo rm -f /etc/salt/pki/minion/minion_master.pub 2>/dev/null || true

# Enable and restart Salt Minion
echo "🚀 Starting Salt Minion..."
sudo systemctl enable salt-minion
sudo systemctl restart salt-minion

# Wait for minion to authenticate
echo "⏳ Waiting for Salt Minion to authenticate with master..."
sleep 8

# Verify minion is running
if sudo systemctl is-active --quiet salt-minion; then
    echo "✅ Salt Minion is running"
    echo "   Master: %s"
    echo "   Minion ID: %s"
    echo "   Auth: Cluster token"

    # Check if key was accepted
    if [ -f /etc/salt/pki/minion/minion_master.pub ]; then
        echo "✅ Key successfully authenticated by master!"
    else
        echo "⏳ Key pending master verification..."
    fi
else
    echo "❌ Salt Minion failed to start"
    sudo journalctl -u salt-minion -n 20 --no-pager
    exit 1
fi
`, nodeName, nodeName, masterIP, nodeWgIP, nodeName, getNodeType(nodeName), nodeWgIP, clusterToken, nodeName, masterIP, nodeName)
			}).(pulumi.StringOutput),
		}, pulumi.Parent(component), pulumi.Timeouts(&pulumi.CustomTimeouts{
			Create: "5m",
		}))
		if err != nil {
			ctx.Log.Warn(fmt.Sprintf("Failed to configure Salt Minion on node %d: %v", i, err), nil)
			continue
		}
		minionCmds = append(minionCmds, minionCmd)
	}

	// Set component outputs
	component.JoinedMinions = pulumi.Int(len(minionCmds)).ToIntOutput()
	component.Status = pulumi.Sprintf("Configured %d Salt Minions (secure token auth)", len(minionCmds))

	ctx.Log.Info(fmt.Sprintf("✅ Configured %d Salt Minions with secure cluster token", len(minionCmds)), nil)

	if err := ctx.RegisterResourceOutputs(component, pulumi.Map{
		"joinedMinions": component.JoinedMinions,
		"status":        component.Status,
	}); err != nil {
		return nil, err
	}

	return component, nil
}

// Helper function to determine node type from name
func getNodeType(nodeName string) string {
	if len(nodeName) >= 6 && nodeName[:6] == "master" {
		return "master"
	}
	if len(nodeName) >= 6 && nodeName[:6] == "worker" {
		return "worker"
	}
	return "node"
}
