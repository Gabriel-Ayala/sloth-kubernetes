# Installation

## Requirements

- Go 1.23+ (for building from source)
- SSH key pair
- Cloud provider account(s)

## Build from Source

```bash
# Clone repository
git clone https://github.com/chalkan3/sloth-kubernetes.git
cd sloth-kubernetes

# Build
go build -o sloth-kubernetes .

# Verify
./sloth-kubernetes version
```

## Download Binary

```bash
# Linux (amd64)
curl -sSL https://github.com/chalkan3/sloth-kubernetes/releases/latest/download/sloth-kubernetes-linux-amd64 -o sloth-kubernetes

# Linux (arm64)
curl -sSL https://github.com/chalkan3/sloth-kubernetes/releases/latest/download/sloth-kubernetes-linux-arm64 -o sloth-kubernetes

# macOS (Intel)
curl -sSL https://github.com/chalkan3/sloth-kubernetes/releases/latest/download/sloth-kubernetes-darwin-amd64 -o sloth-kubernetes

# macOS (Apple Silicon)
curl -sSL https://github.com/chalkan3/sloth-kubernetes/releases/latest/download/sloth-kubernetes-darwin-arm64 -o sloth-kubernetes

# Make executable
chmod +x sloth-kubernetes
sudo mv sloth-kubernetes /usr/local/bin/
```

## Cloud Provider Setup

### DigitalOcean

1. Create API token at [DigitalOcean API](https://cloud.digitalocean.com/account/api/tokens)
2. Set environment variable:

```bash
export DIGITALOCEAN_TOKEN="your-token"
```

### Linode

1. Create API token at [Linode API Tokens](https://cloud.linode.com/profile/tokens)
2. Set environment variable:

```bash
export LINODE_TOKEN="your-token"
```

### AWS

1. Create IAM user with EC2, VPC, and S3 permissions
2. Set environment variables:

```bash
export AWS_ACCESS_KEY_ID="your-key"
export AWS_SECRET_ACCESS_KEY="your-secret"
export AWS_REGION="us-east-1"

# For temporary credentials (STS)
export AWS_SESSION_TOKEN="your-token"
```

### Azure

1. Create service principal:

```bash
az ad sp create-for-rbac --name sloth-kubernetes --role Contributor
```

2. Set environment variables:

```bash
export ARM_CLIENT_ID="app-id"
export ARM_CLIENT_SECRET="password"
export ARM_TENANT_ID="tenant-id"
export ARM_SUBSCRIPTION_ID="subscription-id"
```

## Backend Configuration

### Local Backend (Default)

No configuration needed. State stored in `~/.pulumi`.

### S3 Backend (Recommended for Production)

```bash
export PULUMI_BACKEND_URL="s3://my-bucket?region=us-east-1"
export PULUMI_CONFIG_PASSPHRASE="your-passphrase"
```

### Persistent Config

Save to `~/.sloth-kubernetes/config`:

```bash
mkdir -p ~/.sloth-kubernetes
cat > ~/.sloth-kubernetes/config << 'EOF'
PULUMI_BACKEND_URL=s3://my-bucket?region=us-east-1
PULUMI_CONFIG_PASSPHRASE=my-passphrase
EOF
```

> **Note:** Environment variables take precedence over config file.

## Verify Installation

```bash
# Check version
sloth-kubernetes version

# Check help
sloth-kubernetes --help

# Test configuration
sloth-kubernetes validate --config cluster.lisp
```

## Next Steps

- [Quick Start](quickstart.md) - Deploy your first cluster
- [Configuration](../configuration/lisp-format.md) - Learn the config format
