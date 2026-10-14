package commands

import (
	"fmt"
	"slices"

	"github.com/arrowheadmind/memex/internal/config"
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
	today, err := config.Today(e.Getenv, e.Now())
	if err != nil {
		return e.failErr(err), nil
	}
	title, summary, body, code, uerr := e.prepareBody(n, today)
	if uerr != nil || code != 0 {
		return code, uerr
	}
	w, code, ok := e.beginWrite(true)
	if !ok {
		return code, nil
	}
	var repos []string
	if len(n.Repos) == 0 && !n.NoRepo {
		repos = attribution(w.root)
	}
	entries := w.entries()
	res := newResolver(entries)
	src := res.resolve(e, o.Source)
	if src == nil {
		w.end(nil)
		return spec.ExitFailure, nil
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
	files := []write.File{{Path: p.path, Data: p.data()}}
	if src.Type.Name == "journal" && slices.Contains([]string{"raw", "kept"}, src.Status()) {
		if why := rewritable(src); why != "" {
			return w.refuse(spec.ExitFailure, "%s has fields that promote cannot keep (%s); edit the file, then memex save", src.Path, why), nil
		}
		f := entry.Fields{}
		for k, v := range src.Fields {
			f[k] = v
		}
		f["status"] = "promoted"
		files = append(files, write.File{Path: src.Path, Data: []byte(entry.Render(f, src.Body))})
	}
	if err := write.Validate(w.root, w.today, files); err != nil {
		w.end(nil)
		return e.failedValidation(err), nil
	}
	msg := fmt.Sprintf("memex: promote %s to %s %s", src.ID, p.typ.Name, p.id)
	return w.commit(files, msg, "created "+p.path), nil
}
