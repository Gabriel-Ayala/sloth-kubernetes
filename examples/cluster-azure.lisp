; Sloth Kubernetes - Azure Cluster Configuration
; Complete Azure cluster with WireGuard mesh VPN
; Uses Lisp built-in functions for dynamic configuration

(cluster
  (metadata
    (name (concat "sloth-azure-" (env "CLUSTER_SUFFIX" "cluster")))
    (environment (env "CLUSTER_ENV" "production"))
    (description "Kubernetes cluster on Azure")
    (owner (env "CLUSTER_OWNER" "chalkan3"))
    (labels
      (project "sloth-kubernetes")
      (provider "azure")
      (created-at (now))))

  (cluster
    (type "rke2")
    (version (env "RKE2_VERSION" "v1.29.0+rke2r1"))
    (high-availability true))

  (providers
    (azure
      (enabled true)
      (subscription-id (env "AZURE_SUBSCRIPTION_ID"))
      (tenant-id (env "AZURE_TENANT_ID"))
      (client-id (env "AZURE_CLIENT_ID"))
      (client-secret (env "AZURE_CLIENT_SECRET"))
      (resource-group (env "AZURE_RESOURCE_GROUP" "sloth-kubernetes-rg"))
      (location (env "AZURE_LOCATION" "eastus"))))

  (network
    (mode "wireguard")
    (pod-cidr (env "POD_CIDR" "10.42.0.0/16"))
    (service-cidr (env "SERVICE_CIDR" "10.43.0.0/16"))
    (wireguard
      (enabled true)
      (create true)
      (provider "azure")
      (region (env "AZURE_LOCATION" "eastus"))
      (subnet-cidr "10.8.0.0/24")
      (port (default (env "WIREGUARD_PORT") 51820))
      (mesh-networking true)
      (auto-config true)))

  (security
    (ssh
      (key-path (env "SSH_KEY_PATH" "~/.ssh/id_rsa"))
      (public-key-path (env "SSH_PUBLIC_KEY_PATH" "~/.ssh/id_rsa.pub"))
      (port 22))
    (bastion
      (enabled true)
      (provider "azure")
      (region (env "AZURE_LOCATION" "eastus"))
      (size "Standard_B1s")
      (name (concat "sloth-bastion-" (uuid-short)))
      (ssh-port 22)
      (allowed-cidrs (env "BASTION_ALLOWED_CIDRS" "0.0.0.0/0"))
      (enable-audit-log true)))

  (node-pools
    (masters
      (name "azure-masters")
      (provider "azure")
      (region (env "AZURE_LOCATION" "eastus"))
      (count (default (env "MASTER_COUNT") 3))
      (roles master etcd)
      (size (env "AZURE_MASTER_SIZE" "Standard_D2s_v3"))
      (labels
        (role "control-plane")))
    (workers
      (name "azure-workers")
      (provider "azure")
      (region (env "AZURE_LOCATION" "eastus"))
      (count (default (env "WORKER_COUNT") 3))
      (roles worker)
      (size (env "AZURE_WORKER_SIZE" "Standard_D4s_v3"))
      (labels
        (role "worker"))))

  (kubernetes
    (version (env "K8S_VERSION" "v1.29.0"))
    (distribution "rke2")
    (network-plugin (env "CNI_PLUGIN" "canal"))
    (pod-cidr (env "POD_CIDR" "10.42.0.0/16"))
    (service-cidr (env "SERVICE_CIDR" "10.43.0.0/16"))
    (cluster-domain "cluster.local")
    (rke2
      (version (env "RKE2_VERSION" "v1.29.0+rke2r1"))
      (channel "stable")))

  (monitoring
    (enabled (env "MONITORING_ENABLED" true))
    (prometheus
      (enabled true)
      (retention (env "PROMETHEUS_RETENTION" "15d")))
    (grafana
      (enabled true))))
