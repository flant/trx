package lock

import (
	"fmt"
	"time"

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

// Acquire blocks until the lock is taken or timeout expires. A zero timeout
// means waiting indefinitely.
func (m *Manager) Acquire(lockName string, timeout time.Duration) error {
	acquired, _, err := m.locker.Acquire(lockName, lockgate.AcquireOptions{Timeout: timeout})
	if err != nil {
		return err
	}
	if !acquired {
		return fmt.Errorf("execution lock %q is held by another trx instance", lockName)
	}
	return nil
}
