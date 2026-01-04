package main

import (
	"fmt"

	"github.com/chalkan3/sloth-kubernetes/internal/orchestrator"
	"github.com/chalkan3/sloth-kubernetes/pkg/config"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Load config
		loader := config.NewLoader("config/cluster-config.lisp")
		clusterConfig, err := loader.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		ctx.Log.Info("Starting REAL Kubernetes cluster deployment: WireGuard + RKE2 + DNS", nil)

		// Create SIMPLE orchestrator with ONLY REAL implementations (no mocks)
		// Pass empty strings for lispManifest and previousMeta since this is a standalone entry point
		_, err = orchestrator.NewSimpleRealOrchestratorComponent(ctx, "kubernetes-cluster", clusterConfig, "", "")
		if err != nil {
			return fmt.Errorf("failed to create orchestrator: %w", err)
		}

		return nil
	})
}
