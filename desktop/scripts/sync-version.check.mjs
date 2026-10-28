import assert from "node:assert/strict";
import { cpSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { pathToFileURL } from "node:url";
import { afterEach, test } from "node:test";

const fixtures = [];
afterEach(() => {
  for (const path of fixtures.splice(0)) rmSync(path, { recursive: true, force: true });
});

function fixture() {
  const path = mkdtempSync(join(tmpdir(), "devorbit-version-"));
  fixtures.push(path);
  for (const directory of ["scripts", "src-tauri"]) mkdirSync(join(path, directory));
  for (const file of ["package.json", "package-lock.json", "scripts/sync-version.mjs", "src-tauri/tauri.conf.json", "src-tauri/Cargo.toml", "src-tauri/Cargo.lock"]) {
    cpSync(new URL(`../${file}`, import.meta.url), join(path, file));
  }
  return path;
}

test("the version hook synchronizes Tauri and Cargo without changing dependencies", async () => {
  const cwd = fixture();
  const read = (file) => readFileSync(join(cwd, file), "utf8");
  const before = [read("src-tauri/Cargo.toml"), read("src-tauri/Cargo.lock")];
  const manifest = JSON.parse(read("package.json"));
  manifest.version = "0.1.3";
  writeFileSync(join(cwd, "package.json"), JSON.stringify(manifest));
  await import(pathToFileURL(join(cwd, "scripts/sync-version.mjs")).href);
  assert.equal(JSON.parse(read("src-tauri/tauri.conf.json")).version, "0.1.3");
  for (const [index, file] of ["src-tauri/Cargo.toml", "src-tauri/Cargo.lock"].entries()) {
    assert.equal(read(file), before[index].replace(/(name = "devorbit-desktop"\r?\nversion = ")[^"]+(")/, (_, before, after) => `${before}0.1.3${after}`));
  }
});

test("a missing app version fails before writing Tauri files", async () => {
  const cwd = fixture();
  const config = join(cwd, "src-tauri/tauri.conf.json");
  const before = readFileSync(config, "utf8");
  writeFileSync(join(cwd, "src-tauri/Cargo.lock"), "");
  await assert.rejects(import(pathToFileURL(join(cwd, "scripts/sync-version.mjs")).href), /Cannot find app version in src-tauri\/Cargo.lock/);
  assert.equal(readFileSync(config, "utf8"), before);
});
