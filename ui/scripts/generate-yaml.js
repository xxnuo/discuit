import { promisify } from 'node:util';
import { execFile } from 'node:child_process';
import { access, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const execFileAsync = promisify(execFile);
const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const rootPath = path.resolve(scriptDir, '../..');
const discuitBin = path.join(rootPath, 'discuit');

let command = discuitBin;
let args = ['inject-config'];

try {
  await access(discuitBin);
} catch {
  command = 'go';
  args = ['run', '.', 'inject-config'];
}

const { stdout, stderr } = await execFileAsync(command, args, { cwd: rootPath });

if (stderr) {
  throw new Error(stderr);
}

await writeFile(path.join(rootPath, 'ui-config.yaml'), stdout);
console.log('ui-config.yaml has been written');
