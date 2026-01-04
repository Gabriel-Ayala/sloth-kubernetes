; Sloth Kubernetes - Scale Test Configuration (Expanded)
; Use this AFTER deploying with scale-test-initial.lisp to add more nodes
;
; Usage:
;   1. First deploy: ./sloth deploy --config examples/scale-test-initial.lisp --stack scale-test --yes
;   2. Then scale:   ./sloth deploy --config examples/scale-test-expanded.lisp --stack scale-test --yes
;
; This will add 2 new worker nodes to the existing cluster

(cluster
  (metadata
    (name "scale-test-cluster")
    (environment "testing")
    (description "Scale test cluster - SCALED with additional workers"))

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

  ; Scaled deployment: 1 master + 1 original worker + 2 NEW workers
  (node-pools
    (masters
      (name "masters")
      (provider "aws")
      (region "us-east-1")
      (count 1)
      (roles master etcd)
      (size "t3.medium"))
    ; Original worker pool - unchanged
    (workers
      (name "workers")
      (provider "aws")
      (region "us-east-1")
      (count 1)
      (roles worker)
      (size "t3.medium"))
    ; NEW: Additional worker pool for scaling
    (workers-scale
      (name "workers-scale")
      (provider "aws")
      (region "us-east-1")
      (count 2)
      (roles worker)
      (size "t3.small")))

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
