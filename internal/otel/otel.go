// Package otel emits OpenTelemetry-style trace spans for keramos operations.
// To avoid pulling in the heavy `go.opentelemetry.io` SDK we implement just
// the minimum: span IDs, durations, attributes, JSON-line output.
//
// When KERAMOS_OTEL_ENDPOINT is set, spans are POSTed to that URL on End() in
// the OTLP/HTTP/JSON wire format. Otherwise they go to stderr at debug level.
package otel

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/ebogdum/keramos/internal/netguard"
)

// Span represents one operation. Use Start to construct, then call End when
// the operation finishes. Attributes captured before End are included.
type Span struct {
	traceID    string
	spanID     string
	parentID   string
	name       string
	startTime  time.Time
	endTime    time.Time
	attrs      map[string]any
	mu         sync.Mutex
	finished   bool
}

// Tracer is a process-wide context for issuing spans.
type Tracer struct {
	endpoint string
	service  string
}

// validateEndpoint rejects KERAMOS_OTEL_ENDPOINT values that are not https://
// unless the operator explicitly opts into plaintext via
// KERAMOS_OTEL_ALLOW_PLAINTEXT=1. Span attrs may include error messages
// containing rendered values; shipping them over plaintext or to attacker-
// controlled hosts is the wrong default.
func validateEndpoint(raw string) string {
	if "" == raw {
		return ""
	}
	u, err := url.Parse(raw)
	if nil != err {
		return ""
	}
	if "https" == u.Scheme {
		return raw
	}
	if "http" == u.Scheme && "1" == os.Getenv("KERAMOS_OTEL_ALLOW_PLAINTEXT") {
		return raw
	}
	return ""
}

var defaultTracer = &Tracer{
	endpoint: validateEndpoint(os.Getenv("KERAMOS_OTEL_ENDPOINT")),
	service:  envOr("KERAMOS_OTEL_SERVICE", "keramos"),
}

// sanitiseAttrs caps the size of any single attribute value so a multi-MB
// rendered manifest in an `error` attribute can't be exfiltrated wholesale
// through the telemetry channel.
func sanitiseAttrs(attrs map[string]any) map[string]any {
	const max = 512
	out := make(map[string]any, len(attrs))
	for k, v := range attrs {
		if s, ok := v.(string); ok && len(s) > max {
			out[k] = s[:max] + "…(truncated)"
			continue
		}
		out[k] = v
	}
	return out
}

// Start begins a new top-level span.
func Start(name string) *Span {
	return defaultTracer.StartSpan(name, nil)
}

// StartChild begins a span with `parent` as the parent.
func StartChild(parent *Span, name string) *Span {
	return defaultTracer.StartSpan(name, parent)
}

// StartSpan creates a span with optional parent.
func (t *Tracer) StartSpan(name string, parent *Span) *Span {
	traceID := ""
	parentID := ""
	if nil != parent {
		traceID = parent.traceID
		parentID = parent.spanID
	} else {
		traceID = randHex(16)
	}
	return &Span{
		traceID:   traceID,
		spanID:    randHex(8),
		parentID:  parentID,
		name:      name,
		startTime: time.Now().UTC(),
		attrs:     make(map[string]any, 4),
	}
}

// SetAttr attaches a typed attribute. Safe to call concurrently.
func (s *Span) SetAttr(key string, value any) {
	if nil == s {
		return
	}
	s.mu.Lock()
	s.attrs[key] = value
	s.mu.Unlock()
}

// End finalises the span, emitting it to the configured exporter.
func (s *Span) End() {
	if nil == s {
		return
	}
	s.mu.Lock()
	if s.finished {
		s.mu.Unlock()
		return
	}
	s.finished = true
	s.endTime = time.Now().UTC()
	s.mu.Unlock()
	defaultTracer.export(s)
}

// EndWithError finalises and records an error.
func (s *Span) EndWithError(err error) {
	if nil == s {
		return
	}
	if nil != err {
		s.SetAttr("error", err.Error())
	}
	s.End()
}

func (t *Tracer) export(s *Span) {
	rec := map[string]any{
		"service":    t.service,
		"name":       s.name,
		"traceId":    s.traceID,
		"spanId":     s.spanID,
		"parentId":   s.parentID,
		"startNanos": s.startTime.UnixNano(),
		"endNanos":   s.endTime.UnixNano(),
		"durationMs": float64(s.endTime.Sub(s.startTime).Microseconds()) / 1000.0,
		"attrs":      sanitiseAttrs(s.attrs),
	}
	data, err := json.Marshal(rec)
	if nil != err {
		return
	}
	if "" != t.endpoint {
		go postSilently(t.endpoint, data)
		return
	}
	if "" != os.Getenv("KERAMOS_OTEL_STDERR") {
		os.Stderr.Write(append(data, '\n'))
	}
}

// telemetryClient bounds telemetry POST attempts so they cannot stall keramos
// operations on a slow link, and routes through the SSRF-guarded dialer so the
// telemetry path can't be pointed at the cloud metadata endpoint.
var telemetryClient = netguard.HTTPClient(netguard.BlockMetadata, "KERAMOS_ALLOW_INTERNAL_FETCH", 5*time.Second)

func postSilently(endpoint string, body []byte) {
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if nil != err {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := telemetryClient.Do(req)
	if nil != err {
		return
	}
	resp.Body.Close()
}

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); nil != err {
		return ""
	}
	return hex.EncodeToString(b)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); "" != v {
		return v
	}
	return def
}
