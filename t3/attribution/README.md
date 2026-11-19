# No agent attribution

Agents that T3 Code runs can add credit lines such as
`Created with <model> in Claude Code under T3 Code.` or
`Co-Authored-By: Claude <noreply@anthropic.com>` to pull requests and commits.
These files stop that for new work. They do not change existing pull requests
or commits.

| File                   | Stops credit in                                     | How                                    |
| ---------------------- | --------------------------------------------------- | -------------------------------------- |
| `AGENTS.md`            | everything an agent writes                          | global instruction for each agent      |
| `claude-settings.json` | Claude Code commits and pull requests               | blanks Claude Code's `attribution`     |
| `commit-msg`           | commit messages                                     | global Git hook that deletes the lines |
| `gh`                   | pull request, issue and release text, `gh api` body | wrapper that deletes the lines         |

T3's own setting, "Remove agent credits when merging", is already on in
[`../settings/environment.json`](../settings/environment.json). It cleans merge
and squash messages.

## Install

Install the instruction in every agent home that T3 uses. Include custom
provider homes declared in T3's provider settings:

```sh
cp t3/attribution/AGENTS.md ~/.claude/CLAUDE.md
cp t3/attribution/AGENTS.md ~/.codex/AGENTS.md
cp t3/attribution/AGENTS.md ~/.gemini/GEMINI.md
```

Append instead of copying when the target file already has content. Merge
`claude-settings.json` into each Claude home's `settings.json`.

Install the Git hook globally. A repository's own `commit-msg` hook still runs
first:

```sh
mkdir -p ~/.config/git/hooks
cp t3/attribution/commit-msg ~/.config/git/hooks/commit-msg
git config --global core.hooksPath ~/.config/git/hooks
```

Install the `gh` wrapper in a PATH directory that comes before the real `gh`:

```sh
cp t3/attribution/gh ~/.local/bin/gh
command -v gh   # should print the wrapper
```

The wrapper needs Python 3 and passes read-only calls straight to the real
`gh`. When it removes a line it prints `gh wrapper: removed N agent credit
line(s)`. Set `GH_WRAPPER_REAL` to point it at a specific `gh`.

## Limits

Text sent outside the `gh` CLI, such as through the GitHub website or a GitHub
tool inside an agent, relies on the instruction alone. Running threads pick up
the hook and wrapper on their next call; they read the instruction files only
when they start.
