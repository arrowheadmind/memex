// Package commands implements the §11 commands. main parses the arguments; each command gets
// an Env and its options, and returns the exit code.
package commands

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/arrowheadmind/memex/internal/config"
	"github.com/arrowheadmind/memex/internal/spec"
)

// Env is what a command sees of the process.
type Env struct {
	Stdin          io.Reader
	Stdout, Stderr io.Writer
	Getenv         config.Env
	Now            func() time.Time
	Store          string // the --store value
	StoreSet       bool   // whether --store was given
}

// OSEnv returns the Env of this process.
func OSEnv(stdout, stderr io.Writer) *Env {
	return &Env{Stdin: os.Stdin, Stdout: stdout, Stderr: stderr, Getenv: os.Getenv, Now: time.Now}
}

// fail prints a diagnostic and returns code.
func (e *Env) fail(code int, format string, args ...any) int {
	fmt.Fprintf(e.Stderr, "memex: "+format+"\n", args...)
	return code
}

// failErr prints err; a configuration error exits 2, anything else is an internal error.
func (e *Env) failErr(err error) int {
	var ce *config.Error
	if errors.As(err, &ce) {
		return e.fail(spec.ExitUsage, "%s", ce.Msg)
	}
	panic(err)
}

// openStore loads the configuration, today's date and the store root (SPEC §3).
func (e *Env) openStore() (*config.Config, time.Time, string, error) {
	cfg, err := config.Load(e.Getenv)
	if err != nil {
		return nil, time.Time{}, "", err
	}
	today, err := config.Today(e.Getenv, e.Now())
	if err != nil {
		return nil, time.Time{}, "", err
	}
	root, err := config.ResolveStore(e.Store, e.StoreSet, e.Getenv, cfg)
	if err != nil {
		return nil, time.Time{}, "", err
	}
	return cfg, today, root, nil
}
