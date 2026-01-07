package vpn

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// HealthStatus represents the health status of a node
type HealthStatus struct {
	Host      string        // Host IP or name
	Reachable bool          // TCP connectivity
	SSHReady  bool          // SSH service responding
	WGReady   bool          // WireGuard interface configured
	Latency   time.Duration // Round-trip latency
	LastCheck time.Time     // When this check was performed
	Error     error         // Any error encountered
	Details   string        // Additional details
}

// HealthChecker performs health checks on VPN nodes
type HealthChecker struct {
	timeout time.Duration
}

// NewHealthChecker creates a new HealthChecker with the specified timeout
func NewHealthChecker(timeout time.Duration) *HealthChecker {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &HealthChecker{
		timeout: timeout,
	}
}

// CheckTCP checks if a host:port is reachable via TCP
func (h *HealthChecker) CheckTCP(ctx context.Context, host string, port int) error {
	addr := fmt.Sprintf("%s:%d", host, port)

	dialer := &net.Dialer{
		Timeout: h.timeout,
	}

	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("TCP connection to %s failed: %w", addr, err)
	}
	conn.Close()

	return nil
}

// CheckBastion verifies that the bastion host is reachable on SSH port
func (h *HealthChecker) CheckBastion(ctx context.Context, host string) error {
	return h.CheckTCP(ctx, host, 22)
}

// CheckNode performs a comprehensive health check on a node via SSH connection
func (h *HealthChecker) CheckNode(ctx context.Context, conn *SSHConnection) (*HealthStatus, error) {
	status := &HealthStatus{
		Host:      conn.Host(),
		LastCheck: time.Now(),
	}

	// Check 1: SSH connectivity with a simple echo
	start := time.Now()
	output, err := conn.Execute("echo 'SSH_OK'")
	status.Latency = time.Since(start)

	if err != nil {
		status.Error = err
		status.Details = fmt.Sprintf("SSH check failed: %v", err)
		return status, err
	}

	status.SSHReady = strings.Contains(output, "SSH_OK")
	status.Reachable = status.SSHReady

	if !status.SSHReady {
		status.Error = fmt.Errorf("unexpected SSH response: %s", output)
		return status, status.Error
	}

	// Check 2: WireGuard interface status
	output, err = conn.Execute("wg show wg0 2>/dev/null && echo 'WG_OK' || echo 'WG_NOT_READY'")
	if err != nil {
		// WireGuard check is non-fatal - interface might not be set up yet
		status.WGReady = false
		status.Details = fmt.Sprintf("WireGuard not ready: %v", err)
	} else {
		status.WGReady = strings.Contains(output, "WG_OK")
		if !status.WGReady {
			status.Details = "WireGuard interface wg0 not configured"
		}
	}

	return status, nil
}

// CheckNodeWireGuardPeers checks if WireGuard has any peers configured
func (h *HealthChecker) CheckNodeWireGuardPeers(ctx context.Context, conn *SSHConnection) (int, error) {
	output, err := conn.Execute("wg show wg0 peers 2>/dev/null | wc -l")
	if err != nil {
		return 0, fmt.Errorf("failed to check WireGuard peers: %w", err)
	}

	var count int
	fmt.Sscanf(strings.TrimSpace(output), "%d", &count)
	return count, nil
}

// CheckNodeWireGuardEndpoint checks if WireGuard is listening on the expected port
func (h *HealthChecker) CheckNodeWireGuardEndpoint(ctx context.Context, conn *SSHConnection, port int) error {
	cmd := fmt.Sprintf("ss -uln | grep -q ':%d ' && echo 'WG_LISTENING' || echo 'WG_NOT_LISTENING'", port)
	output, err := conn.Execute(cmd)
	if err != nil {
		return fmt.Errorf("failed to check WireGuard port: %w", err)
	}

	if !strings.Contains(output, "WG_LISTENING") {
		return fmt.Errorf("WireGuard not listening on port %d", port)
	}

	return nil
}

// NodeInfo contains basic node information for health checks
type NodeInfo struct {
	Name     string
	PublicIP string
	VPNIP    string
	Provider string
}

// MultiNodeHealthResult contains health check results for multiple nodes
type MultiNodeHealthResult struct {
	TotalNodes   int
	HealthyNodes int
	Statuses     map[string]*HealthStatus
	Errors       []string
}

// CheckMultipleNodes checks health of multiple nodes concurrently
func (h *HealthChecker) CheckMultipleNodes(ctx context.Context, nodes []NodeInfo, connMgr *ConnectionManager, cfg ConnectionConfig) *MultiNodeHealthResult {
	result := &MultiNodeHealthResult{
		TotalNodes: len(nodes),
		Statuses:   make(map[string]*HealthStatus),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, node := range nodes {
		wg.Add(1)
		go func(n NodeInfo) {
			defer wg.Done()

			nodeCfg := cfg
			nodeCfg.Host = n.PublicIP

			conn, err := connMgr.Connect(ctx, nodeCfg)
			if err != nil {
				mu.Lock()
				result.Statuses[n.Name] = &HealthStatus{
					Host:      n.PublicIP,
					LastCheck: time.Now(),
					Error:     err,
				}
				result.Errors = append(result.Errors, fmt.Sprintf("%s: connection failed: %v", n.Name, err))
				mu.Unlock()
				return
			}
			defer conn.Close()

			status, err := h.CheckNode(ctx, conn)
			mu.Lock()
			result.Statuses[n.Name] = status
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: health check failed: %v", n.Name, err))
			} else if status.SSHReady {
				result.HealthyNodes++
			}
			mu.Unlock()
		}(node)
	}

	wg.Wait()
	return result
}

// WaitForReady waits until all nodes are ready or timeout
func (h *HealthChecker) WaitForReady(ctx context.Context, nodes []NodeInfo, connMgr *ConnectionManager, cfg ConnectionConfig) error {
	deadline := time.Now().Add(h.timeout)
	checkInterval := 5 * time.Second

	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for nodes to be ready")
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}

		result := h.CheckMultipleNodes(ctx, nodes, connMgr, cfg)
		if result.HealthyNodes == result.TotalNodes {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(checkInterval):
		}
	}
}

// WaitForNodeReady waits until a specific node is ready
func (h *HealthChecker) WaitForNodeReady(ctx context.Context, conn *SSHConnection, requireWG bool) error {
	deadline := time.Now().Add(h.timeout)
	checkInterval := 2 * time.Second

	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for node %s to be ready", conn.Host())
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}

		status, err := h.CheckNode(ctx, conn)
		if err == nil && status.SSHReady {
			if !requireWG || status.WGReady {
				return nil
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(checkInterval):
		}
	}
}

// PingVPNPeer checks if a VPN peer is reachable via WireGuard
func (h *HealthChecker) PingVPNPeer(ctx context.Context, conn *SSHConnection, vpnIP string) error {
	cmd := fmt.Sprintf("ping -c 1 -W 5 %s > /dev/null 2>&1 && echo 'PING_OK' || echo 'PING_FAIL'", vpnIP)
	output, err := conn.Execute(cmd)
	if err != nil {
		return fmt.Errorf("ping command failed: %w", err)
	}

	if !strings.Contains(output, "PING_OK") {
		return fmt.Errorf("VPN peer %s is not reachable", vpnIP)
	}

	return nil
}

// CheckVPNMesh checks connectivity between all VPN peers
func (h *HealthChecker) CheckVPNMesh(ctx context.Context, conn *SSHConnection, peerIPs []string) map[string]error {
	results := make(map[string]error)

	for _, ip := range peerIPs {
		results[ip] = h.PingVPNPeer(ctx, conn, ip)
	}

	return results
}
