package cmd

import (
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all deployed clusters",
	Long:  `List all Pulumi stacks (clusters) with their status and last update time`,
	Example: `  # List all clusters
  sloth-kubernetes list

  # This is equivalent to:
  sloth-kubernetes stacks list`,
	RunE: runListStacks,
}

func init() {
	rootCmd.AddCommand(listCmd)
}
