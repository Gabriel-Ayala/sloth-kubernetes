; Sloth Kubernetes - Simple Salt Push Test Configuration
; Minimal cluster with Salt for testing push commands
; Uses only Hetzner for cost efficiency

(cluster
  (metadata
    (name "salt-push-test")
    (environment "test")
    (description "Simple cluster for testing Salt push commands"))

  (cluster
    (type "rke2")
    (version "v1.29.0+rke2r1")
    (high-availability false)
    (multi-cloud false))

  ; Cloud providers - Hetzner only
  (providers
    (hetzner
      (enabled true)
      (token (env "HETZNER_TOKEN"))
      (location "nbg1")))

  ; WireGuard mesh networking
  (network
    (mode "wireguard")
    (pod-cidr "10.42.0.0/16")
    (service-cidr "10.43.0.0/16")
    (wireguard
      (enabled true)
      (create true)
      (provider "hetzner")
      (region "nbg1")
      (subnet-cidr "10.8.0.0/24")
      (port 51820)
      (mesh-networking true)
      (auto-config true)))

  ; Salt configuration management
  (salt
    (enabled true)
    (master
      (provider "hetzner")
      (region "nbg1")
      (size "cx21")
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
      (provider "hetzner")
      (region "nbg1")
      (size "cx21")
      (name "bastion-test")))

  ; Node pools: 1 master + 1 worker (minimal)
  (node-pools
    (masters
      (name "masters")
      (provider "hetzner")
      (region "nbg1")
      (count 1)
      (roles master etcd)
      (size "cx21")
      (labels
        (role "control-plane")))

    (workers
      (name "workers")
      (provider "hetzner")
      (region "nbg1")
      (count 1)
      (roles worker)
      (size "cx21")
      (labels
        (role "worker"))))

  (kubernetes
    (version "v1.29.0")
    (distribution "rke2")
    (network-plugin "canal")
    (cluster-domain "cluster.local"))

  (monitoring
    (enabled false)))
