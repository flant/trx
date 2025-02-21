package main

import (
	"log"

	"github.com/spf13/cobra"
)

var (
	configPath  string
	force       bool
	disableLock bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "trx",
		Short: "Runs quorum validation and runs specified command",
		Long: `trx is a tool for quorum verification and command execution in a Git repository.

By default, it uses the ./trx.yaml configuration file, but you can specify a different path using the --config flag.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := run(); err != nil {
				return err
			}
			return nil
		},
	}

	rootCmd.SilenceUsage = true
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "./trx.yaml", "Path to config file")
	rootCmd.Flags().BoolVarP(&force, "force", "f", false, "Force execution if no new version found")
	rootCmd.Flags().BoolVarP(&disableLock, "disable-lock", "", false, "Disbale execution locking")

	rootCmd.AddCommand(forceUnlockCmd())

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
