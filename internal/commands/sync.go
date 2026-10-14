package commands

import (
	"fmt"
	"time"

	"github.com/arrowheadmind/memex/internal/write"
)

// syncState runs §9.5 under the lock, waiting up to wait, and returns the time of the fetch
// and the state line with its exit code. A lock that stays held is a *write.LockError.
func syncState(e *Env, root string, timeout int, wait time.Duration) (string, string, int, error) {
	s, err := write.Begin(root, timeout, wait)
	if err != nil {
		return "", "", 0, err
	}
	defer s.End()
	at := e.Now().UTC().Format(time.RFC3339)
	state, code := s.Sync()
	return at, state, code, nil
}

// Sync runs `memex sync` (SPEC §9.5): exactly two lines, the fetch time and the state line.
func Sync(e *Env) int {
	cfg, _, root, err := e.openStore()
	if err != nil {
		return e.failErr(err)
	}
	if code, ok := e.checkManifest(root); !ok {
		return code
	}
	if code, ok := e.checkCommitted(root); !ok {
		return code
	}
	at, state, code, err := syncState(e, root, cfg.NetworkTimeout, write.LockWait)
	if err != nil {
		return e.beginFailed(err)
	}
	fmt.Fprintf(e.Stdout, "fetched %s\n%s\n", at, state)
	return code
}
