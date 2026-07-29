# Implement a Claude analog to the OpenAI client for summarisation

## Goal

`internal/llm/client.go` only speaks OpenAI's Chat Completions API. Add an
Anthropic Claude client that plugs into the same summarisation path
(`internal/llm/summarise.go`), so `cmd/notify` can generate the staging
deployment summary using one provider or the other — never both at once —
picked per-project via the `LLM_PROVIDER` GitLab CI variable. This gives
users a choice when they'd rather not hold an OpenAI key, or want to
compare summary quality/cost between providers, without forking the tool.

## Out of scope / non-goals

- Supporting more than two providers (OpenAI, Claude) — no generic
  multi-provider plugin system, just one additional concrete client.
- Running both clients concurrently, or constructing both in the same
  process — exactly one client is active per GitLab CI run, chosen once at
  config load from the `LLM_PROVIDER` CI variable, never both at once.
- Runtime/automatic provider fallback (e.g. retry on Claude if OpenAI's
  quota is exceeded) — the provider is a fixed choice per CI run via
  config, not a failover chain.
- Streaming responses — both clients stay single-shot request/response,
  matching the current OpenAI client.
- Changing the summarisation prompt/rules in `summarise.go` — the same
  `systemPrompt` and `BuildPrompt` output are sent to whichever provider is
  configured.
- Dynamic/fetched pricing — Claude pricing is added as a hardcoded,
  manually-verified table, same convention as `modelPricing` today.

## Prerequisites

None.

## User decisions

<TODO: which Claude model(s) should be supported/default (e.g. a Sonnet
tier as the default, matching gpt-5-mini's role as the current cheap
default), and their per-1k-token pricing — needs a human decision once
current Anthropic pricing/model names are confirmed from a live,
authoritative source, not guessed from training data. See
`docs/tasks/update-the-gpt-models-and-price-to-the-newest-versions-there-ware-only-the-old-ones-present.md`
for the same caution applied to the OpenAI side.>

- (2026-07-29) Provider is a GitLab CI variable, `LLM_PROVIDER`, with
  values `openai` (default) and `claude`. Only one client is ever active
  per CI run — the project's GitLab CI/CD variables settle it once for
  that run, no runtime switching or dual clients.
- (2026-07-29) Must stay backwards compatible where possible: with
  `LLM_PROVIDER` unset, an existing project's `.gitlab-ci.yml` and CI/CD
  variables keep working unmodified — same required vars
  (`OPENAI_API_KEY`), same default model, same client behaviour as today.
  `ANTHROPIC_API_KEY` (and optionally `ANTHROPIC_MODEL`) are net-new,
  required only when `LLM_PROVIDER=claude`.

## Reconciliations

- `internal/llm/summarise.go:59` — `Summarise(client *Client, input Input)`
  takes the concrete OpenAI `*Client` type. Making this provider-agnostic
  requires either an interface (e.g. a `ChatCompleter` interface exposing
  `ChatCompletion(systemPrompt, userPrompt string) (Result, error)`, which
  both `*Client` and the new Claude client satisfy) or an equivalent seam —
  `Summarise` itself and `BuildPrompt`/`systemPrompt` need no changes
  beyond that.
- `internal/llm/client.go:93-101` — `modelPricing` and `estimateCost` are
  OpenAI-specific (map keyed by OpenAI model names, computed inside
  `ChatCompletion`). The Claude client needs its own equivalent pricing
  table and cost calculation, since Anthropic's request/response shapes
  and usage field names (`input_tokens`/`output_tokens` under
  `usage`, vs. OpenAI's `prompt_tokens`/`completion_tokens`) differ and a
  single shared map/function can't safely serve both without a model-name
  collision risk.
- `internal/httpretry` (used by `internal/llm/client.go:118` via
  `c.retry.Do`) is already provider-agnostic (shared by GitLab, LLM, and
  Slack clients per its package doc) — the Claude client should reuse it
  the same way, not reimplement retry/backoff.
- Anthropic's Messages API differs structurally from OpenAI's Chat
  Completions API: system prompt is a top-level `system` field rather than
  a `system`-role message, `content` in requests/responses is a list of
  typed blocks rather than a plain string, auth uses an `x-api-key` header
  plus a required `anthropic-version` header rather than `Authorization:
  Bearer`, and the base URL/path differ
  (`https://api.anthropic.com/v1/messages` vs.
  `https://api.openai.com/v1/chat/completions`). The new client's request
  building and response parsing cannot mirror `client.go`'s structs
  verbatim.
- `internal/config/config.go` currently treats `OPENAI_API_KEY` as always
  required (`Load()` adds it to `missing` unconditionally at
  config.go:68-71). With `LLM_PROVIDER` defaulting to `openai`, an
  unmodified project keeps requiring exactly `OPENAI_API_KEY` as today
  (backwards compatible); `LLM_PROVIDER=claude` instead requires
  `ANTHROPIC_API_KEY` and makes `OPENAI_API_KEY` optional. Only one key is
  ever required per run, matching the one-active-client constraint.
- `cmd/notify/main.go:47` constructs the LLM client directly
  (`llm.NewClient(cfg.OpenAIAPIKey, cfg.OpenAIModel)`); this becomes a
  one-time `switch cfg.LLMProvider` at startup that constructs exactly one
  client — OpenAI or Claude — and passes it to `llm.Summarise` through the
  shared interface.
- README's Quick start CI snippet (`README.md:41-56`) and Configuration
  tables (`README.md:82-101`) document `OPENAI_API_KEY`/`OPENAI_MODEL` as
  the only LLM config — these need new rows/notes for the provider switch
  and Claude-specific vars, plus the Dockerfile/image likely needs no
  change (it's just a Go binary), but should be checked.
- Existing tests to extend, following existing patterns: a
  `TestChatCompletion_*` suite for the new Claude client mirroring
  `internal/llm/client_test.go` (httptest server, rate-limit/error-body
  cases, retry-then-succeed case), a `TestEstimateCost`-equivalent for
  Claude pricing, and `internal/config/config_test.go` cases for the new
  conditional-required-var logic.

## Passes

### Pass 1 — Provider-agnostic summarisation seam

**Gate:** `go build ./...` succeeds; `internal/llm` tests green with no
behavioural change to the OpenAI path.

- [ ] Introduce a `ChatCompleter` interface in `internal/llm` exposing
      `ChatCompletion(systemPrompt, userPrompt string) (Result, error)`.
- [ ] Change `Summarise` to accept a `ChatCompleter` instead of `*Client`.
- [ ] Confirm `*Client` (OpenAI) satisfies the interface with no signature
      changes.

### Pass 2 — Claude client

**Gate:** new Claude client has test coverage matching the OpenAI client's
existing suite (rate limit with/without error body, retry-then-succeed,
request shape, cost estimation) and satisfies `ChatCompleter`.

- [ ] Add a Claude client (e.g. `internal/llm/claude.go`) implementing
      Anthropic's Messages API: request/response structs matching its
      actual shape (top-level `system`, block-based `content`,
      `input_tokens`/`output_tokens` usage), `x-api-key` +
      `anthropic-version` headers, reusing `internal/httpretry` for
      retries.
- [ ] Add a Claude-specific pricing table and cost calculation, per the
      model(s)/prices resolved in User decisions.
- [ ] Map Anthropic's error responses (rate limit, other API errors) onto
      the existing shared `ErrQuotaExceeded`/`ErrAPIError` sentinels from
      `internal/llm/errors.go`, so `cmd/notify`'s existing error handling
      (main.go:96-104) needs no provider-specific branches.
- [ ] Test suite mirroring `internal/llm/client_test.go`'s cases against
      an `httptest` server standing in for the Anthropic API.

### Pass 3 — Config, wiring, and docs

**Gate:** the Acceptance suite below.

- [ ] Add `LLM_PROVIDER` (default `openai`), `ANTHROPIC_API_KEY`, and
      `ANTHROPIC_MODEL` to `internal/config/config.go`, with only the
      selected provider's API key required (update the `missing`-vars
      logic accordingly).
- [ ] Wire `cmd/notify/main.go` to construct the OpenAI or Claude client
      based on `cfg`'s provider selection and pass it to `llm.Summarise`
      via the `ChatCompleter` interface.
- [ ] Update the LLM-usage log line (main.go:107-113) to work for either
      provider (it currently logs `cfg.OpenAIModel` unconditionally).
- [ ] Update README: Quick start snippet, required/optional variable
      tables, and any prose describing "an LLM" to reflect the provider
      choice.
- [ ] Add/update `internal/config/config_test.go` cases for: default
      provider requires only `OPENAI_API_KEY`, `LLM_PROVIDER=claude`
      requires `ANTHROPIC_API_KEY` instead, and existing required-var
      tests still pass for the default (`openai`) path.

## Acceptance

- [ ] With `LLM_PROVIDER` unset (or `openai`), behaviour is unchanged from
      today — same required vars, same default model, same client used.
      An existing project's `.gitlab-ci.yml`/CI variables need no changes.
- [ ] With `LLM_PROVIDER=claude` and `ANTHROPIC_API_KEY` set,
      `cmd/notify` generates the Slack summary via the Claude client
      instead of OpenAI, with no `OPENAI_API_KEY` required.
- [ ] At no point does `cmd/notify` construct or call both clients in the
      same run — exactly one `ChatCompleter` is built, per `LLM_PROVIDER`.
- [ ] Both clients satisfy the same `ChatCompleter` interface and produce
      a `Result` (summary text, token counts, estimated cost) in the same
      shape, so `cmd/notify` and `slack.PostSummary` need no
      provider-specific branching beyond client construction.
- [ ] `go test ./...` passes, including new Claude client and updated
      config tests.
- [ ] README documents both providers' setup clearly enough that a new
      user can pick one without reading the Go source.

## Main risk

Anthropic's Messages API has a genuinely different request/response shape
from OpenAI's Chat Completions API (system-as-field vs. system-as-message,
block-based content, different usage field names, different rate-limit
error signalling) — a naive port that reuses `client.go`'s structs will
silently produce wrong requests or fail to parse responses. The second risk
is pricing/model-name accuracy (see the linked GPT pricing task spec for
the same problem on the OpenAI side): Claude model names and prices must
come from a live, authoritative source at implementation time, not be
guessed.
