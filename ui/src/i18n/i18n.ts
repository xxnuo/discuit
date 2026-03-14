import i18n from 'i18next';
import HttpBackend from 'i18next-http-backend';
import LanguageDetector from 'i18next-browser-languagedetector';
import { initReactI18next } from 'react-i18next';

export const supportedLanguages = [
  { code: 'en', name: 'English', dir: 'ltr' },
  { code: 'zh-CN', name: '简体中文', dir: 'ltr' },
  { code: 'zh-TW', name: '繁體中文', dir: 'ltr' },
  { code: 'ja', name: '日本語', dir: 'ltr' },
  { code: 'ko', name: '한국어', dir: 'ltr' },
  { code: 'es', name: 'Español', dir: 'ltr' },
  { code: 'fr', name: 'Français', dir: 'ltr' },
  { code: 'de', name: 'Deutsch', dir: 'ltr' },
  { code: 'pt', name: 'Português', dir: 'ltr' },
  { code: 'ru', name: 'Русский', dir: 'ltr' },
  { code: 'ar', name: 'العربية', dir: 'rtl' },
  { code: 'hi', name: 'हिन्दी', dir: 'ltr' },
] as const;

export const namespaces = [
  'common',
  'auth',
  'post',
  'comment',
  'community',
  'modtools',
  'admin',
  'settings',
  'user',
  'about',
] as const;

export type SupportedLanguage = (typeof supportedLanguages)[number]['code'];

export function getLanguageDir(lng: string): 'ltr' | 'rtl' {
  const lang = supportedLanguages.find((l) => l.code === lng);
  return lang?.dir ?? 'ltr';
}

export function updateDocumentDir(lng: string) {
  if (typeof document === 'undefined') return;
  const dir = getLanguageDir(lng);
  document.documentElement.dir = dir;
  document.documentElement.lang = lng;
}

i18n
  .use(HttpBackend)
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    ns: namespaces as unknown as string[],
    defaultNS: 'common',
    supportedLngs: supportedLanguages.map((l) => l.code),
    fallbackLng: {
      'zh-Hans': ['zh-CN', 'en'],
      'zh-Hant': ['zh-TW', 'en'],
      'zh': ['zh-CN', 'en'],
      default: ['en'],
    },
    interpolation: {
      escapeValue: false,
    },
    detection: {
      order: ['localStorage', 'navigator'],
      lookupLocalStorage: 'i18nextLng',
      caches: ['localStorage'],
    },
    backend: {
      loadPath: '/locales/{{lng}}/{{ns}}.json',
    },
  });

i18n.on('languageChanged', updateDocumentDir);

export default i18n;
