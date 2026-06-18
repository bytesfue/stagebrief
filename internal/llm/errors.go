package llm

import "errors"

// ErrQuotaExceeded is returned when the API key has no remaining credits.
var ErrQuotaExceeded = errors.New("llm quota exceeded")

// ErrAPIError is returned for other non-retryable API errors.
var ErrAPIError = errors.New("llm API error")
