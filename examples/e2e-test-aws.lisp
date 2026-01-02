; Sloth Kubernetes - E2E Test Configuration
; Minimal AWS cluster for end-to-end testing

(cluster
  ; Cluster metadata
  (metadata
    (name "sloth-e2e-test")
    (environment "testing")
    (description "E2E test cluster for sloth-kubernetes"))

  ; Cloud providers
  (providers
    (aws
      (enabled true)
      (region "us-east-1")
      (vpc
        (create true)
        (cidr "10.100.0.0/16"))))

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

  ; Node pools - minimal for E2E testing
  (node-pools
    (masters
      (name "e2e-masters")
      (provider "aws")
      (region "us-east-1")
      (count 1)
      (roles master etcd)
      (size "t3.medium"))
    (workers
      (name "e2e-workers")
      (provider "aws")
      (region "us-east-1")
      (count 2)
      (roles worker)
      (size "t3.medium")))

  ; Kubernetes configuration
  (kubernetes
    (distribution "rke2")
    (version "v1.29.0+rke2r1"))

  ; Salt for node management
  ; Salt uses sharedsecret authentication for reliable API access
  (addons
    (salt
      (enabled true)
      (master-node "0")
      (api-enabled true)
      (api-port 8000)
      (api-username "saltapi")  ; Default: saltapi
      ; api-password is auto-generated if not specified (recommended)
      ; (api-password "your-strong-password")
      (secure-auth true)        ; Use hash-based minion authentication
      (auto-join true)          ; Automatically join nodes as minions
      (audit-logging true))))   ; Log authentication events
