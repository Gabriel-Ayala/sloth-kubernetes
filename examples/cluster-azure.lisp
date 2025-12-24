; Sloth Kubernetes - Azure Cluster Configuration
; Complete Azure cluster with WireGuard mesh VPN

(cluster
  (metadata
    (name "sloth-azure-cluster")
    (environment "production")
    (description "Kubernetes cluster on Azure")
    (owner "chalkan3")
    (labels
      (project "sloth-kubernetes")
      (provider "azure")))

  (cluster
    (type "rke2")
    (version "v1.29.0+rke2r1")
    (high-availability true))

  (providers
    (azure
      (enabled true)
      (subscription-id "${AZURE_SUBSCRIPTION_ID}")
      (tenant-id "${AZURE_TENANT_ID}")
      (client-id "${AZURE_CLIENT_ID}")
      (client-secret "${AZURE_CLIENT_SECRET}")
      (resource-group "sloth-kubernetes-rg")
      (location "eastus")))

  (network
    (mode "wireguard")
    (pod-cidr "10.42.0.0/16")
    (service-cidr "10.43.0.0/16")
    (wireguard
      (enabled true)
      (create true)
      (provider "azure")
      (region "eastus")
      (subnet-cidr "10.8.0.0/24")
      (port 51820)
      (mesh-networking true)
      (auto-config true)))

  (security
    (ssh
      (key-path "~/.ssh/id_rsa")
      (public-key-path "~/.ssh/id_rsa.pub")
      (port 22))
    (bastion
      (enabled true)
      (provider "azure")
      (region "eastus")
      (size "Standard_B1s")
      (name "sloth-bastion")
      (ssh-port 22)
      (allowed-cidrs "0.0.0.0/0")
      (enable-audit-log true)))

  (node-pools
    (masters
      (name "azure-masters")
      (provider "azure")
      (region "eastus")
      (count 3)
      (roles master etcd)
      (size "Standard_D2s_v3")
      (labels
        (role "control-plane")))
    (workers
      (name "azure-workers")
      (provider "azure")
      (region "eastus")
      (count 3)
      (roles worker)
      (size "Standard_D4s_v3")
      (labels
        (role "worker"))))

  (kubernetes
    (version "v1.29.0")
    (distribution "rke2")
    (network-plugin "canal")
    (pod-cidr "10.42.0.0/16")
    (service-cidr "10.43.0.0/16")
    (cluster-domain "cluster.local")
    (rke2
      (version "v1.29.0+rke2r1")
      (channel "stable")))

  (monitoring
    (enabled true)
    (prometheus
      (enabled true)
      (retention "15d"))
    (grafana
      (enabled true))))
