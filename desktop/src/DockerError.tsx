import { TriangleAlert } from "lucide-react";

export default function DockerError({ error, title = "Docker operation failed" }: { error: string; title?: string }) {
  const variables = [...new Set(Array.from(error.matchAll(/required variable ([A-Za-z_][A-Za-z0-9_]*) is missing a value/g), (match) => match[1]))];
  return <div className="docker-error" role="alert">
    <strong><TriangleAlert size={16} aria-hidden="true" />{variables.length ? "Compose configuration is incomplete" : title}</strong>
    {variables.length ? <>
      <p>Provide non-empty values in the environment used by Docker Compose, then refresh or retry the operation.</p>
      <ul>{variables.map((name) => <li key={name}><code>{name}</code></li>)}</ul>
      <details><summary>Technical details</summary><pre>{error}</pre></details>
    </> : <pre>{error}</pre>}
  </div>;
}
