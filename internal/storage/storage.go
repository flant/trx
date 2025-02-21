package storage

import (
	"trx/internal/config"
	local "trx/internal/storage/local"
)

type Storage interface {
	CheckLastSucceedTag() (string, error)
	StoreSucceedTag(commit string) error
	StoreLockInfo() error
	RemoveLockInfo() error
	CheckLock() error
	ForceRemoveLockInfo() error
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

func (s *StorageService) CheckLastSucceedTag() (string, error) {
	return s.storage.CheckLastSucceedTag()
}

func (s *StorageService) StoreSucceedTag(commit string) error {
	return s.storage.StoreSucceedTag(commit)
}

func (s *StorageService) StoreLockInfo() error {
	return s.storage.StoreLockInfo()
}

func (s *StorageService) RemoveLockInfo() error {
	return s.storage.RemoveLockInfo()
}

func (s *StorageService) CheckLock() error {
	return s.storage.CheckLock()
}

func (s *StorageService) ForceRemoveLockInfo() error {
	return s.storage.ForceRemoveLockInfo()
}
