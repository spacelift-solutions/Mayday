# Mayday demo walkthrough

Follow this order: stacks, PR and Wiz, tracked run, dependencies, Intent repair, Templates and policy, then Flows outage and capacity recovery.
Prepare the slow runs before presenting and leave [PR #15](https://github.com/spacelift-solutions/Mayday/pull/15) open.
Historical runs below are completed examples; distinguish them from the pending PR.

## 1. Show the stacks

Start in the Spacelift stacks view and filter on `mayday`.

| Stack | Purpose |
| --- | --- |
| [mayday-infrastructure](https://spacelift-solutions.app.spacelift.io/stack/mayday-infrastructure) | OpenTofu provisions the target, license bucket, and access controls. |
| [mayday-service](https://spacelift-solutions.app.spacelift.io/stack/mayday-service) | Ansible installs and repairs the application and changes worker capacity. |
| [mayday-diagnostics](https://spacelift-solutions.app.spacelift.io/stack/mayday-diagnostics) | Ansible gathers evidence when the application has a dependency failure. |
| [mayday-flows](https://spacelift-solutions.app.spacelift.io/stack/mayday-flows) | Manages the recovery Flow as code. |

> A stack brings together the code, inputs, credentials, policies, and run history for a deployment.
> Each tool keeps its own execution experience.

## 2. Show the infrastructure PR and Wiz

Open [PR #15](https://github.com/spacelift-solutions/Mayday/pull/15).
Show the change adding `Application = "mayday"` to the existing target's tags.
Open its [completed preview plan](https://spacelift-solutions.app.spacelift.io/stack/mayday-infrastructure/run/01M6F39M68Q4PQ57WVVPA2QAJG): zero additions, one in-place change, zero deletions.
Nothing from this PR has been applied.

> I opened the PR ahead of time so we can review the plan immediately.
> The developer can see what will change before merging.

The infrastructure stack has the `wiz` label, which attaches the account's existing Wiz plugin.
Use a completed run with a successful Wiz scan and open **Plugins Output** to show findings, severity, affected files, and the report link where available.
The installed plugin is `wiz-no-policies`: its findings are not a demonstration of a blocking security policy.
The separate plan policy in step 8 provides the explicit enforcement example.

Wiz preparation: the first fresh scan run, `01M6F4ZKWR68NP9G5SV00361QM`, failed because Wiz rejected the installed client credentials.
Refresh the installed plugin's credentials, start a fresh preview of PR #15's commit, and use that completed scan for presentation.
The earlier completed preview above predates Wiz attachment and has no Wiz findings to show.

## 3. Explain merge to tracked run

Keep PR #15 open for reuse.
Open [this completed tracked infrastructure run](https://spacelift-solutions.app.spacelift.io/stack/mayday-infrastructure/run/01M6EYYNRV2JQ1CHDTNR6G2SPW), which came from an earlier merged change.
Show the commit, plan, confirmation, and apply output.

> When a PR is merged, Spacelift creates a tracked run against the merged commit.
> This is a previously completed example so we do not need to wait for a new deployment.
> This infrastructure stack requires confirmation before applying.

## 4. Show the dependency to Ansible

Open the infrastructure stack's dependency graph and follow `mayday-service`.
Show `inventory_tofu` passing into `TOFUSIBLE_INVENTORY`.
Open [this completed dependency-triggered Ansible run](https://spacelift-solutions.app.spacelift.io/stack/mayday-service/run/01M679PH06WWZNCMFDMKZGN843) and its Tasks view.

> OpenTofu provisions the machine and passes inventory to Ansible.
> Ansible configures the service while keeping its native task output and troubleshooting view.

## 5. Break the license

Open [Mayday](https://mayday.spacelift.shovel-emporium.com) and sign in.
Start with Healthy, a valid license, two workers, and Light traffic.
Click **Invalidate license** and wait for the dependency incident.
Open **Open diagnostic run** and show the failed diagnostic task's evidence.
The diagnostic failure is expected when the license is invalid.

## 6. Ask what is wrong and repair through Intent

Ask Infra Assistant:

> What's happening with this service?

Let the assistant diagnose the evidence, then ask:

> Fix it using Intent.

Intelligence investigates; Intent performs the governed resource update.
The repair must update the existing `mayday-license` resource in the **Mayday license** project.
If a more specific prompt is needed:

> Refresh the existing mayday-license resource, then update that same resource with the valid checked-in demo-license.json content, including its trailing newline, and ETag 0ddb3ebffccfbffc98c50fbbcfd18fdc.
> Preserve its bucket/key, empty ACL, and AES256 encryption.
> Check live service health afterward.

Show the operation history, then return to Mayday for independently observed recovery.
The next step demonstrates a policy denial through the self-service form, so there is no need to manufacture an AI mistake here.

## 7. Request a license through a Template

Open **Templates**, select **Mayday sandbox license**, and show the instructions and license-key input.
For an authoring view, open its published version in **Templates Workbench** and show the form, stack definition, cloud integration, and attached plan policy.

> The platform team provides the implementation and controls.
> The consumer fills out a form without writing infrastructure code.

Create a deployment and enter `INVALID` as the license key.
Version 1.1.0 adds the deployment's creation timestamp to the stack name and object key, keeping the suffix stable when correcting inputs.
This creates a sandbox request under `license-requests/`; it does not replace the service's live license owned by Intent.
To avoid a planning wait, use the prepared **Mayday license demo** deployment and its recorded denied run in step 8.

Prepared records: [denied request](https://spacelift-solutions.app.spacelift.io/stack/mayday-license-mayday-license-demo/run/01M6F4Z95QEPG78R7ZAYTYEKH0), then [successful correction](https://spacelift-solutions.app.spacelift.io/stack/mayday-license-mayday-license-demo/run/01M6F552KDFE936AAZ4ZWCJPJH).

## 8. Show the denial, correct the input, and rerun

Open the deployment's stack and its failed run.
The **Mayday sandbox license requests** plan policy rejects the proposed object before apply:

```text
Mayday sandbox licenses require license_key DEMO-ONLY, service mayday, and enabled true
```

Show the policy evaluation and source condition checking the proposed JSON content.
You are changing the requested license value, not changing the policy.

Return to the same deployment, choose **Update deployment**, and change the license key to `DEMO-ONLY`.
Keep the deployment name unchanged and submit.
This updates the same stack and starts a new tracked run; the unchanged policy accepts the valid content.
After apply, open the deployment's **Outputs** to show the S3 URI and license JSON.

For a presentation without waiting, show the staged denied and corrected runs instead of submitting again.
For a live run, move on to explaining Flows while it executes, then return to show the result.
Updating deployment inputs is the supported workflow; see [the Templates Workbench guide](https://docs.spacelift.io/concepts/template/workbench).

## 9. Explain Flows

Open [Mayday recovery](https://useflows.eu/flow/01a18e9c-62ac-7af4-9176-1a00ddbd5f2a).
Keep **Validate and accept** and **Track and report** collapsed.
Show the webhook trigger, service-outage branch, and capacity branches.

> Flows connects an operational event to the appropriate action.
> The monitor sends a new incident; Flows selects the repair, triggers Spacelift, and reports the result back.
> Routine health checks stay in the monitor and do not invoke Flows.

## 10. Cause an outage and follow automatic recovery

Once the license incident is recovered, click **Cause outage**.
Use **Open recovery Flow** to show the new invocation, then **Open recovery run** when it appears.
Show that the Ansible stack was triggered by Flows automatically.
Return to Mayday and watch it recover.

> A successful automation run is useful evidence, but the monitor waits for the service to complete requests successfully before declaring recovery.

## 11. Increase and decrease traffic

Select **Heavy** and show the rising queue, latency, and rejected requests.
Follow the Flow as it sets `WORKERS=8` and triggers Ansible.
Keep the same Heavy load until recovery shows that the additional capacity handles it.

Select **Light** and explain the cooldown before Flows returns the service to two workers.
This adjusts application concurrency on the existing instance; it is not EC2 autoscaling.
Finish with Healthy, a valid license, Light traffic, and two workers.

## Presenter preparation and source

- Keep PR #15 open; prepare a fresh small PR and finished preview if it is merged or closed.
- Complete the Wiz credential repair and stage a successful scan before promising findings.
- Keep the denied and corrected Template runs ready so plan time does not interrupt the story.
- Resolve one application incident before starting the next; do not inject faults during an application deployment.
- Mayday's sandbox module is in `infra/license-request`; the Template, plan policy, and Wiz label are in `spacelift-solutions/demo`, under `admin/modules/mayday`.
- The existing Intent policy remains in `license.rego.tftpl`; the Template's separate Deploy policy is `license_request.rego.tftpl`.
