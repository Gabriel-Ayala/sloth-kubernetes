package common

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfigFile(t *testing.T) {
	tests := []struct {
		name         string
		fileContent  string
		expectedKeys map[string]string
		expectError  bool
	}{
		{
			name: "ValidConfig",
			fileContent: `AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE
AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
DO_TOKEN=dop_v1_abc123
LINODE_TOKEN=linode123`,
			expectedKeys: map[string]string{
				"AWS_ACCESS_KEY_ID":     "AKIAIOSFODNN7EXAMPLE",
				"AWS_SECRET_ACCESS_KEY": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
				"DO_TOKEN":              "dop_v1_abc123",
				"LINODE_TOKEN":          "linode123",
			},
			expectError: false,
		},
		{
			name: "ConfigWithComments",
			fileContent: `# AWS Credentials
AWS_ACCESS_KEY_ID=AKIATEST
# DO Token
DO_TOKEN=dop_test

# Empty lines above and below

LINODE_TOKEN=lin_test`,
			expectedKeys: map[string]string{
				"AWS_ACCESS_KEY_ID": "AKIATEST",
				"DO_TOKEN":          "dop_test",
				"LINODE_TOKEN":      "lin_test",
			},
			expectError: false,
		},
		{
			name: "ConfigWithQuotes",
			fileContent: `KEY1="value with quotes"
KEY2='value with single quotes'
KEY3=value without quotes`,
			expectedKeys: map[string]string{
				"KEY1": "value with quotes",
				"KEY2": "value with single quotes",
				"KEY3": "value without quotes",
			},
			expectError: false,
		},
		{
			name: "ConfigWithSpaces",
			fileContent: `KEY1  =  value1
KEY2=  value2
  KEY3=value3
KEY4  =  "value with spaces"  `,
			expectedKeys: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
				"KEY3": "value3",
				"KEY4": "value with spaces",
			},
			expectError: false,
		},
		{
			name:         "EmptyConfig",
			fileContent:  "",
			expectedKeys: map[string]string{},
			expectError:  false,
		},
		{
			name: "OnlyComments",
			fileContent: `# Comment 1
# Comment 2
# Comment 3`,
			expectedKeys: map[string]string{},
			expectError:  false,
		},
		{
			name: "MixedValidAndInvalid",
			fileContent: `VALID_KEY=valid_value
INVALID LINE WITHOUT EQUALS
ANOTHER_VALID=another_value`,
			expectedKeys: map[string]string{
				"VALID_KEY":     "valid_value",
				"ANOTHER_VALID": "another_value",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "test-config")

			err := os.WriteFile(configPath, []byte(tt.fileContent), 0600)
			require.NoError(t, err)

			// Load config
			config, err := loadConfigFile(configPath)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedKeys, config)
			}
		})
	}
}

func TestLoadConfigFile_NonExistentFile(t *testing.T) {
	config, err := loadConfigFile("/non/existent/path/config")

	assert.Error(t, err)
	assert.Empty(t, config)
}

func TestLoadSavedConfig_Success(t *testing.T) {
	// Create temporary home directory
	tmpHome := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	// Set temporary home
	os.Setenv("HOME", tmpHome)

	// Create config directory and file
	configDir := filepath.Join(tmpHome, ".sloth-kubernetes")
	err := os.MkdirAll(configDir, 0755)
	require.NoError(t, err)

	configContent := `TEST_KEY_1=test_value_1
TEST_KEY_2=test_value_2
AWS_REGION=us-east-1`

	configPath := filepath.Join(configDir, "config")
	err = os.WriteFile(configPath, []byte(configContent), 0600)
	require.NoError(t, err)

	// Clear environment variables
	os.Unsetenv("TEST_KEY_1")
	os.Unsetenv("TEST_KEY_2")
	os.Unsetenv("AWS_REGION")

	// Load config
	err = LoadSavedConfig()
	assert.NoError(t, err)

	// Verify environment variables were set
	assert.Equal(t, "test_value_1", os.Getenv("TEST_KEY_1"))
	assert.Equal(t, "test_value_2", os.Getenv("TEST_KEY_2"))
	assert.Equal(t, "us-east-1", os.Getenv("AWS_REGION"))

	// Cleanup
	os.Unsetenv("TEST_KEY_1")
	os.Unsetenv("TEST_KEY_2")
	os.Unsetenv("AWS_REGION")
}

func TestLoadSavedConfig_NoConfigFile(t *testing.T) {
	// Create temporary home directory without config
	tmpHome := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	os.Setenv("HOME", tmpHome)

	// Should not error when config file doesn't exist
	err := LoadSavedConfig()
	assert.NoError(t, err)
}

func TestLoadSavedConfig_EnvVarsTakePrecedence(t *testing.T) {
	// Create temporary home directory
	tmpHome := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	os.Setenv("HOME", tmpHome)

	// Create config directory and file
	configDir := filepath.Join(tmpHome, ".sloth-kubernetes")
	err := os.MkdirAll(configDir, 0755)
	require.NoError(t, err)

	configContent := `PRECEDENCE_TEST=from_config_file`

	configPath := filepath.Join(configDir, "config")
	err = os.WriteFile(configPath, []byte(configContent), 0600)
	require.NoError(t, err)

	// Set environment variable - this should NOT be overridden
	os.Setenv("PRECEDENCE_TEST", "from_environment")

	// Load config
	err = LoadSavedConfig()
	assert.NoError(t, err)

	// Verify environment variable was NOT overridden (env takes precedence)
	assert.Equal(t, "from_environment", os.Getenv("PRECEDENCE_TEST"))

	// Cleanup
	os.Unsetenv("PRECEDENCE_TEST")
}

func TestLoadSavedCredentials_Deprecated(t *testing.T) {
	// This function is deprecated but should still work
	tmpHome := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	os.Setenv("HOME", tmpHome)

	// Should not error
	err := LoadSavedCredentials()
	assert.NoError(t, err)
}

func TestGetCredentialsStatus_FileExists(t *testing.T) {
	tmpHome := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	os.Setenv("HOME", tmpHome)

	// Create credentials file
	credsDir := filepath.Join(tmpHome, ".sloth-kubernetes")
	err := os.MkdirAll(credsDir, 0755)
	require.NoError(t, err)

	credsPath := filepath.Join(credsDir, "credentials")
	err = os.WriteFile(credsPath, []byte("test credentials"), 0600)
	require.NoError(t, err)

	// Check status
	exists, path, err := GetCredentialsStatus()

	assert.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, credsPath, path)
}

func TestGetCredentialsStatus_FileDoesNotExist(t *testing.T) {
	tmpHome := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	os.Setenv("HOME", tmpHome)

	// Check status without creating file
	exists, path, err := GetCredentialsStatus()

	assert.NoError(t, err)
	assert.False(t, exists)
	assert.Contains(t, path, ".sloth-kubernetes/credentials")
}

func TestLoadConfigFile_SpecialCharacters(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "special-config")

	content := `PASSWORD=P@ssw0rd!#$%^&*()
URL=https://example.com:8080/path?query=value
JSON={"key":"value","nested":{"data":true}}
BASE64=YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXo=`

	err := os.WriteFile(configPath, []byte(content), 0600)
	require.NoError(t, err)

	config, err := loadConfigFile(configPath)

	assert.NoError(t, err)
	assert.Equal(t, "P@ssw0rd!#$%^&*()", config["PASSWORD"])
	assert.Equal(t, "https://example.com:8080/path?query=value", config["URL"])
	assert.Equal(t, `{"key":"value","nested":{"data":true}}`, config["JSON"])
	assert.Equal(t, "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXo=", config["BASE64"])
}

func TestLoadConfigFile_EmptyValues(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "empty-config")

	content := `KEY_WITH_VALUE=value
KEY_WITH_EMPTY_VALUE=
KEY_WITH_WHITESPACE=
KEY_WITH_QUOTES=""`

	err := os.WriteFile(configPath, []byte(content), 0600)
	require.NoError(t, err)

	config, err := loadConfigFile(configPath)

	assert.NoError(t, err)
	assert.Equal(t, "value", config["KEY_WITH_VALUE"])
	assert.Equal(t, "", config["KEY_WITH_EMPTY_VALUE"])
	assert.Equal(t, "", config["KEY_WITH_WHITESPACE"])
	assert.Equal(t, "", config["KEY_WITH_QUOTES"])
}

func TestLoadConfigFile_LongLines(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "long-config")

	longValue := ""
	for i := 0; i < 10000; i++ {
		longValue += "a"
	}

	content := "LONG_KEY=" + longValue

	err := os.WriteFile(configPath, []byte(content), 0600)
	require.NoError(t, err)

	config, err := loadConfigFile(configPath)

	assert.NoError(t, err)
	assert.Contains(t, config, "LONG_KEY")
}

func TestLoadSavedConfig_MultipleKeys(t *testing.T) {
	tmpHome := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)

	os.Setenv("HOME", tmpHome)

	// Create config with many keys
	configDir := filepath.Join(tmpHome, ".sloth-kubernetes")
	err := os.MkdirAll(configDir, 0755)
	require.NoError(t, err)

	configContent := `TEST_MULTI_1=value1
TEST_MULTI_2=value2
TEST_MULTI_3=value3
TEST_MULTI_4=value4
TEST_MULTI_5=value5
TEST_MULTI_6=value6
TEST_MULTI_7=value7
TEST_MULTI_8=value8
TEST_MULTI_9=value9
TEST_MULTI_10=value10`

	configPath := filepath.Join(configDir, "config")
	err = os.WriteFile(configPath, []byte(configContent), 0600)
	require.NoError(t, err)

	// Clear all test keys
	os.Unsetenv("TEST_MULTI_1")
	os.Unsetenv("TEST_MULTI_2")
	os.Unsetenv("TEST_MULTI_3")
	os.Unsetenv("TEST_MULTI_10")

	// Load config
	err = LoadSavedConfig()
	assert.NoError(t, err)

	// Verify all keys were set
	assert.Equal(t, "value1", os.Getenv("TEST_MULTI_1"))
	assert.Equal(t, "value2", os.Getenv("TEST_MULTI_2"))
	assert.Equal(t, "value10", os.Getenv("TEST_MULTI_10"))

	// Cleanup
	os.Unsetenv("TEST_MULTI_1")
	os.Unsetenv("TEST_MULTI_2")
	os.Unsetenv("TEST_MULTI_3")
	os.Unsetenv("TEST_MULTI_10")
}
