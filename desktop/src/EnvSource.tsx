import { useRef } from "react";

export default function EnvSource({ value, onChange, onSave }: {
  value: string;
  onChange: (value: string) => void;
  onSave: () => void;
}) {
  const highlight = useRef<HTMLPreElement>(null);
  return <div className="yaml-source-wrap env-source-wrap">
    <pre className="yaml-highlight" aria-hidden="true" ref={highlight}>{value.replace(/\r\n?/g, "\n").split("\n").map((line, index, lines) => {
      const match = line.match(/^(\s*)(export\s+)?([A-Za-z_][A-Za-z0-9_]*)(\s*=\s*)(.*)$/);
      return <span key={index}>{match ? <>{match[1]}<span className="yaml-key">{match[2]}{match[3]}</span><span className="yaml-punctuation">{match[4]}</span><span className="yaml-value">{match[5]}</span></> : line.trimStart().startsWith("#") ? <span className="yaml-comment">{line}</span> : line}{index < lines.length - 1 ? "\n" : null}</span>;
    })}</pre>
    <textarea
      aria-label="Environment file contents"
      className="mono configuration-source"
      spellCheck={false}
      wrap="off"
      value={value}
      onChange={(event) => {
        const endings = value.match(/\r\n|\r|\n/g) ?? [];
        const next = event.target.value;
        const sameLineCount = endings.length === (next.match(/\n/g)?.length ?? 0);
        let index = 0;
        onChange(next.replace(/\n/g, () => sameLineCount ? endings[index++] : endings[0] ?? "\n"));
      }}
      onKeyDown={(event) => {
        if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "s") {
          event.preventDefault();
          onSave();
        }
      }}
      onScroll={(event) => {
        if (highlight.current) {
          highlight.current.scrollTop = event.currentTarget.scrollTop;
          highlight.current.scrollLeft = event.currentTarget.scrollLeft;
        }
      }}
    />
  </div>;
}
