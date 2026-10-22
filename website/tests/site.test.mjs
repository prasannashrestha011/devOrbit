import assert from 'node:assert/strict';
import { readFileSync, readdirSync, statSync } from 'node:fs';
import { resolve, sep } from 'node:path';
import { test } from 'node:test';

const dist = resolve('dist');
const files = readdirSync(dist, { recursive: true }).filter((file) => statSync(resolve(dist, file)).isFile());
const pages = new Map(files.filter((file) => file.endsWith('.html')).map((file) => [
  `/${file.split(sep).join('/')}`, readFileSync(resolve(dist, file), 'utf8'),
]));

test('all requested pages have a title, description, main landmark, and one heading', () => {
  for (const route of ['', 'architecture/', 'docs/', 'docs/installation/', 'docs/configuration/', 'docs/services/', 'docs/api/']) {
    assert.ok(pages.has(`/${route}index.html`), `Missing page: /${route}`);
  }
  assert.ok(pages.has('/404.html'));
  for (const [path, html] of pages) {
    assert.match(html, /<html lang="en"/, path);
    assert.match(html, /<title>[^<]+DevOrbit<\/title>/, path);
    assert.match(html, /<meta name="description" content="[^"]+"/, path);
    assert.match(html, /<main id="main"/, path);
    assert.equal([...html.matchAll(/<h1(?:\s|>)/g)].length, 1, path);
  }
});

test('every internal link, fragment, and asset resolves in the static build', () => {
  let checked = 0;
  for (const [path, html] of pages) {
    for (const [, , value] of html.matchAll(/\b(href|src)="([^"]+)"/g)) {
      const url = new URL(value.replaceAll('&amp;', '&'), `https://devorbit.test${path}`);
      if (url.origin !== 'https://devorbit.test') continue;
      const target = decodeURIComponent(url.pathname);
      const file = target.endsWith('/') ? `${target}index.html` : target;
      assert.ok(files.includes(file.slice(1)), `${path}: missing target ${value}`);
      if (url.hash) {
        const targetHtml = pages.get(file);
        const ids = [...(targetHtml ?? '').matchAll(/\bid="([^"]+)"/g)].map((match) => match[1]);
        assert.ok(ids.includes(decodeURIComponent(url.hash.slice(1))), `${path}: missing anchor ${value}`);
      }
      checked++;
    }
  }
  assert.ok(checked > 100, 'Expected navigation and table-of-contents links to be rendered');
});

test('the website ships no JavaScript and stays below a 256 KiB total asset budget', () => {
  for (const [path, html] of pages) assert.doesNotMatch(html, /<script\b/i, path);
  assert.ok(!files.some((file) => /\.(?:m?js)$/.test(file)));
  const bytes = files.reduce((total, file) => total + statSync(resolve(dist, file)).size, 0);
  assert.ok(bytes < 256 * 1024, `Static output is ${bytes} bytes`);
});
