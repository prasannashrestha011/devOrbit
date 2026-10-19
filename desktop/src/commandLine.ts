// Parse one executable and its arguments without shell expansion.
export function parseCommandLine(input: string): string[] {
  const words: string[] = [];
  let word = "";
  let quote = "";
  let started = false;
  for (let i = 0; i < input.length; i++) {
    const char = input[i];
    const next = input[i + 1];
    if (char === "\\" && quote !== "'" && next && (/[\s"'\\]/.test(next))) {
      word += next;
      started = true;
      i++;
    } else if (quote && char === quote) {
      quote = "";
    } else if (!quote && (char === "'" || char === '"')) {
      quote = char;
      started = true;
    } else if (!quote && /\s/.test(char)) {
      if (started) words.push(word);
      word = "";
      started = false;
    } else {
      word += char;
      started = true;
    }
  }
  if (quote) throw new Error("Close the quote in the command.");
  if (started) words.push(word);
  if (!words[0]) throw new Error("Enter an executable command.");
  return words;
}

export function formatCommandLine(words: string[]): string {
  return words
    .map((word) => /[\s"'\\]/.test(word) || !word
      ? '"' + word.replace(/\\/g, "\\\\").replace(/"/g, '\\"') + '"'
      : word)
    .join(" ");
}
