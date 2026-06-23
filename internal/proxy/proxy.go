package proxy

import (
	"context"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"
)

// counter accumulates traffic for one Key over a flush interval.
type counter struct {
	Requests int64
	BytesIn  int64
	BytesOut int64
}

// Aggregator accumulates per-Key counters in memory and drains them on flush. It is
// safe for concurrent use.
type Aggregator struct {
	mu       sync.Mutex
	counters map[Key]*counter
}

// NewAggregator builds an empty aggregator.
func NewAggregator() *Aggregator {
	return &Aggregator{counters: map[Key]*counter{}}
}

// Add records one request's bytes for a Key.
func (a *Aggregator) Add(k Key, bytesIn, bytesOut int64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	c := a.counters[k]
	if c == nil {
		c = &counter{}
		a.counters[k] = c
	}
	c.Requests++
	c.BytesIn += bytesIn
	c.BytesOut += bytesOut
}

// Sample is one drained counter row.
type Sample struct {
	Key
	Requests int64
	BytesIn  int64
	BytesOut int64
}

// Drain atomically swaps out and returns the accumulated counters.
func (a *Aggregator) Drain() []Sample {
	a.mu.Lock()
	old := a.counters
	a.counters = map[Key]*counter{}
	a.mu.Unlock()

	out := make([]Sample, 0, len(old))
	for k, c := range old {
		out = append(out, Sample{Key: k, Requests: c.Requests, BytesIn: c.BytesIn, BytesOut: c.BytesOut})
	}
	return out
}

// countingWriter wraps a ResponseWriter to count response (egress) bytes while
// preserving streaming via Flush.
type countingWriter struct {
	http.ResponseWriter
	n int64
}

func (c *countingWriter) Write(b []byte) (int, error) {
	n, err := c.ResponseWriter.Write(b)
	c.n += int64(n)
	return n, err
}

func (c *countingWriter) Flush() {
	if f, ok := c.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Proxy is the counting reverse proxy.
type Proxy struct {
	rp  *httputil.ReverseProxy
	agg *Aggregator
}

// New builds a proxy forwarding to upstream (the Garage S3 endpoint).
func New(upstream *url.URL, agg *Aggregator) *Proxy {
	rp := httputil.NewSingleHostReverseProxy(upstream)
	// Preserve the upstream host header behavior for SigV4 path-style addressing:
	// Garage signs against the Host it receives; the client already signed against
	// the public host, so we keep the incoming Host (do not overwrite to upstream).
	orig := rp.Director
	rp.Director = func(r *http.Request) {
		host := r.Host
		orig(r)
		if host != "" {
			r.Host = host
		}
	}
	return &Proxy{rp: rp, agg: agg}
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	k := KeyFor(r)
	var bytesIn int64
	if r.ContentLength > 0 {
		bytesIn = r.ContentLength
	}
	cw := &countingWriter{ResponseWriter: w}
	p.rp.ServeHTTP(cw, r)
	p.agg.Add(k, bytesIn, cw.n)
}

// FlushFunc persists a drained batch of samples.
type FlushFunc func(ctx context.Context, samples []Sample) error

// RunFlusher periodically drains the aggregator and persists samples until ctx is
// cancelled (a final drain runs on shutdown).
func (p *Proxy) RunFlusher(ctx context.Context, interval time.Duration, flush FlushFunc) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			if s := p.agg.Drain(); len(s) > 0 {
				_ = flush(context.Background(), s)
			}
			return
		case <-ticker.C:
			if s := p.agg.Drain(); len(s) > 0 {
				if err := flush(ctx, s); err != nil {
					// Re-add nothing (best-effort metering); a dropped flush loses
					// one interval of counters but never blocks the data path.
					_ = err
				}
			}
		}
	}
}
