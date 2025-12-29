; Sloth Kubernetes - Advanced Features Example
; This example demonstrates all the new provisioning features

(cluster
  ; Cluster metadata
  (metadata
    (name "advanced-cluster")
    (environment "production")
    (description "Cluster with all advanced features enabled")
    (owner "platform-team"))

  ; Cluster specification
  (cluster
    (type "rke2")
    (version "v1.29.0+rke2r1")
    (high-availability true))

  ; Cloud providers
  (providers
    (aws
      (enabled true)
      (region "us-east-1")
      (vpc
        (create true)
        (cidr "10.0.0.0/16")
        (nat-gateway true)
        (internet-gateway true))))

  ; Network configuration with Private Cluster
  (network
    (mode "wireguard")
    (pod-cidr "10.42.0.0/16")
    (service-cidr "10.43.0.0/16")
    (wireguard
      (enabled true)
      (create true)
      (provider "aws")
      (region "us-east-1")
      (subnet-cidr "10.8.0.0/24")
      (port 51820)
      (mesh-networking true))
    ; Private Cluster Mode
    (private-cluster
      (enabled true)
      (nat-gateway true)
      (private-endpoint true)
      (public-endpoint false)
      (allowed-cidrs "10.0.0.0/8" "192.168.0.0/16")
      (vpn-required true)))

  ; Security configuration
  (security
    (ssh
      (key-path "~/.ssh/id_rsa")
      (public-key-path "~/.ssh/id_rsa.pub")
      (port 22))
    (bastion
      (enabled true)
      (provider "aws")
      (region "us-east-1")
      (size "t3.micro")
      (name "cluster-bastion")
      (ssh-port 22)
      (enable-audit-log true)))

  ; Node pools with advanced configurations
  (node-pools
    ; Master nodes with autoscaling disabled
    (masters
      (name "control-plane")
      (provider "aws")
      (region "us-east-1")
      (count 3)
      (roles master etcd)
      (size "t3.medium")
      (labels
        (role "control-plane")
        (tier "master")))

    ; Worker nodes with Auto-Scaling
    (workers
      (name "app-workers")
      (provider "aws")
      (region "us-east-1")
      (count 3)
      (roles worker)
      (size "t3.large")
      (labels
        (role "worker")
        (tier "application"))
      ; Auto-Scaling configuration
      (autoscaling
        (enabled true)
        (min-nodes 2)
        (max-nodes 10)
        (target-cpu 70)
        (scale-down-delay 300))
      ; Spot Instance configuration
      (spot-config
        (enabled true)
        (max-price "0.05")
        (fallback-on-demand true)
        (spot-percentage 70))
      ; Multi-AZ Distribution
      (distribution
        (zone "us-east-1a" (count 2))
        (zone "us-east-1b" (count 2))
        (zone "us-east-1c" (count 1))))

    ; GPU workers with taints
    (gpu-workers
      (name "gpu-pool")
      (provider "aws")
      (region "us-east-1")
      (count 2)
      (roles worker)
      (size "p3.2xlarge")
      (labels
        (accelerator "nvidia-tesla-v100")
        (workload "ml-training"))
      ; Taints configuration
      (taints
        (taint "nvidia.com/gpu" "NoSchedule"))
      ; Custom image
      (image
        (type "custom")
        (id "ami-gpu-optimized-12345")
        (user "ubuntu"))))

  ; Kubernetes configuration
  (kubernetes
    (version "v1.29.0")
    (distribution "rke2")
    (network-plugin "canal")
    (pod-cidr "10.42.0.0/16")
    (service-cidr "10.43.0.0/16")
    (cluster-domain "cluster.local")
    (rke2
      (version "v1.29.0+rke2r1")
      (channel "stable")
      (disable-components "rke2-ingress-nginx")
      (secrets-encryption true)
      (snapshot-schedule-cron "0 */6 * * *")
      (snapshot-retention 5)))

  ; Rolling Upgrade configuration
  (upgrade
    (strategy "rolling")
    (max-unavailable 1)
    (max-surge 1)
    (drain-timeout 300)
    (health-check-interval 30)
    (pause-on-failure true)
    (auto-rollback true))

  ; Backup configuration
  (backup
    (enabled true)
    (schedule "0 2 * * *")
    (retention-days 7)
    (components etcd volumes secrets)
    (storage
      (type "s3")
      (bucket "my-k8s-backups")
      (region "us-east-1")
      (path "backups/")))

  ; Provisioning Hooks
  (hooks
    (post-node-create
      (script "/scripts/install-monitoring-agent.sh")
      (script "/scripts/configure-security.sh"))
    (pre-cluster-destroy
      (script "/scripts/backup-all.sh"))
    (post-cluster-ready
      (kubectl "apply -f /manifests/essential.yaml")))

  ; Cost Control configuration
  (cost-control
    (estimate true)
    (monthly-limit 500)
    (alert-threshold 80)
    (notify "team@example.com")
    (right-sizing true)
    (unused-resources-alert true))

  ; Monitoring
  (monitoring
    (enabled true)
    (prometheus
      (enabled true)
      (retention "15d"))
    (grafana
      (enabled true))))
