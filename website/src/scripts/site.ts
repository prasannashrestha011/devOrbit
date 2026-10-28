import { searchEntries, excerpt } from './search.mjs';

const root = document.documentElement;
const themeToggle = document.querySelector<HTMLButtonElement>('[data-theme-toggle]')!;
const systemTheme = matchMedia('(prefers-color-scheme: dark)');
const isDark = () => root.dataset.theme ? root.dataset.theme === 'dark' : systemTheme.matches;
const updateThemeLabel = () => {
  themeToggle.setAttribute('aria-label', `Switch to ${isDark() ? 'light' : 'dark'} theme`);
  document.querySelector('meta[name="theme-color"]')?.setAttribute('content', isDark() ? '#121b19' : '#ffffff');
};
themeToggle.hidden = false;
updateThemeLabel();
systemTheme.addEventListener('change', updateThemeLabel);
themeToggle.addEventListener('click', () => {
  root.dataset.theme = isDark() ? 'light' : 'dark';
  try { localStorage.setItem('devorbit.theme', root.dataset.theme); } catch {}
  updateThemeLabel();
});

const dialog = document.querySelector<HTMLDialogElement>('.search-dialog')!;
const input = dialog.querySelector<HTMLInputElement>('input')!;
const results = dialog.querySelector<HTMLUListElement>('.search-results')!;
const status = dialog.querySelector<HTMLParagraphElement>('.search-status')!;
const retry = dialog.querySelector<HTMLButtonElement>('[data-search-retry]')!;
const opener = document.querySelector<HTMLButtonElement>('[data-search-open]')!;
type Entry = { title: string; text: string; href: string };
let entries: Entry[] | undefined;
let loading = false;
let previousFocus: HTMLElement | null;
opener.hidden = false;
if (/Mac|iPhone|iPad/.test(navigator.platform)) opener.querySelector('kbd')!.textContent = '⌘ K';

function renderResults() {
  if (!entries) return;
  const query = input.value.trim();
  const matches: Entry[] = searchEntries(entries, query);
  status.textContent = !query ? 'Suggested guides' : matches.length ? `${matches.length} ${matches.length === 1 ? 'result' : 'results'}` : `No results for “${query}”. Try a service name, “Docker”, or “configuration”.`;
  results.replaceChildren(...matches.map((entry) => {
    const li = document.createElement('li');
    const link = document.createElement('a');
    link.href = entry.href;
    const title = document.createElement('strong');
    title.textContent = entry.title;
    const description = document.createElement('p');
    description.textContent = excerpt(entry.text, query);
    link.append(title, description);
    link.addEventListener('click', () => dialog.close());
    li.append(link);
    return li;
  }));
}

async function loadSearch() {
  if (entries) { renderResults(); return; }
  if (loading) return;
  loading = true;
  status.textContent = 'Loading documentation…';
  retry.hidden = true;
  try {
    const response = await fetch('/search-index.json');
    if (!response.ok) throw new Error('Search unavailable');
    entries = await response.json();
    renderResults();
  } catch {
    status.textContent = 'Documentation search couldn’t load. Check your connection and retry, or close search to browse the guides.';
    retry.hidden = false;
  } finally { loading = false; }
}

function openSearch() {
  if (dialog.open) return;
  previousFocus = document.activeElement as HTMLElement;
  dialog.showModal();
  document.body.style.overflow = 'hidden';
  input.focus();
  void loadSearch();
}
opener.addEventListener('click', openSearch);
retry.addEventListener('click', () => void loadSearch());
dialog.querySelector('[data-search-close]')!.addEventListener('click', () => dialog.close());
dialog.addEventListener('close', () => { document.body.style.overflow = ''; previousFocus?.focus(); });
dialog.addEventListener('click', (event) => { if (event.target === dialog) { const box = dialog.getBoundingClientRect(); if (event.clientX < box.left || event.clientX > box.right || event.clientY < box.top || event.clientY > box.bottom) dialog.close(); } });
input.addEventListener('input', renderResults);
dialog.querySelector('form')!.addEventListener('submit', (event) => { event.preventDefault(); results.querySelector('a')?.click(); });
document.addEventListener('keydown', (event) => {
  if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') { event.preventDefault(); if (dialog.open) dialog.close(); else openSearch(); }
});
dialog.addEventListener('keydown', (event) => {
  if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return;
  const links = [...results.querySelectorAll('a')];
  if (!links.length) return;
  event.preventDefault();
  const index = links.indexOf(document.activeElement as HTMLAnchorElement);
  if (index === -1) links[event.key === 'ArrowDown' ? 0 : links.length - 1].focus();
  else if (event.key === 'ArrowUp' && index === 0) input.focus();
  else links[(index + (event.key === 'ArrowDown' ? 1 : -1) + links.length) % links.length].focus();
});

for (const pre of document.querySelectorAll<HTMLPreElement>('pre')) {
  const frame = document.createElement('div');
  frame.className = 'code-frame';
  const toolbar = document.createElement('div');
  toolbar.className = 'code-toolbar';
  const label = document.createElement('span');
  label.textContent = pre.closest<HTMLElement>('[data-code-title]')?.dataset.codeTitle || pre.dataset.language || 'Code';
  const copy = document.createElement('button');
  copy.type = 'button';
  copy.className = 'copy-code';
  copy.textContent = 'Copy';
  copy.setAttribute('aria-label', `Copy ${label.textContent}`);
  copy.addEventListener('click', async () => {
    try {
      await navigator.clipboard.writeText(pre.querySelector('code')?.textContent ?? pre.textContent ?? '');
      copy.textContent = 'Copied';
      document.querySelector('#copy-status')!.textContent = `${label.textContent} copied to clipboard.`;
      window.setTimeout(() => { copy.textContent = 'Copy'; }, 2000);
    } catch {
      const range = document.createRange();
      range.selectNodeContents(pre);
      window.getSelection()?.removeAllRanges();
      window.getSelection()?.addRange(range);
      document.querySelector('#copy-status')!.textContent = 'Clipboard unavailable. Code selected; use your device’s copy command.';
      copy.textContent = 'Code selected';
    }
  });
  toolbar.append(label, copy);
  pre.before(frame);
  frame.append(toolbar, pre);
  pre.tabIndex = 0;
  pre.setAttribute('aria-label', `${label.textContent} code`);
}
for (const table of document.querySelectorAll('.prose table')) {
  const wrapper = document.createElement('div');
  wrapper.className = 'table-scroll';
  wrapper.tabIndex = 0;
  wrapper.setAttribute('role', 'region');
  wrapper.setAttribute('aria-label', 'Scrollable reference table');
  table.before(wrapper);
  wrapper.append(table);
}
const headings = [...document.querySelectorAll<HTMLElement>('.prose h2[id], .prose h3[id]')];
for (const heading of headings) {
  const anchor = document.createElement('a');
  anchor.href = `#${heading.id}`;
  anchor.className = 'heading-anchor';
  anchor.setAttribute('aria-label', `Link to ${heading.textContent}`);
  anchor.textContent = '#';
  heading.append(anchor);
}
const tocLinks = [...document.querySelectorAll<HTMLAnchorElement>('.toc nav a')];
if (tocLinks.length) {
  let scheduled = false;
  const updateOutline = () => {
    let current = headings[0]?.id;
    for (const heading of headings) if (heading.getBoundingClientRect().top <= 140) current = heading.id;
    if (innerHeight + scrollY >= document.documentElement.scrollHeight - 4) current = headings.at(-1)?.id;
    for (const link of tocLinks) {
      if (link.hash === `#${current}`) link.setAttribute('aria-current', 'location');
      else link.removeAttribute('aria-current');
    }
    scheduled = false;
  };
  addEventListener('scroll', () => { if (!scheduled) { scheduled = true; requestAnimationFrame(updateOutline); } }, { passive: true });
  updateOutline();
}
