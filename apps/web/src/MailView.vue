<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue';
import { APIError, mutate, request } from './client';
import { errors, messages, folderLabels, type Locale } from './i18n';

const props = defineProps<{ locale: Locale; user: any }>();
const emit = defineEmits<{ expired: [] }>();

const t = computed(() => messages[props.locale]);
const labels = computed(() => folderLabels[props.locale]);

const folders = ref<any[]>([]);
const folder = ref('');
const emails = ref<any[]>([]);
const selected = ref<any>(null);
const busy = ref(false);
const error = ref('');
const notice = ref('');
const query = ref('');
const position = ref(0);
const total = ref(0);
const composing = ref(false);
const html = ref(false);
const folderInput = ref('');

const draft = ref<any>({ to: '', cc: '', subject: '', text: '', attachments: [] });
let generation = 0;
let timer: ReturnType<typeof setInterval> | undefined;

const errorText = computed(() => errors[props.locale][error.value] || errors[props.locale].fallback);

function problem(e: unknown) {
  if (e instanceof APIError && e.status === 401) {
    emit('expired');
    return;
  }
  error.value = e instanceof APIError ? e.code : 'fallback';
}

function folderName(box: any) {
  return (t.value as any)[box.role] || box.name;
}

async function load() {
  const own = ++generation;
  busy.value = true;
  error.value = '';
  try {
    const data = await request(`/mail/messages?${new URLSearchParams({ mailbox: folder.value, q: query.value, position: String(position.value) })}`);
    if (own === generation) {
      emails.value = data.messages;
      total.value = data.total || 0;
    }
  } catch (e) {
    problem(e);
  } finally {
    if (own === generation) busy.value = false;
  }
}

async function connect() {
  busy.value = true;
  error.value = '';
  try {
    const data = await request('/mail/mailboxes');
    folders.value = data.mailboxes;
    folder.value = folders.value.find(x => x.role === 'inbox')?.id || folders.value[0]?.id || '';
    if (folder.value) await load();
  } catch (e) {
    problem(e);
  } finally {
    busy.value = false;
  }
}

function switchFolder(id: string) {
  folder.value = id;
  position.value = 0;
  selected.value = null;
  composing.value = false;
  query.value = '';
  load();
}

async function read(id: string) {
  busy.value = true;
  error.value = '';
  try {
    selected.value = await request(`/mail/messages/${encodeURIComponent(id)}`);
    html.value = false;
    composing.value = false;
    await mutate(`/mail/messages/${encodeURIComponent(id)}`, { seen: true }, 'PATCH');
    selected.value.keywords.$seen = true;
    const item = emails.value.find(x => x.id === id);
    if (item) item.keywords.$seen = true;
  } catch (e) {
    problem(e);
  } finally {
    busy.value = false;
  }
}

async function update(change: any) {
  busy.value = true;
  error.value = '';
  try {
    await mutate(`/mail/messages/${encodeURIComponent(selected.value.id)}`, change, 'PATCH');
    if (change.mailbox) {
      selected.value = null;
      await load();
    } else {
      if (change.seen !== undefined) selected.value.keywords.$seen = change.seen;
      if (change.flagged !== undefined) selected.value.keywords.$flagged = change.flagged;
      await load();
    }
  } catch (e) {
    problem(e);
  } finally {
    busy.value = false;
  }
}

function compose(kind = 'new') {
  const previous = selected.value;
  draft.value = { to: '', cc: '', subject: '', text: '', attachments: [] };
  notice.value = '';
  if (previous && kind === 'draft') {
    draft.value = {
      id: previous.id,
      to: previous.to?.map((x: any) => x.email).join(', ') || '',
      cc: previous.cc?.map((x: any) => x.email).join(', ') || '',
      subject: previous.subject,
      text: previous.text,
      attachments: previous.attachments || [],
    };
  } else if (previous && kind !== 'new') {
    if (kind !== 'forward') draft.value.replyToId = previous.id;
    const from = previous.from?.map((x: any) => x.email) || [];
    draft.value.to = kind === 'forward' ? '' : from.filter((x: string) => x !== props.user.email).join(', ');
    if (kind === 'replyAll') {
      draft.value.cc = [...(previous.to || []), ...(previous.cc || [])]
        .map((x: any) => x.email)
        .filter((x: string) => x !== props.user.email && !from.includes(x))
        .join(', ');
    }
    draft.value.subject = `${kind === 'forward' ? 'Fwd:' : 'Re:'} ${previous.subject || ''}`;
    draft.value.text = `\n\n> ${(previous.text || '').replaceAll('\n', '\n> ')}`;
  }
  composing.value = true;
}

async function save(send = false) {
  busy.value = true;
  error.value = '';
  notice.value = '';
  try {
    const saved = await mutate('/mail/drafts', draft.value);
    draft.value.id = saved.id;
    if (send) {
      await mutate('/mail/send', { id: saved.id });
      notice.value = t.value.sendAccepted;
      composing.value = false;
    } else {
      notice.value = t.value.saved;
    }
    await load();
  } catch (e) {
    problem(e);
  } finally {
    busy.value = false;
  }
}

async function attach(event: Event) {
  const input = event.target as HTMLInputElement;
  const files = Array.from(input.files || []);
  busy.value = true;
  error.value = '';
  try {
    if (files.length + draft.value.attachments.length > 5) throw new APIError('ATTACHMENT_REJECTED', 422);
    for (const file of files) {
      if (file.size > 5 * 1024 * 1024) throw new APIError('ATTACHMENT_REJECTED', 422);
      const attachment = await request('/mail/upload', {
        method: 'POST',
        headers: {
          'Content-Type': file.type || 'application/octet-stream',
          'X-File-Name': encodeURIComponent(file.name),
        },
        body: file,
      });
      draft.value.attachments.push(attachment);
    }
  } catch (e) {
    problem(e);
  } finally {
    busy.value = false;
    input.value = '';
  }
}

function move(role: string) {
  const box = folders.value.find(x => x.role === role);
  if (box) update({ mailbox: box.id });
}

async function manageFolder(action = 'create') {
  busy.value = true;
  error.value = '';
  try {
    await mutate('/mail/mailboxes', {
      ...(action === 'create' ? {} : { id: folder.value }),
      name: folderInput.value,
      destroy: action === 'delete',
    });
    folders.value = (await request('/mail/mailboxes')).mailboxes;
    if (action === 'delete') switchFolder(folders.value.find(x => x.role === 'inbox').id);
    notice.value = t.value.saved;
    folderInput.value = '';
  } catch (e) {
    problem(e);
  } finally {
    busy.value = false;
  }
}

const date = (value: string) =>
  new Intl.DateTimeFormat(props.locale, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value));

watch(() => props.locale, () => {
  notice.value = '';
});

onMounted(() => {
  connect();
  timer = setInterval(async () => {
    if (!busy.value && !composing.value && folders.value.length) {
      try {
        folders.value = (await request('/mail/mailboxes')).mailboxes;
      } catch (e) {
        problem(e);
      }
    }
  }, 60000);
});

onUnmounted(() => {
  if (timer) clearInterval(timer);
});
</script>

<template>
  <div class="mail-container">
    <!-- Header Bar -->
    <header class="mail-heading">
      <div class="mail-heading-left">
        <p class="eyebrow">OpenScan</p>
        <h1>{{ t.mail }}</h1>
      </div>
      <div class="mail-heading-right">
        <button type="button" class="button" :disabled="busy" @click="folders.length ? load() : connect()">
          {{ t.refresh }}
        </button>
      </div>
    </header>

    <!-- Error Alert Box -->
    <div v-if="error" class="alert alert-error" role="alert">
      <p><strong>{{ errorText }}</strong></p>
      <p v-if="['MAIL_NOT_CONFIGURED', 'MAIL_NOT_PROVISIONED'].includes(error)" class="mail-setup-tip">
        {{ t.mailSetup }}
      </p>
    </div>

    <!-- Status Notice -->
    <p v-if="notice" class="notice alert-success" role="status">
      {{ notice }}
    </p>

    <!-- Folder Manager Dropdown -->
    <details v-if="folders.length" class="folder-manager">
      <summary>{{ labels.manage }}</summary>
      <form class="actions folder-action-form" @submit.prevent="manageFolder()">
        <label>
          {{ labels.name }}
          <input v-model="folderInput" maxlength="100" required />
        </label>
        <button class="button" :disabled="busy">{{ labels.create }}</button>
        <template v-if="!folders.find(x => x.id === folder)?.role">
          <button type="button" class="button" :disabled="busy || !folderInput.trim()" @click="manageFolder('rename')">
            {{ labels.rename }}
          </button>
          <button type="button" class="button" :disabled="busy" @click="manageFolder('delete')">
            {{ labels.remove }}
          </button>
        </template>
      </form>
    </details>

    <!-- Main Mail 3-Pane Layout -->
    <div v-if="folders.length" class="mail-layout">
      <!-- 1. Left Sidebar: Compose Button + Folder List -->
      <aside class="mail-sidebar">
        <button type="button" class="button primary btn-compose" @click="compose()">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" aria-hidden="true">
            <line x1="12" y1="5" x2="12" y2="19" />
            <line x1="5" y1="12" x2="19" y2="12" />
          </svg>
          {{ t.compose }}
        </button>

        <nav class="mail-folder-nav" :aria-label="t.mail">
          <button
            v-for="box in folders"
            :key="box.id"
            type="button"
            class="mail-folder-btn"
            :class="{ active: folder === box.id }"
            @click="switchFolder(box.id)"
          >
            <span class="folder-title">{{ folderName(box) }}</span>
            <span v-if="box.unreadEmails > 0" class="unread-badge">
              {{ new Intl.NumberFormat(locale).format(box.unreadEmails) }}
            </span>
          </button>
        </nav>
      </aside>

      <!-- 2. Middle List Pane: Search & Messages List -->
      <section class="mail-list">
        <form class="mail-search" @submit.prevent="position = 0; load()">
          <label class="sr-only" for="mail-search">{{ t.searchMail }}</label>
          <input
            id="mail-search"
            v-model="query"
            type="search"
            :placeholder="t.searchMail"
          />
          <button type="submit" class="button">{{ t.search }}</button>
        </form>

        <p v-if="busy" role="status" class="mail-status-msg">{{ t.loading }}</p>
        <p v-else-if="!emails.length" class="mail-status-msg empty">{{ t.mailboxEmpty }}</p>

        <div v-else class="mail-items-container">
          <button
            v-for="message in emails"
            :key="message.id"
            type="button"
            class="mail-item"
            :class="{ unread: !message.keywords?.$seen, selected: selected?.id === message.id }"
            @click="read(message.id)"
          >
            <div class="mail-item-header">
              <span class="mail-sender">{{ message.from?.[0]?.name || message.from?.[0]?.email }}</span>
              <time :datetime="message.receivedAt" class="mail-time">{{ date(message.receivedAt) }}</time>
            </div>
            <strong class="mail-subject">{{ message.subject || t.noSubject }}</strong>
            <span class="mail-preview">{{ message.preview }}</span>
          </button>
        </div>

        <div v-if="position > 0 || position + 30 < total" class="pagination mail-pagination">
          <button v-if="position > 0" type="button" class="button" @click="position -= 30; load()">
            {{ t.previous }}
          </button>
          <button v-if="position + 30 < total" type="button" class="button" @click="position += 30; load()">
            {{ t.next }}
          </button>
        </div>
      </section>

      <!-- 3. Right Reader / Compose Pane -->
      <section class="mail-reader">
        <!-- Compose View -->
        <form v-if="composing" class="form-stack compose-form" @submit.prevent="save(true)">
          <h2>{{ t.compose }}</h2>
          <div class="compose-field">
            <label :for="'draft-to'">{{ t.to }}</label>
            <input id="draft-to" v-model="draft.to" required />
          </div>
          <div class="compose-field">
            <label :for="'draft-cc'">{{ t.cc }}</label>
            <input id="draft-cc" v-model="draft.cc" />
          </div>
          <div class="compose-field">
            <label :for="'draft-subject'">{{ t.subject }}</label>
            <input id="draft-subject" v-model="draft.subject" maxlength="998" />
          </div>
          <div class="compose-field">
            <label :for="'draft-body'">{{ t.body }}</label>
            <textarea id="draft-body" v-model="draft.text" rows="14" maxlength="500000" />
          </div>
          <div class="compose-field compose-attach-box">
            <label :for="'draft-attach'">{{ t.attachment }}</label>
            <input id="draft-attach" type="file" multiple accept=".txt,.pdf,.png,.jpg,.jpeg" :disabled="busy" @change="attach" />
            <small>{{ t.attachmentHint }}</small>
          </div>
          <ul v-if="draft.attachments.length" class="draft-attach-list">
            <li v-for="(file, index) in draft.attachments" :key="file.blobId" class="draft-attach-item">
              <span>📎 {{ file.name }}</span>
              <button type="button" class="button clear-btn-inline" @click="draft.attachments.splice(index, 1)">
                {{ t.clear }}
              </button>
            </li>
          </ul>
          <div class="actions compose-actions">
            <button type="submit" class="button primary" :disabled="busy">{{ t.send }}</button>
            <button type="button" class="button" :disabled="busy" @click="save()">{{ t.saveDraft }}</button>
            <button type="button" class="button" @click="composing = false">{{ t.cancel }}</button>
          </div>
        </form>

        <!-- Read Email View -->
        <article v-else-if="selected" class="mail-article">
          <div class="article-header">
            <h2>{{ selected.subject || t.noSubject }}</h2>
            <div class="article-meta">
              <span class="article-from"><strong>{{ t.to ? 'From' : '보낸사람' }}:</strong> {{ selected.from?.map((x: any) => x.email).join(', ') }}</span>
              <time :datetime="selected.receivedAt" class="article-date">{{ date(selected.receivedAt) }}</time>
            </div>
            <div class="article-move-row">
              <label>
                {{ labels.move }}
                <select :aria-label="labels.move" @change="update({ mailbox: ($event.target as HTMLSelectElement).value })">
                  <option value="" disabled selected>{{ labels.move }}</option>
                  <option v-for="box in folders" :key="box.id" :value="box.id">{{ folderName(box) }}</option>
                </select>
              </label>
            </div>
          </div>

          <!-- Email Actions Bar -->
          <div class="actions mail-action-bar">
            <button type="button" class="button" @click="compose('reply')">{{ t.reply }}</button>
            <button type="button" class="button" @click="compose('replyAll')">{{ t.replyAll }}</button>
            <button type="button" class="button" @click="compose('forward')">{{ t.forward }}</button>
            <button v-if="selected.keywords?.$draft" type="button" class="button" @click="compose('draft')">{{ t.compose }}</button>
            <button type="button" class="button" @click="update({ seen: !selected.keywords?.$seen })">
              {{ selected.keywords?.$seen ? t.markUnread : t.markRead }}
            </button>
            <button type="button" class="button" @click="update({ flagged: !selected.keywords?.$flagged })">
              {{ selected.keywords?.$flagged ? t.unstar : t.star }}
            </button>
            <button type="button" class="button" @click="move('trash')">{{ t.moveTrash }}</button>
            <button type="button" class="button" @click="move('inbox')">{{ t.restore }}</button>
          </div>

          <p class="source-note">{{ t.externalImages }}</p>

          <button v-if="selected.html" type="button" class="button toggle-html-btn" @click="html = !html">
            {{ html ? t.hideHTML : t.showHTML }}
          </button>

          <!-- Safe Sanitized Email Content -->
          <iframe
            v-if="html"
            class="mail-html"
            sandbox=""
            referrerpolicy="no-referrer"
            :title="selected.subject || t.body"
            :srcdoc="`<meta http-equiv='Content-Security-Policy' content=&quot;default-src 'none'; style-src 'none'; img-src 'none'&quot;>${selected.html}`"
          />
          <pre v-else class="mail-text">{{ selected.text }}</pre>

          <!-- Attachments Section -->
          <div v-if="selected.attachments?.length" class="mail-attachments-section">
            <h3>{{ t.attachments }}</h3>
            <div class="attachment-pills">
              <a
                v-for="file in selected.attachments"
                :key="file.blobId"
                class="attachment"
                :href="`/api/v2/mail/attachments/${encodeURIComponent(selected.id)}/${encodeURIComponent(file.blobId)}`"
              >
                📎 {{ file.name }}
              </a>
            </div>
          </div>
        </article>

        <!-- No Email Selected Placeholder -->
        <div v-else class="mail-placeholder">
          <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z"/>
            <polyline points="22,6 12,13 2,6"/>
          </svg>
          <p>{{ t.chooseMessage }}</p>
        </div>
      </section>
    </div>
  </div>
</template>
