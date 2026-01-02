# kubectl Commands

sloth-kubernetes embeds kubectl for Kubernetes operations. All standard kubectl commands work through the CLI.

## Overview

The embedded kubectl:
- Uses the cluster's kubeconfig automatically
- No separate kubectl installation required
- Full kubectl functionality available

## Basic Usage

```bash
sloth-kubernetes kubectl <command>
```

---

## Cluster Information

### Get Nodes

```bash
sloth-kubernetes kubectl get nodes
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
sloth-kubernetes kubectl get nodes -o wide
```

### Describe a Node

```bash
sloth-kubernetes kubectl describe node masters-1
```

### Get Cluster Info

```bash
sloth-kubernetes kubectl cluster-info
```

---

## Pod Management

### List All Pods

```bash
# All namespaces
sloth-kubernetes kubectl get pods -A

# Specific namespace
sloth-kubernetes kubectl get pods -n kube-system
```

### Get Pod Details

```bash
sloth-kubernetes kubectl describe pod nginx-7d9b8c6b5-x2k9m -n default
```

### View Pod Logs

```bash
# Current logs
sloth-kubernetes kubectl logs nginx-7d9b8c6b5-x2k9m

# Follow logs
sloth-kubernetes kubectl logs -f nginx-7d9b8c6b5-x2k9m

# Previous container logs
sloth-kubernetes kubectl logs nginx-7d9b8c6b5-x2k9m --previous

# All containers in pod
sloth-kubernetes kubectl logs nginx-7d9b8c6b5-x2k9m --all-containers
```

### Execute Commands in Pod

```bash
# Interactive shell
sloth-kubernetes kubectl exec -it nginx-7d9b8c6b5-x2k9m -- /bin/bash

# Run single command
sloth-kubernetes kubectl exec nginx-7d9b8c6b5-x2k9m -- cat /etc/nginx/nginx.conf
```

---

## Deployments

### Create Deployment

```bash
sloth-kubernetes kubectl create deployment nginx --image=nginx
```

### Scale Deployment

```bash
sloth-kubernetes kubectl scale deployment nginx --replicas=3
```

### Update Deployment

```bash
sloth-kubernetes kubectl set image deployment/nginx nginx=nginx:1.25
```

### Rollout Status

```bash
sloth-kubernetes kubectl rollout status deployment/nginx
```

### Rollback Deployment

```bash
sloth-kubernetes kubectl rollout undo deployment/nginx
```

---

## Services

### Create Service

```bash
# Expose deployment as LoadBalancer
sloth-kubernetes kubectl expose deployment nginx --port=80 --type=LoadBalancer

# Expose as ClusterIP
sloth-kubernetes kubectl expose deployment nginx --port=80 --type=ClusterIP
```

### List Services

```bash
sloth-kubernetes kubectl get services
```

### Get Service Details

```bash
sloth-kubernetes kubectl describe service nginx
```

---

## Apply Manifests

### Apply from File

```bash
sloth-kubernetes kubectl apply -f deployment.yaml
```

### Apply from URL

```bash
sloth-kubernetes kubectl apply -f https://example.com/manifest.yaml
```

### Apply from Directory

```bash
sloth-kubernetes kubectl apply -f ./manifests/
```

### Delete Resources

```bash
sloth-kubernetes kubectl delete -f deployment.yaml
```

---

## Namespaces

### List Namespaces

```bash
sloth-kubernetes kubectl get namespaces
```

### Create Namespace

```bash
sloth-kubernetes kubectl create namespace production
```

### Set Default Namespace

```bash
sloth-kubernetes kubectl config set-context --current --namespace=production
```

---

## ConfigMaps and Secrets

### Create ConfigMap

```bash
# From literal values
sloth-kubernetes kubectl create configmap app-config \
  --from-literal=DATABASE_URL=postgres://db:5432/app

# From file
sloth-kubernetes kubectl create configmap app-config --from-file=config.json
```

### Create Secret

```bash
# From literal values
sloth-kubernetes kubectl create secret generic db-credentials \
  --from-literal=username=admin \
  --from-literal=password=secret123

# From file
sloth-kubernetes kubectl create secret generic tls-cert \
  --from-file=cert.pem --from-file=key.pem
```

### View Secret (base64 decoded)

```bash
sloth-kubernetes kubectl get secret db-credentials -o jsonpath='{.data.password}' | base64 -d
```

---

## Resource Management

### Get Resource Usage

```bash
# Node resources
sloth-kubernetes kubectl top nodes

# Pod resources
sloth-kubernetes kubectl top pods -A
```

### Get Events

```bash
sloth-kubernetes kubectl get events --sort-by='.lastTimestamp'
```

### Debug Node

```bash
sloth-kubernetes kubectl debug node/workers-1 -it --image=busybox
```

---

## Node Operations

### Cordon Node

Prevent new pods from being scheduled:

```bash
sloth-kubernetes kubectl cordon workers-1
```

### Drain Node

Safely evict pods for maintenance:

```bash
sloth-kubernetes kubectl drain workers-1 --ignore-daemonsets --delete-emptydir-data
```

### Uncordon Node

Allow scheduling again:

```bash
sloth-kubernetes kubectl uncordon workers-1
```

---

## Troubleshooting

### Pod Not Starting

```bash
# Check pod status
sloth-kubernetes kubectl describe pod <pod-name>

# Check events
sloth-kubernetes kubectl get events --field-selector involvedObject.name=<pod-name>
```

### Service Not Reachable

```bash
# Check endpoints
sloth-kubernetes kubectl get endpoints <service-name>

# Check service selector matches pod labels
sloth-kubernetes kubectl get pods --show-labels
```

### Resource Limits

```bash
# Check resource requests/limits
sloth-kubernetes kubectl describe pod <pod-name> | grep -A 5 "Limits:"

# Check node capacity
sloth-kubernetes kubectl describe node <node-name> | grep -A 10 "Allocated resources:"
```

---

## Common Operations

### Quick Health Check

```bash
# Check nodes
sloth-kubernetes kubectl get nodes

# Check system pods
sloth-kubernetes kubectl get pods -n kube-system

# Check events
sloth-kubernetes kubectl get events -A --sort-by='.lastTimestamp' | tail -20
```

### Deploy Application

```bash
# Create deployment
sloth-kubernetes kubectl create deployment myapp --image=myapp:v1

# Expose service
sloth-kubernetes kubectl expose deployment myapp --port=80 --type=LoadBalancer

# Check status
sloth-kubernetes kubectl get pods,svc
```

### View Logs Across Pods

```bash
sloth-kubernetes kubectl logs -l app=nginx --all-containers
```

---

## Kubeconfig

### Export Kubeconfig

```bash
# Save to file
sloth-kubernetes kubeconfig > ~/.kube/config

# Use with standard kubectl
export KUBECONFIG=~/.kube/config
kubectl get nodes
```

### Use with External kubectl

```bash
sloth-kubernetes kubeconfig > cluster.kubeconfig
kubectl --kubeconfig=cluster.kubeconfig get nodes
```

---

## Next Steps

- [Salt Commands](salt.md) - Node management
- [Stack Management](stacks.md) - Infrastructure state
- [CLI Reference](cli-reference.md) - All available commands
