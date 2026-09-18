package lock

import (
	"fmt"
	"log"
	"time"

	"github.com/werf/lockgate"
)

type Locker interface {
	Acquire(lockName string, opts lockgate.AcquireOptions) (bool, lockgate.LockHandle, error)
}
type Manager struct {
	locker   Locker
	disabled bool
	timeout  time.Duration
}

func NewManager(locker Locker, disabled bool, timeout time.Duration) *Manager {
	return &Manager{locker: locker, disabled: disabled, timeout: timeout}
}

// Acquire takes the execution lock and blocks until it is free, or until timeout
// expires if a non-zero timeout is configured.
func (m *Manager) Acquire(lockName string) error {
	if m.disabled {
		log.Println("Processing without execution lock")
		return nil
	}

	acquired, _, err := m.locker.Acquire(lockName, lockgate.AcquireOptions{
		Timeout: m.timeout,
		OnWaitFunc: func(lockName string, doWait func() error) error {
			log.Printf("Waiting for execution lock %q held by another trx instance\n", lockName)
			return doWait()
		},
	})
	if err != nil {
		return err
	}
	if !acquired {
		return fmt.Errorf("execution lock %q is held by another trx instance", lockName)
	}
	return nil
}
