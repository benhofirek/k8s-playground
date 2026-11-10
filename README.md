# Kubernetes Playground

A hands-on platform engineering lab that runs entirely on your laptop. Build a
multi-node Kubernetes cluster with **kind** on **Podman**, provision it with
**Terraform**, practise against AWS-compatible APIs from **Floci**, and operate
a small application on it using the tooling you'd find on a real platform:
ingress, GitOps, observability, backups, autoscaling and policy.

## What will be build

A small **Status Tracker** application running on a local Kubernetes cluster.
It registers HTTP endpoints, checks their availability periodically, stores the
check history, exposes an API and shows a simple dashboard. A background worker
can consume jobs from a queue, which gives the platform a real reason to use
SQS-compatible infrastructure.

The application is deliberately simple. The platform around it is the point:
networking, storage, delivery, observability, AWS integration and day-two
operations. See [Technology choices](#technology-choices) for the full stack.

## Learning objectives

- Provision and destroy a multi-node Kubernetes cluster reproducibly with
  Terraform.
- Explain what Terraform, kind, Helm and Argo CD each own, and why one resource
  should have one owner.
- Configure CNI networking, ingress, TLS, service discovery and network
  policies.
- Deploy and operate a multi-component application with persistent data.
- Diagnose problems using metrics, logs and traces.
- Integrate workloads with AWS-compatible object storage, queues and secrets.
- Configure autoscaling and admission policies.
- Back up and restore application data, and test failure scenarios.

## Architecture at a glance

```text
┌─ Your machine ───────────────────────────────────────────────────────┐
│                                                                      │
│  Nix dev shell (direnv)                                              │
│  terraform · kind · kubectl · helm · aws · argocd · ...              │
│        │                                                             │
│        │ Terraform provisions                                        │
│        ▼                                                             │
│  ┌─ Podman ────────────────────────────────────────────────────────┐ │
│  │                                                                 │ │
│  │  ┌─ Floci ───────────────┐      ┌─ kind cluster ──────────────┐ │ │
│  │  │ AWS-compatible APIs   │      │ control plane · worker ×2   │ │ │
│  │  │ S3 · SQS · IAM · ...  │◄─────┤                             │ │ │
│  │  └───────────────────────┘      │ Cilium · Traefik · Argo CD  │ │ │
│  │                                 │ Prometheus · Grafana        │ │ │
│  │                                 │ Loki · Tempo                │ │ │
│  │                                 │                             │ │ │
│  │                                 │ Status Tracker              │ │ │
│  │                                 │ frontend · API · worker     │ │ │
│  │                                 │ PostgreSQL · Redis          │ │ │
│  │                                 └─────────────────────────────┘ │ │
│  └─────────────────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────────────┘
```

The kind nodes are Podman containers, not real EC2 instances. Floci emulates
selected AWS APIs; it does not turn kind into an EC2 environment. This project
therefore focuses on Kubernetes, platform engineering and AWS-service
integration, not on realistic EC2 networking or VM administration.

Components, boundaries, data flows and trade-offs are described in
[`architecture.md`](architecture.md).

## Technology choices

| Area | Choice | Why it's here |
| --- | --- | --- |
| Container runtime | [Podman](https://podman.io/) | Daemonless, rootless-capable engine that runs the kind nodes and Floci |
| Local Kubernetes | [kind](https://kind.sigs.k8s.io/) | Multi-node Kubernetes using containers |
| Infrastructure as code | [Terraform](https://developer.hashicorp.com/terraform) | Repeatable cluster and AWS-compatible resource provisioning |
| kind Terraform provider | [tehpsphinx/kind](https://registry.terraform.io/providers/tehpsphinx/kind/latest) | Manage the kind cluster lifecycle from Terraform |
| AWS emulation | [Floci](https://floci.io/) | Practise against local AWS-compatible APIs |
| CNI | [Cilium](https://cilium.io/) | Pod networking and NetworkPolicy practice |
| Ingress | [Traefik](https://traefik.io/traefik/) | Route local HTTP(S) traffic into the cluster |
| TLS | [cert-manager](https://cert-manager.io/) | Issuers and certificates with a local CA |
| Packaging | [Helm](https://helm.sh/) | Install and configure platform components and the app |
| GitOps | [Argo CD](https://argo-cd.readthedocs.io/) | Reconcile cluster state from Git |
| Metrics and dashboards | [Prometheus](https://prometheus.io/) + [Grafana](https://grafana.com/) | Collect and explore application and cluster metrics |
| Logs | [Loki](https://grafana.com/oss/loki/) + Alloy | Collect and query logs |
| Traces | [Tempo](https://grafana.com/oss/tempo/) + OpenTelemetry | Follow requests across services |
| Database | PostgreSQL | Persist registered services and check history |
| Queue / cache | Redis first; SQS (via Floci) later | Background work and queue-based scaling |
| Dev environment | [Nix](https://nixos.org/) flake + [direnv](https://direnv.net/) | A reproducible, pinned CLI toolchain |

Versions and provider details are pinned in the repository and reviewed
deliberately. Check Floci's current service coverage before designing a
workflow around a specific AWS API.

## Quick start

This walks through the first milestone ([Stage 1 of the plan](plan.md)): Floci
running, and a three-node kind cluster (one control plane, two workers) created
by Terraform on Podman.

### Prerequisites

| Requirement | Notes |
| --- | --- |
| Linux or macOS | Windows users can follow the Linux steps inside WSL2. |
| [Podman](https://podman.io/docs/installation) | Installed on the host. See the note below on why it isn't part of the flake. |
| [Nix](https://nixos.org/) with flakes enabled | Provides every other CLI tool at pinned versions. |
| [direnv](https://direnv.net/) | Activates the Nix dev shell when you enter the repository. [nix-direnv](https://github.com/nix-community/nix-direnv) is recommended for fast reloads. |
| Git | To clone the repository. |
| Resources | Roughly 4 CPU cores and 8 GB of RAM available to Podman for the early stages. Plan for more once the observability stack arrives. |

> **Why isn't Podman in the flake?** Rootless Podman needs system integration
> that a dev shell can't provide: user-namespace mappings, cgroup delegation
> and, on macOS, a Linux VM. Install it with your OS package manager (or the
> Podman module on NixOS) and let the flake supply everything else.

### 1. Prepare Podman

**Linux (rootless).** kind needs cgroup v2 with the CPU, cpuset, I/O, memory
and pids controllers delegated to your user.

```bash
# Expect "cgroupVersion: v2"
podman info | grep -i cgroupVersion

# Delegate controllers to user sessions
sudo mkdir -p /etc/systemd/system/user@.service.d
sudo tee /etc/systemd/system/user@.service.d/delegate.conf <<'EOF'
[Service]
Delegate=yes
EOF
sudo systemctl daemon-reload
```

Log out and back in for the change to take effect. See kind's
[rootless guide](https://kind.sigs.k8s.io/docs/user/rootless/) for details.

**macOS.** Podman runs in a Linux VM, and kind generally needs that VM to be
rootful.

```bash
brew install podman
podman machine init --cpus 4 --memory 8192 --rootful
podman machine start
```

Already have a rootless machine? Run `podman machine stop`, then
`podman machine set --rootful`, then `podman machine start`.

### 2. Enter the dev environment

```bash
git clone <repository-url>
cd k8s-playground
direnv allow
```

The first run builds the dev shell, which can take a few minutes. After that it
activates automatically whenever you `cd` into the repository. The repository's
`.envrc` does three things:

```bash
use flake                                 # load the Nix dev shell
export KIND_EXPERIMENTAL_PROVIDER=podman  # make kind use Podman, not Docker
dotenv_if_exists .env                     # load your local settings
```

The dev shell provides these tools (`flake.nix` is the authoritative list):

| Tool | Used for |
| --- | --- |
| `terraform` | Provisioning the kind cluster and Floci resources |
| `kind` | The local Kubernetes cluster (running on Podman) |
| `kubectl`, `helm` | Operating the cluster and installing software on it |
| `podman-compose` | Running Floci from `floci/compose.yaml` |
| `aws` | Talking to Floci's AWS-compatible APIs |
| `argocd`, `cilium` | Later stages: GitOps and networking |
| `jq`, `yq` | Working with JSON and YAML |

Check that everything resolves:

```bash
terraform version
kind version
kubectl version --client
helm version
```

Prefer not to use direnv? Run `nix develop` instead, then export the variables
from `.envrc` and `.env` yourself.

### 3. Configure local settings

```bash
cp .env.example .env
```

The defaults point Terraform and the AWS CLI at the local Floci endpoint using
dummy credentials. `.env` is git-ignored, and real secrets must never be
committed. direnv reloads the environment when the file changes.

### 4. Start Floci

```bash
podman-compose -f floci/compose.yaml up -d
```

Confirm it is answering. With the endpoint from `.env`, `aws s3 ls` should
return without an error. Floci's own documentation describes its health check
and the AWS services your pinned version supports.

### 5. Create the cluster

```bash
terraform -chdir=terraform init
terraform -chdir=terraform apply
```

### 6. Verify

```bash
kubectl --context kind-playground get nodes
```

You should see three nodes in the `Ready` state: `playground-control-plane`,
`playground-worker` and `playground-worker2`. `podman ps` lists them as
containers alongside Floci.

From here, continue with [`plan.md`](plan.md) at Goal 1.4: deploy a test
workload and prove the cluster works end to end.

### 7. Tear down

> [!WARNING]
> Destroying the cluster deletes everything inside it, including data on
> persistent volumes, because that storage is local to the kind nodes. Decide
> whether you need to keep database data or object-storage contents first.

```bash
terraform -chdir=terraform destroy
podman-compose -f floci/compose.yaml down
```

<details>
<summary><strong>Troubleshooting (Podman and kind)</strong></summary>

<br>

- **Cluster creation hangs or fails on rootless Linux.** Controller delegation
  usually hasn't taken effect. Run
  `cat /sys/fs/cgroup/user.slice/user-$(id -u).slice/user@$(id -u).service/cgroup.controllers`
  and look for `cpu cpuset io memory pids`. If any are missing, revisit step 1
  and log out and back in.
- **Ports 80 and 443 can't be bound.** Rootless processes can't bind ports
  below 1024 by default. Map ingress to high host ports such as 8080 and 8443,
  or deliberately lower `net.ipv4.ip_unprivileged_port_start`.
- **Podman asks which registry to use, or fails with a short-name error.** Use
  fully qualified image names, for example `docker.io/library/postgres`, in
  Compose files and Podman commands.
- **kind can't find an image you built locally.** Podman stores local images as
  `localhost/<name>:<tag>`. Load them through an archive, and reference the full
  name in Kubernetes manifests:

  ```bash
  podman save -o app.tar localhost/status-tracker-api:dev
  kind load image-archive app.tar --name playground
  ```

- **A pod can't reach Floci at `localhost`.** Inside a pod, `localhost` is the
  pod itself. Use an address that's reachable over the Podman network.
  `host.containers.internal` is a common starting point; test it from a
  temporary pod. See [`architecture.md`](architecture.md), section 4.3.
- **macOS: cluster creation fails.** Make sure the Podman machine is rootful
  (see step 1).

</details>

## Roadmap

The plan is organised into stages. Each stage has goals with concrete tasks and
acceptance criteria; track progress in [`plan.md`](plan.md).

| Stage | Focus |
| --- | --- |
| 0 | Prepare the workstation |
| 1 | Run Floci and provision kind with Terraform |
| 2 | Cluster networking and ingress (Cilium, Traefik, TLS) |
| 3 | Storage and application dependencies (PostgreSQL, Redis) |
| 4 | Build the Status Tracker (API, worker, frontend, Helm chart) |
| 5 | Observability: metrics, logs and traces |
| 6 | Floci integrations: S3 backups, SQS, Secrets Manager |
| 7 | GitOps and delivery automation (Argo CD, CI) |
| 8 | Scaling, policy and security (HPA, KEDA, NetworkPolicy, Kyverno) |
| 9 | Recovery and operational exercises |
| 10 | Final reproducibility test |

Don't install the whole platform in one go. Work through the stages in order,
and record the commands and decisions as you go.

## Repository layout proposal

The repository grows in stages. The target layout is:

```text
.
├── flake.nix              # Dev shell: pinned CLI tools
├── flake.lock
├── .envrc                 # direnv: loads the flake and .env
├── .env.example           # Template for local settings (copy to .env)
├── README.md
├── architecture.md        # Components, boundaries, data flows, decisions
├── plan.md                # Stage-by-stage roadmap with acceptance criteria
├── terraform/             # kind cluster and Floci resources
│   ├── versions.tf
│   ├── providers.tf
│   ├── main.tf
│   ├── variables.tf
│   ├── outputs.tf
│   └── modules/
│       ├── kind/
│       └── aws-local/
├── floci/
│   └── compose.yaml       # Floci, run with podman-compose
├── kubernetes/
│   ├── bootstrap/
│   ├── platform/
│   │   ├── networking/
│   │   ├── ingress/
│   │   ├── observability/
│   │   └── gitops/
│   └── apps/
│       └── status-tracker/
├── application/
│   ├── api/
│   ├── worker/
│   └── frontend/
├── .github/
│   └── workflows/         # CI
└── docs/                  # Runbooks and notes
```

The exact layout can evolve, but infrastructure provisioning, platform
configuration and application source should stay clearly separated.

## Design principles

1. **One owner per resource.** Terraform owns infrastructure and cluster
   lifecycle; Helm installs platform packages; Argo CD reconciles Git-managed
   Kubernetes workloads. Avoid managing the same object from multiple tools.
2. **Incremental complexity.** Get a working cluster and application before
   adding advanced components.
3. **Reproducibility.** Pin versions, commit configuration, document
   prerequisites and make setup and teardown repeatable.
4. **Observable by design.** The application exposes health endpoints, useful
   metrics, structured logs and distributed traces.
5. **Secrets stay out of Git.** Commit examples and templates, never
   credentials or local state files.
6. **Local limitations are explicit.** kind, Podman and Floci are excellent
   learning tools, but they don't reproduce every behaviour of real AWS, EC2,
   load balancers or cloud storage.

## Documentation

- [`architecture.md`](architecture.md): components, boundaries, data flows and
  design decisions.
- [`plan.md`](plan.md): the goal-by-goal implementation roadmap and acceptance
  criteria.
