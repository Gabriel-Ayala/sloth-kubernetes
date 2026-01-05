---
title: Health Monitoring
description: Comprehensive cluster health checks and monitoring
sidebar_position: 11
---

# Health Monitoring

sloth-kubernetes provides comprehensive health monitoring to ensure your Kubernetes cluster is operating correctly.

## Overview

The health system provides:
- **Comprehensive checks**: Node health, pods, DNS, certificates, etcd, API server
- **Multiple modes**: Full report, summary, specific checks
- **CI/CD integration**: Exit codes for automation
- **Remediation guidance**: Suggestions to fix issues
- **Flexible access**: Via SSH or kubeconfig

## Commands

All health commands require the **stack name** as the first argument. The kubeconfig is automatically retrieved from the Pulumi stack.

### health

Run comprehensive health checks on the cluster.

```bash
# Check cluster health
sloth-kubernetes health my-cluster

# Run only specific checks
sloth-kubernetes health my-cluster --checks nodes,pods,dns

# Verbose output with all details
sloth-kubernetes health my-cluster --verbose

# Compact output (only show issues)
sloth-kubernetes health my-cluster --compact
```

**Flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--verbose`, `-v` | Show verbose output with all details | `false` |
| `--compact` | Show compact output (only issues) | `false` |
| `--checks` | Specific checks to run | All |

**Available checks:**
- `nodes` - Node health and readiness
- `pods` - System pods status (kube-system)
- `dns` - CoreDNS availability
- `certs` - Certificate expiration
- `etcd` - Etcd cluster health
- `api` - API server responsiveness
- `storage` - Persistent volume claims status
- `network` - CNI/networking status
- `memory` - Memory pressure on nodes
- `disk` - Disk pressure on nodes

**Example output:**

```
═══════════════════════════════════════════════════════════════
                   CLUSTER HEALTH CHECK
═══════════════════════════════════════════════════════════════

Running health checks...

CHECK              STATUS    MESSAGE                           DURATION
-----              ------    -------                           --------
Node Health        [OK]      All 4 nodes are Ready             1.2s
System Pods        [OK]      All 15 system pods running        0.8s
CoreDNS            [OK]      DNS resolution working            0.5s
Certificates       [WARN]    2 certs expiring in 30 days       0.3s
Etcd Cluster       [OK]      Etcd healthy (3 members)          0.6s
API Server         [OK]      Response time: 45ms               0.1s
Storage (PVCs)     [OK]      All 8 PVCs bound                  0.4s
Networking         [OK]      CNI operational                   0.7s
Memory Pressure    [OK]      No nodes under memory pressure    0.2s
Disk Pressure      [OK]      No nodes under disk pressure      0.2s

═══════════════════════════════════════════════════════════════
                        SUMMARY
═══════════════════════════════════════════════════════════════

Overall Status: HEALTHY (with warnings)

  [OK] 9 checks healthy
  [WARN] 1 check with warnings
  [FAIL] 0 critical issues

Recommendations:
  - Renew certificates before expiration
```

### health summary

Show a quick one-line health summary.

```bash
sloth-kubernetes health summary my-cluster
```

**Example output:**

```
[OK] production: HEALTHY (9 healthy, 1 warning, 0 critical)
```

### health nodes

Check only node health.

```bash
sloth-kubernetes health nodes my-cluster
sloth-kubernetes health nodes my-cluster --verbose
```

**Example output:**

```
═══════════════════════════════════════════════════════════════
                   NODE HEALTH CHECK
═══════════════════════════════════════════════════════════════

[OK] Node Health
   Status:  healthy
   Message: All 4 nodes are Ready
   Duration: 1.2s
   Details:
     - master-1: Ready (v1.28.2)
     - master-2: Ready (v1.28.2)
     - worker-1: Ready (v1.28.2)
     - worker-2: Ready (v1.28.2)
```

### health pods

Check only system pods health.

```bash
sloth-kubernetes health pods my-cluster
sloth-kubernetes health pods my-cluster --verbose
```

**Example output:**

```
═══════════════════════════════════════════════════════════════
                 SYSTEM PODS HEALTH CHECK
═══════════════════════════════════════════════════════════════

[OK] System Pods
   Status:  healthy
   Message: All 15 system pods running
   Duration: 0.8s
   Details:
     - kube-system/coredns-5d78c9869d-abc12: Running
     - kube-system/coredns-5d78c9869d-def34: Running
     - kube-system/etcd-master-1: Running
     - kube-system/kube-apiserver-master-1: Running
     - kube-system/kube-controller-manager-master-1: Running
     - kube-system/kube-scheduler-master-1: Running
     ...
```

### health certs

Check certificate expiration.

```bash
sloth-kubernetes health certs my-cluster
```

**Example output:**

```
═══════════════════════════════════════════════════════════════
                CERTIFICATE HEALTH CHECK
═══════════════════════════════════════════════════════════════

[WARN] Certificates
   Status:  warning
   Message: 2 certificates expiring within 30 days
   Duration: 0.3s

   Remediation: Renew certificates using 'kubeadm certs renew all' or
                wait for automatic RKE2 renewal
```

---

## Health Check Details

### Node Health

Checks if all nodes are in `Ready` status.

**Healthy indicators:**
- All nodes report `Ready` condition
- No nodes have `NotReady` or `Unknown` status
- kubelet is responding on all nodes

**Remediation for failures:**
```bash
# Check node status
sloth-kubernetes kubectl my-cluster get nodes

# Describe problematic node
sloth-kubernetes kubectl my-cluster describe node <node-name>

# Check kubelet logs
sloth-kubernetes nodes ssh <node-name>
journalctl -u kubelet -f
```

### System Pods

Verifies all pods in `kube-system` namespace are running.

**Healthy indicators:**
- All pods in `Running` or `Completed` state
- No pods in `CrashLoopBackOff`, `Error`, or `Pending`
- Ready containers match total containers

**Remediation for failures:**
```bash
# Check pod status
sloth-kubernetes kubectl my-cluster get pods -n kube-system

# Check events for failing pod
sloth-kubernetes kubectl my-cluster describe pod <pod-name> -n kube-system

# View pod logs
sloth-kubernetes kubectl my-cluster logs <pod-name> -n kube-system
```

### CoreDNS

Tests DNS resolution within the cluster.

**Healthy indicators:**
- CoreDNS pods are running
- Internal DNS queries resolve correctly
- `kubernetes.default.svc.cluster.local` resolves

**Remediation for failures:**
```bash
# Check CoreDNS pods
sloth-kubernetes kubectl my-cluster get pods -n kube-system -l k8s-app=kube-dns

# View CoreDNS logs
sloth-kubernetes kubectl my-cluster logs -n kube-system -l k8s-app=kube-dns

# Test DNS manually
sloth-kubernetes kubectl my-cluster run dns-test --rm -it --image=busybox -- nslookup kubernetes
```

### Certificates

Checks certificate expiration dates.

**Warning thresholds:**
- Warning: Certificates expiring within 30 days
- Critical: Certificates expiring within 7 days

**Remediation:**
```bash
# For RKE2 clusters (automatic renewal)
# RKE2 auto-renews certificates on restart

# Manual renewal
sloth-kubernetes nodes ssh master-1
sudo systemctl restart rke2-server

# Check certificate dates
openssl x509 -in /var/lib/rancher/rke2/server/tls/client-admin.crt -noout -dates
```

### Etcd Cluster

Verifies etcd cluster health and membership.

**Healthy indicators:**
- All etcd members are healthy
- Quorum is maintained
- No leader election issues

**Remediation for failures:**
```bash
# Check etcd health
sloth-kubernetes nodes ssh master-1
sudo etcdctl --cacert=/var/lib/rancher/rke2/server/tls/etcd/server-ca.crt \
  --cert=/var/lib/rancher/rke2/server/tls/etcd/server-client.crt \
  --key=/var/lib/rancher/rke2/server/tls/etcd/server-client.key \
  endpoint health

# Check etcd member list
sudo etcdctl member list
```

### API Server

Tests API server responsiveness.

**Healthy indicators:**
- API server responds within acceptable latency
- Typically < 200ms is good
- No connection errors

**Remediation for failures:**
```bash
# Check API server pods
sloth-kubernetes kubectl my-cluster get pods -n kube-system -l component=kube-apiserver

# Test API directly
sloth-kubernetes kubectl my-cluster cluster-info

# Check API server logs
sloth-kubernetes nodes ssh master-1
journalctl -u rke2-server | grep apiserver
```

### Storage (PVCs)

Checks PersistentVolumeClaim status.

**Healthy indicators:**
- All PVCs in `Bound` state
- No `Pending` or `Lost` PVCs

**Remediation for failures:**
```bash
# Check PVC status
sloth-kubernetes kubectl my-cluster get pvc --all-namespaces

# Describe pending PVC
sloth-kubernetes kubectl my-cluster describe pvc <pvc-name> -n <namespace>

# Check storage provisioner logs
sloth-kubernetes kubectl my-cluster logs -n <storage-namespace> <provisioner-pod>
```

### Memory/Disk Pressure

Checks for resource pressure conditions on nodes.

**Healthy indicators:**
- No nodes reporting `MemoryPressure`
- No nodes reporting `DiskPressure`

**Remediation for failures:**
```bash
# Check node conditions
sloth-kubernetes kubectl my-cluster describe node <node-name> | grep -A5 Conditions

# Check resource usage
sloth-kubernetes kubectl my-cluster top nodes
sloth-kubernetes kubectl my-cluster top pods --all-namespaces

# Free up disk space
sloth-kubernetes nodes ssh <node-name>
sudo crictl rmi --prune  # Remove unused images
```

---

## CI/CD Integration

The health command returns appropriate exit codes for automation:

| Exit Code | Meaning |
|-----------|---------|
| 0 | Healthy or warnings only |
| 1 | Critical issues found |

**Example CI script:**

```bash
#!/bin/bash
# health-check.sh
STACK_NAME="production"

# Run health check
if ! sloth-kubernetes health $STACK_NAME --compact; then
  echo "Cluster health check failed!"

  # Get detailed report for debugging
  sloth-kubernetes health $STACK_NAME --verbose

  exit 1
fi

echo "Cluster is healthy, proceeding with deployment..."
```

**GitHub Actions example:**

```yaml
- name: Check Cluster Health
  run: |
    sloth-kubernetes health production --checks nodes,pods,dns

- name: Deploy Application
  if: success()
  run: |
    sloth-kubernetes kubectl production apply -f manifests/
```

---

## Examples

### Quick Health Check

```bash
# One-line summary
sloth-kubernetes health summary my-cluster
```

### Pre-Deployment Validation

```bash
# Check critical components before deploying
sloth-kubernetes health my-cluster --checks nodes,pods,api,storage

# Exit code will be non-zero if critical issues exist
```

### Troubleshooting Workflow

```bash
# 1. Run full health check
sloth-kubernetes health my-cluster --verbose

# 2. If issues found, check specific components
sloth-kubernetes health nodes my-cluster --verbose
sloth-kubernetes health pods my-cluster --verbose

# 3. Follow remediation steps in output
```

### Monitoring Integration

```bash
# Script for monitoring systems (Prometheus, Nagios, etc.)
#!/bin/bash
STACK_NAME="production"
OUTPUT=$(sloth-kubernetes health summary $STACK_NAME 2>&1)
EXIT_CODE=$?

if [ $EXIT_CODE -eq 0 ]; then
  echo "OK - $OUTPUT"
  exit 0
else
  echo "CRITICAL - $OUTPUT"
  exit 2
fi
```
