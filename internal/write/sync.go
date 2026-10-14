package write

import (
	"fmt"

	"github.com/arrowheadmind/memex/internal/spec"
)

// Sync is SPEC §9.5 after the lock: fetch, then fast-forward, rebase and push, or push. It
// returns the state line and its exit code. A dirty tree is never rebased or fast-forwarded.
func (s *Session) Sync() (string, int) {
	notSynced := func(reason string) (string, int) { return "NOT SYNCED: " + reason, spec.ExitNotPushed }
	if op := s.InProgress(); op != "" {
		return notSynced(op + " in progress")
	}
	if s.up == nil {
		return "no upstream", spec.ExitOK
	}
	rebased := false
	var last string
	for attempt := 0; attempt < MaxPushAttempts; attempt++ {
		if err := s.fetch(); err != nil {
			return notSynced(s.reason(err))
		}
		ahead, behind, err := s.counts()
		if err != nil {
			return notSynced(s.reason(err))
		}
		switch {
		case ahead == 0 && behind == 0:
			return "up to date", spec.ExitOK
		case behind > 0 && s.dirty():
			return notSynced("uncommitted changes (run memex save)")
		case ahead == 0:
			if _, err := s.run(s.git, "merge", "--ff-only", "-q", "@{u}"); err != nil {
				return notSynced(s.reason(err))
			}
			return fmt.Sprintf("fast-forwarded %d commits", behind), spec.ExitOK
		case behind > 0:
			if _, err := s.run(s.git, "rebase", "-q", "--no-autostash", "@{u}"); err != nil {
				if !s.rebaseInProgress() {
					return notSynced(s.reason(err))
				}
				paths := ""
				if !timedOut(err) {
					paths = s.conflicted()
				}
				if why := s.abortRebase(); why != "" {
					return notSynced(why)
				}
				if paths == "" {
					return notSynced(s.reason(err))
				}
				return notSynced("rebase conflict in " + paths)
			}
			rebased = true
		}
		if s.Interrupted() {
			return notSynced("interrupted")
		}
		rejected, err := s.push()
		if err == nil {
			if rebased {
				return fmt.Sprintf("rebased and pushed %d commits", ahead), spec.ExitOK
			}
			return fmt.Sprintf("pushed %d commits", ahead), spec.ExitOK
		}
		last = s.reason(err)
		if !rejected || s.Interrupted() {
			break
		}
	}
	return notSynced(last)
}
