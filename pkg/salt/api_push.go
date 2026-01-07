package salt

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// APIPushConfig holds configuration for pushing files via Salt API
type APIPushConfig struct {
	Client     *Client  // Salt API client
	LocalPath  string   // Local directory path
	RemotePath string   // Remote destination (/srv/salt or /srv/pillar)
	Excludes   []string // Patterns to exclude
	DryRun     bool     // Show what would be transferred
}

// APIPushResult contains the result of an API push operation
type APIPushResult struct {
	FilesTransferred int
	BytesTransferred int64
	Duration         time.Duration
	Files            []string
	Errors           []string
}

// PushDirectoryViaAPI transfers files to Salt master using Salt API
// Note: This uses SSH to the Salt master directly since the master typically
// doesn't run as a minion. The Salt API URL is used to determine the master IP.
func PushDirectoryViaAPI(cfg APIPushConfig) (*APIPushResult, error) {
	startTime := time.Now()
	result := &APIPushResult{}

	// Validate local path exists
	info, err := os.Stat(cfg.LocalPath)
	if err != nil {
		return nil, fmt.Errorf("local path error: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("local path must be a directory: %s", cfg.LocalPath)
	}

	// Collect files to transfer
	files, totalSize, err := collectFiles(cfg.LocalPath, cfg.Excludes)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	result.Files = files
	result.FilesTransferred = len(files)
	result.BytesTransferred = totalSize

	if cfg.DryRun {
		result.Duration = time.Since(startTime)
		return result, nil
	}

	if len(files) == 0 {
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Get SSH config from client
	sshCfg := cfg.Client.GetSSHConfig()
	if sshCfg == nil {
		return nil, fmt.Errorf("SSH configuration not available - ensure stack has SSH key configured")
	}

	// Create PushConfig for SSH-based transfer
	pushCfg := PushConfig{
		LocalPath:  cfg.LocalPath,
		RemotePath: cfg.RemotePath,
		SSHKeyPath: sshCfg.KeyPath,
		MasterIP:   sshCfg.Host,
		SSHUser:    sshCfg.User,
		Excludes:   cfg.Excludes,
		DryRun:     cfg.DryRun,
	}

	// Use SSH-based push
	sshResult, err := PushDirectory(pushCfg)
	if err != nil {
		return nil, err
	}

	result.Duration = sshResult.Duration
	result.Errors = sshResult.Errors
	return result, nil
}

// collectFiles returns all files in a directory with their total size
func collectFiles(root string, excludes []string) ([]string, int64, error) {
	var files []string
	var totalSize int64

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		// Check excludes
		for _, pattern := range excludes {
			matched, _ := filepath.Match(pattern, relPath)
			if matched {
				return nil
			}
			// Also check if pattern matches any part of the path
			if strings.Contains(relPath, pattern) {
				return nil
			}
		}

		files = append(files, relPath)
		totalSize += info.Size()
		return nil
	})

	return files, totalSize, err
}

// getMasterMinionID finds the minion ID of the Salt master
func getMasterMinionID(client *Client) (string, error) {
	// Get list of minions
	minions, err := client.GetMinions()
	if err != nil {
		return "", err
	}

	// Look for common master minion patterns
	masterPatterns := []string{"bastion", "salt-master", "master"}

	for _, minion := range minions {
		minionLower := strings.ToLower(minion)
		for _, pattern := range masterPatterns {
			if strings.Contains(minionLower, pattern) {
				return minion, nil
			}
		}
	}

	// If no pattern match, try the first minion or return the master itself
	if len(minions) > 0 {
		// The bastion typically has the salt-minion pointing to localhost
		// Try to find it by checking which minion has /srv/salt
		for _, minion := range minions {
			resp, err := client.RunCommand(minion, "file.directory_exists", []string{"/srv/salt"})
			if err == nil && resp != nil && len(resp.Return) > 0 {
				// Check if this minion has /srv/salt
				if result, ok := resp.Return[0][minion]; ok {
					if exists, ok := result.(bool); ok && exists {
						return minion, nil
					}
				}
			}
		}
		// Fallback to first minion
		return minions[0], nil
	}

	return "", fmt.Errorf("no minions found")
}
