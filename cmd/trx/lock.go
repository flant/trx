package main

import (
	"fmt"
	"log"
	"trx/internal/config"
	"trx/internal/lock"
	"trx/internal/storage"

	"github.com/spf13/cobra"
)

func forceUnlockCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "force-unlock",
		Short: "Removes execution lock",
		Long:  "This command removes execution lock. It can be used to unlock the execution in case of an error.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return removeLock()
		},
	}
}

func removeLock() error {
	cfg, err := config.NewConfig(configPath)
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	storage, err := storage.NewStorage(&storage.StorageOpts{
		Config: cfg,
	})
	if err != nil {
		return fmt.Errorf("init storage error: %w", err)
	}

	locker := lock.NewLocker(storage, false)

	if err := locker.ForceUnlock(); err != nil {
		return fmt.Errorf("failed to force remove lock: %w", err)
	}

	log.Println("Lock removed successfully")
	return nil
}
