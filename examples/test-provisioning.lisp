; Sloth Kubernetes - Test Provisioning Features
; Small test cluster to validate new provisioning features
; Uses Lisp built-in functions for dynamic configuration

(cluster
  ; Cluster metadata
  (metadata
    (name (env "TEST_CLUSTER_NAME" "test-provisioning"))
    (environment (env "CLUSTER_ENV" "test"))
    (description "Test cluster for provisioning features")
    (labels
      (test-run-id (uuid-short))
      (created-at (now))))

  ; Cluster specification
  (cluster
    (type "rke2")
    (version (env "RKE2_VERSION" "v1.29.0+rke2r1")))

  ; AWS Provider
  (providers
    (aws
      (enabled true)
      (region (env "AWS_REGION" "us-east-1"))
      (vpc
        (create true)
        (cidr (env "VPC_CIDR" "10.0.0.0/16")))))

  ; Network configuration
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
      (mesh-networking true)))

  ; Security configuration
  (security
    (ssh
      (key-path (env "SSH_KEY_PATH" "~/.ssh/id_rsa"))
      (public-key-path (env "SSH_PUBLIC_KEY_PATH" "~/.ssh/id_rsa.pub"))
      (port 22)))

  ; Node pools with new features
  (node-pools
    ; Master node - single for test
    (masters
      (name "control-plane")
      (provider "aws")
      (region (env "AWS_REGION" "us-east-1"))
      (count (default (env "MASTER_COUNT") 1))
      (roles master etcd)
      (size (env "MASTER_SIZE" "t3.medium"))
      (labels
        (role "control-plane")
        (tier "master")))

    ; Worker nodes with Spot and Multi-AZ
    (workers
      (name "app-workers")
      (provider "aws")
      (region (env "AWS_REGION" "us-east-1"))
      (count (default (env "WORKER_COUNT") 2))
      (roles worker)
      (size (env "WORKER_SIZE" "t3.small"))
      (labels
        (role "worker")
        (tier "application"))
      ; Spot Instance configuration
      (spot-config
        (enabled (env "USE_SPOT_INSTANCES" true))
        (max-price (env "SPOT_MAX_PRICE" "0.02"))
        (fallback-on-demand true)
        (spot-percentage (default (env "SPOT_PERCENTAGE") 100)))
      ; Multi-AZ Distribution
      (distribution
        (zone "us-east-1a" (count 1))
        (zone "us-east-1b" (count 1)))))

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
      (secrets-encryption true)))

  ; Provisioning Hooks
  (hooks
    (post-node-create
      (script "echo 'Node created successfully'"))
    (post-cluster-ready
      (kubectl "get nodes")))

  ; Cost Control
  (cost-control
    (estimate true)
    (monthly-limit (default (env "MONTHLY_COST_LIMIT") 100))
    (alert-threshold (default (env "COST_ALERT_THRESHOLD") 80))))
