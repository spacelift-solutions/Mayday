let signedIn = false,
  epoch = 0,
  rate = 2,
  busy = false,
  currentAttempt = "",
  pendingAction = null;
const el = (id) => document.getElementById(id),
  text = (id, value) => {
    el(id).textContent = value;
  };
const labels = {
  "fault.outage": "Outage requested",
  "fault.license": "License invalidated",
  "incident.opened": "Incident detected",
  "incident.recovered": "Recovery verified",
  "incident.failed": "Recovery timed out",
  "traffic.changed": "Traffic adjusted",
};
const kinds = {
  outage: "Service unavailable",
  dependency: "Required dependency unavailable",
  capacity_up: "Capacity under pressure",
  capacity_down: "Capacity ready to scale down",
  reset: "Restoring the baseline",
};
const flowStatuses = {
  started: "Run started",
  succeeded: "Run finished",
  evidence_ready: "Diagnostic evidence ready",
  failed: "Recovery failed",
};
function link(id, url) {
  const anchor = el(id);
  anchor.hidden = !url;
  if (url) anchor.href = url;
  else anchor.removeAttribute("href");
}
function followRecovery(s) {
  const i = s.incident,
    nav = s.navigation || {};
  currentAttempt = i?.attempt_id || "";
  if (pendingAction && currentAttempt !== pendingAction.attempt) pendingAction = null;
  const kind = pendingAction?.kind || i?.kind,
    stack = pendingAction
      ? kind === "dependency" ? nav.diagnostics_url : nav.service_url
      : nav.stack_url,
    run = pendingAction ? "" : nav.run_url;
  link("flow-link", nav.flow_url);
  link("stack-link", stack);
  link("run-link", run);
  text("stack-link", kind === "dependency" ? "Open diagnostics stack ↗" : "Open service stack ↗");
  text("run-link", kind === "dependency" ? "Open diagnostic run ↗" : "Open recovery run ↗");
  text("follow-status", pendingAction
    ? "Waiting for the monitor to detect the incident. Follow the Flow while it starts recovery."
    : run
      ? "Open the exact run for this attempt. This dashboard continues checking service health."
      : i
        ? "Follow the Flow now. The run link appears when Spacelift starts the run."
        : "Choose a fault or change traffic, then follow recovery here.");
  el("investigate-prompt").hidden = kind !== "dependency";
}
function showSession(active) {
  signedIn = active;
  el("login-panel").hidden = active;
  el("workspace").hidden = !active;
  el("logout").hidden = !active;
  text("connection", active ? "CONNECTED" : "SIGNED OUT");
  el("connection").className = active ? "pill good" : "pill";
}
async function api(path, body) {
  const res = await fetch("/api/" + path, {
    method: body === undefined ? "GET" : "POST",
    credentials: "same-origin",
    headers: { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const data = await res.json();
  if (!res.ok) {
    if (res.status === 401 && path !== "login") showSession(false);
    throw Error(data.error || "Request failed");
  }
  return data;
}
async function refresh() {
  const started = epoch;
  try {
    const s = await api("state"),
      o = s.observation,
      h = o.health || {},
      i = s.incident;
    if (started !== epoch) return;
    showSession(true);
    rate = s.rate;
    text("connection", "CONNECTED");
    el("connection").className = "pill good";
    text(
      "health",
      o.reachable
        ? h.status === "healthy"
          ? "Healthy"
          : "Degraded"
        : "Unavailable",
    );
    el("health").className =
      o.reachable && h.status === "healthy" ? "good" : "bad";
    text("license", "License: " + (h.license || "unknown"));
    text("rate", rate);
    document.querySelectorAll("[data-rate]").forEach((button) => {
      button.setAttribute(
        "aria-pressed",
        String(Number(button.dataset.rate) === rate),
      );
      button.disabled = busy;
    });
    text("latency", Math.round(o.latency_ms || 0));
    text("errors", ((o.error_rate || 0) * 100).toFixed(1) + "%");
    text(
      "workers",
      (h.workers || 0) + " workers · " + (h.active || 0) + " busy",
    );
    text("queue", h.queued || 0);
    text(
      "observed",
      o.at ? new Date(o.at).toLocaleTimeString() : "Awaiting telemetry",
    );
    text("incident-status", i ? i.status.toUpperCase() : "STANDING BY");
    text(
      "incident-title",
      i
        ? i.status === "recovered"
          ? "Resolved"
          : kinds[i.kind] || i.kind
        : "No active incident",
    );
    text("incident-id", i ? i.id : "None");
    text("incident-message", i?.message || "");
    followRecovery(s);
    const result = pendingAction ? null : i?.flow_result;
    text(
      "flow-result",
      result
        ? (flowStatuses[result.status] || result.status) +
          (result.run_id ? " · " + result.run_id : "")
        : i
          ? "Awaiting result"
          : "None",
    );
    text("delivery", s.delivery_error || "");
    el("retry").disabled = busy || !i || i.status !== "failed";
    const active = i && i.status !== "recovered";
    document
      .querySelectorAll("[data-action]")
      .forEach((b) => (b.disabled = busy || active));
    el("reset").disabled = busy || active;
    const history = (s.history || []).slice().reverse();
    el("timeline").replaceChildren(
      ...history.map((e) => {
        const li = document.createElement("li"),
          t = document.createElement("time"),
          v = document.createElement("span");
        t.textContent = new Date(e.at).toLocaleTimeString();
        v.textContent =
          (labels[e.type] || e.type) +
          (e.incident && e.type.startsWith("incident.")
            ? " · " + (kinds[e.incident.kind] || e.incident.kind)
            : "");
        if (e.type === "incident.recovered") v.className = "good";
        li.append(t, v);
        return li;
      }),
    );
  } catch (e) {
    if (started !== epoch || !signedIn) return;
    text("connection", "DISCONNECTED");
    el("connection").className = "pill bad";
    text("message", e.message);
  }
}
async function act(path, body = {}) {
  if (busy) return;
  busy = true;
  const attempt = currentAttempt;
  try {
    const response = await api(path, body);
    if (path.startsWith("fault/") || path === "retry" || path === "reset") {
      pendingAction = { attempt, kind: path === "fault/license" ? "dependency" : path === "retry" ? null : "outage" };
      el("follow-recovery").scrollIntoView({ behavior: "smooth", block: "nearest" });
    }
    text("message", response.status || "Traffic updated. Watch the service metrics and follow any recovery below.");
  } catch (e) {
    text("message", e.message);
  } finally {
    busy = false;
    await refresh();
  }
}
el("login").addEventListener("submit", async (e) => {
  e.preventDefault();
  const button = el("login").querySelector("button");
  button.disabled = true;
  text("login-error", "");
  try {
    await api("login", { password: el("password").value });
    el("password").value = "";
    epoch++;
    showSession(true);
    await refresh();
  } catch (error) {
    text("login-error", error.message);
  } finally {
    button.disabled = false;
  }
});
el("logout").addEventListener("click", async () => {
  epoch++;
  try {
    await api("logout", {});
    showSession(false);
    el("password").focus();
  } catch (error) {
    text("message", error.message);
  }
});
document
  .querySelectorAll("[data-action]")
  .forEach((b) =>
    b.addEventListener("click", () => act("fault/" + b.dataset.action)),
  );
document
  .querySelectorAll("[data-rate]")
  .forEach((button) =>
    button.addEventListener("click", () =>
      act("traffic", { rate: Number(button.dataset.rate) }),
    ),
  );
el("reset").addEventListener("click", () => act("reset"));
el("retry").addEventListener("click", () => act("retry"));
setInterval(() => {
  if (signedIn) refresh();
}, 2000);
refresh();
