package lock

import (
	"fmt"
)

type Locker struct {
	storage         Storage
	lockingDisbaled bool
}

type Storage interface {
	StoreLockInfo() error
	RemoveLockInfo() error
	CheckLock() error
	ForceRemoveLockInfo() error
}

type ErrLocked struct {
	User      string
	CreatedAt string
}

func (e *ErrLocked) Error() string {
	return fmt.Sprintf("locked by: %s at %s", e.User, e.CreatedAt)
}

func NewLocker(storage Storage, lockingDisbaled bool) *Locker {
	return &Locker{
		storage:         storage,
		lockingDisbaled: lockingDisbaled}
}

func (l *Locker) CheckLock() error {
	return l.storage.CheckLock()
}

func (l *Locker) Lock() error {
	if l.lockingDisbaled {
		return nil
	}
	return l.storage.StoreLockInfo()
}

func (l *Locker) Unlock() error {
	if l.lockingDisbaled {
		return nil
	}
	return l.storage.RemoveLockInfo()
}

func (l *Locker) ForceUnlock() error {
	return l.storage.ForceRemoveLockInfo()
}
