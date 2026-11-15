# Security

memex runs on your own machine, from your shell, your coding agents' sessions and the git hooks
it writes. It keeps your notes in a git repository at a path you configure, and it has no server,
no account and no telemetry. It has no network code of its own: the only traffic is `git`
fetching from and pushing to the store's own remote, with every prompt switched off. The store
is meant to stay private, so the things that would matter most are:

- a way to make memex send or copy anything from the store anywhere other than the store's own
  remote;
- a crafted entry, id, link, config file or imported store that makes memex write a file outside
  the store (other than the pointer files `memex init --pointers` writes where you tell it),
  run a program other than `git`, or hand `git` an argument it takes as an option;
- a problem in how the release binary or the Nix package is built or delivered.

The credential warning (`W-SECRET`) is a best-effort check, not a guarantee. A credential it
misses is worth an ordinary issue, not a private report.

To report a vulnerability, write to security@arrowheadmind.com rather than opening a public
issue. Where the project has a GitHub repository, its Security tab has a private reporting form
as a second way. Say what you found, the memex version (`memex version`) and how to see it,
with the smallest store or command that shows it. You will get an answer as soon as possible,
saying what will be done. Please allow a reasonable time, normally 90 days, for a fix before
making it public.

Only the latest release is supported.
