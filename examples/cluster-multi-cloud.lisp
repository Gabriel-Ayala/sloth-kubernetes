; Sloth Kubernetes - Multi-Cloud Cluster Configuration
; Cluster spanning multiple cloud providers with WireGuard mesh

(cluster
  (metadata
    (name "multi-cloud-cluster")
    (environment "production")
    (description "Kubernetes cluster spanning AWS, Azure, and DigitalOcean")
    (owner "platform-team")
    (labels
      (project "sloth-kubernetes")
      (type "multi-cloud")))

  (cluster
    (type "rke2")
    (version "v1.29.0+rke2r1")
    (high-availability true)
    (multi-cloud true))

  ; Multiple cloud providers enabled
  (providers
    (aws
      (enabled true)
      (region "us-east-1"))
    (azure
      (enabled true)
      (subscription-id "${AZURE_SUBSCRIPTION_ID}")
      (tenant-id "${AZURE_TENANT_ID}")
      (client-id "${AZURE_CLIENT_ID}")
      (client-secret "${AZURE_CLIENT_SECRET}")
      (resource-group "sloth-k8s-rg")
      (location "eastus"))
    (digitalocean
      (enabled true)
      (token "${DIGITALOCEAN_TOKEN}")
      (region "nyc3")))

  ; WireGuard mesh connects all providers
  (network
    (mode "wireguard")
    (pod-cidr "10.42.0.0/16")
    (service-cidr "10.43.0.0/16")
    (cross-provider-networking true)
    (wireguard
      (enabled true)
      (create true)
      (provider "digitalocean")
      (region "nyc3")
      (subnet-cidr "10.8.0.0/24")
      (port 51820)
      (mesh-networking true)
      (auto-config true)))

  (security
    (ssh
      (key-path "~/.ssh/id_rsa")
      (public-key-path "~/.ssh/id_rsa.pub"))
    (bastion
      (enabled true)
      (provider "digitalocean")
      (region "nyc3")
      (size "s-1vcpu-1gb")
      (name "bastion")))

  ; Node pools distributed across providers
  (node-pools
    ; AWS masters for primary control plane
    (aws-masters
      (name "aws-masters")
      (provider "aws")
      (region "us-east-1")
      (count 3)
      (roles master etcd)
      (size "t3.medium")
      (labels
        (cloud "aws")
        (role "control-plane")))

    ; AWS workers
    (aws-workers
      (name "aws-workers")
      (provider "aws")
      (region "us-east-1")
      (count 3)
      (roles worker)
      (size "t3.large")
      (spot-instance true)
      (labels
        (cloud "aws")
        (role "worker")))

    ; Azure workers for geographic distribution
    (azure-workers
      (name "azure-workers")
      (provider "azure")
      (region "eastus")
      (count 2)
      (roles worker)
      (size "Standard_D2s_v3")
      (labels
        (cloud "azure")
        (role "worker")))

    ; DigitalOcean workers for cost optimization
    (do-workers
      (name "do-workers")
      (provider "digitalocean")
      (region "nyc3")
      (count 2)
      (roles worker)
      (size "s-4vcpu-8gb")
      (labels
        (cloud "digitalocean")
        (role "worker"))))

  (kubernetes
    (version "v1.29.0")
    (distribution "rke2")
    (network-plugin "canal")
    (cluster-domain "cluster.local"))

  (monitoring
    (enabled true)
    (prometheus
      (enabled true)
      (retention "30d"))
    (grafana
      (enabled true))))
