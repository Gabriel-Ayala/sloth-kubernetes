; Sloth Kubernetes - Advanced Features Example
; This example demonstrates all the new provisioning features
; Uses Lisp built-in functions for dynamic configuration

(cluster
  ; Cluster metadata
  (metadata
    (name (concat "advanced-" (env "CLUSTER_SUFFIX" "cluster")))
    (environment (env "CLUSTER_ENV" "production"))
    (description "Cluster with all advanced features enabled")
    (owner (env "CLUSTER_OWNER" "platform-team"))
    (labels
      (project "sloth-kubernetes")
      (created-at (now))
      (config-version (sha256 (concat "advanced" (now))))))

  ; Cluster specification
  (cluster
    (type "rke2")
    (version (env "RKE2_VERSION" "v1.29.0+rke2r1"))
    (high-availability true))

  ; Cloud providers
  (providers
    (aws
      (enabled true)
      (region (env "AWS_REGION" "us-east-1"))
      (vpc
        (create true)
        (cidr (env "VPC_CIDR" "10.0.0.0/16"))
        (nat-gateway true)
        (internet-gateway true))))

  ; Network configuration with Private Cluster
  (network
    (mode "wireguard")
    (pod-cidr (env "POD_CIDR" "10.42.0.0/16"))
    (service-cidr (env "SERVICE_CIDR" "10.43.0.0/16"))
    (wireguard
      (enabled true)
      (create true)
      (provider "aws")
      (region (env "AWS_REGION" "us-east-1"))
      (subnet-cidr "10.8.0.0/24")
      (port (default (env "WIREGUARD_PORT") 51820))
      (mesh-networking true))
    ; Private Cluster Mode
    (private-cluster
      (enabled (env "PRIVATE_CLUSTER" true))
      (nat-gateway true)
      (private-endpoint true)
      (public-endpoint false)
      (allowed-cidrs (env "PRIVATE_ALLOWED_CIDRS" "10.0.0.0/8" "192.168.0.0/16"))
      (vpn-required true)))

  ; Security configuration
  (security
    (ssh
      (key-path (env "SSH_KEY_PATH" "~/.ssh/id_rsa"))
      (public-key-path (env "SSH_PUBLIC_KEY_PATH" "~/.ssh/id_rsa.pub"))
      (port 22))
    (bastion
      (enabled true)
      (provider "aws")
      (region (env "AWS_REGION" "us-east-1"))
      (size "t3.micro")
      (name (concat "cluster-bastion-" (uuid-short)))
      (ssh-port 22)
      (enable-audit-log true)))

  ; Node pools with advanced configurations
  (node-pools
    ; Master nodes with autoscaling disabled
    (masters
      (name "control-plane")
      (provider "aws")
      (region (env "AWS_REGION" "us-east-1"))
      (count (default (env "MASTER_COUNT") 3))
      (roles master etcd)
      (size (env "AWS_MASTER_SIZE" "t3.medium"))
      (labels
        (role "control-plane")
        (tier "master")))

    ; Worker nodes with Auto-Scaling
    (workers
      (name "app-workers")
      (provider "aws")
      (region (env "AWS_REGION" "us-east-1"))
      (count (default (env "WORKER_COUNT") 3))
      (roles worker)
      (size (env "AWS_WORKER_SIZE" "t3.large"))
      (labels
        (role "worker")
        (tier "application"))
      ; Auto-Scaling configuration
      (autoscaling
        (enabled (env "AUTOSCALING_ENABLED" true))
        (min-nodes (default (env "MIN_WORKERS") 2))
        (max-nodes (default (env "MAX_WORKERS") 10))
        (target-cpu (default (env "TARGET_CPU") 70))
        (scale-down-delay 300))
      ; Spot Instance configuration
      (spot-config
        (enabled (env "USE_SPOT_INSTANCES" true))
        (max-price (env "SPOT_MAX_PRICE" "0.05"))
        (fallback-on-demand true)
        (spot-percentage (default (env "SPOT_PERCENTAGE") 70)))
      ; Multi-AZ Distribution
      (distribution
        (zone "us-east-1a" (count 2))
        (zone "us-east-1b" (count 2))
        (zone "us-east-1c" (count 1))))

    ; GPU workers with taints
    (gpu-workers
      (name "gpu-pool")
      (provider "aws")
      (region (env "AWS_REGION" "us-east-1"))
      (count (default (env "GPU_WORKER_COUNT") 2))
      (roles worker)
      (size (env "AWS_GPU_SIZE" "p3.2xlarge"))
      (labels
        (accelerator "nvidia-tesla-v100")
        (workload "ml-training"))
      ; Taints configuration
      (taints
        (taint "nvidia.com/gpu" "NoSchedule"))
      ; Custom image
      (image
        (type "custom")
        (id (env "GPU_AMI_ID" "ami-gpu-optimized-12345"))
        (user "ubuntu"))))

  ; Kubernetes configuration
  (kubernetes
    (version (env "K8S_VERSION" "v1.29.0"))
    (distribution "rke2")
    (network-plugin (env "CNI_PLUGIN" "canal"))
    (pod-cidr (env "POD_CIDR" "10.42.0.0/16"))
    (service-cidr (env "SERVICE_CIDR" "10.43.0.0/16"))
    (cluster-domain "cluster.local")
    (rke2
      (version (env "RKE2_VERSION" "v1.29.0+rke2r1"))
      (channel "stable")
      (disable-components "rke2-ingress-nginx")
      (secrets-encryption true)
      (snapshot-schedule-cron (env "SNAPSHOT_CRON" "0 */6 * * *"))
      (snapshot-retention (default (env "SNAPSHOT_RETENTION") 5))))

  ; Rolling Upgrade configuration
  (upgrade
    (strategy "rolling")
    (max-unavailable 1)
    (max-surge 1)
    (drain-timeout (default (env "DRAIN_TIMEOUT") 300))
    (health-check-interval 30)
    (pause-on-failure true)
    (auto-rollback true))

  ; Backup configuration
  (backup
    (enabled (env "BACKUP_ENABLED" true))
    (schedule (env "BACKUP_SCHEDULE" "0 2 * * *"))
    (retention-days (default (env "BACKUP_RETENTION_DAYS") 7))
    (components etcd volumes secrets)
    (storage
      (type "s3")
      (bucket (env "BACKUP_S3_BUCKET" "my-k8s-backups"))
      (region (env "AWS_REGION" "us-east-1"))
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
    (monthly-limit (default (env "MONTHLY_COST_LIMIT") 500))
    (alert-threshold (default (env "COST_ALERT_THRESHOLD") 80))
    (notify (env "COST_ALERT_EMAIL" "team@example.com"))
    (right-sizing true)
    (unused-resources-alert true))

  ; Monitoring
  (monitoring
    (enabled (env "MONITORING_ENABLED" true))
    (prometheus
      (enabled true)
      (retention (env "PROMETHEUS_RETENTION" "15d")))
    (grafana
      (enabled true))))
