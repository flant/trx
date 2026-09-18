package main

import (
	"log"
	"time"

	"github.com/spf13/cobra"
)

var (
	configPath  string
	force       bool
	disableLock bool
	lockTimeout time.Duration
)

type runOptions struct {
	cmdFromCli []string
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "trx",
		Short: "Runs quorum validation and runs specified command",
		Long: `trx is a tool for quorum verification and command execution in a Git repository.

By default, it uses the ./trx.yaml configuration file, but you can specify a different path using the --config flag.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := run(runOptions{cmdFromCli: getCommandFromCli(cmd, args)}); err != nil {
				return err
			}
			return nil
		},
	}

	rootCmd.SilenceUsage = true
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "./trx.yaml", "Path to config file")
	rootCmd.Flags().BoolVarP(&force, "force", "f", false, "Force execution if no new version found, or if the tag already failed")
	rootCmd.Flags().BoolVarP(&disableLock, "disable-lock", "", false, "Disable execution locking")
	rootCmd.Flags().DurationVar(&lockTimeout, "lock-timeout", 0, "Give up waiting for the execution lock after this duration (0 waits forever)")

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func getCommandFromCli(cmd *cobra.Command, args []string) []string {
	argsLenAtDash := cmd.ArgsLenAtDash()
	if argsLenAtDash >= 0 {
		return args[argsLenAtDash:]
	}
	return []string{}
}
