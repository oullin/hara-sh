## Agent state policy

- Keep Claude-managed cache, scratch files, browser artifacts, clones, and manually-created Git worktrees under `/Users/gocanto/.claude`.
- Use `/Users/gocanto/.claude/work` for scratch work and `/Users/gocanto/.claude/worktrees` for manually-created Git worktrees.
- Do not create new Claude-managed state under `~/Documents`, `~/.cache`, or a project checkout unless the user explicitly requests a project-local deliverable. Legacy paths may be symlinks into `/Users/gocanto/.claude` for compatibility.
