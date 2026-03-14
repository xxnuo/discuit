const fs = require('fs');
const path = require('path');

function walk(dir, callback) {
  fs.readdirSync(dir).forEach((f) => {
    let dirPath = path.join(dir, f);
    let isDirectory = fs.statSync(dirPath).isDirectory();
    isDirectory ? walk(dirPath, callback) : callback(dirPath);
  });
}

walk('./src', function (filePath) {
  if (filePath.endsWith('.tsx') || filePath.endsWith('.ts')) {
    let content = fs.readFileSync(filePath, 'utf8');
    // We only replace t('common.*') where * is alphanumeric
    let newContent = content.replace(/t\('common\.([A-Za-z0-9_]+)'([^)]*)\)/g, "t('$1'$2)");
    
    // Some are inside objects like t(`common.${var}`), or translation namespaces.
    // For now, replacing the literal string t('common.key') covers most.
    if (content !== newContent) {
      fs.writeFileSync(filePath, newContent, 'utf8');
      console.log('Fixed:', filePath);
    }
  }
});
