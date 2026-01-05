# kubectl Commands

sloth-kubernetes embeds kubectl for Kubernetes operations. All standard kubectl commands work through the CLI.

## Overview

The embedded kubectl:
- Uses the cluster's kubeconfig automatically from the Pulumi stack
- No separate kubectl installation required
- Full kubectl functionality available
- Stack-aware: specify the stack name once, kubeconfig is auto-retrieved

## Basic Usage

```bash
sloth-kubernetes kubectl <stack-name> <command>
```

The stack name is the name of your Pulumi stack (e.g., `production`, `staging`, `my-cluster`).

---

## Cluster Information

### Get Nodes

```bash
sloth-kubernetes kubectl my-cluster get nodes
```

**Output:**
```
NAME        STATUS   ROLES                  AGE   VERSION
masters-1   Ready    control-plane,master   5d    v1.29.0+rke2r1
masters-2   Ready    control-plane,master   5d    v1.29.0+rke2r1
masters-3   Ready    control-plane,master   5d    v1.29.0+rke2r1
workers-1   Ready    worker                 5d    v1.29.0+rke2r1
workers-2   Ready    worker                 5d    v1.29.0+rke2r1
```

### Get Detailed Node Info

```bash
sloth-kubernetes kubectl my-cluster get nodes -o wide
```

### Describe a Node

```bash
sloth-kubernetes kubectl my-cluster describe node masters-1
```

### Get Cluster Info

```bash
sloth-kubernetes kubectl my-cluster cluster-info
```

---

## Pod Management

### List All Pods

```bash
# All namespaces
sloth-kubernetes kubectl my-cluster get pods -A

# Specific namespace
sloth-kubernetes kubectl my-cluster get pods -n kube-system
```

### Get Pod Details

```bash
sloth-kubernetes kubectl my-cluster describe pod nginx-7d9b8c6b5-x2k9m -n default
```

### View Pod Logs

```bash
# Current logs
sloth-kubernetes kubectl my-cluster logs nginx-7d9b8c6b5-x2k9m

# Follow logs
sloth-kubernetes kubectl my-cluster logs -f nginx-7d9b8c6b5-x2k9m

# Previous container logs
sloth-kubernetes kubectl my-cluster logs nginx-7d9b8c6b5-x2k9m --previous

# All containers in pod
sloth-kubernetes kubectl my-cluster logs nginx-7d9b8c6b5-x2k9m --all-containers
```

### Execute Commands in Pod

```bash
# Interactive shell
sloth-kubernetes kubectl my-cluster exec -it nginx-7d9b8c6b5-x2k9m -- /bin/bash

# Run single command
sloth-kubernetes kubectl my-cluster exec nginx-7d9b8c6b5-x2k9m -- cat /etc/nginx/nginx.conf
```

---

## Deployments

### Create Deployment

```bash
sloth-kubernetes kubectl my-cluster create deployment nginx --image=nginx
```

### Scale Deployment

```bash
sloth-kubernetes kubectl my-cluster scale deployment nginx --replicas=3
```

### Update Deployment

```bash
sloth-kubernetes kubectl my-cluster set image deployment/nginx nginx=nginx:1.25
```

### Rollout Status

```bash
sloth-kubernetes kubectl my-cluster rollout status deployment/nginx
```

### Rollback Deployment

```bash
sloth-kubernetes kubectl my-cluster rollout undo deployment/nginx
```

---

## Services

### Create Service

```bash
# Expose deployment as LoadBalancer
sloth-kubernetes kubectl my-cluster expose deployment nginx --port=80 --type=LoadBalancer

# Expose as ClusterIP
sloth-kubernetes kubectl my-cluster expose deployment nginx --port=80 --type=ClusterIP
```

### List Services

```bash
sloth-kubernetes kubectl my-cluster get services
```

### Get Service Details

```bash
sloth-kubernetes kubectl my-cluster describe service nginx
```

---

## Apply Manifests

### Apply from File

```bash
sloth-kubernetes kubectl my-cluster apply -f deployment.yaml
```

### Apply from URL

```bash
sloth-kubernetes kubectl my-cluster apply -f https://example.com/manifest.yaml
```

### Apply from Directory

```bash
sloth-kubernetes kubectl my-cluster apply -f ./manifests/
```

### Delete Resources

```bash
sloth-kubernetes kubectl my-cluster delete -f deployment.yaml
```

---

## Namespaces

### List Namespaces

```bash
sloth-kubernetes kubectl my-cluster get namespaces
```

### Create Namespace

```bash
sloth-kubernetes kubectl my-cluster create namespace production
```

### Set Default Namespace

```bash
sloth-kubernetes kubectl my-cluster config set-context --current --namespace=production
```

---

## ConfigMaps and Secrets

### Create ConfigMap

```bash
# From literal values
sloth-kubernetes kubectl my-cluster create configmap app-config \
  --from-literal=DATABASE_URL=postgres://db:5432/app

# From file
sloth-kubernetes kubectl my-cluster create configmap app-config --from-file=config.json
```

### Create Secret

```bash
# From literal values
sloth-kubernetes kubectl my-cluster create secret generic db-credentials \
  --from-literal=username=admin \
  --from-literal=password=secret123

# From file
sloth-kubernetes kubectl my-cluster create secret generic tls-cert \
  --from-file=cert.pem --from-file=key.pem
```

### View Secret (base64 decoded)

```bash
sloth-kubernetes kubectl my-cluster get secret db-credentials -o jsonpath='{.data.password}' | base64 -d
```

---

## Resource Management

### Get Resource Usage

```bash
# Node resources
sloth-kubernetes kubectl my-cluster top nodes

# Pod resources
sloth-kubernetes kubectl my-cluster top pods -A
```

### Get Events

```bash
sloth-kubernetes kubectl my-cluster get events --sort-by='.lastTimestamp'
```

### Debug Node

```bash
sloth-kubernetes kubectl my-cluster debug node/workers-1 -it --image=busybox
```

---

## Node Operations

### Cordon Node

Prevent new pods from being scheduled:

```bash
sloth-kubernetes kubectl my-cluster cordon workers-1
```

### Drain Node

Safely evict pods for maintenance:

```bash
sloth-kubernetes kubectl my-cluster drain workers-1 --ignore-daemonsets --delete-emptydir-data
```

### Uncordon Node

Allow scheduling again:

```bash
sloth-kubernetes kubectl my-cluster uncordon workers-1
```

---

## Troubleshooting

### Pod Not Starting

```bash
# Check pod status
sloth-kubernetes kubectl my-cluster describe pod <pod-name>

# Check events
sloth-kubernetes kubectl my-cluster get events --field-selector involvedObject.name=<pod-name>
```

### Service Not Reachable

```bash
# Check endpoints
sloth-kubernetes kubectl my-cluster get endpoints <service-name>

# Check service selector matches pod labels
sloth-kubernetes kubectl my-cluster get pods --show-labels
```

### Resource Limits

```bash
# Check resource requests/limits
sloth-kubernetes kubectl my-cluster describe pod <pod-name> | grep -A 5 "Limits:"

# Check node capacity
sloth-kubernetes kubectl my-cluster describe node <node-name> | grep -A 10 "Allocated resources:"
```

---

## Common Operations

### Quick Health Check

```bash
# Check nodes
sloth-kubernetes kubectl my-cluster get nodes

# Check system pods
sloth-kubernetes kubectl my-cluster get pods -n kube-system

# Check events
sloth-kubernetes kubectl my-cluster get events -A --sort-by='.lastTimestamp' | tail -20
```

### Deploy Application

```bash
# Create deployment
sloth-kubernetes kubectl my-cluster create deployment myapp --image=myapp:v1

# Expose service
sloth-kubernetes kubectl my-cluster expose deployment myapp --port=80 --type=LoadBalancer

# Check status
sloth-kubernetes kubectl my-cluster get pods,svc
```

### View Logs Across Pods

```bash
sloth-kubernetes kubectl my-cluster logs -l app=nginx --all-containers
```

---

## Kubeconfig

The kubeconfig is automatically retrieved from the specified Pulumi stack. You no longer need to manually export or manage kubeconfig files.

### Export Kubeconfig (Optional)

If you need to use external tools, you can still export the kubeconfig:

```bash
# Save to file for a specific stack
sloth-kubernetes kubeconfig my-cluster > ~/.kube/config

# Use with standard kubectl
export KUBECONFIG=~/.kube/config
kubectl get nodes
```

### Use with External kubectl

```bash
sloth-kubernetes kubeconfig my-cluster > cluster.kubeconfig
kubectl --kubeconfig=cluster.kubeconfig get nodes
```

---

## Next Steps

- [Salt Commands](salt.md) - Node management
- [Stack Management](stacks.md) - Infrastructure state
- [CLI Reference](cli-reference.md) - All available commands
