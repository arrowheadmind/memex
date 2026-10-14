// Package store implements SPEC §4: the manifest, the allowed top-level items and the
// enumeration of entries.
package store

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/arrowheadmind/memex/internal/entry"
	"github.com/arrowheadmind/memex/internal/spec"
)

// Finding is one §7 finding about a path of the store.
type Finding struct {
	Path    string
	Code    string
	Message string
}

// FormatError is a manifest whose format this memex cannot read (exit 2, SPEC §4).
type FormatError struct{ Msg string }

func (e *FormatError) Error() string { return e.Msg }

// Manifest is memex.toml.
type Manifest struct {
	Format int64
	Name   string
}

// ManifestFile is the manifest's path.
const ManifestFile = "memex.toml"

// ReadManifest parses memex.toml from t. A format other than spec.StoreFormat is a
// *FormatError; anything else wrong is an E-MANIFEST finding.
func ReadManifest(t Tree) (*Manifest, []Finding, error) {
	data, err := t.ReadFile(ManifestFile)
	if err != nil {
		return nil, nil, err
	}
	bad := func(format string, args ...any) Finding {
		return Finding{ManifestFile, "E-MANIFEST", fmt.Sprintf(format, args...)}
	}
	var raw map[string]any
	if _, err := toml.Decode(string(data), &raw); err != nil {
		msg := "invalid TOML"
		var pe toml.ParseError
		if errors.As(err, &pe) {
			msg += fmt.Sprintf(" at line %d", pe.Position.Line)
		}
		return &Manifest{}, []Finding{bad("%s", msg)}, nil
	}
	m := &Manifest{}
	var fs []Finding
	keys := make([]string, 0, len(raw))
	for k := range raw {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		switch v := raw[k].(type) {
		case int64:
			if k == "format" {
				m.Format = v
				continue
			}
		case string:
			if k == "name" {
				m.Name = v
				continue
			}
		}
		switch k {
		case "format":
			fs = append(fs, bad("format must be an integer"))
		case "name":
			fs = append(fs, bad("name must be a string"))
		default:
			msg := "unknown key " + entry.Quote(k)
			if near, ok := spec.Nearest(k, []string{"format", "name"}); ok {
				msg += " (did you mean " + entry.Quote(near) + "?)"
			}
			fs = append(fs, bad("%s", msg))
		}
	}
	if _, ok := raw["format"]; !ok {
		fs = append(fs, bad("missing key \"format\""))
	}
	if _, ok := raw["format"].(int64); ok {
		switch {
		case m.Format > spec.StoreFormat:
			return nil, nil, &FormatError{fmt.Sprintf(
				"store format %d is newer than this memex reads (%d); upgrade memex", m.Format, spec.StoreFormat)}
		case m.Format < spec.StoreFormat:
			return nil, nil, &FormatError{fmt.Sprintf(
				"store format %d is older than this memex reads (%d); run the migration that ships with the memex version that introduced format %d",
				m.Format, spec.StoreFormat, spec.StoreFormat)}
		}
	}
	return m, fs, nil
}

// File is one entry file found in a type directory.
type File struct {
	Path string // e.g. rules/2026-01-15-x.md
	Type *spec.Type
	ID   string // the filename without .md
}

// Layout is the result of enumerating a store.
type Layout struct {
	Files    []File   // entries, sorted by path
	Owners   []string // the names directly under files/, sorted
	Findings []Finding
}

// ignored are top-level names memex ignores besides dot-files (SPEC §4).
var ignored = []string{"LICENSE", "flake.nix", "flake.lock", "import-report.json"}

// deltaReport matches the report names of a delta import (SPEC §4, §13.7).
var deltaReport = regexp.MustCompile(`^import-report-[0-9]{4}-[0-9]{2}-[0-9]{2}(-[0-9]+)?\.json$`)

// known are the other top-level names of SPEC §4 that are not type directories.
var known = []string{ManifestFile, "AGENT.md", "README.md"}

// Enumerate lists the entries of t and reports E-NOT-ENTRY for unknown top-level items and
// for anything in a type directory that is not a .md file. files/ owners are returned for
// the caller to check against the ids (E-FILES-OWNER).
func Enumerate(t Tree) (*Layout, error) {
	top, err := t.Children("")
	if err != nil {
		return nil, err
	}
	l := &Layout{}
	notEntry := func(p, msg string) {
		l.Findings = append(l.Findings, Finding{p, "E-NOT-ENTRY", msg})
	}
	for _, it := range top {
		name := it.Name
		switch {
		case strings.HasPrefix(name, "."), slices.Contains(ignored, name), deltaReport.MatchString(name):
		case slices.Contains(known, name):
			if !it.Regular {
				notEntry(name, "not a regular file")
			}
		case name == "files":
			if !it.Dir {
				notEntry(name, "not a directory; files/ holds attachments as files/<id>/")
				continue
			}
			items, err := t.Children(name)
			if err != nil {
				return nil, err
			}
			for _, o := range items {
				if strings.HasPrefix(o.Name, ".") {
					continue // dot-files and dot-directories are ignored, as at the top level
				}
				if !o.Dir {
					l.Findings = append(l.Findings, Finding{"files/" + o.Name, "E-FILES-OWNER",
						"not a directory; attachments go in files/<id>/"})
					continue
				}
				l.Owners = append(l.Owners, o.Name)
			}
		case spec.LookupDirectory(name) != nil:
			typ := spec.LookupDirectory(name)
			if !it.Dir {
				notEntry(name, "not a directory; "+name+"/ holds "+typ.Name+" entries")
				continue
			}
			items, err := t.Children(name)
			if err != nil {
				return nil, err
			}
			for _, e := range items {
				p := name + "/" + e.Name
				switch {
				case e.Dir:
					notEntry(p, "subdirectory in a type directory; type directories are flat, attachments go in files/<id>/")
				case !e.Regular || !strings.HasSuffix(e.Name, ".md"):
					notEntry(p, "not a .md file; type directories hold only entries, attachments go in files/<id>/")
				default:
					l.Files = append(l.Files, File{Path: p, Type: typ, ID: strings.TrimSuffix(e.Name, ".md")})
				}
			}
		default:
			notEntry(name, "unknown top-level item; it is not part of the store layout")
		}
	}
	return l, nil
}
