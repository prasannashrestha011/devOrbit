const normalize = (value) => value.toLocaleLowerCase().normalize('NFKC');

export function searchEntries(entries, query) {
  const terms = normalize(query).trim().split(/\s+/).filter(Boolean);
  if (!terms.length) return entries.filter((entry) => !entry.href.includes('#')).slice(0, 5);
  return entries.map((entry) => {
    const title = normalize(entry.title);
    const text = normalize(entry.text);
    const haystack = `${title} ${text}`;
    if (!terms.every((term) => haystack.includes(term))) return null;
    const score = terms.reduce((total, term) => total + (title.includes(term) ? 10 : 1), 0) + (title === normalize(query.trim()) ? 30 : 0);
    return { ...entry, score };
  }).filter(Boolean).sort((a, b) => b.score - a.score).slice(0, 8);
}

export function excerpt(text, query) {
  const terms = normalize(query).trim().split(/\s+/).filter(Boolean);
  const position = terms.map((term) => normalize(text).indexOf(term)).filter((index) => index >= 0).sort((a, b) => a - b)[0] ?? 0;
  const start = Math.max(0, position - 45);
  return `${start ? '…' : ''}${text.slice(start, start + 160)}${text.length > start + 160 ? '…' : ''}`;
}
