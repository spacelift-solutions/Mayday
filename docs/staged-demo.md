# Mayday demo walkthrough

## 1. Stack the Basics

Open the [Stacks view](https://spacelift-solutions.app.spacelift.io/stacks).
Show the tool indicators on a few stacks, including OpenTofu and Ansible, before opening a specific application.

> Spacelift supports multiple infrastructure tools in the same platform.
> Each stack tracks a piece of code and brings together the inputs, credentials, policies, and run history needed to execute it.
> This stack uses OpenTofu; this one uses Ansible.
> The workflow is familiar across both, but the execution and output stay native to the tool.
> You can provision infrastructure with one tool and configure it with another without building a separate operating model for each.

## 2. The Developer Workflow

Navigate to the [PRs tab in mayday-infrastructure](https://spacelift-solutions.app.spacelift.io/stack/mayday-infrastructure/prs).

> Think about a developer's normal workflow: they iterate on code locally, then open a pull request.
> When that change affects code tracked by a stack, the PR appears here in that stack's PR view.
> Spacelift runs a preview so the developer and reviewer can inspect the proposed changes before merging.

Click into PR #15 from the stack's PR view and open its [completed preview run](https://spacelift-solutions.app.spacelift.io/stack/mayday-infrastructure/run/01M6F39M68Q4PQ57WVVPA2QAJG).
Show initialization, planning, and the familiar OpenTofu output.
Point out the proposed `Application = "mayday"` tag: zero additions, one in-place update, zero deletions.
The [GitHub PR](https://github.com/spacelift-solutions/Mayday/pull/15) is available if you want to show its diff and checks.

> The run takes the code through its execution phases, and policies evaluate the applicable inputs along the way.
> For example, a plan policy can inspect the proposed infrastructure and deny a change that violates your rules.
> Results can appear as Git checks so the developer can review them from the PR as well.
> Once the developer is satisfied with the plan, policy results, and code review, they merge the PR.
> That merge creates a tracked run against the merged commit.

## 3. Tracked Runs

Open the [tracked runs view](https://spacelift-solutions.app.spacelift.io/stack/mayday-infrastructure), then [this completed tracked run](https://spacelift-solutions.app.spacelift.io/stack/mayday-infrastructure/run/01M6EYYNRV2JQ1CHDTNR6G2SPW).
Use this earlier deployment to show the commit link, resource changes, run phases, policy results, confirmation, and apply output.

> The tracked run gives you the full record of the deployment: which commit ran, what it planned, which controls evaluated it, and what happened during apply.
> Merging does not bypass those controls.
> A policy can stop a run before apply or require approval when a change needs a human decision.
> You can also make a policy advisory, so it warns without blocking.
> This infrastructure stack requires confirmation before apply; other stacks can apply automatically when their checks pass.

On the prepared Wiz scan run, open **Plugins Output**.
Show the findings, severity, affected file and line, and the report link where available.

> Wiz adds security findings to the run, alongside the infrastructure plan.
> The reviewer can see the evidence in Spacelift and follow the report into Wiz for more detail.
> Plugins can connect other scanners, cost tools, approval systems, or internal services to this same workflow.

Open the stack's [Contexts tab](https://spacelift-solutions.app.spacelift.io/stack/mayday-infrastructure/contexts) and show the attached Wiz context without revealing secret values.
Explain that contexts share configuration, environment variables, mounted files, and hooks across stacks.
The `wiz` label attaches this plugin's context to the infrastructure stack.

> Hooks are commands that run before or after an execution phase, such as initialization, planning, or apply.
> A plugin uses those hooks to run its integration at the appropriate point and collect its results.
> The context delivers the configuration and hooks to the stacks that need them, so we do not repeat that setup on every stack.

Open the [Spaceforge Python framework](https://github.com/spacelift-io/plugins/blob/main/spaceforge/README.md).

> Customers can build their own plugins in Python using Spaceforge.
> They implement the hooks they need, define their configuration inputs, and generate the plugin manifest.
> That could call an internal API, run a custom check, retrieve information, or automate another step around the deployment.
> It gives them a way to package their own integrations and reuse them across stacks.

Reference: [Spacelift hooks](https://docs.spacelift.io/concepts/stack/stack-settings#customizing-workflow).

## 4. Stack Dependencies

Open [mayday-infrastructure's dependencies](https://spacelift-solutions.app.spacelift.io/stack/mayday-infrastructure/dependencies) and follow the connection to `mayday-service`.

> OpenTofu and Terraform are good at provisioning infrastructure, and Ansible is good at configuring it.
> Connecting those tools yourself can become awkward.
> If Ansible calls OpenTofu inside a task, the separate plan-and-review workflow gets buried inside the playbook.
> If OpenTofu calls Ansible through a provisioner, Ansible's execution becomes harder to inspect as its own configuration workflow.
> Stack dependencies let each tool run natively, with its own phases, output, and policies, while passing information between them.

Show the infrastructure stack's `inventory_tofu` output and its dependency reference to `TOFUSIBLE_INVENTORY` on the Ansible stack.
Open [Tofusible](https://github.com/spacelift-solutions/tofusible) to show the inventory handoff.

> Here, OpenTofu exports the machine information as a normal output using Tofusible.
> Spacelift passes that output to the Ansible stack.
> Ansible's dynamic inventory consumes it, and the playbook uses the resulting hosts normally.
> We keep the OpenTofu plan and apply visible, and we keep Ansible's tasks and host-level troubleshooting visible too.

Open [this completed dependency-triggered Ansible run](https://spacelift-solutions.app.spacelift.io/stack/mayday-service/run/01M679PH06WWZNCMFDMKZGN843) and its Tasks view.
Show the inventory host and an individual task's output before moving into self-service.

## 5. Request a license through a Template

Open **Templates**, select **Mayday sandbox license**, and show the instructions and license-key input.
For an authoring view, open its published version in **Templates Workbench** and show the form, stack definition, cloud integration, and attached plan policy.

> The platform team provides the implementation and controls.
> The consumer fills out a form without writing infrastructure code.

Create a deployment and enter `INVALID` as the license key.
Version 1.1.0 adds the deployment's creation timestamp to the stack name and object key, keeping the suffix stable when correcting inputs.
Use version 1.2.0 for disposable demos: one hour after creation, it deletes the deployment, stack, and sandbox object.
The shared bucket and Intent's live `license/demo-license.json` are outside that stack and remain intact.
The policy rejects the reserved deployment name `demo-license`, including timestamped variants.
The prepared older deployments keep their history because they have no automatic deletion schedule.
This creates a sandbox request under `license-requests/`; it does not replace the service's live license owned by Intent.
To avoid a planning wait, use the prepared **Mayday license demo** deployment and its recorded denied run in step 6.

Prepared records: [denied request](https://spacelift-solutions.app.spacelift.io/stack/mayday-license-mayday-license-demo/run/01M6F4Z95QEPG78R7ZAYTYEKH0), then [successful correction](https://spacelift-solutions.app.spacelift.io/stack/mayday-license-mayday-license-demo/run/01M6F552KDFE936AAZ4ZWCJPJH).

## 6. Show the denial, correct the input, and rerun

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
For a live run, continue the walkthrough while it executes, then return to show the result.
Updating deployment inputs is the supported workflow; see [the Templates Workbench guide](https://docs.spacelift.io/concepts/template/workbench).

## 7. Break the license

Open [Mayday](https://mayday.spacelift.shovel-emporium.com) and sign in.
Start with Healthy, a valid license, two workers, and Light traffic.
Click **Invalidate license** and wait for the dependency incident.
Open **Open diagnostic run** and show the failed diagnostic task's evidence.
The diagnostic failure is expected when the license is invalid.

## 8. Ask what is wrong and repair through Intent

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
We have now shown both a governed self-service request and a repair through Intent.

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
- Wiz scan `01M6F4ZKWR68NP9G5SV00361QM` failed authentication. Refresh the installed credentials and stage a successful scan before presenting Plugins Output; the older linked runs have no Wiz results.
- The installed `wiz-no-policies` plugin supplies scan output without a blocking policy. Use the Template plan policy in step 6 to demonstrate an explicit denial.
- Keep the denied and corrected Template runs ready so plan time does not interrupt the story.
- Resolve one application incident before starting the next; do not inject faults during an application deployment.
- Mayday's sandbox module is in `infra/license-request`; the Template, plan policy, and Wiz label are in `spacelift-solutions/demo`, under `admin/modules/mayday`.
- The existing Intent policy remains in `license.rego.tftpl`; the Template's separate Deploy policy is `license_request.rego.tftpl`.
