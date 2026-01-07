package vpn

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// RegisteredPeer represents a peer registered in the VPN
type RegisteredPeer struct {
	PublicKey  string    `json:"publicKey"`
	VPNIP      string    `json:"vpnIP"`
	Label      string    `json:"label"`
	Machine    string    `json:"machine,omitempty"` // Hostname or identifier
	AddedAt    time.Time `json:"addedAt"`
	LastSeen   time.Time `json:"lastSeen"`
	Endpoint   string    `json:"endpoint,omitempty"` // Last known endpoint
	AllowedIPs []string  `json:"allowedIPs,omitempty"`
}

// PeerRegistry manages peer persistence
type PeerRegistry struct {
	dataDir string
	mu      sync.RWMutex
	peers   map[string]map[string]RegisteredPeer // stackName -> publicKey -> peer
}

// NewPeerRegistry creates a new PeerRegistry
func NewPeerRegistry(dataDir string) (*PeerRegistry, error) {
	if dataDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		dataDir = filepath.Join(homeDir, ".sloth-kubernetes", "vpn")
	}

	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	registry := &PeerRegistry{
		dataDir: dataDir,
		peers:   make(map[string]map[string]RegisteredPeer),
	}

	return registry, nil
}

// registryFilePath returns the path to the registry file for a stack
func (r *PeerRegistry) registryFilePath(stackName string) string {
	return filepath.Join(r.dataDir, fmt.Sprintf("%s-peers.json", stackName))
}

// loadStack loads peers for a specific stack from disk
func (r *PeerRegistry) loadStack(stackName string) error {
	filePath := r.registryFilePath(stackName)

	data, err := os.ReadFile(filePath)
	if os.IsNotExist(err) {
		r.peers[stackName] = make(map[string]RegisteredPeer)
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to read registry file: %w", err)
	}

	var peers []RegisteredPeer
	if err := json.Unmarshal(data, &peers); err != nil {
		return fmt.Errorf("failed to parse registry file: %w", err)
	}

	r.peers[stackName] = make(map[string]RegisteredPeer)
	for _, peer := range peers {
		r.peers[stackName][peer.PublicKey] = peer
	}

	return nil
}

// saveStack saves peers for a specific stack to disk
func (r *PeerRegistry) saveStack(stackName string) error {
	peers, ok := r.peers[stackName]
	if !ok {
		return nil
	}

	// Convert map to slice
	peerList := make([]RegisteredPeer, 0, len(peers))
	for _, peer := range peers {
		peerList = append(peerList, peer)
	}

	data, err := json.MarshalIndent(peerList, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal peers: %w", err)
	}

	filePath := r.registryFilePath(stackName)
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write registry file: %w", err)
	}

	return nil
}

// ensureStackLoaded makes sure the stack's peers are loaded
func (r *PeerRegistry) ensureStackLoaded(stackName string) error {
	if _, ok := r.peers[stackName]; !ok {
		return r.loadStack(stackName)
	}
	return nil
}

// Register adds or updates a peer in the registry
func (r *PeerRegistry) Register(stackName string, peer RegisteredPeer) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.ensureStackLoaded(stackName); err != nil {
		return err
	}

	// Set timestamps
	if peer.AddedAt.IsZero() {
		// Check if it's an update
		if existing, ok := r.peers[stackName][peer.PublicKey]; ok {
			peer.AddedAt = existing.AddedAt
		} else {
			peer.AddedAt = time.Now()
		}
	}
	peer.LastSeen = time.Now()

	r.peers[stackName][peer.PublicKey] = peer

	return r.saveStack(stackName)
}

// Unregister removes a peer from the registry
func (r *PeerRegistry) Unregister(stackName string, publicKey string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.ensureStackLoaded(stackName); err != nil {
		return err
	}

	delete(r.peers[stackName], publicKey)

	return r.saveStack(stackName)
}

// GetByPublicKey retrieves a peer by public key
func (r *PeerRegistry) GetByPublicKey(stackName string, publicKey string) (*RegisteredPeer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if err := r.ensureStackLoaded(stackName); err != nil {
		return nil, err
	}

	if peer, ok := r.peers[stackName][publicKey]; ok {
		return &peer, nil
	}

	return nil, fmt.Errorf("peer not found: %s", publicKey)
}

// GetByLabel retrieves a peer by label
func (r *PeerRegistry) GetByLabel(stackName string, label string) (*RegisteredPeer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if err := r.ensureStackLoaded(stackName); err != nil {
		return nil, err
	}

	for _, peer := range r.peers[stackName] {
		if peer.Label == label {
			return &peer, nil
		}
	}

	return nil, fmt.Errorf("peer with label '%s' not found", label)
}

// GetByIP retrieves a peer by VPN IP
func (r *PeerRegistry) GetByIP(stackName string, vpnIP string) (*RegisteredPeer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if err := r.ensureStackLoaded(stackName); err != nil {
		return nil, err
	}

	for _, peer := range r.peers[stackName] {
		if peer.VPNIP == vpnIP {
			return &peer, nil
		}
	}

	return nil, fmt.Errorf("peer with IP '%s' not found", vpnIP)
}

// GetByMachine retrieves a peer by machine name
func (r *PeerRegistry) GetByMachine(stackName string, machine string) (*RegisteredPeer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if err := r.ensureStackLoaded(stackName); err != nil {
		return nil, err
	}

	for _, peer := range r.peers[stackName] {
		if peer.Machine == machine {
			return &peer, nil
		}
	}

	return nil, fmt.Errorf("peer with machine '%s' not found", machine)
}

// List returns all registered peers for a stack
func (r *PeerRegistry) List(stackName string) ([]RegisteredPeer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if err := r.ensureStackLoaded(stackName); err != nil {
		return nil, err
	}

	peers := make([]RegisteredPeer, 0, len(r.peers[stackName]))
	for _, peer := range r.peers[stackName] {
		peers = append(peers, peer)
	}

	return peers, nil
}

// UpdateLastSeen updates the LastSeen timestamp for a peer
func (r *PeerRegistry) UpdateLastSeen(stackName string, publicKey string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.ensureStackLoaded(stackName); err != nil {
		return err
	}

	if peer, ok := r.peers[stackName][publicKey]; ok {
		peer.LastSeen = time.Now()
		r.peers[stackName][publicKey] = peer
		return r.saveStack(stackName)
	}

	return fmt.Errorf("peer not found: %s", publicKey)
}

// NextAvailableIP finds the next available VPN IP in the given subnet
func (r *PeerRegistry) NextAvailableIP(stackName string, subnetCIDR string, reservedIPs []string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if err := r.ensureStackLoaded(stackName); err != nil {
		return "", err
	}

	// Parse subnet
	_, ipnet, err := net.ParseCIDR(subnetCIDR)
	if err != nil {
		return "", fmt.Errorf("invalid subnet: %w", err)
	}

	// Collect used IPs
	usedIPs := make(map[string]bool)
	for _, ip := range reservedIPs {
		usedIPs[ip] = true
	}
	for _, peer := range r.peers[stackName] {
		usedIPs[peer.VPNIP] = true
	}

	// Find next available IP
	// Start from .2 (assuming .1 is the gateway)
	ip := incrementIP(ipnet.IP)
	ip = incrementIP(ip) // Skip network address and gateway

	for ipnet.Contains(ip) {
		ipStr := ip.String()
		if !usedIPs[ipStr] {
			return ipStr, nil
		}
		ip = incrementIP(ip)
	}

	return "", fmt.Errorf("no available IPs in subnet %s", subnetCIDR)
}

// Count returns the number of registered peers for a stack
func (r *PeerRegistry) Count(stackName string) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if err := r.ensureStackLoaded(stackName); err != nil {
		return 0, err
	}

	return len(r.peers[stackName]), nil
}

// Clear removes all peers for a stack
func (r *PeerRegistry) Clear(stackName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.peers[stackName] = make(map[string]RegisteredPeer)

	// Delete the file
	filePath := r.registryFilePath(stackName)
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove registry file: %w", err)
	}

	return nil
}

// incrementIP increments an IP address by 1
func incrementIP(ip net.IP) net.IP {
	// Make a copy to avoid modifying the original
	result := make(net.IP, len(ip))
	copy(result, ip)

	for i := len(result) - 1; i >= 0; i-- {
		result[i]++
		if result[i] != 0 {
			break
		}
	}

	return result
}

// Exists checks if a peer exists by any identifier
func (r *PeerRegistry) Exists(stackName string, identifier string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if err := r.ensureStackLoaded(stackName); err != nil {
		return false
	}

	// Check by public key
	if _, ok := r.peers[stackName][identifier]; ok {
		return true
	}

	// Check by label, IP, or machine
	for _, peer := range r.peers[stackName] {
		if peer.Label == identifier || peer.VPNIP == identifier || peer.Machine == identifier {
			return true
		}
	}

	return false
}
