package salt

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// PushConfig holds configuration for pushing files to Salt master
type PushConfig struct {
	LocalPath  string   // Local directory path
	RemotePath string   // Remote destination (/srv/salt or /srv/pillar)
	SSHKeyPath string   // SSH private key path
	BastionIP  string   // Bastion host IP (optional)
	MasterIP   string   // Salt master IP
	SSHUser    string   // SSH user (default: root)
	Excludes   []string // Patterns to exclude
	DryRun     bool     // Show what would be transferred
}

// PushResult contains the result of a push operation
type PushResult struct {
	FilesTransferred int
	BytesTransferred int64
	Duration         time.Duration
	Files            []string
	Errors           []string
}

// PushDirectory transfers a local directory to remote Salt master
func PushDirectory(cfg PushConfig) (*PushResult, error) {
	startTime := time.Now()
	result := &PushResult{}

	// Validate local path exists
	info, err := os.Stat(cfg.LocalPath)
	if err != nil {
		return nil, fmt.Errorf("local path error: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("local path must be a directory: %s", cfg.LocalPath)
	}

	// Count files and calculate size
	files, totalSize, err := listFiles(cfg.LocalPath, cfg.Excludes)
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

	// Create temporary archive
	archivePath, err := createTarGz(cfg.LocalPath, cfg.Excludes)
	if err != nil {
		return nil, fmt.Errorf("failed to create archive: %w", err)
	}
	defer os.Remove(archivePath)

	// Generate unique remote temp path
	remoteTmpPath := fmt.Sprintf("/tmp/salt-push-%s.tar.gz", uuid.New().String()[:8])

	// SCP archive to remote
	if err := scpToRemote(cfg, archivePath, remoteTmpPath); err != nil {
		return nil, fmt.Errorf("failed to transfer archive: %w", err)
	}

	// Extract on remote and cleanup
	extractCmd := fmt.Sprintf(
		"mkdir -p %s && tar -xzf %s -C %s && rm -f %s",
		cfg.RemotePath,
		remoteTmpPath,
		cfg.RemotePath,
		remoteTmpPath,
	)
	if err := sshExec(cfg, extractCmd); err != nil {
		return nil, fmt.Errorf("failed to extract archive on remote: %w", err)
	}

	result.Duration = time.Since(startTime)
	return result, nil
}

// listFiles returns all files in a directory with their total size
func listFiles(root string, excludes []string) ([]string, int64, error) {
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

// createTarGz creates a tar.gz archive of a directory
func createTarGz(sourceDir string, excludes []string) (string, error) {
	// Create temp file
	tmpFile, err := os.CreateTemp("", "salt-push-*.tar.gz")
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	// Create gzip writer
	gzWriter := gzip.NewWriter(tmpFile)
	defer gzWriter.Close()

	// Create tar writer
	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	// Walk directory and add files
	err = filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		// Skip root
		if relPath == "." {
			return nil
		}

		// Check excludes
		for _, pattern := range excludes {
			matched, _ := filepath.Match(pattern, relPath)
			if matched {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// Create tar header
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = relPath

		// Handle symlinks
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			header.Linkname = link
		}

		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		// Write file content
		if !info.IsDir() && info.Mode().IsRegular() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			if _, err := io.Copy(tarWriter, file); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		os.Remove(tmpFile.Name())
		return "", err
	}

	return tmpFile.Name(), nil
}

// scpToRemote transfers a file to remote host via SCP
func scpToRemote(cfg PushConfig, localPath, remotePath string) error {
	args := []string{
		"-T", // Disable pseudo-terminal allocation
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"-i", cfg.SSHKeyPath,
	}

	// Add ProxyCommand if bastion is configured
	if cfg.BastionIP != "" {
		proxyCmd := fmt.Sprintf(
			"ssh -i %s -o BatchMode=yes -o StrictHostKeyChecking=accept-new -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR -W %%h:%%p root@%s",
			cfg.SSHKeyPath, cfg.BastionIP,
		)
		args = append(args, "-o", fmt.Sprintf("ProxyCommand=%s", proxyCmd))
	}

	sshUser := cfg.SSHUser
	if sshUser == "" {
		sshUser = "root"
	}

	args = append(args, localPath, fmt.Sprintf("%s@%s:%s", sshUser, cfg.MasterIP, remotePath))

	cmd := exec.Command("scp", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("scp failed: %w, output: %s", err, string(output))
	}

	return nil
}

// sshExec executes a command on remote host via SSH
func sshExec(cfg PushConfig, command string) error {
	args := []string{
		"-i", cfg.SSHKeyPath,
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
	}

	// Add ProxyCommand if bastion is configured
	if cfg.BastionIP != "" {
		proxyCmd := fmt.Sprintf(
			"ssh -i %s -o StrictHostKeyChecking=accept-new -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR -W %%h:%%p root@%s",
			cfg.SSHKeyPath, cfg.BastionIP,
		)
		args = append(args, "-o", fmt.Sprintf("ProxyCommand=%s", proxyCmd))
	}

	sshUser := cfg.SSHUser
	if sshUser == "" {
		sshUser = "root"
	}

	args = append(args, fmt.Sprintf("%s@%s", sshUser, cfg.MasterIP), command)

	cmd := exec.Command("ssh", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ssh exec failed: %w, output: %s", err, string(output))
	}

	return nil
}
