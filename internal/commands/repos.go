package commands

import (
	"strings"

	"github.com/arrowheadmind/memex/internal/config"
	"github.com/arrowheadmind/memex/internal/gitx"
	"github.com/arrowheadmind/memex/internal/neutral"
	"github.com/arrowheadmind/memex/internal/spec"
	"github.com/arrowheadmind/memex/internal/store"
)

// requireRepos reports the store's require_repos (SPEC §4, §5.5): true when the working
// tree's memex.toml or HEAD's sets it. A write commits only its own paths, so HEAD's manifest
// is the one the commit carries, and an uncommitted edit of the key must not turn it off.
// beginWrite has already checked the manifest, so a read failure means the key is not set.
func (w *writing) requireRepos() bool {
	if m, _, err := store.ReadManifest(store.WorkTree{Root: w.root}); err == nil && m.RequireRepos {
		return true
	}
	out, err := gitx.New(w.root, 0).Run("show", "HEAD:"+store.ManifestFile)
	if err != nil {
		return false
	}
	m, _, err := store.ParseManifest([]byte(out))
	return err == nil && m.RequireRepos
}

// refuseNoRepo ends a write refused for missing repos (SPEC §5.5): the refusal line, then
// the line listing the workspace's repositories. Exit 1, nothing changed.
func (w *writing) refuseNoRepo(format string, args ...any) int {
	line := w.repositoryLine()
	w.refuse(spec.ExitFailure, format, args...)
	return w.e.fail(spec.ExitFailure, "%s", line)
}

// repositoryLine names the repositories a retry with --repo could give: the git top-level
// directories of the §12 workspace scan, or why there is no list (SPEC §5.5). A workspace
// that cannot be used never changes the refusal's exit code.
func (w *writing) repositoryLine() string {
	ws, err := config.ResolveWorkspace(w.e.Getenv, w.cfg, w.root)
	if err != nil {
		msg := err.Error()
		if ce, ok := err.(*config.Error); ok {
			msg = ce.Msg
		}
		return "no repository names to list: " + msg
	}
	names := neutral.Scan{Workspace: ws, Store: w.root}.Repositories()
	if len(names) == 0 {
		return "no repositories in the workspace " + ws
	}
	return "repositories in the workspace " + ws + ": " + strings.Join(names, ", ")
}

// newRefusalNoRepo is the first line of the new and capture refusal (SPEC §5.5).
func newRefusalNoRepo(root string) string {
	return root + " requires repos (require_repos in memex.toml) and this entry has none; " +
		"retry with --repo <name>, or with --no-repo if it concerns no repository; nothing was changed"
}

// updateRefusalNoRepo is the first line of the update refusal (SPEC §5.5).
func updateRefusalNoRepo(root, id string) string {
	return root + " requires repos (require_repos in memex.toml) and " + id + " has none; " +
		"add the repository it concerns first: memex set " + id + " repos=<name>, " +
		"or retry with --no-repo if it concerns none; nothing was changed"
}
