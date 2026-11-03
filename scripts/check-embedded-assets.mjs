import { existsSync, readFileSync, statSync } from 'node:fs';
import { resolve } from 'node:path';

const staticDirectory = resolve(process.env.EMBEDDED_STATIC_DIR ?? 'internal/transport/httpapi/static');
const browserEntry = resolve(staticDirectory, 'index.html');
const appEntry = resolve(staticDirectory, 'app.html');

const fail = (message) => {
  console.error(`Embedded UI verification failed: ${message}`);
  console.error('Run make web-build before building the release executable.');
  process.exit(1);
};

const readRequired = (path, label, marker = 'Workbook editor') => {
  if (!existsSync(path) || !statSync(path).isFile()) {
    fail(`${label} is missing at ${path}`);
  }
  const content = readFileSync(path, 'utf8');
  if (!content.includes(marker)) {
    fail(`${label} does not contain the expected application marker`);
  }
  return content;
};

const browser = readRequired(browserEntry, 'browser entry');
const app = readRequired(appEntry, 'MCP App entry');
const referencedAssets = [...browser.matchAll(/(?:src|href)="(\/assets\/[^"]+)"/g)].map((match) => match[1]);
if (referencedAssets.length < 2) {
  fail('browser entry does not reference both JavaScript and CSS assets');
}
for (const asset of referencedAssets) {
  const path = resolve(staticDirectory, asset.slice(1));
  if (!existsSync(path) || !statSync(path).isFile() || statSync(path).size === 0) {
    fail(`browser asset ${asset} is missing or empty`);
  }
}
const docx = readRequired(resolve(staticDirectory, 'docx.html'), 'DOCX browser entry', 'DOCX');
const docxAssets = [...docx.matchAll(/(?:src|href)="(\/assets\/[^"]+)"/g)].map((match) => match[1]);
if (docxAssets.length < 2) fail('DOCX entry does not reference JavaScript and CSS assets');
for (const asset of docxAssets) {
  const path = resolve(staticDirectory, asset.slice(1));
  if (!existsSync(path) || !statSync(path).isFile() || statSync(path).size === 0) fail(`DOCX asset ${asset} is missing or empty`);
}
if (/(?:src|href)="\/assets\//.test(app)) {
  fail('MCP App entry is not self-contained');
}
if (!app.includes('<script') || !app.includes('<style')) {
  fail('MCP App entry does not contain inlined script and style content');
}

console.log(`Embedded UI verification passed: ${referencedAssets.length} browser assets and one self-contained MCP App bundle`);
