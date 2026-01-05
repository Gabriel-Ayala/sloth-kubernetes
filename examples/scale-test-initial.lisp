; Sloth Kubernetes - Scale Test Configuration (Initial)
; Use this to deploy initial cluster, then use scale-test-expanded.lisp to add nodes
; Uses Lisp built-in functions for dynamic configuration

(cluster
  (metadata
    (name (env "SCALE_TEST_NAME" "scale-test-cluster"))
    (environment "testing")
    (description "Scale test cluster - initial deployment")
    (labels
      (test-id (uuid-short))
      (created-at (now))))

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

  ; Initial deployment: 1 master + 1 worker
  (node-pools
    (masters
      (name "masters")
      (provider "aws")
      (region (env "AWS_REGION" "us-east-1"))
      (count (default (env "INITIAL_MASTER_COUNT") 1))
      (roles master etcd)
      (size (env "MASTER_SIZE" "t3.medium")))
    (workers
      (name "workers")
      (provider "aws")
      (region (env "AWS_REGION" "us-east-1"))
      (count (default (env "INITIAL_WORKER_COUNT") 1))
      (roles worker)
      (size (env "WORKER_SIZE" "t3.medium"))))

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
