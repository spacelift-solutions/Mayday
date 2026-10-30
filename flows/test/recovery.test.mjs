import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { Script, runInNewContext } from "node:vm";
import test from "node:test";

// Exercise the actual exported expressions and links, without an SDK or a
// duplicate implementation of the recovery logic. PyYAML is already used by CI.
const flow = JSON.parse(execFileSync(process.env.PYTHON || "python3", ["-c", `
import json, sys, yaml
class Loader(yaml.SafeLoader): pass
Loader.add_constructor('!expr', lambda loader, node: {'expression': loader.construct_scalar(node)})
Loader.add_constructor('!blockUnits', lambda loader, node: loader.construct_sequence(node))
with open(sys.argv[1]) as source: print(json.dumps(yaml.load(source, Loader=Loader)))
`, fileURLToPath(new URL("../recovery.yaml", import.meta.url))], { encoding: "utf8" }));
const blocks = Object.fromEntries(flow.blocks.map(b => [b.name, b]));
const now = Date.now();
function evaluate(value, context = {}) {
  if (value && typeof value === "object" && "expression" in value) {
    return runInNewContext(`(${value.expression})`, {
      Date: { now: () => now, parse: Date.parse }, ...context,
    }, { timeout: 1000 });
  }
  return value;
}
const input = (name, key = "default") => blocks[name].inputs[key].config;
const condition = (name, outputs, secrets = {}) => evaluate(input(name).condition, { outputs, secrets });
const sample = () => ({
  id: "abcdefabcdefabcdefabcdef", type: "incident.opened", service: "mayday",
  at: new Date(now).toISOString(),
  incident: { id: "abcdefabcdefabcdefabcdea", kind: "outage", status: "open" },
  observation: { at: new Date(now).toISOString(), reachable: true, samples: 4,
    errors: 0, latency_ms: 200, health: { status: "healthy", workers: 2 } },
});
const context = event => ({ normalizeIncident: { event, since: now - 1000, deadline: now + 600000 } });
const next = (name, output) => flow.blocks.filter(b => Object.values(b.inputs || {})
  .some(i => (i.links || []).some(l => l.block === name && l.output === output)));

test("definition uses native blocks, valid links, and compilable expressions", () => {
  assert.equal(Object.keys(blocks).length, flow.blocks.length);
  for (const block of flow.blocks) {
    assert.ok(block.type.coreBlock);
    for (const socket of Object.values(block.inputs || {})) {
      for (const link of socket.links || []) assert.ok(blocks[link.block], link.block);
    }
    if (block.errorHandler) assert.equal(blocks[block.errorHandler].type.coreBlock, "handleError");
  }
  function check(value) {
    if (!value || typeof value !== "object") return;
    if (value.expression) new Script(`(${value.expression})`);
    else Object.values(value).forEach(check);
  }
  check(flow);
});

test("webhook fails closed and accepts only the monitor token", () => {
  const outputs = { monitorWebhook: { headers: { authorization: "Bearer test-token" } } };
  assert.equal(condition("Authenticate monitor", outputs), false);
  assert.equal(condition("Authenticate monitor", outputs, { MAYDAY_WEBHOOK_TOKEN: "wrong" }), false);
  assert.equal(condition("Authenticate monitor", outputs, { MAYDAY_WEBHOOK_TOKEN: "test-token" }), true);
  assert.ok(blocks["Monitor webhook"].outputs.requests.sensitivePaths.includes("$.headers.authorization"));
});

test("accepts Go observation events and rejects malformed evidence", () => {
  const e = sample();
  e.type = "observation";
  assert.equal(condition("Validate observation", { monitorWebhook: { body: e } }), true);
  assert.equal(condition("New incident", context(e)), false);
  for (const bad of [{ ...e, service: "other" }, { ...e, type: "fault.outage" },
    { ...e, observation: { ...e.observation, errors: 5 } },
    { ...e, observation: { ...e.observation, at: "invalid" } }, {}]) {
    assert.equal(condition("Validate observation", { monitorWebhook: { body: bad } }), false);
  }
});

test("all incident branches reach the intended stack with bounded capacity", () => {
  for (const [kind, stack, workers] of [["outage", "mayday-service"],
    ["dependency", "mayday-diagnostics"], ["capacity_up", "mayday-service", "8"],
    ["capacity_down", "mayday-service", "2"], ["reset", "mayday-service", "2"]]) {
    const e = sample(); e.incident.kind = kind;
    const outputs = context(e);
    let block = blocks["Missing dependency"], capacity;
    while (block.type.coreBlock === "condition") {
      const targets = next(block.name, condition(block.name, outputs) ? "then" : "else");
      assert.equal(targets.length, 1); block = targets[0];
    }
    if (block.type.kind === "graphql") {
      capacity = evaluate(input(block.name, "request")["variables.config"], { outputs }).value;
      assert.equal(evaluate(input(block.name, "request")["variables.stack"]), "mayday-service");
      block = next(block.name, "result")[0];
    }
    assert.equal(capacity, workers, kind);
    assert.equal(block.type.kind, "triggerRun");
    assert.equal(evaluate(input(block.name, "trigger").stackId), stack);
    assert.equal(evaluate(input(block.name, "trigger").runType), "TRACKED");
    assert.equal(next(block.name, "completed")[0].name, "Capture run result");
  }
});

test("deduplicates before dispatch and releases intake on both outcomes", () => {
  assert.equal(condition("First delivery", { findPriorDelivery: { found: true } }), false);
  assert.equal(condition("First delivery", { findPriorDelivery: { found: false } }), true);
  assert.equal(next("First delivery", "else")[0].name, "Acknowledge duplicate");
  assert.ok(next("Acknowledge duplicate", "default").some(b => b.name === "Serialize event intake"));
  assert.ok(next("Catch intake errors", "default").some(b => b.name === "Serialize event intake"));
  assert.ok(next("Remember delivery", "default").some(b => b.name === "New incident"));
});

test("out-of-order observations cannot replace newer recovery evidence", () => {
  const stored = sample();
  const old = sample(); old.at = new Date(now - 10000).toISOString();
  assert.equal(evaluate(input("Store observation").value, { outputs: context(old), stored }), stored);
});

test("verification rejects failed, stale, future, unrelated and empty observations", () => {
  const e = sample(); e.incident.status = "recovered";
  const outputs = { ...context(e), readIndependentObservation: { value: e } };
  assert.equal(condition("Recovery verified", outputs), true);
  for (const mutate of [e => e.incident.id = "different", e => e.incident.status = "open",
    e => e.observation.errors = 1, e => e.observation.samples = 0,
    e => e.observation.at = new Date(now - 16000).toISOString(),
    e => e.observation.at = new Date(now + 6000).toISOString(),
    e => e.observation.reachable = false, e => e.observation.health.status = "degraded"]) {
    const bad = structuredClone(e); mutate(bad);
    assert.equal(condition("Recovery verified", { ...outputs, readIndependentObservation: { value: bad } }), false);
  }
  assert.equal(condition("Recovery verified", { ...outputs, readIndependentObservation: { found: false } }), false);
  assert.equal(condition("Recovery verified", { ...outputs, normalizeIncident: { ...outputs.normalizeIncident, deadline: now } }), false);
});

test("run completion and deadline preserve terminal outcomes", () => {
  for (const state of ["FAILED", "CANCELED", "DISCARDED"]) {
    assert.equal(condition("Run finished successfully", { captureRunResult: { state } }), false);
  }
  assert.equal(condition("Run finished successfully", { captureRunResult: { state: "FINISHED" } }), true);
  for (const status of ["recovered", "failed", "evidence_ready"]) {
    const stored = { status };
    for (const name of ["Record verified recovery", "Record failed recovery", "Evidence ready for Intelligence"]) {
      assert.equal(evaluate(input(name).value, { stored, outputs: {} }), stored);
    }
  }
});
