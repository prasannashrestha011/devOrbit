import { expect, it } from "vitest";
import { formatCommandLine, parseCommandLine } from "./commandLine";

it("splits normal commands and keeps quoted arguments together", () => {
  expect(parseCommandLine("flutter run -d linux")).toEqual(["flutter", "run", "-d", "linux"]);
  expect(parseCommandLine('flutter run --dart-define "GREETING=hello world"')).toEqual([
    "flutter", "run", "--dart-define", "GREETING=hello world",
  ]);
  expect(parseCommandLine("flutter run\n -d linux")).toEqual(["flutter", "run", "-d", "linux"]);
});

it("preserves existing command and argument values through the editor", () => {
  const words = ["/tmp/Flutter SDK/flutter", "run", "--name", "hello world", "", 'a"b', "C:\\tools"];
  expect(parseCommandLine(formatCommandLine(words))).toEqual(words);
});

it("rejects incomplete quotes", () => {
  expect(() => parseCommandLine('flutter run "unfinished')).toThrow("Close the quote");
});
