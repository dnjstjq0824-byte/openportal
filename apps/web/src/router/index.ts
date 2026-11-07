import { createRouter, createWebHistory, type NavigationGuardNext, type RouteLocationNormalized, type RouteLocationRaw } from 'vue-router';
import { ref } from 'vue';
import { locales, messages, type Locale } from '../i18n';
import { detectLocale, setPortalLocale, syncMetadata } from '../i18n/index';
import { useSessionStore } from '../stores/session';
import { stored } from '../utils/storage';
import HomeView from '../views/HomeView.vue';
import SearchView from '../views/SearchView.vue';
import LoginView from '../views/auth/LoginView.vue';
import SignupView from '../views/auth/SignupView.vue';
import ForgotPasswordView from '../views/auth/ForgotPasswordView.vue';
import ResetPasswordView from '../views/auth/ResetPasswordView.vue';
import VerifyEmailView from '../views/auth/VerifyEmailView.vue';
import MailView from '../views/MailView.vue';
import AccountView from '../views/AccountView.vue';
import AdminView from '../views/AdminView.vue';
import SettingsView from '../views/SettingsView.vue';
import PolicyView from '../views/PolicyView.vue';
import NotFoundView from '../views/NotFoundView.vue';

export const AUTH_PAGES = ['login', 'signup', 'forgot-password', 'reset-password', 'verify-email'] as const;
export const POLICY_PAGES = ['about', 'store', 'advertising', 'business', 'howsearchworks', 'privacy', 'terms'] as const;
export const GUARDED_PAGES = ['mail', 'account', 'admin'];

export type PolicyPage = (typeof POLICY_PAGES)[number];

const localePattern = locales.join('|');

function detectInitialLocale(accountLocale: unknown): Locale {
  const country = (document.querySelector('meta[name="country"]')?.getAttribute('content') || '').toUpperCase();
  return detectLocale(stored('openscanner.locale', ''), accountLocale, navigator.languages, country);
}

// Mail draft guard: leaving /mail with unsaved changes needs confirmation.
export interface PendingNavigation {
  to?: RouteLocationRaw;
  action?: () => void;
}
export const pendingRoute = ref<PendingNavigation | null>(null);

export function confirmPendingRoute(): void {
  const pending = pendingRoute.value;
  pendingRoute.value = null;
  const session = useSessionStore();
  session.mailDraftDirty = false;
  if (pending?.action) pending.action();
  else if (pending?.to) void router.push(pending.to);
}

export function cancelPendingRoute(): void {
  pendingRoute.value = null;
}

export function safeReturn(value: string | null, locale: Locale): string {
  if (!value || /[\\\r\n]/.test(value)) return `/${locale}`;
  try {
    const url = new URL(value, location.origin);
    return url.origin === location.origin ? url.pathname + url.search : `/${locale}`;
  } catch {
    return `/${locale}`;
  }
}

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: () => ({ path: `/${detectInitialLocale(null)}` }) },
    { path: '/register', redirect: () => ({ path: `/${detectInitialLocale(null)}/signup` }) },
    { path: `/:locale(${localePattern})`, name: 'home', component: HomeView },
    { path: `/:locale(${localePattern})/search`, name: 'search', component: SearchView },
    { path: `/:locale(${localePattern})/login`, name: 'login', component: LoginView },
    { path: `/:locale(${localePattern})/signup`, name: 'signup', component: SignupView },
    { path: `/:locale(${localePattern})/forgot-password`, name: 'forgot-password', component: ForgotPasswordView },
    { path: `/:locale(${localePattern})/reset-password`, name: 'reset-password', component: ResetPasswordView },
    { path: `/:locale(${localePattern})/verify-email`, name: 'verify-email', component: VerifyEmailView },
    { path: `/:locale(${localePattern})/mail`, name: 'mail', component: MailView, meta: { requiresAuth: true } },
    { path: `/:locale(${localePattern})/account`, name: 'account', component: AccountView, meta: { requiresAuth: true } },
    { path: `/:locale(${localePattern})/admin`, name: 'admin', component: AdminView, meta: { requiresAuth: true } },
    { path: `/:locale(${localePattern})/settings`, name: 'settings', component: SettingsView },
    { path: `/:locale(${localePattern})/register`, redirect: to => ({ path: `/${to.params.locale}/signup`, query: to.query }) },
    { path: `/:locale(${localePattern})/:page(${POLICY_PAGES.join('|')})`, name: 'policy', component: PolicyView },
    { path: `/:locale(${localePattern})/:pathMatch(.*)*`, name: 'not-found', component: NotFoundView },
    // No locale prefix: detect and redirect, preserving the path.
    {
      path: '/:pathMatch(.*)*',
      beforeEnter: (to, _from, next: NavigationGuardNext) => {
        const session = useSessionStore();
        const locale = session.ready ? detectInitialLocale(session.user?.locale) : detectInitialLocale(null);
        next({ path: `/${locale}${to.path === '/' ? '' : to.path}`, query: to.query, hash: to.hash, replace: true });
      },
      component: NotFoundView,
    },
  ],
  scrollBehavior() {
    return { top: 0 };
  },
});

async function guard(to: RouteLocationNormalized, from: RouteLocationNormalized, next: NavigationGuardNext): Promise<void> {
  const session = useSessionStore();
  if (!session.ready) {
    try {
      await session.loadSession();
    } catch {
      /* session stays unavailable; public pages still render */
    }
  }
  // First navigation may need account-locale detection.
  if (from.name === undefined && to.params.locale && !session.user) {
    // Keep the URL locale: the catch-all already handled locale-less paths.
  }
  if (session.mailDraftDirty && from.name === 'mail' && to.name !== 'mail') {
    pendingRoute.value = { to: { path: to.path, query: to.query, hash: to.hash } };
    next(false);
    return;
  }
  if (to.meta.requiresAuth && !session.user) {
    next({ path: `/${to.params.locale}/login`, query: { returnTo: to.path + (to.query && Object.keys(to.query).length ? `?${new URLSearchParams(to.query as Record<string, string>)}` : '') }, replace: true });
    return;
  }
  next();
}

router.beforeEach(guard);

router.afterEach(to => {
  const locale = (to.params.locale as Locale) || detectInitialLocale(null);
  const session = useSessionStore();
  if (locales.includes(locale)) {
    session.locale = locale;
    setPortalLocale(locale);
    try {
      localStorage.setItem('openscanner.locale', locale);
    } catch {
      /* ignore */
    }
  }
  const name = String(to.name ?? '');
  const page = name === 'home' ? '' : name === 'policy' ? String(to.params.page ?? '') : name;
  const titles: Record<string, string> = {
    login: messages[locale].login,
    signup: messages[locale].signup,
    'forgot-password': messages[locale].reset,
    'reset-password': messages[locale].reset,
    'verify-email': messages[locale].verify,
    account: messages[locale].account,
    mail: messages[locale].mail,
    privacy: messages[locale].privacy,
    terms: messages[locale].terms,
    about: messages[locale].about,
    store: messages[locale].store,
    advertising: messages[locale].advertising,
    business: messages[locale].business,
    howsearchworks: messages[locale].howSearchWorks,
    settings: messages[locale].settings,
    admin: messages[locale].admin,
  };
  syncMetadata(locale, page, name === 'policy' ? titles[page] || '' : titles[name] || '', messages[locale].intro);
});

export default router;
