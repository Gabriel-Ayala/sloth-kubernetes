package tailscale

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewHeadscaleManager(t *testing.T) {
	config := HeadscaleConfig{
		APIURL:    "https://headscale.example.com",
		APIKey:    "test-api-key",
		Namespace: "kubernetes",
	}

	manager := NewHeadscaleManager(config)

	if manager == nil {
		t.Fatal("expected non-nil manager")
	}

	if manager.config.APIURL != config.APIURL {
		t.Errorf("expected APIURL '%s', got '%s'", config.APIURL, manager.config.APIURL)
	}

	if manager.config.APIKey != config.APIKey {
		t.Errorf("expected APIKey '%s', got '%s'", config.APIKey, manager.config.APIKey)
	}

	if manager.config.Namespace != config.Namespace {
		t.Errorf("expected Namespace '%s', got '%s'", config.Namespace, manager.config.Namespace)
	}
}

func TestGetConfig(t *testing.T) {
	config := HeadscaleConfig{
		APIURL:    "https://headscale.example.com",
		APIKey:    "test-api-key",
		Namespace: "kubernetes",
	}

	manager := NewHeadscaleManager(config)
	gotConfig := manager.GetConfig()

	if gotConfig.APIURL != config.APIURL {
		t.Errorf("expected APIURL '%s', got '%s'", config.APIURL, gotConfig.APIURL)
	}
}

func TestSetNamespace(t *testing.T) {
	manager := NewHeadscaleManager(HeadscaleConfig{
		APIURL:    "https://headscale.example.com",
		APIKey:    "test-api-key",
		Namespace: "default",
	})

	manager.SetNamespace("production")

	if manager.config.Namespace != "production" {
		t.Errorf("expected Namespace 'production', got '%s'", manager.config.Namespace)
	}
}

func TestListNodes(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET request, got %s", r.Method)
		}

		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			t.Errorf("expected Authorization header 'Bearer test-api-key'")
		}

		response := listNodesResponse{
			Nodes: []HeadscaleNode{
				{
					ID:          "1",
					Name:        "master-1",
					GivenName:   "master-1",
					IPAddresses: []string{"100.64.0.1"},
					Namespace:   "kubernetes",
					Online:      true,
				},
				{
					ID:          "2",
					Name:        "worker-1",
					GivenName:   "worker-1",
					IPAddresses: []string{"100.64.0.2"},
					Namespace:   "kubernetes",
					Online:      true,
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	manager := NewHeadscaleManager(HeadscaleConfig{
		APIURL:    server.URL,
		APIKey:    "test-api-key",
		Namespace: "kubernetes",
	})

	ctx := context.Background()
	nodes, err := manager.ListNodes(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(nodes))
	}

	if nodes[0].Name != "master-1" {
		t.Errorf("expected first node name 'master-1', got '%s'", nodes[0].Name)
	}
}

func TestListNamespaces(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/user" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		response := listNamespacesResponse{
			Users: []HeadscaleNamespace{
				{ID: "1", Name: "default", CreatedAt: time.Now()},
				{ID: "2", Name: "kubernetes", CreatedAt: time.Now()},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	manager := NewHeadscaleManager(HeadscaleConfig{
		APIURL:    server.URL,
		APIKey:    "test-api-key",
		Namespace: "kubernetes",
	})

	ctx := context.Background()
	namespaces, err := manager.ListNamespaces(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(namespaces) != 2 {
		t.Errorf("expected 2 namespaces, got %d", len(namespaces))
	}
}

func TestGetHealthStatus(t *testing.T) {
	tests := []struct {
		name         string
		statusCode   int
		expectHealth bool
	}{
		{
			name:         "healthy server",
			statusCode:   http.StatusOK,
			expectHealth: true,
		},
		{
			name:         "unhealthy server",
			statusCode:   http.StatusServiceUnavailable,
			expectHealth: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/health" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			manager := NewHeadscaleManager(HeadscaleConfig{
				APIURL: server.URL,
			})

			ctx := context.Background()
			healthy, err := manager.GetHealthStatus(ctx)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if healthy != tt.expectHealth {
				t.Errorf("expected health %v, got %v", tt.expectHealth, healthy)
			}
		})
	}
}

func TestAuthKeyOptions(t *testing.T) {
	opts := AuthKeyOptions{
		Reusable:   true,
		Ephemeral:  false,
		Expiration: 24 * time.Hour,
		Tags:       []string{"tag:test"},
	}

	if !opts.Reusable {
		t.Error("expected Reusable to be true")
	}

	if opts.Ephemeral {
		t.Error("expected Ephemeral to be false")
	}

	if opts.Expiration != 24*time.Hour {
		t.Errorf("expected Expiration 24h, got %v", opts.Expiration)
	}

	if len(opts.Tags) != 1 || opts.Tags[0] != "tag:test" {
		t.Errorf("expected Tags ['tag:test'], got %v", opts.Tags)
	}
}

func TestDeleteNode(t *testing.T) {
	deleteCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("expected DELETE request, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/node/123" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		deleteCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	manager := NewHeadscaleManager(HeadscaleConfig{
		APIURL:    server.URL,
		APIKey:    "test-api-key",
		Namespace: "kubernetes",
	})

	ctx := context.Background()
	err := manager.DeleteNode(ctx, "123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !deleteCalled {
		t.Error("expected DELETE to be called")
	}
}

func TestRenameNode(t *testing.T) {
	renameCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST request, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/node/123/rename/new-name" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		renameCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	manager := NewHeadscaleManager(HeadscaleConfig{
		APIURL:    server.URL,
		APIKey:    "test-api-key",
		Namespace: "kubernetes",
	})

	ctx := context.Background()
	err := manager.RenameNode(ctx, "123", "new-name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !renameCalled {
		t.Error("expected rename to be called")
	}
}
