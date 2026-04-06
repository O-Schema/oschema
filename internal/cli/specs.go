package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newSpecsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "specs",
		Short: "Manage adapter specs",
	}
	cmd.AddCommand(newSpecsListCmd())
	return cmd
}

func newSpecsListCmd() *cobra.Command {
	var specsDir string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all loaded adapter specs",
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, err := loadRegistry(specsDir)
			if err != nil {
				return err
			}

			list := reg.List()
			if len(list) == 0 {
				fmt.Println("No specs loaded.")
				return nil
			}

			fmt.Printf("%-20s %-15s %-20s\n", "SOURCE", "VERSION", "TYPE HEADER")
			fmt.Printf("%-20s %-15s %-20s\n", "------", "-------", "-----------")
			for _, s := range list {
				fmt.Printf("%-20s %-15s %-20s\n", s.Source, s.Version, s.TypeHeader)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&specsDir, "specs-dir", os.Getenv("OSCHEMA_SPECS_DIR"), "additional specs directory")
	return cmd
}
