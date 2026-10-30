# Deploying and presenting Mayday

## Prerequisites

- Spacelift solutions account access, an authenticated `spacectl` solutions profile, and the existing AWS cloud integration.
- Shared EKS cluster with Pod Identity, EBS CSI, the `gp2` storage class, and the existing EC2 worker pool.
- Flux bootstrapped from `spacelift-solutions/flux`, with its dedicated SOPS KMS key and controller decryption permissions applied.
- An AWS author session with encryption/decryption access to that key, plus SOPS, Flux CLI 2.9.6, and kubeconform 0.8.0.
- A Flows project on `useflows.eu`, a valid `FLOWS_TOKEN`, and `flowctl`.
- Infra Assistant enabled in Ask and Build mode.

## Deployment sequence

1. Merge the reviewed Mayday code and let GitHub Actions publish the service binary and monitor image.
   CI compiles the Linux binary with Go caching and copies the artifact into the runtime image.
   Keep the GHCR packages private.
   Confirm the `build-<commit SHA>` prerelease has `mayday-linux-amd64` and its `.sha256` checksum.
   Ansible selects that exact commit through `TF_VAR_spacelift_commit_sha` and retries briefly if publication is still in progress.
   The existing EC2 pool uses Spacelift’s standard AWS Ansible runner; no custom runner or worker registry credentials are required.
   The monitor's pull Secret is `kubernetes/ghcr.sops.yaml` and reconciles with its workload.
   Record the runtime image digest from the publishing workflow.
2. Apply the companion `demo/admin` change.
   It creates four stacks, the SSH context, scoped Flows project access, and Intent policy.
   The public key is mounted at `/mnt/workspace/mayday.pub` for OpenTofu; the private key is mounted separately for Ansible.
   The idempotent GraphQL bootstrap creates the Intent project and attaches its policy before its existing AWS integration.
3. Run `mayday-infrastructure`.
   It creates the private target, bucket, and monitor Pod Identity association.
   The pinned Tofusible host module produces `inventory_tofu`, which triggers `mayday-service` through `TOFUSIBLE_INVENTORY`.
   Mayday passes only host metadata and an SSH key path to the module, so its output is explicitly nonsensitive and needs no private-worker sensitive-output upload setting.
4. In the **Mayday license** Intent project, create `license/demo-license.json` in the infrastructure stack's `license_bucket` output.
   Use resource type `aws_s3_object`, `server_side_encryption = "AES256"`, `acl = ""`, and the exact content from `demo-license.json`.
   The bucket uses `BucketOwnerEnforced` ownership and blocks public access; leave the ACL empty because S3 disables object ACLs for this bucket.
   Intent exclusively owns this object.
5. Create or edit `kubernetes/control.sops.yaml` with `AWS_PROFILE=demo sops edit kubernetes/control.sops.yaml`.
   It is a Kubernetes Secret named `mayday-control` in namespace `mayday`, with `stringData.DEMO_PASSWORD` and `stringData.FLOWS_WEBHOOK_TOKEN`.
   Use a password of at least 12 characters and a webhook token of at least 16 characters.
   The repository's `.sops.yaml` selects Flux's dedicated KMS key and encrypts the values.
   Commit only the encrypted file; use SOPS for subsequent edits.
6. Set `TF_VAR_webhook_token` on `mayday-flows` as a write-only environment variable, using the same webhook token from the encrypted Secret.
   The admin module leaves this variable under operator control.
7. Run/confirm `mayday-flows` after the admin apply.
   Admin supplies `TF_VAR_project_id` and the project-scoped `FLOWS_TOKEN` deployment credential.
   This stack manages `MAYDAY_WEBHOOK_TOKEN` as a project secret and imports the native definition from `flows/recovery.yaml`.
   No custom app, published app version, or Spacelift recovery API key is required.
   For the existing live deployment, follow the migration instructions below before confirming the first apply.
8. Edit `kubernetes/configmap.yaml` using the infrastructure stack's outputs.
   Set `TARGET_URL` from `target_url`, `TARGET_INSTANCE_ID` from `instance_id`, and verify the region, bucket, license key, and stop document name match their outputs.
   Set `FLOWS_WEBHOOK_URL` to the **Monitor webhook** block's HTTP endpoint.
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

### Native Flow access

The Mayday Flows project uses the `Mayday recovery` role in its Operations space: `SPACE_READ`, `RUN_TRIGGER`, and `STACK_ADD_CONFIG`.
The native blocks use project access to trigger `mayday-service` / `mayday-diagnostics` and update the service's `WORKERS` input.
They do not exchange a Spacelift API key.
`FLOWS_TOKEN` remains necessary for the OpenTofu deployment stack; it is a separate credential used to manage the Flow.

The HTTP endpoint authenticates the monitor with `secrets.MAYDAY_WEBHOOK_TOKEN` and redacts the authorization header in events.
The Flow stack uses OpenTofu 1.13.1 because the provider's secret value is write-only (requires 1.11+). The token variable is also ephemeral, keeping it out of saved plans.
Provider 0.7.0 has no secret revision argument, so changing the variable alone does not schedule an update. To rotate, update `MAYDAY_WEBHOOK_TOKEN` in the Flows project, the write-only `TF_VAR_webhook_token`, and the encrypted Kubernetes Secret to the same value; then restart the monitor after Flux reconciles the Secret. Do not replace the secret resource merely to rotate it.

### Private image pulls

The monitor uses a dedicated GitHub classic PAT with only `read:packages`, authorized for the organization and with read access to the Mayday package.
The source credential is stored in the vault at `/work/ghcr/mayday-pull-token`.
The Deployment references its namespace-local `mayday-ghcr` Secret through `imagePullSecrets`.
The existing EC2 workers use the public standard Ansible runner and need no GHCR credentials.

To rotate, replace the vault token and update `.dockerconfigjson` in `kubernetes/ghcr.sops.yaml` using SOPS.
Keep plaintext out of Git and command arguments.
Validate and merge the encrypted update, wait for Flux reconciliation, then verify a fresh monitor pull before revoking the old token.
The monitor remains suspended until its runtime configuration and image digest are ready.

### Private target access

The existing EC2 workers and shared EKS cluster are in separate VPCs with overlapping CIDRs.
Ansible uses [SSH through SSM](https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-getting-started-enable-ssh-connections.html) to reach the target by instance ID; the standard AWS runner already includes the Session Manager plugin and boto3.
The small `ansible/ssm_proxy.py` adapter starts an SSM session with boto3 and hands the SSH byte stream to the installed plugin.
The service and diagnostics stacks receive AWS credentials through the existing cloud integration, and SSH still authenticates with the Mayday key.
No inbound SSH rule is required.
The monitor reaches port 8081 within the target's EKS VPC.
Verify the target is online in SSM and the Ansible deployment succeeds before presenting.

### Dashboard access

The demo password is exchanged for an HttpOnly, SameSite=Strict session cookie valid for eight hours.
Sign out revokes that session; a monitor restart invalidates all sessions.
The browser does not store the password or an API token.
The application rejects cross-origin browser mutations and limits failed login attempts.
The default connection is a local HTTP port-forward.
If the dashboard is exposed through HTTPS, set `SESSION_COOKIE_SECURE=true` on the monitor so the browser sends its cookie only over HTTPS.
The outbound Flows webhook still uses its separate shared token.

## Bootstrap and ownership

### Adopt the native Flow reviewed live

The Flow is already enabled in project `01a18e62-7f44-7168-9a2f-a420c435d1b0` with ID `01a18e9c-62ac-7af4-9176-1a00ddbd5f2a`.
The export preserves the reviewed block names, layout, notes, links, and webhook path.
Keep the resource address `flows_flow.recovery`: it already owns this Flow in the `mayday-flows` stack state.

1. Merge this Mayday change before applying the companion admin change, so any new Flow run uses the native definition.
2. Apply the companion admin change. Its idempotent access bootstrap recognizes the existing scoped project binding; it also upgrades only the Flow stack to OpenTofu 1.13.1 and removes the retired Spacelift recovery API key and its environment variables.
3. Plan `mayday-flows`. `flows/deploy/imports.tf` adopts the existing project secret without rewriting its value. The old app installation, six config fields, and confirmation are retired.
4. Confirm only after the plan preserves the Flow ID, native blocks, and webhook path. No target infrastructure or service changes belong in this migration.
5. Remove the obsolete operator-set `TF_VAR_app_version_id` variable from `mayday-flows`. The published custom app can be archived after its installation is removed.
6. Merge the prepared Flux unsuspend change after the monitor image and runtime settings are ready, then rehearse all five incident paths.

The one-time import targets the secret created during review. Omit `imports.tf` when bootstrapping a different project, so OpenTofu creates a new project secret.
The administrative project-access bootstrap is temporary until the provider releases `spacelift_role_attachment.flows_project_id`; it does not delete bindings on destroy.
Revoke project access explicitly before deleting its role, and use a native imported role attachment when support ships.

### Keep canvas and code in sync

After reviewing live canvas edits, export the complete definition:

```sh
flowctl flow export --id 01a18e9c-62ac-7af4-9176-1a00ddbd5f2a --file flows/recovery.yaml
make flows-check PYTHON=.venv/bin/python
```

Review the exported diff before applying the stack. An apply can overwrite unexported canvas changes.
Do not rename blocks casually: the provider matches them by name. Keep the HTTP path stable so the monitor does not lose its endpoint.

### Monitor bootstrap

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
  A native concurrency group and Memory blocks deduplicate event IDs for 24 hours.
  The Flow records delivery acceptance and a recovery attempt before calling Spacelift, so retrying delivery after an ambiguous failure does not automatically launch another repair.
  Intake failures release the lock and return HTTP 503; valid events and duplicates return 202.
- Native Trigger Run completion is checked for `FINISHED`, then the Flow polls independent monitor evidence every five seconds with a ten-minute deadline.
  A failed run or orchestration error records a failed outcome. A successful diagnostic run records `evidence_ready` for the human Intelligence/Intent handoff.
  The dashboard's five-minute incident timer remains independent; inspect ongoing runs before explicitly retrying.
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
Delete the Flow and project secret, and revoke its project role binding before deleting the recovery role or revoking the Flows deployment credential.
Do not remove the shared EKS cluster or workers.
