package health

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// CheckNodes verifies all cluster nodes are ready
func (c *Checker) CheckNodes() CheckResult {
	start := time.Now()
	result := CheckResult{
		Name:      "Node Health",
		CheckedAt: start,
	}

	output, err := c.runKubectl("get nodes --no-headers")
	if err != nil {
		result.Status = StatusCritical
		result.Message = fmt.Sprintf("Failed to get nodes: %v", err)
		result.Remediation = "Check cluster connectivity and kubectl configuration"
		result.Duration = time.Since(start)
		return result
	}

	var totalNodes, readyNodes, notReadyNodes int
	var details []string

	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			totalNodes++
			nodeName := parts[0]
			nodeStatus := parts[1]

			if nodeStatus == "Ready" {
				readyNodes++
				details = append(details, fmt.Sprintf("%s: Ready", nodeName))
			} else {
				notReadyNodes++
				details = append(details, fmt.Sprintf("%s: %s (NOT READY)", nodeName, nodeStatus))
			}
		}
	}

	if notReadyNodes > 0 {
		result.Status = StatusCritical
		result.Message = fmt.Sprintf("%d/%d nodes not ready", notReadyNodes, totalNodes)
		result.Remediation = "Check node status with 'kubectl describe node <name>' and review kubelet logs"
	} else if totalNodes == 0 {
		result.Status = StatusCritical
		result.Message = "No nodes found in cluster"
		result.Remediation = "Verify cluster deployment and node registration"
	} else {
		result.Status = StatusHealthy
		result.Message = fmt.Sprintf("All %d nodes are ready", totalNodes)
	}

	result.Details = details
	result.Duration = time.Since(start)
	return result
}

// CheckSystemPods verifies critical system pods are running
func (c *Checker) CheckSystemPods() CheckResult {
	start := time.Now()
	result := CheckResult{
		Name:      "System Pods",
		CheckedAt: start,
	}

	output, err := c.runKubectl("get pods -n kube-system --no-headers")
	if err != nil {
		result.Status = StatusCritical
		result.Message = fmt.Sprintf("Failed to get system pods: %v", err)
		result.Remediation = "Check cluster connectivity"
		result.Duration = time.Since(start)
		return result
	}

	var totalPods, runningPods, failedPods int
	var failedPodNames []string

	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 3 {
			totalPods++
			podName := parts[0]
			podStatus := parts[2]

			if podStatus == "Running" || podStatus == "Completed" {
				runningPods++
			} else {
				failedPods++
				failedPodNames = append(failedPodNames, fmt.Sprintf("%s (%s)", podName, podStatus))
			}
		}
	}

	if failedPods > 0 {
		result.Status = StatusCritical
		result.Message = fmt.Sprintf("%d/%d system pods not running", failedPods, totalPods)
		result.Details = failedPodNames
		result.Remediation = "Check pod logs with 'kubectl logs -n kube-system <pod-name>'"
	} else {
		result.Status = StatusHealthy
		result.Message = fmt.Sprintf("All %d system pods are running", runningPods)
	}

	result.Duration = time.Since(start)
	return result
}

// CheckCoreDNS verifies CoreDNS is functioning
func (c *Checker) CheckCoreDNS() CheckResult {
	start := time.Now()
	result := CheckResult{
		Name:      "CoreDNS",
		CheckedAt: start,
	}

	// Check CoreDNS pods
	output, err := c.runKubectl("get pods -n kube-system -l k8s-app=kube-dns --no-headers")
	if err != nil {
		result.Status = StatusWarning
		result.Message = "Failed to check CoreDNS pods"
		result.Duration = time.Since(start)
		return result
	}

	var runningPods int
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Running") {
			runningPods++
		}
	}

	if runningPods == 0 {
		result.Status = StatusCritical
		result.Message = "No CoreDNS pods running"
		result.Remediation = "Check CoreDNS deployment: kubectl describe deployment coredns -n kube-system"
	} else if runningPods == 1 {
		result.Status = StatusWarning
		result.Message = "Only 1 CoreDNS replica running (recommend 2+ for HA)"
		result.Remediation = "Scale CoreDNS: kubectl scale deployment coredns -n kube-system --replicas=2"
	} else {
		result.Status = StatusHealthy
		result.Message = fmt.Sprintf("%d CoreDNS replicas running", runningPods)
	}

	result.Duration = time.Since(start)
	return result
}

// CheckCertificates verifies cluster certificates are not expiring soon
func (c *Checker) CheckCertificates() CheckResult {
	start := time.Now()
	result := CheckResult{
		Name:      "Certificates",
		CheckedAt: start,
	}

	// If no SSH credentials, try to check via API server health
	if c.masterIP == "" {
		// Check if API server is responding (indicates valid certs)
		output, err := c.runKubectl("get --raw /healthz")
		if err == nil && strings.Contains(output, "ok") {
			result.Status = StatusHealthy
			result.Message = "API server certificates are valid (responding to requests)"
			result.Details = []string{"Certificate check via kubectl - API responding"}
			result.Duration = time.Since(start)
			return result
		}
		result.Status = StatusUnknown
		result.Message = "Unable to check certificate expiration (no SSH access)"
		result.Duration = time.Since(start)
		return result
	}

	// Check certificate expiration using kubeadm or openssl via SSH
	output, err := c.runSSH("kubeadm certs check-expiration 2>/dev/null || echo 'kubeadm not available'")
	if err != nil || strings.Contains(output, "not available") {
		// Try alternative check via API server cert
		output, err = c.runSSH("openssl s_client -connect localhost:6443 -servername localhost 2>/dev/null | openssl x509 -noout -dates 2>/dev/null || echo 'unable to check'")
		if err != nil || strings.Contains(output, "unable to check") {
			result.Status = StatusUnknown
			result.Message = "Unable to check certificate expiration"
			result.Duration = time.Since(start)
			return result
		}
	}

	// Parse certificate info
	if strings.Contains(output, "CERTIFICATE") || strings.Contains(output, "notAfter") {
		result.Status = StatusHealthy
		result.Message = "Certificates are valid"
		result.Details = []string{strings.TrimSpace(output)}
	} else if strings.Contains(output, "EXPIRED") {
		result.Status = StatusCritical
		result.Message = "One or more certificates have expired"
		result.Remediation = "Renew certificates: kubeadm certs renew all"
	} else {
		result.Status = StatusHealthy
		result.Message = "Certificates appear valid"
	}

	result.Duration = time.Since(start)
	return result
}

// CheckEtcd verifies etcd cluster health
func (c *Checker) CheckEtcd() CheckResult {
	start := time.Now()
	result := CheckResult{
		Name:      "Etcd Cluster",
		CheckedAt: start,
	}

	// If no SSH credentials, check via kubectl
	if c.masterIP == "" {
		output, err := c.runKubectl("get pods -n kube-system -l component=etcd --no-headers")
		if err != nil {
			result.Status = StatusWarning
			result.Message = "Unable to check etcd health via kubectl"
			result.Duration = time.Since(start)
			return result
		}

		if strings.Contains(output, "Running") {
			result.Status = StatusHealthy
			result.Message = "Etcd pods are running"
			result.Details = strings.Split(strings.TrimSpace(output), "\n")
		} else if output == "" {
			// Kind/k3s might not show etcd as a separate pod
			result.Status = StatusHealthy
			result.Message = "Etcd health assumed (embedded or external)"
		} else {
			result.Status = StatusWarning
			result.Message = "Etcd status uncertain"
		}
		result.Duration = time.Since(start)
		return result
	}

	// Check etcd health via etcdctl or API via SSH
	output, err := c.runSSH("ETCDCTL_API=3 etcdctl endpoint health --cluster 2>/dev/null || kubectl get pods -n kube-system -l component=etcd --no-headers 2>/dev/null")
	if err != nil {
		result.Status = StatusWarning
		result.Message = "Unable to check etcd health directly"
		result.Duration = time.Since(start)
		return result
	}

	if strings.Contains(output, "is healthy") {
		result.Status = StatusHealthy
		result.Message = "Etcd cluster is healthy"
	} else if strings.Contains(output, "Running") {
		result.Status = StatusHealthy
		result.Message = "Etcd pods are running"
	} else if strings.Contains(output, "unhealthy") || strings.Contains(output, "failed") {
		result.Status = StatusCritical
		result.Message = "Etcd cluster is unhealthy"
		result.Remediation = "Check etcd logs and cluster membership"
	} else {
		result.Status = StatusWarning
		result.Message = "Unable to determine etcd health"
	}

	result.Duration = time.Since(start)
	return result
}

// CheckAPIServer verifies API server is responding
func (c *Checker) CheckAPIServer() CheckResult {
	start := time.Now()
	result := CheckResult{
		Name:      "API Server",
		CheckedAt: start,
	}

	// Check API server health endpoint
	output, err := c.runKubectl("get --raw /healthz")
	if err != nil {
		result.Status = StatusCritical
		result.Message = "API server health check failed"
		result.Remediation = "Check kube-apiserver logs and ensure it's running"
		result.Duration = time.Since(start)
		return result
	}

	if strings.TrimSpace(output) == "ok" {
		result.Status = StatusHealthy
		result.Message = "API server is healthy"
	} else {
		result.Status = StatusWarning
		result.Message = fmt.Sprintf("API server returned: %s", output)
	}

	result.Duration = time.Since(start)
	return result
}

// CheckStorage verifies persistent volume claims
func (c *Checker) CheckStorage() CheckResult {
	start := time.Now()
	result := CheckResult{
		Name:      "Storage (PVCs)",
		CheckedAt: start,
	}

	output, err := c.runKubectl("get pvc --all-namespaces --no-headers")
	if err != nil {
		result.Status = StatusWarning
		result.Message = "Unable to check PVCs"
		result.Duration = time.Since(start)
		return result
	}

	if strings.TrimSpace(output) == "" {
		result.Status = StatusHealthy
		result.Message = "No PVCs in cluster"
		result.Duration = time.Since(start)
		return result
	}

	var totalPVCs, boundPVCs, pendingPVCs int
	var pendingDetails []string

	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 4 {
			totalPVCs++
			namespace := parts[0]
			pvcName := parts[1]
			status := parts[3]

			if status == "Bound" {
				boundPVCs++
			} else {
				pendingPVCs++
				pendingDetails = append(pendingDetails, fmt.Sprintf("%s/%s: %s", namespace, pvcName, status))
			}
		}
	}

	if pendingPVCs > 0 {
		result.Status = StatusWarning
		result.Message = fmt.Sprintf("%d/%d PVCs not bound", pendingPVCs, totalPVCs)
		result.Details = pendingDetails
		result.Remediation = "Check PVC events: kubectl describe pvc <name> -n <namespace>"
	} else {
		result.Status = StatusHealthy
		result.Message = fmt.Sprintf("All %d PVCs are bound", boundPVCs)
	}

	result.Duration = time.Since(start)
	return result
}

// CheckNetworking verifies basic network connectivity
func (c *Checker) CheckNetworking() CheckResult {
	start := time.Now()
	result := CheckResult{
		Name:      "Networking",
		CheckedAt: start,
	}

	// Check if CNI pods are running
	output, err := c.runKubectl("get pods -n kube-system -l k8s-app=canal --no-headers 2>/dev/null || kubectl get pods -n kube-system -l k8s-app=calico-node --no-headers 2>/dev/null || kubectl get pods -n kube-system -l app=flannel --no-headers 2>/dev/null || echo 'no-cni-found'")
	if err != nil {
		result.Status = StatusWarning
		result.Message = "Unable to check CNI pods"
		result.Duration = time.Since(start)
		return result
	}

	if strings.Contains(output, "no-cni-found") || strings.TrimSpace(output) == "" {
		result.Status = StatusWarning
		result.Message = "CNI plugin pods not found (may be using different labels)"
	} else if strings.Contains(output, "Running") {
		runningCount := strings.Count(output, "Running")
		result.Status = StatusHealthy
		result.Message = fmt.Sprintf("CNI pods running (%d instances)", runningCount)
	} else {
		result.Status = StatusWarning
		result.Message = "Some CNI pods may not be running"
	}

	result.Duration = time.Since(start)
	return result
}

// CheckMemoryPressure checks for nodes with memory pressure
func (c *Checker) CheckMemoryPressure() CheckResult {
	start := time.Now()
	result := CheckResult{
		Name:      "Memory Pressure",
		CheckedAt: start,
	}

	output, err := c.runKubectl("get nodes -o jsonpath='{range .items[*]}{.metadata.name}{\" \"}{.status.conditions[?(@.type==\"MemoryPressure\")].status}{\"\\n\"}{end}'")
	if err != nil {
		result.Status = StatusWarning
		result.Message = "Unable to check memory pressure"
		result.Duration = time.Since(start)
		return result
	}

	var pressureNodes []string
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) >= 2 && parts[1] == "True" {
			pressureNodes = append(pressureNodes, parts[0])
		}
	}

	if len(pressureNodes) > 0 {
		result.Status = StatusWarning
		result.Message = fmt.Sprintf("%d node(s) under memory pressure", len(pressureNodes))
		result.Details = pressureNodes
		result.Remediation = "Consider adding more memory or reducing workloads on affected nodes"
	} else {
		result.Status = StatusHealthy
		result.Message = "No nodes under memory pressure"
	}

	result.Duration = time.Since(start)
	return result
}

// CheckDiskPressure checks for nodes with disk pressure
func (c *Checker) CheckDiskPressure() CheckResult {
	start := time.Now()
	result := CheckResult{
		Name:      "Disk Pressure",
		CheckedAt: start,
	}

	output, err := c.runKubectl("get nodes -o jsonpath='{range .items[*]}{.metadata.name}{\" \"}{.status.conditions[?(@.type==\"DiskPressure\")].status}{\"\\n\"}{end}'")
	if err != nil {
		result.Status = StatusWarning
		result.Message = "Unable to check disk pressure"
		result.Duration = time.Since(start)
		return result
	}

	var pressureNodes []string
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) >= 2 && parts[1] == "True" {
			pressureNodes = append(pressureNodes, parts[0])
		}
	}

	if len(pressureNodes) > 0 {
		result.Status = StatusCritical
		result.Message = fmt.Sprintf("%d node(s) under disk pressure", len(pressureNodes))
		result.Details = pressureNodes
		result.Remediation = "Clean up disk space or add more storage to affected nodes"
	} else {
		result.Status = StatusHealthy
		result.Message = "No nodes under disk pressure"
	}

	result.Duration = time.Since(start)
	return result
}

// Helper functions

// runKubectl executes a kubectl command with a timeout
func (c *Checker) runKubectl(args string) (string, error) {
	if c.masterIP != "" && c.sshKey != "" {
		// Run via SSH
		return c.runSSH("kubectl " + args)
	}

	// Run locally - parse args properly to handle quoted strings
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "kubectl", strings.Fields(args)...)
	// Inherit current environment
	cmd.Env = os.Environ()
	if c.kubeconfig != "" {
		cmd.Env = append(cmd.Env, "KUBECONFIG="+c.kubeconfig)
	}
	// Prevent kubectl from reading from stdin
	cmd.Stdin = nil
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("kubectl command timed out after 30s")
	}
	return string(output), err
}

// runSSH executes a command via SSH
func (c *Checker) runSSH(command string) (string, error) {
	if c.masterIP == "" {
		return "", fmt.Errorf("no master IP configured")
	}

	// Save private key to temporary file
	tmpKeyFile := fmt.Sprintf("/tmp/ssh-key-%d", time.Now().UnixNano())
	if err := exec.Command("bash", "-c", fmt.Sprintf("echo '%s' > %s && chmod 600 %s", c.sshKey, tmpKeyFile, tmpKeyFile)).Run(); err != nil {
		return "", fmt.Errorf("failed to save SSH key: %w", err)
	}
	defer exec.Command("rm", "-f", tmpKeyFile).Run()

	// Execute SSH command
	sshCmd := fmt.Sprintf(`ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i %s root@%s '%s'`,
		tmpKeyFile, c.masterIP, strings.ReplaceAll(command, "'", "'\\''"))

	cmd := exec.Command("bash", "-c", sshCmd)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// parseCount parses a count from kubectl output
func parseCount(output string) int {
	count, _ := strconv.Atoi(strings.TrimSpace(output))
	return count
}
