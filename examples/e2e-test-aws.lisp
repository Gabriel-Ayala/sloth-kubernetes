; Sloth Kubernetes - E2E Test Configuration
; Minimal AWS cluster for end-to-end testing
; Uses Lisp built-in functions for dynamic configuration

(cluster
  ; Cluster metadata
  (metadata
    (name (concat "sloth-e2e-" (uuid-short)))
    (environment "testing")
    (description "E2E test cluster for sloth-kubernetes")
    (labels
      (test-run-id (uuid))
      (created-at (now))))

  ; Cloud providers
  (providers
    (aws
      (enabled true)
      (region (env "AWS_REGION" "us-east-1"))
      (vpc
        (create true)
        (cidr (env "VPC_CIDR" "10.100.0.0/16")))))

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

  ; Node pools - minimal for E2E testing
  (node-pools
    (masters
      (name (concat "e2e-masters-" (uuid-short)))
      (provider "aws")
      (region (env "AWS_REGION" "us-east-1"))
      (count (default (env "E2E_MASTER_COUNT") 1))
      (roles master etcd)
      (size (env "E2E_MASTER_SIZE" "t3.medium")))
    (workers
      (name (concat "e2e-workers-" (uuid-short)))
      (provider "aws")
      (region (env "AWS_REGION" "us-east-1"))
      (count (default (env "E2E_WORKER_COUNT") 2))
      (roles worker)
      (size (env "E2E_WORKER_SIZE" "t3.medium"))))

  ; Kubernetes configuration
  (kubernetes
    (distribution "rke2")
    (version (env "RKE2_VERSION" "v1.29.0+rke2r1")))

  ; Salt for node management
  ; Salt uses sharedsecret authentication for reliable API access
  (addons
    (salt
      (enabled (env "SALT_ENABLED" true))
      (master-node "0")
      (api-enabled true)
      (api-port (default (env "SALT_API_PORT") 8000))
      (api-username (env "SALT_API_USER" "saltapi"))
      ; api-password is auto-generated if not specified (recommended)
      ; (api-password (env "SALT_API_PASSWORD"))
      (secure-auth true)        ; Use hash-based minion authentication
      (auto-join true)          ; Automatically join nodes as minions
      (audit-logging true))))   ; Log authentication events
