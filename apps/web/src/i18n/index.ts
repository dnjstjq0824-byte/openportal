import { createI18n } from 'vue-i18n';
import { computed } from 'vue';
import { errors, folderLabels, locales, messages, type Locale } from '../i18n';
import { designMessages } from '../design-copy';
import { serpMessages } from '../serp-copy';
import { foundationMessages } from '../foundation-copy';
import { signupErrors, signupMessages } from '../signup-copy';

export type { Locale };
export { locales };

// Merge the existing per-module dictionaries into one vue-i18n catalogue.
// Core keys stay flat (t('search')); module dictionaries live under namespaces.
function buildMessages(locale: Locale) {
  return {
    ...messages[locale],
    errors: errors[locale],
    folders: folderLabels[locale],
    design: designMessages[locale],
    serp: serpMessages[locale],
    foundation: foundationMessages[locale],
    signup: signupMessages[locale],
    signupErrors: signupErrors[locale],
  };
}

export const i18n = createI18n({
  legacy: false,
  locale: 'en',
  fallbackLocale: 'en',
  messages: {
    ko: buildMessages('ko'),
    th: buildMessages('th'),
    en: buildMessages('en'),
  },
});

export const portalLocale = computed(() => i18n.global.locale.value as Locale);

export function setPortalLocale(locale: Locale): void {
  if (!locales.includes(locale)) return;
  i18n.global.locale.value = locale;
  document.documentElement.lang = locale;
}

// Typed record accessors for migrated views (same shape as the old computeds).
export const coreMessages = computed(() => messages[portalLocale.value]);
export const coreErrors = computed(() => errors[portalLocale.value]);
export const designCopy = computed(() => designMessages[portalLocale.value]);
export const serpCopy = computed(() => serpMessages[portalLocale.value]);
export const foundationCopy = computed(() => foundationMessages[portalLocale.value]);
export const signupCopy = computed(() => signupMessages[portalLocale.value]);
export const signupErrorCopy = computed(() => signupErrors[portalLocale.value]);
export const folderLabelCopy = computed(() => folderLabels[portalLocale.value]);

export function detectLocale(saved: string | null, accountLocale: unknown, browser: readonly string[], country: string): Locale {
  if (saved && locales.includes(saved as Locale)) return saved as Locale;
  if (typeof accountLocale === 'string' && locales.includes(accountLocale as Locale)) return accountLocale as Locale;
  const match = browser.map(x => x.split('-')[0]).find(x => locales.includes(x as Locale));
  if (match) return match as Locale;
  if (country === 'TH') return 'th';
  if (country === 'KR') return 'ko';
  return 'en';
}

// Keep document metadata in sync with the active route (SEO: title, robots,
// canonical + hreflang on home only).
export function syncMetadata(locale: Locale, page: string, title: string, intro: string): void {
  const fullTitle = `${title || messages[locale].tagline} · OpenScanner`;
  document.documentElement.lang = locale;
  document.title = fullTitle;
  document.querySelector('meta[name="description"]')?.setAttribute('content', intro);
  document.querySelector('meta[name="robots"]')?.setAttribute('content', page === '' ? 'index,follow' : 'noindex,nofollow');
  document.querySelector('meta[property="og:title"]')?.setAttribute('content', fullTitle);
  document.querySelector('meta[property="og:description"]')?.setAttribute('content', intro);
  document.querySelectorAll('link[rel="canonical"],link[hreflang]').forEach(x => x.remove());
  document.querySelectorAll('script[data-portal-jsonld]').forEach(x => x.remove());
  document.querySelector('meta[property="og:url"]')?.remove();
  if (page === '') {
    const canonical = document.createElement('link');
    canonical.rel = 'canonical';
    canonical.href = `${location.origin}/${locale}`;
    document.head.append(canonical);
    const ogUrl = document.createElement('meta');
    ogUrl.setAttribute('property', 'og:url');
    ogUrl.setAttribute('content', `${location.origin}/${locale}`);
    document.head.append(ogUrl);
    for (const language of locales) {
      const alt = document.createElement('link');
      alt.rel = 'alternate';
      alt.hreflang = language;
      alt.href = `${location.origin}/${language}`;
      document.head.append(alt);
    }
    const jsonLd = document.createElement('script');
    jsonLd.type = 'application/ld+json';
    jsonLd.dataset.portalJsonld = 'website';
    jsonLd.textContent = JSON.stringify({
      '@context': 'https://schema.org',
      '@type': 'WebSite',
      name: 'OpenScanner',
      url: `${location.origin}/${locale}`,
      inLanguage: locale,
      potentialAction: {
        '@type': 'SearchAction',
        target: `${location.origin}/${locale}/search?q={query}`,
        'query-input': 'required name=query',
      },
    });
    document.head.append(jsonLd);
  }
}
