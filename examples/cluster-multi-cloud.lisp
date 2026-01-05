; Sloth Kubernetes - Multi-Cloud Cluster Configuration
; Cluster spanning multiple cloud providers with WireGuard mesh
; Uses Lisp built-in functions for dynamic configuration

(cluster
  (metadata
    (name (concat "multi-cloud-" (env "CLUSTER_SUFFIX" "cluster")))
    (environment (env "CLUSTER_ENV" "production"))
    (description "Kubernetes cluster spanning AWS, Azure, and DigitalOcean")
    (owner (env "CLUSTER_OWNER" "platform-team"))
    (labels
      (project "sloth-kubernetes")
      (type "multi-cloud")
      (created-at (now))
      (config-hash (sha256 (concat (env "CLUSTER_SUFFIX" "cluster") (now))))))

  (cluster
    (type "rke2")
    (version (env "RKE2_VERSION" "v1.29.0+rke2r1"))
    (high-availability true)
    (multi-cloud true))

  ; Multiple cloud providers enabled
  (providers
    (aws
      (enabled true)
      (region (env "AWS_REGION" "us-east-1")))
    (azure
      (enabled true)
      (subscription-id (env "AZURE_SUBSCRIPTION_ID"))
      (tenant-id (env "AZURE_TENANT_ID"))
      (client-id (env "AZURE_CLIENT_ID"))
      (client-secret (env "AZURE_CLIENT_SECRET"))
      (resource-group (env "AZURE_RESOURCE_GROUP" "sloth-k8s-rg"))
      (location (env "AZURE_LOCATION" "eastus")))
    (digitalocean
      (enabled true)
      (token (env "DIGITALOCEAN_TOKEN"))
      (region (env "DO_REGION" "nyc3"))))

  ; WireGuard mesh connects all providers
  (network
    (mode "wireguard")
    (pod-cidr (env "POD_CIDR" "10.42.0.0/16"))
    (service-cidr (env "SERVICE_CIDR" "10.43.0.0/16"))
    (cross-provider-networking true)
    (wireguard
      (enabled true)
      (create true)
      (provider "digitalocean")
      (region (env "DO_REGION" "nyc3"))
      (subnet-cidr "10.8.0.0/24")
      (port (default (env "WIREGUARD_PORT") 51820))
      (mesh-networking true)
      (auto-config true)))

  (security
    (ssh
      (key-path (env "SSH_KEY_PATH" "~/.ssh/id_rsa"))
      (public-key-path (env "SSH_PUBLIC_KEY_PATH" "~/.ssh/id_rsa.pub")))
    (bastion
      (enabled true)
      (provider "digitalocean")
      (region (env "DO_REGION" "nyc3"))
      (size "s-1vcpu-1gb")
      (name (concat "bastion-" (uuid-short)))))

  ; Node pools distributed across providers
  (node-pools
    ; AWS masters for primary control plane
    (aws-masters
      (name "aws-masters")
      (provider "aws")
      (region (env "AWS_REGION" "us-east-1"))
      (count (default (env "AWS_MASTER_COUNT") 3))
      (roles master etcd)
      (size (env "AWS_MASTER_SIZE" "t3.medium"))
      (labels
        (cloud "aws")
        (role "control-plane")))

    ; AWS workers
    (aws-workers
      (name "aws-workers")
      (provider "aws")
      (region (env "AWS_REGION" "us-east-1"))
      (count (default (env "AWS_WORKER_COUNT") 3))
      (roles worker)
      (size (env "AWS_WORKER_SIZE" "t3.large"))
      (spot-instance (env "USE_SPOT_INSTANCES" true))
      (labels
        (cloud "aws")
        (role "worker")))

    ; Azure workers for geographic distribution
    (azure-workers
      (name "azure-workers")
      (provider "azure")
      (region (env "AZURE_LOCATION" "eastus"))
      (count (default (env "AZURE_WORKER_COUNT") 2))
      (roles worker)
      (size (env "AZURE_WORKER_SIZE" "Standard_D2s_v3"))
      (labels
        (cloud "azure")
        (role "worker")))

    ; DigitalOcean workers for cost optimization
    (do-workers
      (name "do-workers")
      (provider "digitalocean")
      (region (env "DO_REGION" "nyc3"))
      (count (default (env "DO_WORKER_COUNT") 2))
      (roles worker)
      (size (env "DO_WORKER_SIZE" "s-4vcpu-8gb"))
      (labels
        (cloud "digitalocean")
        (role "worker"))))

  (kubernetes
    (version (env "K8S_VERSION" "v1.29.0"))
    (distribution "rke2")
    (network-plugin (env "CNI_PLUGIN" "canal"))
    (cluster-domain "cluster.local"))

  (monitoring
    (enabled (env "MONITORING_ENABLED" true))
    (prometheus
      (enabled true)
      (retention (env "PROMETHEUS_RETENTION" "30d")))
    (grafana
      (enabled true))))
