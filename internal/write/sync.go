package write

import (
	"fmt"

	"github.com/arrowheadmind/memex/internal/spec"
)

// Sync is SPEC §9.5 after the lock: fetch, then fast-forward, rebase and push, or push. It
// returns the state line and its exit code. A dirty tree is never rebased or fast-forwarded.
func (s *Session) Sync() (string, int) {
	notSynced := func(reason string) (string, int) { return "NOT SYNCED: " + reason, spec.ExitNotPushed }
	if s.up == nil {
		return "no upstream", spec.ExitOK
	}
	if s.rebaseInProgress() {
		return notSynced("rebase in progress")
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
				if s.rebaseInProgress() {
					paths := s.conflicted()
					s.run(s.git, "rebase", "--abort")
					return notSynced("rebase conflict in " + paths)
				}
				return notSynced(s.reason(err))
			}
			rebased = true
		}
		rejected, err := s.push()
		if err == nil {
			if rebased {
				return fmt.Sprintf("rebased and pushed %d commits", ahead), spec.ExitOK
			}
			return fmt.Sprintf("pushed %d commits", ahead), spec.ExitOK
		}
		last = s.reason(err)
		if !rejected {
			break
		}
	}
	return notSynced(last)
}
