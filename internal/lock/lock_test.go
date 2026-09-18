package lock

import (
	"testing"

	"github.com/werf/lockgate"
)

type stubLocker struct {
	calls    int
	acquired bool
	opts     lockgate.AcquireOptions
}

func (s *stubLocker) Acquire(lockName string, opts lockgate.AcquireOptions) (bool, lockgate.LockHandle, error) {
	s.calls++
	s.opts = opts
	return s.acquired, lockgate.LockHandle{LockName: lockName}, nil
}

func TestAcquireDisabledDoesNotLock(t *testing.T) {
	locker := &stubLocker{}
	if err := NewManager(locker, true, 0).Acquire("repo"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if locker.calls != 0 {
		t.Fatalf("expected no acquisition attempt, got %d", locker.calls)
	}
}

func TestAcquireLostRaceIsAnError(t *testing.T) {
	locker := &stubLocker{acquired: false}
	if err := NewManager(locker, false, 0).Acquire("repo"); err == nil {
		t.Fatal("expected an error when the lock is held by another instance")
	}
}

func TestAcquireSucceedsAndIsBlocking(t *testing.T) {
	locker := &stubLocker{acquired: true}
	if err := NewManager(locker, false, 0).Acquire("repo"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if locker.opts.NonBlocking {
		t.Fatal("expected a blocking acquisition")
	}
}
