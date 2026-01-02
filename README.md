# Sloth Kubernetes

**Multi-Cloud Kubernetes Deployment** - Single binary, zero external dependencies.

Deploy production-ready K8s clusters across DigitalOcean, Linode, AWS, and Azure with embedded Pulumi, Salt, and kubectl.

## Installation

```bash
# Build from source
go build -o sloth-kubernetes .

# Or download binary (coming soon)
curl -fsSL https://github.com/chalkan3/sloth-kubernetes/releases/latest/download/sloth-kubernetes -o sloth-kubernetes
chmod +x sloth-kubernetes
```

## Configuration

### Backend Modes

Sloth Kubernetes uses Pulumi for infrastructure state. You can choose between:

#### 1. Local Backend (Default)

State stored locally in `~/.pulumi`. Simple for development.

```bash
# No configuration needed - works out of the box
sloth-kubernetes deploy --config cluster.lisp
```

#### 2. S3 Backend (Recommended for Production)

State stored in S3 bucket. Enables team collaboration and CI/CD.

```bash
# Set environment variables
export PULUMI_BACKEND_URL="s3://my-bucket?region=us-east-1"
export PULUMI_CONFIG_PASSPHRASE="your-secret-passphrase"
export AWS_ACCESS_KEY_ID="..."
export AWS_SECRET_ACCESS_KEY="..."
export AWS_SESSION_TOKEN="..."  # Optional, for temporary credentials

# Deploy
sloth-kubernetes deploy --config cluster.lisp
```

#### 3. Persistent Config File

Save backend settings to `~/.sloth-kubernetes/config`:

```bash
mkdir -p ~/.sloth-kubernetes
cat > ~/.sloth-kubernetes/config << 'EOF'
PULUMI_BACKEND_URL=s3://my-bucket?region=us-east-1
PULUMI_CONFIG_PASSPHRASE=my-passphrase
EOF
```

> **Note:** Environment variables take precedence over config file.

### Cloud Provider Credentials

Set credentials for your cloud providers:

```bash
# DigitalOcean
export DIGITALOCEAN_TOKEN="your-token"

# Linode
export LINODE_TOKEN="your-token"

# AWS
export AWS_ACCESS_KEY_ID="..."
export AWS_SECRET_ACCESS_KEY="..."
export AWS_REGION="us-east-1"

# Azure
export ARM_CLIENT_ID="..."
export ARM_CLIENT_SECRET="..."
export ARM_TENANT_ID="..."
export ARM_SUBSCRIPTION_ID="..."
```

## Quick Start

### 1. Create Configuration

```lisp
; cluster.lisp
(cluster
  (metadata
    (name "my-cluster")
    (environment "production"))

  (providers
    (digitalocean
      (enabled true)
      (token "${DIGITALOCEAN_TOKEN}")
      (region "nyc3")
      (vpc
        (create true)
        (cidr "10.10.0.0/16"))))

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
      (size "s-2vcpu-4gb"))
    (workers
      (name "workers")
      (provider "digitalocean")
      (count 2)
      (roles worker)
      (size "s-2vcpu-4gb")))

  (kubernetes
    (distribution "rke2")
    (version "v1.28.5+rke2r1")))
```

### 2. Deploy

```bash
sloth-kubernetes deploy --config cluster.lisp
```

### 3. Access Your Cluster

```bash
# Get kubeconfig
sloth-kubernetes kubeconfig > ~/.kube/config

# Use embedded kubectl
sloth-kubernetes kubectl get nodes
```

## Commands

### Infrastructure

| Command | Description |
|---------|-------------|
| `deploy --config <file>` | Deploy cluster |
| `destroy` | Destroy cluster |
| `refresh` | Sync state with cloud |
| `preview` | Preview changes |

### Stack Management

| Command | Description |
|---------|-------------|
| `stacks list` | List all stacks |
| `stacks info <name>` | Show stack details |
| `stacks output <name>` | Show stack outputs |
| `stacks delete <name>` | Delete a stack |
| `stacks cancel <name>` | Unlock stuck stack |

### Node Management (Salt)

| Command | Description |
|---------|-------------|
| `salt login` | Authenticate with Salt master |
| `salt ping` | Ping all nodes |
| `salt cmd "<command>"` | Run command on all nodes |
| `salt system disk` | Check disk usage |
| `salt system memory` | Check memory usage |

### Kubernetes (kubectl)

| Command | Description |
|---------|-------------|
| `kubectl get nodes` | List nodes |
| `kubectl get pods -A` | List all pods |
| `kubectl apply -f <file>` | Apply manifest |
| `kubectl logs <pod>` | View pod logs |

### Addons

| Command | Description |
|---------|-------------|
| `addons install <name>` | Install addon |
| `addons list` | List available addons |
| `addons bootstrap --repo <url>` | Bootstrap ArgoCD |

## Examples

### Multi-Cloud Cluster

```lisp
(cluster
  (metadata
    (name "multi-cloud-cluster"))

  (providers
    (digitalocean
      (enabled true)
      (region "nyc3"))
    (linode
      (enabled true)
      (region "us-east"))
    (aws
      (enabled true)
      (region "us-east-1")))

  (network
    (mode "wireguard")
    (wireguard
      (enabled true)
      (create true)
      (mesh-networking true)))

  (node-pools
    (do-masters
      (name "do-masters")
      (provider "digitalocean")
      (count 1)
      (roles master etcd)
      (size "s-2vcpu-4gb"))
    (linode-workers
      (name "linode-workers")
      (provider "linode")
      (count 2)
      (roles worker)
      (size "g6-standard-2"))
    (aws-workers
      (name "aws-workers")
      (provider "aws")
      (count 2)
      (roles worker)
      (size "t3.medium"))))
```

### HA Cluster (3 Masters)

```lisp
(cluster
  (metadata
    (name "ha-cluster")
    (environment "production"))

  (providers
    (digitalocean
      (enabled true)
      (region "nyc3")))

  (node-pools
    (masters
      (name "masters")
      (provider "digitalocean")
      (count 3)
      (roles master etcd)
      (size "s-4vcpu-8gb"))
    (workers
      (name "workers")
      (provider "digitalocean")
      (count 5)
      (roles worker)
      (size "s-2vcpu-4gb")))

  (kubernetes
    (distribution "rke2")
    (version "v1.29.0+rke2r1")
    (high-availability true)))
```

### AWS with Spot Instances

```lisp
(cluster
  (metadata
    (name "aws-spot-cluster"))

  (providers
    (aws
      (enabled true)
      (region "us-east-1")))

  (node-pools
    (masters
      (name "masters")
      (provider "aws")
      (count 3)
      (roles master etcd)
      (size "t3.large"))
    (workers
      (name "workers")
      (provider "aws")
      (count 10)
      (roles worker)
      (size "t3.large")
      (spot-instance true)
      (spot-max-price "0.05"))))
```

## Troubleshooting

### Stack is Locked

```bash
# Unlock a stuck stack
sloth-kubernetes stacks cancel <stack-name>
```

### View Deployment Logs

```bash
# Verbose output
sloth-kubernetes deploy --config cluster.lisp --verbose
```

### Check Node Status

```bash
# Using embedded Salt
sloth-kubernetes salt ping
sloth-kubernetes salt system status

# Using embedded kubectl
sloth-kubernetes kubectl get nodes -o wide
```

### Reset Backend

```bash
# Force use of S3 backend (override config file)
export PULUMI_BACKEND_URL="s3://my-bucket?region=us-east-1"
sloth-kubernetes stacks list
```

## Architecture

```
sloth-kubernetes (single binary)
├── Pulumi Automation API  → Infrastructure as Code
├── Salt API Client        → Node configuration & management
└── kubectl Client         → Kubernetes operations

Cloud Providers:
├── DigitalOcean (Droplets, VPCs, Firewalls)
├── Linode (Instances, VPCs, NodeBalancers)
├── AWS (EC2, VPCs, Security Groups, NLB)
└── Azure (VMs, VNets, NSGs)

Networking:
└── WireGuard VPN Mesh → Cross-provider connectivity
```

## Development

```bash
# Run tests
go test ./...

# Run E2E tests (requires AWS credentials)
go test -tags=e2e ./test/e2e/ -timeout 10m

# Build
go build -o sloth-kubernetes .
```

## License

MIT
