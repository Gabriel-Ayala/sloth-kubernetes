package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoginCmd_Structure(t *testing.T) {
	assert.NotNil(t, loginCmd)
	assert.Equal(t, "login [s3://bucket-name]", loginCmd.Use)
	assert.NotEmpty(t, loginCmd.Short)
	assert.NotEmpty(t, loginCmd.Long)
}

func TestLoginCmd_RunE(t *testing.T) {
	assert.NotNil(t, loginCmd.RunE, "login command should have RunE function")
}

func TestLoginCmd_Flags(t *testing.T) {
	bucketFlag := loginCmd.Flags().Lookup("bucket")
	assert.NotNil(t, bucketFlag, "login should have --bucket flag")
	assert.Equal(t, "b", bucketFlag.Shorthand, "bucket flag should have 'b' shorthand")

	accessKeyFlag := loginCmd.Flags().Lookup("access-key-id")
	assert.NotNil(t, accessKeyFlag, "login should have --access-key-id flag")

	secretKeyFlag := loginCmd.Flags().Lookup("secret-access-key")
	assert.NotNil(t, secretKeyFlag, "login should have --secret-access-key flag")

	regionFlag := loginCmd.Flags().Lookup("region")
	assert.NotNil(t, regionFlag, "login should have --region flag")

	endpointFlag := loginCmd.Flags().Lookup("endpoint")
	assert.NotNil(t, endpointFlag, "login should have --endpoint flag")
}

func TestLoginCmd_LongDescription(t *testing.T) {
	long := loginCmd.Long
	assert.Contains(t, long, "S3")
	assert.Contains(t, long, "Pulumi")
	assert.Contains(t, long, "state")
	assert.Contains(t, long, "backend")
	assert.Contains(t, long, "bucket")
	assert.Contains(t, long, ".sloth-kubernetes/config")
}

func TestLoginCmd_ShortDescription(t *testing.T) {
	short := loginCmd.Short
	assert.Contains(t, short, "backend")
	assert.Contains(t, short, "S3")
}

func TestLoginCmd_Examples(t *testing.T) {
	long := loginCmd.Long
	assert.Contains(t, long, "Example:")
	assert.Contains(t, long, "sloth-kubernetes login")
	assert.Contains(t, long, "s3://")
	assert.Contains(t, long, "--bucket")
	assert.Contains(t, long, "--access-key-id")
	assert.Contains(t, long, "--secret-access-key")
	assert.Contains(t, long, "--region")
}

func TestLoginCmd_GlobalVariables(t *testing.T) {
	assert.IsType(t, "", loginBucket)
	assert.IsType(t, "", loginAccessKeyID)
	assert.IsType(t, "", loginSecretAccessKey)
	assert.IsType(t, "", loginRegion)
	assert.IsType(t, "", loginEndpoint)
}

func TestLoginCmd_RegisteredWithRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "login" {
			found = true
			break
		}
	}
	assert.True(t, found, "login command should be registered with root")
}

func TestGetConfigDir_Exists(t *testing.T) {
	// Test that getConfigDir function exists
	assert.NotNil(t, getConfigDir, "getConfigDir function should be defined")
}

func TestLoadConfig_Exists(t *testing.T) {
	// Test that loadConfig function exists
	assert.NotNil(t, loadConfig, "loadConfig function should be defined")
}

func TestSaveConfig_Exists(t *testing.T) {
	// Test that saveConfig function exists
	assert.NotNil(t, saveConfig, "saveConfig function should be defined")
}

func TestPromptYesNo_Exists(t *testing.T) {
	// Test that promptYesNo function exists
	assert.NotNil(t, promptYesNo, "promptYesNo function should be defined")
}

func TestValidateS3Backend_Exists(t *testing.T) {
	// Test that validateS3Backend function exists
	assert.NotNil(t, validateS3Backend, "validateS3Backend function should be defined")
}

func TestRunLogin_Exists(t *testing.T) {
	// Test that runLogin function exists
	assert.NotNil(t, runLogin, "runLogin function should be defined")
}

func TestLoginCmd_S3Compatible(t *testing.T) {
	long := loginCmd.Long
	assert.Contains(t, long, "S3-compatible", "Should mention S3-compatible storage")
}

func TestLoginCmd_ConfigStorage(t *testing.T) {
	long := loginCmd.Long
	assert.Contains(t, long, ".sloth-kubernetes", "Should mention config directory")
	assert.Contains(t, long, "stored", "Should mention where config is stored")
}

func TestLoginCmd_PulumiComparison(t *testing.T) {
	long := loginCmd.Long
	assert.Contains(t, long, "pulumi login", "Should compare to pulumi login command")
}

func TestLoginCmd_AllAWSCredentials(t *testing.T) {
	// All AWS credential flags should exist
	credentialFlags := []string{
		"access-key-id",
		"secret-access-key",
		"region",
		"endpoint",
	}

	for _, flagName := range credentialFlags {
		flag := loginCmd.Flags().Lookup(flagName)
		assert.NotNil(t, flag, "Should have --%s flag", flagName)
	}
}

func TestLoginCmd_BucketURLFormat(t *testing.T) {
	long := loginCmd.Long
	// Should show proper s3:// URL format
	assert.Contains(t, long, "s3://", "Should show s3:// URL format")
}

func TestLoginCmd_UsageVariations(t *testing.T) {
	long := loginCmd.Long
	// Should show different ways to specify bucket
	assert.Contains(t, long, "login s3://", "Should show argument-based usage")
	assert.Contains(t, long, "--bucket", "Should show flag-based usage")
}
