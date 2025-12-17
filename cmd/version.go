package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("cdp %s\n", Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
