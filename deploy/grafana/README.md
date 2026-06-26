# Grafana dashboard

[`buktio.json`](buktio.json) is an importable Grafana dashboard for buktio's storage and
S3 metrics (request/error rates, disk utilisation, resync backlog).

## Import

1. Point a Prometheus instance at buktio's metrics. buktio exposes its own `/metrics`
   and proxies the Garage engine metrics; scrape the API service (guard `/metrics` with
   `BUKTIO_METRICS_TOKEN` if it's reachable off-box).
2. In Grafana: **Dashboards → New → Import → Upload JSON file**, pick `buktio.json`, and
   select your Prometheus data source for the `DS_PROMETHEUS` input.

The panels query Garage metric families (`api_s3_request_counter`, `api_s3_error_counter`,
`garage_local_disk_avail` / `garage_local_disk_total`, `block_resync_queue_length`) — the
same data the in-app Ops page shows, for users who already run Grafana.
