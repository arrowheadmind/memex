package commands

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/arrowheadmind/memex/internal/config"
	"github.com/arrowheadmind/memex/internal/gitx"
	"github.com/arrowheadmind/memex/internal/neutral"
	"github.com/arrowheadmind/memex/internal/spec"
	"github.com/arrowheadmind/memex/internal/store"
)

// InitOptions are the options of `memex init` (SPEC §11.1).
type InitOptions struct {
	Path     string // the store form
	Name     string
	NameSet  bool
	Pointers string // the pointer form's DIR
	Force    bool
}

// Init runs `memex init` (SPEC §11.1).
func Init(e *Env, o InitOptions) int {
	if o.Pointers != "" {
		return e.initPointers(o)
	}
	dir, err := filepath.Abs(o.Path)
	if err != nil {
		return e.fail(spec.ExitUsage, "%s: %v", o.Path, err)
	}
	if info, err := os.Stat(dir); err == nil && !info.IsDir() {
		return e.fail(spec.ExitFailure, "%s exists and is not a directory", o.Path)
	}
	name := filepath.Base(dir)
	if o.NameSet {
		name = o.Name
	}
	lines, err := store.CreateLayout(dir, name, gitx.New(dir, 0))
	if err != nil {
		return e.fail(spec.ExitFailure, "init %s: %v", o.Path, err)
	}
	for _, l := range lines {
		fmt.Fprintln(e.Stdout, l)
	}
	fmt.Fprintf(e.Stdout, "initialized %s; run: memex save -m \"init store\"\n", dir)
	return spec.ExitOK
}

// PointerFiles are the files `init --pointers` writes (SPEC §11.1, §12).
var PointerFiles = []string{"CLAUDE.md", "AGENTS.md", "GEMINI.md"}

// pointerStore is how the pointer text names the store (SPEC §12 rule 2): the config file's
// `store` string when the store came from there, otherwise the resolved absolute path.
func (e *Env) pointerStore(cfg *config.Config, root string) string {
	if !e.StoreSet && e.Getenv("MEMEX_STORE") == "" && cfg.Store != "" {
		return cfg.Store
	}
	return root
}

// samePointer reports whether content is the pointer line naming root, comparing the paths
// after the §3.1 expansion of both, as check --workspace does (SPEC §11.1, §12).
func (e *Env) samePointer(content, root string) bool {
	return neutral.IsPointer(content, root, e.Getenv("HOME"))
}

func (e *Env) initPointers(o InitOptions) int {
	cfg, _, root, err := e.openStore()
	if err != nil {
		return e.failErr(err)
	}
	if info, err := os.Stat(o.Pointers); err != nil || !info.IsDir() {
		return e.fail(spec.ExitFailure, "%s is not a directory", o.Pointers)
	}
	text := []byte(neutral.PointerText(e.pointerStore(cfg, root)))
	code := spec.ExitOK
	for _, name := range PointerFiles {
		p := filepath.Join(o.Pointers, name)
		old, err := os.ReadFile(p)
		// A symbolic link is never the pointer file, and writing through it would change its
		// target, so it is replaced by a file of its own.
		link := false
		if info, lerr := os.Lstat(p); lerr == nil && info.Mode()&os.ModeSymlink != 0 {
			link = true
			if err != nil {
				old, err = []byte{}, nil
			}
		}
		switch {
		case err == nil && !link && (bytes.Equal(old, text) || e.samePointer(string(old), root)):
			fmt.Fprintf(e.Stdout, "unchanged %s\n", p)
			continue
		case err == nil && !o.Force:
			fmt.Fprintf(e.Stderr, "memex: %s exists with other content; left alone (use --force to replace it)\n", p)
			code = spec.ExitFailure
			continue
		case err == nil:
			e.Stdout.Write(old)
			if len(old) > 0 && old[len(old)-1] != '\n' {
				fmt.Fprintln(e.Stdout)
			}
		case !os.IsNotExist(err):
			return e.fail(spec.ExitFailure, "%s: %v", p, unwrap(err))
		}
		if link {
			if err := os.Remove(p); err != nil {
				return e.fail(spec.ExitFailure, "%s: %v", p, unwrap(err))
			}
		}
		if err := os.WriteFile(p, text, 0o644); err != nil {
			return e.fail(spec.ExitFailure, "%s: %v", p, unwrap(err))
		}
		if old != nil {
			fmt.Fprintf(e.Stdout, "replaced %s\n", p)
		} else {
			fmt.Fprintf(e.Stdout, "created %s\n", p)
		}
	}
	return code
}
