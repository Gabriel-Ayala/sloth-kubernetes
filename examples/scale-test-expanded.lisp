; Sloth Kubernetes - Scale Test Configuration (Expanded)
; Use this AFTER deploying with scale-test-initial.lisp to add more nodes
; Uses Lisp built-in functions for dynamic configuration
;
; Usage:
;   1. First deploy: ./sloth deploy --config examples/scale-test-initial.lisp --stack scale-test --yes
;   2. Then scale:   ./sloth deploy --config examples/scale-test-expanded.lisp --stack scale-test --yes
;
; This will add 2 new worker nodes to the existing cluster

(cluster
  (metadata
    (name (env "SCALE_TEST_NAME" "scale-test-cluster"))
    (environment "testing")
    (description "Scale test cluster - SCALED with additional workers")
    (labels
      (scaled-at (now))
      (scale-operation "expand")))

  (providers
    (aws
      (enabled true)
      (region (env "AWS_REGION" "us-east-1"))
      (vpc
        (create true)
        (cidr (env "VPC_CIDR" "10.100.0.0/16")))))

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

  ; Scaled deployment: 1 master + 1 original worker + 2 NEW workers
  (node-pools
    (masters
      (name "masters")
      (provider "aws")
      (region (env "AWS_REGION" "us-east-1"))
      (count (default (env "MASTER_COUNT") 1))
      (roles master etcd)
      (size (env "MASTER_SIZE" "t3.medium")))
    ; Original worker pool - unchanged
    (workers
      (name "workers")
      (provider "aws")
      (region (env "AWS_REGION" "us-east-1"))
      (count (default (env "WORKER_COUNT") 1))
      (roles worker)
      (size (env "WORKER_SIZE" "t3.medium")))
    ; NEW: Additional worker pool for scaling
    (workers-scale
      (name "workers-scale")
      (provider "aws")
      (region (env "AWS_REGION" "us-east-1"))
      (count (default (env "SCALE_WORKER_COUNT") 2))
      (roles worker)
      (size (env "SCALE_WORKER_SIZE" "t3.small"))))

  (kubernetes
    (distribution "rke2")
    (version (env "RKE2_VERSION" "v1.29.0+rke2r1")))

  (addons
    (salt
      (enabled (env "SALT_ENABLED" true))
      (master-node "0")
      (api-enabled true)
      (api-port (default (env "SALT_API_PORT") 8000))
      (api-username (env "SALT_API_USER" "saltapi"))
      (secure-auth true)
      (auto-join true))))
