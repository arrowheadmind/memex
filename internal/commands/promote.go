package commands

import (
	"fmt"
	"slices"

	"github.com/arrowheadmind/memex/internal/entry"
	"github.com/arrowheadmind/memex/internal/spec"
	"github.com/arrowheadmind/memex/internal/write"
)

// PromoteOptions are the arguments of `memex promote` (SPEC §11.8).
type PromoteOptions struct {
	Source string
	New    NewOptions // Type is the --to type
}

// Promote runs `memex promote` (SPEC §11.8): a new entry citing the source, and a journal
// source marked promoted, in one commit.
func Promote(e *Env, o PromoteOptions) (int, *UsageError) {
	n := &o.New
	if uerr := n.checkUsage(); uerr != nil {
		return 0, uerr
	}
	w, code, ok := e.beginWrite(true)
	if !ok {
		return code, nil
	}
	entries := w.entries()
	res := newResolver(entries)
	src, r := w.target(res, o.Source)
	if r != nil {
		return w.refuse(spec.ExitFailure, "%s", r.msg), nil
	}
	rewrite := src.Type.Name == "journal" && slices.Contains([]string{"raw", "kept"}, src.Status())
	_, unusable := w.unusable[src.Path]
	if rewrite || unusable {
		if code, refused := w.refuseUncommitted(src.Path); refused {
			return code, nil
		}
		why := w.unusable[src.Path]
		if why == "" {
			why = rewritable(src)
		}
		if why != "" {
			return w.refuse(spec.ExitFailure, "%s has fields that promote cannot keep (%s); edit the file, then memex save", src.Path, why), nil
		}
	}
	title, summary, body, code, uerr := e.prepareBody(n, w.today)
	if uerr != nil || code != 0 {
		w.end(nil)
		return code, uerr
	}
	var repos []string
	if len(n.Repos) == 0 && !n.NoRepo {
		repos = attribution(w.root)
	}
	n.Sources = append([]string{src.ID}, n.Sources...)
	p := n.build(w, res, title, summary, body, repos)
	if p == nil {
		w.end(nil)
		return spec.ExitFailure, nil
	}
	if f, en := existing(w.root, entries, p.id); f != nil {
		if f.Type == p.typ && en != nil && en.Body == p.body {
			return w.nothing("exists " + f.Path), nil
		}
		return w.refuseExisting(f, en), nil
	}
	files := []write.File{{Path: p.path, Data: p.data(), Check: true}}
	if rewrite {
		f := entry.Fields{}
		for k, v := range src.Fields {
			f[k] = v
		}
		f["status"] = "promoted"
		files = append(files, write.File{Path: src.Path, Data: []byte(entry.Render(f, src.Body)), Check: true, Expect: src.Data})
	}
	if err := write.Validate(w.root, w.today, files); err != nil {
		w.end(nil)
		return e.failedValidation(err), nil
	}
	msg := fmt.Sprintf("memex: promote %s to %s %s", src.ID, p.typ.Name, p.id)
	return w.commit(files, msg, "created "+p.path), nil
}
