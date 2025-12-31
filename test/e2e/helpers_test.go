// +build e2e

// Package e2e provides test helpers and utilities
package e2e

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// =============================================================================
// AWS Test Utilities
// =============================================================================

// AWSTestClient holds AWS SDK clients for testing
type AWSTestClient struct {
	EC2Client *ec2.Client
	STSClient *sts.Client
	AccountID string
	Region    string
}

// NewAWSTestClient creates a new AWS test client
func NewAWSTestClient(ctx context.Context, region string) (*AWSTestClient, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := &AWSTestClient{
		EC2Client: ec2.NewFromConfig(cfg),
		STSClient: sts.NewFromConfig(cfg),
		Region:    region,
	}

	// Get account ID
	identity, err := client.STSClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to get caller identity: %w", err)
	}
	client.AccountID = *identity.Account

	return client, nil
}

// ValidateAWSCredentials checks if AWS credentials are valid
func ValidateAWSCredentials(t *testing.T) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		t.Logf("AWS credentials not configured: %v", err)
		return false
	}

	client := sts.NewFromConfig(cfg)
	_, err = client.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		t.Logf("AWS credentials invalid: %v", err)
		return false
	}

	return true
}

// =============================================================================
// Test Environment Helpers
// =============================================================================

// TestEnvironment holds test environment configuration
type TestEnvironment struct {
	AWSRegion       string
	AWSAccessKeyID  string
	AWSSecretKey    string
	AWSSessionToken string
	S3Bucket        string
	TestPrefix      string
	Timeout         time.Duration
}

// NewTestEnvironment creates a new test environment from env vars
func NewTestEnvironment() *TestEnvironment {
	return &TestEnvironment{
		AWSRegion:       getEnvOrDefault("AWS_REGION", "us-east-1"),
		AWSAccessKeyID:  os.Getenv("AWS_ACCESS_KEY_ID"),
		AWSSecretKey:    os.Getenv("AWS_SECRET_ACCESS_KEY"),
		AWSSessionToken: os.Getenv("AWS_SESSION_TOKEN"),
		S3Bucket:        os.Getenv("E2E_S3_BUCKET"),
		TestPrefix:      fmt.Sprintf("e2e-test-%d", time.Now().Unix()),
		Timeout:         30 * time.Minute,
	}
}

// HasAWSCredentials checks if AWS credentials are available
func (e *TestEnvironment) HasAWSCredentials() bool {
	return e.AWSAccessKeyID != "" && e.AWSSecretKey != ""
}

// HasS3Bucket checks if S3 bucket is configured
func (e *TestEnvironment) HasS3Bucket() bool {
	return e.S3Bucket != ""
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// =============================================================================
// Test Logging Helpers
// =============================================================================

// TestLogger provides structured logging for tests
type TestLogger struct {
	t         *testing.T
	component string
}

// NewTestLogger creates a new test logger
func NewTestLogger(t *testing.T, component string) *TestLogger {
	return &TestLogger{t: t, component: component}
}

// Info logs an info message
func (l *TestLogger) Info(format string, args ...interface{}) {
	l.t.Logf("[%s] ℹ️  %s", l.component, fmt.Sprintf(format, args...))
}

// Success logs a success message
func (l *TestLogger) Success(format string, args ...interface{}) {
	l.t.Logf("[%s] ✅ %s", l.component, fmt.Sprintf(format, args...))
}

// Warning logs a warning message
func (l *TestLogger) Warning(format string, args ...interface{}) {
	l.t.Logf("[%s] ⚠️  %s", l.component, fmt.Sprintf(format, args...))
}

// Error logs an error message
func (l *TestLogger) Error(format string, args ...interface{}) {
	l.t.Logf("[%s] ❌ %s", l.component, fmt.Sprintf(format, args...))
}

// Phase logs a phase marker
func (l *TestLogger) Phase(name string) {
	l.t.Logf("[%s] 📋 PHASE: %s", l.component, name)
}

// =============================================================================
// Test Timing Helpers
// =============================================================================

// Timer tracks test duration
type Timer struct {
	name      string
	startTime time.Time
	t         *testing.T
}

// NewTimer creates a new timer
func NewTimer(t *testing.T, name string) *Timer {
	timer := &Timer{
		name:      name,
		startTime: time.Now(),
		t:         t,
	}
	t.Logf("⏱️  Starting: %s", name)
	return timer
}

// Stop stops the timer and logs duration
func (timer *Timer) Stop() time.Duration {
	duration := time.Since(timer.startTime)
	timer.t.Logf("⏱️  Completed: %s in %v", timer.name, duration)
	return duration
}

// =============================================================================
// Resource Cleanup Helpers
// =============================================================================

// CleanupFunc is a function that cleans up resources
type CleanupFunc func() error

// CleanupRegistry tracks cleanup functions
type CleanupRegistry struct {
	t        *testing.T
	cleanups []CleanupFunc
}

// NewCleanupRegistry creates a new cleanup registry
func NewCleanupRegistry(t *testing.T) *CleanupRegistry {
	return &CleanupRegistry{t: t}
}

// Register adds a cleanup function
func (r *CleanupRegistry) Register(fn CleanupFunc) {
	r.cleanups = append(r.cleanups, fn)
}

// RunAll runs all cleanup functions in reverse order
func (r *CleanupRegistry) RunAll() {
	r.t.Log("🧹 Running cleanup...")
	for i := len(r.cleanups) - 1; i >= 0; i-- {
		if err := r.cleanups[i](); err != nil {
			r.t.Logf("⚠️  Cleanup warning: %v", err)
		}
	}
	r.t.Log("✅ Cleanup complete")
}

// =============================================================================
// Assertion Helpers
// =============================================================================

// AssertEventuallyTrue retries an assertion until it passes or times out
func AssertEventuallyTrue(t *testing.T, condition func() bool, timeout time.Duration, message string) bool {
	deadline := time.Now().Add(timeout)
	interval := time.Second

	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(interval)
	}

	t.Errorf("Condition not met within %v: %s", timeout, message)
	return false
}

// WaitForCondition waits for a condition to be true
func WaitForCondition(ctx context.Context, condition func() (bool, error), interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			done, err := condition()
			if err != nil {
				return err
			}
			if done {
				return nil
			}
		}
	}
}

// =============================================================================
// Pulumi Stack Helpers
// =============================================================================

// PulumiStackConfig holds configuration for test Pulumi stacks
type PulumiStackConfig struct {
	ProjectName string
	StackName   string
	WorkDir     string
	Config      map[string]string
}

// NewPulumiStackConfig creates a default test stack configuration
func NewPulumiStackConfig(testName string) *PulumiStackConfig {
	return &PulumiStackConfig{
		ProjectName: "sloth-kubernetes-e2e-test",
		StackName:   fmt.Sprintf("e2e-%s-%d", testName, time.Now().UnixNano()),
		WorkDir:     "",
		Config:      make(map[string]string),
	}
}

// WithWorkDir sets the working directory
func (c *PulumiStackConfig) WithWorkDir(dir string) *PulumiStackConfig {
	c.WorkDir = dir
	return c
}

// WithConfig adds a configuration value
func (c *PulumiStackConfig) WithConfig(key, value string) *PulumiStackConfig {
	c.Config[key] = value
	return c
}

// =============================================================================
// Configuration Builders
// =============================================================================

// TestClusterConfigBuilder builds test cluster configurations
type TestClusterConfigBuilder struct {
	name        string
	provider    string
	region      string
	masterCount int
	workerCount int
	masterSize  string
	workerSize  string
	vpcCIDR     string
	spotEnabled bool
}

// NewTestClusterConfigBuilder creates a new cluster config builder
func NewTestClusterConfigBuilder(name string) *TestClusterConfigBuilder {
	return &TestClusterConfigBuilder{
		name:        name,
		provider:    "aws",
		region:      "us-east-1",
		masterCount: 1,
		workerCount: 2,
		masterSize:  "t3.medium",
		workerSize:  "t3.medium",
		vpcCIDR:     "10.0.0.0/16",
		spotEnabled: false,
	}
}

// WithProvider sets the cloud provider
func (b *TestClusterConfigBuilder) WithProvider(provider string) *TestClusterConfigBuilder {
	b.provider = provider
	return b
}

// WithRegion sets the region
func (b *TestClusterConfigBuilder) WithRegion(region string) *TestClusterConfigBuilder {
	b.region = region
	return b
}

// WithMasters sets the master node count
func (b *TestClusterConfigBuilder) WithMasters(count int) *TestClusterConfigBuilder {
	b.masterCount = count
	return b
}

// WithWorkers sets the worker node count
func (b *TestClusterConfigBuilder) WithWorkers(count int) *TestClusterConfigBuilder {
	b.workerCount = count
	return b
}

// WithMasterSize sets the master node size
func (b *TestClusterConfigBuilder) WithMasterSize(size string) *TestClusterConfigBuilder {
	b.masterSize = size
	return b
}

// WithWorkerSize sets the worker node size
func (b *TestClusterConfigBuilder) WithWorkerSize(size string) *TestClusterConfigBuilder {
	b.workerSize = size
	return b
}

// WithVPCCIDR sets the VPC CIDR
func (b *TestClusterConfigBuilder) WithVPCCIDR(cidr string) *TestClusterConfigBuilder {
	b.vpcCIDR = cidr
	return b
}

// WithSpotInstances enables spot instances
func (b *TestClusterConfigBuilder) WithSpotInstances(enabled bool) *TestClusterConfigBuilder {
	b.spotEnabled = enabled
	return b
}

// GetName returns the cluster name
func (b *TestClusterConfigBuilder) GetName() string {
	return b.name
}

// GetProvider returns the provider
func (b *TestClusterConfigBuilder) GetProvider() string {
	return b.provider
}

// GetRegion returns the region
func (b *TestClusterConfigBuilder) GetRegion() string {
	return b.region
}

// GetMasterCount returns the master count
func (b *TestClusterConfigBuilder) GetMasterCount() int {
	return b.masterCount
}

// GetWorkerCount returns the worker count
func (b *TestClusterConfigBuilder) GetWorkerCount() int {
	return b.workerCount
}

// GetMasterSize returns the master size
func (b *TestClusterConfigBuilder) GetMasterSize() string {
	return b.masterSize
}

// GetWorkerSize returns the worker size
func (b *TestClusterConfigBuilder) GetWorkerSize() string {
	return b.workerSize
}

// GetVPCCIDR returns the VPC CIDR
func (b *TestClusterConfigBuilder) GetVPCCIDR() string {
	return b.vpcCIDR
}

// IsSpotEnabled returns whether spot instances are enabled
func (b *TestClusterConfigBuilder) IsSpotEnabled() bool {
	return b.spotEnabled
}

// =============================================================================
// Retry Helpers
// =============================================================================

// RetryConfig holds retry configuration
type RetryConfig struct {
	MaxAttempts int
	InitialWait time.Duration
	MaxWait     time.Duration
	Multiplier  float64
}

// DefaultRetryConfig returns default retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts: 5,
		InitialWait: 1 * time.Second,
		MaxWait:     30 * time.Second,
		Multiplier:  2.0,
	}
}

// RetryWithBackoff executes a function with exponential backoff
func RetryWithBackoff(ctx context.Context, cfg *RetryConfig, fn func() error) error {
	var lastErr error
	wait := cfg.InitialWait

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		if err := fn(); err != nil {
			lastErr = err

			// Check if context is cancelled
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			// Last attempt, don't wait
			if attempt == cfg.MaxAttempts {
				break
			}

			// Wait with backoff
			time.Sleep(wait)

			// Increase wait time
			wait = time.Duration(float64(wait) * cfg.Multiplier)
			if wait > cfg.MaxWait {
				wait = cfg.MaxWait
			}
		} else {
			return nil
		}
	}

	return fmt.Errorf("after %d attempts: %w", cfg.MaxAttempts, lastErr)
}

// =============================================================================
// Mock Event Emitter
// =============================================================================

// MockEventEmitter is a mock event emitter for testing
type MockEventEmitter struct {
	events []MockEvent
}

// MockEvent represents a captured event
type MockEvent struct {
	Type    string
	Payload interface{}
	Time    time.Time
}

// NewMockEventEmitter creates a new mock event emitter
func NewMockEventEmitter() *MockEventEmitter {
	return &MockEventEmitter{
		events: make([]MockEvent, 0),
	}
}

// Emit captures an event
func (m *MockEventEmitter) Emit(eventType string, payload interface{}) {
	m.events = append(m.events, MockEvent{
		Type:    eventType,
		Payload: payload,
		Time:    time.Now(),
	})
}

// GetEvents returns all captured events
func (m *MockEventEmitter) GetEvents() []MockEvent {
	return m.events
}

// GetEventsByType returns events of a specific type
func (m *MockEventEmitter) GetEventsByType(eventType string) []MockEvent {
	var result []MockEvent
	for _, e := range m.events {
		if e.Type == eventType {
			result = append(result, e)
		}
	}
	return result
}

// Clear clears all captured events
func (m *MockEventEmitter) Clear() {
	m.events = make([]MockEvent, 0)
}

// Count returns the number of captured events
func (m *MockEventEmitter) Count() int {
	return len(m.events)
}

// =============================================================================
// Mock Metrics Collector
// =============================================================================

// MockMetricsCollector is a mock metrics collector for testing
type MockMetricsCollector struct {
	cpuUsage    float64
	memoryUsage float64
	nodeCount   int
}

// NewMockMetricsCollector creates a new mock metrics collector
func NewMockMetricsCollector() *MockMetricsCollector {
	return &MockMetricsCollector{
		cpuUsage:    50.0,
		memoryUsage: 50.0,
		nodeCount:   3,
	}
}

// SetCPUUsage sets the mock CPU usage
func (m *MockMetricsCollector) SetCPUUsage(usage float64) {
	m.cpuUsage = usage
}

// SetMemoryUsage sets the mock memory usage
func (m *MockMetricsCollector) SetMemoryUsage(usage float64) {
	m.memoryUsage = usage
}

// SetNodeCount sets the mock node count
func (m *MockMetricsCollector) SetNodeCount(count int) {
	m.nodeCount = count
}

// GetCPUUsage returns the mock CPU usage
func (m *MockMetricsCollector) GetCPUUsage() float64 {
	return m.cpuUsage
}

// GetMemoryUsage returns the mock memory usage
func (m *MockMetricsCollector) GetMemoryUsage() float64 {
	return m.memoryUsage
}

// GetNodeCount returns the mock node count
func (m *MockMetricsCollector) GetNodeCount() int {
	return m.nodeCount
}

// CollectMetrics returns mock metrics
func (m *MockMetricsCollector) CollectMetrics(ctx context.Context) (map[string]float64, error) {
	return map[string]float64{
		"cpu_usage":    m.cpuUsage,
		"memory_usage": m.memoryUsage,
		"node_count":   float64(m.nodeCount),
	}, nil
}

// =============================================================================
// Test Fixture Helpers
// =============================================================================

// TestFixture holds test data and resources
type TestFixture struct {
	t        *testing.T
	name     string
	cleanups []CleanupFunc
	data     map[string]interface{}
}

// NewTestFixture creates a new test fixture
func NewTestFixture(t *testing.T, name string) *TestFixture {
	return &TestFixture{
		t:        t,
		name:     name,
		cleanups: make([]CleanupFunc, 0),
		data:     make(map[string]interface{}),
	}
}

// Set stores a value in the fixture
func (f *TestFixture) Set(key string, value interface{}) {
	f.data[key] = value
}

// Get retrieves a value from the fixture
func (f *TestFixture) Get(key string) (interface{}, bool) {
	value, ok := f.data[key]
	return value, ok
}

// GetString retrieves a string value from the fixture
func (f *TestFixture) GetString(key string) string {
	if value, ok := f.data[key]; ok {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return ""
}

// GetInt retrieves an int value from the fixture
func (f *TestFixture) GetInt(key string) int {
	if value, ok := f.data[key]; ok {
		if i, ok := value.(int); ok {
			return i
		}
	}
	return 0
}

// AddCleanup adds a cleanup function
func (f *TestFixture) AddCleanup(fn CleanupFunc) {
	f.cleanups = append(f.cleanups, fn)
}

// Cleanup runs all cleanup functions
func (f *TestFixture) Cleanup() {
	f.t.Logf("[%s] Running fixture cleanup...", f.name)
	for i := len(f.cleanups) - 1; i >= 0; i-- {
		if err := f.cleanups[i](); err != nil {
			f.t.Logf("[%s] Cleanup warning: %v", f.name, err)
		}
	}
}

// =============================================================================
// Test Skip Helpers
// =============================================================================

// SkipIfNoAWSCredentials skips the test if AWS credentials are not available
func SkipIfNoAWSCredentials(t *testing.T) {
	env := NewTestEnvironment()
	if !env.HasAWSCredentials() {
		t.Skip("Skipping: AWS credentials not configured (set AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)")
	}
}

// SkipIfNoS3Bucket skips the test if S3 bucket is not configured
func SkipIfNoS3Bucket(t *testing.T) {
	env := NewTestEnvironment()
	if !env.HasS3Bucket() {
		t.Skip("Skipping: S3 bucket not configured (set E2E_S3_BUCKET)")
	}
}

// SkipIfShortTest skips the test if running in short mode
func SkipIfShortTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping: test is too long for short mode")
	}
}

// =============================================================================
// Test Report Helpers
// =============================================================================

// TestReport holds test execution report data
type TestReport struct {
	TestName    string
	StartTime   time.Time
	EndTime     time.Time
	Duration    time.Duration
	Status      string
	Phases      []PhaseReport
	Errors      []string
	Warnings    []string
	Metrics     map[string]interface{}
}

// PhaseReport holds phase execution data
type PhaseReport struct {
	Name      string
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
	Status    string
	Details   string
}

// NewTestReport creates a new test report
func NewTestReport(testName string) *TestReport {
	return &TestReport{
		TestName:  testName,
		StartTime: time.Now(),
		Status:    "running",
		Phases:    make([]PhaseReport, 0),
		Errors:    make([]string, 0),
		Warnings:  make([]string, 0),
		Metrics:   make(map[string]interface{}),
	}
}

// StartPhase starts a new phase
func (r *TestReport) StartPhase(name string) *PhaseReport {
	phase := PhaseReport{
		Name:      name,
		StartTime: time.Now(),
		Status:    "running",
	}
	r.Phases = append(r.Phases, phase)
	return &r.Phases[len(r.Phases)-1]
}

// EndPhase ends the current phase
func (r *TestReport) EndPhase(phase *PhaseReport, status, details string) {
	phase.EndTime = time.Now()
	phase.Duration = phase.EndTime.Sub(phase.StartTime)
	phase.Status = status
	phase.Details = details
}

// AddError adds an error to the report
func (r *TestReport) AddError(err string) {
	r.Errors = append(r.Errors, err)
}

// AddWarning adds a warning to the report
func (r *TestReport) AddWarning(warning string) {
	r.Warnings = append(r.Warnings, warning)
}

// SetMetric sets a metric value
func (r *TestReport) SetMetric(key string, value interface{}) {
	r.Metrics[key] = value
}

// Finish finalizes the report
func (r *TestReport) Finish(status string) {
	r.EndTime = time.Now()
	r.Duration = r.EndTime.Sub(r.StartTime)
	r.Status = status
}

// Print prints the report to the test log
func (r *TestReport) Print(t *testing.T) {
	t.Logf("============================================================")
	t.Logf("TEST REPORT: %s", r.TestName)
	t.Logf("============================================================")
	t.Logf("Status: %s", r.Status)
	t.Logf("Duration: %v", r.Duration)
	t.Logf("")

	if len(r.Phases) > 0 {
		t.Logf("PHASES:")
		for _, phase := range r.Phases {
			t.Logf("  - %s: %s (%v)", phase.Name, phase.Status, phase.Duration)
			if phase.Details != "" {
				t.Logf("    Details: %s", phase.Details)
			}
		}
		t.Logf("")
	}

	if len(r.Errors) > 0 {
		t.Logf("ERRORS:")
		for _, err := range r.Errors {
			t.Logf("  - %s", err)
		}
		t.Logf("")
	}

	if len(r.Warnings) > 0 {
		t.Logf("WARNINGS:")
		for _, warning := range r.Warnings {
			t.Logf("  - %s", warning)
		}
		t.Logf("")
	}

	if len(r.Metrics) > 0 {
		t.Logf("METRICS:")
		for key, value := range r.Metrics {
			t.Logf("  - %s: %v", key, value)
		}
	}

	t.Logf("============================================================")
}
