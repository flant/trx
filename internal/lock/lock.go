package lock

import (
	"github.com/werf/lockgate"
)

type Locker interface {
	Acquire(lockName string, opts lockgate.AcquireOptions) (bool, lockgate.LockHandle, error)
}
type Manager struct {
	locker Locker
}

func NewManager(locker Locker) *Manager {
	return &Manager{locker: locker}
}

func (m *Manager) Acquire(lockName string) error {
	_, _, err := m.locker.Acquire(lockName, lockgate.AcquireOptions{})
	return err
}
