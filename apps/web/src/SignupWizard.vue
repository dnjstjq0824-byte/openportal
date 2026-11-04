<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue';
import { APIError, mutate, request } from './client';
import { errors, messages, type Locale } from './i18n';
import { designMessages } from './design-copy';
import { signupErrors, signupMessages } from './signup-copy';
import { signupStatusMessages } from './signup-status-copy';

const props = defineProps<{ locale: Locale }>();
const emit = defineEmits<{ created: [address: string]; navigate: [url: string] }>();
const t = computed(() => messages[props.locale]);
const copy = computed(() => designMessages[props.locale]);
const s = computed(() => signupMessages[props.locale]);
const statusCopy = computed(() => signupStatusMessages[props.locale]);

type Step = 'name' | 'basic' | 'username' | 'password' | 'recovery' | 'review' | 'done';
const order: Step[] = ['name', 'basic', 'username', 'password', 'recovery', 'review'];
const step = ref<Step>('name');
const stepIndex = computed(() => order.indexOf(step.value));

const familyName = ref(''), givenName = ref('');
const year = ref(''), month = ref(''), day = ref('');
const gender = ref(''), genderCustom = ref(''), pronoun = ref('');
const username = ref(''), domain = ref('');
const password = ref(''), confirm = ref(''), showPassword = ref(false);
const recoveryEmail = ref(''), country = ref(props.locale === 'th' ? '+66' : props.locale === 'ko' ? '+82' : '+1'), phone = ref('');
const agree = ref(false);
const busy = ref(false), error = ref(''), unavailable = ref(false), created = ref(''), recoveryResult = ref('none');
const checkingService = ref(true);
const creationUncertain = ref(false);
const stepError = ref('');

const countries = [['+82', 'KR'], ['+66', 'TH'], ['+1', 'US/CA'], ['+81', 'JP'], ['+86', 'CN'], ['+44', 'UK'], ['+61', 'AU'], ['+65', 'SG'], ['+84', 'VN'], ['+49', 'DE'], ['+33', 'FR']];
const monthNames = computed(() => Array.from({ length: 12 }, (_, i) => new Intl.DateTimeFormat(props.locale, { month: 'long' }).format(new Date(Date.UTC(2000, i, 15)))));

const errorText = computed(() => {
  const code = error.value;
  if(code==='SIGNUP_STATUS_UNKNOWN')return statusCopy.value.unknownResult;
  return (signupErrors[props.locale] as Record<string, string>)[code] || errors[props.locale][code] || errors[props.locale].fallback;
});

// ----- Step: basic information -----
const birthDate = computed(() => {
  const y = Number(year.value), m = Number(month.value), d = Number(day.value);
  if (!Number.isInteger(y) || !Number.isInteger(m) || !Number.isInteger(d) || y < 1890 || m < 1 || m > 12 || d < 1) return null;
  const date = new Date(Date.UTC(y, m - 1, d));
  if (date.getUTCMonth() !== m - 1 || date.getUTCDate() !== d || date > new Date()) return null;
  return `${String(y).padStart(4, '0')}-${String(m).padStart(2, '0')}-${String(d).padStart(2, '0')}`;
});
function age(iso: string) {
  const [y, m, d] = iso.split('-').map(Number); const now = new Date();
  let value = now.getUTCFullYear() - y;
  if (now.getUTCMonth() + 1 < m || (now.getUTCMonth() + 1 === m && now.getUTCDate() < d)) value--;
  return value;
}

// ----- Step: username availability -----
type Availability = { state: 'idle' | 'checking' | 'available' | 'taken' | 'format' | 'reserved' | 'error'; suggestions: string[] };
const availability = ref<Availability>({ state: 'idle', suggestions: [] });
let timer: ReturnType<typeof setTimeout> | undefined; let controller: AbortController | undefined; let sequence = 0;
const normalizedUsername = computed(() => username.value.trim().toLowerCase());
function localProblem(name: string) {
  return /^[a-z0-9][a-z0-9.]{4,28}[a-z0-9]$/.test(name) && !name.includes('..') ? null : 'format';
}
async function check(name: string) {
  const own = ++sequence; controller?.abort(); controller = new AbortController();
  try {
    const data = await request(`/auth/username?${new URLSearchParams({ name })}`, { signal: controller.signal });
    if (own !== sequence) return;
    if (data.domain) domain.value = data.domain;
    unavailable.value = data.signupEnabled === false;
    if (unavailable.value) { availability.value = { state: 'error', suggestions: [] }; return; }
    if (!name) return; // initial call only discovers the mail domain
    availability.value = { state: data.available ? 'available' : (data.reason || 'taken'), suggestions: Array.isArray(data.suggestions) ? data.suggestions : [] };
  } catch (e) {
    if (own !== sequence || controller.signal.aborted) return;
    // No gateway (e.g. static Pages deploy) or no provisioner: say so before the user fills every step.
    if (e instanceof APIError && (e.code === 'MAIL_PROVISIONING_UNAVAILABLE' || e.status >= 500)) unavailable.value = true;
    if (!name) return;
    availability.value = { state: 'error', suggestions: [] };
  } finally { if (own === sequence) checkingService.value = false; }
}
function retryService() { checkingService.value = true; error.value = ''; check(normalizedUsername.value); }
watch(normalizedUsername, name => {
  clearTimeout(timer); ++sequence; controller?.abort();
  if (!name) { availability.value = { state: 'idle', suggestions: [] }; return; }
  if (localProblem(name)) { availability.value = { state: 'format', suggestions: [] }; return; }
  availability.value = { state: 'checking', suggestions: [] };
  timer = setTimeout(() => check(name), 350);
});
const usernameMessage = computed(() => ({ checking: s.value.checking, available: s.value.available, taken: s.value.taken, format: s.value.format, reserved: s.value.reserved, error: errors[props.locale].fallback, idle: '' })[availability.value.state]);

// ----- Step: password -----
const passwordScore = computed(() => {
  const value = password.value; if (!value) return -1; if (value.length < 12) return 0;
  const variety = [/[a-z]/, /[A-Z]/, /\d/, /[^A-Za-z0-9]/].filter(p => p.test(value)).length;
  return 1 + (variety >= 3 ? 1 : 0) + (variety >= 3 && value.length >= 16 ? 1 : 0);
});
const passwordVaried = computed(() => [/[A-Za-z]/, /\d/, /[^A-Za-z0-9]/].every(p => p.test(password.value)));
const strengthLabel = computed(() => [copy.value.strengthWeak, copy.value.strengthFair, copy.value.strengthGood, copy.value.strengthStrong][passwordScore.value] || '');

// ----- Navigation -----
const phoneE164 = computed(() => { const digits = phone.value.replace(/\D/g, '').replace(/^0+/, ''); return digits ? country.value + digits : ''; });
function validate(current: Step): string {
  if (current === 'name') return givenName.value.trim() ? '' : t.value.displayName;
  if (current === 'basic') {
    if (!birthDate.value) return s.value.invalidDate;
    if (age(birthDate.value) < 14) return s.value.underAge;
    if (!gender.value) return s.value.gender;
    if (gender.value === 'custom' && (!genderCustom.value.trim() || !pronoun.value)) return s.value.genderCustom;
    return '';
  }
  if (current === 'username') return availability.value.state === 'available' ? '' : (usernameMessage.value || s.value.format);
  if (current === 'password') return password.value.length < 12 ? t.value.passwordHint : new TextEncoder().encode(password.value).length > 1024 ? statusCopy.value.passwordTooLong : password.value !== confirm.value ? t.value.mismatch : '';
  if (current === 'recovery') {
    if (recoveryEmail.value && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(recoveryEmail.value.trim())) return t.value.email;
    if (domain.value && recoveryEmail.value.trim().toLowerCase().endsWith('@' + domain.value)) return (signupErrors[props.locale] as Record<string, string>).RECOVERY_EMAIL_SAME_DOMAIN;
    if (phone.value && !/^\+[1-9]\d{6,14}$/.test(phoneE164.value)) return (signupErrors[props.locale] as Record<string, string>).INVALID_PHONE;
  }
  return '';
}
const heading = ref<HTMLElement | null>(null);
function focusHeading() { requestAnimationFrame(() => heading.value?.focus()); }
function next() {
  if (unavailable.value || checkingService.value || busy.value) return;
  stepError.value = validate(step.value);
  if (stepError.value) return;
  step.value = order[stepIndex.value + 1]; showPassword.value = false; error.value = ''; focusHeading();
}
function back() { stepError.value = ''; error.value = ''; if (stepIndex.value > 0) { step.value = order[stepIndex.value - 1]; focusHeading(); } }
function goTo(target: Step) { stepError.value = ''; step.value = target; focusHeading(); }
const genderLabel = computed(() => gender.value === 'custom' ? `${genderCustom.value} · ${s.value.pronoun}: ${({ female: s.value.pronounFemale, male: s.value.pronounMale, other: s.value.pronounOther } as Record<string, string>)[pronoun.value] || ''}` : ({ female: s.value.female, male: s.value.male, unspecified: s.value.unspecified } as Record<string, string>)[gender.value] || '');
const birthdayLabel = computed(() => birthDate.value ? new Intl.DateTimeFormat(props.locale, { dateStyle: 'long', timeZone: 'UTC' }).format(new Date(birthDate.value + 'T00:00:00Z')) : '');
const fullName = computed(() => props.locale === 'ko' ? `${familyName.value.trim()}${givenName.value.trim()}` : `${givenName.value.trim()} ${familyName.value.trim()}`.trim());

async function submit() {
  if (!agree.value || busy.value || creationUncertain.value || unavailable.value) return;
  busy.value = true; error.value = '';
  try {
    const data = await mutate('/auth/signup', {
      givenName: givenName.value, familyName: familyName.value, birthDate: birthDate.value, gender: gender.value,
      genderCustom: gender.value === 'custom' ? genderCustom.value : undefined, pronoun: gender.value === 'custom' ? pronoun.value : undefined,
      username: normalizedUsername.value, password: password.value, recoveryEmail: recoveryEmail.value.trim() || undefined,
      recoveryPhone: phoneE164.value || undefined, locale: props.locale, agree: true,
    });
    if(data.created!==true || data.email!==`${normalizedUsername.value}@${domain.value}`){creationUncertain.value=true;throw new APIError('SIGNUP_STATUS_UNKNOWN',502);}
    created.value = data.email; recoveryResult.value = data.recoveryEmail || 'none';
    password.value = ''; confirm.value = '';
    step.value = 'done'; focusHeading(); emit('created', data.email);
  } catch (e) {
    error.value = e instanceof APIError ? e.code : 'fallback';
    const target: Record<string, Step> = { USERNAME_TAKEN: 'username', USERNAME_RESERVED: 'username', INVALID_USERNAME: 'username', AGE_REQUIREMENT: 'basic', RECOVERY_EMAIL_SAME_DOMAIN: 'recovery', INVALID_PHONE: 'recovery', PASSWORD_POLICY: 'password' };
    if (target[error.value]) { step.value = target[error.value]; if (target[error.value] === 'username') check(normalizedUsername.value); }
    if (error.value === 'MAIL_PROVISIONING_UNAVAILABLE' || error.value === 'GATEWAY_UNAVAILABLE') unavailable.value = true;
  } finally { busy.value = false; }
}
function link(event: MouseEvent, url: string) {
  if (event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) return;
  event.preventDefault(); emit('navigate', url);
}
onMounted(() => { check(''); });
onUnmounted(() => { clearTimeout(timer); controller?.abort(); });
</script>

<template>
  <div class="wizard">
    <div v-if="step!=='done'" class="wizard-progress">
      <span class="wizard-count">{{s.steps.replace('{n}', String(stepIndex+1)).replace('{total}', String(order.length))}}</span>
      <span class="wizard-bar" :data-step="stepIndex+1" role="progressbar" :aria-label="s.steps.replace('{n}', String(stepIndex+1)).replace('{total}', String(order.length))" :aria-valuenow="stepIndex+1" :aria-valuemin="1" :aria-valuemax="order.length"><i></i></span>
    </div>
    <div v-if="unavailable" class="alert" role="alert"><p>{{s.unavailable}}</p><button type="button" class="button" :disabled="checkingService" @click="retryService">{{checkingService?t.loading:t.retry}}</button></div>
    <p v-else-if="domain.endsWith('.test')" class="notice" role="note">{{statusCopy.developmentService}}</p>

    <form v-if="step==='name'" class="auth-form" @submit.prevent="next">
      <header class="auth-header"><h1 ref="heading" tabindex="-1">{{s.nameTitle}}</h1><p>{{s.nameIntro}}</p></header>
      <div class="form-row">
        <div class="form-group"><label for="su-family">{{s.familyName}}</label><input id="su-family" v-model="familyName" autocomplete="family-name" maxlength="40"/></div>
        <div class="form-group"><label for="su-given">{{s.givenName}}</label><input id="su-given" v-model="givenName" autocomplete="given-name" maxlength="40" required/></div>
      </div>
      <p v-if="stepError" class="field-error" role="alert">{{stepError}}</p>
      <div class="wizard-actions"><span></span><button class="button primary" type="submit" :disabled="unavailable||checkingService">{{checkingService?t.loading:s.next}}</button></div>
    </form>

    <form v-else-if="step==='basic'" class="auth-form" @submit.prevent="next">
      <header class="auth-header"><h1 ref="heading" tabindex="-1">{{s.basicTitle}}</h1><p>{{s.basicIntro}}</p></header>
      <fieldset class="form-fieldset"><legend>{{s.birthday}}</legend>
        <div class="date-row">
          <div class="form-group"><label for="su-year">{{s.year}}</label><input id="su-year" v-model="year" inputmode="numeric" autocomplete="bday-year" maxlength="4" placeholder="1990" required/></div>
          <div class="form-group"><label for="su-month">{{s.month}}</label><select id="su-month" v-model="month" autocomplete="bday-month" required><option value="" disabled>{{s.month}}</option><option v-for="(name,index) in monthNames" :key="index" :value="String(index+1)">{{name}}</option></select></div>
          <div class="form-group"><label for="su-day">{{s.day}}</label><input id="su-day" v-model="day" inputmode="numeric" autocomplete="bday-day" maxlength="2" placeholder="1" required/></div>
        </div>
        <small class="hint">{{s.ageHint}} {{statusCopy.calendarHint}}</small>
      </fieldset>
      <div class="form-group"><label for="su-gender">{{s.gender}}</label><select id="su-gender" v-model="gender" required><option value="" disabled>{{s.genderSelect}}</option><option value="female">{{s.female}}</option><option value="male">{{s.male}}</option><option value="unspecified">{{s.unspecified}}</option><option value="custom">{{s.custom}}</option></select></div>
      <template v-if="gender==='custom'">
        <div class="form-group"><label for="su-gender-custom">{{s.genderCustom}}</label><input id="su-gender-custom" v-model="genderCustom" maxlength="40" required/></div>
        <div class="form-group"><label for="su-pronoun">{{s.pronoun}}</label><select id="su-pronoun" v-model="pronoun" required><option value="" disabled>{{s.genderSelect}}</option><option value="female">{{s.pronounFemale}}</option><option value="male">{{s.pronounMale}}</option><option value="other">{{s.pronounOther}}</option></select></div>
      </template>
      <p v-if="stepError" class="field-error" role="alert">{{stepError}}</p>
      <div class="wizard-actions"><button type="button" class="button" @click="back">{{s.back}}</button><button class="button primary" type="submit">{{s.next}}</button></div>
    </form>

    <form v-else-if="step==='username'" class="auth-form" @submit.prevent="next">
      <header class="auth-header"><h1 ref="heading" tabindex="-1">{{s.usernameTitle}}</h1><p>{{s.usernameIntro}}</p></header>
      <div class="form-group">
        <label for="su-username">{{s.username}}</label>
        <div class="address-field" :class="availability.state"><input id="su-username" v-model="username" autocomplete="username" autocapitalize="none" spellcheck="false" maxlength="30" aria-describedby="su-username-status su-username-rule" required/><span class="address-domain">@{{domain||'…'}}</span></div>
        <small id="su-username-status" class="availability" :class="availability.state" aria-live="polite">{{availability.state==='available'?'✓ ':''}}{{usernameMessage}}</small>
        <div v-if="availability.suggestions.length" class="suggestion-chips"><span>{{s.suggestions}}</span><button v-for="name in availability.suggestions" :key="name" type="button" class="chip" @click="username=name">{{name}}</button></div>
        <small id="su-username-rule" class="hint">{{s.usernameRule}}</small>
      </div>
      <p v-if="error" class="alert" role="alert">{{errorText}}</p>
      <p v-else-if="stepError&&availability.state!=='checking'" class="field-error" role="alert">{{stepError}}</p>
      <div class="wizard-actions"><button type="button" class="button" @click="back">{{s.back}}</button><button class="button primary" type="submit" :disabled="availability.state!=='available'">{{s.next}}</button></div>
    </form>

    <form v-else-if="step==='password'" class="auth-form" @submit.prevent="next">
      <header class="auth-header"><h1 ref="heading" tabindex="-1">{{s.passwordTitle}}</h1><p>{{s.passwordIntro}}</p></header>
      <input type="text" class="sr-only" :value="`${normalizedUsername}@${domain}`" autocomplete="username" tabindex="-1" aria-hidden="true" readonly/>
      <div class="form-group">
        <label for="su-password">{{t.password}}</label>
        <div class="password-field">
          <input id="su-password" v-model="password" :type="showPassword?'text':'password'" autocomplete="new-password" minlength="12" maxlength="1024" aria-describedby="su-password-help" required/>
          <button type="button" class="icon-button" :aria-label="showPassword?copy.hidePassword:copy.showPassword" :aria-pressed="showPassword" @click="showPassword=!showPassword"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" aria-hidden="true"><path d="M2.5 12S6 5.5 12 5.5 21.5 12 21.5 12 18 18.5 12 18.5 2.5 12 2.5 12Z"/><circle cx="12" cy="12" r="3"/><path v-if="showPassword" d="m4 4 16 16"/></svg></button>
        </div>
        <div id="su-password-help" class="password-help">
          <div v-if="password" class="strength" :data-score="passwordScore"><span class="strength-bar" aria-hidden="true"><i v-for="n in 3" :key="n" :class="{on:passwordScore>=n}"></i></span><span>{{copy.strength}}: <strong>{{strengthLabel}}</strong></span></div>
          <ul class="requirements"><li :class="{met:password.length>=12}"><span aria-hidden="true">{{password.length>=12?'✓':'•'}}</span>{{copy.reqLength}}</li><li :class="{met:passwordVaried}"><span aria-hidden="true">{{passwordVaried?'✓':'•'}}</span>{{copy.reqVariety}}</li></ul>
        </div>
      </div>
      <div class="form-group"><label for="su-confirm">{{t.confirmPassword}}</label><input id="su-confirm" v-model="confirm" :type="showPassword?'text':'password'" autocomplete="new-password" maxlength="1024" required/><small class="match-hint" :class="!confirm?'':confirm===password?'match':'differ'" aria-live="polite">{{!confirm?'':confirm===password?'✓ '+copy.passwordsMatch:copy.passwordsDiffer}}</small></div>
      <p v-if="error" class="alert" role="alert">{{errorText}}</p>
      <p v-else-if="stepError" class="field-error" role="alert">{{stepError}}</p>
      <div class="wizard-actions"><button type="button" class="button" @click="back">{{s.back}}</button><button class="button primary" type="submit">{{s.next}}</button></div>
    </form>

    <form v-else-if="step==='recovery'" class="auth-form" @submit.prevent="next">
      <header class="auth-header"><h1 ref="heading" tabindex="-1">{{s.recoveryTitle}}</h1><p>{{s.recoveryIntro}}</p></header>
      <div class="form-group"><label for="su-recovery">{{s.recoveryEmail}}</label><input id="su-recovery" v-model="recoveryEmail" type="email" inputmode="email" autocomplete="email" maxlength="254"/><small class="hint">{{s.recoveryEmailHint}}</small></div>
      <div class="form-group"><label for="su-phone">{{s.phone}}</label>
        <div class="phone-row"><select v-model="country" :aria-label="s.country"><option v-for="[code,name] in countries" :key="code+name" :value="code">{{name}} {{code}}</option></select><input id="su-phone" v-model="phone" type="tel" inputmode="tel" autocomplete="tel-national" maxlength="20"/></div>
        <small class="hint">{{s.phoneHint}}</small>
      </div>
      <p v-if="!recoveryEmail" class="notice">{{s.recoveryWarning}}</p>
      <p v-if="error" class="alert" role="alert">{{errorText}}</p>
      <p v-else-if="stepError" class="field-error" role="alert">{{stepError}}</p>
      <div class="wizard-actions"><button type="button" class="button" @click="back">{{s.back}}</button><button class="button primary" type="submit">{{s.next}}</button></div>
    </form>

    <form v-else-if="step==='review'" class="auth-form" @submit.prevent="submit">
      <header class="auth-header"><h1 ref="heading" tabindex="-1">{{s.reviewTitle}}</h1><p>{{s.reviewIntro}}</p></header>
      <dl class="review-list">
        <div><dt>{{s.address}}</dt><dd><strong>{{normalizedUsername}}@{{domain}}</strong></dd><button type="button" class="text-button" @click="goTo('username')">{{s.edit}}</button></div>
        <div><dt>{{s.name}}</dt><dd>{{fullName}}</dd><button type="button" class="text-button" @click="goTo('name')">{{s.edit}}</button></div>
        <div><dt>{{s.birthday}}</dt><dd>{{birthdayLabel}}</dd><button type="button" class="text-button" @click="goTo('basic')">{{s.edit}}</button></div>
        <div><dt>{{s.gender}}</dt><dd>{{genderLabel}}</dd><button type="button" class="text-button" @click="goTo('basic')">{{s.edit}}</button></div>
        <div><dt>{{s.recoveryStatus}}</dt><dd>{{recoveryEmail.trim()||s.none}}</dd><button type="button" class="text-button" @click="goTo('recovery')">{{s.edit}}</button></div>
        <div><dt>{{s.phone}}</dt><dd>{{phoneE164||s.none}}</dd><button type="button" class="text-button" @click="goTo('recovery')">{{s.edit}}</button></div>
      </dl>
      <label class="checkbox-row"><input v-model="agree" type="checkbox" required/><span>{{t.agree}} <a :href="`/${locale}/terms`" target="_blank" rel="noopener">{{t.terms}}</a> · <a :href="`/${locale}/privacy`" target="_blank" rel="noopener">{{t.privacy}}</a></span></label>
      <p v-if="error" class="alert" role="alert">{{errorText}}</p>
      <div class="wizard-actions"><button type="button" class="button" :disabled="busy||creationUncertain" @click="back">{{s.back}}</button><button class="button primary" type="submit" :disabled="busy||!agree||unavailable||creationUncertain">{{busy?s.creating:s.create}}</button></div>
      <a v-if="creationUncertain" class="button" :href="`/${locale}/login`" @click="link($event,`/${locale}/login`)">{{t.login}}</a>
    </form>

    <div v-else class="auth-sent">
      <span class="auth-sent-icon" aria-hidden="true"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linejoin="round"><rect x="3" y="5" width="18" height="14" rx="3"/><path d="m4 7 8 6 8-6"/></svg></span>
      <h1 ref="heading" tabindex="-1">{{s.doneTitle}}</h1>
      <p role="status">{{s.doneIntro}}</p>
      <p class="auth-sent-to"><span>{{s.mailAddress}}</span><strong>{{created}}</strong></p>
      <p v-if="recoveryResult==='sent'" class="source-note">{{s.doneRecoverySent}}</p>
      <p v-else-if="recoveryResult==='not_sent'" class="source-note">{{s.doneRecoveryNotSent}}</p>
      <a class="button primary full-width" :href="`/${locale}/mail`" @click="link($event,`/${locale}/mail`)">{{s.openMail}}</a>
      <a class="text-button" :href="`/${locale}`" @click="link($event,`/${locale}`)">{{s.goHome}}</a>
    </div>
  </div>
</template>
