# Architecture

## 1. Purpose and scope

This document describes the target architecture for the Kubernetes
Playground: a local, reproducible cloud-native environment built with
kind, Terraform and Floci.

The project has two goals:

1.  Build a Kubernetes platform that is rich enough to practise
    day-to-day platform engineering.
2.  Run a deliberately small application that exercises the platform's
    networking, storage, deployment, observability and AWS-integration
    capabilities.

The application is not the main challenge. The platform and the
operational learning are.

### In scope

-   Local multi-node Kubernetes using kind.
-   Terraform-managed cluster lifecycle and local AWS-compatible
    resources.
-   Floci integration for selected AWS APIs.
-   CNI, ingress, storage and secrets integration.
-   A small API, frontend and background worker.
-   Metrics, logs, traces and alerting.
-   Helm-based packaging and Argo CD GitOps.
-   Autoscaling, policy, backup/restore and failure exercises.

### Out of scope

-   Production-grade high availability.
-   Real EC2 provisioning or realistic EC2 networking.
-   Full AWS parity from Floci.
-   A highly available database or distributed storage system as a first
    milestone.
-   Public production exposure or real customer data.

## 2. High-level system

``` text
┌──────────────────────────── Local workstation ───────────────────────────┐
│                                                                          │
│  Developer tools (Nix flake + direnv)                                    │
│  Terraform · kind · kubectl · Helm · Git · Podman                        │
│       │                                                                  │
│       ├──────────── Terraform providers ────────────┐                    │
│       │                                             │                    │
│       ▼                                             ▼                    │
│  kind cluster                                  Floci services            │
│  (Podman containers)                           S3 / SQS / IAM / ...      │
│       │                                             ▲                    │
│       │                                             │                    │
│       └──────── Kubernetes workloads ───────────────┘                    │
│                                                                          │
│  ┌──────────────────────────── kind ─────────────────────────────────┐  │
│  │                                                                    │  │
│  │  Control plane              Worker 1             Worker 2          │  │
│  │  ─────────────              ────────             ────────          │  │
│  │  API server                 Traefik              API               │  │
│  │  scheduler                  Frontend             Worker            │  │
│  │  controller manager         Observability        PostgreSQL        │  │
│  │  etcd                       Argo CD              Redis             │  │
│  │                                                                    │  │
│  │  Cilium networking · Services · DNS · Storage · NetworkPolicies    │  │
│  └────────────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────────────┘
```

The diagram is logical, not a strict pod-placement guarantee. Kubernetes
scheduling decides where pods run unless node affinity, taints or other
constraints are configured.

## 3. Important local-environment distinction

kind runs Kubernetes nodes as containers managed by Podman. A three-node
kind cluster gives useful practice with Kubernetes scheduling, node
labels, disruptions and multi-node networking, but it does not create
three independent virtual machines.

Floci provides local AWS-compatible service endpoints. It is not an EC2
hypervisor and does not make kind nodes behave like EC2 instances. This
architecture therefore uses:

-   kind for Kubernetes.
-   Terraform for declarative provisioning.
-   Floci for supported AWS-compatible service APIs.
-   Podman networking and kind port mappings for local connectivity.

If the learning objective later expands to EC2 instance lifecycle, Linux
node administration or realistic VPC routing, add a separate VM-based
lab rather than pretending kind covers those topics.

## 4. Infrastructure and provisioning layer

### 4.1 Terraform

Terraform is the entry point for repeatable infrastructure provisioning.

Responsibilities:

-   Declare required provider versions.
-   Configure the kind provider.
-   Create and configure the kind cluster.
-   Configure the AWS provider to use Floci endpoints for services
    supported by the chosen Floci release.
-   Create AWS-compatible resources such as buckets and queues where
    supported.
-   Export useful outputs without exposing secrets.

Suggested ownership boundary:

  -----------------------------------------------------------------------
  Resource                            Owner
  ----------------------------------- -----------------------------------
  kind cluster and node configuration Terraform kind provider

  Floci S3 bucket / SQS queue / IAM   Terraform AWS provider
  resources                           

  CNI and platform Helm releases      Helm initially; optionally Argo CD
                                      later

  Application Kubernetes resources    Argo CD after GitOps is introduced

  Application container images        CI build process / local image
                                      build

  Application data                    PostgreSQL and backup process
  -----------------------------------------------------------------------

Do not have Terraform and Argo CD manage the same Kubernetes objects. A
clean migration is to use Terraform only for the cluster, then bootstrap
Argo CD and let it own the Git-managed platform and application
resources.

### 4.2 kind cluster

Initial topology:

-   One control-plane node.
-   Two worker nodes.
-   A stable cluster name, such as `playground`.
-   Host port mappings for HTTP and HTTPS ingress access.
-   A documented Kubernetes version.
-   A documented CNI installation strategy.

Use kind's default networking for the first smoke test if necessary.
When ready to learn CNI installation, configure kind with its default
CNI disabled and install Cilium as the cluster CNI. Do not disable the
default CNI until the Cilium installation procedure is ready.

For a local ingress route, map host ports to NodePorts on the kind
control-plane container, then configure the ingress controller service
to use those NodePorts. Verify the exact port mapping and service type
together; a host mapping alone does not expose an arbitrary Kubernetes
Service.

kind runs on Podman through its Podman provider. Select the provider
explicitly with `KIND_EXPERIMENTAL_PROVIDER=podman` (set in `.envrc`)
rather than relying on auto-detection, and confirm that the pinned
Terraform kind provider honours it. kind's Podman support is still
labelled experimental, so pin the kind version and record any
workarounds. Podman changes a few details of the design:

-   Rootless Podman needs cgroup v2 with the `cpu`, `cpuset`, `io`,
    `memory` and `pids` controllers delegated to your user. See kind's
    rootless guide for the systemd settings.
-   Rootless processes cannot bind host ports below 1024 by default.
    Map ingress to high ports such as 8080 and 8443, or lower
    `net.ipv4.ip_unprivileged_port_start` deliberately.
-   On macOS, Podman runs inside a Linux VM (`podman machine`). kind
    generally needs that machine to be rootful, and its CPU and memory
    should be sized for the platform you intend to run.

### 4.3 Floci

Run Floci locally as a container, started with `podman-compose`, and
configure Terraform's AWS provider with the endpoint settings required
by the current Floci version.

Candidate integrations:

  Service           Intended exercise
  ----------------- ----------------------------------------------------------------
  S3                PostgreSQL backup objects and restore
  SQS               Queue-based background checks
  IAM               Least-privilege policies for application access
  Secrets Manager   Store application credentials
  ECR               Store application images, if supported by the selected version
  SNS               Optional status-change notification
  DynamoDB          Optional extension, not required for the core application

Treat this list as a target, not a guarantee of service parity. Before
implementing a feature, verify that the current Floci version supports
the required API operations, Terraform resource behaviour and
authentication flow.

Connectivity is a key design concern. A workload in kind must connect to
the Floci endpoint through an address reachable from the Podman network.
`localhost` inside a pod refers to that pod, not the host machine. Use a
documented host gateway or Podman-network route (Podman's
`host.containers.internal` name is a common starting point, but check
that pods can actually resolve it) and test it from a temporary pod
before wiring it into the application.

### 4.4 Developer environment

Command-line tooling comes from a Nix flake (`flake.nix`, with
`flake.lock` pinning exact versions) that direnv activates when you
enter the repository (`.envrc`). Everyone gets the same Terraform,
kind, kubectl, Helm and supporting CLIs without installing them by
hand, and versions change only when the lock file does.

Two things deliberately stay outside the flake:

-   Podman. Rootless Podman needs host integration that a dev shell
    cannot provide: user namespace mappings, cgroup delegation and, on
    macOS, a Linux VM. Install it with your operating system's package
    manager or, on NixOS, its Podman module.
-   Local settings and credentials. These live in an untracked `.env`
    file that `.envrc` loads with direnv's `dotenv` helper. Only
    `.env.example` is committed.

`.envrc` also exports `KIND_EXPERIMENTAL_PROVIDER=podman`.

## 5. Kubernetes platform layer

### 5.1 Networking: Cilium

Cilium is the cluster CNI and provides pod networking and Kubernetes
NetworkPolicy enforcement.

Learning objectives:

-   Understand pod-to-pod and pod-to-service traffic.
-   Inspect endpoints and network state.
-   Apply default-deny policies.
-   Permit only the API to connect to PostgreSQL.
-   Observe the effect of a policy that blocks a dependency.

Start with a conservative configuration that retains kube-proxy unless
there is a specific exercise for kube-proxy replacement. Advanced eBPF
features can be added after basic connectivity works.

Confirm early that Cilium runs under your chosen Podman mode (rootless
or rootful). Its eBPF features may need privileges that rootless mode
does not grant.

### 5.2 Ingress: Traefik

Traefik provides the HTTP entry point into the cluster.

Expected routes:

-   `status.local.test/` → frontend service.
-   `status.local.test/api` → API service.
-   Optional `grafana.local.test` → Grafana during local-only
    development.

Use kind host port mappings and a NodePort service for local access. Add
local host-file entries if using named local domains. Do not expose
dashboards or Floci publicly.

### 5.3 TLS: cert-manager

Introduce cert-manager after basic HTTP ingress works.

Use a local CA or self-signed issuer for the lab. This provides practice
with Issuer, Certificate and TLS Secret resources without requiring
public DNS or public certificate issuance.

### 5.4 Storage

Begin with the simplest storage class that works reliably in kind, such
as a local-path provisioner.

Use it for PostgreSQL's persistent volume claim. Document that this
storage is local to the kind/Podman environment and is not equivalent to
resilient cloud block storage.

Longhorn can be an optional later experiment, but it adds resource
overhead and storage complexity. Do not make it a prerequisite for the
core project.

### 5.5 Metrics Server and autoscaling

Install Metrics Server when ready to practise resource-based
autoscaling. Verify that it can collect metrics from kind nodes; local
cluster configurations may need specific TLS or kubelet settings.

Use:

-   Metrics Server for resource metrics.
-   HPA to scale the API or worker based on CPU/memory.
-   KEDA later to scale the worker based on SQS queue depth, if the
    chosen Floci SQS implementation and scaler integration work
    together.

### 5.6 Secrets

Kubernetes Secrets are the native destination for credentials, but
base64 encoding is not encryption.

Start with local development secrets supplied securely outside Git.
Later, introduce External Secrets Operator to synchronize from Floci
Secrets Manager if the provider and API behaviour are compatible.

Never commit actual secret values, Terraform state containing secrets,
kubeconfig files or generated credentials.

## 6. Application: Status Tracker

### 6.1 Functional concept

The Status Tracker monitors a small set of configured HTTP endpoints.

A user can:

1.  Register a service with a name and URL.
2.  View registered services.
3.  See the latest check result.
4.  Inspect recent check history.
5.  See whether a service is healthy, degraded or unavailable.

Keep the product deliberately small. Avoid accounts, billing, complex
permissions or elaborate UI work.

### 6.2 Components

  -----------------------------------------------------------------------
  Component               Responsibility          Kubernetes workload
  ----------------------- ----------------------- -----------------------
  Frontend                Dashboard and user      Deployment + Service
                          interaction             

  API                     CRUD operations and     Deployment + Service
                          status queries          

  Worker                  Executes periodic       Deployment initially
                          endpoint checks         

  PostgreSQL              Service definitions and StatefulSet or
                          check history           chart-managed
                                                  database + PVC

  Redis                   Optional cache or local Deployment or
                          job queue               chart-managed service
  -----------------------------------------------------------------------

The worker can initially poll PostgreSQL for due checks or use Redis.
Once the core application works, introduce SQS as an alternative queue
backend or job-dispatch path. This staged approach keeps the first
application implementation simple.

### 6.3 API sketch

Example endpoints:

``` http
GET    /healthz
GET    /readyz
GET    /api/services
POST   /api/services
GET    /api/services/{id}
GET    /api/services/{id}/checks
GET    /api/services/{id}/status
DELETE /api/services/{id}
```

Use `/healthz` for process liveness and `/readyz` for readiness,
including required dependency checks where appropriate. Avoid making
liveness depend on a database: a database outage should not cause
endless API restarts.

### 6.4 Data model

A minimal relational model:

-   `services`: id, name, url, interval_seconds, enabled, created_at.
-   `checks`: id, service_id, checked_at, status_code, latency_ms,
    success, error_summary.

Add indexes for service history queries and retention/cleanup for old
check records.

### 6.5 Observability requirements in the application

The API and worker should provide:

-   Structured JSON logs to stdout/stderr.
-   Request count, request latency and error metrics.
-   Worker checks processed, failed checks and queue depth where
    available.
-   Trace context propagation between API, queue and worker when
    asynchronous processing is introduced.
-   Health and readiness endpoints.
-   Configurable log level and service identity.

Use OpenTelemetry instrumentation rather than implementing a custom
tracing format.

## 7. Observability architecture

The observability stack should help answer three different questions:

-   Metrics: what is happening, and how often?
-   Logs: what happened in this particular event?
-   Traces: where did a request spend its time across components?

### 7.1 Metrics

Deploy kube-prometheus-stack, which provides Prometheus, Grafana,
Alertmanager and common Kubernetes monitoring components.

Add:

-   kube-state-metrics for Kubernetes object state.
-   Node exporter for node-level metrics.
-   Application Prometheus metrics.
-   Dashboards for cluster capacity, pod restarts, API latency and
    worker activity.

Example alerts:

-   Pod repeatedly restarting.
-   Node or pod memory pressure.
-   API error ratio above a threshold.
-   No successful endpoint checks for an extended period.
-   Backup job failure.

### 7.2 Logs

Deploy Loki and Grafana Alloy.

-   Applications write structured logs to stdout/stderr.
-   Alloy collects container logs and forwards them to Loki.
-   Grafana queries logs and correlates them with metrics and traces.

Avoid high-cardinality labels such as request IDs, full URLs or
user-generated values in Loki labels. Put those values in the log body
instead.

### 7.3 Traces

Deploy Tempo and instrument the API and worker with OpenTelemetry.

A useful trace should show:

1.  Request received by the API.
2.  Database query or queue publish.
3.  Worker receives a job.
4.  HTTP endpoint check.
5.  Result persisted in PostgreSQL.

For asynchronous work, propagate trace context in the queue message so
the worker span can be associated with the originating operation.

### 7.4 Resource planning

Prometheus, Grafana, Loki and Tempo can consume significant memory and
disk relative to a small local cluster. Install them incrementally, set
resource requests/limits and retention limits, and start with modest
storage. Do not assume a laptop can comfortably run every component at
production scale.

## 8. Delivery and GitOps

### 8.1 Image build

Build frontend, API and worker images locally with `podman build` or in
CI. Use explicit image tags, ideally immutable commit SHA tags.

For the initial local loop, export each image with `podman save` and
load the archive into kind using `kind load image-archive`. This avoids
requiring a registry before the application is running. The archive
route works with any container engine, whereas `kind load docker-image`
targets the Docker CLI.

Podman qualifies locally built images with `localhost/`, so
`status-tracker-api:abc123` is stored as
`localhost/status-tracker-api:abc123`. Use the full name in Helm values
and set the pull policy to `IfNotPresent` so the kubelet uses the loaded
image instead of trying to pull it.

Later, push images to Floci ECR if the selected Floci version supports
the required registry workflow. Verify Podman registry authentication
(`podman login`), image pull credentials and node-to-registry
connectivity.

### 8.2 Helm

Package the Status Tracker as a Helm chart.

The chart should define:

-   Deployments and Services.
-   ConfigMaps and Secret references.
-   Ingress routes.
-   Resource requests and limits.
-   Liveness and readiness probes.
-   Service accounts.
-   Optional HPA.
-   PostgreSQL/Redis dependency configuration only if deliberately
    chosen.

Keep environment-specific values in separate values files, with secrets
injected at deployment time rather than committed.

### 8.3 Argo CD

Introduce Argo CD after a manual Helm deployment is understood.

A sensible ownership model:

-   Terraform creates the cluster and Floci infrastructure.
-   A documented bootstrap step installs Argo CD.
-   Argo CD manages platform and application Helm releases/manifests
    from Git.
-   Terraform does not also manage those same Helm releases or
    Kubernetes objects.

Start with manual sync, inspect diffs, then enable automated sync and
pruning once the repository structure and ownership boundaries are
stable.

## 9. Backup and recovery

The primary recovery exercise is PostgreSQL backup to S3-compatible
object storage in Floci.

Proposed workflow:

1.  A Kubernetes CronJob runs `pg_dump`.
2.  The dump is compressed and uploaded to an S3 bucket.
3.  The job reports success/failure through logs and metrics.
4.  Grafana displays the latest backup result and age.
5.  A restore procedure creates a fresh database and imports a selected
    dump.

A backup is not proven until a restore has succeeded. Document the
restore procedure and test it.

Velero can be explored later for Kubernetes resource backups, but it
does not replace a database-aware backup and restore strategy.

## 10. Security and policy

Introduce security controls progressively:

-   Non-root containers where practical.
-   Read-only root filesystems where compatible.
-   Resource requests and limits.
-   Dedicated ServiceAccounts.
-   Least-privilege Kubernetes RBAC.
-   NetworkPolicies for API/database/worker boundaries.
-   Kyverno policies for image tags, security context and required
    labels.
-   Trivy image scanning in CI.
-   Secrets excluded from Git and Terraform outputs.

The lab should demonstrate useful controls without becoming a security
product in its own right.

## 11. Key data flows

### 11.1 User request

``` text
Browser
  → Traefik
  → Frontend
  → API
  → PostgreSQL
```

### 11.2 Endpoint check

``` text
Worker
  → obtains due service/job
  → checks configured HTTP endpoint
  → stores result in PostgreSQL
  → emits metrics and structured logs
```

### 11.3 Queue-based extension

``` text
API / scheduler
  → Floci SQS
  → worker
  → endpoint check
  → PostgreSQL
```

### 11.4 Backup

``` text
Kubernetes CronJob
  → pg_dump
  → compressed dump
  → Floci S3
```

### 11.5 Observability

``` text
Application + Kubernetes
  ├── metrics → Prometheus → Grafana
  ├── logs → Alloy → Loki → Grafana
  └── traces → OpenTelemetry → Tempo → Grafana
```

## 12. Decisions and trade-offs

  -------------------------------------------------------------------------------
  Decision                        Reason                  Trade-off
  ------------------------------- ----------------------- -----------------------
  kind instead of VM-based        Fast, reproducible      Does not simulate real
  Kubernetes                      local cluster           EC2 nodes

  Podman instead of Docker        Daemonless, rootless-   kind's Podman provider
                                  capable container       is experimental; setup
                                  runtime                 differs per OS

  Nix flake + direnv for          Pinned, identical       Requires Nix; Podman
  tooling                         CLI toolchain for       itself is installed on
                                  every contributor       the host

  Floci instead of real AWS       Local AWS API practice  Service/API parity must
                                                          be checked

  Cilium                          Strong networking and   More setup than default
                                  policy learning         kind networking

  Traefik                         Straightforward ingress Local port mapping
                                                          still needs
                                                          configuration

  Local-path storage first        Low complexity          Not resilient or
                                                          portable storage

  PostgreSQL                      Familiar persistent     Stateful workload adds
                                  relational data         operational complexity

  Prometheus/Grafana/Loki/Tempo   Covers metrics, logs    Significant local
                                  and traces              resource use

  Helm + Argo CD                  Clear packaging and     Requires deliberate
                                  GitOps separation       ownership boundaries
  -------------------------------------------------------------------------------

## 13. Definition of done

The project is complete when:

-   A documented command sequence provisions the local Floci
    dependencies and kind cluster.
-   The cluster has one control-plane node and two workers in `Ready`
    state.
-   Cilium, ingress and persistent storage work.
-   The Status Tracker is reachable from the host.
-   PostgreSQL data persists across application pod restarts.
-   Metrics, logs and traces can be queried in Grafana.
-   Application images can be built and deployed repeatably.
-   Argo CD can reconcile the application from Git.
-   At least one workload integrates with a Floci service.
-   A PostgreSQL backup is uploaded to S3 and successfully restored.
-   A worker failure and a blocked network path have been tested and
    diagnosed.
-   Teardown and recreation procedures are documented, including what
    data is lost.
