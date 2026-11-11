# Implementation Plan

## Guidelines
- Work through the goals sequentially.
- Keep commits small, focused, and free of secrets.
- Record troubleshooting notes and decisions directly in the repository.

---

## Stage 0 — Prepare the workstation

### Goal 0.1 — Install and verify prerequisites
- **Outcome:** The workstation runs Podman, and pinned CLI tooling is accessible via Nix.
- **Tasks:**
  - Install Podman and create/start a rootful Podman machine on macOS.
  - Allocate sufficient CPU, memory, and disk to the Podman environment.
  - Enable cgroup v2 controller delegation for rootless Podman on Linux.
  - Install Nix with flakes enabled, direnv, and Git.
  - Install Floci following its official installation guide.
  - Enter the development shell using `direnv allow` to load pinned tools.
  - Commit `flake.lock` to version control.
- **Acceptance criteria:**
  - `podman version`, `terraform version`, `kind version`, `kubectl version --client`, and `helm version` all succeed inside the shell.
  - Podman resources are verified to support a multi-node cluster and platform components.

### Goal 0.2 — Create the repository skeleton
- **Outcome:** The repository structure is clearly separated for infrastructure, configuration, and code.
- **Tasks:**
  - Create `README.md`, `architecture.md`, and `plan.md`.
  - Create `terraform/`, `floci/`, `kubernetes/`, and `application/` directories.
  - Add a `.gitignore` file.
  - Exclude Terraform state, `.terraform/`, kubeconfig files, local env files, and build artifacts.
  - Add `.env.example` templates for configuration parameters.
  - Enforce a strict policy against committing real secrets.
- **Acceptance criteria:**
  - Repository layout is intuitive and clean.
  - `git status` displays no untracked local state or credentials.

---

## Stage 1 — Run Floci and provision kind

### Goal 1.1 — Start Floci locally
- **Outcome:** Floci is running locally with a predictable, reachable endpoint.
- **Tasks:**
  - Create a Docker/Podman Compose configuration for Floci.
  - Use fully qualified image names with pinned versions.
  - Document the supported Floci service coverage and API operations.
  - Configure a static local endpoint and verify its health.
- **Acceptance criteria:**
  - Floci starts reliably via `podman-compose`.
  - The health check endpoint responds successfully.

### Goal 1.2 — Configure Terraform providers
- **Outcome:** Terraform is initialized with kind and AWS-compatible providers.
- **Tasks:**
  - Create `terraform/versions.tf` with pinned provider versions.
  - Declare the `tehpsphinx/kind` provider and the AWS provider.
  - Configure AWS provider endpoints and path-style S3 access for Floci.
  - Supply provider credentials via environment variables or local untracked files.
  - Generate and commit the provider lock file.
- **Acceptance criteria:**
  - `terraform init` succeeds cleanly from a fresh checkout.
  - No credentials exist in committed Terraform files.

### Goal 1.3 — Create the kind cluster with Terraform
- **Outcome:** Terraform provisions a 3-node Kubernetes cluster.
- **Tasks:**
  - Define a cluster named `playground` with one control-plane and two worker nodes.
  - Configure host port mappings for ingress (e.g., ports 8080 and 8443).
  - Define variables for cluster name, node image, and scale.
  - Set `KIND_EXPERIMENTAL_PROVIDER=podman` in `.envrc`.
  - Run `terraform apply` to create the cluster infrastructure.
- **Acceptance criteria:**
  - `kind get clusters` lists `playground`.
  - `kubectl get nodes` reports one control plane and two workers in `Ready` state.
  - The cluster can be cleanly destroyed and recreated via Terraform.

### Goal 1.4 — Prove basic Kubernetes operation
- **Outcome:** A test workload runs successfully and validates cluster functionality.
- **Tasks:**
  - Deploy a simple web workload using a Deployment and Service.
  - Verify pod-to-pod communication and cluster DNS resolution.
  - Inspect pods, logs, and events.
  - Delete a test pod and verify automated replacement.
- **Acceptance criteria:**
  - Test pod reaches `Running` status and answers HTTP queries.
  - Deleted pods are automatically recreated.

---

## Stage 2 — Install cluster networking and ingress

### Goal 2.1 — Install Cilium
- **Outcome:** Cilium provides advanced pod networking and network policies.
- **Tasks:**
  - Prepare a Cilium installation via its pinned Helm chart.
  - Keep kube-proxy enabled for initial simplicity.
  - Apply the chart and verify operator and agent pod health.
  - Run Cilium connectivity verification tests across nodes.
- **Acceptance criteria:**
  - All Cilium components report healthy status.
  - Inter-node pod communication and ClusterIP services function correctly.

### Goal 2.2 — Install Traefik and configure local ingress
- **Outcome:** External HTTP traffic reaches cluster services through Traefik.
- **Tasks:**
  - Install Traefik using its official Helm chart.
  - Configure NodePort settings matching the kind host port mappings.
  - Deploy a test application with an Ingress route.
- **Acceptance criteria:**
  - Workstation requests successfully route through Traefik to the test application.
  - Multi-path routing rules operate correctly.

### Goal 2.3 — Add local TLS
- **Outcome:** Ingress endpoints securely serve HTTPS via a local certificate authority.
- **Tasks:**
  - Install cert-manager via Helm.
  - Create a self-signed local CA Issuer resource.
  - Issue a TLS certificate for the local hostname.
  - Configure Traefik to use the generated TLS secret.
- **Acceptance criteria:**
  - The Certificate resource reaches a `Ready` state.
  - HTTPS requests to the test application succeed.

---

## Stage 3 — Add storage and application dependencies

### Goal 3.1 — Install a local StorageClass
- **Outcome:** Workloads can dynamically provision persistent storage.
- **Tasks:**
  - Select and install a local storage provisioner compatible with kind.
  - Inspect the resulting StorageClass definitions.
  - Create a PersistentVolumeClaim (PVC) and mount it to a test pod.
  - Write test data, delete the pod, and verify data retention.
- **Acceptance criteria:**
  - PVC binds successfully.
  - Replacement pods retain access to previously written data.

### Goal 3.2 — Deploy PostgreSQL
- **Outcome:** PostgreSQL runs with persistent storage and is accessible internally.
- **Tasks:**
  - Select a maintained PostgreSQL Helm chart and pin its version.
  - Configure persistent storage using a PVC.
  - Supply database credentials securely using environment variables or secrets.
  - Set resource requests, limits, and health probes.
  - Test connectivity using a temporary client pod.
- **Acceptance criteria:**
  - PostgreSQL pod becomes `Ready`.
  - Data survives pod restarts.

### Goal 3.3 — Define Redis usage
- **Outcome:** The application architecture clarifies whether caching or queuing uses Redis.
- **Tasks:**
  - Determine if Redis is required for the initial application release.
  - Deploy Redis with explicit persistence settings if selected.
  - Document architectural boundaries between Redis and SQS.
- **Acceptance criteria:**
  - Redis role is explicitly defined in project documentation.
  - Redis can be removed without destabilizing core platform components.

---

## Stage 4 — Build the Status Tracker application

### Goal 4.1 — Implement the API
- **Outcome:** A containerized API manages service registrations and statuses.
- **Tasks:**
  - Build the API service in Go or Python/FastAPI.
  - Implement `/healthz`, `/readyz`, service CRUD endpoints, and check-history routes.
  - Integrate database migrations and input validation.
  - Output structured JSON logs and write unit/integration tests.
  - Build and tag the container image locally.
- **Acceptance criteria:**
  - Automated API tests pass cleanly.
  - Container runs successfully and serves valid responses.

### Goal 4.2 — Implement the checker worker
- **Outcome:** A background worker monitors registered endpoints and logs results.
- **Tasks:**
  - Implement the check execution loop with configurable timeouts and intervals.
  - Record status codes, latencies, timestamps, and errors.
  - Implement graceful shutdown and structured logging.
  - Write tests covering success, timeout, and DNS failure paths.
- **Acceptance criteria:**
  - Worker successfully pings endpoints and writes results to PostgreSQL.
  - Unreachable targets are recorded without crashing the worker.

### Goal 4.3 — Implement the frontend
- **Outcome:** A minimal dashboard displays services and their live status.
- **Tasks:**
  - Build a simple web UI showing service lists, statuses, and check history.
  - Implement a form for registering new services.
  - Parameterize the API base URL via environment variables.
  - Containerize the frontend application.
- **Acceptance criteria:**
  - The dashboard correctly renders data retrieved from the API.
  - Users can register services directly through the UI.

### Goal 4.4 — Package the application with Helm
- **Outcome:** A single Helm chart deploys the full application stack into Kubernetes.
- **Tasks:**
  - Create a unified Helm chart containing API, worker, and frontend deployments.
  - Add Services, Ingress routes, ConfigMaps, and Secret references.
  - Configure resource limits, probes, and database connection settings.
  - Build development images with `podman build` and load them into kind.
  - Deploy the chart into the cluster.
- **Acceptance criteria:**
  - `helm lint` passes without warnings.
  - Helm installs the release successfully and all pods become `Ready`.

---

## Stage 5 — Add observability

### Goal 5.1 — Install Prometheus and Grafana
- **Outcome:** Cluster and application metrics are collected and visualized in Grafana.
- **Tasks:**
  - Install `kube-prometheus-stack` via its Helm chart.
  - Configure resource requests, limits, and retention periods.
  - Instrument the application to expose metrics.
  - Import dashboard templates for Kubernetes overview and application metrics.
- **Acceptance criteria:**
  - Grafana is accessible and displays node, pod, and application metrics.

### Goal 5.2 — Add centralized logging
- **Outcome:** Application and cluster logs are routed to a central log store.
- **Tasks:**
  - Install Loki via Helm.
  - Install and configure Grafana Alloy to collect container logs.
  - Create log queries and dashboards in Grafana.
- **Acceptance criteria:**
  - Structured logs from the API and worker arrive in Loki.
  - Logs can be queried and filtered by workload and severity level.

### Goal 5.3 — Add distributed tracing
- **Outcome:** Requests can be traced across application components.
- **Tasks:**
  - Install Tempo via Helm.
  - Add OpenTelemetry SDK instrumentation to the API and worker.
  - Configure OTLP trace export for HTTP requests and database queries.
- **Acceptance criteria:**
  - Traces arrive in Tempo and link properly with logs and metrics in Grafana.

### Goal 5.4 — Practice diagnosis
- **Outcome:** Observability stack is validated through controlled fault injection.
- **Tasks:**
  - Inject an API error and locate its corresponding metric and log entry.
  - Introduce artificial latency and identify it in traces.
  - Stop PostgreSQL and observe application symptoms.
  - Document troubleshooting steps and runbooks for each exercise.
- **Acceptance criteria:**
  - Induced faults are successfully diagnosed using monitoring data.

---

## Stage 6 — Integrate Floci AWS-compatible services

### Goal 6.1 — Create an S3 bucket with Terraform
- **Outcome:** Terraform provisions an S3-compatible bucket in Floci.
- **Tasks:**
  - Verify required S3 API support in the active Floci release.
  - Add an S3 bucket resource to the Terraform configuration.
  - Configure the AWS CLI to point to the local Floci endpoint.
  - Upload and retrieve a test object.
- **Acceptance criteria:**
  - Terraform creates the bucket successfully.
  - Test objects can be uploaded and downloaded via the AWS CLI.

### Goal 6.2 — Back up PostgreSQL to S3
- **Outcome:** A Kubernetes CronJob automatically backs up the database to Floci S3.
- **Tasks:**
  - Write a backup script utilizing `pg_dump` and compression.
  - Create a Kubernetes CronJob to execute the backup script.
  - Configure credentials securely inside the cluster.
  - Emit success and failure metrics and logs.
- **Acceptance criteria:**
  - Manual and scheduled database backup jobs succeed and upload objects to Floci S3.

### Goal 6.3 — Prove database restore
- **Outcome:** Database backups can be restored cleanly from S3.
- **Tasks:**
  - Delete or reset the test database.
  - Download a backup archive from the Floci S3 bucket.
  - Restore the database dump into a fresh instance.
  - Verify data integrity and check history.
- **Acceptance criteria:**
  - The restore procedure completes successfully and the application resumes normal reading.

### Goal 6.4 — Introduce SQS for background jobs
- **Outcome:** Endpoint-check jobs are processed asynchronously via an AWS SQS queue.
- **Tasks:**
  - Create an SQS queue resource using Terraform.
  - Implement a queue producer path in the API or scheduler.
  - Update the worker to consume, process, and acknowledge queue messages.
  - Implement idempotent message processing and retry logic.
- **Acceptance criteria:**
  - Jobs published to Floci SQS are successfully consumed and processed by workers.

### Goal 6.5 — Add secrets integration
- **Outcome:** Application secrets are loaded securely from Floci Secrets Manager.
- **Tasks:**
  - Create a test secret in Floci Secrets Manager.
  - Install the External Secrets Operator in the cluster.
  - Configure a SecretStore to connect to the Floci backend.
  - Synchronize the secret into a Kubernetes Secret resource.
- **Acceptance criteria:**
  - Kubernetes Secrets are successfully populated from the external provider without storage in Git.

### Goal 6.6 — Optional: ECR and SNS
- **Outcome:** Additional AWS integrations (ECR, SNS) are evaluated or documented.
- **Tasks:**
  - Test pushing an image to Floci ECR and pulling it into kind.
  - Configure SNS notifications for state changes if supported.
- **Acceptance criteria:**
  - Image workflow completes successfully or limitations are documented.

---

## Stage 7 — GitOps and delivery automation

### Goal 7.1 — Install Argo CD
- **Outcome:** Argo CD manages cluster configurations and connects to the Git repository.
- **Tasks:**
  - Install Argo CD using its official Helm chart.
  - Access the Argo CD UI/CLI locally.
  - Configure repository credentials securely.
  - Create an initial Application resource with manual synchronization.
- **Acceptance criteria:**
  - Argo CD UI is reachable and reports healthy synchronization status.

### Goal 7.2 — Move application deployment to GitOps
- **Outcome:** Application state is defined in Git and reconciled automatically by Argo CD.
- **Tasks:**
  - Commit the application Helm chart and non-secret values to the repository.
  - Define an Argo CD Application targeting the application namespace.
  - Perform an initial manual sync, then test updating a value in Git.
  - Enable automated synchronization once verified.
- **Acceptance criteria:**
  - Git changes automatically propagate to the running cluster via Argo CD.

### Goal 7.3 — Add CI checks
- **Outcome:** Pull requests undergo automated code quality and security checks.
- **Tasks:**
  - Add linting and formatting validation for Terraform and Helm.
  - Configure automated execution of unit tests and container builds.
  - Integrate Trivy for image vulnerability scanning.
- **Acceptance criteria:**
  - CI pipeline successfully blocks merges on failing tests or critical scan findings.

---

## Stage 8 — Scaling, policy, and security

### Goal 8.1 — Configure HPA
- **Outcome:** Workloads scale automatically based on resource utilization metrics.
- **Tasks:**
  - Install and validate the Kubernetes Metrics Server.
  - Configure resource requests and limits on target deployments.
  - Create a HorizontalPodAutoscaler (HPA) resource.
  - Generate load and observe replica scaling behavior.
- **Acceptance criteria:**
  - HPA successfully adjusts replica counts in response to load fluctuations.

### Goal 8.2 — Configure KEDA with SQS
- **Outcome:** Worker replicas scale dynamically based on SQS queue depth.
- **Tasks:**
  - Install KEDA via Helm.
  - Configure authentication parameters for Floci SQS.
  - Create a KEDA ScaledObject for the worker deployment.
  - Publish message bursts and observe worker scaling behavior.
- **Acceptance criteria:**
  - Worker replicas scale out based on queue depth and scale back down when idle.

### Goal 8.3 — Add network policies
- **Outcome:** Namespace traffic is restricted to explicitly allowed communication paths.
- **Tasks:**
  - Create a default-deny network policy for the application namespace.
  - Write explicit rules allowing ingress from Traefik to the frontend/API.
  - Permit traffic from API and workers to PostgreSQL and DNS.
  - Test and verify that unauthorized connections are blocked.
- **Acceptance criteria:**
  - Legitimate application traffic succeeds while unauthorized connections fail.

### Goal 8.4 — Add Kyverno policies and image scanning
- **Outcome:** Cluster governance rules and security baselines are enforced.
- **Tasks:**
  - Install Kyverno using its Helm chart.
  - Implement audit-mode policies enforcing resource limits and non-root execution.
  - Transition tested policies from audit mode to enforce mode.
- **Acceptance criteria:**
  - Non-compliant test workloads are successfully audited or rejected.

---

## Stage 9 — Recovery and operational exercises

### Goal 9.1 — Test node and pod disruption
- **Outcome:** Cluster resilience and recovery limits are understood.
- **Tasks:**
  - Delete individual application pods and scale deployments to zero and back.
  - Simulate node failure by stopping a kind worker container.
  - Observe rescheduling behavior and verify persistence boundaries.
- **Acceptance criteria:**
  - Workloads recover correctly according to expected Kubernetes guarantees.

### Goal 9.2 — Test dependency failures
- **Outcome:** Application behavior under external dependency outages is validated.
- **Tasks:**
  - Terminate PostgreSQL and Floci availability during runtime.
  - Inject network timeouts and DNS failures.
  - Monitor application logs, metrics, and alerts during the outage.
  - Restore dependencies and verify system recovery.
- **Acceptance criteria:**
  - Failures are observable and systems recover gracefully without entering crash loops.

### Goal 9.3 — Document runbooks
- **Outcome:** Concise operational guides exist for common administrative tasks.
- **Tasks:**
  - Document procedures for cluster setup and teardown.
  - Document application deployment, rollback, and backup/restore workflows.
  - Document secret rotation steps and troubleshooting commands.
- **Acceptance criteria:**
  - A fresh user can successfully execute operational tasks using the documentation.

---

## Stage 10 — Final reproducibility test

### Goal 10.1 — Rebuild the environment from scratch
- **Outcome:** The entire platform can be cleanly recreated from source documentation.
- **Tasks:**
  - Review and commit all configuration files to version control.
  - Destroy the kind cluster using the documented teardown workflow.
  - Recreate the cluster, bootstrap platform tooling, and trigger Argo CD sync.
  - Restore database backups and run application smoke tests.
- **Acceptance criteria:**
  - The entire environment is successfully rebuilt without requiring undocumented manual fixes.

---

## Optional extensions
- Add a second application and a shared platform namespace.
- Add a local container registry independent of Floci.
- Explore Cilium Hubble for network flow visualization.
- Add ExternalDNS for local DNS record automation.
- Explore Gateway API alternatives to Ingress.
- Configure a multi-cluster GitOps layout with a second kind cluster.

## Final project checklist
- [ ] Terraform provisions and destroys the kind cluster cleanly.
- [ ] Cluster consists of one control plane and two worker nodes.
- [ ] Cilium networking and Traefik ingress are fully operational.
- [ ] PostgreSQL utilizes persistent storage.
- [ ] Status Tracker frontend, API, and worker run successfully in Kubernetes.
- [ ] Prometheus, Grafana, Loki, and Tempo provide full observability.
- [ ] Floci AWS-compatible integrations (S3, SQS, Secrets Manager) function correctly.
- [ ] PostgreSQL backups are archived to S3 and can be restored.
- [ ] Argo CD manages continuous deployment from Git.
- [ ] Autoscaling (HPA and KEDA) responds dynamically to load and queue depth.
- [ ] Network and security policies restrict traffic correctly.
- [ ] Clean environment rebuild completes successfully from documentation.
