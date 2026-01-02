# Backend Configuration

sloth-kubernetes uses Pulumi for infrastructure state management. You can choose between local storage or S3-compatible backends.

## Backend Modes

### Local Backend (Default)

State is stored locally in `~/.pulumi`. Simple setup for development and single-user scenarios.

```bash
# No configuration needed - works out of the box
sloth-kubernetes deploy --config cluster.lisp
```

**Characteristics:**
- Zero configuration required
- State stored in `~/.pulumi/`
- Not suitable for team collaboration
- No remote backup

---

### S3 Backend (Recommended for Production)

State is stored in an S3-compatible bucket. Enables team collaboration and CI/CD integration.

```bash
# Set environment variables
export PULUMI_BACKEND_URL="s3://my-bucket?region=us-east-1"
export PULUMI_CONFIG_PASSPHRASE="your-secret-passphrase"

# AWS credentials for S3 access
export AWS_ACCESS_KEY_ID="your-key"
export AWS_SECRET_ACCESS_KEY="your-secret"
export AWS_SESSION_TOKEN="your-token"  # Optional, for temporary credentials

# Deploy
sloth-kubernetes deploy --config cluster.lisp
```

**Characteristics:**
- Enables team collaboration
- Suitable for CI/CD pipelines
- State is encrypted at rest
- Supports state locking
- Remote backup and recovery

---

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `PULUMI_BACKEND_URL` | For S3 | S3 bucket URL with region |
| `PULUMI_CONFIG_PASSPHRASE` | Yes | Encryption passphrase for secrets |
| `AWS_ACCESS_KEY_ID` | For S3 | AWS access key |
| `AWS_SECRET_ACCESS_KEY` | For S3 | AWS secret key |
| `AWS_SESSION_TOKEN` | No | AWS session token (temporary credentials) |

---

## Persistent Configuration

Save backend settings to `~/.sloth-kubernetes/config` to avoid setting environment variables every time:

```bash
mkdir -p ~/.sloth-kubernetes
cat > ~/.sloth-kubernetes/config << 'EOF'
PULUMI_BACKEND_URL=s3://my-bucket?region=us-east-1
PULUMI_CONFIG_PASSPHRASE=my-secret-passphrase
EOF
```

**Important:** Environment variables take precedence over the config file.

---

## S3 Backend Setup

### Create S3 Bucket

```bash
# Create bucket (if not exists)
aws s3 mb s3://my-sloth-kubernetes-state --region us-east-1

# Enable versioning (recommended)
aws s3api put-bucket-versioning \
  --bucket my-sloth-kubernetes-state \
  --versioning-configuration Status=Enabled
```

### Bucket Policy (Minimal Permissions)

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "s3:GetObject",
        "s3:PutObject",
        "s3:DeleteObject",
        "s3:ListBucket"
      ],
      "Resource": [
        "arn:aws:s3:::my-sloth-kubernetes-state",
        "arn:aws:s3:::my-sloth-kubernetes-state/*"
      ]
    }
  ]
}
```

### S3-Compatible Backends

You can use any S3-compatible storage:

**MinIO:**
```bash
export PULUMI_BACKEND_URL="s3://bucket-name?endpoint=minio.example.com:9000&disableSSL=true&s3ForcePathStyle=true"
```

**DigitalOcean Spaces:**
```bash
export PULUMI_BACKEND_URL="s3://space-name?endpoint=nyc3.digitaloceanspaces.com"
export AWS_ACCESS_KEY_ID="your-spaces-key"
export AWS_SECRET_ACCESS_KEY="your-spaces-secret"
```

**Backblaze B2:**
```bash
export PULUMI_BACKEND_URL="s3://bucket-name?endpoint=s3.us-west-000.backblazeb2.com"
```

---

## Stack Management

Stacks are stored in the backend and identified by name:

```bash
# List all stacks
sloth-kubernetes stacks list

# View stack details
sloth-kubernetes stacks info my-cluster

# View stack outputs (IPs, kubeconfig, etc.)
sloth-kubernetes stacks output my-cluster

# Unlock a stuck stack
sloth-kubernetes stacks cancel my-cluster

# Delete a stack
sloth-kubernetes stacks delete my-cluster
```

---

## Backend URL Format

```
s3://<bucket-name>?region=<region>&endpoint=<custom-endpoint>
```

**Parameters:**

| Parameter | Description |
|-----------|-------------|
| `region` | AWS region (e.g., `us-east-1`) |
| `endpoint` | Custom S3-compatible endpoint |
| `disableSSL` | Set to `true` for non-HTTPS endpoints |
| `s3ForcePathStyle` | Set to `true` for path-style URLs |

---

## Security Best Practices

1. **Use strong passphrases** for `PULUMI_CONFIG_PASSPHRASE`
2. **Enable bucket versioning** for state recovery
3. **Restrict bucket access** with IAM policies
4. **Enable server-side encryption** on the S3 bucket
5. **Use temporary credentials** (AWS STS) when possible
6. **Never commit credentials** to version control

### Enable Bucket Encryption

```bash
aws s3api put-bucket-encryption \
  --bucket my-sloth-kubernetes-state \
  --server-side-encryption-configuration '{
    "Rules": [{
      "ApplyServerSideEncryptionByDefault": {
        "SSEAlgorithm": "AES256"
      }
    }]
  }'
```

---

## CI/CD Integration

For GitHub Actions:

```yaml
name: Deploy Cluster
on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Deploy cluster
        env:
          PULUMI_BACKEND_URL: ${{ secrets.PULUMI_BACKEND_URL }}
          PULUMI_CONFIG_PASSPHRASE: ${{ secrets.PULUMI_CONFIG_PASSPHRASE }}
          AWS_ACCESS_KEY_ID: ${{ secrets.AWS_ACCESS_KEY_ID }}
          AWS_SECRET_ACCESS_KEY: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
          DIGITALOCEAN_TOKEN: ${{ secrets.DIGITALOCEAN_TOKEN }}
        run: |
          sloth-kubernetes deploy --config cluster.lisp --auto-approve
```

---

## Troubleshooting

### Stack is Locked

If a deployment was interrupted, the stack may remain locked:

```bash
sloth-kubernetes stacks cancel my-cluster
```

### Reset Backend

To force use of S3 backend (overriding config file):

```bash
export PULUMI_BACKEND_URL="s3://my-bucket?region=us-east-1"
sloth-kubernetes stacks list
```

### View Backend Location

The current backend is determined by:
1. `PULUMI_BACKEND_URL` environment variable (highest priority)
2. `~/.sloth-kubernetes/config` file
3. Local backend `~/.pulumi` (default)

---

## Next Steps

- [LISP Configuration](lisp-format.md) - Configuration syntax reference
- [Examples](examples.md) - More configuration examples
- [Stack Management](../user-guide/stacks.md) - Managing Pulumi stacks
