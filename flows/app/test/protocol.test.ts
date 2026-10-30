import { test } from "node:test";
import assert from "node:assert/strict";
import {
  parseEvent,
  route,
  isVerified,
  workersFor,
  type MonitorEvent,
} from "../protocol";
const sample = (now = Date.now()): MonitorEvent => ({
  id: "abcdefabcdefabcdefabcdef",
  type: "incident.opened",
  service: "mayday",
  at: new Date(now).toISOString(),
  incident: {
    id: "abcdefabcdefabcdefabcdea",
    kind: "outage",
    status: "open",
    opened_at: new Date(now).toISOString(),
  },
  observation: {
    at: new Date(now).toISOString(),
    reachable: true,
    samples: 4,
    errors: 0,
    latency_ms: 200,
    health: { status: "healthy", workers: 2 },
  },
});
test("routes independently observed incidents", () => {
  const e = sample();
  assert.equal(route(parseEvent(e)), "restart");
  e.incident!.kind = "dependency";
  assert.equal(route(e), "diagnose");
  e.type = "fault.outage";
  assert.equal(route(e), null);
});
test("verification rejects old and unrelated evidence", () => {
  const now = Date.now(),
    e = sample(now);
  e.incident!.status = "recovered";
  assert.equal(isVerified(e, e.incident!.id, now - 1000, now), true);
  assert.equal(isVerified(e, "different", now - 1000, now), false);
  assert.equal(isVerified(e, e.incident!.id, now + 1, now), false);
  assert.equal(isVerified(e, e.incident!.id, now - 1000, now + 16000), false);
  e.observation.errors = 1;
  assert.equal(isVerified(e, e.incident!.id, now - 1000, now), false);
});
test("capacity actions are bounded", () => {
  assert.equal(workersFor("scaleUp"), "8");
  assert.equal(workersFor("reset"), "2");
  assert.throws(() => workersFor("shell"));
});
test("rejects unrelated services and malformed events", () => {
  assert.throws(() => parseEvent({}));
  const e = sample();
  e.service = "production";
  assert.throws(() => parseEvent(e));
});
test("rejects incomplete and impossible health evidence", () => {
  for (const changes of [
    { health: null },
    { at: "invalid" },
    { samples: -1 },
    { errors: 5 },
    { latency_ms: NaN },
  ]) {
    const e = sample();
    Object.assign(e.observation, changes);
    assert.throws(() => parseEvent(e));
  }
});
