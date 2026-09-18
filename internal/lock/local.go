package lock

import (
	"fmt"

	lock "github.com/werf/common-go/pkg/lock"
	"github.com/werf/lockgate"
)

type Local struct {
	locker Locker
}

func NewLocalLocker() (*Local, error) {
	locker, err := lock.HostLocker()
	if err != nil {
		return nil, fmt.Errorf("unable to create the host locker: %w", err)
	}
	return &Local{locker: locker}, nil
}

func (l *Local) Acquire(lockName string, opts lockgate.AcquireOptions) (bool, lockgate.LockHandle, error) {
	return l.locker.Acquire(lockName, opts)
}
