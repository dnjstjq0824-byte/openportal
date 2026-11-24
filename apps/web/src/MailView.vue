<script setup lang="ts">
import { computed } from 'vue';
import { useMailbox } from './useMailbox';
import ConfirmDialog from './ConfirmDialog.vue';
import type { Locale } from './i18n';
import { designMessages } from './design-copy';
const props = defineProps<{ locale: Locale; user: any }>();
const emit = defineEmits<{ expired: []; dirtyChange: [dirty: boolean] }>();
const { t, labels, folders, folder, emails, selected, busy, listBusy, readBusy, mutationBusy,
  error, errorText, notice, query, position, total, composing, html, folderInput, draft,
  sendBlocked, folderName, load, connect, switchFolder, read, update, compose, save, attach,
  move, manageFolder, date, closeCompose, confirmOpen, confirmDiscard, cancelDiscard } = useMailbox(props, () => emit('expired'), dirty => emit('dirtyChange', dirty));

function backToList(): void {
  selected.value = null;
}
const selectedIndex = computed(() => emails.value.findIndex((m: any) => m.id === selected.value?.id));
const positionText = computed(() => selectedIndex.value < 0 ? '' : t.value.positionOf
  .replace('{current}', String(position.value + selectedIndex.value + 1))
  .replace('{total}', String(total.value)));
const senderInitial = computed(() => {
  const from = selected.value?.from?.[0];
  const name = ((from?.name || from?.email || '?').trim().charAt(0) || '?').toUpperCase();
  return name;
});
const currentFolderName = computed(() => {
  const box = folders.value.find((f: any) => f.id === folder.value);
  return box ? folderName(box) : '';
});
function stepMessage(delta: number): void {
  const msg = emails.value[selectedIndex.value + delta];
  if (msg) read(msg.id);
}
</script>

<template>
  <div class="mail-container">
    <!-- Slim toolbar (Gmail-style: no title block) -->
    <header class="mail-toolbar">
      <h1 class="sr-only">{{ t.mail }}</h1>
      <button type="button" class="button" :disabled="busy" @click="folders.length ? load() : connect()">
        {{ t.refresh }}
      </button>
    </header>

    <!-- Error Alert Box -->
    <div v-if="error" class="alert" role="alert">
      <p><strong>{{ errorText }}</strong></p>
      <p v-if="['MAIL_NOT_CONFIGURED', 'MAIL_NOT_PROVISIONED'].includes(error)" class="mail-setup-tip">
        {{ t.mailSetup }}
      </p>
    </div>

    <!-- Status Notice -->
    <p v-if="notice" class="notice alert-success" role="status">
      {{ notice }}
    </p>

    <!-- Main Mail 3-Pane Layout -->
    <div v-if="folders.length" class="mail-layout">
      <!-- 1. Left Sidebar: Compose Button + Folder List -->
      <aside class="mail-sidebar">
        <button type="button" class="button primary btn-compose" :disabled="mutationBusy" @click="compose()">
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
            :disabled="mutationBusy"
            :aria-current="folder === box.id ? 'page' : undefined"
            @click="switchFolder(box.id)"
          >
            <span class="folder-title">{{ folderName(box) }}</span>
            <span v-if="box.unreadEmails > 0" class="unread-badge">
              {{ new Intl.NumberFormat(locale).format(box.unreadEmails) }}
            </span>
          </button>
        </nav>

        <details class="folder-manager">
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
      </aside>

      <!-- 2. List Pane (Gmail-style rows; hidden while reading) -->
      <section v-if="!selected && !composing" class="mail-list">
        <form class="mail-search" @submit.prevent="position = 0; load()">
          <label class="sr-only" for="mail-search">{{ t.searchMail }}</label>
          <input
            id="mail-search"
            v-model="query"
            type="search"
            :placeholder="t.searchMail"
          />
          <button type="submit" class="button">{{ designMessages[locale].searchButton }}</button>
        </form>

        <p v-if="listBusy" role="status" class="mail-status-msg">{{ t.loading }}</p>
        <p v-else-if="!emails.length" class="mail-status-msg empty">{{ t.mailboxEmpty }}</p>

        <div v-else class="mail-items-container">
          <button
            v-for="message in emails"
            :key="message.id"
            type="button"
            class="mail-item"
            :class="{ unread: !message.keywords?.$seen }"
            :disabled="mutationBusy"
            @click="read(message.id)"
          >
            <span class="mail-row-sender">{{ message.from?.[0]?.name || message.from?.[0]?.email }}</span>
            <span class="mail-row-subject">
              <strong>{{ message.subject || t.noSubject }}</strong>
              <span class="mail-row-snippet"> — {{ message.preview }}</span>
            </span>
            <time :datetime="message.receivedAt" class="mail-row-date">{{ date(message.receivedAt) }}</time>
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

      <!-- 3. Reader / Compose Pane (replaces the list, Gmail-style) -->
      <section v-else class="mail-reader" :aria-busy="readBusy || mutationBusy">
        <p v-if="readBusy" role="status">{{ t.loading }}</p>
        <!-- Compose View -->
        <form v-if="composing" class="form-stack compose-form" @submit.prevent="save(true)">
          <h2>{{ t.compose }}</h2>
          <div class="compose-field">
            <label :for="'draft-to'">{{ t.to }}</label>
            <input id="draft-to" v-model="draft.to" :disabled="busy || sendBlocked" required />
          </div>
          <div class="compose-field">
            <label :for="'draft-cc'">{{ t.cc }}</label>
            <input id="draft-cc" v-model="draft.cc" :disabled="busy || sendBlocked" />
          </div>
          <div class="compose-field">
            <label :for="'draft-subject'">{{ t.subject }}</label>
            <input id="draft-subject" v-model="draft.subject" :disabled="busy || sendBlocked" maxlength="998" />
          </div>
          <div class="compose-field">
            <label :for="'draft-body'">{{ t.body }}</label>
            <textarea id="draft-body" v-model="draft.text" :disabled="busy || sendBlocked" rows="14" maxlength="500000" />
          </div>
          <div class="compose-field compose-attach-box">
            <label :for="'draft-attach'">{{ t.attachment }}</label>
            <input id="draft-attach" type="file" multiple accept=".txt,.pdf,.png,.jpg,.jpeg" :disabled="busy || sendBlocked" @change="attach" />
            <small>{{ t.attachmentHint }}</small>
          </div>
          <ul v-if="draft.attachments.length" class="draft-attach-list">
            <li v-for="(file, index) in draft.attachments" :key="file.blobId" class="draft-attach-item">
              <span class="attach-name"><svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="m21 12-8.5 8.5a5.5 5.5 0 0 1-7.8-7.8L13 4.5a3.5 3.5 0 0 1 5 5L9.5 18a1.5 1.5 0 0 1-2.1-2.1L15 8.5"/></svg> {{ file.name }}</span>
              <button type="button" class="button clear-btn-inline" :disabled="busy || sendBlocked" @click="draft.attachments.splice(index, 1)">
                {{ t.clear }}
              </button>
            </li>
          </ul>
          <div class="actions compose-actions">
            <button type="submit" class="button primary" :disabled="busy || sendBlocked">{{ t.send }}</button>
            <button type="button" class="button" :disabled="busy || sendBlocked" @click="save()">{{ t.saveDraft }}</button>
            <button type="button" class="button" :disabled="mutationBusy" @click="closeCompose">{{ t.cancel }}</button>
          </div>
        </form>

        <!-- Read Email View (Gmail-style) -->
        <article v-else-if="selected" class="mail-article">
          <div class="reader-toolbar" role="toolbar" :aria-label="selected.subject || t.noSubject">
            <button type="button" class="icon-btn" :title="t.backToList" :aria-label="t.backToList" @click="backToList()">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><line x1="19" y1="12" x2="5" y2="12"/><polyline points="12 19 5 12 12 5"/></svg>
            </button>
            <button type="button" class="icon-btn" :title="t.archiveMail" :aria-label="t.archiveMail" :disabled="busy" @click="move('archive')">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect x="2" y="3" width="20" height="5" rx="1"/><path d="M4 8v11a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8"/><path d="M10 12h4"/></svg>
            </button>
            <button type="button" class="icon-btn" :title="t.reportSpam" :aria-label="t.reportSpam" :disabled="busy" @click="move('junk')">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polygon points="7.86 2 16.14 2 22 7.86 22 16.14 16.14 22 7.86 22 2 16.14 2 7.86 7.86 2"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>
            </button>
            <button type="button" class="icon-btn" :title="t.moveTrash" :aria-label="t.moveTrash" :disabled="busy" @click="move('trash')">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>
            </button>
            <button type="button" class="icon-btn" :title="selected.keywords?.$seen ? t.markUnread : t.markRead" :aria-label="selected.keywords?.$seen ? t.markUnread : t.markRead" :disabled="busy" @click="update({ seen: !selected.keywords?.$seen })">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect x="2" y="4" width="20" height="16" rx="2"/><path d="m22 7-10 6L2 7"/></svg>
            </button>
            <span class="reader-spacer"></span>
            <span class="reader-counter">{{ positionText }}</span>
            <button type="button" class="icon-btn" :title="t.previous" :aria-label="t.previous" :disabled="busy || selectedIndex <= 0" @click="stepMessage(-1)">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="15 18 9 12 15 6"/></svg>
            </button>
            <button type="button" class="icon-btn" :title="t.next" :aria-label="t.next" :disabled="busy || selectedIndex < 0 || selectedIndex >= emails.length - 1" @click="stepMessage(1)">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="9 18 15 12 9 6"/></svg>
            </button>
          </div>

          <div class="reader-subject-row">
            <h2>{{ selected.subject || t.noSubject }}</h2>
            <span v-if="currentFolderName" class="reader-folder-chip">{{ currentFolderName }}</span>
          </div>

          <div class="reader-sender-row">
            <span class="reader-avatar" aria-hidden="true">{{ senderInitial }}</span>
            <div class="reader-sender-meta">
              <div class="reader-sender-line">
                <strong>{{ selected.from?.[0]?.name || selected.from?.[0]?.email }}</strong>
                <span class="reader-email">&lt;{{ selected.from?.map((x: any) => x.email).join(', ') }}&gt;</span>
              </div>
              <div class="reader-to">{{ t.to }}: {{ selected.to?.map((x: any) => x.email).join(', ') }}</div>
            </div>
            <time :datetime="selected.receivedAt" class="reader-date">{{ date(selected.receivedAt) }}</time>
            <button type="button" class="icon-btn" :title="selected.keywords?.$flagged ? t.unstar : t.star" :aria-label="selected.keywords?.$flagged ? t.unstar : t.star" :aria-pressed="!!selected.keywords?.$flagged" :disabled="busy" @click="update({ flagged: !selected.keywords?.$flagged })">
              <svg width="18" height="18" viewBox="0 0 24 24" :fill="selected.keywords?.$flagged ? 'currentColor' : 'none'" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2"/></svg>
            </button>
            <button type="button" class="icon-btn" :title="t.reply" :aria-label="t.reply" :disabled="busy" @click="compose('reply')">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="9 14 4 9 9 4"/><path d="M20 20v-7a4 4 0 0 0-4-4H4"/></svg>
            </button>
            <details class="reader-more">
              <summary class="icon-btn" :title="t.services" :aria-label="t.services">
                <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><circle cx="12" cy="5" r="1.6"/><circle cx="12" cy="12" r="1.6"/><circle cx="12" cy="19" r="1.6"/></svg>
              </summary>
              <div class="reader-more-menu">
                <button type="button" :disabled="busy" @click="compose('replyAll')">{{ t.replyAll }}</button>
                <button type="button" :disabled="busy" @click="compose('forward')">{{ t.forward }}</button>
                <button v-if="selected.keywords?.$draft" type="button" :disabled="busy" @click="compose('draft')">{{ t.compose }}</button>
                <button type="button" :disabled="busy" @click="move('inbox')">{{ t.restore }}</button>
                <label class="reader-move">
                  {{ labels.move }}
                  <select :aria-label="labels.move" :disabled="busy" @change="update({ mailbox: ($event.target as HTMLSelectElement).value })">
                    <option value="" disabled selected>{{ labels.move }}</option>
                    <option v-for="box in folders" :key="box.id" :value="box.id">{{ folderName(box) }}</option>
                  </select>
                </label>
              </div>
            </details>
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
                <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="m21 12-8.5 8.5a5.5 5.5 0 0 1-7.8-7.8L13 4.5a3.5 3.5 0 0 1 5 5L9.5 18a1.5 1.5 0 0 1-2.1-2.1L15 8.5"/></svg> {{ file.name }}
              </a>
            </div>
          </div>

          <div class="reader-bottom">
            <button type="button" class="reader-pill" :disabled="busy" @click="compose('reply')">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="9 14 4 9 9 4"/><path d="M20 20v-7a4 4 0 0 0-4-4H4"/></svg>
              {{ t.reply }}
            </button>
            <button type="button" class="reader-pill" :disabled="busy" @click="compose('forward')">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="15 14 20 9 15 4"/><path d="M4 20v-7a4 4 0 0 1 4-4h12"/></svg>
              {{ t.forward }}
            </button>
          </div>
        </article>
      </section>
    </div>
    <ConfirmDialog v-if="confirmOpen" :title="t.discardTitle" :body="labels.discard" :confirm="t.discardConfirm" :cancel="t.cancel" @confirm="confirmDiscard" @cancel="cancelDiscard"/>
  </div>
</template>
