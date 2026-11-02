# Mayday demo without deployment waits

Use completed runs for the developer workflow, then switch to live incident recovery.
Open the linked tabs before the call.
Say that the deployment completed ahead of time; the PR, plan, apply, and policy results are real records.

## 1. Walk through a completed deployment (2–3 minutes)

| Open | Show and explain |
| --- | --- |
| [Mayday PR #7](https://github.com/spacelift-solutions/Mayday/pull/7) | A developer changes license fault injection from deleting the object to disabling its content. Show the diff, review, and checks. |
| [Completed PR plan](https://spacelift-solutions.app.spacelift.io/stack/mayday-infrastructure/run/01M6EYW5TB6T529R9409C3X7GB) | Open the plan output. The monitor IAM permission changes from `s3:DeleteObject` to `s3:PutObject` for the designated license object. This preview did not apply the change. |
| [Completed tracked apply](https://spacelift-solutions.app.spacelift.io/stack/mayday-infrastructure/run/01M6EYYNRV2JQ1CHDTNR6G2SPW) | The merged commit produced a tracked run. Show its plan and completed apply. |
| [Infrastructure stack](https://spacelift-solutions.app.spacelift.io/stack/mayday-infrastructure) | Show the dependency graph and `inventory_tofu` output passed as `TOFUSIBLE_INVENTORY` to Ansible. |
| [Completed dependency-triggered Ansible deployment](https://spacelift-solutions.app.spacelift.io/stack/mayday-service/run/01M679PH06WWZNCMFDMKZGN843) | Show Ansible's Tasks view. This is an earlier deployment that was actually triggered by the infrastructure dependency; distinguish it from the PR #7 runs triggered by Git. |

Talk track:

> I ran this deployment before our call so we can spend our time on the decisions.
> Here is the proposed change and its plan, followed by the merged commit and apply.
> OpenTofu provisions the infrastructure and passes inventory to Ansible, which configures the service.
> Each tool keeps its own run history and execution view.

These records show review, planning, deployment, and the handoff between tools.
They do not contain a staged Deploy policy rejection or security scanner finding.

## 2. Recover a service outage live

Open [Mayday](https://mayday.spacelift.shovel-emporium.com), sign in, and start with Healthy, two workers, and Light traffic.
Click **Cause outage**, then use **Open recovery Flow** and **Open recovery run**.

Keep the validation and reporting groups collapsed.
Follow the service-outage branch into Ansible.
Return to Mayday to show that completed work requests, independently observed by the monitor, establish recovery.

## 3. Show Intent deny an incorrect repair, then accept the correction

Click **Invalidate license** and open the diagnostic run.
Show the failed task's evidence, then ask Infra Assistant:

> What's happening with this service?

After the diagnosis, explain that the next proposal deliberately uses the wrong JSON type to demonstrate enforcement.
Do not rely on the assistant accidentally making a mistake, or describe this injected mistake as an unprompted AI error.

In the **Mayday license** Intent project, ask:

> Demonstrate the existing license policy with one deliberately incorrect repair.
> Refresh the existing mayday-license resource.
> Submit an Intent update with content {"service":"mayday","license_key":"DEMO-ONLY","enabled":"true"} followed by a newline, and the ETag matching those exact bytes.
> Keep the current bucket/key, empty ACL, and AES256 encryption.
> The enabled value must be the string "true" for this test.
> Leave the policy unchanged.
> Stop after the denial and show me the tool result; do not correct it yet.

The JSON is syntactically valid, but the existing policy requires `entitlement.enabled == true`, a boolean.
The expected server-side response is:

```text
POLICY RESTRICTION: Operation blocked by governance policy.

- Restore the private, AES256-encrypted demo entitlement with its documented content
```

Show the denied operation and the policy condition.
A chat explanation that a request would fail is not an executed policy denial: use the actual tool result or the staged operation record below if the assistant declines to submit it.

Then ask:

> Correct enabled to the boolean true and repair the same mayday-license resource through Intent.
> Refresh first, then use the exact checked-in demo-license.json content with its trailing newline and ETag 0ddb3ebffccfbffc98c50fbbcfd18fdc.
> Preserve the bucket/key, empty ACL, and AES256 encryption.
> Leave the policy unchanged.
> Show the successful operation and check live service health.

Show the allowed operation, then return to Mayday for recovery.
Talk track: the proposed change was rejected before it could replace the object; correcting the input allowed the repair through the same policy.
This demonstrates Intent policy enforcement, not an administrator's inability to edit policy or a Deploy plan policy.

### Staged policy records

The November 2 rehearsal submitted these operations against the existing resource:

- Wrong-type proposal: `01M6F2T2GD29ZR01DAY84NYKPG`; returned the policy denial above.
- Corrected proposal: `01M6F2TDJD18QTSM9BE4YXNW1Q`; finished with `Scoped Mayday license repair permitted` and no denials.

Open the **Mayday license** project's operation history before presenting.
These records allow the policy discussion without submitting another operation.
The staged correction was submitted while the license was already valid; use the live invalidation sequence above when demonstrating recovery from an outage.

## 4. Show capacity recovery

Select **Heavy**, follow the Flow and Ansible run, and show eight workers handling the same elevated load.
Select **Light** and show the cooldown returning to two.

Worker capacity is currently owned by Flows and applied by Ansible.
Intent owns only the license object.
An even-worker-count constraint would be a separate Deploy governance example; it is not enforced by the current Intent policy.

Finish with the application healthy, a valid license, Light traffic, and two workers.
