package config

import (
	"fmt"
	"strings"
)

// GenerateLisp converts a ClusterConfig back to Lisp S-expression format
// This enables regenerating the config file from Pulumi state
func GenerateLisp(cfg *ClusterConfig) string {
	var sb strings.Builder

	sb.WriteString("; Sloth Kubernetes - Generated Configuration\n")
	sb.WriteString("; Regenerated from Pulumi state\n")
	sb.WriteString(";\n")
	sb.WriteString("; WARNING: Sensitive values are replaced with environment variable placeholders.\n")
	sb.WriteString("; Set the appropriate environment variables before deploying.\n\n")

	sb.WriteString("(cluster\n")

	// Metadata section
	writeMetadata(&sb, &cfg.Metadata)

	// Providers section
	writeProviders(&sb, &cfg.Providers)

	// Network section
	writeNetwork(&sb, &cfg.Network)

	// Security section (if configured)
	if cfg.Security.Bastion != nil && cfg.Security.Bastion.Enabled {
		writeSecurity(&sb, &cfg.Security)
	}

	// Node pools section
	writeNodePools(&sb, cfg.NodePools)

	// Kubernetes section
	writeKubernetes(&sb, &cfg.Kubernetes)

	// Addons section
	writeAddons(&sb, &cfg.Addons)

	sb.WriteString(")\n")

	return sb.String()
}

func writeMetadata(sb *strings.Builder, m *Metadata) {
	sb.WriteString("  (metadata\n")
	sb.WriteString(fmt.Sprintf("    (name %q)\n", m.Name))
	if m.Environment != "" {
		sb.WriteString(fmt.Sprintf("    (environment %q)\n", m.Environment))
	}
	if m.Description != "" {
		sb.WriteString(fmt.Sprintf("    (description %q)\n", m.Description))
	}
	if m.Version != "" {
		sb.WriteString(fmt.Sprintf("    (version %q)\n", m.Version))
	}
	if m.Owner != "" {
		sb.WriteString(fmt.Sprintf("    (owner %q)\n", m.Owner))
	}
	if m.Team != "" {
		sb.WriteString(fmt.Sprintf("    (team %q)\n", m.Team))
	}
	sb.WriteString("  )\n\n")
}

func writeProviders(sb *strings.Builder, p *ProvidersConfig) {
	sb.WriteString("  (providers\n")

	if p.DigitalOcean != nil && p.DigitalOcean.Enabled {
		sb.WriteString("    (digitalocean\n")
		sb.WriteString("      (enabled true)\n")
		sb.WriteString(fmt.Sprintf("      (token %q)\n", p.DigitalOcean.Token))
		if p.DigitalOcean.Region != "" {
			sb.WriteString(fmt.Sprintf("      (region %q)\n", p.DigitalOcean.Region))
		}
		if p.DigitalOcean.VPC != nil && p.DigitalOcean.VPC.Create {
			sb.WriteString("      (vpc\n")
			sb.WriteString("        (create true)\n")
			if p.DigitalOcean.VPC.CIDR != "" {
				sb.WriteString(fmt.Sprintf("        (cidr %q)\n", p.DigitalOcean.VPC.CIDR))
			}
			sb.WriteString("      )\n")
		}
		sb.WriteString("    )\n")
	}

	if p.Linode != nil && p.Linode.Enabled {
		sb.WriteString("    (linode\n")
		sb.WriteString("      (enabled true)\n")
		sb.WriteString(fmt.Sprintf("      (token %q)\n", p.Linode.Token))
		if p.Linode.Region != "" {
			sb.WriteString(fmt.Sprintf("      (region %q)\n", p.Linode.Region))
		}
		if p.Linode.VPC != nil && p.Linode.VPC.Create {
			sb.WriteString("      (vpc\n")
			sb.WriteString("        (create true)\n")
			if p.Linode.VPC.CIDR != "" {
				sb.WriteString(fmt.Sprintf("        (cidr %q)\n", p.Linode.VPC.CIDR))
			}
			sb.WriteString("      )\n")
		}
		sb.WriteString("    )\n")
	}

	if p.AWS != nil && p.AWS.Enabled {
		sb.WriteString("    (aws\n")
		sb.WriteString("      (enabled true)\n")
		if p.AWS.Region != "" {
			sb.WriteString(fmt.Sprintf("      (region %q)\n", p.AWS.Region))
		}
		if p.AWS.VPC != nil && p.AWS.VPC.Create {
			sb.WriteString("      (vpc\n")
			sb.WriteString("        (create true)\n")
			if p.AWS.VPC.CIDR != "" {
				sb.WriteString(fmt.Sprintf("        (cidr %q)\n", p.AWS.VPC.CIDR))
			}
			sb.WriteString("      )\n")
		}
		sb.WriteString("    )\n")
	}

	if p.Azure != nil && p.Azure.Enabled {
		sb.WriteString("    (azure\n")
		sb.WriteString("      (enabled true)\n")
		sb.WriteString(fmt.Sprintf("      (subscription-id %q)\n", p.Azure.SubscriptionID))
		sb.WriteString(fmt.Sprintf("      (tenant-id %q)\n", p.Azure.TenantID))
		sb.WriteString(fmt.Sprintf("      (client-id %q)\n", p.Azure.ClientID))
		sb.WriteString(fmt.Sprintf("      (client-secret %q)\n", p.Azure.ClientSecret))
		if p.Azure.Location != "" {
			sb.WriteString(fmt.Sprintf("      (location %q)\n", p.Azure.Location))
		}
		sb.WriteString("    )\n")
	}

	sb.WriteString("  )\n\n")
}

func writeNetwork(sb *strings.Builder, n *NetworkConfig) {
	sb.WriteString("  (network\n")

	if n.Mode != "" {
		sb.WriteString(fmt.Sprintf("    (mode %q)\n", n.Mode))
	}
	if n.PodCIDR != "" {
		sb.WriteString(fmt.Sprintf("    (pod-cidr %q)\n", n.PodCIDR))
	}
	if n.ServiceCIDR != "" {
		sb.WriteString(fmt.Sprintf("    (service-cidr %q)\n", n.ServiceCIDR))
	}

	// WireGuard configuration
	if n.WireGuard != nil && n.WireGuard.Enabled {
		sb.WriteString("    (wireguard\n")
		sb.WriteString("      (enabled true)\n")
		if n.WireGuard.Create {
			sb.WriteString("      (create true)\n")
		}
		if n.WireGuard.Provider != "" {
			sb.WriteString(fmt.Sprintf("      (provider %q)\n", n.WireGuard.Provider))
		}
		if n.WireGuard.Region != "" {
			sb.WriteString(fmt.Sprintf("      (region %q)\n", n.WireGuard.Region))
		}
		if n.WireGuard.SubnetCIDR != "" {
			sb.WriteString(fmt.Sprintf("      (subnet-cidr %q)\n", n.WireGuard.SubnetCIDR))
		}
		if n.WireGuard.Port != 0 {
			sb.WriteString(fmt.Sprintf("      (port %d)\n", n.WireGuard.Port))
		}
		sb.WriteString(fmt.Sprintf("      (mesh-networking %t)\n", n.WireGuard.MeshNetworking))
		sb.WriteString("    )\n")
	}

	// DNS configuration
	if n.DNS.Domain != "" || n.DNS.Provider != "" {
		sb.WriteString("    (dns\n")
		if n.DNS.Domain != "" {
			sb.WriteString(fmt.Sprintf("      (domain %q)\n", n.DNS.Domain))
		}
		if n.DNS.Provider != "" {
			sb.WriteString(fmt.Sprintf("      (provider %q)\n", n.DNS.Provider))
		}
		sb.WriteString("    )\n")
	}

	sb.WriteString("  )\n\n")
}

func writeSecurity(sb *strings.Builder, s *SecurityConfig) {
	sb.WriteString("  (security\n")

	if s.Bastion != nil && s.Bastion.Enabled {
		sb.WriteString("    (bastion\n")
		sb.WriteString("      (enabled true)\n")
		if s.Bastion.Provider != "" {
			sb.WriteString(fmt.Sprintf("      (provider %q)\n", s.Bastion.Provider))
		}
		if s.Bastion.Region != "" {
			sb.WriteString(fmt.Sprintf("      (region %q)\n", s.Bastion.Region))
		}
		if s.Bastion.Size != "" {
			sb.WriteString(fmt.Sprintf("      (size %q)\n", s.Bastion.Size))
		}
		if s.Bastion.Name != "" {
			sb.WriteString(fmt.Sprintf("      (name %q)\n", s.Bastion.Name))
		}
		sb.WriteString("    )\n")
	}

	sb.WriteString("  )\n\n")
}

func writeNodePools(sb *strings.Builder, pools map[string]NodePool) {
	if len(pools) == 0 {
		return
	}

	sb.WriteString("  (node-pools\n")

	for name, pool := range pools {
		sb.WriteString(fmt.Sprintf("    (%s\n", name))
		sb.WriteString(fmt.Sprintf("      (name %q)\n", pool.Name))
		if pool.Provider != "" {
			sb.WriteString(fmt.Sprintf("      (provider %q)\n", pool.Provider))
		}
		if pool.Region != "" {
			sb.WriteString(fmt.Sprintf("      (region %q)\n", pool.Region))
		}
		sb.WriteString(fmt.Sprintf("      (count %d)\n", pool.Count))

		// Roles as space-separated atoms
		if len(pool.Roles) > 0 {
			sb.WriteString("      (roles")
			for _, role := range pool.Roles {
				sb.WriteString(fmt.Sprintf(" %s", role))
			}
			sb.WriteString(")\n")
		}

		if pool.Size != "" {
			sb.WriteString(fmt.Sprintf("      (size %q)\n", pool.Size))
		}
		if pool.Image != "" {
			sb.WriteString(fmt.Sprintf("      (image %q)\n", pool.Image))
		}

		// Spot instance configuration
		if pool.SpotInstance {
			sb.WriteString("      (spot-instance true)\n")
			if pool.SpotConfig != nil && pool.SpotConfig.MaxPrice != "" {
				sb.WriteString(fmt.Sprintf("      (spot-max-price %q)\n", pool.SpotConfig.MaxPrice))
			}
		}

		// Labels
		if len(pool.Labels) > 0 {
			sb.WriteString("      (labels\n")
			for k, v := range pool.Labels {
				sb.WriteString(fmt.Sprintf("        (%s %q)\n", k, v))
			}
			sb.WriteString("      )\n")
		}

		// Taints
		if len(pool.Taints) > 0 {
			sb.WriteString("      (taints\n")
			for _, taint := range pool.Taints {
				sb.WriteString(fmt.Sprintf("        (%s %q %s)\n", taint.Key, taint.Value, taint.Effect))
			}
			sb.WriteString("      )\n")
		}

		sb.WriteString("    )\n")
	}

	sb.WriteString("  )\n\n")
}

func writeKubernetes(sb *strings.Builder, k *KubernetesConfig) {
	sb.WriteString("  (kubernetes\n")

	if k.Distribution != "" {
		sb.WriteString(fmt.Sprintf("    (distribution %q)\n", k.Distribution))
	}
	if k.Version != "" {
		sb.WriteString(fmt.Sprintf("    (version %q)\n", k.Version))
	}
	if k.NetworkPlugin != "" {
		sb.WriteString(fmt.Sprintf("    (network-plugin %q)\n", k.NetworkPlugin))
	}
	if k.PodCIDR != "" {
		sb.WriteString(fmt.Sprintf("    (pod-cidr %q)\n", k.PodCIDR))
	}
	if k.ServiceCIDR != "" {
		sb.WriteString(fmt.Sprintf("    (service-cidr %q)\n", k.ServiceCIDR))
	}

	sb.WriteString("  )\n\n")
}

func writeAddons(sb *strings.Builder, a *AddonsConfig) {
	hasAddons := false

	// Check if any addon is enabled
	if a.Salt != nil && a.Salt.Enabled {
		hasAddons = true
	}
	if a.ArgoCD != nil && a.ArgoCD.Enabled {
		hasAddons = true
	}

	if !hasAddons {
		return
	}

	sb.WriteString("  (addons\n")

	// Salt configuration
	if a.Salt != nil && a.Salt.Enabled {
		sb.WriteString("    (salt\n")
		sb.WriteString("      (enabled true)\n")
		if a.Salt.MasterNode != "" {
			sb.WriteString(fmt.Sprintf("      (master-node %q)\n", a.Salt.MasterNode))
		}
		if a.Salt.APIEnabled {
			sb.WriteString("      (api-enabled true)\n")
		}
		if a.Salt.APIPort != 0 {
			sb.WriteString(fmt.Sprintf("      (api-port %d)\n", a.Salt.APIPort))
		}
		if a.Salt.APIUsername != "" {
			sb.WriteString(fmt.Sprintf("      (api-username %q)\n", a.Salt.APIUsername))
		}
		if a.Salt.SecureAuth {
			sb.WriteString("      (secure-auth true)\n")
		}
		if a.Salt.AutoJoin {
			sb.WriteString("      (auto-join true)\n")
		}
		sb.WriteString("    )\n")
	}

	// ArgoCD configuration
	if a.ArgoCD != nil && a.ArgoCD.Enabled {
		sb.WriteString("    (argocd\n")
		sb.WriteString("      (enabled true)\n")
		if a.ArgoCD.Namespace != "" {
			sb.WriteString(fmt.Sprintf("      (namespace %q)\n", a.ArgoCD.Namespace))
		}
		if a.ArgoCD.Version != "" {
			sb.WriteString(fmt.Sprintf("      (version %q)\n", a.ArgoCD.Version))
		}
		sb.WriteString("    )\n")
	}

	sb.WriteString("  )\n")
}
