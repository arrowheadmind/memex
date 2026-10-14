// Package write implements the write protocol of SPEC §9: the lock, freshening, atomic
// writes with read-back, the commit of exact paths, integration with the upstream, and sync.
package write

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
)

// Lock waits (SPEC §9.1 step 1). They are variables only so that tests can shorten them.
var (
	LockWait          = 60 * time.Second
	PreflightLockWait = 10 * time.Second
)

const lockPoll = 100 * time.Millisecond

// LockFile is the store-relative path of the write lock.
const LockFile = ".memex/write.lock"

// LockError is a lock that another process held for the whole wait (exit 4).
type LockError struct {
	Path string
	Wait time.Duration
}

func (e *LockError) Error() string {
	return fmt.Sprintf("the store is locked by another memex; gave up after %ds waiting for %s", int(e.Wait/time.Second), e.Path)
}

// lock takes an exclusive advisory lock on root/.memex/write.lock, creating .memex/ if
// needed, polling until wait has passed. The lock is released when the file is closed.
func lock(root string, wait time.Duration) (*os.File, error) {
	p := filepath.Join(root, filepath.FromSlash(LockFile))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(p, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(wait)
	for {
		err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			return f, nil
		}
		if !errors.Is(err, unix.EWOULDBLOCK) && !errors.Is(err, unix.EINTR) {
			f.Close()
			return nil, err
		}
		if !time.Now().Before(deadline) {
			f.Close()
			return nil, &LockError{Path: p, Wait: wait}
		}
		time.Sleep(lockPoll)
	}
}
