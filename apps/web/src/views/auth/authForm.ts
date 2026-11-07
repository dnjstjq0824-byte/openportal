import { computed, ref } from 'vue';
import { useRoute } from 'vue-router';
import { coreErrors, coreMessages, designCopy, signupCopy, signupErrorCopy } from '../../i18n/index';
import { useSessionStore } from '../../stores/session';

export function useAuthForm() {
  const session = useSessionStore();
  const route = useRoute();
  const page = computed(() => String(route.name ?? ''));

  const email = ref('');
  const password = ref('');
  const confirm = ref('');
  const showPassword = ref(false);
  const busy = ref(false);
  const error = ref('');
  const errorStatus = ref(0);
  const notice = ref('');

  // Advisory only: the gateway enforces the real policy (12+ characters, <= 1024 bytes).
  const passwordScore = computed(() => {
    const value = password.value;
    if (!value) return -1;
    if (value.length < 12) return 0;
    const variety = [/[a-z]/, /[A-Z]/, /\d/, /[^A-Za-z0-9]/].filter(pattern => pattern.test(value)).length;
    return 1 + (variety >= 3 ? 1 : 0) + (variety >= 3 && value.length >= 16 ? 1 : 0);
  });
  const passwordVaried = computed(() => [/[A-Za-z]/, /\d/, /[^A-Za-z0-9]/].every(pattern => pattern.test(password.value)));
  const strengthLabel = computed(
    () => [designCopy.value.strengthWeak, designCopy.value.strengthFair, designCopy.value.strengthGood, designCopy.value.strengthStrong][passwordScore.value] || ''
  );
  const confirmState = computed(() => (!confirm.value ? '' : confirm.value === password.value ? 'match' : 'differ'));

  const needsNewPassword = computed(() => page.value === 'signup' || page.value === 'reset-password');
  const titles = computed(
    () =>
      ({
        login: coreMessages.value.login,
        signup: coreMessages.value.signup,
        'forgot-password': coreMessages.value.reset,
        'reset-password': coreMessages.value.reset,
        'verify-email': coreMessages.value.verify,
      }) as Record<string, string>
  );
  const authTitle = computed(() =>
    page.value === 'login' ? designCopy.value.loginTitle : page.value === 'signup' ? designCopy.value.signupTitle : titles.value[page.value]
  );
  const authIntro = computed(() =>
    page.value === 'login'
      ? signupCopy.value.loginIntro
      : page.value === 'signup'
        ? designCopy.value.signupIntro
        : page.value === 'forgot-password'
          ? signupCopy.value.forgotIntro
          : page.value === 'verify-email'
            ? designCopy.value.verifyIntro
            : page.value === 'reset-password'
              ? designCopy.value.resetIntro
              : designCopy.value.authIntro
  );
  const authBenefits = computed(() => [designCopy.value.benefitMail, designCopy.value.benefitVerify, designCopy.value.benefitHash, designCopy.value.benefitHistory]);
  const serverDown = computed(() => page.value !== 'signup' && session.ready && (session.sessionUnavailable || errorStatus.value >= 500));
  const errorText = computed(
    () => (signupErrorCopy.value as Record<string, string>)[error.value] || coreErrors.value[error.value] || coreErrors.value.fallback
  );

  function resetSecrets(): void {
    password.value = '';
    confirm.value = '';
  }

  return {
    session, page, email, password, confirm, showPassword, busy, error, errorStatus, notice,
    passwordScore, passwordVaried, strengthLabel, confirmState, needsNewPassword,
    titles, authTitle, authIntro, authBenefits, serverDown, errorText, resetSecrets,
  };
}
