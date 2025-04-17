package main

import (
	"log"

	"github.com/spf13/cobra"
)

var (
	configPath          string
	force               bool
	disableLock         bool
	disableQuorumsCheck bool
	reference           string
	task                string
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
	rootCmd.Flags().BoolVarP(&force, "force", "f", false, "Force execution if no new version found")
	rootCmd.Flags().BoolVarP(&disableLock, "disable-lock", "", false, "Disable execution locking")
	rootCmd.Flags().BoolVarP(&disableQuorumsCheck, "disable-quorums-check", "", false, "Run without checking quorums")
	rootCmd.Flags().StringVarP(&reference, "reference", "r", "", "Tag to run on (default: latest tag)")
	rootCmd.Flags().StringVarP(&task, "task", "t", "", "Name of the task to run. If no name provided use ordinal number e.g. 1,2,3...etc. (default: first task in config)")

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
