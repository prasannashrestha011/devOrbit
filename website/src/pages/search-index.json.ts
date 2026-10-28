import type { APIRoute, MarkdownInstance } from 'astro';
import { guides } from '../data/navigation';

const documents = import.meta.glob<MarkdownInstance<{ title: string; label: string; description: string }>>('./docs/*.md', { eager: true });
const plainText = (html: string) => html.replace(/<[^>]*>/g, ' ').replace(/&(?:amp|lt|gt|quot|#39);/g, (entity) => ({ '&amp;': '&', '&lt;': '<', '&gt;': '>', '&quot;': '"', '&#39;': "'" })[entity]!).replace(/\s+/g, ' ').trim();

export const GET: APIRoute = async () => {
  const entries = (await Promise.all(guides.map(async ({ href }) => {
    const doc = Object.values(documents).find((doc) => doc.url === href || `${doc.url}/` === href);
    if (!doc) return [];
    const sections = (await doc.compiledContent()).split(/(?=<h[23]\b)/);
    return [
      { href, title: doc.frontmatter.title, text: doc.frontmatter.description },
      ...sections.flatMap((section) => {
        const heading = section.match(/^<h[23]\b[^>]*id="([^"]+)"[^>]*>(.*?)<\/h[23]>/);
        if (!heading) return [];
        return [{ href: `${href}#${heading[1]}`, title: `${plainText(heading[2])} · ${doc.frontmatter.label}`, text: plainText(section.slice(heading[0].length)) }];
      }),
    ];
  }))).flat();
  entries.push({ href: '/architecture/', title: 'Architecture', text: 'React and Tauri desktop, authenticated local API, Go backend, process supervisor, workspace YAML, SQLite metadata, local logs. The desktop starts its bundled backend automatically.' });
  return new Response(JSON.stringify(entries), { headers: { 'Content-Type': 'application/json' } });
};
