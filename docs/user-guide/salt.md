# Salt Commands

sloth-kubernetes embeds a Salt API client for node management. Execute commands on all cluster nodes without SSH access.

## Overview

Salt (SaltStack) is used for:
- Running commands across all nodes
- System health monitoring
- Package management
- Service control
- Configuration management

## Authentication

Before using Salt commands, authenticate with the Salt master:

```bash
sloth-kubernetes salt login
```

This retrieves credentials from the cluster and stores them locally.

---

## Basic Commands

### Ping All Nodes

Check connectivity to all nodes:

```bash
sloth-kubernetes salt ping
```

**Output:**
```
masters-1: True
workers-1: True
workers-2: True
```

### Run Command on All Nodes

Execute any shell command:

```bash
sloth-kubernetes salt cmd "uptime"
```

**Output:**
```
masters-1:
   10:30:45 up 5 days, 3:21, 0 users, load average: 0.12, 0.15, 0.10
workers-1:
   10:30:45 up 5 days, 3:20, 0 users, load average: 0.08, 0.10, 0.09
workers-2:
   10:30:45 up 5 days, 3:19, 0 users, load average: 0.05, 0.08, 0.07
```

---

## System Monitoring

### Check Disk Usage

```bash
sloth-kubernetes salt system disk
```

**Output:**
```
masters-1:
  /: 45% used (18G/40G)
  /var: 30% used (12G/40G)
workers-1:
  /: 38% used (15G/40G)
  /var: 25% used (10G/40G)
```

### Check Memory Usage

```bash
sloth-kubernetes salt system memory
```

**Output:**
```
masters-1:
  Total: 8192 MB
  Used: 4096 MB (50%)
  Free: 4096 MB
workers-1:
  Total: 8192 MB
  Used: 3072 MB (37%)
  Free: 5120 MB
```

### Check System Status

```bash
sloth-kubernetes salt system status
```

**Output:**
```
masters-1:
  Uptime: 5 days, 3:21
  Load: 0.12, 0.15, 0.10
  Memory: 50% used
  Disk: 45% used
workers-1:
  Uptime: 5 days, 3:20
  Load: 0.08, 0.10, 0.09
  Memory: 37% used
  Disk: 38% used
```

---

## Service Management

### Check Service Status

```bash
sloth-kubernetes salt cmd "systemctl status kubelet"
```

### Restart a Service

```bash
sloth-kubernetes salt cmd "systemctl restart kubelet"
```

### View Service Logs

```bash
sloth-kubernetes salt cmd "journalctl -u kubelet -n 50"
```

---

## Package Management

### Update Package Lists

```bash
sloth-kubernetes salt cmd "apt update"
```

### Install a Package

```bash
sloth-kubernetes salt cmd "apt install -y htop"
```

### Check Installed Package Version

```bash
sloth-kubernetes salt cmd "dpkg -l | grep docker"
```

---

## Targeting Specific Nodes

### Target by Name Pattern

```bash
# Only master nodes
sloth-kubernetes salt cmd "uptime" --target "masters-*"

# Only worker nodes
sloth-kubernetes salt cmd "uptime" --target "workers-*"

# Specific node
sloth-kubernetes salt cmd "uptime" --target "masters-1"
```

### Target by Role

```bash
# All masters
sloth-kubernetes salt cmd "uptime" --role master

# All workers
sloth-kubernetes salt cmd "uptime" --role worker
```

---

## Common Operations

### Check Kubernetes Components

```bash
# Kubelet status
sloth-kubernetes salt cmd "systemctl status kubelet"

# Container runtime status
sloth-kubernetes salt cmd "systemctl status containerd"

# Check running containers
sloth-kubernetes salt cmd "crictl ps"
```

### Check WireGuard VPN

```bash
# WireGuard interface status
sloth-kubernetes salt cmd "wg show"

# Check WireGuard connectivity
sloth-kubernetes salt cmd "ping -c 3 10.8.0.1"
```

### View System Logs

```bash
# System messages
sloth-kubernetes salt cmd "dmesg | tail -20"

# Kernel messages
sloth-kubernetes salt cmd "journalctl -k -n 20"
```

### Network Troubleshooting

```bash
# Check network interfaces
sloth-kubernetes salt cmd "ip addr"

# Check routing table
sloth-kubernetes salt cmd "ip route"

# Check DNS resolution
sloth-kubernetes salt cmd "nslookup kubernetes.default.svc.cluster.local"
```

---

## Maintenance Tasks

### Drain Node for Maintenance

```bash
# Drain from Kubernetes
sloth-kubernetes kubectl drain workers-1 --ignore-daemonsets

# Perform maintenance
sloth-kubernetes salt cmd "apt update && apt upgrade -y" --target "workers-1"

# Reboot if needed
sloth-kubernetes salt cmd "reboot" --target "workers-1"

# Uncordon after maintenance
sloth-kubernetes kubectl uncordon workers-1
```

### Rotate Logs

```bash
sloth-kubernetes salt cmd "journalctl --vacuum-time=7d"
```

### Clean Docker/Containerd

```bash
sloth-kubernetes salt cmd "crictl rmi --prune"
```

---

## Troubleshooting

### Node Not Responding

If a node doesn't respond to ping:

1. Check if node is running in cloud console
2. Check WireGuard connectivity
3. Check Salt minion status

```bash
# Check minion status on the node (if accessible via other means)
systemctl status salt-minion
```

### Authentication Failed

```bash
# Re-authenticate
sloth-kubernetes salt login

# Verify credentials
sloth-kubernetes salt ping
```

### Command Timeout

For long-running commands, increase timeout:

```bash
sloth-kubernetes salt cmd "apt upgrade -y" --timeout 600
```

---

## Next Steps

- [kubectl Commands](kubectl.md) - Kubernetes operations
- [Stack Management](stacks.md) - Infrastructure state
- [CLI Reference](cli-reference.md) - All available commands
