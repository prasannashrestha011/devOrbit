import { useRef } from "react";

function highlightLine(line: string) {
  let quote = "";
  let colon = -1;
  let comment = line.length;
  for (let i = 0; i < line.length; i++) {
    const char = line[i];
    if (quote) {
      if (char === "\\" && quote === '"') i++;
      else if (char === quote) {
        if (quote === "'" && line[i + 1] === "'") i++;
        else quote = "";
      }
    } else if (char === '"' || char === "'") {
      quote = char;
    } else if (char === "#" && (i === 0 || /\s/.test(line[i - 1]))) {
      comment = i;
      break;
    } else if (char === ":" && colon < 0 && (i === line.length - 1 || /\s/.test(line[i + 1]))) {
      colon = i;
    }
  }

  const code = line.slice(0, comment);
  const suffix = comment < line.length ? <span className="yaml-comment">{line.slice(comment)}</span> : null;
  if (colon < 0 || colon >= comment) return <>{code}{suffix}</>;

  const key = code.slice(0, colon);
  const indentation = key.match(/^\s*/)?.[0] ?? "";
  const value = code.slice(colon + 1);
  const valueIndentation = value.match(/^\s*/)?.[0] ?? "";
  const scalar = value.slice(valueIndentation.length);
  const valueClass = /^[-+]?\d+(?:\.\d+)?$/.test(scalar.trim()) ? "yaml-number" : "yaml-value";
  return <>
    {indentation}<span className="yaml-key">{key.slice(indentation.length)}</span><span className="yaml-punctuation">:</span>
    {valueIndentation}<span className={valueClass}>{scalar}</span>{suffix}
  </>;
}

export default function YamlSource({ value, onChange }: { value: string; onChange: (value: string) => void }) {
  const highlight = useRef<HTMLPreElement>(null);
  return <div className="yaml-source-wrap">
    <pre className="yaml-highlight" aria-hidden="true" ref={highlight}>{value.split("\n").map((line, index, lines) =>
      <span key={index}>{highlightLine(line)}{index < lines.length - 1 ? "\n" : null}</span>
    )}</pre>
    <textarea
      id="configuration-source"
      className="mono configuration-source"
      spellCheck={false}
      wrap="off"
      value={value}
      onChange={(event) => onChange(event.target.value)}
      onScroll={(event) => {
        if (highlight.current) {
          highlight.current.scrollTop = event.currentTarget.scrollTop;
          highlight.current.scrollLeft = event.currentTarget.scrollLeft;
        }
      }}
    />
  </div>;
}
