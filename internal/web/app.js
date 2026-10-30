let signedIn = false,
  epoch = 0,
  rate = 2,
  busy = false;
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
    const stack = history.find((e) => e.stack_id)?.stack_id || "mayday-service";
    el("stack-link").href =
      "https://spacelift-solutions.app.spacelift.io/stack/" +
      encodeURIComponent(stack);
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
  try {
    await api(path, body);
    text("message", "Done.");
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
