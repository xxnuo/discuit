const fs = require('fs');
const path = require('path');
const srcDir = path.join(__dirname, '..', 'src');

const namespaceMap = {
  'auth': 'auth',
  'settings': 'settings',
  'changePassword': 'settings',
  'deleteAccount': 'settings',
  'communities': 'community',
  'about': 'about',
};

function processFile(filePath) {
  let content = fs.readFileSync(filePath, 'utf-8');
  let changed = false;
  for (const [prefix, ns] of Object.entries(namespaceMap)) {
    const re = new RegExp("t\\('" + prefix + "\\.", 'g');
    if (re.test(content)) {
      if (ns === prefix) {
        content = content.replace(new RegExp("t\\('" + prefix + "\\.", 'g'), "t('" + ns + ":");
      } else {
        content = content.replace(new RegExp("t\\('" + prefix + "\\.", 'g'), "t('" + ns + ":" + prefix + ".");
      }
      changed = true;
    }
  }
  if (changed) {
    fs.writeFileSync(filePath, content);
    console.log('Updated: ' + path.relative(srcDir, filePath));
  }
}

function walkDir(dir) {
  const entries = fs.readdirSync(dir, { withFileTypes: true });
  for (const entry of entries) {
    const fullPath = path.join(dir, entry.name);
    if (entry.isDirectory() && entry.name !== 'node_modules') {
      walkDir(fullPath);
    } else if (entry.isFile() && (entry.name.endsWith('.tsx') || entry.name.endsWith('.ts')) && !entry.name.endsWith('.d.ts')) {
      processFile(fullPath);
    }
  }
}

walkDir(srcDir);
console.log('Done!');
