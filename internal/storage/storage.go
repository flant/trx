package storage

import (
	"trx/internal/config"
	local "trx/internal/storage/local"
)

type Storage interface {
	CheckLastSucceedTag() (string, error)
	StoreSucceedTag(commit string) error
}

type StorageService struct {
	storage Storage
}

type StorageOpts struct {
	Config *config.Config
}

func NewStorage(opts *StorageOpts) (*StorageService, error) {
	storage, err := local.NewLocalStorage(opts.Config.Repo.Url)
	if err != nil {
		return nil, err
	}
	return &StorageService{storage: storage}, nil
}

func (s *StorageService) CheckLastSucceedTag() (string, error) {
	return s.storage.CheckLastSucceedTag()
}

func (s *StorageService) StoreSucceedTag(commit string) error {
	return s.storage.StoreSucceedTag(commit)
}
