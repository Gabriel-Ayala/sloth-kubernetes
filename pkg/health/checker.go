// Package health provides cluster health checking functionality
package health

import (
	"fmt"
	"strings"
	"time"
)

// CheckStatus represents the status of a health check
type CheckStatus string

const (
	StatusHealthy  CheckStatus = "healthy"
	StatusWarning  CheckStatus = "warning"
	StatusCritical CheckStatus = "critical"
	StatusUnknown  CheckStatus = "unknown"
)

// CheckResult represents the result of a single health check
type CheckResult struct {
	Name        string
	Status      CheckStatus
	Message     string
	Details     []string
	Duration    time.Duration
	CheckedAt   time.Time
	Remediation string
}

// HealthReport represents the overall health report of a cluster
type HealthReport struct {
	ClusterName     string
	CheckedAt       time.Time
	Duration        time.Duration
	OverallStatus   CheckStatus
	Checks          []CheckResult
	Summary         Summary
	Recommendations []string
}

// Summary provides aggregate statistics
type Summary struct {
	TotalChecks      int
	HealthyChecks    int
	WarningChecks    int
	CriticalChecks   int
	UnknownChecks    int
	NodesTotal       int
	NodesReady       int
	PodsTotal        int
	PodsRunning      int
	PodsPending      int
	PodsFailed       int
	CertDaysToExpire int
}

// Checker performs health checks on a cluster
type Checker struct {
	masterIP   string
	sshKey     string
	kubeconfig string
	verbose    bool
}

// NewChecker creates a new health checker
func NewChecker(masterIP, sshKey string) *Checker {
	return &Checker{
		masterIP: masterIP,
		sshKey:   sshKey,
	}
}

// SetVerbose enables verbose output
func (c *Checker) SetVerbose(v bool) {
	c.verbose = v
}

// SetKubeconfig sets the kubeconfig path for local checks
func (c *Checker) SetKubeconfig(path string) {
	c.kubeconfig = path
}

// RunAllChecks executes all health checks and returns a report
func (c *Checker) RunAllChecks(clusterName string) (*HealthReport, error) {
	startTime := time.Now()

	report := &HealthReport{
		ClusterName:   clusterName,
		CheckedAt:     startTime,
		OverallStatus: StatusHealthy,
		Checks:        []CheckResult{},
	}

	// Run all checks
	checks := []func() CheckResult{
		c.CheckNodes,
		c.CheckSystemPods,
		c.CheckCoreDNS,
		c.CheckCertificates,
		c.CheckEtcd,
		c.CheckAPIServer,
		c.CheckStorage,
		c.CheckNetworking,
		c.CheckMemoryPressure,
		c.CheckDiskPressure,
	}

	checkNames := []string{"Nodes", "SystemPods", "CoreDNS", "Certificates", "Etcd", "APIServer", "Storage", "Networking", "MemoryPressure", "DiskPressure"}
	for i, check := range checks {
		if c.verbose {
			fmt.Printf("Running check: %s...\n", checkNames[i])
		}
		result := check()
		if c.verbose {
			fmt.Printf("Check %s completed: %s\n", checkNames[i], result.Status)
		}
		report.Checks = append(report.Checks, result)

		// Update overall status (critical > warning > healthy)
		if result.Status == StatusCritical {
			report.OverallStatus = StatusCritical
		} else if result.Status == StatusWarning && report.OverallStatus != StatusCritical {
			report.OverallStatus = StatusWarning
		}

		// Collect summary stats
		switch result.Status {
		case StatusHealthy:
			report.Summary.HealthyChecks++
		case StatusWarning:
			report.Summary.WarningChecks++
		case StatusCritical:
			report.Summary.CriticalChecks++
		default:
			report.Summary.UnknownChecks++
		}
		report.Summary.TotalChecks++
	}

	// Generate recommendations
	report.Recommendations = c.generateRecommendations(report)

	report.Duration = time.Since(startTime)
	return report, nil
}

// generateRecommendations creates actionable recommendations based on check results
func (c *Checker) generateRecommendations(report *HealthReport) []string {
	var recommendations []string

	for _, check := range report.Checks {
		if check.Status == StatusCritical || check.Status == StatusWarning {
			if check.Remediation != "" {
				recommendations = append(recommendations, check.Remediation)
			}
		}
	}

	// Add general recommendations
	if report.Summary.WarningChecks > 0 || report.Summary.CriticalChecks > 0 {
		recommendations = append(recommendations, "Review the detailed check results above for specific issues")
	}

	return recommendations
}

// PrintReport prints the health report in a formatted way
func (r *HealthReport) PrintReport() {
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════════")
	fmt.Printf("  Cluster Health Report: %s\n", r.ClusterName)
	fmt.Println("═══════════════════════════════════════════════════════════════════")
	fmt.Printf("  Checked At: %s\n", r.CheckedAt.Format(time.RFC3339))
	fmt.Printf("  Duration:   %s\n", r.Duration.Round(time.Millisecond))
	fmt.Println()

	// Overall status
	statusIcon := getStatusIcon(r.OverallStatus)
	fmt.Printf("  Overall Status: %s %s\n", statusIcon, strings.ToUpper(string(r.OverallStatus)))
	fmt.Println()

	// Summary
	fmt.Println("─────────────────────────────────────────────────────────────────────")
	fmt.Println("  Summary")
	fmt.Println("─────────────────────────────────────────────────────────────────────")
	fmt.Printf("  Total Checks:    %d\n", r.Summary.TotalChecks)
	fmt.Printf("  Healthy:         %d\n", r.Summary.HealthyChecks)
	fmt.Printf("  Warnings:        %d\n", r.Summary.WarningChecks)
	fmt.Printf("  Critical:        %d\n", r.Summary.CriticalChecks)
	fmt.Println()

	// Detailed checks
	fmt.Println("─────────────────────────────────────────────────────────────────────")
	fmt.Println("  Detailed Checks")
	fmt.Println("─────────────────────────────────────────────────────────────────────")
	fmt.Println()

	for _, check := range r.Checks {
		icon := getStatusIcon(check.Status)
		fmt.Printf("  %s %s\n", icon, check.Name)
		fmt.Printf("     Status:  %s\n", check.Status)
		fmt.Printf("     Message: %s\n", check.Message)

		if len(check.Details) > 0 {
			fmt.Println("     Details:")
			for _, detail := range check.Details {
				fmt.Printf("       - %s\n", detail)
			}
		}
		fmt.Println()
	}

	// Recommendations
	if len(r.Recommendations) > 0 {
		fmt.Println("─────────────────────────────────────────────────────────────────────")
		fmt.Println("  Recommendations")
		fmt.Println("─────────────────────────────────────────────────────────────────────")
		for i, rec := range r.Recommendations {
			fmt.Printf("  %d. %s\n", i+1, rec)
		}
		fmt.Println()
	}

	fmt.Println("═══════════════════════════════════════════════════════════════════")
}

// PrintCompact prints a compact version of the health report
func (r *HealthReport) PrintCompact() {
	statusIcon := getStatusIcon(r.OverallStatus)
	fmt.Printf("\n%s Cluster: %s - %s\n", statusIcon, r.ClusterName, strings.ToUpper(string(r.OverallStatus)))
	fmt.Printf("   Checks: %d total, %d healthy, %d warning, %d critical\n",
		r.Summary.TotalChecks, r.Summary.HealthyChecks, r.Summary.WarningChecks, r.Summary.CriticalChecks)

	// Show failed checks only
	for _, check := range r.Checks {
		if check.Status == StatusCritical || check.Status == StatusWarning {
			icon := getStatusIcon(check.Status)
			fmt.Printf("   %s %s: %s\n", icon, check.Name, check.Message)
		}
	}
	fmt.Println()
}

func getStatusIcon(status CheckStatus) string {
	switch status {
	case StatusHealthy:
		return "[OK]"
	case StatusWarning:
		return "[WARN]"
	case StatusCritical:
		return "[FAIL]"
	default:
		return "[?]"
	}
}
