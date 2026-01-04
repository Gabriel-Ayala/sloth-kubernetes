; Sloth Kubernetes - Scale Test Configuration (Initial)
; Use this to deploy initial cluster, then use scale-test-expanded.lisp to add nodes

(cluster
  (metadata
    (name "scale-test-cluster")
    (environment "testing")
    (description "Scale test cluster - initial deployment"))

  (providers
    (aws
      (enabled true)
      (region "us-east-1")
      (vpc
        (create true)
        (cidr "10.100.0.0/16"))))

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

  ; Initial deployment: 1 master + 1 worker
  (node-pools
    (masters
      (name "masters")
      (provider "aws")
      (region "us-east-1")
      (count 1)
      (roles master etcd)
      (size "t3.medium"))
    (workers
      (name "workers")
      (provider "aws")
      (region "us-east-1")
      (count 1)
      (roles worker)
      (size "t3.medium")))

  (kubernetes
    (distribution "rke2")
    (version "v1.29.0+rke2r1"))

  (addons
    (salt
      (enabled true)
      (master-node "0")
      (api-enabled true)
      (api-port 8000)
      (api-username "saltapi")
      (secure-auth true)
      (auto-join true))))
