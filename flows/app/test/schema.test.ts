import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { runInNewContext } from "node:vm";
import { transformSync } from "esbuild";

test("Flows can load the named app export and choose default outputs", () => {
  const source = readFileSync(new URL("../main.ts", import.meta.url), "utf8");
  const { code } = transformSync(source, {
    loader: "ts",
    format: "cjs",
    target: "es2022",
  });
  const module = { exports: {} as Record<string, any> };
  runInNewContext(code, {
    module,
    exports: module.exports,
    require: (name: string) => {
      if (name === "@slflows/sdk/v1")
        return { defineApp: (app: unknown) => app };
      if (name === "./protocol") return {};
      throw new Error(`Unexpected schema dependency: ${name}`);
    },
  });
  // The Flows executor imports { app }; handlers are not invoked during schema loading.
  assert.ok(
    module.exports.app?.blocks,
    "The executor needs a named app export",
  );
  for (const [name, block] of Object.entries<any>(module.exports.app.blocks)) {
    const outputs = Object.values<any>(block.outputs ?? {});
    if (outputs.length > 1) {
      assert.equal(
        outputs.filter((output) => output.default === true).length,
        1,
        `${name} needs exactly one default output`,
      );
    }
  }
});
