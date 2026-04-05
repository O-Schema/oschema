package cli

import (
	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "oschema",
		Short: "Open, spec-driven ingestion engine",
		Long:  "oschema normalizes data from webhooks and external APIs into a unified schema using versioned YAML adapter specs.",
	}

	root.AddCommand(newServeCmd())
	root.AddCommand(newSpecsCmd())
	root.AddCommand(newReplayCmd())
	return root
}
