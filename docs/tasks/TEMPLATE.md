---
# Optional. Omit this whole block for a single-sitting spec with no passes —
# dispatch then works exactly as if it were absent (a missing or malformed
# block never errors, it just seeds no model/effort).
#
# Two shapes, pick whichever matches this spec:
#
# 1. Multi-pass — a `passes:` list, one entry per pass, in pass order:
# passes:
#   - name: "Pass 1 — <short pass title>"
#     claude: { model: sonnet-5, effort: medium }
#     codex: { model: gpt-5-codex, effort: medium }
#   - name: "Pass 2 — <short pass title>"
#     claude: { model: opus-4-8, effort: high }
#
# 2. Passless (single sitting) — a top-level per-agent block instead:
# claude: { model: sonnet-5, effort: medium }
# codex: { model: gpt-5-codex, effort: medium }
#
# `model` is whatever alias the target CLI accepts. `effort` is the
# canonical vocabulary low | medium | high | xhigh | max; it's mapped per
# agent, and an agent that doesn't support a given value just dispatches
# without that flag rather than guessing. Naming a pass or agent is
# optional throughout — anything you don't name just dispatches with no
# seeded model/effort, same as a spec with no front-matter at all.
---

# <Task title>

## Goal

<TODO: one paragraph — what this spec proves or builds, and why it's scoped
this way. If it was promoted from an ideas backlog, name the source entry.>

## Out of scope / non-goals

- <TODO: explicitly excluded thing — scope creep is the top agent failure
  mode, so name what this spec deliberately does not do.>

## Prerequisites

<TODO: what must exist first, if anything. Use `After: <slug>` to sequence
against another task spec, or "None" if this can start immediately.>

## User decisions

<TODO: judgment calls resolved before/while drafting this spec, dated
`(YYYY-MM-DD)`. Delete this section if there were none.>

## Reconciliations

<TODO: what was checked against the actual codebase while drafting this
spec — existing files/commands/components, whether a dependency is already
in use, whether a named blocking prerequisite has actually landed. "Wiring,
not new machinery." Delete this section if nothing needed reconciling.>

## Passes

<Omit this whole section for a spec small enough for one sitting — drop
straight to Acceptance. Otherwise, one subsection per pass below, each
ending at a concrete gate before the next starts.>

### Pass 1 — <short pass title>

**Gate:** <TODO: exact, checkable criteria that ends this pass — e.g. a
test suite green, a lint clean, or "the Acceptance suite below">

- [ ] <task>
- [ ] <task>

<Repeat "Pass N" as needed.>

## Acceptance

- [ ] <TODO: verifiable behavior tied back to the Goal — not just "the code
      compiles">

## Main risk

<TODO: one or two sentences naming where this spec's risk concentrates and
why.>
