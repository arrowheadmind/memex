package store

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/arrowheadmind/memex/internal/gitx"
)

// HistoryReason is the <reason> of a history that cannot be read (SPEC §7, summary currency):
// git's first line of standard error (§9.4), or "git exited with status <n>" when git printed
// none; "timed out after <N>s" for a git command bounded by timeout; "object <oid> is
// missing" for a file version the log names that the repository does not hold.
func HistoryReason(err error, timeout time.Duration) string {
	var ge *gitx.Error
	if !errors.As(err, &ge) {
		return err.Error()
	}
	switch {
	case ge.TimedOut:
		return fmt.Sprintf("timed out after %ds", int(timeout/time.Second))
	case ge.Interrupted:
		return "interrupted"
	}
	if l := gitx.Reason(ge.Stderr); l != "" {
		return l
	}
	return fmt.Sprintf("git exited with status %d", ge.ExitCode)
}

// History holds, for each entry path under the directories it was read for, the file as it
// is in each commit on HEAD's first-parent history that changes it, newest first (SPEC §7,
// summary currency). The first element is the file in HEAD. A path that HEAD does not hold
// has no versions. A commit that deletes or adds the path ends its history, so a file deleted
// and added again starts over at the add, and in a shallow clone the oldest commit it holds
// is where every path starts.
type History struct {
	Versions map[string][][]byte
	// Changed is, for each path with versions, the committer date (YYYY-MM-DD, in the offset
	// the commit records) of the newest commit that changes it: the last change of SPEC §7,
	// W-OPEN-STALE.
	Changed map[string]string
	// Boundary holds the paths whose newest version comes from the boundary commit of a
	// shallow clone: that commit is not a change, so their Changed date is the boundary's,
	// and the last change was on that date or earlier (SPEC §7, W-OPEN-STALE).
	Boundary map[string]bool
	HasHEAD  bool
}

// ReadHistory reads the history of the files under dirs with one `git log` over HEAD's
// first-parent history and one `git cat-file --batch` for the blobs it names. A store that is
// not in a git repository, or has no commits (HEAD is an unborn branch), has an empty
// history; a HEAD that does not resolve otherwise is an error (SPEC §7, summary currency).
func ReadHistory(g *gitx.Git, dirs []string) (*History, error) {
	h := &History{Versions: map[string][][]byte{}, Changed: map[string]string{}, Boundary: map[string]bool{}}
	if _, err := g.Run("rev-parse", "--git-dir"); err != nil {
		return h, nil
	}
	if _, err := g.Run("rev-parse", "--verify", "-q", "HEAD"); err != nil {
		if _, serr := g.Run("symbolic-ref", "-q", "HEAD"); serr == nil {
			return h, nil // an unborn branch: no commits yet
		}
		_, err := g.Run("rev-parse", "--verify", "HEAD")
		if err == nil {
			err = fmt.Errorf("HEAD does not resolve")
		}
		return nil, err
	}
	h.HasHEAD = true
	// In a shallow clone a commit with no parents is the boundary, not a change: a path whose
	// newest version comes from it was last changed on the boundary's date or earlier (SPEC
	// §7, W-OPEN-STALE).
	shallow := false
	if out, err := g.Run("rev-parse", "--is-shallow-repository"); err == nil && strings.TrimSpace(out) == "true" {
		shallow = true
	}
	args := []string{"log", "--first-parent", "--diff-merges=first-parent", "--root", "--raw",
		"--no-abbrev", "--no-renames", "--relative", "-z", "--format=%x01%H %cs %P", "HEAD", "--"}
	out, err := g.Run(append(args, dirs...)...)
	if err != nil {
		return nil, err
	}
	closed := map[string]bool{}
	objects := map[string][]string{} // path -> blob ids, newest first
	var order []string
	date := ""        // the committer date of the commit whose records follow
	boundary := false // that commit is the boundary of a shallow clone
	tokens := strings.Split(out, "\x00")
	for i := 0; i < len(tokens); i++ {
		t := strings.TrimPrefix(tokens[i], "\n")
		if header, ok := strings.CutPrefix(t, "\x01"); ok {
			f := strings.Fields(header) // hash, date, parents
			date, boundary = "", false
			if len(f) >= 2 {
				date = f[1]
				boundary = len(f) == 2 && shallow
			}
			continue
		}
		if !strings.HasPrefix(t, ":") || i+1 >= len(tokens) {
			continue // the empty token after the last record
		}
		// :<old mode> <new mode> <old blob> <new blob> <status>, then the path.
		meta := strings.Fields(t[1:])
		i++
		p := tokens[i]
		if len(meta) != 5 || closed[p] {
			continue
		}
		status := meta[4]
		if status == "D" || (meta[1] != "100644" && meta[1] != "100755") {
			closed[p] = true
			continue
		}
		if _, seen := objects[p]; !seen {
			order = append(order, p)
			h.Changed[p] = date
			if boundary {
				h.Boundary[p] = true
			}
		}
		objects[p] = append(objects[p], meta[3])
		if status == "A" {
			closed[p] = true
		}
	}
	var all []string
	seen := map[string]bool{}
	for _, p := range order {
		for _, o := range objects[p] {
			if !seen[o] {
				seen[o] = true
				all = append(all, o)
			}
		}
	}
	blobs, err := catBlobs(g, all)
	if err != nil {
		return nil, err
	}
	for _, p := range order {
		for _, o := range objects[p] {
			h.Versions[p] = append(h.Versions[p], blobs[o])
		}
	}
	return h, nil
}

// ReadHeadFiles reads, in one ls-tree and one cat-file batch, only the wanted regular files
// that HEAD holds under dir. The caller has already established that HEAD resolves.
func ReadHeadFiles(g *gitx.Git, dir string, wanted map[string]bool) (map[string][]byte, error) {
	out := map[string][]byte{}
	if len(wanted) == 0 {
		return out, nil
	}
	raw, err := g.Run("ls-tree", "-r", "-z", "HEAD", "--", dir)
	if err != nil {
		return nil, err
	}
	objects := map[string]string{}
	var order []string
	for _, rec := range strings.Split(raw, "\x00") {
		meta, p, ok := strings.Cut(rec, "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 3 || fields[1] != "blob" || !wanted[p] {
			continue
		}
		if fields[0] != "100644" && fields[0] != "100755" {
			continue
		}
		objects[p] = fields[2]
		order = append(order, fields[2])
	}
	blobs, err := catBlobs(g, order)
	if err != nil {
		return nil, err
	}
	for p, oid := range objects {
		out[p] = blobs[oid]
	}
	return out, nil
}
