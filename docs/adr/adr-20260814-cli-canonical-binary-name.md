---
id: adr-20260814-cli-canonical-binary-name
kind: adr
title: Install the terminal CLI as agent-grid and install no short alias
status: accepted
created: '2026-08-14'
decision_makers:
- unknown
tags:
- native-clients
- cli
- packaging
owners:
- agent-grid-maintainers
relations:
- {type: originatedFrom, target: change-20260812-cli-terminal-client}
- {type: references, target: adr-20260814-cli-independent-go-module}
- {type: references, target: design-workspace-desktop}
source_paths:
- Makefile
- clients/cli
consequences:
  positive:
  - The installed name, the command package and the built artifact are one name, so
    no document has to explain a mapping.
  - The installer claims no name it does not own; an operator's PATH keeps whatever
    was already on it.
  - '`design-workspace-desktop` RESP-003''s named CLI consumer of the control endpoint
    can be identified by name rather than guessed at.'
  negative:
  - Nine characters at every prompt, which is the cost of not taking a name that is
    already in wide use.
  neutral:
  - An operator who wants brevity adds a shell alias, which is theirs to change and
    theirs to remove.
confirmation: '`make install-cli` places exactly one file, `$(HOME)/.local/bin/agent-grid`,
  and installs no other name.'
summary: Resolve handoff-cli-binary-canonical-name in favour of the self-describing
  name agent-grid, and leave the two-letter alias to the operator.
updated: '2026-08-14'
---

# Install the terminal CLI as agent-grid and install no short alias

## Context

`handoff-cli-binary-canonical-name` left the executable's canonical name to
packaging time, on the grounds that the name changes no observable. Packaging
time is now: `make install-cli` has to place a file under a name.

Three facts constrain the choice.

The UX member writes every example as `ag ls`, `ag attach`, `ag new`, `ag kill`
— sixty-four times — but never introduces `ag` as a decision. It is a
placeholder for "the command", written before the name existed.

`ag` is not a free name. It is the executable of The Silver Searcher, packaged
as `silversearcher-ag` on Debian and Ubuntu and as `the_silver_searcher` on
Homebrew, and it is installed on a large share of developer machines. An
installer that writes `~/.local/bin/ag` shadows it for every shell that
resolves `~/.local/bin` first, and does so silently.

The design plan recorded a second reason for care:
`design-workspace-desktop` RESP-003 names *the CLI* as a paired consumer of the
Workspace named-pipe control endpoint, and no such executable existed when that
was written. Resolving the name into a collision with a separate accepted
consumer contract would make that reference ambiguous rather than answerable.

## Decision

The canonical installed name is **`agent-grid`**. It is already the command
package (`clients/cli/cmd/agent-grid`) and the built artifact
(`dist/bin/agent-grid`), so the installed name introduces no mapping.

`make install-cli` installs that one file and no alias. It prints the alias an
operator can add — `alias ag=agent-grid` — as a suggestion, because a name the
operator writes into their own shell configuration is a name they can see and
remove.

## Consequences

{% consequence kind="positive" %}
One name spans the module path, the artifact and the installed file, so nothing
has to document a translation. The installer takes no name it does not own,
which means installing this project cannot break an unrelated tool. The CLI
that `design-workspace-desktop` RESP-003 names now exists under a name that
identifies it.
{% /consequence %}

{% consequence kind="negative" %}
`agent-grid attach` is longer to type than `ag attach`, at every prompt. That is
the price of not claiming a name that is already widely installed, and it is
paid by the people who use the tool most.
{% /consequence %}

{% consequence kind="neutral" %}
The UX member's `ag` examples stay as written. They are readable as the aliased
form, and rewriting sixty-four occurrences of a placeholder in a settled
requirements artifact would be churn without an observable change.
{% /consequence %}

## Alternatives

- **Install both `agent-grid` and an `ag` symlink** — rejected: it takes the
  contested name anyway, and takes it in the place where a shadowed tool is
  hardest to notice.
- **Install `ag` only** — rejected for the same collision, plus it leaves the
  module path and the artifact naming a different command than the one the
  operator runs.
- **Ask at install time** — rejected: `make install-cli` is used
  non-interactively, and a prompt would make the installed name unpredictable
  from the repository.

Confirmation: `make install-cli` places exactly `$(INSTALL_DIR)/agent-grid`.

## Related

- requirements: the UX member's §9 open question *実行 binary の canonical 名*
- handoff: `handoff-cli-binary-canonical-name`
- plan: `docs/changes/change-20260812-cli-terminal-client/design-plan/design.md`
  anchor `handoff-cli-binary-canonical-name`
