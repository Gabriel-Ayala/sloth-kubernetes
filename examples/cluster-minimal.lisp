; Sloth Kubernetes - Minimal Cluster Configuration
; The simplest possible cluster configuration
; Uses Lisp built-in functions for dynamic configuration

(cluster
  (metadata
    (name (env "CLUSTER_NAME" "minimal-cluster"))
    (environment (env "CLUSTER_ENV" "development")))

  (providers
    (digitalocean
      (enabled true)
      (token (env "DIGITALOCEAN_TOKEN"))
      (region (env "DO_REGION" "nyc3"))))

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
      (count (default (env "MASTER_COUNT") 1))
      (roles master etcd)
      (size (env "DO_MASTER_SIZE" "s-2vcpu-4gb"))
      (region (env "DO_REGION" "nyc3")))
    (workers
      (name "workers")
      (provider "digitalocean")
      (count (default (env "WORKER_COUNT") 2))
      (roles worker)
      (size (env "DO_WORKER_SIZE" "s-2vcpu-4gb"))
      (region (env "DO_REGION" "nyc3"))))

  (kubernetes
    (distribution (env "K8S_DISTRIBUTION" "k3s"))
    (version (env "K8S_VERSION" "v1.29.0"))))
