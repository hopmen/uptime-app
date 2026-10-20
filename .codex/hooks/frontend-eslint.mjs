import { readFileSync } from "node:fs";
import { dirname, isAbsolute, relative, resolve } from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "../..");
const frontendRoot = resolve(repoRoot, "frontend");

function editedPaths(event) {
  const input = event.tool_input;

  if (event.tool_name === "apply_patch") {
    const patch = typeof input === "string" ? input : input?.command;
    if (typeof patch !== "string") {
      throw new Error("В событии apply_patch отсутствует текст патча.");
    }

    return [...patch.matchAll(/^\*\*\* (?:Add File|Update File|Delete File|Move to): (.+)\r?$/gm)]
      .map((match) => match[1].replace(/\r$/, ""));
  }

  if (typeof input?.file_path !== "string") {
    throw new Error("В событии редактирования отсутствует file_path.");
  }

  return [input.file_path];
}

function reportFailure(reason) {
  console.log(JSON.stringify({ decision: "block", reason }));
}

try {
  const event = JSON.parse(readFileSync(0, "utf8"));
  const touchesFrontend = editedPaths(event).some((path) => {
    const target = resolve(event.cwd ?? repoRoot, path);
    const local = relative(frontendRoot, target);
    return local === "" || (local !== ".." && !local.startsWith("../") && !isAbsolute(local));
  });

  if (touchesFrontend) {
    const result = spawnSync("npm", ["run", "lint"], {
      cwd: frontendRoot,
      encoding: "utf8",
      timeout: 110_000,
      maxBuffer: 4 * 1024 * 1024,
      shell: false,
    });

    if (result.error || result.status !== 0) {
      reportFailure([
        "ESLint фронтенда не прошёл. Исправь ошибки и повтори npm run lint из frontend/.",
        result.error?.message,
        result.stdout,
        result.stderr,
      ].filter(Boolean).join("\n"));
    } else {
      console.log(JSON.stringify({
        hookSpecificOutput: {
          hookEventName: "PostToolUse",
          additionalContext: "ESLint фронтенда: npm run lint выполнен успешно.",
        },
      }));
    }
  }
} catch (error) {
  reportFailure(`Не удалось выполнить хук ESLint фронтенда: ${error.message}`);
}
