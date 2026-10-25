# memex help

Print help.

## Usage

```
memex help [COMMAND|TOPIC] [--all] [--json]
```

## Description

With no argument, prints the overview. With a command name, prints that command's page (the
same as `memex <command> --help` or `-h`). With a topic name, prints that topic. `memex help
topics` lists every command and topic with its one-line description. Help needs no store, no
configuration and no network, and never writes anything.

## Options

| option | value | default | meaning |
|---|---|---|---|
| `--all` | | | Print the overview, every topic page, then every command page, separated by `---` |
| `--json` | | | Print the machine-readable interface: commands, options, types, fields, exit codes, topics |

## Output

Markdown, plain enough to read raw: headings, lists, tables and fenced code; lines of at most
100 characters; no colour. `--all` stays under 3,000 lines so that reading all of memex is one
affordable call. `--json` is one compact object whose first keys are `version` and `format`.

## Exit codes

| code | when |
|---|---|
| 0 | The page was printed |
| 2 | Unknown name (with `did you mean "…"?` when one is close), or `--all` combined with `--json` |

## Examples

```example
$ memex help topics
init  Create a store, or write pointer files into a directory
new  Create an entry of any type and commit it
capture  Create a journal entry and commit it
search  Ranked full-text search of the store
preflight  Start-of-task ritual: sync, Always rules, then search
list  List entries, newest first
show  Print entries with their links
set  Change frontmatter fields of an entry and commit it
promote  Create an entry derived from another and mark a journal source promoted
fmt  Rewrite entries into canonical form and commit them
save  Commit direct edits to the store
sync  Fetch, fast-forward or rebase, and push the store
check  Validate the store
doctor  Check the installation, the store and tool neutrality
import  Convert brain v3 stores into a new store, or add what is new to an existing one
help  Print help
version  Print the version
update  Append a dated section to an entry and replace its summary, in one commit
agents  Rules of engagement for language-model agents
concepts  The model and the reasoning behind it
entries  Types, statuses, frontmatter fields and body outlines
ids  The id and slug rules, with worked examples
links  Link syntax and resolution
search  Query syntax, ranking, filters and how to read output
writing  The write protocol, direct edits, outcomes and conflicts
config  Store resolution, the config file and environment variables
check-codes  Every E- and W- code, with its condition and the usual fix
exit-codes  The exit-code table
neutrality  Pointer files, the leak scan and tool memory
importing  The brain v3 importer and its report
```

```example
$ memex help serch
! memex: no help for "serch"; did you mean "search"? Run: memex help topics
exit 2
```

## Mistakes to avoid

- `memex help import` is the command page; the topic about the importer is `memex help
  importing`.
- Do not paste `memex help --all` into a context that only needs one command: fetch the page
  you need.

## See also

`memex help agents`, `memex help topics`.
