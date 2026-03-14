import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const localesDir = path.join(__dirname, '..', 'src', 'i18n', 'locales');
const enDir = path.join(localesDir, 'en');
const namespaces = fs.readdirSync(enDir).filter(f => f.endsWith('.json')).map(f => f.replace('.json', ''));
const languages = ['zh-TW', 'ja', 'ko', 'es', 'fr', 'de', 'pt', 'ru', 'ar', 'hi'];

function createStubs(enObj, lang) {
  const result = {};
  for (const [key, value] of Object.entries(enObj)) {
    if (typeof value === 'object' && value !== null) {
      result[key] = createStubs(value, lang);
    } else {
      result[key] = value;
    }
  }
  return result;
}

for (const lang of languages) {
  const langDir = path.join(localesDir, lang);
  if (!fs.existsSync(langDir)) fs.mkdirSync(langDir, { recursive: true });
  for (const ns of namespaces) {
    const enFile = path.join(enDir, `${ns}.json`);
    const enContent = JSON.parse(fs.readFileSync(enFile, 'utf-8'));
    const targetFile = path.join(langDir, `${ns}.json`);
    if (!fs.existsSync(targetFile)) {
      fs.writeFileSync(targetFile, JSON.stringify(createStubs(enContent, lang), null, 2) + '\n');
      console.log(`Created: ${lang}/${ns}.json`);
    } else {
      console.log(`Exists: ${lang}/${ns}.json`);
    }
  }
}

console.log('Done!');
