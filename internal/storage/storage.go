package storage

import (
	"trx/internal/config"
	local "trx/internal/storage/local"
)

type Storage interface {
	CheckLastSucceedTag() (string, error)
	StoreSucceedTag(commit string) error
	CheckLastFailedTag() (string, error)
	StoreFailedTag(tag string) error
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

func (s *StorageService) CheckLastFailedTag() (string, error) {
	return s.storage.CheckLastFailedTag()
}

func (s *StorageService) StoreFailedTag(tag string) error {
	return s.storage.StoreFailedTag(tag)
}
