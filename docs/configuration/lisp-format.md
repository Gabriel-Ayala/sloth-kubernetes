# LISP Configuration Format

sloth-kubernetes uses a LISP-inspired configuration format for cluster definitions. This format is expressive, readable, and supports nested structures naturally.

## Basic Syntax

### Structure

```lisp
; This is a comment
(keyword
  (nested-keyword value)
  (another-keyword "string value")
  (list-values item1 item2 item3))
```

### Rules

1. **Parentheses** - Define structure boundaries
2. **Keywords** - Lowercase with hyphens (e.g., `node-pools`, `mesh-networking`)
3. **Strings** - Quoted with double quotes for values with spaces or special characters
4. **Numbers** - Written directly without quotes
5. **Booleans** - `true` or `false` (no quotes)
6. **Lists** - Space-separated values within parentheses
7. **Comments** - Start with `;` and continue to end of line

### Environment Variables

Reference environment variables with `${VAR_NAME}`:

```lisp
(providers
  (digitalocean
    (token "${DIGITALOCEAN_TOKEN}")))
```

---

## Complete Configuration Reference

### Cluster Root

```lisp
(cluster
  (metadata ...)
  (providers ...)
  (network ...)
  (node-pools ...)
  (kubernetes ...))
```

---

## Metadata Section

Define cluster identification and environment:

```lisp
(metadata
  (name "my-cluster")
  (environment "production")
  (labels
    (team "platform")
    (cost-center "engineering")))
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Cluster name (used for stack naming) |
| `environment` | string | No | Environment label (development, staging, production) |
| `labels` | nested | No | Custom key-value labels |

---

## Providers Section

Configure cloud provider credentials and defaults:

### DigitalOcean

```lisp
(providers
  (digitalocean
    (enabled true)
    (token "${DIGITALOCEAN_TOKEN}")
    (region "nyc3")
    (vpc
      (create true)
      (cidr "10.10.0.0/16"))))
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `enabled` | boolean | Yes | Enable this provider |
| `token` | string | Yes | API token (use env var) |
| `region` | string | Yes | Default region |
| `vpc.create` | boolean | No | Create new VPC |
| `vpc.cidr` | string | No | VPC CIDR block |

### Linode

```lisp
(providers
  (linode
    (enabled true)
    (token "${LINODE_TOKEN}")
    (region "us-east")
    (vpc
      (create true)
      (cidr "10.11.0.0/16"))))
```

### AWS

```lisp
(providers
  (aws
    (enabled true)
    (region "us-east-1")
    (vpc
      (create true)
      (cidr "10.0.0.0/16"))))
```

AWS credentials are read from environment variables:
- `AWS_ACCESS_KEY_ID`
- `AWS_SECRET_ACCESS_KEY`
- `AWS_SESSION_TOKEN` (optional, for temporary credentials)

### Azure

```lisp
(providers
  (azure
    (enabled true)
    (location "eastus")
    (resource-group "my-rg")
    (vnet
      (create true)
      (cidr "10.12.0.0/16"))))
```

Azure credentials are read from environment variables:
- `ARM_CLIENT_ID`
- `ARM_CLIENT_SECRET`
- `ARM_TENANT_ID`
- `ARM_SUBSCRIPTION_ID`

### Multi-Cloud Example

```lisp
(providers
  (digitalocean
    (enabled true)
    (token "${DIGITALOCEAN_TOKEN}")
    (region "nyc3")
    (vpc
      (create true)
      (cidr "10.10.0.0/16")))
  (linode
    (enabled true)
    (token "${LINODE_TOKEN}")
    (region "us-east")
    (vpc
      (create true)
      (cidr "10.11.0.0/16")))
  (aws
    (enabled true)
    (region "us-east-1")
    (vpc
      (create true)
      (cidr "10.0.0.0/16"))))
```

---

## Network Section

Configure VPN and mesh networking:

```lisp
(network
  (mode "wireguard")
  (wireguard
    (enabled true)
    (create true)
    (mesh-networking true)
    (subnet "10.8.0.0/24")
    (port 51820)))
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `mode` | string | Yes | Network mode: `wireguard`, `direct` |
| `wireguard.enabled` | boolean | Yes | Enable WireGuard VPN |
| `wireguard.create` | boolean | No | Create VPN server |
| `wireguard.mesh-networking` | boolean | No | Full mesh between all nodes |
| `wireguard.subnet` | string | No | VPN subnet (default: 10.8.0.0/24) |
| `wireguard.port` | number | No | UDP port (default: 51820) |

---

## Node Pools Section

Define groups of nodes with specific configurations:

```lisp
(node-pools
  (masters
    (name "masters")
    (provider "digitalocean")
    (count 3)
    (roles master etcd)
    (size "s-2vcpu-4gb")
    (region "nyc3"))
  (workers
    (name "workers")
    (provider "digitalocean")
    (count 5)
    (roles worker)
    (size "s-4vcpu-8gb")
    (region "nyc3")))
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Pool identifier |
| `provider` | string | Yes | Cloud provider name |
| `count` | number | Yes | Number of nodes |
| `roles` | list | Yes | Node roles: `master`, `etcd`, `worker` |
| `size` | string | Yes | Instance size/type |
| `region` | string | No | Override provider default region |
| `spot-instance` | boolean | No | Use spot/preemptible instances |
| `spot-max-price` | string | No | Maximum spot price (AWS) |
| `labels` | nested | No | Kubernetes node labels |
| `taints` | nested | No | Kubernetes node taints |

### Node Roles

- **master** - Kubernetes control plane
- **etcd** - etcd cluster member (usually combined with master)
- **worker** - Workload nodes

### Instance Sizes by Provider

**DigitalOcean:**
- `s-1vcpu-1gb`, `s-2vcpu-2gb`, `s-2vcpu-4gb`, `s-4vcpu-8gb`, `s-8vcpu-16gb`

**Linode:**
- `g6-nanode-1`, `g6-standard-1`, `g6-standard-2`, `g6-standard-4`, `g6-standard-8`

**AWS:**
- `t3.micro`, `t3.small`, `t3.medium`, `t3.large`, `m5.large`, `c5.xlarge`

**Azure:**
- `Standard_B2s`, `Standard_D2s_v3`, `Standard_D4s_v3`

### Spot Instances (AWS)

```lisp
(workers
  (name "spot-workers")
  (provider "aws")
  (count 10)
  (roles worker)
  (size "t3.large")
  (spot-instance true)
  (spot-max-price "0.05"))
```

### Node Labels and Taints

```lisp
(gpu-workers
  (name "gpu-workers")
  (provider "aws")
  (count 2)
  (roles worker)
  (size "p3.2xlarge")
  (labels
    (node-type "gpu")
    (accelerator "nvidia"))
  (taints
    (gpu
      (key "nvidia.com/gpu")
      (value "true")
      (effect "NoSchedule"))))
```

---

## Kubernetes Section

Configure the Kubernetes distribution:

```lisp
(kubernetes
  (distribution "rke2")
  (version "v1.29.0+rke2r1")
  (high-availability true))
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `distribution` | string | Yes | Distribution: `rke2`, `k3s` |
| `version` | string | Yes | Kubernetes version |
| `high-availability` | boolean | No | Enable HA mode |

### RKE2 Versions

- `v1.29.0+rke2r1` (latest stable)
- `v1.28.5+rke2r1`
- `v1.27.10+rke2r1`

### K3s Versions

- `v1.29.0+k3s1`
- `v1.28.5+k3s1`

---

## Complete Examples

### Minimal Development Cluster

```lisp
; cluster.lisp - Development cluster
(cluster
  (metadata
    (name "dev-cluster")
    (environment "development"))

  (providers
    (digitalocean
      (enabled true)
      (token "${DIGITALOCEAN_TOKEN}")
      (region "nyc3")))

  (node-pools
    (all-in-one
      (name "all-in-one")
      (provider "digitalocean")
      (count 1)
      (roles master worker)
      (size "s-2vcpu-4gb")))

  (kubernetes
    (distribution "rke2")
    (version "v1.29.0+rke2r1")))
```

### Production HA Cluster

```lisp
; cluster.lisp - Production HA cluster
(cluster
  (metadata
    (name "production")
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
      (count 3)
      (roles master etcd)
      (size "s-4vcpu-8gb")
      (region "nyc3"))
    (workers
      (name "workers")
      (provider "digitalocean")
      (count 5)
      (roles worker)
      (size "s-4vcpu-8gb")
      (region "nyc3")))

  (kubernetes
    (distribution "rke2")
    (version "v1.29.0+rke2r1")
    (high-availability true)))
```

### Multi-Cloud HA Cluster

```lisp
; cluster.lisp - Multi-cloud HA cluster
(cluster
  (metadata
    (name "multi-cloud")
    (environment "production"))

  (providers
    (digitalocean
      (enabled true)
      (token "${DIGITALOCEAN_TOKEN}")
      (region "nyc3")
      (vpc
        (create true)
        (cidr "10.10.0.0/16")))
    (linode
      (enabled true)
      (token "${LINODE_TOKEN}")
      (region "us-east")
      (vpc
        (create true)
        (cidr "10.11.0.0/16")))
    (aws
      (enabled true)
      (region "us-east-1")
      (vpc
        (create true)
        (cidr "10.0.0.0/16"))))

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
    (linode-masters
      (name "linode-masters")
      (provider "linode")
      (count 1)
      (roles master etcd)
      (size "g6-standard-2"))
    (aws-masters
      (name "aws-masters")
      (provider "aws")
      (count 1)
      (roles master etcd)
      (size "t3.medium"))
    (do-workers
      (name "do-workers")
      (provider "digitalocean")
      (count 2)
      (roles worker)
      (size "s-4vcpu-8gb"))
    (aws-spot-workers
      (name "aws-spot-workers")
      (provider "aws")
      (count 5)
      (roles worker)
      (size "t3.large")
      (spot-instance true)
      (spot-max-price "0.05")))

  (kubernetes
    (distribution "rke2")
    (version "v1.29.0+rke2r1")
    (high-availability true)))
```

---

## Validation

Validate your configuration before deploying:

```bash
sloth-kubernetes validate --config cluster.lisp
```

---

## Next Steps

- [Backend Configuration](backend.md) - S3 and local state storage
- [Examples](examples.md) - More configuration examples
- [CLI Reference](../user-guide/cli-reference.md) - All available commands
