import { cpSync, rmSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const rootPath = path.resolve(scriptDir, '..');

cpSync(path.join(rootPath, 'dist-sw'), path.join(rootPath, 'dist'), { recursive: true });
rmSync(path.join(rootPath, 'dist-sw'), { recursive: true, force: true });
