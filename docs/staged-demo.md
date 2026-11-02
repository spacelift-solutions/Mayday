# Mayday demo without deployment waits

Start with an open PR whose preview plan has already finished, then switch to live incident recovery.
Open the linked tabs before the call.
Keep [PR #15](https://github.com/spacelift-solutions/Mayday/pull/15) open before the presentation.
The PR proposes a new change; the running application was deployed earlier.

## 1. Review a staged PR and its completed plan (2–3 minutes)

| Open | Show and explain |
| --- | --- |
| [Open Mayday PR #15](https://github.com/spacelift-solutions/Mayday/pull/15) | A developer adds `Application = "mayday"` to the EC2 target's tags. Show the small diff, PR checks, and review controls. Keep it unmerged for reuse. |
| [PR #15 preview plan](https://spacelift-solutions.app.spacelift.io/stack/mayday-infrastructure/run/01M6F39M68Q4PQ57WVVPA2QAJG) | Show the planned tag update on the existing instance. The preview has not applied the change. |
| [Infrastructure stack](https://spacelift-solutions.app.spacelift.io/stack/mayday-infrastructure) | Show the dependency graph and `inventory_tofu` output passed as `TOFUSIBLE_INVENTORY` to Ansible. |
| [Existing Ansible deployment](https://spacelift-solutions.app.spacelift.io/stack/mayday-service/run/01M679PH06WWZNCMFDMKZGN843) | Show Ansible's Tasks view from the earlier dependency-triggered deployment. This explains how the running service was configured; it is not an apply of PR #15. |

Talk track:

> I opened this PR before our call so its plan is ready for review.
> The developer wants to label this instance with the application it belongs to.
> We can inspect the exact proposed change before merging anything.
> Merging starts a tracked run; this infrastructure stack requires confirmation before apply.
> OpenTofu provisions the infrastructure and passes inventory to Ansible, which configures the service.
> That deployment is already running, so let's see how we operate it when something breaks.

The main walkthrough leaves the PR open and does not wait for a new deployment.
If you choose to demonstrate merging, do so after the incident scenarios; review and confirm the resulting infrastructure run separately.
Do not inject faults while a deployment is changing the target.
Once PR #15 is merged or closed, prepare a fresh small change and completed preview before the next demo rather than presenting a merged PR as pending review.

If someone asks to see completed apply output, open [this earlier infrastructure apply](https://spacelift-solutions.app.spacelift.io/stack/mayday-infrastructure/run/01M6EYYNRV2JQ1CHDTNR6G2SPW) and identify it as historical.
The staged tag change does not demonstrate a Deploy policy rejection or security scanner finding.

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
