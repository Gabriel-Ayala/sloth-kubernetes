# Stack Management

sloth-kubernetes uses Pulumi stacks to manage infrastructure state. Each cluster deployment creates a stack that tracks all resources.

## Stack-First Architecture

**Important:** All CLI operations require a valid encrypted stack. You must create a stack before any other operation:

```bash
# Create stack with passphrase encryption
echo "your-secure-passphrase" | sloth-kubernetes stacks create my-cluster --password-stdin

# Or with AWS KMS encryption (recommended for production)
sloth-kubernetes stacks create my-cluster --kms-key alias/sloth-secrets
```

**Why encryption is mandatory:**

- Stacks store sensitive data (kubeconfig, SSH keys, cloud credentials)
- Encryption protects your cluster configuration at rest
- Supports both passphrase (AES-256-GCM) and AWS KMS encryption
- The stack acts as a secure database for all cluster operations

Without a stack, commands will fail with a helpful message:

```
Error: No stack found.

Create an encrypted stack first:
  sloth-kubernetes stacks create <name> --password-stdin

Or with AWS KMS:
  sloth-kubernetes stacks create <name> --kms-key <arn-or-alias>
```

---

## What is a Stack?

A stack represents a single deployment of your cluster configuration. It contains:

- All provisioned resources (VMs, VPCs, firewalls, etc.)
- Resource metadata and dependencies
- Output values (IPs, kubeconfig, etc.)
- State history
- Encrypted secrets and credentials

## Stack Commands

### Create Stack

Create a new encrypted stack (required before any operation):

```bash
# Passphrase encryption (development)
echo "your-passphrase" | sloth-kubernetes stacks create my-cluster --password-stdin

# AWS KMS encryption (production)
sloth-kubernetes stacks create my-cluster --kms-key alias/sloth-secrets
sloth-kubernetes stacks create my-cluster --kms-key arn:aws:kms:us-east-1:123456789:key/key-id
```

**Output:**
```
Creating encrypted stack: my-cluster
Encryption: Passphrase (AES-256-GCM)

✓ Stack created successfully

Next steps:
  sloth-kubernetes deploy my-cluster --config cluster.lisp
```

### List Stacks

View all stacks in your backend:

```bash
sloth-kubernetes stacks list
```

**Output:**
```
NAME                LAST UPDATE       RESOURCE COUNT
my-cluster          2 hours ago       47 resources
staging-cluster     1 day ago         23 resources
dev-cluster         3 days ago        15 resources
```

### Stack Info

View detailed information about a stack:

```bash
sloth-kubernetes stacks info my-cluster
```

**Output:**
```
Stack: my-cluster
Backend: s3://my-bucket?region=us-east-1
Last Update: 2025-01-02 10:30:00
Status: succeeded

Resources: 47
  - digitalocean:Vpc: 1
  - digitalocean:Droplet: 5
  - digitalocean:Firewall: 1
  - digitalocean:DnsRecord: 10
  ...
```

### Stack Outputs

View output values from a stack (IPs, kubeconfig, etc.):

```bash
sloth-kubernetes stacks output my-cluster
```

**Output:**
```
master_ips:
  - 203.0.113.10
  - 203.0.113.11
  - 203.0.113.12

worker_ips:
  - 203.0.113.20
  - 203.0.113.21

vpn_server_ip: 203.0.113.5
kubeconfig: <base64-encoded>
```

### Cancel (Unlock) Stack

If a deployment was interrupted, the stack may remain locked. Unlock it:

```bash
sloth-kubernetes stacks cancel my-cluster
```

**When to use:**
- Deployment was interrupted (Ctrl+C, network issue)
- Stack shows "locked by another process"
- You need to force unlock for maintenance

### Delete Stack

Remove a stack and its state (does not destroy resources):

```bash
sloth-kubernetes stacks delete my-cluster
```

**Warning:** This removes the state only. Resources in the cloud remain. Use `sloth-kubernetes destroy` to remove both resources and state.

---

## Stack Naming

Stacks are automatically named based on your cluster configuration:

```lisp
(cluster
  (metadata
    (name "my-cluster")          ; Stack name
    (environment "production"))) ; Optional prefix
```

The stack name is derived from `metadata.name`.

---

## Backend and Stacks

Stacks are stored in your configured backend:

### Local Backend

```bash
# Stacks stored in ~/.pulumi/stacks/
sloth-kubernetes stacks list
```

### S3 Backend

```bash
export PULUMI_BACKEND_URL="s3://my-bucket?region=us-east-1"
sloth-kubernetes stacks list
```

---

## Common Scenarios

### View Resources in a Stack

```bash
sloth-kubernetes stacks info my-cluster
```

### Get Master Node IPs

```bash
sloth-kubernetes stacks output my-cluster | grep master_ips
```

### Unlock After Failed Deployment

```bash
# If deployment was interrupted
sloth-kubernetes stacks cancel my-cluster

# Then retry deployment
sloth-kubernetes deploy --config cluster.lisp
```

### Check Stack Status

```bash
sloth-kubernetes stacks info my-cluster | grep Status
```

---

## Troubleshooting

### Stack is Locked

```
error: the stack is currently locked by another process
```

**Solution:**
```bash
sloth-kubernetes stacks cancel my-cluster
```

### Stack Not Found

```
error: stack 'my-cluster' not found
```

**Possible causes:**
1. Wrong backend configured (check `PULUMI_BACKEND_URL`)
2. Stack was never created
3. Stack was deleted

**Solution:**
```bash
# Check which backend you're using
echo $PULUMI_BACKEND_URL

# List available stacks
sloth-kubernetes stacks list
```

### Resources Exist But No Stack

If resources exist in the cloud but no stack is found:

```bash
# Import existing resources into a new stack
sloth-kubernetes refresh --config cluster.lisp
```

---

## Next Steps

- [Backend Configuration](../configuration/backend.md) - Configure S3/local backends
- [CLI Reference](cli-reference.md) - All available commands
- [Salt Commands](salt.md) - Node management
