package tailscale

import (
	"testing"

	"github.com/chalkan3/sloth-kubernetes/pkg/vpn"
)

func TestNewProvider(t *testing.T) {
	tests := []struct {
		name   string
		config interface{}
	}{
		{
			name:   "nil config uses defaults",
			config: nil,
		},
		{
			name: "custom config",
			config: &TailscaleConfig{
				HeadscaleURL: "https://headscale.example.com",
				AuthKey:      "test-auth-key",
				Namespace:    "test-namespace",
				Tags:         []string{"tag:test"},
				AcceptRoutes: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewProvider(tt.config)

			if provider == nil {
				t.Error("expected non-nil provider")
				return
			}

			if provider.Name() != "Tailscale (Headscale)" {
				t.Errorf("expected name 'Tailscale (Headscale)', got '%s'", provider.Name())
			}

			if provider.Type() != vpn.ProviderTailscale {
				t.Errorf("expected type ProviderTailscale, got '%s'", provider.Type())
			}

			if !provider.RequiresCoordinator() {
				t.Error("expected RequiresCoordinator to return true")
			}

			if provider.GetInterfaceName() != "tailscale0" {
				t.Errorf("expected interface name 'tailscale0', got '%s'", provider.GetInterfaceName())
			}
		})
	}
}

func TestDefaultTailscaleConfig(t *testing.T) {
	config := DefaultTailscaleConfig()

	if config == nil {
		t.Fatal("expected non-nil config")
	}

	if !config.AcceptRoutes {
		t.Error("expected AcceptRoutes to be true by default")
	}

	if config.Namespace != "default" {
		t.Errorf("expected Namespace 'default', got '%s'", config.Namespace)
	}
}

func TestProviderWithHeadscaleManager(t *testing.T) {
	config := &TailscaleConfig{
		HeadscaleURL: "https://headscale.example.com",
		APIKey:       "test-api-key",
		Namespace:    "kubernetes",
	}

	provider := NewProvider(config)

	if provider.GetHeadscaleManager() == nil {
		t.Error("expected HeadscaleManager to be initialized when APIKey is provided")
	}
}

func TestProviderWithoutHeadscaleManager(t *testing.T) {
	config := &TailscaleConfig{
		HeadscaleURL: "https://headscale.example.com",
		AuthKey:      "test-auth-key",
		// No APIKey
	}

	provider := NewProvider(config)

	if provider.GetHeadscaleManager() != nil {
		t.Error("expected HeadscaleManager to be nil when APIKey is not provided")
	}
}

func TestGenerateClientConfig(t *testing.T) {
	provider := NewProvider(&TailscaleConfig{
		HeadscaleURL: "https://headscale.example.com",
	})

	params := vpn.ClientConfigParams{
		CoordinatorURL: "https://headscale.example.com",
		AuthKey:        "test-auth-key",
		Hostname:       "test-client",
		Tags:           []string{"tag:test"},
		NodeEndpoints: []vpn.NodeEndpoint{
			{Name: "master-1", VPNIP: "100.64.0.1"},
			{Name: "worker-1", VPNIP: "100.64.0.2"},
		},
	}

	config, err := provider.GenerateClientConfig(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check for expected content
	expectedContent := []string{
		"Tailscale Join Instructions",
		"curl -fsSL https://tailscale.com/install.sh",
		"--login-server=https://headscale.example.com",
		"--authkey=test-auth-key",
		"--hostname=test-client",
		"--advertise-tags=tag:test",
		"tailscale status",
		"master-1: 100.64.0.1",
		"worker-1: 100.64.0.2",
	}

	for _, expected := range expectedContent {
		if !contains(config, expected) {
			t.Errorf("expected config to contain '%s'", expected)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
