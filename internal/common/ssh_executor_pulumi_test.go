package common

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
)

// SSHMocks provides mock implementations for SSH executor tests
type SSHMocks struct {
	pulumi.MockResourceMonitor
}

func (m *SSHMocks) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	// Mock remote.Command creation
	if args.TypeToken == "command:remote:Command" {
		return args.Name + "_id", resource.PropertyMap{
			"stdout": resource.NewStringProperty("command output"),
			"stderr": resource.NewStringProperty(""),
		}, nil
	}
	return args.Name + "_id", args.Inputs, nil
}

func (m *SSHMocks) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return args.Args, nil
}

// TEST 1: NewSSHExecutor - Constructor test
func TestNewSSHExecutor(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		privateKey := pulumi.String("test-private-key").ToStringOutput()

		executor := NewSSHExecutor(ctx, privateKey)

		// Verify executor was created
		assert.NotNil(t, executor)
		assert.NotNil(t, executor.ctx)
		assert.NotNil(t, executor.privateKey)

		return nil
	}, pulumi.WithMocks("test-project", "test-stack", &SSHMocks{}))

	assert.NoError(t, err)
}

// TEST 2: Execute - Basic execution test
func TestExecute_BasicCommand(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		privateKey := pulumi.String("ssh-rsa AAAAB3...").ToStringOutput()
		executor := NewSSHExecutor(ctx, privateKey)

		host := pulumi.String("192.168.1.10").ToStringOutput()
		command := "echo hello"

		// Execute command
		cmd, err := executor.Execute("test-command", host, command)

		// Should not error in mock environment
		assert.NoError(t, err)
		assert.NotNil(t, cmd)

		return nil
	}, pulumi.WithMocks("test-project", "test-stack", &SSHMocks{}))

	assert.NoError(t, err)
}

// TEST 3: Execute - With options
func TestExecute_WithOptions(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		privateKey := pulumi.String("ssh-rsa AAAAB3...").ToStringOutput()
		executor := NewSSHExecutor(ctx, privateKey)

		host := pulumi.String("10.0.0.5").ToStringOutput()
		command := "apt-get update"

		// Execute with options
		cmd, err := executor.Execute(
			"update-command",
			host,
			command,
			pulumi.DependsOn([]pulumi.Resource{}),
		)

		assert.NoError(t, err)
		assert.NotNil(t, cmd)

		return nil
	}, pulumi.WithMocks("test-project", "test-stack", &SSHMocks{}))

	assert.NoError(t, err)
}

// TEST 4: Execute - Complex command
func TestExecute_ComplexCommand(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		privateKey := pulumi.String("test-key").ToStringOutput()
		executor := NewSSHExecutor(ctx, privateKey)

		host := pulumi.String("server.example.com").ToStringOutput()
		command := "cd /app && docker-compose up -d && docker ps"

		cmd, err := executor.Execute("complex-deploy", host, command)

		assert.NoError(t, err)
		assert.NotNil(t, cmd)

		return nil
	}, pulumi.WithMocks("test-project", "test-stack", &SSHMocks{}))

	assert.NoError(t, err)
}

// TEST 5: ExecuteWithRetry - Single retry
func TestExecuteWithRetry_SingleRetry(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		privateKey := pulumi.String("test-key").ToStringOutput()
		executor := NewSSHExecutor(ctx, privateKey)

		host := pulumi.String("192.168.1.20").ToStringOutput()
		command := "systemctl restart nginx"

		// Execute with 1 retry (should not add retry logic)
		cmd, err := executor.ExecuteWithRetry("restart-nginx", host, command, 1)

		assert.NoError(t, err)
		assert.NotNil(t, cmd)

		return nil
	}, pulumi.WithMocks("test-project", "test-stack", &SSHMocks{}))

	assert.NoError(t, err)
}

// TEST 6: ExecuteWithRetry - Multiple retries
func TestExecuteWithRetry_MultipleRetries(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		privateKey := pulumi.String("test-key").ToStringOutput()
		executor := NewSSHExecutor(ctx, privateKey)

		host := pulumi.String("10.0.0.10").ToStringOutput()
		command := "wget https://example.com/file.tar.gz"

		// Execute with 3 retries (should wrap with retry logic)
		cmd, err := executor.ExecuteWithRetry("download-file", host, command, 3)

		assert.NoError(t, err)
		assert.NotNil(t, cmd)

		return nil
	}, pulumi.WithMocks("test-project", "test-stack", &SSHMocks{}))

	assert.NoError(t, err)
}

// TEST 7: ExecuteWithRetry - Five retries
func TestExecuteWithRetry_FiveRetries(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		privateKey := pulumi.String("test-key").ToStringOutput()
		executor := NewSSHExecutor(ctx, privateKey)

		host := pulumi.String("192.168.50.100").ToStringOutput()
		command := "apt-get update && apt-get install -y docker.io"

		// Execute with 5 retries
		cmd, err := executor.ExecuteWithRetry("install-docker", host, command, 5)

		assert.NoError(t, err)
		assert.NotNil(t, cmd)

		return nil
	}, pulumi.WithMocks("test-project", "test-stack", &SSHMocks{}))

	assert.NoError(t, err)
}

// TEST 8: ExecuteWithRetry - With options
func TestExecuteWithRetry_WithOptions(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		privateKey := pulumi.String("test-key").ToStringOutput()
		executor := NewSSHExecutor(ctx, privateKey)

		host := pulumi.String("node-1.cluster.local").ToStringOutput()
		command := "kubeadm join 10.0.0.1:6443 --token abc123"

		// Execute with retry and options
		cmd, err := executor.ExecuteWithRetry(
			"join-cluster",
			host,
			command,
			3,
			pulumi.DependsOn([]pulumi.Resource{}),
		)

		assert.NoError(t, err)
		assert.NotNil(t, cmd)

		return nil
	}, pulumi.WithMocks("test-project", "test-stack", &SSHMocks{}))

	assert.NoError(t, err)
}

// TEST 9: Execute - Multiple hosts
func TestExecute_MultipleCommands(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		privateKey := pulumi.String("test-key").ToStringOutput()
		executor := NewSSHExecutor(ctx, privateKey)

		hosts := []string{"host1", "host2", "host3"}
		commands := []string{"cmd1", "cmd2", "cmd3"}

		for i, hostname := range hosts {
			host := pulumi.String(hostname).ToStringOutput()
			cmd, err := executor.Execute(
				"command-"+hostname,
				host,
				commands[i],
			)
			assert.NoError(t, err)
			assert.NotNil(t, cmd)
		}

		return nil
	}, pulumi.WithMocks("test-project", "test-stack", &SSHMocks{}))

	assert.NoError(t, err)
}

// TEST 10: NewSSHExecutor - Nil context handling
func TestNewSSHExecutor_WithContext(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		privateKey := pulumi.String("").ToStringOutput()

		// Create executor with valid context
		executor := NewSSHExecutor(ctx, privateKey)

		assert.NotNil(t, executor)
		assert.Equal(t, ctx, executor.ctx)

		return nil
	}, pulumi.WithMocks("test-project", "test-stack", &SSHMocks{}))

	assert.NoError(t, err)
}

// TEST 11: Execute - Empty command
func TestExecute_EmptyCommand(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		privateKey := pulumi.String("test-key").ToStringOutput()
		executor := NewSSHExecutor(ctx, privateKey)

		host := pulumi.String("testhost").ToStringOutput()

		// Execute empty command
		cmd, err := executor.Execute("empty-cmd", host, "")

		assert.NoError(t, err)
		assert.NotNil(t, cmd)

		return nil
	}, pulumi.WithMocks("test-project", "test-stack", &SSHMocks{}))

	assert.NoError(t, err)
}

// TEST 12: ExecuteWithRetry - Zero retries
func TestExecuteWithRetry_ZeroRetries(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		privateKey := pulumi.String("test-key").ToStringOutput()
		executor := NewSSHExecutor(ctx, privateKey)

		host := pulumi.String("testhost").ToStringOutput()
		command := "ls -la"

		// Execute with 0 retries (should not add retry logic)
		cmd, err := executor.ExecuteWithRetry("list-files", host, command, 0)

		assert.NoError(t, err)
		assert.NotNil(t, cmd)

		return nil
	}, pulumi.WithMocks("test-project", "test-stack", &SSHMocks{}))

	assert.NoError(t, err)
}
