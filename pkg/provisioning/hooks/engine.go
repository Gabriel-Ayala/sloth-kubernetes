// Package hooks provides provisioning hook execution
package hooks

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/chalkan3/sloth-kubernetes/pkg/provisioning"
)

// =============================================================================
// Hook Engine
// =============================================================================

// Engine manages and executes provisioning hooks
type Engine struct {
	hooks        map[provisioning.HookEvent][]HookWrapper
	executors    map[string]Executor
	eventEmitter provisioning.EventEmitter
	mu           sync.RWMutex
}

// HookWrapper wraps a hook with metadata
type HookWrapper struct {
	ID       string
	Hook     *config.HookAction
	Priority int
}

// EngineConfig holds engine configuration
type EngineConfig struct {
	EventEmitter provisioning.EventEmitter
}

// NewEngine creates a new hook engine
func NewEngine(cfg *EngineConfig) *Engine {
	engine := &Engine{
		hooks:        make(map[provisioning.HookEvent][]HookWrapper),
		executors:    make(map[string]Executor),
		eventEmitter: cfg.EventEmitter,
	}

	// Register default executors
	engine.RegisterExecutor("script", NewScriptExecutor())
	engine.RegisterExecutor("kubectl", &KubectlExecutor{})
	engine.RegisterExecutor("http", NewHTTPExecutor())

	return engine
}

// RegisterExecutor adds an executor for a hook type
func (e *Engine) RegisterExecutor(hookType string, executor Executor) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.executors[hookType] = executor
}

// RegisterHook adds a hook for an event
func (e *Engine) RegisterHook(event provisioning.HookEvent, action *config.HookAction, priority int) string {
	e.mu.Lock()
	defer e.mu.Unlock()

	hookID := fmt.Sprintf("%s-%d", event, time.Now().UnixNano())

	wrapper := HookWrapper{
		ID:       hookID,
		Hook:     action,
		Priority: priority,
	}

	e.hooks[event] = append(e.hooks[event], wrapper)

	// Sort by priority
	hooks := e.hooks[event]
	for i := len(hooks) - 1; i > 0; i-- {
		if hooks[i].Priority < hooks[i-1].Priority {
			hooks[i], hooks[i-1] = hooks[i-1], hooks[i]
		}
	}

	return hookID
}

// RegisterHooksFromConfig registers hooks from configuration
func (e *Engine) RegisterHooksFromConfig(hooksConfig *config.HooksConfig) {
	if hooksConfig == nil {
		return
	}

	// Register post-node-create hooks
	for i, action := range hooksConfig.PostNodeCreate {
		e.RegisterHook(provisioning.HookEventPostNodeCreate, &action, i)
	}

	// Register pre-cluster-destroy hooks
	for i, action := range hooksConfig.PreClusterDestroy {
		e.RegisterHook(provisioning.HookEventPreClusterDestroy, &action, i)
	}

	// Register post-cluster-ready hooks
	for i, action := range hooksConfig.PostClusterReady {
		e.RegisterHook(provisioning.HookEventPostClusterReady, &action, i)
	}

	// Register pre-node-delete hooks
	for i, action := range hooksConfig.PreNodeDelete {
		e.RegisterHook(provisioning.HookEventPreNodeDelete, &action, i)
	}

	// Register post-upgrade hooks
	for i, action := range hooksConfig.PostUpgrade {
		e.RegisterHook(provisioning.HookEventPostUpgrade, &action, i)
	}
}

// UnregisterHook removes a hook
func (e *Engine) UnregisterHook(event provisioning.HookEvent, hookID string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	hooks := e.hooks[event]
	for i, wrapper := range hooks {
		if wrapper.ID == hookID {
			e.hooks[event] = append(hooks[:i], hooks[i+1:]...)
			return true
		}
	}

	return false
}

// TriggerHooks executes all hooks for an event
func (e *Engine) TriggerHooks(ctx context.Context, event provisioning.HookEvent, data interface{}) error {
	e.mu.RLock()
	hooks := e.hooks[event]
	e.mu.RUnlock()

	if len(hooks) == 0 {
		return nil
	}

	e.emitEvent("hooks_triggered", map[string]interface{}{
		"event":      string(event),
		"hook_count": len(hooks),
	})

	for _, wrapper := range hooks {
		if err := e.executeHook(ctx, wrapper, data); err != nil {
			e.emitEvent("hook_failed", map[string]interface{}{
				"event":   string(event),
				"hook_id": wrapper.ID,
				"error":   err.Error(),
			})

			// Continue to next hook even if one fails
			// unless it's a critical hook
			continue
		}

		e.emitEvent("hook_completed", map[string]interface{}{
			"event":   string(event),
			"hook_id": wrapper.ID,
		})
	}

	return nil
}

// executeHook runs a single hook
func (e *Engine) executeHook(ctx context.Context, wrapper HookWrapper, data interface{}) error {
	action := wrapper.Hook

	// Determine executor type
	hookType := action.Type
	if hookType == "" {
		// Infer type from action content
		if action.Script != "" {
			hookType = "script"
		} else if action.Command != "" && isKubectlCommand(action.Command) {
			hookType = "kubectl"
		} else if action.URL != "" {
			hookType = "http"
		} else {
			hookType = "script"
		}
	}

	e.mu.RLock()
	executor, exists := e.executors[hookType]
	e.mu.RUnlock()

	if !exists {
		return fmt.Errorf("no executor for hook type: %s", hookType)
	}

	// Create context with timeout
	timeout := time.Duration(action.Timeout) * time.Second
	if timeout == 0 {
		timeout = 60 * time.Second // Default timeout
	}

	hookCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Execute with retry
	var lastErr error
	retryCount := action.RetryCount
	if retryCount == 0 {
		retryCount = 1
	}

	for attempt := 0; attempt < retryCount; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * time.Second) // Exponential backoff
		}

		if err := executor.Execute(hookCtx, action, data); err != nil {
			lastErr = err
			continue
		}
		return nil
	}

	return fmt.Errorf("hook failed after %d attempts: %w", retryCount, lastErr)
}

// GetHooks returns all registered hooks for an event
func (e *Engine) GetHooks(event provisioning.HookEvent) []HookWrapper {
	e.mu.RLock()
	defer e.mu.RUnlock()

	hooks := e.hooks[event]
	result := make([]HookWrapper, len(hooks))
	copy(result, hooks)
	return result
}

func (e *Engine) emitEvent(eventType string, data map[string]interface{}) {
	if e.eventEmitter != nil {
		e.eventEmitter.Emit(provisioning.Event{
			Type:      eventType,
			Timestamp: time.Now().Unix(),
			Data:      data,
			Source:    "hook_engine",
		})
	}
}

func isKubectlCommand(cmd string) bool {
	return len(cmd) > 6 && (cmd[:7] == "kubectl" || cmd[:5] == "apply" || cmd[:3] == "get")
}

// =============================================================================
// Executor Interface
// =============================================================================

// Executor executes hook actions
type Executor interface {
	Execute(ctx context.Context, action *config.HookAction, data interface{}) error
}

// =============================================================================
// Script Executor
// =============================================================================

// ScriptExecutor executes shell scripts
type ScriptExecutor struct {
	shell      string
	workDir    string
	defaultEnv map[string]string
}

// NewScriptExecutor creates a new script executor
func NewScriptExecutor() *ScriptExecutor {
	return &ScriptExecutor{
		shell:      "/bin/bash",
		defaultEnv: make(map[string]string),
	}
}

// Execute runs a script
func (e *ScriptExecutor) Execute(ctx context.Context, action *config.HookAction, data interface{}) error {
	script := action.Script
	if script == "" {
		script = action.Command
	}

	if script == "" {
		return fmt.Errorf("no script or command specified")
	}

	// Check if script is a file path
	var cmd *exec.Cmd
	if _, err := os.Stat(script); err == nil {
		// It's a file path
		cmd = exec.CommandContext(ctx, e.shell, script)
	} else {
		// It's inline script
		cmd = exec.CommandContext(ctx, e.shell, "-c", script)
	}

	// Set working directory
	if e.workDir != "" {
		cmd.Dir = e.workDir
	}

	// Merge environment variables
	cmd.Env = os.Environ()
	for k, v := range e.defaultEnv {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}
	for k, v := range action.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Add hook data to environment
	if data != nil {
		if dataMap, ok := data.(map[string]interface{}); ok {
			for k, v := range dataMap {
				cmd.Env = append(cmd.Env, fmt.Sprintf("HOOK_%s=%v", k, v))
			}
		}
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("script execution failed: %w, output: %s", err, string(output))
	}

	return nil
}

// =============================================================================
// Kubectl Executor
// =============================================================================

// KubectlExecutor executes kubectl commands
type KubectlExecutor struct {
	kubeconfigPath string
	context        string
}

// NewKubectlExecutor creates a new kubectl executor
func NewKubectlExecutor(kubeconfigPath string) *KubectlExecutor {
	return &KubectlExecutor{
		kubeconfigPath: kubeconfigPath,
	}
}

// Execute runs a kubectl command
func (e *KubectlExecutor) Execute(ctx context.Context, action *config.HookAction, data interface{}) error {
	command := action.Command
	if command == "" {
		return fmt.Errorf("no kubectl command specified")
	}

	// Build kubectl command
	args := []string{}
	if e.kubeconfigPath != "" {
		args = append(args, "--kubeconfig", e.kubeconfigPath)
	}
	if e.context != "" {
		args = append(args, "--context", e.context)
	}

	// Parse command string into args
	// Simple parsing - production would use proper shell parsing
	cmdParts := splitCommand(command)
	args = append(args, cmdParts...)

	cmd := exec.CommandContext(ctx, "kubectl", args...)

	// Set environment
	cmd.Env = os.Environ()
	for k, v := range action.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl command failed: %w, output: %s", err, string(output))
	}

	return nil
}

// =============================================================================
// HTTP Executor
// =============================================================================

// HTTPExecutor executes HTTP webhooks
type HTTPExecutor struct {
	client  *http.Client
	headers map[string]string
}

// NewHTTPExecutor creates a new HTTP executor
func NewHTTPExecutor() *HTTPExecutor {
	return &HTTPExecutor{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		headers: make(map[string]string),
	}
}

// Execute sends an HTTP request
func (e *HTTPExecutor) Execute(ctx context.Context, action *config.HookAction, data interface{}) error {
	url := action.URL
	if url == "" {
		return fmt.Errorf("no URL specified for HTTP hook")
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	req.Header.Set("Content-Type", "application/json")
	for k, v := range e.headers {
		req.Header.Set(k, v)
	}

	// Send request
	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP hook returned status %d", resp.StatusCode)
	}

	return nil
}

// =============================================================================
// Helper Functions
// =============================================================================

// splitCommand splits a command string into arguments
func splitCommand(cmd string) []string {
	var args []string
	var current string
	inQuote := false
	quoteChar := rune(0)

	for _, r := range cmd {
		switch {
		case r == '"' || r == '\'':
			if inQuote && r == quoteChar {
				inQuote = false
				quoteChar = 0
			} else if !inQuote {
				inQuote = true
				quoteChar = r
			} else {
				current += string(r)
			}
		case r == ' ' && !inQuote:
			if current != "" {
				args = append(args, current)
				current = ""
			}
		default:
			current += string(r)
		}
	}

	if current != "" {
		args = append(args, current)
	}

	return args
}

// =============================================================================
// Predefined Hook Templates
// =============================================================================

// HookTemplate represents a predefined hook configuration
type HookTemplate struct {
	Name        string
	Description string
	Event       provisioning.HookEvent
	Action      config.HookAction
}

// GetPredefinedHooks returns common hook templates
func GetPredefinedHooks() []HookTemplate {
	return []HookTemplate{
		{
			Name:        "install-monitoring-agent",
			Description: "Installs monitoring agent on new nodes",
			Event:       provisioning.HookEventPostNodeCreate,
			Action: config.HookAction{
				Type:    "script",
				Script:  "/opt/scripts/install-monitoring.sh",
				Timeout: 300,
			},
		},
		{
			Name:        "configure-node-security",
			Description: "Configures security settings on new nodes",
			Event:       provisioning.HookEventPostNodeCreate,
			Action: config.HookAction{
				Type:    "script",
				Script:  "/opt/scripts/configure-security.sh",
				Timeout: 120,
			},
		},
		{
			Name:        "notify-slack-ready",
			Description: "Sends Slack notification when cluster is ready",
			Event:       provisioning.HookEventPostClusterReady,
			Action: config.HookAction{
				Type: "http",
				URL:  "https://hooks.slack.com/services/xxx/yyy/zzz",
			},
		},
		{
			Name:        "backup-before-destroy",
			Description: "Creates backup before cluster destruction",
			Event:       provisioning.HookEventPreClusterDestroy,
			Action: config.HookAction{
				Type:    "script",
				Script:  "/opt/scripts/full-backup.sh",
				Timeout: 600,
			},
		},
		{
			Name:        "apply-essential-manifests",
			Description: "Applies essential Kubernetes manifests",
			Event:       provisioning.HookEventPostClusterReady,
			Action: config.HookAction{
				Type:    "kubectl",
				Command: "apply -f /manifests/essential/",
				Timeout: 120,
			},
		},
	}
}
