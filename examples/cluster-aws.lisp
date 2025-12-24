; Sloth Kubernetes - AWS Cluster Configuration
; This is a complete example of an AWS cluster with WireGuard mesh VPN

(cluster
  ; Cluster metadata
  (metadata
    (name "sloth-aws-cluster")
    (environment "production")
    (description "Kubernetes cluster on AWS with HA control plane")
    (owner "chalkan3")
    (labels
      (project "sloth-kubernetes")
      (provider "aws")))

  ; Cluster specification
  (cluster
    (type "rke2")
    (version "v1.29.0+rke2r1")
    (high-availability true))

  ; Cloud providers
  (providers
    (aws
      (enabled true)
      (region "us-east-1")))

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
      (mesh-networking true)
      (auto-config true)))

  ; Security configuration
  (security
    (ssh
      (key-path "~/.ssh/id_rsa")
      (public-key-path "~/.ssh/id_rsa.pub")
      (port 22))
    (bastion
      (enabled true)
      (provider "aws")
      (region "us-east-1")
      (size "t3.micro")
      (name "sloth-bastion")
      (ssh-port 22)
      (allowed-cidrs "0.0.0.0/0")
      (enable-audit-log true)
      (idle-timeout 30)
      (max-sessions 10)))

  ; Node pools
  (node-pools
    (masters
      (name "aws-masters")
      (provider "aws")
      (region "us-east-1")
      (count 3)
      (roles master etcd)
      (size "t3.medium")
      (labels
        (role "control-plane")
        (tier "master")))
    (workers
      (name "aws-workers")
      (provider "aws")
      (region "us-east-1")
      (count 5)
      (roles worker)
      (size "t3.large")
      (spot-instance true)
      (labels
        (role "worker")
        (tier "application"))))

  ; Kubernetes configuration
  (kubernetes
    (version "v1.29.0")
    (distribution "rke2")
    (network-plugin "canal")
    (pod-cidr "10.42.0.0/16")
    (service-cidr "10.43.0.0/16")
    (cluster-domain "cluster.local")
    (rke2
      (version "v1.29.0+rke2r1")
      (channel "stable")
      (disable-components "rke2-ingress-nginx")
      (snapshot-schedule-cron "0 */6 * * *")
      (snapshot-retention 5)))

  ; Monitoring
  (monitoring
    (enabled true)
    (prometheus
      (enabled true)
      (retention "15d"))
    (grafana
      (enabled true))))
