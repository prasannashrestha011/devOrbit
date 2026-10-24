import { spawnSync } from "node:child_process";
import { mkdirSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const desktop = dirname(dirname(fileURLToPath(import.meta.url)));
const target = process.env.TAURI_ENV_TARGET_TRIPLE || spawnSync("rustc", ["--print", "host-tuple"], { encoding: "utf8" }).stdout.trim();
if (!target) throw new Error("Cannot determine the Rust target triple");

const binaries = join(desktop, "src-tauri", "binaries");
mkdirSync(binaries, { recursive: true });
const output = join(binaries, `devorbit-${target}${target.includes("windows") ? ".exe" : ""}`);
const result = spawnSync("go", ["build", "-trimpath", "-o", output, "./cmd/devorbit"], {
  cwd: join(desktop, "..", "server"),
  stdio: "inherit",
  env: { ...process.env, CGO_ENABLED: "1" },
});
if (result.error) throw result.error;
if (result.status !== 0) process.exit(result.status || 1);
