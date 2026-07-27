# Update GPT models and pricing to current versions

## Goal

`internal/llm/client.go` and `internal/config/config.go` only know about
`gpt-4o` and `gpt-4o-mini`, with pricing last verified June 2026. Update the
default model, the `modelPricing` table, and any related test fixtures to
the current OpenAI model lineup and per-1k-token pricing, so cost estimates
and default behavior reflect what's actually available today rather than a
stale snapshot.

## Out of scope / non-goals

- Adding support for non-OpenAI providers.
- Changing the cost-estimation logic itself (`estimateCost`), only the data
  it's fed.
- Adding automatic/dynamic pricing lookups (e.g. fetching from an API) —
  this stays a hardcoded, manually-verified table as it is today.
- Changing `OPENAI_MODEL` env var override behavior in config.go.

## Prerequisites

None.

## User decisions

<TODO: which specific model(s) should become the new default — needs a
human decision once current OpenAI pricing/model names are confirmed,
since the agent doing this task cannot browse OpenAI's live pricing page
from local repo context alone.>

## Reconciliations

- Confirmed only two call sites define/use the model default and pricing
  table: `internal/llm/client.go:15` (`defaultModel`) and
  `internal/llm/client.go:93-101` (`modelPricing` map, comment says
  "verify at https://openai.com/pricing", "Last updated: June 2026").
- `internal/config/config.go:95` sets `cfg.OpenAIModel = "gpt-4o-mini"` as
  the config-level default, separate from `client.go`'s `defaultModel`
  constant — both need to move together or be reconciled into one source
  of truth.
- Existing tests referencing model/pricing values found in
  `internal/llm/client_test.go` and `internal/config/config_test.go` — these
  will need updated expectations.

## Acceptance

- [ ] `defaultModel` in `internal/llm/client.go` and `OpenAIModel` default
      in `internal/config/config.go` both point to the same, currently
      available OpenAI model.
- [ ] `modelPricing` in `internal/llm/client.go` contains entries for all
      models the codebase can select (including any new default), with
      per-1k-token input/output prices matching OpenAI's published pricing
      as of the update date.
- [ ] The "Last updated" comment above `modelPricing` reflects the date of
      this change.
- [ ] `internal/llm/client_test.go` and `internal/config/config_test.go`
      pass with updated model/price expectations.
- [ ] `go test ./...` passes.

## Main risk

OpenAI's current model names and per-token prices aren't verifiable from
within this repo — an agent must fetch them from a live, authoritative
source (not guess or recall from training data) before writing the new
`modelPricing` values, since a wrong price silently produces wrong cost
estimates rather than a visible failure.
