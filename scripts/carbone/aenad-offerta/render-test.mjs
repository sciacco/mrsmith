// Carica offerta-template.docx su Carbone Cloud e renderizza i payload di
// samples/ in PDF sotto artifacts/claude/aenad-renders/ per il confronto
// visivo con gli originali in artifacts/aenad/.
//
// Uso: node render-test.mjs            (legge CARBONE_API_KEY da backend/.env)
import { readFileSync, readdirSync, writeFileSync, mkdirSync } from 'node:fs';

const API = 'https://api.carbone.io';
const repoRoot = new URL('../../../', import.meta.url);
const outDir = new URL('artifacts/claude/aenad-renders/', repoRoot);

function apiKey() {
  if (process.env.CARBONE_API_KEY) return process.env.CARBONE_API_KEY;
  const env = readFileSync(new URL('backend/.env', repoRoot), 'utf8');
  const m = env.match(/^CARBONE_API_KEY=(.+)$/m);
  if (!m) throw new Error('CARBONE_API_KEY non trovata (env o backend/.env)');
  return m[1].trim();
}

const KEY = apiKey();
const headers = { Authorization: `Bearer ${KEY}`, 'carbone-version': '4' };

async function uploadTemplate() {
  const buf = readFileSync(new URL('./offerta-template.docx', import.meta.url));
  const form = new FormData();
  form.append(
    'template',
    new Blob([buf], { type: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document' }),
    'offerta-template.docx',
  );
  const res = await fetch(`${API}/template`, { method: 'POST', headers, body: form });
  const body = await res.json();
  if (!res.ok || !body.success) throw new Error(`upload fallito: ${res.status} ${JSON.stringify(body)}`);
  return body.data.templateId;
}

async function render(templateId, data) {
  const res = await fetch(`${API}/render/${templateId}`, {
    method: 'POST',
    headers: { ...headers, 'Content-Type': 'application/json' },
    body: JSON.stringify({ data, convertTo: 'pdf' }),
  });
  const body = await res.json();
  if (!res.ok || !body.success) throw new Error(`render fallito: ${res.status} ${JSON.stringify(body)}`);
  const dl = await fetch(`${API}/render/${body.data.renderId}`, { headers });
  if (!dl.ok) throw new Error(`download fallito: ${dl.status}`);
  return Buffer.from(await dl.arrayBuffer());
}

const templateId = await uploadTemplate();
console.log('templateId:', templateId);

mkdirSync(outDir, { recursive: true });
const samples = readdirSync(new URL('./samples/', import.meta.url)).filter((f) => f.endsWith('.json'));
for (const file of samples) {
  const data = JSON.parse(readFileSync(new URL(`./samples/${file}`, import.meta.url), 'utf8'));
  const pdf = await render(templateId, data);
  const out = new URL(file.replace(/\.json$/, '.pdf'), outDir);
  writeFileSync(out, pdf);
  console.log('renderizzato:', out.pathname);
}
