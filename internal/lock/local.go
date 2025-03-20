package lock

import (
	lock "github.com/werf/common-go/pkg/lock"
	"github.com/werf/lockgate"
)

type Local struct {
	locker   Locker
	disabled bool
}

func NewLocalLocker(disabled bool) *Local {
	locker, _ := lock.HostLocker()
	return &Local{locker: locker, disabled: disabled}
}

func (l *Local) Acquire(lockName string, opts lockgate.AcquireOptions) (bool, lockgate.LockHandle, error) {
	return l.locker.Acquire(lockName, lockgate.AcquireOptions{
		NonBlocking: l.disabled,
	})
}
