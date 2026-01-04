package salt

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		username string
		password string
	}{
		{
			name:     "ValidClient",
			baseURL:  "https://salt-master.example.com:8000",
			username: "admin",
			password: "password123",
		},
		{
			name:     "EmptyCredentials",
			baseURL:  "https://localhost:8000",
			username: "",
			password: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.baseURL, tt.username, tt.password)

			assert.NotNil(t, client)
			assert.Equal(t, tt.baseURL, client.BaseURL)
			assert.Equal(t, tt.username, client.Username)
			assert.Equal(t, tt.password, client.Password)
			assert.NotNil(t, client.HTTPClient)
			assert.Empty(t, client.Token)
		})
	}
}

func TestClient_Login_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/login", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Decode request
		var loginReq LoginRequest
		err := json.NewDecoder(r.Body).Decode(&loginReq)
		require.NoError(t, err)

		assert.Equal(t, "testuser", loginReq.Username)
		assert.Equal(t, "testpass", loginReq.Password)
		assert.Equal(t, "sharedsecret", loginReq.Eauth)

		// Send response
		resp := LoginResponse{
			Return: []struct {
				Token  string   `json:"token"`
				Expire float64  `json:"expire"`
				Start  float64  `json:"start"`
				User   string   `json:"user"`
				Eauth  string   `json:"eauth"`
				Perms  []string `json:"perms"`
			}{
				{
					Token:  "test-token-12345",
					Expire: 3600,
					Start:  0,
					User:   "testuser",
					Eauth:  "pam",
					Perms:  []string{".*"},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "testuser", "testpass")
	err := client.Login()

	assert.NoError(t, err)
	assert.Equal(t, "test-token-12345", client.Token)
}

func TestClient_Login_InvalidCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("Invalid credentials"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "baduser", "badpass")
	err := client.Login()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "401")
	assert.Empty(t, client.Token)
}

func TestClient_Login_NoTokenReturned(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := LoginResponse{
			Return: []struct {
				Token  string   `json:"token"`
				Expire float64  `json:"expire"`
				Start  float64  `json:"start"`
				User   string   `json:"user"`
				Eauth  string   `json:"eauth"`
				Perms  []string `json:"perms"`
			}{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "testuser", "testpass")
	err := client.Login()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no token returned")
}

func TestClient_RunCommand_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			resp := LoginResponse{
				Return: []struct {
					Token  string   `json:"token"`
					Expire float64  `json:"expire"`
					Start  float64  `json:"start"`
					User   string   `json:"user"`
					Eauth  string   `json:"eauth"`
					Perms  []string `json:"perms"`
				}{
					{Token: "test-token"},
				},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}

		// Verify auth token
		assert.Equal(t, "test-token", r.Header.Get("X-Auth-Token"))

		// Decode command request
		var cmdReq CommandRequest
		err := json.NewDecoder(r.Body).Decode(&cmdReq)
		require.NoError(t, err)

		assert.Equal(t, "local", cmdReq.Client)
		assert.Equal(t, "*", cmdReq.Tgt)
		assert.Equal(t, "test.ping", cmdReq.Fun)

		// Send response
		resp := CommandResponse{
			Return: []map[string]interface{}{
				{
					"minion1": true,
					"minion2": true,
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test", "test")
	result, err := client.RunCommand("*", "test.ping", nil)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Return, 1)
}

func TestClient_RunCommand_AutoLogin(t *testing.T) {
	loginCalled := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			loginCalled = true
			resp := LoginResponse{
				Return: []struct {
					Token  string   `json:"token"`
					Expire float64  `json:"expire"`
					Start  float64  `json:"start"`
					User   string   `json:"user"`
					Eauth  string   `json:"eauth"`
					Perms  []string `json:"perms"`
				}{
					{Token: "auto-token"},
				},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}

		resp := CommandResponse{
			Return: []map[string]interface{}{{"result": "ok"}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test", "test")
	// Don't login manually, RunCommand should auto-login
	result, err := client.RunCommand("*", "test.cmd", nil)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, loginCalled, "Expected auto-login to be called")
	assert.Equal(t, "auto-token", client.Token)
}

func TestClient_RunCommand_TokenExpiredRetry(t *testing.T) {
	requestCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			resp := LoginResponse{
				Return: []struct {
					Token  string   `json:"token"`
					Expire float64  `json:"expire"`
					Start  float64  `json:"start"`
					User   string   `json:"user"`
					Eauth  string   `json:"eauth"`
					Perms  []string `json:"perms"`
				}{
					{Token: "new-token"},
				},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}

		requestCount++
		if requestCount == 1 {
			// First request - return 401 (token expired)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Second request - success
		resp := CommandResponse{
			Return: []map[string]interface{}{{"result": "ok"}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test", "test")
	client.Token = "expired-token"
	result, err := client.RunCommand("*", "test.cmd", nil)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 2, requestCount, "Expected retry after 401")
	assert.Equal(t, "new-token", client.Token)
}

func TestClient_Ping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			resp := LoginResponse{
				Return: []struct {
					Token  string   `json:"token"`
					Expire float64  `json:"expire"`
					Start  float64  `json:"start"`
					User   string   `json:"user"`
					Eauth  string   `json:"eauth"`
					Perms  []string `json:"perms"`
				}{
					{Token: "token"},
				},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}

		resp := CommandResponse{
			Return: []map[string]interface{}{
				{
					"minion1": true,
					"minion2": false,
					"minion3": true,
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test", "test")
	results, err := client.Ping("*")

	assert.NoError(t, err)
	assert.Len(t, results, 3)
	assert.True(t, results["minion1"])
	assert.False(t, results["minion2"])
	assert.True(t, results["minion3"])
}

func TestClient_GetMinions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			resp := LoginResponse{
				Return: []struct {
					Token  string   `json:"token"`
					Expire float64  `json:"expire"`
					Start  float64  `json:"start"`
					User   string   `json:"user"`
					Eauth  string   `json:"eauth"`
					Perms  []string `json:"perms"`
				}{
					{Token: "token"},
				},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}

		resp := CommandResponse{
			Return: []map[string]interface{}{
				{
					"web01": true,
					"web02": true,
					"db01":  true,
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test", "test")
	minions, err := client.GetMinions()

	assert.NoError(t, err)
	assert.Len(t, minions, 3)
	assert.Contains(t, minions, "web01")
	assert.Contains(t, minions, "web02")
	assert.Contains(t, minions, "db01")
}

// Test wrapper methods (they all call RunCommand)
func TestClient_WrapperMethods(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			resp := LoginResponse{
				Return: []struct {
					Token  string   `json:"token"`
					Expire float64  `json:"expire"`
					Start  float64  `json:"start"`
					User   string   `json:"user"`
					Eauth  string   `json:"eauth"`
					Perms  []string `json:"perms"`
				}{
					{Token: "token"},
				},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}

		// Decode to verify correct function is called
		var cmdReq CommandRequest
		json.NewDecoder(r.Body).Decode(&cmdReq)

		resp := CommandResponse{
			Return: []map[string]interface{}{
				{"result": cmdReq.Fun}, // Echo the function name
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test", "test")

	tests := []struct {
		name         string
		fn           func() (*CommandResponse, error)
		expectedFunc string
	}{
		{"RunShellCommand", func() (*CommandResponse, error) {
			return client.RunShellCommand("*", "uptime")
		}, "cmd.run"},
		{"GetGrains", func() (*CommandResponse, error) {
			return client.GetGrains("*")
		}, "grains.items"},
		{"ApplyState", func() (*CommandResponse, error) {
			return client.ApplyState("*", "webserver")
		}, "state.apply"},
		{"HighState", func() (*CommandResponse, error) {
			return client.HighState("*")
		}, "state.highstate"},
		{"ServiceStart", func() (*CommandResponse, error) {
			return client.ServiceStart("*", "nginx")
		}, "service.start"},
		{"ServiceStop", func() (*CommandResponse, error) {
			return client.ServiceStop("*", "nginx")
		}, "service.stop"},
		{"ServiceRestart", func() (*CommandResponse, error) {
			return client.ServiceRestart("*", "nginx")
		}, "service.restart"},
		{"ServiceStatus", func() (*CommandResponse, error) {
			return client.ServiceStatus("*", "nginx")
		}, "service.status"},
		{"PackageInstall", func() (*CommandResponse, error) {
			return client.PackageInstall("*", "vim")
		}, "pkg.install"},
		{"PackageRemove", func() (*CommandResponse, error) {
			return client.PackageRemove("*", "vim")
		}, "pkg.remove"},
		{"PackageList", func() (*CommandResponse, error) {
			return client.PackageList("*")
		}, "pkg.list_pkgs"},
		{"SystemReboot", func() (*CommandResponse, error) {
			return client.SystemReboot("*")
		}, "system.reboot"},
		{"DiskUsage", func() (*CommandResponse, error) {
			return client.DiskUsage("*")
		}, "disk.usage"},
		{"FileExists", func() (*CommandResponse, error) {
			return client.FileExists("*", "/etc/passwd")
		}, "file.file_exists"},
		{"UserAdd", func() (*CommandResponse, error) {
			return client.UserAdd("*", "newuser")
		}, "user.add"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.fn()
			assert.NoError(t, err)
			assert.NotNil(t, result)
			if len(result.Return) > 0 {
				assert.Equal(t, tt.expectedFunc, result.Return[0]["result"])
			}
		})
	}
}

func TestClient_PackageUpgrade(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			resp := LoginResponse{
				Return: []struct {
					Token  string   `json:"token"`
					Expire float64  `json:"expire"`
					Start  float64  `json:"start"`
					User   string   `json:"user"`
					Eauth  string   `json:"eauth"`
					Perms  []string `json:"perms"`
				}{
					{Token: "token"},
				},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}

		var cmdReq CommandRequest
		json.NewDecoder(r.Body).Decode(&cmdReq)

		resp := CommandResponse{
			Return: []map[string]interface{}{
				{"packages": cmdReq.Arg},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test", "test")

	t.Run("UpgradeAll", func(t *testing.T) {
		result, err := client.PackageUpgrade("*")
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("UpgradeSpecific", func(t *testing.T) {
		result, err := client.PackageUpgrade("*", "vim", "git")
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})
}

func TestClient_KeyAccept(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			resp := LoginResponse{
				Return: []struct {
					Token  string   `json:"token"`
					Expire float64  `json:"expire"`
					Start  float64  `json:"start"`
					User   string   `json:"user"`
					Eauth  string   `json:"eauth"`
					Perms  []string `json:"perms"`
				}{
					{Token: "token"},
				},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}

		var cmdReq map[string]interface{}
		json.NewDecoder(r.Body).Decode(&cmdReq)

		assert.Equal(t, "wheel", cmdReq["client"])
		assert.Equal(t, "key.accept", cmdReq["fun"])
		assert.Equal(t, "new-minion", cmdReq["match"])

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test", "test")
	err := client.KeyAccept("new-minion")

	assert.NoError(t, err)
}

func TestClient_KeyList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			resp := LoginResponse{
				Return: []struct {
					Token  string   `json:"token"`
					Expire float64  `json:"expire"`
					Start  float64  `json:"start"`
					User   string   `json:"user"`
					Eauth  string   `json:"eauth"`
					Perms  []string `json:"perms"`
				}{
					{Token: "token"},
				},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}

		result := struct {
			Return []struct {
				Data map[string]interface{} `json:"data"`
			} `json:"return"`
		}{
			Return: []struct {
				Data map[string]interface{} `json:"data"`
			}{
				{
					Data: map[string]interface{}{
						"minions":          []interface{}{"minion1", "minion2"},
						"minions_pre":      []interface{}{"pending-minion"},
						"minions_rejected": []interface{}{},
					},
				},
			},
		}

		json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test", "test")
	keys, err := client.KeyList()

	assert.NoError(t, err)
	assert.Contains(t, keys, "minions")
	assert.Contains(t, keys, "minions_pre")
	assert.Len(t, keys["minions"], 2)
	assert.Len(t, keys["minions_pre"], 1)
}

func TestClient_NetworkPing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			resp := LoginResponse{
				Return: []struct {
					Token  string   `json:"token"`
					Expire float64  `json:"expire"`
					Start  float64  `json:"start"`
					User   string   `json:"user"`
					Eauth  string   `json:"eauth"`
					Perms  []string `json:"perms"`
				}{
					{Token: "token"},
				},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}

		var cmdReq CommandRequest
		json.NewDecoder(r.Body).Decode(&cmdReq)

		assert.Equal(t, "network.ping", cmdReq.Fun)
		assert.Equal(t, "8.8.8.8", cmdReq.Arg[0])
		assert.Equal(t, "4", cmdReq.Arg[1])

		resp := CommandResponse{
			Return: []map[string]interface{}{{"result": "ok"}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test", "test")
	result, err := client.NetworkPing("*", "8.8.8.8", 4)

	assert.NoError(t, err)
	assert.NotNil(t, result)
}
