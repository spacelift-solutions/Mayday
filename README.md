# Mayday

An incident and recovery story for Spacelift.
Use Mission Control to stop a service, remove its license, or increase traffic.
An independent monitor detects the effects and sends events to Flows.
Ansible handles known repairs; Intelligence investigates missing dependencies and Intent restores them under policy.

## Run locally

Requires Go 1.27.3+, Python 3, Node.js 22+, and OpenTofu 1.8+ for configuration checks.
Install the shared repository hooks with `make hooks`.

```sh
make dev
```

Open <http://127.0.0.1:8080> and sign in with `mayday-local-demo` (or your `DEMO_PASSWORD` environment variable).
The browser uses an HttpOnly session cookie; signing out revokes the session.
The target listens on port 8081.
Both processes bind to loopback.

Local mode uses `.local/demo-license.json` instead of S3.
The **Remove license file** button deletes it and the monitor detects the resulting dependency failure.
Restore it with:

```sh
cp demo-license.json .local/demo-license.json
```

Traffic buttons generate real requests.
Local mode deliberately does not fake Flows, Intent, or SSM: cloud repair controls require deployment.
`make smoke` starts isolated real processes and exercises all detection/recovery paths, with the test harness performing the repairs.

## Components

| Path | Responsibility |
| --- | --- |
| `cmd/mayday`, `internal/target` | Go target with license checks, bounded concurrency, queue limits, and synthetic I/O |
| `internal/control`, `internal/web` | Kubernetes monitor, fault controls, request generator, durable incident outbox, and dashboard |
| `infra/target` | EC2, private S3 bucket, SSM stop document, and scoped AWS identities |
| `ansible` | Tofusible-compatible inventory, deployment/repair, and diagnostic evidence |
| `kubernetes` | Plain Kubernetes manifests and the SOPS-encrypted monitor Secret |
| `flows/app` | Custom Flows app: observed events, bounded Spacelift repairs, and independent verification |
| `flows/deploy`, `flows/recovery.yaml` | App installation and the recovery workflow |

Spacelift spaces, stacks, dependencies, credentials, and the Intent policy are managed in `spacelift-solutions/demo`, under `admin/modules/mayday`.
Flux reconciles the monitor and its namespace from this repository.
The `spacelift-solutions/flux` repository owns the remote source and reconciliation object.
The four Spacelift stacks manage infrastructure, service configuration, diagnostics, and Flows.

## Scenarios

| Control | Observed effect | Recovery |
| --- | --- | --- |
| Cause outage | EC2 systemd service stops through a fixed SSM document | Flows triggers the Ansible service stack |
| Remove license file | Readiness and work requests fail while diagnostics stay available | Flows collects evidence; ask Intelligence “What's happening with this service?”, then “Fix it” using Intent |
| Heavy traffic (22 req/s) | A two-worker target builds a queue and rejects excess requests | Flows persists `WORKERS=8` on the service stack and runs Ansible |
| Light traffic (2 req/s) | Sustained low load with excess capacity | After cooldown, Flows persists `WORKERS=2` and runs Ansible |
| Reset demo | Load returns to two requests/second | Flows reapplies baseline service configuration; restore a missing license through Intent first |

Each synthetic request waits 200 ms, modeling an I/O-bound job.
This is application concurrency adjustment, not EC2 autoscaling.
Light sends 2 requests/second, Medium sends 8, and Heavy sends 22.
Heavy overloads two workers while remaining within eight-worker capacity; Light brings capacity back down after cooldown.

Fault button history is not a monitoring signal.
Three failed observations open an incident; three successful observations with completed work close it.
Ansible success alone cannot close an incident.
A five-minute monitor timeout leaves the incident visibly failed, with an explicit retry action.

## Deploy and present

Follow [the deployment and demo runbook](docs/runbook.md).
Slow infrastructure is provisioned before the live incident walkthrough.
Images are published privately to GHCR by the main-branch workflow.
The monitor and EKS workers use namespace-local `mayday-ghcr` pull Secrets, encrypted with SOPS in their respective GitOps sources.
See the runbook for deployment order and credential rotation.

## Development checks

```sh
python3 -m venv .venv
.venv/bin/pip install -r requirements-dev.txt
make check PYTHON=.venv/bin/python       # Go race tests/vet, TypeScript tests/typecheck, formatting
make smoke       # approximately two minutes of real HTTP failures and recovery
make build
make manifests-check PYTHON=.venv/bin/python
make build-linux
docker build --platform linux/amd64 -t mayday:local .
```

Initialize and validate the `infra/target` and `flows/deploy` OpenTofu roots with `tofu init -backend=false` and `tofu validate`.
Install Flux CLI 2.9.6 and kubeconform 0.8.0 for manifest checks.
Flux discovers the manifests directly; there are no `kustomization.yaml` files.
CI compiles the Linux artifact with Go caching, then copies it into the runtime image.
The runner image remains a separate build.
The custom app can be bundled locally with:

```sh
cd flows/app
npm ci
flowctl version bundle -e main.ts -o /tmp/mayday-flows-app.tar.gz
```

No local check proves cloud IAM, cluster connectivity, Flows delivery, or Intent recreation of an externally deleted object.
Rehearse those with the runbook in the live account before presenting.

## Releases

The [release workflow](.github/workflows/release.yml) uses [`NerdsWhoFish/quill`](https://github.com/NerdsWhoFish/quill) to validate release mechanics on every pull request.

To publish, run the workflow manually from `main`, choose **Release** or **Release Candidate**, and select a `patch`, `minor`, or `major` increment.
The workflow calculates versions from final `vMAJOR.MINOR.PATCH` tags.
Release candidates use `vMAJOR.MINOR.PATCH-rc.N`; they increment `N` without consuming the corresponding final version.

The starter publishes a source-only GitHub Release.
Projects that build release artifacts can add their publisher configuration and change `publish: none` in the workflow to a supported Quill publisher such as `goreleaser`.
Quill restricts releases to `main`, validates before tagging, and removes a failed release's tag so the version can be retried.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for the contribution workflow.

## Architecture decisions

Significant technical decisions are recorded as Architecture Decision Records (ADRs) in [adr/](adr/README.md).
Add an ADR when a decision has meaningful alternatives, is costly to reverse, and will need its reasoning preserved for future contributors.

## License

This project is licensed under the [MIT License](LICENSE).
