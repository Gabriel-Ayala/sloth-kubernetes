// Package taints provides Kubernetes taint management
package taints

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/chalkan3/sloth-kubernetes/pkg/provisioning"
)

// =============================================================================
// Taint Manager
// =============================================================================

// Manager manages Kubernetes node taints
type Manager struct {
	kubeClient   KubeClient
	eventEmitter provisioning.EventEmitter
	mu           sync.RWMutex
}

// KubeClient interface for Kubernetes operations
type KubeClient interface {
	// ApplyTaint applies a taint to a node
	ApplyTaint(ctx context.Context, nodeName string, taint *Taint) error
	// RemoveTaint removes a taint from a node
	RemoveTaint(ctx context.Context, nodeName string, taintKey string) error
	// GetNodeTaints gets current taints on a node
	GetNodeTaints(ctx context.Context, nodeName string) ([]*Taint, error)
	// ExecuteCommand executes a kubectl command
	ExecuteCommand(ctx context.Context, args ...string) (string, error)
}

// Taint represents a Kubernetes taint
type Taint struct {
	Key    string
	Value  string
	Effect TaintEffect
}

// TaintEffect represents the effect of a taint
type TaintEffect string

const (
	TaintEffectNoSchedule       TaintEffect = "NoSchedule"
	TaintEffectPreferNoSchedule TaintEffect = "PreferNoSchedule"
	TaintEffectNoExecute        TaintEffect = "NoExecute"
)

// ManagerConfig holds manager configuration
type ManagerConfig struct {
	KubeClient   KubeClient
	EventEmitter provisioning.EventEmitter
}

// NewManager creates a new taint manager
func NewManager(cfg *ManagerConfig) *Manager {
	return &Manager{
		kubeClient:   cfg.KubeClient,
		eventEmitter: cfg.EventEmitter,
	}
}

// ApplyTaints applies taints to a node
func (m *Manager) ApplyTaints(ctx context.Context, nodeName string, taints []config.TaintConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, taintCfg := range taints {
		taint := &Taint{
			Key:    taintCfg.Key,
			Value:  taintCfg.Value,
			Effect: ParseTaintEffect(taintCfg.Effect),
		}

		if err := m.kubeClient.ApplyTaint(ctx, nodeName, taint); err != nil {
			m.emitEvent("taint_apply_failed", map[string]interface{}{
				"node":  nodeName,
				"taint": taint.String(),
				"error": err.Error(),
			})
			return fmt.Errorf("failed to apply taint %s to node %s: %w", taint.String(), nodeName, err)
		}

		m.emitEvent("taint_applied", map[string]interface{}{
			"node":  nodeName,
			"taint": taint.String(),
		})
	}

	return nil
}

// RemoveTaints removes taints from a node
func (m *Manager) RemoveTaints(ctx context.Context, nodeName string, taintKeys []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, key := range taintKeys {
		if err := m.kubeClient.RemoveTaint(ctx, nodeName, key); err != nil {
			m.emitEvent("taint_remove_failed", map[string]interface{}{
				"node":      nodeName,
				"taint_key": key,
				"error":     err.Error(),
			})
			return fmt.Errorf("failed to remove taint %s from node %s: %w", key, nodeName, err)
		}

		m.emitEvent("taint_removed", map[string]interface{}{
			"node":      nodeName,
			"taint_key": key,
		})
	}

	return nil
}

// GetTaints gets current taints on a node
func (m *Manager) GetTaints(ctx context.Context, nodeName string) ([]config.TaintConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	taints, err := m.kubeClient.GetNodeTaints(ctx, nodeName)
	if err != nil {
		return nil, fmt.Errorf("failed to get taints for node %s: %w", nodeName, err)
	}

	result := make([]config.TaintConfig, 0, len(taints))
	for _, taint := range taints {
		result = append(result, config.TaintConfig{
			Key:    taint.Key,
			Value:  taint.Value,
			Effect: string(taint.Effect),
		})
	}

	return result, nil
}

// SyncTaints synchronizes taints to match desired state
func (m *Manager) SyncTaints(ctx context.Context, nodeName string, desiredTaints []config.TaintConfig) error {
	currentTaints, err := m.GetTaints(ctx, nodeName)
	if err != nil {
		return err
	}

	// Build maps for comparison
	currentMap := make(map[string]config.TaintConfig)
	for _, t := range currentTaints {
		currentMap[t.Key] = t
	}

	desiredMap := make(map[string]config.TaintConfig)
	for _, t := range desiredTaints {
		desiredMap[t.Key] = t
	}

	// Remove taints that shouldn't exist
	for key := range currentMap {
		if _, exists := desiredMap[key]; !exists {
			if err := m.RemoveTaints(ctx, nodeName, []string{key}); err != nil {
				return err
			}
		}
	}

	// Add or update taints
	taintsToApply := make([]config.TaintConfig, 0)
	for key, desired := range desiredMap {
		current, exists := currentMap[key]
		if !exists || current.Value != desired.Value || current.Effect != desired.Effect {
			taintsToApply = append(taintsToApply, desired)
		}
	}

	if len(taintsToApply) > 0 {
		if err := m.ApplyTaints(ctx, nodeName, taintsToApply); err != nil {
			return err
		}
	}

	return nil
}

// emitEvent sends an event
func (m *Manager) emitEvent(eventType string, data map[string]interface{}) {
	if m.eventEmitter != nil {
		m.eventEmitter.Emit(provisioning.Event{
			Type:      eventType,
			Timestamp: time.Now().Unix(),
			Data:      data,
			Source:    "taint_manager",
		})
	}
}

// String returns string representation of taint
func (t *Taint) String() string {
	if t.Value == "" {
		return fmt.Sprintf("%s:%s", t.Key, t.Effect)
	}
	return fmt.Sprintf("%s=%s:%s", t.Key, t.Value, t.Effect)
}

// ParseTaintEffect parses a taint effect string
func ParseTaintEffect(effect string) TaintEffect {
	switch strings.ToLower(effect) {
	case "noschedule":
		return TaintEffectNoSchedule
	case "prefernoschedule":
		return TaintEffectPreferNoSchedule
	case "noexecute":
		return TaintEffectNoExecute
	default:
		return TaintEffectNoSchedule
	}
}

// =============================================================================
// Kubectl-based Kube Client Implementation
// =============================================================================

// KubectlClient implements KubeClient using kubectl
type KubectlClient struct {
	kubeconfigPath string
	executor       CommandExecutor
}

// CommandExecutor executes shell commands
type CommandExecutor interface {
	Execute(ctx context.Context, command string, args ...string) (string, error)
}

// NewKubectlClient creates a new kubectl-based client
func NewKubectlClient(kubeconfigPath string, executor CommandExecutor) *KubectlClient {
	return &KubectlClient{
		kubeconfigPath: kubeconfigPath,
		executor:       executor,
	}
}

// ApplyTaint applies a taint using kubectl
func (c *KubectlClient) ApplyTaint(ctx context.Context, nodeName string, taint *Taint) error {
	taintStr := taint.String()
	args := []string{"taint", "nodes", nodeName, taintStr, "--overwrite"}

	if c.kubeconfigPath != "" {
		args = append([]string{"--kubeconfig", c.kubeconfigPath}, args...)
	}

	_, err := c.executor.Execute(ctx, "kubectl", args...)
	return err
}

// RemoveTaint removes a taint using kubectl
func (c *KubectlClient) RemoveTaint(ctx context.Context, nodeName string, taintKey string) error {
	// Remove taint by appending "-" to the key
	args := []string{"taint", "nodes", nodeName, taintKey + "-"}

	if c.kubeconfigPath != "" {
		args = append([]string{"--kubeconfig", c.kubeconfigPath}, args...)
	}

	_, err := c.executor.Execute(ctx, "kubectl", args...)
	return err
}

// GetNodeTaints gets taints from a node using kubectl
func (c *KubectlClient) GetNodeTaints(ctx context.Context, nodeName string) ([]*Taint, error) {
	args := []string{"get", "node", nodeName, "-o", "jsonpath={.spec.taints}"}

	if c.kubeconfigPath != "" {
		args = append([]string{"--kubeconfig", c.kubeconfigPath}, args...)
	}

	output, err := c.executor.Execute(ctx, "kubectl", args...)
	if err != nil {
		return nil, err
	}

	// Parse output (simplified - in production would use JSON parsing)
	return parseTaintsOutput(output), nil
}

// ExecuteCommand executes a kubectl command
func (c *KubectlClient) ExecuteCommand(ctx context.Context, args ...string) (string, error) {
	if c.kubeconfigPath != "" {
		args = append([]string{"--kubeconfig", c.kubeconfigPath}, args...)
	}
	return c.executor.Execute(ctx, "kubectl", args...)
}

// parseTaintsOutput parses kubectl taint output
func parseTaintsOutput(output string) []*Taint {
	// Simplified parsing - in production would use proper JSON parsing
	taints := make([]*Taint, 0)

	// Remove brackets and split
	output = strings.Trim(output, "[]")
	if output == "" || output == "null" {
		return taints
	}

	// Parse JSON-like output
	// This is a simplified version - production would use encoding/json
	parts := strings.Split(output, "},{")
	for _, part := range parts {
		part = strings.Trim(part, "{}")
		taint := &Taint{}

		fields := strings.Split(part, ",")
		for _, field := range fields {
			kv := strings.SplitN(field, ":", 2)
			if len(kv) != 2 {
				continue
			}
			key := strings.Trim(kv[0], "\"")
			value := strings.Trim(kv[1], "\"")

			switch key {
			case "key":
				taint.Key = value
			case "value":
				taint.Value = value
			case "effect":
				taint.Effect = TaintEffect(value)
			}
		}

		if taint.Key != "" {
			taints = append(taints, taint)
		}
	}

	return taints
}

// =============================================================================
// Predefined Taint Templates
// =============================================================================

// TaintTemplate provides predefined taint configurations
type TaintTemplate struct {
	Name        string
	Description string
	Taints      []config.TaintConfig
}

// GetPredefinedTemplates returns common taint templates
func GetPredefinedTemplates() []TaintTemplate {
	return []TaintTemplate{
		{
			Name:        "master-only",
			Description: "Prevents workloads from scheduling on master nodes",
			Taints: []config.TaintConfig{
				{Key: "node-role.kubernetes.io/master", Value: "", Effect: "NoSchedule"},
				{Key: "node-role.kubernetes.io/control-plane", Value: "", Effect: "NoSchedule"},
			},
		},
		{
			Name:        "gpu-workloads",
			Description: "Restricts node to GPU workloads only",
			Taints: []config.TaintConfig{
				{Key: "nvidia.com/gpu", Value: "true", Effect: "NoSchedule"},
			},
		},
		{
			Name:        "spot-instance",
			Description: "Marks node as spot/preemptible instance",
			Taints: []config.TaintConfig{
				{Key: "kubernetes.io/spot-instance", Value: "true", Effect: "PreferNoSchedule"},
			},
		},
		{
			Name:        "dedicated-team",
			Description: "Dedicates node to a specific team",
			Taints: []config.TaintConfig{
				{Key: "dedicated", Value: "team", Effect: "NoSchedule"},
			},
		},
		{
			Name:        "maintenance",
			Description: "Marks node for maintenance (evicts pods)",
			Taints: []config.TaintConfig{
				{Key: "node.kubernetes.io/maintenance", Value: "true", Effect: "NoExecute"},
			},
		},
	}
}

// GetTemplate returns a specific template by name
func GetTemplate(name string) *TaintTemplate {
	templates := GetPredefinedTemplates()
	for _, t := range templates {
		if t.Name == name {
			return &t
		}
	}
	return nil
}
