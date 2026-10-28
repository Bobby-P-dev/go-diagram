package llm

import (
	"math/rand"
	"net/http"
	"strings"
	"time"
)

// retryableHTTPStatus reports whether an upstream status code is worth re-trying.
// Retry transient/router overloads (429 rate-limit, 502 Bad Gateway, 503/504, Cloudflare 520-524/529 proxy issues) but
// never deterministic client errors (400/401/403/404/422), which will fail again.
func retryableHTTPStatus(code int) bool {
	switch code {
	case http.StatusBadGateway, // 502 (TokenomixAI / Cloudflare origin overloaded/down)
		http.StatusTooManyRequests,    // 429
		http.StatusServiceUnavailable, // 503
		http.StatusGatewayTimeout,     // 504
		520,                           // Cloudflare Web Server Returns an Unknown Error
		521,                           // Cloudflare Web Server Is Down
		522,                           // Cloudflare Connection Timed Out
		523,                           // Cloudflare Origin Is Unreachable
		524,                           // Cloudflare A Timeout Occurred
		529:                           // upstream overload (Anthropic/OpenRouter convention)
		return true
	default:
		return false
	}
}

// isAggregatorErrorText detects when third-party aggregator proxies (like TokenomixAI)
// return HTTP 200 with error descriptions in the body instead of returning HTTP 5xx.
func isAggregatorErrorText(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "ai provider temporarily unavailable") ||
		strings.Contains(lower, "upstreamresponsecode:") ||
		strings.Contains(lower, "upstream server error") ||
		strings.Contains(lower, "no available accounts")
}

// retryBackoff returns an exponential backoff with jitter for a 1-based attempt,
// capped so a burst of retries cannot stack into unbounded latency.
// attempt 1 -> ~250-500ms, 2 -> ~500-1000ms, 3 -> ~1-2s.
func retryBackoff(attempt int) time.Duration {
	if attempt > 4 {
		return 4 * time.Second
	}
	base := time.Duration(1<<uint(attempt-1)) * 250 * time.Millisecond
	jitter := time.Duration(rand.Intn(int(base))) // full jitter avoids thundering herd
	return base + jitter
}
