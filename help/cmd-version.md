# memex version

Print the version.

## Usage

```
memex version
```

## Description

Prints the tool version and the store format it reads and writes, on one line. It needs no
store and no configuration. `memex --version` is the same.

## Output

```
memex <version> (store format 1)
```

`<version>` is the release version, or `0.0.0-dev` for a build that is not a release.

## Exit codes

| code | when |
|---|---|
| 0 | Always, unless given an argument |
| 2 | An argument or option was given |

## Examples

```example
$ memex version
memex <VERSION> (store format 1)
```

```example
$ memex --version
memex <VERSION> (store format 1)
```

## Mistakes to avoid

- `memex version --json` is refused: the line is already machine-readable. Use `memex help
  --json` for the full interface, which starts with the version.

## See also

`memex help`, `memex help config`.
