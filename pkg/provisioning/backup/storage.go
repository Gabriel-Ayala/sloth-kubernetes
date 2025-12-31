package backup

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// =============================================================================
// Storage Factory
// =============================================================================

// StorageFactory creates storage backends
type StorageFactory struct {
	creators map[string]StorageCreator
	mu       sync.RWMutex
}

// StorageCreator creates a storage backend
type StorageCreator func(config map[string]string) (Storage, error)

// Storage interface for backup storage backends
type Storage interface {
	// Name returns storage name
	Name() string
	// Upload uploads backup data
	Upload(ctx context.Context, key string, data []byte) error
	// Download downloads backup data
	Download(ctx context.Context, key string) ([]byte, error)
	// Delete deletes backup data
	Delete(ctx context.Context, key string) error
	// List lists available backups
	List(ctx context.Context) ([]string, error)
}

// NewStorageFactory creates a new storage factory
func NewStorageFactory() *StorageFactory {
	factory := &StorageFactory{
		creators: make(map[string]StorageCreator),
	}

	// Register default storage backends
	factory.Register("local", NewLocalStorage)
	factory.Register("s3", NewS3Storage)
	factory.Register("gcs", NewGCSStorage)

	return factory
}

// Register adds a storage creator
func (f *StorageFactory) Register(name string, creator StorageCreator) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.creators[name] = creator
}

// Create creates a storage backend
func (f *StorageFactory) Create(storageType string, config map[string]string) (Storage, error) {
	f.mu.RLock()
	creator, exists := f.creators[storageType]
	f.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("unknown storage type: %s", storageType)
	}

	return creator(config)
}

// =============================================================================
// Local Storage Backend
// =============================================================================

// LocalStorage stores backups on local filesystem
type LocalStorage struct {
	basePath string
}

// NewLocalStorage creates a new local storage backend
func NewLocalStorage(config map[string]string) (Storage, error) {
	basePath := config["path"]
	if basePath == "" {
		basePath = "/var/lib/sloth-kubernetes/backups"
	}

	// Ensure directory exists
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}

	return &LocalStorage{basePath: basePath}, nil
}

// Name returns storage name
func (s *LocalStorage) Name() string {
	return "local:" + s.basePath
}

// Upload stores data locally
func (s *LocalStorage) Upload(ctx context.Context, key string, data []byte) error {
	filePath := filepath.Join(s.basePath, key)

	// Ensure parent directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write file
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// Download retrieves data locally
func (s *LocalStorage) Download(ctx context.Context, key string) ([]byte, error) {
	filePath := filepath.Join(s.basePath, key)

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return data, nil
}

// Delete removes data locally
func (s *LocalStorage) Delete(ctx context.Context, key string) error {
	filePath := filepath.Join(s.basePath, key)

	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// List returns all backup keys
func (s *LocalStorage) List(ctx context.Context) ([]string, error) {
	var keys []string

	err := filepath.Walk(s.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			relPath, _ := filepath.Rel(s.basePath, path)
			keys = append(keys, relPath)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list backups: %w", err)
	}

	return keys, nil
}

// =============================================================================
// S3 Storage Backend
// =============================================================================

// S3Storage stores backups in AWS S3
type S3Storage struct {
	bucket    string
	region    string
	prefix    string
	endpoint  string
	accessKey string
	secretKey string
	client    S3Client
}

// S3Client interface for S3 operations
type S3Client interface {
	PutObject(ctx context.Context, bucket, key string, data io.Reader) error
	GetObject(ctx context.Context, bucket, key string) ([]byte, error)
	DeleteObject(ctx context.Context, bucket, key string) error
	ListObjects(ctx context.Context, bucket, prefix string) ([]string, error)
}

// NewS3Storage creates a new S3 storage backend
func NewS3Storage(config map[string]string) (Storage, error) {
	bucket := config["bucket"]
	if bucket == "" {
		return nil, fmt.Errorf("S3 bucket is required")
	}

	return &S3Storage{
		bucket:    bucket,
		region:    config["region"],
		prefix:    config["prefix"],
		endpoint:  config["endpoint"],
		accessKey: config["access_key"],
		secretKey: config["secret_key"],
	}, nil
}

// Name returns storage name
func (s *S3Storage) Name() string {
	return fmt.Sprintf("s3://%s/%s", s.bucket, s.prefix)
}

// Upload stores data in S3
func (s *S3Storage) Upload(ctx context.Context, key string, data []byte) error {
	fullKey := s.prefix + "/" + key

	if s.client != nil {
		return s.client.PutObject(ctx, s.bucket, fullKey, bytes.NewReader(data))
	}

	// Placeholder - actual implementation would use AWS SDK
	return fmt.Errorf("S3 client not configured")
}

// Download retrieves data from S3
func (s *S3Storage) Download(ctx context.Context, key string) ([]byte, error) {
	fullKey := s.prefix + "/" + key

	if s.client != nil {
		return s.client.GetObject(ctx, s.bucket, fullKey)
	}

	return nil, fmt.Errorf("S3 client not configured")
}

// Delete removes data from S3
func (s *S3Storage) Delete(ctx context.Context, key string) error {
	fullKey := s.prefix + "/" + key

	if s.client != nil {
		return s.client.DeleteObject(ctx, s.bucket, fullKey)
	}

	return fmt.Errorf("S3 client not configured")
}

// List returns all backup keys from S3
func (s *S3Storage) List(ctx context.Context) ([]string, error) {
	if s.client != nil {
		return s.client.ListObjects(ctx, s.bucket, s.prefix)
	}

	return nil, fmt.Errorf("S3 client not configured")
}

// SetClient sets the S3 client (for dependency injection)
func (s *S3Storage) SetClient(client S3Client) {
	s.client = client
}

// =============================================================================
// GCS Storage Backend
// =============================================================================

// GCSStorage stores backups in Google Cloud Storage
type GCSStorage struct {
	bucket      string
	prefix      string
	credentials string
	client      GCSClient
}

// GCSClient interface for GCS operations
type GCSClient interface {
	Upload(ctx context.Context, bucket, key string, data []byte) error
	Download(ctx context.Context, bucket, key string) ([]byte, error)
	Delete(ctx context.Context, bucket, key string) error
	List(ctx context.Context, bucket, prefix string) ([]string, error)
}

// NewGCSStorage creates a new GCS storage backend
func NewGCSStorage(config map[string]string) (Storage, error) {
	bucket := config["bucket"]
	if bucket == "" {
		return nil, fmt.Errorf("GCS bucket is required")
	}

	return &GCSStorage{
		bucket:      bucket,
		prefix:      config["prefix"],
		credentials: config["credentials"],
	}, nil
}

// Name returns storage name
func (s *GCSStorage) Name() string {
	return fmt.Sprintf("gs://%s/%s", s.bucket, s.prefix)
}

// Upload stores data in GCS
func (s *GCSStorage) Upload(ctx context.Context, key string, data []byte) error {
	fullKey := s.prefix + "/" + key

	if s.client != nil {
		return s.client.Upload(ctx, s.bucket, fullKey, data)
	}

	return fmt.Errorf("GCS client not configured")
}

// Download retrieves data from GCS
func (s *GCSStorage) Download(ctx context.Context, key string) ([]byte, error) {
	fullKey := s.prefix + "/" + key

	if s.client != nil {
		return s.client.Download(ctx, s.bucket, fullKey)
	}

	return nil, fmt.Errorf("GCS client not configured")
}

// Delete removes data from GCS
func (s *GCSStorage) Delete(ctx context.Context, key string) error {
	fullKey := s.prefix + "/" + key

	if s.client != nil {
		return s.client.Delete(ctx, s.bucket, fullKey)
	}

	return fmt.Errorf("GCS client not configured")
}

// List returns all backup keys from GCS
func (s *GCSStorage) List(ctx context.Context) ([]string, error) {
	if s.client != nil {
		return s.client.List(ctx, s.bucket, s.prefix)
	}

	return nil, fmt.Errorf("GCS client not configured")
}

// SetClient sets the GCS client
func (s *GCSStorage) SetClient(client GCSClient) {
	s.client = client
}

// =============================================================================
// Backup Components
// =============================================================================

// EtcdComponent backs up etcd data
type EtcdComponent struct {
	etcdEndpoints []string
	certFile      string
	keyFile       string
	caFile        string
	executor      CommandExecutor
}

// CommandExecutor executes commands
type CommandExecutor interface {
	Execute(ctx context.Context, command string, args ...string) (string, error)
}

// NewEtcdComponent creates a new etcd backup component
func NewEtcdComponent(endpoints []string, executor CommandExecutor) *EtcdComponent {
	return &EtcdComponent{
		etcdEndpoints: endpoints,
		executor:      executor,
	}
}

// Name returns component name
func (c *EtcdComponent) Name() string {
	return "etcd"
}

// Backup creates etcd snapshot
func (c *EtcdComponent) Backup(ctx context.Context) ([]byte, error) {
	// Create temporary snapshot file
	tmpFile := fmt.Sprintf("/tmp/etcd-snapshot-%d.db", time.Now().Unix())

	args := []string{
		"snapshot", "save", tmpFile,
		"--endpoints=" + c.etcdEndpoints[0],
	}

	if c.certFile != "" {
		args = append(args, "--cert="+c.certFile)
	}
	if c.keyFile != "" {
		args = append(args, "--key="+c.keyFile)
	}
	if c.caFile != "" {
		args = append(args, "--cacert="+c.caFile)
	}

	if _, err := c.executor.Execute(ctx, "etcdctl", args...); err != nil {
		return nil, fmt.Errorf("failed to create etcd snapshot: %w", err)
	}

	// Read snapshot file
	data, err := os.ReadFile(tmpFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read snapshot file: %w", err)
	}

	// Cleanup
	os.Remove(tmpFile)

	return data, nil
}

// Restore restores etcd from snapshot
func (c *EtcdComponent) Restore(ctx context.Context, data []byte) error {
	// Write snapshot to temp file
	tmpFile := fmt.Sprintf("/tmp/etcd-restore-%d.db", time.Now().Unix())
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write snapshot file: %w", err)
	}

	// Restore from snapshot
	args := []string{
		"snapshot", "restore", tmpFile,
		"--data-dir=/var/lib/etcd-restored",
	}

	if _, err := c.executor.Execute(ctx, "etcdctl", args...); err != nil {
		return fmt.Errorf("failed to restore etcd snapshot: %w", err)
	}

	// Cleanup
	os.Remove(tmpFile)

	return nil
}

// SecretsComponent backs up Kubernetes secrets
type SecretsComponent struct {
	kubeconfigPath string
	executor       CommandExecutor
	namespaces     []string
}

// NewSecretsComponent creates a new secrets backup component
func NewSecretsComponent(kubeconfigPath string, executor CommandExecutor) *SecretsComponent {
	return &SecretsComponent{
		kubeconfigPath: kubeconfigPath,
		executor:       executor,
		namespaces:     []string{}, // Empty means all namespaces
	}
}

// Name returns component name
func (c *SecretsComponent) Name() string {
	return "secrets"
}

// Backup exports secrets as YAML
func (c *SecretsComponent) Backup(ctx context.Context) ([]byte, error) {
	args := []string{"get", "secrets", "--all-namespaces", "-o", "yaml"}

	if c.kubeconfigPath != "" {
		args = append([]string{"--kubeconfig", c.kubeconfigPath}, args...)
	}

	output, err := c.executor.Execute(ctx, "kubectl", args...)
	if err != nil {
		return nil, fmt.Errorf("failed to export secrets: %w", err)
	}

	return []byte(output), nil
}

// Restore imports secrets from YAML
func (c *SecretsComponent) Restore(ctx context.Context, data []byte) error {
	// Write to temp file
	tmpFile := fmt.Sprintf("/tmp/secrets-%d.yaml", time.Now().Unix())
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write secrets file: %w", err)
	}

	args := []string{"apply", "-f", tmpFile}
	if c.kubeconfigPath != "" {
		args = append([]string{"--kubeconfig", c.kubeconfigPath}, args...)
	}

	if _, err := c.executor.Execute(ctx, "kubectl", args...); err != nil {
		return fmt.Errorf("failed to restore secrets: %w", err)
	}

	os.Remove(tmpFile)
	return nil
}

// ConfigMapsComponent backs up ConfigMaps
type ConfigMapsComponent struct {
	kubeconfigPath string
	executor       CommandExecutor
}

// NewConfigMapsComponent creates a new ConfigMaps backup component
func NewConfigMapsComponent(kubeconfigPath string, executor CommandExecutor) *ConfigMapsComponent {
	return &ConfigMapsComponent{
		kubeconfigPath: kubeconfigPath,
		executor:       executor,
	}
}

// Name returns component name
func (c *ConfigMapsComponent) Name() string {
	return "configmaps"
}

// Backup exports ConfigMaps as YAML
func (c *ConfigMapsComponent) Backup(ctx context.Context) ([]byte, error) {
	args := []string{"get", "configmaps", "--all-namespaces", "-o", "yaml"}

	if c.kubeconfigPath != "" {
		args = append([]string{"--kubeconfig", c.kubeconfigPath}, args...)
	}

	output, err := c.executor.Execute(ctx, "kubectl", args...)
	if err != nil {
		return nil, fmt.Errorf("failed to export configmaps: %w", err)
	}

	return []byte(output), nil
}

// Restore imports ConfigMaps from YAML
func (c *ConfigMapsComponent) Restore(ctx context.Context, data []byte) error {
	tmpFile := fmt.Sprintf("/tmp/configmaps-%d.yaml", time.Now().Unix())
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return err
	}

	args := []string{"apply", "-f", tmpFile}
	if c.kubeconfigPath != "" {
		args = append([]string{"--kubeconfig", c.kubeconfigPath}, args...)
	}

	if _, err := c.executor.Execute(ctx, "kubectl", args...); err != nil {
		return err
	}

	os.Remove(tmpFile)
	return nil
}
