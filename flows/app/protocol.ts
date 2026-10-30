export interface MonitorEvent {
  id: string;
  type: string;
  service: string;
  at: string;
  incident?: { id: string; kind: string; status: string; opened_at: string };
  observation: {
    at: string;
    reachable: boolean;
    samples: number;
    errors: number;
    latency_ms: number;
    health: { status: string; workers: number };
  };
}

export function parseEvent(body: unknown): MonitorEvent {
  if (!body || typeof body !== "object")
    throw new Error("Expected a monitor event");
  const e = body as MonitorEvent;
  if (
    !/^[a-f0-9]{24}$/.test(e.id) ||
    e.service !== "mayday" ||
    typeof e.type !== "string" ||
    !Number.isFinite(Date.parse(e.at)) ||
    !e.observation ||
    typeof e.observation.reachable !== "boolean" ||
    !Number.isFinite(Date.parse(e.observation.at)) ||
    !Number.isSafeInteger(e.observation.samples) ||
    e.observation.samples < 0 ||
    !Number.isSafeInteger(e.observation.errors) ||
    e.observation.errors < 0 ||
    e.observation.errors > e.observation.samples ||
    !Number.isFinite(e.observation.latency_ms) ||
    e.observation.latency_ms < 0 ||
    !e.observation.health ||
    typeof e.observation.health.status !== "string" ||
    !Number.isSafeInteger(e.observation.health.workers)
  )
    throw new Error("Invalid monitor event");
  if (
    e.type.startsWith("incident.") &&
    (!e.incident || !/^[a-f0-9]{24}$/.test(e.incident.id))
  )
    throw new Error("Incident ID required");
  return e;
}

export function route(e: MonitorEvent): string | null {
  if (e.type === "incident.recovered") return "recovered";
  if (e.type === "incident.failed") return "failed";
  if (e.type !== "incident.opened") return null;
  return (
    (
      {
        outage: "restart",
        dependency: "diagnose",
        capacity_up: "scaleUp",
        capacity_down: "scaleDown",
        reset: "reset",
      } as Record<string, string>
    )[e.incident?.kind || ""] || null
  );
}

export function isVerified(
  e: MonitorEvent | undefined,
  incidentID: string,
  since: number,
  now = Date.now(),
): boolean {
  if (!e || e.incident?.id !== incidentID || e.incident.status !== "recovered")
    return false;
  const o = e.observation,
    at = Date.parse(o.at);
  return (
    Number.isFinite(at) &&
    at >= since &&
    now - at < 15000 &&
    at <= now + 5000 &&
    o.reachable &&
    o.health.status === "healthy" &&
    o.samples > 0 &&
    o.errors === 0 &&
    o.latency_ms <= 600
  );
}

export function workersFor(action: string): string | null {
  switch (action) {
    case "scaleUp":
      return "8";
    case "scaleDown":
    case "reset":
      return "2";
    case "restart":
    case "diagnose":
      return null;
    default:
      throw new Error("Unknown recovery action");
  }
}
