; Sloth Kubernetes - Minimal Cluster Configuration
; The simplest possible cluster configuration

(cluster
  (metadata
    (name "minimal-cluster")
    (environment "development"))

  (providers
    (digitalocean
      (enabled true)
      (token "${DIGITALOCEAN_TOKEN}")
      (region "nyc3")))

  (network
    (mode "wireguard")
    (wireguard
      (enabled true)
      (create true)
      (mesh-networking true)))

  (node-pools
    (masters
      (name "masters")
      (provider "digitalocean")
      (count 1)
      (roles master etcd)
      (size "s-2vcpu-4gb")
      (region "nyc3"))
    (workers
      (name "workers")
      (provider "digitalocean")
      (count 2)
      (roles worker)
      (size "s-2vcpu-4gb")
      (region "nyc3")))

  (kubernetes
    (distribution "k3s")
    (version "v1.29.0")))
