import { computed, onMounted, onUnmounted, ref, watch } from 'vue';
import { APIError, mutate, request } from './client';
import { errors, messages, folderLabels, type Locale } from './i18n';

export function useMailbox(props: { locale: Locale; user: any }, expired: () => void, dirtyChange: (dirty: boolean) => void = () => {}) {
  const t = computed(() => messages[props.locale]);
  const labels = computed(() => folderLabels[props.locale]);
  const folders = ref<any[]>([]), folder = ref(''), emails = ref<any[]>([]);
  const selected = ref<any>(null), error = ref(''), notice = ref('');
  const query = ref(''), position = ref(0), total = ref(0), folderInput = ref('');
  const composing = ref(false), html = ref(false);
  const listBusy = ref(false), readBusy = ref(false), mutationBusy = ref(false), connectionBusy = ref(false);
  const busy = computed(() => listBusy.value || readBusy.value || mutationBusy.value || connectionBusy.value);
  const emptyDraft = () => ({ to: '', cc: '', subject: '', text: '', attachments: [] });
  const draft = ref<any>(emptyDraft());
  const baseline = ref(JSON.stringify(draft.value));
  const uncertainDrafts = ref(new Set<string>());
  const sendBlocked = computed(() => uncertainDrafts.value.has(draft.value.id));
  const draftDirty = computed(() => composing.value && JSON.stringify(draft.value) !== baseline.value);
  const errorText = computed(() => errors[props.locale][error.value] || errors[props.locale].fallback);
  let listGeneration = 0, readerGeneration = 0, disposed = false;
  let listController: AbortController | undefined, readerController: AbortController | undefined;
  let timer: ReturnType<typeof setInterval> | undefined;

  function problem(e: unknown) {
    if (disposed) return;
    if (e instanceof APIError && e.status === 401) { expired(); return; }
    error.value = e instanceof APIError ? e.code : 'fallback';
  }
  const folderName = (box: any) => (t.value as any)[box.role] || box.name;
  function clearReader() {
    ++readerGeneration;
    readerController?.abort();
    readBusy.value = false;
    selected.value = null;
    html.value = false;
  }
  function canDiscard() { return !draftDirty.value || window.confirm(labels.value.discard); }
  function closeCompose() {
    if (mutationBusy.value || !canDiscard()) return;
    composing.value = false;
  }
  async function load() {
    const own = ++listGeneration;
    listController?.abort();
    const controller = new AbortController(); listController = controller;
    listBusy.value = true; error.value = '';
    try {
      const data = await request(`/mail/messages?${new URLSearchParams({ mailbox: folder.value, q: query.value, position: String(position.value) })}`, { signal: AbortSignal.any([controller.signal, AbortSignal.timeout(15000)]) });
      if (disposed || own !== listGeneration) return;
      if (!Array.isArray(data.messages)) throw new APIError('MAIL_PROVIDER_ERROR', 502);
      emails.value = data.messages;
      total.value = Number.isInteger(data.total) && data.total >= 0 ? data.total : data.messages.length;
    } catch (e) { if (own === listGeneration && !controller.signal.aborted) problem(e); }
    finally { if (own === listGeneration) listBusy.value = false; }
  }
  async function connect() {
    if (connectionBusy.value || mutationBusy.value) return;
    connectionBusy.value = true; error.value = '';
    try {
      const data = await request('/mail/mailboxes');
      if (disposed) return;
      folders.value = data.mailboxes;
      folder.value = folders.value.find(x => x.role === 'inbox')?.id || folders.value[0]?.id || '';
      if (folder.value) await load();
    } catch (e) { problem(e); }
    finally { connectionBusy.value = false; }
  }
  function switchFolder(id: string) {
    if (mutationBusy.value || !canDiscard()) return;
    clearReader(); folder.value = id; position.value = 0;
    emails.value = []; total.value = 0; composing.value = false; query.value = '';
    load();
  }
  async function read(id: string) {
    if (mutationBusy.value || !canDiscard()) return;
    clearReader();
    const own = readerGeneration;
    const controller = new AbortController(); readerController = controller;
    const current = () => !disposed && own === readerGeneration && !controller.signal.aborted;
    readBusy.value = true; error.value = ''; composing.value = false;
    try {
      const signal = AbortSignal.any([controller.signal, AbortSignal.timeout(15000)]);
      const message = await request(`/mail/messages/${encodeURIComponent(id)}`, { signal });
      if (!current()) return;
      message.keywords ||= {};
      selected.value = message;
      await request(`/mail/messages/${encodeURIComponent(id)}`, { method: 'PATCH', body: JSON.stringify({ seen: true }), signal });
      if (!current()) return;
      message.keywords.$seen = true;
      const item = emails.value.find(x => x.id === id);
      if (item) { item.keywords ||= {}; item.keywords.$seen = true; }
    } catch (e) { if (current()) problem(e); }
    finally { if (own === readerGeneration) readBusy.value = false; }
  }
  async function update(change: any) {
    if (mutationBusy.value || readBusy.value || !selected.value) return;
    const message = selected.value;
    mutationBusy.value = true; error.value = '';
    try {
      await mutate(`/mail/messages/${encodeURIComponent(message.id)}`, change, 'PATCH');
      if (disposed) return;
      if (change.mailbox) clearReader();
      else {
        if (change.seen !== undefined) message.keywords.$seen = change.seen;
        if (change.flagged !== undefined) message.keywords.$flagged = change.flagged;
      }
      await load();
    } catch (e) { problem(e); }
    finally { mutationBusy.value = false; }
  }
  function compose(kind = 'new') {
    if (mutationBusy.value || !canDiscard()) return;
    const previous = selected.value;
    clearReader(); notice.value = ''; error.value = '';
    draft.value = emptyDraft();
    if (previous && kind === 'draft') {
      draft.value = { id: previous.id, to: previous.to?.map((x: any) => x.email).join(', ') || '', cc: previous.cc?.map((x: any) => x.email).join(', ') || '', subject: previous.subject, text: previous.text, attachments: previous.attachments || [] };
      if (previous.submissionStatus) uncertainDrafts.value.add(previous.id);
      if (sendBlocked.value) {
        if (previous.submissionStatus === 'accepted') notice.value = t.value.sendAccepted;
        else error.value = 'SEND_STATUS_UNKNOWN';
      }
    } else if (previous && kind !== 'new') {
      const from = previous.from?.map((x: any) => x.email) || [];
      if (kind !== 'forward') draft.value.replyToId = previous.id;
      draft.value.to = kind === 'forward' ? '' : from.filter((x: string) => x.toLowerCase() !== props.user.email.toLowerCase()).join(', ');
      if (kind === 'replyAll') draft.value.cc = [...new Set<string>([...(previous.to || []), ...(previous.cc || [])].map((x: any) => x.email).filter((x: string) => x.toLowerCase() !== props.user.email.toLowerCase() && !from.some((sender: string) => sender.toLowerCase() === x.toLowerCase())))].join(', ');
      draft.value.subject = `${kind === 'forward' ? 'Fwd:' : 'Re:'} ${previous.subject || ''}`;
      draft.value.text = `\n\n> ${(previous.text || '').replaceAll('\n', '\n> ')}`;
    }
    baseline.value = JSON.stringify(draft.value);
    composing.value = true;
  }
  async function save(send = false) {
    if (mutationBusy.value || sendBlocked.value) return;
    mutationBusy.value = true; error.value = ''; notice.value = '';
    let submitting = false;
    try {
      const saved = await mutate('/mail/drafts', draft.value);
      draft.value.id = saved.id;
      baseline.value = JSON.stringify(draft.value);
      if (send) {
        submitting = true;
        uncertainDrafts.value.add(saved.id);
        const result = await mutate('/mail/send', { id: saved.id });
        if (result.accepted !== true) throw new APIError('SEND_STATUS_UNKNOWN', 409);
        uncertainDrafts.value.delete(saved.id);
        notice.value = t.value.sendAccepted; composing.value = false;
      } else notice.value = t.value.saved;
      await load();
    } catch (e) {
      if (e instanceof APIError && e.code === 'SEND_STATUS_UNKNOWN' && draft.value.id) uncertainDrafts.value.add(draft.value.id);
      if (submitting && !(e instanceof APIError && e.status === 401)) error.value = 'SEND_STATUS_UNKNOWN';
      else problem(e);
    } finally { mutationBusy.value = false; }
  }
  async function attach(event: Event) {
    const input = event.target as HTMLInputElement;
    const files = Array.from(input.files || []);
    if (mutationBusy.value || sendBlocked.value) { input.value = ''; return; }
    mutationBusy.value = true; error.value = '';
    try {
      if (files.length + draft.value.attachments.length > 5) throw new APIError('ATTACHMENT_REJECTED', 422);
      for (const file of files) {
        if (file.size > 5 * 1024 * 1024) throw new APIError('ATTACHMENT_REJECTED', 422);
        const attachment = await request('/mail/upload', { method: 'POST', headers: { 'Content-Type': file.type || 'application/octet-stream', 'X-File-Name': encodeURIComponent(file.name) }, body: file });
        if (!disposed) draft.value.attachments.push(attachment);
      }
    } catch (e) { problem(e); }
    finally { mutationBusy.value = false; input.value = ''; }
  }
  function move(role: string) {
    const box = folders.value.find(x => x.role === role);
    if (box) update({ mailbox: box.id });
  }
  async function manageFolder(action = 'create') {
    if (mutationBusy.value) return;
    mutationBusy.value = true; error.value = '';
    try {
      await mutate('/mail/mailboxes', { ...(action === 'create' ? {} : { id: folder.value }), name: folderInput.value, destroy: action === 'delete' });
      folders.value = (await request('/mail/mailboxes')).mailboxes;
      if (action === 'delete') {
        clearReader(); composing.value = false;
        folder.value = folders.value.find(x => x.role === 'inbox')?.id || folders.value[0]?.id || '';
        position.value = 0; emails.value = [];
        if (folder.value) await load();
      }
      notice.value = t.value.saved; folderInput.value = '';
    } catch (e) { problem(e); }
    finally { mutationBusy.value = false; }
  }
  function date(value: string) {
    const parsed = new Date(value);
    return Number.isNaN(parsed.getTime()) ? labels.value.unknownDate : new Intl.DateTimeFormat(props.locale, { dateStyle: 'medium', timeStyle: 'short' }).format(parsed);
  }
  function beforeUnload(event: BeforeUnloadEvent) {
    if (draftDirty.value) { event.preventDefault(); event.returnValue = ''; }
  }
  watch(() => props.locale, () => { notice.value = ''; });
  watch(draftDirty, dirtyChange, { flush: 'sync' });
  onMounted(() => {
    connect(); window.addEventListener('beforeunload', beforeUnload);
    timer = setInterval(async () => {
      if (!busy.value && !composing.value && folders.value.length) {
        try { const data = await request('/mail/mailboxes'); if (!disposed) folders.value = data.mailboxes; }
        catch (e) { problem(e); }
      }
    }, 60000);
  });
  onUnmounted(() => {
    disposed = true; ++listGeneration; ++readerGeneration;
    listController?.abort(); readerController?.abort();
    if (timer) clearInterval(timer);
    window.removeEventListener('beforeunload', beforeUnload);
    dirtyChange(false);
  });
  return { t, labels, folders, folder, emails, selected, busy, listBusy, readBusy, mutationBusy, error, errorText, notice, query, position, total, composing, html, folderInput, draft, draftDirty, sendBlocked, folderName, load, connect, switchFolder, read, update, compose, save, attach, move, manageFolder, date, closeCompose };
}
