import { mkdirSync, writeFileSync } from 'node:fs';
import { resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { buildDocuMCPDashboard } from './dashboards/documcp.js';
import { buildDocuMCPAlerts } from './alerts/rules.js';

const __dirname = dirname(fileURLToPath(import.meta.url));

interface Artifact {
  name: string;
  path: string;
  build: () => object;
}

const distDir = resolve(__dirname, '../../dist');
const alertsDir = resolve(distDir, 'alerts');
mkdirSync(distDir, { recursive: true });
mkdirSync(alertsDir, { recursive: true });

const artifacts: Artifact[] = [
  {
    name: 'DocuMCP dashboard',
    path: resolve(distDir, 'documcp.json'),
    build: () => buildDocuMCPDashboard().build(),
  },
  {
    name: 'DocuMCP alerts',
    path: resolve(alertsDir, 'documcp.json'),
    build: () => buildDocuMCPAlerts(),
  },
];

for (const { name, path, build } of artifacts) {
  const json = JSON.stringify(build(), null, 2) + '\n';
  writeFileSync(path, json);
  console.log(`Generated: ${name} → ${path}`);
}
