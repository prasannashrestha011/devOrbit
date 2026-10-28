import { readFileSync, writeFileSync } from "node:fs";

const desktop = new URL("../", import.meta.url);
const { version } = JSON.parse(readFileSync(new URL("package.json", desktop), "utf8"));
const files = [
  ["src-tauri/tauri.conf.json", /("version"\s*:\s*")[^"]+(")/],
  ["src-tauri/Cargo.toml", /(\[package\]\r?\nname = "devorbit-desktop"\r?\nversion = ")[^"]+(")/],
  ["src-tauri/Cargo.lock", /(\[\[package\]\]\r?\nname = "devorbit-desktop"\r?\nversion = ")[^"]+(")/],
];

// Validate every match before changing any files.
const updates = files.map(([path, pattern]) => {
  const file = new URL(path, desktop);
  const content = readFileSync(file, "utf8");
  if (!pattern.test(content)) throw new Error(`Cannot find app version in ${path}`);
  return [file, content.replace(pattern, (_, before, after) => `${before}${version}${after}`)];
});
for (const [file, content] of updates) writeFileSync(file, content);
