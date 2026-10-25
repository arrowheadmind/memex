package store

import (
	"strings"

	"github.com/arrowheadmind/memex/internal/gitx"
)

// History holds, for each entry path under the directories it was read for, the file as it
// is in each commit on HEAD's first-parent history that changes it, newest first (SPEC §7,
// summary currency). The first element is the file in HEAD. A path that HEAD does not hold
// has no versions. A commit that deletes or adds the path ends its history, so a file deleted
// and added again starts over at the add, and in a shallow clone the oldest commit it holds
// is where every path starts.
type History struct {
	Versions map[string][][]byte
}

// ReadHistory reads the history of the files under dirs with one `git log` over HEAD's
// first-parent history and one `git cat-file --batch` for the blobs it names. A store without
// commits has an empty history.
func ReadHistory(g *gitx.Git, dirs []string) (*History, error) {
	h := &History{Versions: map[string][][]byte{}}
	if _, err := g.Run("rev-parse", "--verify", "-q", "HEAD"); err != nil {
		return h, nil
	}
	args := []string{"log", "--first-parent", "--diff-merges=first-parent", "--root", "--raw",
		"--no-abbrev", "--no-renames", "--relative", "-z", "--format=%x01%H", "HEAD", "--"}
	out, err := g.Run(append(args, dirs...)...)
	if err != nil {
		return nil, err
	}
	closed := map[string]bool{}
	objects := map[string][]string{} // path -> blob ids, newest first
	var order []string
	tokens := strings.Split(out, "\x00")
	for i := 0; i < len(tokens); i++ {
		t := strings.TrimPrefix(tokens[i], "\n")
		if !strings.HasPrefix(t, ":") || i+1 >= len(tokens) {
			continue // a commit header, or the empty token after the last record
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
