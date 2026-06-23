// Command buktio-s3proxy is a thin counting reverse proxy that fronts the Garage
// S3 plane. It forwards every request unchanged (Garage validates SigV4) and counts
// per-(access-key, bucket, method) requests + bytes, flushing them to PostgreSQL so
// buktio can report per-key traffic/egress — data Garage does not expose.
//
// It NEVER reads or buffers object payloads beyond counting transferred bytes, and
// never holds any Garage admin token. Deployed on the internal network behind Caddy:
//
//	Caddy s3.<domain>  ->  buktio-s3proxy:3900  ->  garage:3900
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/buktio/buktio/internal/db"
	"github.com/buktio/buktio/internal/proxy"
	"github.com/buktio/buktio/internal/repository"
)

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	addr := getenv("BUKTIO_S3PROXY_ADDR", ":3900")
	upstreamRaw := getenv("BUKTIO_S3PROXY_UPSTREAM", "http://garage:3900")
	dbURL := os.Getenv("DATABASE_URL")

	// Optional cluster id; a malformed value would make every traffic-flush INSERT
	// fail the ::uuid cast and silently drop ALL counters, so validate it up front
	// and fall back to "" (recorded as NULL) when invalid.
	clusterID := os.Getenv("BUKTIO_S3PROXY_CLUSTER_ID")
	if clusterID != "" {
		if _, perr := uuid.Parse(clusterID); perr != nil {
			logger.Warn("invalid BUKTIO_S3PROXY_CLUSTER_ID; recording traffic without a cluster id",
				slog.String("value", clusterID))
			clusterID = ""
		}
	}

	flushInterval := 30 * time.Second
	if d, err := time.ParseDuration(getenv("BUKTIO_S3PROXY_FLUSH_INTERVAL", "30s")); err == nil {
		flushInterval = d
	}

	upstream, err := proxy.ParseUpstream(upstreamRaw)
	if err != nil {
		logger.Error("invalid upstream URL", slog.String("upstream", upstreamRaw), slog.Any("error", err))
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	agg := proxy.NewAggregator()
	p := proxy.New(upstream, agg)

	// Metering is best-effort: without a DB the proxy still forwards traffic, it just
	// doesn't record counters. The flusher runs under a WaitGroup so its final
	// drain on shutdown completes BEFORE the pool is closed (no lost last interval).
	var wg sync.WaitGroup
	var pool *pgxpool.Pool
	if dbURL != "" {
		var perr error
		pool, perr = db.OpenPool(ctx, dbURL)
		if perr != nil {
			logger.Error("cannot open database pool; traffic will not be recorded", slog.Any("error", perr))
			pool = nil
		} else {
			store := repository.NewStore(pool)
			wg.Add(1)
			go func() {
				defer wg.Done()
				p.RunFlusher(ctx, flushInterval, func(ctx context.Context, samples []proxy.Sample) error {
					rows := make([]repository.TrafficSample, 0, len(samples))
					for _, s := range samples {
						rows = append(rows, repository.TrafficSample{
							ClusterID: clusterID, AccessKeyID: s.AccessKeyID, Bucket: s.Bucket,
							Method: s.Method, Requests: s.Requests, BytesIn: s.BytesIn, BytesOut: s.BytesOut,
						})
					}
					return store.InsertTrafficSnapshots(ctx, rows)
				})
			}()
			logger.Info("traffic metering enabled", slog.Duration("flush_interval", flushInterval))
		}
	} else {
		logger.Warn("DATABASE_URL not set — forwarding without traffic metering")
	}

	srv := &http.Server{Addr: addr, Handler: p}
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()

	logger.Info("buktio-s3proxy listening", slog.String("addr", addr), slog.String("upstream", upstreamRaw))
	srvErr := srv.ListenAndServe()

	// Orderly shutdown: wait for the flusher's final drain, THEN close the pool.
	wg.Wait()
	if pool != nil {
		pool.Close()
	}
	if srvErr != nil && srvErr != http.ErrServerClosed {
		logger.Error("server error", slog.Any("error", srvErr))
		os.Exit(1)
	}
}
