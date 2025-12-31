// Package costs provides infrastructure cost estimation
package costs

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/chalkan3/sloth-kubernetes/pkg/provisioning"
)

// =============================================================================
// Cost Estimator
// =============================================================================

// Estimator calculates infrastructure costs
type Estimator struct {
	providers    map[string]PriceProvider
	cache        *PriceCache
	eventEmitter provisioning.EventEmitter
	mu           sync.RWMutex
}

// EstimatorConfig holds estimator configuration
type EstimatorConfig struct {
	EventEmitter provisioning.EventEmitter
	CacheTTL     time.Duration
}

// NewEstimator creates a new cost estimator
func NewEstimator(cfg *EstimatorConfig) *Estimator {
	cacheTTL := cfg.CacheTTL
	if cacheTTL == 0 {
		cacheTTL = 1 * time.Hour
	}

	estimator := &Estimator{
		providers:    make(map[string]PriceProvider),
		cache:        NewPriceCache(cacheTTL),
		eventEmitter: cfg.EventEmitter,
	}

	// Register default price providers
	estimator.RegisterProvider("aws", NewAWSPriceProvider())
	estimator.RegisterProvider("gcp", NewGCPPriceProvider())
	estimator.RegisterProvider("digitalocean", NewDigitalOceanPriceProvider())
	estimator.RegisterProvider("linode", NewLinodePriceProvider())

	return estimator
}

// RegisterProvider adds a price provider
func (e *Estimator) RegisterProvider(name string, provider PriceProvider) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.providers[name] = provider
}

// EstimateNodeCost estimates cost for a single node
func (e *Estimator) EstimateNodeCost(ctx context.Context, nodeConfig *config.NodeConfig) (*provisioning.CostEstimate, error) {
	e.mu.RLock()
	provider, exists := e.providers[nodeConfig.Provider]
	e.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no price provider for: %s", nodeConfig.Provider)
	}

	// Check cache (include spot status in cache key)
	cacheKey := fmt.Sprintf("%s:%s:%s:%v", nodeConfig.Provider, nodeConfig.Size, nodeConfig.Region, nodeConfig.SpotInstance)
	if cached := e.cache.Get(cacheKey); cached != nil {
		return cached, nil
	}

	// Get instance price
	hourlyPrice, err := provider.GetInstancePrice(ctx, nodeConfig.Size, nodeConfig.Region)
	if err != nil {
		return nil, fmt.Errorf("failed to get instance price: %w", err)
	}

	// Calculate spot savings if applicable
	var spotSavings float64
	isSpot := nodeConfig.SpotInstance
	if isSpot {
		spotPrice, err := provider.GetSpotPrice(ctx, nodeConfig.Size, nodeConfig.Region)
		if err == nil && spotPrice < hourlyPrice {
			spotSavings = (hourlyPrice - spotPrice) / hourlyPrice * 100
			hourlyPrice = spotPrice
		}
	}

	// Get storage price (assume 50GB default)
	storageGB := 50
	storagePrice, _ := provider.GetStoragePrice(ctx, "ssd", nodeConfig.Region)
	monthlyStorage := storagePrice * float64(storageGB)

	estimate := &provisioning.CostEstimate{
		Resource:    nodeConfig.Name,
		HourlyCost:  hourlyPrice,
		MonthlyCost: hourlyPrice*730 + monthlyStorage, // 730 hours per month
		YearlyCost:  (hourlyPrice*730 + monthlyStorage) * 12,
		Currency:    "USD",
		IsSpot:      isSpot,
		SpotSavings: spotSavings,
		Breakdown: map[string]float64{
			"compute": hourlyPrice * 730,
			"storage": monthlyStorage,
		},
	}

	// Cache result
	e.cache.Set(cacheKey, estimate)

	return estimate, nil
}

// EstimateClusterCost estimates total cluster cost
func (e *Estimator) EstimateClusterCost(ctx context.Context, clusterConfig *config.ClusterConfig) (*provisioning.ClusterCostEstimate, error) {
	estimate := &provisioning.ClusterCostEstimate{
		Currency:        "USD",
		NodeCosts:       make([]*provisioning.CostEstimate, 0),
		Recommendations: make([]provisioning.CostRecommendation, 0),
	}

	var totalMonthly, totalYearly, spotSavings float64

	// Estimate node pool costs
	for poolName, pool := range clusterConfig.NodePools {
		for i := 0; i < pool.Count; i++ {
			nodeConfig := &config.NodeConfig{
				Name:         fmt.Sprintf("%s-%d", poolName, i+1),
				Provider:     pool.Provider,
				Size:         pool.Size,
				Region:       pool.Region,
				SpotInstance: pool.SpotInstance,
			}

			nodeCost, err := e.EstimateNodeCost(ctx, nodeConfig)
			if err != nil {
				continue // Skip nodes we can't price
			}

			estimate.NodeCosts = append(estimate.NodeCosts, nodeCost)
			totalMonthly += nodeCost.MonthlyCost
			totalYearly += nodeCost.YearlyCost
			if nodeCost.IsSpot {
				spotSavings += nodeCost.SpotSavings
			}
		}
	}

	// Estimate individual nodes
	for _, node := range clusterConfig.Nodes {
		nodeCost, err := e.EstimateNodeCost(ctx, &node)
		if err != nil {
			continue
		}

		estimate.NodeCosts = append(estimate.NodeCosts, nodeCost)
		totalMonthly += nodeCost.MonthlyCost
		totalYearly += nodeCost.YearlyCost
	}

	// Estimate load balancer cost
	if clusterConfig.LoadBalancer.Type != "" {
		lbCost := e.estimateLoadBalancerCost(ctx, &clusterConfig.LoadBalancer)
		estimate.LoadBalancerCost = lbCost
		totalMonthly += lbCost
		totalYearly += lbCost * 12
	}

	// Estimate network costs (rough estimate)
	estimate.NetworkCost = 10.0 // Base network cost
	totalMonthly += estimate.NetworkCost
	totalYearly += estimate.NetworkCost * 12

	estimate.TotalMonthlyCost = totalMonthly
	estimate.TotalYearlyCost = totalYearly
	estimate.SpotSavings = spotSavings

	// Generate recommendations
	estimate.Recommendations = e.generateRecommendations(ctx, clusterConfig, estimate)

	e.emitEvent("cost_estimate_generated", map[string]interface{}{
		"monthly_cost": totalMonthly,
		"yearly_cost":  totalYearly,
		"node_count":   len(estimate.NodeCosts),
	})

	return estimate, nil
}

// GetCurrentSpend returns estimated current month spending
func (e *Estimator) GetCurrentSpend(ctx context.Context) (float64, error) {
	// This would integrate with cloud provider billing APIs
	// For now, return 0 as placeholder
	return 0, nil
}

// estimateLoadBalancerCost estimates load balancer monthly cost
func (e *Estimator) estimateLoadBalancerCost(ctx context.Context, lb *config.LoadBalancerConfig) float64 {
	// Approximate load balancer costs by provider
	switch lb.Provider {
	case "aws":
		return 16.20 // AWS NLB base cost
	case "gcp":
		return 18.00 // GCP LB base cost
	case "digitalocean":
		return 12.00 // DO LB cost
	case "linode":
		return 10.00 // Linode NodeBalancer
	default:
		return 15.00 // Default estimate
	}
}

// generateRecommendations generates cost optimization recommendations
func (e *Estimator) generateRecommendations(
	ctx context.Context,
	cfg *config.ClusterConfig,
	estimate *provisioning.ClusterCostEstimate,
) []provisioning.CostRecommendation {
	recommendations := make([]provisioning.CostRecommendation, 0)

	// Check for spot instance opportunities
	spotEligibleNodes := 0
	for poolName, pool := range cfg.NodePools {
		if !pool.SpotInstance && isWorkerPool(pool.Roles) {
			spotEligibleNodes += pool.Count
			potentialSavings := estimate.TotalMonthlyCost * 0.3 * float64(pool.Count) / float64(len(estimate.NodeCosts))

			recommendations = append(recommendations, provisioning.CostRecommendation{
				Type:              "spot_usage",
				Description:       fmt.Sprintf("Consider using spot instances for pool '%s'", poolName),
				PotentialSavings:  potentialSavings,
				Resource:          poolName,
				CurrentConfig:     "on-demand",
				RecommendedConfig: "spot/preemptible",
			})
		}
	}

	// Check for right-sizing opportunities
	for _, nodeCost := range estimate.NodeCosts {
		if nodeCost.HourlyCost > 0.5 {
			recommendations = append(recommendations, provisioning.CostRecommendation{
				Type:             "right_sizing",
				Description:      fmt.Sprintf("Review sizing for node '%s'", nodeCost.Resource),
				PotentialSavings: nodeCost.MonthlyCost * 0.2,
				Resource:         nodeCost.Resource,
			})
		}
	}

	// Reserved instances recommendation for long-term clusters
	if estimate.TotalYearlyCost > 5000 {
		recommendations = append(recommendations, provisioning.CostRecommendation{
			Type:              "reserved_instances",
			Description:       "Consider reserved instances for 1-year commitment",
			PotentialSavings:  estimate.TotalYearlyCost * 0.25,
			CurrentConfig:     "on-demand",
			RecommendedConfig: "1-year reserved",
		})
	}

	return recommendations
}

func (e *Estimator) emitEvent(eventType string, data map[string]interface{}) {
	if e.eventEmitter != nil {
		e.eventEmitter.Emit(provisioning.Event{
			Type:      eventType,
			Timestamp: time.Now().Unix(),
			Data:      data,
			Source:    "cost_estimator",
		})
	}
}

func isWorkerPool(roles []string) bool {
	for _, role := range roles {
		if role == "worker" {
			return true
		}
	}
	return false
}

// =============================================================================
// Price Provider Interface
// =============================================================================

// PriceProvider provides pricing information for a cloud provider
type PriceProvider interface {
	Name() string
	GetInstancePrice(ctx context.Context, instanceType, region string) (float64, error)
	GetSpotPrice(ctx context.Context, instanceType, region string) (float64, error)
	GetStoragePrice(ctx context.Context, storageType, region string) (float64, error)
	GetNetworkPrice(ctx context.Context, region string) (float64, error)
}

// =============================================================================
// AWS Price Provider
// =============================================================================

// AWSPriceProvider provides AWS pricing
type AWSPriceProvider struct {
	// In production, would use AWS Pricing API
	instancePrices map[string]map[string]float64
	spotDiscounts  map[string]float64
}

// NewAWSPriceProvider creates a new AWS price provider
func NewAWSPriceProvider() *AWSPriceProvider {
	return &AWSPriceProvider{
		instancePrices: map[string]map[string]float64{
			"us-east-1": {
				"t3.micro":   0.0104,
				"t3.small":   0.0208,
				"t3.medium":  0.0416,
				"t3.large":   0.0832,
				"t3.xlarge":  0.1664,
				"m5.large":   0.096,
				"m5.xlarge":  0.192,
				"m5.2xlarge": 0.384,
				"c5.large":   0.085,
				"c5.xlarge":  0.17,
				"c5.2xlarge": 0.34,
			},
			"us-west-2": {
				"t3.micro":   0.0104,
				"t3.small":   0.0208,
				"t3.medium":  0.0416,
				"t3.large":   0.0832,
				"t3.xlarge":  0.1664,
				"m5.large":   0.096,
				"m5.xlarge":  0.192,
			},
			"eu-west-1": {
				"t3.micro":   0.0114,
				"t3.small":   0.0228,
				"t3.medium":  0.0456,
				"t3.large":   0.0912,
				"m5.large":   0.107,
			},
		},
		spotDiscounts: map[string]float64{
			"t3.micro":   0.70, // 70% off
			"t3.small":   0.65,
			"t3.medium":  0.60,
			"t3.large":   0.65,
			"m5.large":   0.60,
			"m5.xlarge":  0.55,
			"c5.large":   0.65,
		},
	}
}

// Name returns provider name
func (p *AWSPriceProvider) Name() string {
	return "aws"
}

// GetInstancePrice returns hourly price for instance type
func (p *AWSPriceProvider) GetInstancePrice(ctx context.Context, instanceType, region string) (float64, error) {
	prices, ok := p.instancePrices[region]
	if !ok {
		// Fallback to us-east-1 with 10% markup
		prices = p.instancePrices["us-east-1"]
	}

	price, ok := prices[instanceType]
	if !ok {
		return 0, fmt.Errorf("unknown instance type: %s", instanceType)
	}

	return price, nil
}

// GetSpotPrice returns spot price for instance type
func (p *AWSPriceProvider) GetSpotPrice(ctx context.Context, instanceType, region string) (float64, error) {
	basePrice, err := p.GetInstancePrice(ctx, instanceType, region)
	if err != nil {
		return 0, err
	}

	discount, ok := p.spotDiscounts[instanceType]
	if !ok {
		discount = 0.50 // Default 50% discount
	}

	return basePrice * (1 - discount), nil
}

// GetStoragePrice returns price per GB/month for storage
func (p *AWSPriceProvider) GetStoragePrice(ctx context.Context, storageType, region string) (float64, error) {
	switch storageType {
	case "ssd", "gp3":
		return 0.08, nil
	case "hdd", "st1":
		return 0.045, nil
	case "io1":
		return 0.125, nil
	default:
		return 0.10, nil
	}
}

// GetNetworkPrice returns price per GB for network transfer
func (p *AWSPriceProvider) GetNetworkPrice(ctx context.Context, region string) (float64, error) {
	return 0.09, nil // AWS data transfer out price
}

// =============================================================================
// GCP Price Provider
// =============================================================================

// GCPPriceProvider provides GCP pricing
type GCPPriceProvider struct {
	instancePrices map[string]map[string]float64
}

// NewGCPPriceProvider creates a new GCP price provider
func NewGCPPriceProvider() *GCPPriceProvider {
	return &GCPPriceProvider{
		instancePrices: map[string]map[string]float64{
			"us-central1": {
				"e2-micro":    0.0084,
				"e2-small":    0.0168,
				"e2-medium":   0.0335,
				"e2-standard-2": 0.067,
				"e2-standard-4": 0.134,
				"n1-standard-1": 0.0475,
				"n1-standard-2": 0.095,
				"n1-standard-4": 0.19,
			},
		},
	}
}

// Name returns provider name
func (p *GCPPriceProvider) Name() string {
	return "gcp"
}

// GetInstancePrice returns hourly price for instance type
func (p *GCPPriceProvider) GetInstancePrice(ctx context.Context, instanceType, region string) (float64, error) {
	prices, ok := p.instancePrices[region]
	if !ok {
		prices = p.instancePrices["us-central1"]
	}

	price, ok := prices[instanceType]
	if !ok {
		return 0.10, nil // Default price
	}

	return price, nil
}

// GetSpotPrice returns preemptible price
func (p *GCPPriceProvider) GetSpotPrice(ctx context.Context, instanceType, region string) (float64, error) {
	basePrice, _ := p.GetInstancePrice(ctx, instanceType, region)
	return basePrice * 0.2, nil // Preemptible is ~80% cheaper
}

// GetStoragePrice returns price per GB/month
func (p *GCPPriceProvider) GetStoragePrice(ctx context.Context, storageType, region string) (float64, error) {
	return 0.04, nil
}

// GetNetworkPrice returns price per GB
func (p *GCPPriceProvider) GetNetworkPrice(ctx context.Context, region string) (float64, error) {
	return 0.12, nil
}

// =============================================================================
// DigitalOcean Price Provider
// =============================================================================

// DigitalOceanPriceProvider provides DigitalOcean pricing
type DigitalOceanPriceProvider struct {
	dropletPrices map[string]float64
}

// NewDigitalOceanPriceProvider creates a new DO price provider
func NewDigitalOceanPriceProvider() *DigitalOceanPriceProvider {
	return &DigitalOceanPriceProvider{
		dropletPrices: map[string]float64{
			"s-1vcpu-1gb":    0.007,
			"s-1vcpu-2gb":    0.014,
			"s-2vcpu-2gb":    0.021,
			"s-2vcpu-4gb":    0.028,
			"s-4vcpu-8gb":    0.056,
			"s-6vcpu-16gb":   0.111,
			"s-8vcpu-32gb":   0.167,
			"g-2vcpu-8gb":    0.089,
			"g-4vcpu-16gb":   0.179,
			"c-2":            0.050,
			"c-4":            0.100,
		},
	}
}

// Name returns provider name
func (p *DigitalOceanPriceProvider) Name() string {
	return "digitalocean"
}

// GetInstancePrice returns hourly price
func (p *DigitalOceanPriceProvider) GetInstancePrice(ctx context.Context, instanceType, region string) (float64, error) {
	price, ok := p.dropletPrices[instanceType]
	if !ok {
		return 0.05, nil
	}
	return price, nil
}

// GetSpotPrice - DigitalOcean doesn't have spot instances
func (p *DigitalOceanPriceProvider) GetSpotPrice(ctx context.Context, instanceType, region string) (float64, error) {
	return p.GetInstancePrice(ctx, instanceType, region)
}

// GetStoragePrice returns price per GB/month
func (p *DigitalOceanPriceProvider) GetStoragePrice(ctx context.Context, storageType, region string) (float64, error) {
	return 0.10, nil
}

// GetNetworkPrice returns price per GB
func (p *DigitalOceanPriceProvider) GetNetworkPrice(ctx context.Context, region string) (float64, error) {
	return 0.01, nil // First 1TB free, then $0.01/GB
}

// =============================================================================
// Linode Price Provider
// =============================================================================

// LinodePriceProvider provides Linode pricing
type LinodePriceProvider struct {
	linodePrices map[string]float64
}

// NewLinodePriceProvider creates a new Linode price provider
func NewLinodePriceProvider() *LinodePriceProvider {
	return &LinodePriceProvider{
		linodePrices: map[string]float64{
			"g6-nanode-1":   0.0075,
			"g6-standard-1": 0.015,
			"g6-standard-2": 0.030,
			"g6-standard-4": 0.060,
			"g6-standard-6": 0.120,
			"g6-standard-8": 0.240,
		},
	}
}

// Name returns provider name
func (p *LinodePriceProvider) Name() string {
	return "linode"
}

// GetInstancePrice returns hourly price
func (p *LinodePriceProvider) GetInstancePrice(ctx context.Context, instanceType, region string) (float64, error) {
	price, ok := p.linodePrices[instanceType]
	if !ok {
		return 0.03, nil
	}
	return price, nil
}

// GetSpotPrice - Linode doesn't have spot instances
func (p *LinodePriceProvider) GetSpotPrice(ctx context.Context, instanceType, region string) (float64, error) {
	return p.GetInstancePrice(ctx, instanceType, region)
}

// GetStoragePrice returns price per GB/month
func (p *LinodePriceProvider) GetStoragePrice(ctx context.Context, storageType, region string) (float64, error) {
	return 0.10, nil
}

// GetNetworkPrice returns price per GB
func (p *LinodePriceProvider) GetNetworkPrice(ctx context.Context, region string) (float64, error) {
	return 0.01, nil
}

// =============================================================================
// Price Cache
// =============================================================================

// PriceCache caches price lookups
type PriceCache struct {
	cache map[string]*cacheEntry
	ttl   time.Duration
	mu    sync.RWMutex
}

type cacheEntry struct {
	estimate  *provisioning.CostEstimate
	expiresAt time.Time
}

// NewPriceCache creates a new price cache
func NewPriceCache(ttl time.Duration) *PriceCache {
	return &PriceCache{
		cache: make(map[string]*cacheEntry),
		ttl:   ttl,
	}
}

// Get retrieves a cached estimate
func (c *PriceCache) Get(key string) *provisioning.CostEstimate {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.cache[key]
	if !exists || time.Now().After(entry.expiresAt) {
		return nil
	}

	return entry.estimate
}

// Set stores an estimate in cache
func (c *PriceCache) Set(key string, estimate *provisioning.CostEstimate) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache[key] = &cacheEntry{
		estimate:  estimate,
		expiresAt: time.Now().Add(c.ttl),
	}
}

// Clear removes all cached entries
func (c *PriceCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[string]*cacheEntry)
}
