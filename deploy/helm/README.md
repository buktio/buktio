# Helm chart (future)

This directory will hold the Kubernetes Helm chart, mirroring the compose graph:
web + api Deployments, PostgreSQL (in-cluster or external), and Garage as a
**StatefulSet** with PersistentVolumeClaims for the meta and data dirs.

Design constraints (see the development plan §13):
- Garage admin (`:3903`) + RPC (`:3901`) exposed only as `ClusterIP` (never a
  `LoadBalancer`); only the edge/ingress is public.
- Secrets in a `Secret` / values; the bootstrap reuses `internal/garagemanager`
  as a Job/initContainer.
- A future `Buktio` CRD + operator generalizes the same idempotent Admin-API
  layout bootstrap to multi-node.
