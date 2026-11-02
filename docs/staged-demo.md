# Mayday demo walkthrough

## 1. Stack the Basics

Open the [Stacks view](https://spacelift-solutions.app.spacelift.io/stacks).
Show the tool indicators on a few stacks, including OpenTofu and Ansible, before opening a specific application.

- Multi-IaC support in one platform: OpenTofu, Ansible, and other tools.
- Stack = tracked code, inputs, credentials, policies, and run history.
- Consistent workflow; native execution and output for each tool.
- Provision and configure infrastructure with the tools suited to each job.

## 2. The Developer Workflow

Navigate to the [PRs tab in mayday-infrastructure](https://spacelift-solutions.app.spacelift.io/stack/mayday-infrastructure/prs).

- Developer workflow: local iteration → pull request.
- Relevant code changes appear in the tracking stack's PR view.
- Preview runs expose proposed changes before merge.

Click into PR #15 from the stack's PR view and open its [completed preview run](https://spacelift-solutions.app.spacelift.io/stack/mayday-infrastructure/run/01M6F39M68Q4PQ57WVVPA2QAJG).
Show initialization, planning, and the familiar OpenTofu output.
Point out the proposed `Application = "mayday"` tag: zero additions, one in-place update, zero deletions.
The [GitHub PR](https://github.com/spacelift-solutions/Mayday/pull/15) is available if you want to show its diff and checks.

- Run phases: initialization, planning, and policy evaluation.
- Plan policies inspect changes and can block violations.
- Git checks bring results back to the developer's PR.
- Review code, plan, and policy results → merge → tracked run.

## 3. Tracked Runs

Open the [tracked runs view](https://spacelift-solutions.app.spacelift.io/stack/mayday-infrastructure), then [this completed tracked run](https://spacelift-solutions.app.spacelift.io/stack/mayday-infrastructure/run/01M6EYYNRV2JQ1CHDTNR6G2SPW).
Use this earlier deployment to show the commit link, resource changes, run phases, policy results, confirmation, and apply output.

- Deployment record: commit, resource changes, policies, approvals, and apply output.
- Governance continues after merge.
- Policies can block, require approval, or issue advisory warnings.
- Manual confirmation here; automatic apply available for other workflows.

On the prepared Wiz scan run, open **Plugins Output**.
Show the findings, severity, affected file and line, and the report link where available.

- Wiz security findings alongside the infrastructure plan.
- Findings in Spacelift; report links into Wiz.
- Plugins connect scanners, cost tools, approvals, and internal services.

Open the stack's [Contexts tab](https://spacelift-solutions.app.spacelift.io/stack/mayday-infrastructure/contexts) and show the attached Wiz context without revealing secret values.
Explain that contexts share configuration, environment variables, mounted files, and hooks across stacks.
The `wiz` label attaches this plugin's context to the infrastructure stack.

- Hooks run commands before or after execution phases.
- Plugins use hooks to execute integrations and collect results.
- Contexts share configuration and hooks across stacks.

- [Spaceforge](https://github.com/spacelift-io/plugins/blob/main/spaceforge/README.md): Python framework for custom plugins.
- Implement hooks, define inputs, generate the manifest.
- Integrate internal APIs, custom checks, and automation.
- Package once; reuse across stacks.

Reference: [Spacelift hooks](https://docs.spacelift.io/concepts/stack/stack-settings#customizing-workflow).

## 4. Stack Dependencies

Open [mayday-infrastructure's dependencies](https://spacelift-solutions.app.spacelift.io/stack/mayday-infrastructure/dependencies) and follow the connection to `mayday-service`.

- OpenTofu/Terraform provision; Ansible configures.
- Calling OpenTofu from Ansible buries the separate plan-and-review workflow.
- Calling Ansible from a provisioner obscures its own execution workflow.
- Dependencies pass data while preserving each tool's phases, output, and policies.

Show the infrastructure stack's `inventory_tofu` output and its dependency reference to `TOFUSIBLE_INVENTORY` on the Ansible stack.
Open [Tofusible](https://github.com/spacelift-solutions/tofusible) to show the inventory handoff.

- Tofusible packages machine information as a native OpenTofu output.
- Dependency passes the output into Ansible's dynamic inventory.
- Playbooks consume the resulting hosts normally.
- Visibility into both infrastructure changes and Ansible tasks/hosts.

Open [this completed dependency-triggered Ansible run](https://spacelift-solutions.app.spacelift.io/stack/mayday-service/run/01M679PH06WWZNCMFDMKZGN843) and its Tasks view.
Show the inventory host and an individual task's output before moving into self-service.

## 5. Self Service

- Platform team owns implementation and controls.
- Consumer requests infrastructure through a form.

1. Open [Templates](https://spacelift-solutions.app.spacelift.io/templates) and select **Mayday sandbox license**.
2. Create a new license deployment with license key `INVALID`.
3. Open the deployment's stack and failed run; show the plan policy denial.
   - Policy checks the requested license before apply.
   - Required values: `license_key: DEMO-ONLY`, `service: mayday`, `enabled: true`.
4. Open [Templates Workbench](https://spacelift-solutions.app.spacelift.io/templates-workbench) and select **Mayday sandbox license**.
   - Show the form, stack definition, cloud integration, and attached policy.
   - Publish new template versions; update existing deployments to roll out changes.
5. Return to the failed deployment, click **three dots → Update deployment**, and change the license key to `DEMO-ONLY`.
6. Show the new run passing the same policy, then open **Outputs** for the S3 URI and license JSON.

## 6. Intent

Open [Mayday](https://mayday.spacelift.shovel-emporium.com).
Set the scene before breaking anything:

- Demo scenario: a shop selling Spacelift swag, with incoming customer requests.
- Mayday simulates the workload and gives us monitoring, traffic controls, and fault injection.
- Deploy and Flows handle recovery; the dashboard shows the service's actual health.

Copy the login password to the macOS clipboard using cluster access, then paste it into Mayday:

```sh
kubectl --context spacelift-solutions -n mayday get secret mayday-control \
  -o jsonpath='{.data.DEMO_PASSWORD}' | base64 --decode | pbcopy
```

- Intent manages resources from a natural-language request.
- Policies govern the proposed changes.
- Operation history records the repair; live service health confirms the result.

1. Start with **Healthy**, a valid license, **two workers**, and **Light** traffic.
2. Click **Invalidate license** and wait for the dependency incident.
3. Click **Open diagnostic run** and show the failed task's evidence.
   - Diagnostic failure is expected with an invalid license.
4. Ask the Assistant: `What's happening with this service?`
5. After diagnosis, ask: `Fix it using Intent.`
   - Intelligence investigates; Intent updates the existing `mayday-license` resource in the **Mayday license** project.
6. Show the operation history, then return to Mayday and watch recovery.

## 7. Flows

Open [Mayday recovery](https://useflows.eu/flow/01a18e9c-62ac-7af4-9176-1a00ddbd5f2a).
Keep **Validate and accept** and **Track and report** collapsed.
Show the webhook trigger, service-outage branch, and capacity branches.

- Event-driven orchestration: incident → repair → Spacelift run → result callback.
- Apps provide reusable blocks for services and integrations; connect blocks to build a Flow.
- Open-source apps, such as the [GitHub app](https://github.com/spacelift-flows-apps/flows-app-github), can be inspected and extended.
- [Node.js runtime](https://docs.spacelift.io/flows/apps) for app code; JavaScript expressions configure dynamic block values.
- Copy/paste Flow definitions between the canvas and YAML; reuse individual blocks or an entire Flow.
- Manage Flows, app installations, and secrets with the [OpenTofu provider](https://docs.spacelift.io/flows/automating-flows/flows-tf-provider); review changes in Git.
- Branches select the action for each incident type.
- Routine health checks stay local; no idle Flow invocations.

## 8. Cause an outage and follow automatic recovery

Once the license incident is recovered, click **Cause outage**.
Use **Open recovery Flow** to show the new invocation, then **Open recovery run** when it appears.
Show that the Ansible stack was triggered by Flows automatically.
Return to Mayday and watch it recover.

- Run success provides execution evidence.
- Successful service requests independently confirm recovery.

## 9. Increase and decrease traffic

Select **Heavy** and show the rising queue, latency, and rejected requests.
Follow the Flow as it sets `WORKERS=8` and triggers Ansible.
Keep the same Heavy load until recovery shows that the additional capacity handles it.

Select **Light** and explain the cooldown before Flows returns the service to two workers.
This adjusts application concurrency on the existing instance; it is not EC2 autoscaling.
Finish with Healthy, a valid license, Light traffic, and two workers.

## Presenter preparation and source

- Keep PR #15 open; prepare a fresh small PR and finished preview if it is merged or closed.
- Wiz scan `01M6F4ZKWR68NP9G5SV00361QM` failed authentication. Refresh the installed credentials and stage a successful scan before presenting Plugins Output; the older linked runs have no Wiz results.
- The installed `wiz-no-policies` plugin supplies scan output without a blocking policy. Use the Template plan policy in step 5 to demonstrate an explicit denial.
- Resolve one application incident before starting the next; do not inject faults during an application deployment.
- Mayday's sandbox module is in `infra/license-request`; the Template, plan policy, and Wiz label are in `spacelift-solutions/demo`, under `admin/modules/mayday`.
- The existing Intent policy remains in `license.rego.tftpl`; the Template's separate Deploy policy is `license_request.rego.tftpl`.
- Use Template version 1.2.0 for demos: unique timestamped stacks expire after one hour, deleting their sandbox objects under `license-requests/`.
- The shared bucket and Intent's live `license/demo-license.json` remain intact; policy reserves the name `demo-license`.

If the short Intent repair prompt needs more detail:

```text
Refresh the existing mayday-license resource, then update that same resource with the valid checked-in demo-license.json content, including its trailing newline, and ETag 0ddb3ebffccfbffc98c50fbbcfd18fdc.
Preserve its bucket/key, empty ACL, and AES256 encryption.
Check live service health afterward.
```
