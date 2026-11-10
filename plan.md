# Implementation Plan

## How to use this plan

Work through the goals in order. Each goal has:

-   **Outcome:** what should exist when the goal is complete.
-   **Tasks:** concrete work to perform.
-   **Acceptance criteria:** observable checks that prove the goal is
    done.

Keep commits small and focused. Record commands, troubleshooting notes
and decisions in the repository. Do not move to the next stage while the
current stage is fundamentally broken.

The plan deliberately separates a minimal working platform from optional
advanced exercises. The final result is a local Kubernetes platform
running the Status Tracker application, with AWS-compatible integrations
and operational tooling.

------------------------------------------------------------------------

## Stage 0 --- Prepare the workstation

### Goal 0.1 --- Install and verify prerequisites

**Outcome:** the workstation can run Podman, and the pinned Terraform
and Kubernetes tooling is available through the Nix dev shell.

Tasks: - \[ \] Install Podman. On macOS, create and start a rootful
Podman machine. - \[ \] Allocate sufficient CPU, memory and disk to
Podman (the Podman machine on macOS; the host itself on Linux). - \[ \]
On Linux, enable cgroup v2 controller delegation for rootless Podman. -
\[ \] Install Nix with flakes enabled, and direnv. - \[ \] Enter the dev
shell (`direnv allow`) so Terraform, kind, kubectl, Helm and the other
CLI tools come from the flake. - \[ \] Install Git. - \[ \] Install
Floci using its current official instructions. - \[ \] Commit
`flake.lock` so tool versions are pinned.

Acceptance criteria: - \[ \] `podman version` and `podman info` succeed.
- \[ \] `terraform version` succeeds. - \[ \] `kind version` succeeds. -
\[ \] `kubectl version --client` succeeds. - \[ \] `helm version`
succeeds. - \[ \] Podman has enough resources to run a three-node
cluster and the planned platform components. - \[ \] The CLI checks
above pass from inside the Nix dev shell.

### Goal 0.2 --- Create the repository skeleton

**Outcome:** the repository has clear places for infrastructure,
platform configuration and application code.

Tasks: - \[ \] Create `README.md`, `architecture.md` and `plan.md`. - \[
\] Create `terraform/`, `floci/`, `kubernetes/` and `application/`. - \[
\] Add a `.gitignore`. - \[ \] Ignore Terraform state, `.terraform/`,
kubeconfig files, local environment files, generated credentials, local
database data and build artifacts. - \[ \] Add `.env.example` files
where configuration examples are needed. - \[ \] Add a rule that real
secrets must never be committed.

Acceptance criteria: - \[ \] The repository structure is understandable
without explanation. - \[ \] `git status` does not show generated state
or credentials. - \[ \] A fresh clone contains enough documentation to
understand the project.

------------------------------------------------------------------------

## Stage 1 --- Run Floci and provision kind

### Goal 1.1 --- Start Floci locally

**Outcome:** Floci is running and its endpoint is reachable from the
host.

Tasks: - \[ \] Add a Floci configuration or Compose file (run with
`podman-compose`) using the current supported installation method. - \[
\] Use fully qualified image names (for example `docker.io/...`),
because Podman may prompt for or reject unqualified short names. - \[ \]
Pin the Floci image/version where applicable. - \[ \] Configure a
predictable local endpoint. - \[ \] Read the current Floci
service-coverage documentation. - \[ \] Record which services and API
operations are supported by the selected version. - \[ \] Keep
credentials and local configuration out of Git.

Acceptance criteria: - \[ \] Floci starts reliably. - \[ \] A documented
health or service check succeeds. - \[ \] The endpoint and credentials
required by Terraform are documented without committing secrets.

### Goal 1.2 --- Configure Terraform providers

**Outcome:** Terraform can initialise the kind and AWS-compatible
providers.

Tasks: - \[ \] Add `terraform/versions.tf`. - \[ \] Declare and pin a
compatible Terraform version. - \[ \] Declare the kind provider,
initially `tehpsphinx/kind`. - \[ \] Declare the AWS provider. - \[ \]
Configure provider endpoints for Floci according to its current
documentation. - \[ \] Configure credentials using environment variables
or an untracked local file. - \[ \] Add provider lock file to Git. - \[
\] Document any AWS provider flags required for local endpoints and
path-style S3 access.

Acceptance criteria: - \[ \] `terraform init` succeeds from a clean
checkout. - \[ \] Provider versions are locked. - \[ \] No credentials
are present in committed Terraform files.

### Goal 1.3 --- Create the kind cluster with Terraform

**Outcome:** Terraform creates a three-node kind cluster.

Tasks: - \[ \] Define a cluster named `playground`. - \[ \] Configure
one control-plane node. - \[ \] Configure two worker nodes. - \[ \]
Select and document the Kubernetes node image/version. - \[ \] Configure
host port mappings for later ingress access (use ports above 1023, such
as 8080 and 8443, with rootless Podman). - \[ \] Add variables for
cluster name and node image. - \[ \] Add useful outputs, such as cluster
name and context. - \[ \] Set `KIND_EXPERIMENTAL_PROVIDER=podman` (in
`.envrc`) and confirm the pinned kind provider creates the cluster
through Podman; record any workaround. - \[ \] Apply the Terraform
configuration.

Acceptance criteria: - \[ \] `terraform apply` creates the cluster
successfully. - \[ \] `kind get clusters` lists `playground`. - \[ \]
`podman ps` shows one container per kind node. - \[ \]
`kubectl config get-contexts` shows the expected context. - \[ \]
`kubectl get nodes` shows one control-plane node and two workers. - \[
\] All nodes become `Ready` using the initial networking configuration.
- \[ \] The cluster can be destroyed and recreated using the documented
Terraform workflow.

### Goal 1.4 --- Prove basic Kubernetes operation

**Outcome:** a simple workload runs and is reachable through a basic
test path.

Tasks: - \[ \] Deploy a small web test workload. - \[ \] Expose it with
a Kubernetes Service. - \[ \] Verify pod-to-pod and Service
connectivity. - \[ \] Verify DNS resolution for a Kubernetes Service. -
\[ \] Inspect pods, events and logs. - \[ \] Delete a pod and observe
its replacement.

Acceptance criteria: - \[ \] The test pod reaches `Running`. - \[ \] The
Service returns the expected response. - \[ \] Cluster DNS works. - \[
\] A deleted Deployment pod is recreated automatically.

------------------------------------------------------------------------

## Stage 2 --- Install cluster networking and ingress

### Goal 2.1 --- Install Cilium

**Outcome:** Cilium provides pod networking and network policy support.

Tasks: - \[ \] Decide whether to recreate kind with its default CNI
disabled. - \[ \] Confirm Cilium runs under your Podman mode (rootless
or rootful); its eBPF features may need privileges that rootless mode
does not grant. - \[ \] If disabling the default CNI, prepare the Cilium
installation before applying the cluster configuration. - \[ \] Install
a pinned Cilium Helm chart version. - \[ \] Keep kube-proxy enabled
initially. - \[ \] Verify Cilium agent and operator health. - \[ \] Run
Cilium connectivity tests where appropriate. - \[ \] Document the
install and upgrade commands.

Acceptance criteria: - \[ \] Cilium components are healthy. - \[ \] Pods
on different kind nodes can communicate. - \[ \] ClusterIP Services
work. - \[ \] DNS works from application pods. - \[ \] A test pod can
reach another test pod on a different node.

### Goal 2.2 --- Install Traefik and configure local ingress

**Outcome:** HTTP traffic from the workstation reaches a Kubernetes
Service through Traefik.

Tasks: - \[ \] Install Traefik using Helm. - \[ \] Configure the
intended NodePort service ports. - \[ \] Match the NodePorts to kind's
host port mappings. - \[ \] Deploy a test application and Ingress or
IngressRoute. - \[ \] Configure a local hostname if desired. - \[ \]
Document the host URL and routing flow.

Acceptance criteria: - \[ \] A request from the workstation reaches the
test application through Traefik. - \[ \] Traefik routes at least two
paths or hostnames correctly. - \[ \] The ingress controller recovers if
its pod is restarted.

### Goal 2.3 --- Add local TLS

**Outcome:** local ingress can serve HTTPS using a lab certificate.

Tasks: - \[ \] Install cert-manager. - \[ \] Create a self-signed or
local CA Issuer. - \[ \] Issue a certificate for the local application
hostname. - \[ \] Configure Traefik to use the generated TLS Secret. -
\[ \] Document how the local CA is trusted, or how browser warnings are
expected.

Acceptance criteria: - \[ \] The certificate resource becomes Ready. -
\[ \] HTTPS reaches the application. - \[ \] The certificate issuer and
renewal behaviour are understood.

------------------------------------------------------------------------

## Stage 3 --- Add storage and application dependencies

### Goal 3.1 --- Install a simple StorageClass

**Outcome:** a workload can request and use persistent storage.

Tasks: - \[ \] Select a kind-compatible local storage provisioner. - \[
\] Install it with a pinned version. - \[ \] Inspect the available
StorageClasses. - \[ \] Create a PVC. - \[ \] Mount it in a test pod. -
\[ \] Write data, delete the pod and verify the data remains.

Acceptance criteria: - \[ \] The PVC binds successfully. - \[ \] A
replacement pod can read the data written by the previous pod. - \[ \]
The limitations and lifecycle of local kind storage are documented.

### Goal 3.2 --- Deploy PostgreSQL

**Outcome:** PostgreSQL runs with persistent storage and can be reached
by an in-cluster client.

Tasks: - \[ \] Select a maintained Helm chart or a small chart of your
own. - \[ \] Pin the chart and database image versions. - \[ \]
Configure a PVC. - \[ \] Configure credentials outside Git. - \[ \] Set
resource requests and limits. - \[ \] Configure probes. - \[ \] Create a
temporary client pod and test connectivity. - \[ \] Test data
persistence across a PostgreSQL pod restart.

Acceptance criteria: - \[ \] PostgreSQL becomes Ready. - \[ \] An
in-cluster client can connect. - \[ \] Data survives a pod restart. - \[
\] Credentials are not committed.

### Goal 3.3 --- Decide how Redis is used

**Outcome:** the project has a clear queue/cache design rather than an
unnecessary dependency.

Tasks: - \[ \] Decide whether Redis is needed for the first application
version. - \[ \] If used, define whether it is a cache, a development
queue or both. - \[ \] Deploy Redis with explicit persistence
expectations. - \[ \] Document how the later SQS integration will
differ. - \[ \] Avoid making both Redis and SQS mandatory for the same
initial workflow.

Acceptance criteria: - \[ \] The application architecture names the role
of Redis. - \[ \] The API or worker can connect to it if the selected
design requires it. - \[ \] Redis can be removed without breaking
unrelated platform components.

------------------------------------------------------------------------

## Stage 4 --- Build the Status Tracker application

### Goal 4.1 --- Implement the API

**Outcome:** an API can register services and return their current
status.

Tasks: - \[ \] Choose Go or Python/FastAPI. - \[ \] Create the API
project and container build. - \[ \] Implement `GET /healthz` and
`GET /readyz`. - \[ \] Implement service registration, listing and
deletion. - \[ \] Implement service status and check-history
endpoints. - \[ \] Add database migrations. - \[ \] Add input validation
and sensible timeouts. - \[ \] Write unit and integration tests. - \[ \]
Emit structured JSON logs.

Acceptance criteria: - \[ \] API tests pass. - \[ \] The API can create
and retrieve a service record. - \[ \] Health and readiness endpoints
behave as documented. - \[ \] Invalid input returns useful client
errors. - \[ \] The API runs locally in a container.

### Goal 4.2 --- Implement the checker worker

**Outcome:** a background process checks registered HTTP endpoints and
records results.

Tasks: - \[ \] Implement the check execution loop. - \[ \] Add
configurable request timeout and check interval. - \[ \] Record status
code, latency, timestamp and failure summary. - \[ \] Handle unreachable
endpoints without crashing the worker. - \[ \] Add graceful shutdown. -
\[ \] Add structured logs. - \[ \] Write tests for success, HTTP
failure, timeout and DNS/network error cases.

Acceptance criteria: - \[ \] The worker checks a registered endpoint. -
\[ \] Results are persisted in PostgreSQL. - \[ \] A failing endpoint is
recorded as a failure rather than crashing the worker. - \[ \] The
worker can be stopped and restarted cleanly.

### Goal 4.3 --- Implement the frontend

**Outcome:** a minimal dashboard displays registered services and their
status.

Tasks: - \[ \] Build a simple services list. - \[ \] Show current state
and latest check time. - \[ \] Add a basic service registration form. -
\[ \] Show recent check history. - \[ \] Configure API base URL through
environment/configuration. - \[ \] Build a container image.

Acceptance criteria: - \[ \] The dashboard lists services from the
API. - \[ \] A user can register a service. - \[ \] Status updates
appear after checks run. - \[ \] Frontend and API can run locally as
containers.

### Goal 4.4 --- Package the application with Helm

**Outcome:** one Helm release deploys the application components into
Kubernetes.

Tasks: - \[ \] Create a Status Tracker Helm chart. - \[ \] Define API,
worker and frontend Deployments. - \[ \] Define Services where needed. -
\[ \] Add Ingress configuration. - \[ \] Add ConfigMaps and Secret
references. - \[ \] Add probes and resource requests/limits. - \[ \]
Configure PostgreSQL connection settings. - \[ \] Add chart values for
image tags and replica counts. - \[ \] Build the development images with
`podman build` and load them into kind with `podman save` and
`kind load image-archive`, referencing them by their `localhost/` names.
- \[ \] Deploy and test the application.

Acceptance criteria: - \[ \] `helm lint` succeeds. - \[ \] Helm installs
the release successfully. - \[ \] All application pods become Ready. -
\[ \] The application is reachable from the workstation. - \[ \]
Application data persists when API and worker pods restart.

------------------------------------------------------------------------

## Stage 5 --- Add observability

### Goal 5.1 --- Install Prometheus and Grafana

**Outcome:** cluster and application metrics are visible in Grafana.

Tasks: - \[ \] Install kube-prometheus-stack using a pinned chart
version. - \[ \] Configure resource requests, limits and retention. - \[
\] Confirm Prometheus targets are healthy. - \[ \] Add application
metrics instrumentation. - \[ \] Create or import a Kubernetes overview
dashboard. - \[ \] Create an application dashboard. - \[ \] Add an
initial set of alerts.

Acceptance criteria: - \[ \] Grafana is reachable locally. - \[ \] Node
and pod metrics are available. - \[ \] API request count and latency
metrics are visible. - \[ \] Worker check counts and failures are
visible. - \[ \] At least one test alert can be triggered and observed.

### Goal 5.2 --- Add centralized logging

**Outcome:** application and cluster logs can be searched centrally.

Tasks: - \[ \] Install Loki. - \[ \] Install/configure Grafana Alloy to
collect container logs. - \[ \] Ensure API and worker logs are
structured JSON. - \[ \] Configure useful labels without
high-cardinality values. - \[ \] Add log queries or dashboards in
Grafana. - \[ \] Trigger a known application error and find its log.

Acceptance criteria: - \[ \] API and worker logs arrive in Loki. - \[ \]
Logs can be filtered by workload and severity. - \[ \] A known failure
can be located without using `kubectl logs` directly.

### Goal 5.3 --- Add distributed tracing

**Outcome:** a request or check operation can be followed across
components.

Tasks: - \[ \] Install Tempo. - \[ \] Add OpenTelemetry SDK
instrumentation to API and worker. - \[ \] Configure OTLP export. - \[
\] Instrument HTTP requests and database calls. - \[ \] Add trace
context propagation to asynchronous jobs when the queue is introduced. -
\[ \] Link traces to logs and metrics in Grafana where supported.

Acceptance criteria: - \[ \] Traces arrive in Tempo. - \[ \] A request
trace includes useful service spans. - \[ \] A slow operation can be
identified from its trace. - \[ \] Trace context can be followed across
the worker boundary after queue integration.

### Goal 5.4 --- Practise diagnosis

**Outcome:** observability is used to solve problems, not merely
installed.

Tasks: - \[ \] Introduce an API error and find the corresponding metric
and log. - \[ \] Add artificial latency and identify it in
metrics/traces. - \[ \] Stop PostgreSQL and observe application
symptoms. - \[ \] Restart a worker and observe recovery. - \[ \]
Document a short incident timeline and root cause.

Acceptance criteria: - \[ \] At least three induced faults are diagnosed
using observability data. - \[ \] The repository contains a short
troubleshooting/runbook entry for each.

------------------------------------------------------------------------

## Stage 6 --- Integrate Floci AWS-compatible services

### Goal 6.1 --- Create an S3 bucket with Terraform

**Outcome:** Terraform provisions an object-storage bucket in Floci.

Tasks: - \[ \] Verify the required S3 APIs are supported by the selected
Floci version. - \[ \] Add the bucket resource to a dedicated Terraform
module or configuration. - \[ \] Configure the AWS CLI or SDK to use the
Floci endpoint. - \[ \] Upload and retrieve a test object. - \[ \]
Document endpoint and addressing requirements.

Acceptance criteria: - \[ \] Terraform creates the bucket. - \[ \] A
test object can be uploaded and downloaded. - \[ \] No real AWS account
is required for the local exercise.

### Goal 6.2 --- Back up PostgreSQL to S3

**Outcome:** a Kubernetes CronJob uploads database backups to Floci S3.

Tasks: - \[ \] Write a backup script using `pg_dump`. - \[ \] Compress
the output. - \[ \] Upload it to the configured S3 bucket. - \[ \] Use a
dedicated credential/identity with only required permissions where
supported. - \[ \] Create a Kubernetes CronJob. - \[ \] Emit
success/failure logs and a metric. - \[ \] Define a basic
retention/cleanup policy.

Acceptance criteria: - \[ \] A manual backup job succeeds. - \[ \] An
object appears in the Floci bucket. - \[ \] A scheduled backup
succeeds. - \[ \] Backup failure is visible in logs and metrics.

### Goal 6.3 --- Prove database restore

**Outcome:** a backup can be restored into a clean database.

Tasks: - \[ \] Create a test dataset. - \[ \] Run a backup. - \[ \]
Delete or replace the test database. - \[ \] Restore the backup into a
fresh database. - \[ \] Verify expected records and check history. - \[
\] Document recovery steps and recovery limitations.

Acceptance criteria: - \[ \] A restore has been completed successfully
from an S3 object. - \[ \] The restored application can read its data. -
\[ \] The restore procedure is documented and repeatable.

### Goal 6.4 --- Introduce SQS for background jobs

**Outcome:** endpoint-check jobs can be dispatched through an
AWS-compatible queue.

Tasks: - \[ \] Verify SQS API support in the selected Floci version. -
\[ \] Create a queue with Terraform. - \[ \] Add a queue producer path
in the API or scheduler. - \[ \] Update the worker to consume
messages. - \[ \] Add retry and visibility-timeout handling. - \[ \]
Make message processing idempotent. - \[ \] Add metrics for queue depth,
processed jobs and failures.

Acceptance criteria: - \[ \] A job can be published to Floci SQS. - \[
\] A worker consumes and processes it. - \[ \] A failed job is handled
without silent message loss. - \[ \] Duplicate delivery does not corrupt
check history.

### Goal 6.5 --- Add secrets integration

**Outcome:** application credentials can be sourced from Floci Secrets
Manager without being committed to Git.

Tasks: - \[ \] Verify Secrets Manager support and required API
operations. - \[ \] Create a test secret outside Git. - \[ \] Install
External Secrets Operator if its provider integration is compatible. -
\[ \] Configure an appropriate SecretStore/ClusterSecretStore. - \[ \]
Synchronize a test secret into Kubernetes. - \[ \] Rotate the source
value and observe synchronization.

Acceptance criteria: - \[ \] No secret value is stored in the
repository. - \[ \] The Kubernetes Secret is created from the external
source. - \[ \] A secret update can be propagated and verified.

### Goal 6.6 --- Optional: ECR and SNS

**Outcome:** explore additional AWS-compatible integrations if
supported.

Tasks: - \[ \] Verify ECR API and registry support in Floci. - \[ \]
Push an application image and pull it from kind. - \[ \] Verify registry
authentication from the cluster. - \[ \] Optionally configure SNS
notifications for service state changes.

Acceptance criteria: - \[ \] The image workflow works end-to-end, or
unsupported limitations are documented. - \[ \] Optional notifications
are received through the selected local integration.

------------------------------------------------------------------------

## Stage 7 --- GitOps and delivery automation

### Goal 7.1 --- Install Argo CD

**Outcome:** Argo CD is running in the cluster and can access the Git
repository.

Tasks: - \[ \] Install Argo CD with a pinned version. - \[ \] Access its
UI or CLI locally. - \[ \] Configure repository credentials securely if
needed. - \[ \] Create an initial Application resource. - \[ \] Start
with manual synchronization.

Acceptance criteria: - \[ \] Argo CD is healthy. - \[ \] It can read the
repository. - \[ \] It displays the expected application resources and
sync status.

### Goal 7.2 --- Move application deployment to GitOps

**Outcome:** application desired state is stored in Git and reconciled
by Argo CD.

Tasks: - \[ \] Commit Helm chart and non-secret values. - \[ \] Define
an Argo CD Application. - \[ \] Configure the target namespace. - \[ \]
Sync the application. - \[ \] Change a harmless Helm value in Git. - \[
\] Observe Argo CD detect and apply the change. - \[ \] Enable automated
sync only after manual reconciliation is understood.

Acceptance criteria: - \[ \] A Git change updates the running
application. - \[ \] Drift can be detected and reconciled. - \[ \]
Terraform does not also manage the same application resources.

### Goal 7.3 --- Add CI checks

**Outcome:** changes are validated before they are merged.

Tasks: - \[ \] Add formatting and validation for Terraform. - \[ \] Run
Helm lint. - \[ \] Run application unit tests. - \[ \] Build container
images. - \[ \] Scan images with Trivy. - \[ \] Optionally publish
images to Floci ECR if supported, or use a local image-loading
workflow. - \[ \] Keep CI credentials scoped and out of source control.

Acceptance criteria: - \[ \] Pull requests run the selected checks. - \[
\] A failing test or lint step blocks a successful pipeline. - \[ \]
Image scanning results are available.

------------------------------------------------------------------------

## Stage 8 --- Scaling, policy and security

### Goal 8.1 --- Configure HPA

**Outcome:** the API or worker can scale based on resource metrics.

Tasks: - \[ \] Install and validate Metrics Server. - \[ \] Configure
resource requests on the target Deployment. - \[ \] Create an HPA. - \[
\] Generate controlled load. - \[ \] Observe replica changes and
scale-down behaviour.

Acceptance criteria: - \[ \] `kubectl top pods` returns metrics. - \[ \]
HPA reports current and desired metrics. - \[ \] Replica count increases
under load and later decreases.

### Goal 8.2 --- Configure KEDA with SQS

**Outcome:** worker replicas respond to queue depth.

Tasks: - \[ \] Verify Floci SQS compatibility with the selected KEDA
scaler. - \[ \] Install KEDA. - \[ \] Configure authentication without
committing credentials. - \[ \] Create a ScaledObject for the worker. -
\[ \] Publish a burst of jobs. - \[ \] Observe worker scale-out and
scale-in.

Acceptance criteria: - \[ \] Queue depth affects desired worker
replicas. - \[ \] Workers process the backlog. - \[ \] Scaling returns
to the configured idle level.

### Goal 8.3 --- Add network policies

**Outcome:** workloads communicate only along intended paths.

Tasks: - \[ \] Create a default-deny policy for the application
namespace. - \[ \] Permit ingress from Traefik to frontend/API as
required. - \[ \] Permit API-to-PostgreSQL traffic. - \[ \] Permit
worker-to-PostgreSQL and required outbound check traffic. - \[ \] Permit
DNS. - \[ \] Test a deliberately blocked connection.

Acceptance criteria: - \[ \] Allowed application flows still work. - \[
\] An unapproved connection is blocked. - \[ \] The policy behaviour is
documented.

### Goal 8.4 --- Add Kyverno policies and image scanning

**Outcome:** basic workload standards are enforced and images are
scanned.

Tasks: - \[ \] Install Kyverno. - \[ \] Start with audit-mode
policies. - \[ \] Require resource requests/limits and useful labels. -
\[ \] Add policies for non-root execution where compatible. - \[ \] Add
a policy against unintended mutable image tags. - \[ \] Add Trivy
scanning to CI. - \[ \] Move selected policies to enforce mode after
testing.

Acceptance criteria: - \[ \] A deliberately non-compliant test workload
is reported or rejected as expected. - \[ \] CI produces image scan
results. - \[ \] Policy exceptions are explicit and documented.

------------------------------------------------------------------------

## Stage 9 --- Recovery and operational exercises

### Goal 9.1 --- Test node and pod disruption

**Outcome:** you understand what kind does and does not recover
automatically.

Tasks: - \[ \] Delete an application pod. - \[ \] Scale a Deployment to
zero and back. - \[ \] Stop or remove a kind worker container in a
controlled test. - \[ \] Observe scheduling and recovery. - \[ \]
Document what happens to local persistent data.

Acceptance criteria: - \[ \] Expected pod recovery is observed. - \[ \]
Any workload that cannot recover is explained. - \[ \] The difference
between pod resilience and node/storage resilience is understood.

### Goal 9.2 --- Test dependency failures

**Outcome:** the application behaves predictably when dependencies fail.

Tasks: - \[ \] Stop PostgreSQL. - \[ \] Make Redis unavailable if
used. - \[ \] Make Floci unavailable. - \[ \] Introduce endpoint timeout
and DNS failures. - \[ \] Observe logs, metrics, traces and alerts. - \[
\] Restore dependencies and verify recovery.

Acceptance criteria: - \[ \] Failures are visible and diagnosable. - \[
\] The API does not enter an uncontrolled restart loop. - \[ \] Worker
retries and error handling behave as designed.

### Goal 9.3 --- Document runbooks

**Outcome:** common operational tasks have concise procedures.

Tasks: - \[ \] Document cluster creation and destruction. - \[ \]
Document application deployment and rollback. - \[ \] Document database
backup and restore. - \[ \] Document secret rotation. - \[ \] Document
common troubleshooting commands. - \[ \] Document how to inspect logs,
metrics and traces. - \[ \] Document known local-environment
limitations.

Acceptance criteria: - \[ \] A fresh checkout can follow the documented
procedures. - \[ \] Destructive actions include clear warnings. - \[ \]
Recovery instructions have been tested, not merely written.

------------------------------------------------------------------------

## Stage 10 --- Final reproducibility test

### Goal 10.1 --- Rebuild the environment from scratch

**Outcome:** the project can be recreated from its documented source of
truth.

Tasks: - \[ \] Review and commit all required configuration. - \[ \]
Identify which state is intentionally retained outside the cluster. - \[
\] Back up any data that should survive. - \[ \] Destroy the kind
cluster using the documented workflow. - \[ \] Recreate the cluster. -
\[ \] Bootstrap the platform. - \[ \] Let Argo CD reconcile
platform/application resources. - \[ \] Restore database data if the
exercise requires it. - \[ \] Run application smoke tests. - \[ \]
Update the README with any missing steps.

Acceptance criteria: - \[ \] The cluster is recreated from documented
commands. - \[ \] Platform components become healthy. - \[ \] The Status
Tracker is reachable. - \[ \] Observability data is being collected. -
\[ \] Floci integrations work. - \[ \] The restore procedure succeeds if
persistent data was part of the rebuild test. - \[ \] No undocumented
manual fix is required.

------------------------------------------------------------------------

## Optional extensions

Only attempt these after the core definition of done is satisfied:

-   [ ] Add a second application and a shared platform namespace.
-   [ ] Add a local container registry independent of Floci.
-   [ ] Explore Cilium Hubble for network flow visibility.
-   [ ] Add ExternalDNS if a suitable local DNS workflow is available.
-   [ ] Compare Traefik with another ingress implementation.
-   [ ] Explore Gateway API.
-   [ ] Add a service mesh only if you have a specific learning
    objective.
-   [ ] Add a separate VM-based Kubernetes lab to compare kind with real
    Linux nodes.
-   [ ] Add a second kind cluster and practise multi-cluster GitOps.

## Final project checklist

The project is finished when all of the following are true:

-   [ ] Terraform creates and destroys the kind cluster.
-   [ ] The cluster has one control plane and two workers.
-   [ ] Cilium networking and Traefik ingress work.
-   [ ] Persistent storage works for PostgreSQL.
-   [ ] The Status Tracker frontend, API and worker run in Kubernetes.
-   [ ] Prometheus and Grafana show cluster and application metrics.
-   [ ] Loki receives application logs.
-   [ ] Tempo receives application traces.
-   [ ] At least one AWS-compatible Floci integration is used by a
    workload.
-   [ ] PostgreSQL backups are stored in Floci S3 and can be restored.
-   [ ] Argo CD deploys the application from Git.
-   [ ] At least one autoscaling exercise works.
-   [ ] Network policy behaviour has been tested.
-   [ ] Failure and recovery exercises are documented.
-   [ ] A clean rebuild has been completed from the repository
    instructions.
