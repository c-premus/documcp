import { mkdirSync, writeFileSync } from 'node:fs';
import { resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { buildDocuMCPDashboard } from './dashboards/documcp.js';

const __dirname = dirname(fileURLToPath(import.meta.url));

interface DashboardDef {
  name: string;
  path: string;
  build: () => object;
}

const distDir = resolve(__dirname, '../../dist');
mkdirSync(distDir, { recursive: true });

const dashboards: DashboardDef[] = [
  {
    name: 'DocuMCP',
    path: resolve(distDir, 'documcp.json'),
    build: () => buildDocuMCPDashboard().build(),
  },
];

for (const { name, path, build } of dashboards) {
  const json = JSON.stringify(build(), null, 2) + '\n';
  writeFileSync(path, json);
  console.log(`Generated: ${name} → ${path}`);
}
