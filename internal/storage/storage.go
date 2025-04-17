package storage

import (
	"trx/internal/config"
	local "trx/internal/storage/local"
)

type Storage interface {
	CheckTaskLastSucceedTag(taskName string) (string, error)
	StoreTaskSucceedTag(taskName, commit string) error
}

type StorageService struct {
	storage Storage
}

type StorageOpts struct {
	Config      *config.Config
	StorageType string
}

func NewStorage(opts *StorageOpts) (*StorageService, error) {
	switch opts.StorageType {
	case "local":
		return &StorageService{storage: local.NewLocalStorage(opts.Config.Repo.Url)}, nil
	default:
		return &StorageService{storage: local.NewLocalStorage(opts.Config.Repo.Url)}, nil
	}
}

func (s *StorageService) CheckTaskLastSucceedTag(taskName string) (string, error) {
	return s.storage.CheckTaskLastSucceedTag(taskName)
}

func (s *StorageService) StoreTaskSucceedTag(taskName, commit string) error {
	return s.storage.StoreTaskSucceedTag(taskName, commit)
}
