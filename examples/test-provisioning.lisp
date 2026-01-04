; Sloth Kubernetes - Test Provisioning Features
; Small test cluster to validate new provisioning features

(cluster
  ; Cluster metadata
  (metadata
    (name "test-provisioning")
    (environment "test")
    (description "Test cluster for provisioning features"))

  ; Cluster specification
  (cluster
    (type "rke2")
    (version "v1.29.0+rke2r1"))

  ; AWS Provider
  (providers
    (aws
      (enabled true)
      (region "us-east-1")
      (vpc
        (create true)
        (cidr "10.0.0.0/16"))))

  ; Network configuration
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
      (mesh-networking true)))

  ; Security configuration
  (security
    (ssh
      (key-path "~/.ssh/id_rsa")
      (public-key-path "~/.ssh/id_rsa.pub")
      (port 22)))

  ; Node pools with new features
  (node-pools
    ; Master node - single for test
    (masters
      (name "control-plane")
      (provider "aws")
      (region "us-east-1")
      (count 1)
      (roles master etcd)
      (size "t3.medium")
      (labels
        (role "control-plane")
        (tier "master")))

    ; Worker nodes with Spot and Multi-AZ
    (workers
      (name "app-workers")
      (provider "aws")
      (region "us-east-1")
      (count 2)
      (roles worker)
      (size "t3.small")
      (labels
        (role "worker")
        (tier "application"))
      ; Spot Instance configuration
      (spot-config
        (enabled true)
        (max-price "0.02")
        (fallback-on-demand true)
        (spot-percentage 100))
      ; Multi-AZ Distribution
      (distribution
        (zone "us-east-1a" (count 1))
        (zone "us-east-1b" (count 1)))))

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
    (monthly-limit 100)
    (alert-threshold 80)))
