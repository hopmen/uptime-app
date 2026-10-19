import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { copyFileSync, existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";

function fixture(t, scenario) {
  const root = mkdtempSync(join(tmpdir(), "openapi skill "));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const skillScripts = join(root, ".codex/skills/backend-openapi/scripts");
  const backend = join(root, "backend");
  const bin = join(root, "bin");
  for (const dir of [skillScripts, join(backend, "docs"), join(backend, "scripts"), bin]) {
    mkdirSync(dir, { recursive: true });
  }
  const script = join(skillScripts, "sync-docs.mjs");
  copyFileSync(new URL("./sync-docs.mjs", import.meta.url), script);
  // Start with an old contract: reproducibility must not compare against Git.
  writeFileSync(join(backend, "docs/openapi.json"), "old JSON");
  writeFileSync(join(backend, "docs/openapi.yaml"), "old YAML");
  writeFileSync(join(bin, "go"), String.raw`#!${process.execPath}
import { appendFileSync, existsSync, readFileSync, writeFileSync } from "node:fs";
if (process.argv.slice(2).join(" ") !== "generate ./docs") process.exit(99);
const count = existsSync("count") ? Number(readFileSync("count", "utf8")) + 1 : 1;
writeFileSync("count", String(count));
appendFileSync("steps", "go\n");
if (process.env.SCENARIO === "fail-" + count) process.exit(7);
for (const format of ["json", "yaml"]) {
  const suffix = process.env.SCENARIO === "unstable-" + format ? count : "stable";
  writeFileSync("docs/openapi." + format, format + suffix);
}
`, { mode: 0o755 });
  writeFileSync(join(backend, "scripts/generate-redoc.mjs"), String.raw`
import { appendFileSync, readFileSync, writeFileSync } from "node:fs";
appendFileSync("steps", "redoc\n");
if (process.env.SCENARIO === "redoc-failure") process.exit(8);
writeFileSync("docs/redoc.html", readFileSync("docs/openapi.json"));
`);
  const result = spawnSync(process.execPath, [script], {
    cwd: tmpdir(), encoding: "utf8",
    env: { ...process.env, PATH: bin + ":" + process.env.PATH, SCENARIO: scenario },
  });
  assert.ifError(result.error);
  assert.ok(existsSync(join(backend, "steps")), result.stdout + result.stderr);
  return { result, backend, steps: readFileSync(join(backend, "steps"), "utf8").trim().split("\n") };
}

test("generates current contract then ReDoc, regardless of cwd and spaces in paths", (t) => {
  const { result, backend, steps } = fixture(t, "success");
  assert.equal(result.status, 0, result.stderr);
  assert.deepEqual(steps, ["go", "go", "redoc"]);
  assert.equal(readFileSync(join(backend, "docs/redoc.html"), "utf8"), "jsonstable");
});

for (const attempt of [1, 2]) {
  test(`stops before ReDoc when generation ${attempt} fails`, (t) => {
    const { result, steps } = fixture(t, `fail-${attempt}`);
    assert.notEqual(result.status, 0);
    assert.deepEqual(steps, Array(attempt).fill("go"));
  });
}

for (const format of ["json", "yaml"]) {
  test(`rejects non-reproducible ${format} before ReDoc`, (t) => {
    const { result, steps } = fixture(t, `unstable-${format}`);
    assert.notEqual(result.status, 0);
    assert.deepEqual(steps, ["go", "go"]);
    assert.match(result.stderr, new RegExp(`openapi\\.${format}`));
  });
}

test("reports ReDoc failure as incomplete synchronization", (t) => {
  const { result, steps } = fixture(t, "redoc-failure");
  assert.notEqual(result.status, 0);
  assert.deepEqual(steps, ["go", "go", "redoc"]);
});
