# Deploying and presenting Mayday

## Prerequisites

- Spacelift solutions account access, an authenticated `spacectl` solutions profile, and the existing AWS cloud integration.
- Shared EKS cluster with Pod Identity, EBS CSI, a default storage class, and the EKS worker pool in the target's VPC.
- Flux bootstrapped from `spacelift-solutions/flux`, with its dedicated SOPS KMS key and controller decryption permissions applied.
- An AWS author session with encryption/decryption access to that key, plus SOPS, Flux CLI 2.9.6, and kubeconform 0.8.0.
- A Flows project on `useflows.eu`, a valid `FLOWS_TOKEN`, and `flowctl`.
- Infra Assistant enabled in Ask and Build mode.

## Deployment sequence

1. Merge the reviewed Mayday code and publish both images.
   CI compiles the Linux binary with Go caching and copies the artifact into the runtime image.
   Keep the GHCR packages private.
   Reconcile the worker pull Secret from `flux/clusters/spacelift-solutions/platform/spacelift-workers/ghcr.sops.yaml`, then apply `demo/worker-pools/kubernetes/aws` through `worker-pool-on-eks`.
   Verify the EKS worker pool has registered workers before running Mayday stacks.
   The monitor's pull Secret is `kubernetes/ghcr.sops.yaml` and reconciles with its workload.
   Record the runtime image digest from the publishing workflow.
2. Apply the companion `demo/admin` change.
   It creates four stacks, the SSH context, scoped Flows API key, and Intent policy.
   The idempotent GraphQL bootstrap creates the Intent project and attaches its policy before its existing AWS integration.
3. Run `mayday-infrastructure`.
   It creates the private target, bucket, and monitor Pod Identity association.
   The pinned Tofusible host module produces `inventory_tofu`, which triggers `mayday-service` through `TOFUSIBLE_INVENTORY`.
   Mayday passes only host metadata and an SSH key path to the module, so its output is explicitly nonsensitive and needs no private-worker sensitive-output upload setting.
4. In the **Mayday license** Intent project, create `license/demo-license.json` in the infrastructure stack's `license_bucket` output.
   Use resource type `aws_s3_object`, `server_side_encryption = "AES256"`, a private ACL, and the exact content from `demo-license.json`.
   Intent exclusively owns this object.
5. Create or edit `kubernetes/control.sops.yaml` with `AWS_PROFILE=demo sops edit kubernetes/control.sops.yaml`.
   It is a Kubernetes Secret named `mayday-control` in namespace `mayday`, with `stringData.DEMO_PASSWORD` and `stringData.FLOWS_WEBHOOK_TOKEN`.
   Use a password of at least 12 characters and a webhook token of at least 16 characters.
   The repository's `.sops.yaml` selects Flux's dedicated KMS key and encrypts the values.
   Commit only the encrypted file; use SOPS for subsequent edits.
6. Set `TF_VAR_webhook_token` on `mayday-flows` as a write-only environment variable, using the same webhook token from the encrypted Secret.
   The admin module leaves this variable under operator control.
7. Publish the custom app with `flowctl app create`, `flowctl version create -e flows/app/main.ts`, and `flowctl version publish` in the Mayday Flows project.
   Set `TF_VAR_project_id` and `TF_VAR_app_version_id` on `mayday-flows`, then run/confirm it.
8. Edit `kubernetes/configmap.yaml` using the infrastructure stack's outputs.
   Set `TARGET_URL` from `target_url`, `TARGET_INSTANCE_ID` from `instance_id`, and verify the region, bucket, license key, and stop document name match their outputs.
   Set `FLOWS_WEBHOOK_URL` to the **Observe Mayday** block's HTTP endpoint.
   In `kubernetes/monitor.yaml`, replace the `unconfigured` image with the published `ghcr.io/spacelift-solutions/mayday@sha256:...` digest.
9. Run `make manifests-check PYTHON=.venv/bin/python` and merge the complete manifests into Mayday.
   In the Flux repository, change `clusters/spacelift-solutions/mayday/workload.yaml` to `suspend: false` and merge after review.
   That resource points to `./kubernetes`; Flux discovers the plain manifests and decrypts the Secret using its existing Pod Identity.
   Verify `flux get sources git -n flux-system` and `flux get kustomizations -n flux-system` show Mayday Ready.
10. Open the dashboard and sign in with the demo password:

    ```sh
    kubectl -n mayday port-forward service/mayday-monitor 8080:8080
    ```

    Confirm health, two workers, Light traffic, webhook delivery, and a successful diagnostic run before presenting.

There is no monitor Spacelift stack or Kubernetes provider in Mayday's OpenTofu configuration.
Infrastructure and Flows stacks require normal apply confirmation; service and diagnostic runs autodeploy for recovery.
The infrastructure-to-Ansible dependency remains visible in Spacelift.
Update the ConfigMap after replacing the target or changing its outputs.
After a ConfigMap or Secret change reconciles, restart the monitor deployment to load its new environment:

```sh
kubectl -n mayday rollout restart deployment/mayday-monitor
kubectl -n mayday rollout status deployment/mayday-monitor
```

## Authentication

### Private image pulls

Both images use a dedicated GitHub classic PAT with only `read:packages`, authorized for the organization and with read access to both packages.
The source credential is stored in the vault at `/work/ghcr/mayday-pull-token`.
Each namespace has its own `mayday-ghcr` Secret of type `kubernetes.io/dockerconfigjson`: `mayday` for the monitor and `spacelift-worker-controller-system` for worker run pods.
The monitor Deployment and the OpenTofu-managed WorkerPool reference these Secrets through `imagePullSecrets`.
Setting a stack environment variable would be too late to authenticate the runner image pull.

To rotate, replace the vault token and update `.dockerconfigjson` in both SOPS files using the repository encryption rules.
Keep plaintext out of Git and command arguments.
Validate both repositories and merge both encrypted updates.
Wait for Flux to reconcile them, verify a fresh pull in each namespace, then revoke the old token.
The controller, worker pool and pool registration credentials remain owned by `demo/worker-pools/kubernetes/aws`; Flux owns only the worker registry Secret.
The monitor remains suspended until its runtime configuration and image digest are ready.

### Dashboard access

The demo password is exchanged for an HttpOnly, SameSite=Strict session cookie valid for eight hours.
Sign out revokes that session; a monitor restart invalidates all sessions.
The browser does not store the password or an API token.
The application rejects cross-origin browser mutations and limits failed login attempts.
The default connection is a local HTTP port-forward.
If the dashboard is exposed through HTTPS, set `SESSION_COOKIE_SECURE=true` on the monitor so the browser sends its cookie only over HTTPS.
The outbound Flows webhook still uses its separate shared token.

## Bootstrap and ownership

The Flux child starts suspended because the initial review precedes infrastructure provisioning, image publication, and webhook creation.
Before resuming it, fill the empty runtime values, commit the encrypted Secret, and pin the published image digest.
No setup script or generated overlay is required.

The removed OpenTofu monitor implementation was never deployed during this work, so there is no live state migration.
If it was applied separately, suspend reconciliation and remove its Kubernetes resources from OpenTofu state without destroying them before enabling Flux.
Never let both tools manage the same monitor objects.

## Walkthrough

### Show the deployment handoff

Open the infrastructure stack, its outputs, and the dependency graph.
Follow `inventory_tofu` into `TOFUSIBLE_INVENTORY` on the Ansible stack.
Show the Go service and the monitor's deployment on the shared cluster.

### Service outage

Click **Cause outage**.
The monitor asks SSM to execute only the fixed `MaydayStopService` document.
The button does not open an incident directly.
After sustained failed probes, the incident event takes the restart branch in Flows.
Follow the Ansible run, then the independent recovery verification.

### License failure and AI policy

Click **Remove license file**.
The Kubernetes service deletes the designated S3 object.
The target keeps its process and diagnostic endpoint alive, but readiness and work requests fail.

Open the resulting `mayday-diagnostics` run.
Ask Infra Assistant:

> What's happening with this service?

The evidence includes systemd state, service logs, the configured bucket/key, and the result of the live license check.
Let the assistant identify the cause; the prompt does not reveal it.
Then say:

> Fix it.

The approved repair is an Intent operation on the license object.
Confirm the policy evaluation and operation history, then watch the independent monitor verify actual work requests.
Validate refresh/recreation of the externally deleted object during rehearsal; do not create a second unmanaged resource.

For the explicit policy demonstration, ask Intent to delete the restored license.
Show the server-side policy denial, then confirm the service stays healthy.
Use an operator identity that can manage Intent resources but cannot edit policies; the administrator's broader permissions should not be presented as a restricted operator role.

### Capacity

Select **Medium** for steady traffic at eight requests/second, then **Heavy** for 22.
Watch the queue, mean latency, and errors rise.
Flows sets the service stack's persistent `WORKERS` input to eight and triggers Ansible.
Recovery requires healthy work at the same elevated rate.

Select **Light** to return to two requests/second.
After the cooldown and sustained low load, Flows returns to two workers.
Reset also returns load and capacity to the baseline; it is available only after the current incident is resolved.

## Failure handling and repeated demos

- The monitor uses a persistent volume for incident history and webhook outbox.
  Failed deliveries are retried; pending telemetry snapshots are coalesced.
- Webhooks require a shared token.
  Incident event IDs are deduplicated by the custom app.
  The repair block records an attempt before calling Spacelift so an ambiguous API failure does not automatically launch another repair.
- Investigate a timed-out incident before clicking **Retry failed recovery**.
  Check for a still-running Spacelift run first.
  Retrying creates a new event ID for the same incident; it is an explicit new attempt.
- Changing worker capacity can briefly restart the service.
  That stays within the active capacity incident instead of launching a competing outage repair.
- A missing or invalid license goes to diagnosis.
  Increasing worker capacity cannot repair it.
  S3 access-denied and timeout errors remain distinct evidence.
- The dashboard is accessed by port-forward; the monitor only makes outbound requests to Flows.
  No SaaS callback into the private cluster is required.

## Ownership and cleanup

OpenTofu owns the bucket and compute.
Intent owns the object.
Flux owns the monitor namespace, workload, storage, configuration, and encrypted Secret.
Ansible owns the target's binary and configuration.
The monitor has deletion permission for fault injection but no object-write permission.
Flows owns subsequent `WORKERS` changes, so the admin resource ignores changes to that value.

Disable the recovery Flow before teardown.
Suspend the Flux child before removing its source or workload manifests.
Remove the Intent object through an authorized administrative cleanup process, then remove the monitor workload and destroy target infrastructure.
The namespace and PVC opt out of Flux pruning to preserve incident history; explicitly delete them after deciding whether to retain that data.
The bucket is not force-destroyed.
The Intent GraphQL bootstrap intentionally has no destroy action; its project and history need an explicit retention/cleanup decision.
Delete the app/Flow before revoking its API key.
Do not remove the shared EKS cluster or workers.
