# CLAUDE.md

## Project

<TODO: one or two sentences — what this project is and what stack it's
built on.>

## Commands

- Build: `<TODO>`
- Test: `<TODO>`
- Lint: `<TODO>`
- Dev/run: `<TODO>`

## Architecture summary

<TODO: a few bullets — major components, where state lives, hard invariants
a change must not break.>

## Hard constraints

<TODO: locked decisions this project's agents must not deviate from without
asking first — approved dependencies, fixed protocols, anything that must
never be re-implemented from scratch.>

## Gotchas

<TODO: non-obvious traps future work will hit — the stuff that costs hours
if undocumented. Add to this list whenever one is found the hard way.>

## Workflow

- Ask before adding any dependency beyond an approved list, and before
  deviating from a locked decision — don't guess, don't silently substitute.
- Don't implement a deferred/out-of-scope item just because it's adjacent to
  the current task — flag it instead of expanding scope mid-task.
- Idea capture, kept as two lists: a scratch log for your own raw,
  out-of-scope observations noticed mid-task (append and keep going), and a
  reviewed backlog the human curates from it. Never promote an entry into
  real scope yourself.
- Commits: small, one per completed task, imperative subject line.
- Task specs live in `docs/tasks/` — see `docs/tasks/TEMPLATE.md` for the
  shape a new one should take.
