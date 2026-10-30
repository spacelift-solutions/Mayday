import {
  defineApp,
  events,
  http,
  kv,
  timers,
  type AppBlock,
  type AppContext,
  type EntityOnTimerInput,
} from "@slflows/sdk/v1";
import {
  parseEvent,
  route,
  isVerified,
  workersFor,
  type MonitorEvent,
} from "./protocol";

async function graphql(
  app: AppContext,
  query: string,
  variables: Record<string, unknown> = {},
) {
  const endpoint = String(app.config.spaceliftEndpoint).replace(/\/$/, "");
  const url = new URL(endpoint);
  if (url.protocol !== "https:" || !url.hostname.endsWith(".spacelift.io"))
    throw new Error("Use your Spacelift HTTPS account endpoint");
  async function request(
    q: string,
    vars: Record<string, unknown>,
    token?: string,
  ) {
    const res = await fetch(endpoint + "/graphql", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...(token ? { Authorization: "Bearer " + token } : {}),
      },
      body: JSON.stringify({ query: q, variables: vars }),
      signal: AbortSignal.timeout(15000),
    });
    if (!res.ok) throw new Error(`Spacelift returned HTTP ${res.status}`);
    const body = await res.json();
    if (body.errors?.length)
      throw new Error(
        body.errors.map((e: { message: string }) => e.message).join("; "),
      );
    return body.data;
  }
  const auth = await request(
    "mutation($id: ID!, $secret: String!) { apiKeyUser(id:$id, secret:$secret) { jwt } }",
    { id: app.config.apiKeyID, secret: app.config.apiKeySecret },
  );
  return request(query, variables, auth.apiKeyUser.jwt);
}

const monitor: AppBlock = {
  name: "Monitor events",
  description:
    "Receives independently observed Mayday incidents and health samples",
  entrypoint: true,
  outputs: Object.fromEntries(
    [
      "restart",
      "diagnose",
      "scaleUp",
      "scaleDown",
      "reset",
      "recovered",
      "failed",
    ].map((key) => [key, { name: key, type: { type: "object" } }]),
  ),
  http: {
    onRequest: async (input) => {
      const { request, app } = input;
      if (request.method !== "POST") {
        await http.respond(request.requestId, { statusCode: 405 });
        return;
      }
      const auth =
        request.headers.authorization || request.headers.Authorization;
      if (
        !app.config.webhookToken ||
        auth !== `Bearer ${app.config.webhookToken}`
      ) {
        await http.respond(request.requestId, { statusCode: 401 });
        return;
      }
      let e: MonitorEvent;
      try {
        e = parseEvent(request.body);
      } catch {
        await http.respond(request.requestId, { statusCode: 400 });
        return;
      }
      const existing = (await kv.app.get("latest")).value as
        MonitorEvent | undefined;
      if (!existing || Date.parse(existing.at) <= Date.parse(e.at))
        await kv.app.set({ key: "latest", value: e, ttl: 600 });
      const output = route(e);
      if (output) {
        const key = `event:${e.id}`,
          lockKey = `lock:${key}`,
          previous = await kv.app.get(key);
        if (previous.value !== "delivered") {
          const lock = crypto.randomUUID();
          if (
            !(await kv.app.set({
              key: lockKey,
              value: true,
              lock: { id: lock, timeout: 60 },
              ttl: 86400,
            }))
          ) {
            await http.respond(request.requestId, { statusCode: 503 });
            return;
          }
          try {
            if ((await kv.app.get(key)).value !== "delivered") {
              await events.emit(e, { outputKey: output });
              await kv.app.set({ key, value: "delivered", ttl: 86400 });
            }
          } finally {
            await kv.app.releaseLock({ key: lockKey, lockId: lock });
          }
        }
      }
      await http.respond(request.requestId, {
        statusCode: 202,
        body: { accepted: true },
      });
    },
  },
};

interface Job {
  event: MonitorEvent;
  stack: string;
  run: string;
  since: number;
  deadline: number;
  action: string;
  pending: string;
  parent: string;
}
async function watch(input: EntityOnTimerInput) {
  const j = input.timer.payload as Job;
  if (Date.now() > j.deadline) {
    await events.emit(
      { ...j, reason: "Run or health verification timed out" },
      { outputKey: "failed", complete: j.pending, parentEventId: j.parent },
    );
    return;
  }
  const data = await graphql(
    input.app,
    "query($stack: ID!, $run: ID!) { stack(id:$stack) { run(id:$run) { id state } } }",
    { stack: j.stack, run: j.run },
  );
  const state = data.stack?.run?.state;
  if (
    ["FAILED", "CANCELED", "DISCARDED", "STOPPED", "SKIPPED"].includes(state)
  ) {
    const output =
      j.action === "diagnose" && state === "FAILED" ? "diagnosed" : "failed";
    await events.emit(
      { incident: j.event.incident, stack: j.stack, run: j.run, state },
      { outputKey: output, complete: j.pending, parentEventId: j.parent },
    );
    return;
  }
  if (state === "FINISHED") {
    if (j.action === "diagnose") {
      await events.emit(
        { incident: j.event.incident, stack: j.stack, run: j.run, state },
        {
          outputKey: "diagnosed",
          complete: j.pending,
          parentEventId: j.parent,
        },
      );
      return;
    }
    const latest = (await kv.app.get("latest")).value as
      MonitorEvent | undefined;
    if (isVerified(latest, j.event.incident!.id, j.since)) {
      await events.emit(
        {
          incident: j.event.incident,
          stack: j.stack,
          run: j.run,
          observation: latest!.observation,
        },
        {
          outputKey: "recovered",
          complete: j.pending,
          parentEventId: j.parent,
        },
      );
      return;
    }
  }
  await timers.block.set(5, {
    inputPayload: j,
    pendingEventId: j.pending,
    description: `Mayday run ${j.run}: ${state || "unknown"}; waiting for independent verification`,
  });
}

const repair: AppBlock = {
  name: "Run Spacelift repair",
  description:
    "Runs a bounded Ansible action, then waits for the monitor to prove recovery",
  config: {
    action: {
      name: "Action",
      type: {
        type: "string",
        enum: ["restart", "scaleUp", "scaleDown", "reset", "diagnose"],
      },
      required: true,
    },
  },
  inputs: {
    default: {
      config: {
        incident: {
          name: "Monitor event",
          type: { type: "object" },
          required: true,
        },
      },
      onEvent: async (input) => {
        const e = parseEvent(input.event.inputConfig.incident),
          action = String(input.block.config.action),
          workers = workersFor(action);
        if (!e.incident || route(e) !== action)
          throw new Error("Incident does not match this action");
        const stack = String(
          action === "diagnose"
            ? input.app.config.diagnosticsStackID
            : input.app.config.serviceStackID,
        );
        const key = `repair:${e.id}`;
        const lockKey = `lock:${key}`;
        if ((await kv.app.get(key)).value) return;
        const lock = crypto.randomUUID();
        if (
          !(await kv.app.set({
            key: lockKey,
            value: true,
            ttl: 86400,
            lock: { id: lock, timeout: 120 },
          }))
        )
          throw new Error("Repair already in progress");
        try {
          if ((await kv.app.get(key)).value) return;
          await kv.app.set({ key, value: "starting", ttl: 86400 });
          if (workers !== null)
            await graphql(
              input.app,
              "mutation($stack: ID!, $config: ConfigInput!) { stackConfigAdd(stack:$stack,config:$config) { id } }",
              {
                stack,
                config: {
                  id: "WORKERS",
                  type: "ENVIRONMENT_VARIABLE",
                  value: workers,
                  writeOnly: false,
                },
              },
            );
          const since = Date.now();
          const data = await graphql(
            input.app,
            "mutation($stack: ID!) { runTrigger(stack:$stack) { id } }",
            { stack },
          );
          const run = String(data.runTrigger.id);
          await kv.app.set({
            key,
            value: { run, stack },
            ttl: 86400,
          });
          const pending = await events.createPending({
            statusDescription: `Waiting for Spacelift run ${run} and measured recovery`,
            parentEventId: input.event.id,
          });
          await timers.block.set(5, {
            inputPayload: {
              event: e,
              stack,
              run,
              since,
              deadline: since + 10 * 60 * 1000,
              action,
              pending,
              parent: input.event.id,
            } satisfies Job,
            pendingEventId: pending,
          });
        } finally {
          await kv.app.releaseLock({ key: lockKey, lockId: lock });
        }
      },
    },
  },
  outputs: {
    recovered: { name: "Verified recovery", type: { type: "object" } },
    diagnosed: {
      name: "Evidence ready for Intelligence",
      type: { type: "object" },
    },
    failed: { name: "Repair failed or timed out", type: { type: "object" } },
  },
  onTimer: watch,
};

export default defineApp({
  name: "Mayday Monitor",
  installationInstructions:
    "Configure the scoped Spacelift API key, then point the monitor at the Monitor events block's HTTP endpoint. The monitor needs outbound HTTPS only.",
  config: {
    webhookToken: {
      name: "Monitor webhook token",
      type: "string",
      required: true,
      sensitive: true,
    },
    spaceliftEndpoint: {
      name: "Spacelift account URL",
      type: "string",
      required: true,
      default: "https://spacelift-solutions.app.spacelift.io",
    },
    apiKeyID: { name: "Spacelift API key ID", type: "string", required: true },
    apiKeySecret: {
      name: "Spacelift API key secret",
      type: "string",
      required: true,
      sensitive: true,
    },
    serviceStackID: {
      name: "Service stack",
      type: "string",
      required: true,
      default: "mayday-service",
    },
    diagnosticsStackID: {
      name: "Diagnostics stack",
      type: "string",
      required: true,
      default: "mayday-diagnostics",
    },
  },
  blocks: {
    monitor,
    repair,
    getIncident: {
      name: "Get incident evidence",
      inputs: {
        default: {
          onEvent: async () => {
            const latest = await kv.app.get("latest");
            if (!latest.value) throw new Error("No recent monitor evidence");
            await events.emit(latest.value);
          },
        },
      },
      outputs: { default: { type: { type: "object" } } },
    },
  },
});
