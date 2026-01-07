; Sloth Kubernetes - Simple AWS Salt Push Test Configuration
; Minimal cluster with Salt for testing push commands

(cluster
  (metadata
    (name "salt-push-test")
    (environment "test")
    (description "Simple AWS cluster for testing Salt push commands"))

  (cluster
    (type "rke2")
    (version "v1.29.0+rke2r1")
    (high-availability false)
    (multi-cloud false))

  ; Cloud providers - AWS only
  (providers
    (aws
      (enabled true)
      (region "us-east-1")))

  ; WireGuard mesh networking
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
      (mesh-networking true)
      (auto-config true)))

  ; Salt configuration management
  (salt
    (enabled true)
    (master
      (provider "aws")
      (region "us-east-1")
      (size "t3.small")
      (name "salt-master"))
    (api
      (enabled true)
      (port 8000)
      (ssl false))
    (minion
      (auto-accept true)))

  (security
    (ssh
      (auto-generate true))
    (bastion
      (enabled true)
      (provider "aws")
      (region "us-east-1")
      (size "t3.micro")
      (name "bastion-test")))

  ; Node pools: 1 master + 1 worker (minimal)
  (node-pools
    (masters
      (name "masters")
      (provider "aws")
      (region "us-east-1")
      (count 1)
      (roles master etcd)
      (size "t3.medium")
      (labels
        (role "control-plane")))

    (workers
      (name "workers")
      (provider "aws")
      (region "us-east-1")
      (count 1)
      (roles worker)
      (size "t3.medium")
      (labels
        (role "worker"))))

  (kubernetes
    (version "v1.29.0")
    (distribution "rke2")
    (network-plugin "canal")
    (cluster-domain "cluster.local"))

  (monitoring
    (enabled false)))
