import { useEffect, useMemo, useRef, useState, type ChangeEvent, type DragEvent } from 'react';
import { AlertCircle, CheckCircle2, FileText, Paperclip, Phone, Send, X } from 'lucide-react';
import { Button } from '../Button/Button';
import { Modal } from '../Modal/Modal';
import { Tooltip } from '../Tooltip/Tooltip';
import styles from './SupportMenu.module.css';

export type SupportPriority = 'low' | 'normal' | 'high' | 'urgent';

export interface SupportUser {
  name?: string;
  email?: string;
  roles?: readonly string[];
}

export interface AppShellSupportConfig {
  appId?: string;
  authenticated?: boolean;
  user?: SupportUser | null;
  getAccessToken?: (minValidity?: number) => Promise<string | undefined> | string | undefined;
  forceRefreshToken?: () => Promise<string | undefined> | string | undefined;
}

interface SupportMenuProps {
  appName?: string;
  support: AppShellSupportConfig;
}

type SubmitState =
  | { status: 'idle' }
  | { status: 'submitting' }
  | { status: 'success'; requestId: number; emailNotification: string }
  | { status: 'error'; message: string };

interface ApiHistoryEntry {
  timestamp: string;
  method: string;
  path: string;
  status: number;
  ok: boolean;
  durationMs: number;
  requestId?: string;
  error?: string;
}

interface SupportResponse {
  id: number;
  status: string;
  emailNotification: string;
}

const priorityOptions: Array<{ value: SupportPriority; label: string }> = [
  { value: 'low', label: 'Bassa' },
  { value: 'normal', label: 'Normale' },
  { value: 'high', label: 'Alta' },
  { value: 'urgent', label: 'Urgente' },
];

const maxAttachmentCount = 5;
const maxAttachmentBytes = 10 * 1024 * 1024;
const maxAttachmentTotalBytes = 25 * 1024 * 1024;

const allowedAttachmentTypes = new Set([
  'application/csv',
  'application/msword',
  'application/pdf',
  'application/vnd.ms-excel',
  'application/vnd.ms-powerpoint',
  'application/vnd.openxmlformats-officedocument.presentationml.presentation',
  'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
  'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
  'image/jpeg',
  'image/png',
  'image/webp',
  'text/csv',
  'text/plain',
]);

const allowedAttachmentExtensions = new Set([
  '.csv',
  '.doc',
  '.docx',
  '.jpeg',
  '.jpg',
  '.pdf',
  '.png',
  '.ppt',
  '.pptx',
  '.txt',
  '.webp',
  '.xls',
  '.xlsx',
]);

const attachmentAccept = [
  ...allowedAttachmentExtensions,
  ...allowedAttachmentTypes,
].join(',');

export function SupportMenu({ appName, support }: SupportMenuProps) {
  const [menuOpen, setMenuOpen] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [message, setMessage] = useState('');
  const [priority, setPriority] = useState<SupportPriority>('normal');
  const [includeContext, setIncludeContext] = useState(true);
  const [attachments, setAttachments] = useState<File[]>([]);
  const [submitState, setSubmitState] = useState<SubmitState>({ status: 'idle' });
  const [isFileInputFocused, setIsFileInputFocused] = useState(false);
  const wrapperRef = useRef<HTMLDivElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const appId = support.appId ?? slugify(appName ?? 'mini-app');

  useEffect(() => {
    if (!menuOpen) return undefined;

    function handleClickOutside(event: MouseEvent) {
      if (wrapperRef.current && !wrapperRef.current.contains(event.target as Node)) {
        setMenuOpen(false);
      }
    }
    function handleEscape(event: KeyboardEvent) {
      if (event.key === 'Escape') setMenuOpen(false);
    }

    document.addEventListener('mousedown', handleClickOutside);
    document.addEventListener('keydown', handleEscape);
    return () => {
      document.removeEventListener('mousedown', handleClickOutside);
      document.removeEventListener('keydown', handleEscape);
    };
  }, [menuOpen]);

  const contextPreview = useMemo(() => buildContextPreview(appName, getApiHistory()), [appName, modalOpen]);
  const attachmentsDisabled = submitState.status === 'submitting' || submitState.status === 'success';
  const attachmentTotalBytes = useMemo(() => totalAttachmentBytes(attachments), [attachments]);

  function openModal() {
    setMenuOpen(false);
    setSubmitState({ status: 'idle' });
    setModalOpen(true);
  }

  function closeModal() {
    if (submitState.status === 'submitting') return;
    setModalOpen(false);
  }

  async function submit() {
    const trimmed = message.trim();
    if (!trimmed) {
      setSubmitState({ status: 'error', message: 'Descrivi brevemente il problema.' });
      return;
    }

    setSubmitState({ status: 'submitting' });
    try {
      const response = await postSupportRequest(support, {
        message: trimmed,
        priority,
        technicalContextIncluded: includeContext,
        context: buildSupportContext({ appId, appName, user: support.user }),
        attachments,
      });
      setSubmitState({
        status: 'success',
        requestId: response.id,
        emailNotification: response.emailNotification,
      });
      setMessage('');
      setPriority('normal');
      setIncludeContext(true);
      setAttachments([]);
    } catch (error) {
      setSubmitState({
        status: 'error',
        message: error instanceof Error ? error.message : 'Richiesta non inviata.',
      });
    }
  }

  function addAttachments(files: File[]) {
    if (files.length === 0) return;

    const next = [...attachments, ...files];
    const validationError = validateAttachments(next);
    if (validationError) {
      setSubmitState({ status: 'error', message: validationError });
      return;
    }
    setAttachments(next);
    setSubmitState({ status: 'idle' });
  }

  function handleAttachmentInput(event: ChangeEvent<HTMLInputElement>) {
    addAttachments(Array.from(event.target.files ?? []));
    event.target.value = '';
  }

  function handleAttachmentDragOver(event: DragEvent<HTMLLabelElement>) {
    event.preventDefault();
  }

  function handleAttachmentDrop(event: DragEvent<HTMLLabelElement>) {
    event.preventDefault();
    if (attachmentsDisabled) return;
    addAttachments(Array.from(event.dataTransfer.files ?? []));
  }

  function removeAttachment(index: number) {
    setAttachments((current) => current.filter((_, currentIndex) => currentIndex !== index));
  }

  return (
    <div className={styles.wrapper} ref={wrapperRef}>
      <Tooltip content="Operator" placement="bottom" disabled={menuOpen}>
        <button
          type="button"
          className={styles.trigger}
          aria-label="Apri richiesta supporto"
          aria-haspopup="menu"
          aria-expanded={menuOpen}
          onClick={() => setMenuOpen((open) => !open)}
        >
          <Phone size={18} strokeWidth={2.1} />
        </button>
      </Tooltip>

      {menuOpen && (
        <div className={styles.menu} role="menu">
          <button type="button" className={styles.menuItem} role="menuitem" onClick={openModal}>
            I need an exit
          </button>
        </div>
      )}

      <Modal open={modalOpen} onClose={closeModal} title="Operator" size="wide" dismissible={submitState.status !== 'submitting'} closeOnEscape={!isFileInputFocused}>
        <div className={styles.form}>
          <label className={styles.field}>
            <span>Messaggio</span>
            <textarea
              value={message}
              maxLength={4000}
              rows={5}
              onChange={(event) => setMessage(event.target.value)}
              placeholder="Cosa sta succedendo?"
              disabled={submitState.status === 'submitting' || submitState.status === 'success'}
            />
          </label>

          <fieldset className={styles.priorityGroup} disabled={submitState.status === 'submitting' || submitState.status === 'success'}>
            <legend>Priorita</legend>
            <div className={styles.priorityOptions}>
              {priorityOptions.map((option) => (
                <label key={option.value} className={styles.priorityOption}>
                  <input
                    type="radio"
                    name="support-priority"
                    value={option.value}
                    checked={priority === option.value}
                    onChange={() => setPriority(option.value)}
                  />
                  <span>{option.label}</span>
                </label>
              ))}
            </div>
          </fieldset>

          <div className={styles.attachmentSection}>
            <div className={styles.attachmentHeader}>
              <span>Allegati</span>
              <small>{attachments.length}/{maxAttachmentCount} - {formatFileSize(attachmentTotalBytes)}</small>
            </div>
            <label
              className={`${styles.attachmentDrop} ${attachmentsDisabled ? styles.attachmentDropDisabled : ''}`}
              onDragOver={handleAttachmentDragOver}
              onDrop={handleAttachmentDrop}
            >
              <Paperclip size={18} />
              <span>Trascina screenshot o file</span>
              <small>PNG, PDF, Office, testo - max 10 MiB ciascuno</small>
              <input
                ref={fileInputRef}
                type="file"
                multiple
                accept={attachmentAccept}
                onChange={handleAttachmentInput}
                onFocus={() => setIsFileInputFocused(true)}
                onBlur={() => setIsFileInputFocused(false)}
                disabled={attachmentsDisabled}
              />
            </label>
            {attachments.length > 0 && (
              <ul className={styles.attachmentList} aria-label="Allegati selezionati">
                {attachments.map((file, index) => (
                  <li key={`${file.name}-${file.size}-${file.lastModified}-${index}`} className={styles.attachmentItem}>
                    <FileText size={16} />
                    <span className={styles.attachmentName}>{file.name}</span>
                    <span className={styles.attachmentSize}>{formatFileSize(file.size)}</span>
                    <button
                      type="button"
                      className={styles.attachmentRemove}
                      aria-label={`Rimuovi ${file.name}`}
                      onClick={() => removeAttachment(index)}
                      disabled={attachmentsDisabled}
                    >
                      <X size={15} />
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </div>

          <label className={styles.contextToggle}>
            <input
              type="checkbox"
              checked={includeContext}
              disabled={submitState.status === 'submitting' || submitState.status === 'success'}
              onChange={(event) => setIncludeContext(event.target.checked)}
            />
            <span>Includi contesto tecnico</span>
          </label>

          {includeContext && (
            <div className={styles.contextPreview} aria-label="Contesto tecnico incluso">
              {contextPreview.map((item) => (
                <span key={item}>{item}</span>
              ))}
            </div>
          )}

          {submitState.status === 'success' && (
            <div className={styles.success} role="status">
              <CheckCircle2 size={18} />
              <span>Richiesta #{submitState.requestId} creata.</span>
            </div>
          )}

          {submitState.status === 'error' && (
            <div className={styles.error} role="alert">
              <AlertCircle size={18} />
              <span>{submitState.message}</span>
            </div>
          )}

          <div className={styles.actions}>
            <Button variant="secondary" onClick={closeModal} disabled={submitState.status === 'submitting'}>
              Chiudi
            </Button>
            {submitState.status !== 'success' && (
              <Button
                onClick={submit}
                loading={submitState.status === 'submitting'}
                rightIcon={<Send size={16} />}
                disabled={!message.trim()}
              >
                Invia
              </Button>
            )}
          </div>
        </div>
      </Modal>
    </div>
  );
}

function buildSupportContext({
  appId,
  appName,
  user,
}: {
  appId: string;
  appName?: string;
  user?: SupportUser | null;
}) {
  const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone;
  return {
    app: {
      id: appId,
      name: appName ?? '',
    },
    page: {
      url: window.location.href,
      path: window.location.pathname,
      search: window.location.search,
      hash: window.location.hash,
      title: document.title,
      referrer: document.referrer,
    },
    browser: {
      userAgent: navigator.userAgent,
      language: navigator.language,
      languages: navigator.languages,
      timezone,
      online: navigator.onLine,
      viewport: {
        width: window.innerWidth,
        height: window.innerHeight,
        devicePixelRatio: window.devicePixelRatio,
      },
    },
    user: {
      name: user?.name ?? '',
      email: user?.email ?? '',
      roles: user?.roles ?? [],
    },
    api: {
      recentRequests: getApiHistory(),
    },
    capturedAt: new Date().toISOString(),
  };
}

function buildContextPreview(appName: string | undefined, apiHistory: ApiHistoryEntry[]) {
  const path = window.location.pathname || '/';
  const failures = apiHistory.filter((entry) => !entry.ok).length;
  return [
    appName ? `App: ${appName}` : 'App corrente',
    `Pagina: ${path}`,
    `${apiHistory.length} chiamate API recenti`,
    failures > 0 ? `${failures} errori API` : 'Nessun errore API recente',
  ];
}

async function postSupportRequest(
  support: AppShellSupportConfig,
  body: {
    message: string;
    priority: SupportPriority;
    technicalContextIncluded: boolean;
    context: unknown;
    attachments: File[];
  },
): Promise<SupportResponse> {
  const token = await support.getAccessToken?.(30);
  if (!token) {
    throw new Error('Sessione non disponibile.');
  }

  let response = await sendSupportFetch(token, body);

  if (response.status === 401 && support.forceRefreshToken) {
    const fresh = await support.forceRefreshToken();
    if (fresh) {
      response = await sendSupportFetch(fresh, body);
    }
  }

  if (!response.ok) {
    throw new Error(await supportErrorMessage(response));
  }
  return response.json() as Promise<SupportResponse>;
}

function sendSupportFetch(
  token: string,
  body: {
    message: string;
    priority: SupportPriority;
    technicalContextIncluded: boolean;
    context: unknown;
    attachments: File[];
  },
) {
  const payload = {
    message: body.message,
    priority: body.priority,
    technicalContextIncluded: body.technicalContextIncluded,
    context: body.context,
  };

  if (body.attachments.length === 0) {
    return fetch('/api/support/v1/requests', {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${token}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(payload),
    });
  }

  const form = new FormData();
  form.append('payload', JSON.stringify(payload));
  for (const attachment of body.attachments) {
    form.append('attachments', attachment, attachment.name);
  }

  return fetch('/api/support/v1/requests', {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${token}`,
    },
    body: form,
  });
}

async function supportErrorMessage(response: Response) {
  try {
    const payload = (await response.clone().json()) as { error?: string };
    switch (payload.error) {
      case 'support_database_not_configured':
      case 'support_database_not_ready':
        return 'Supporto non configurato.';
      case 'too_many_attachments':
        return 'Puoi allegare al massimo 5 file.';
      case 'attachment_too_large':
        return 'Un allegato supera 10 MiB.';
      case 'attachments_too_large':
        return 'Gli allegati superano 25 MiB totali.';
      case 'unsupported_attachment_type':
        return 'Uno degli allegati ha un formato non supportato.';
      default:
        break;
    }
  } catch {
    // Fall back to status-only message below.
  }
  return `Richiesta non inviata (${response.status}).`;
}

function getApiHistory(): ApiHistoryEntry[] {
  const win = window as Window & { __MRSMITH_API_HISTORY__?: ApiHistoryEntry[] };
  return [...(win.__MRSMITH_API_HISTORY__ ?? [])].slice(0, 10);
}

function validateAttachments(files: File[]): string | null {
  if (files.length > maxAttachmentCount) {
    return 'Puoi allegare al massimo 5 file.';
  }

  let total = 0;
  for (const file of files) {
    if (file.size <= 0) {
      return 'Uno degli allegati e vuoto.';
    }
    if (file.size > maxAttachmentBytes) {
      return `${file.name} supera 10 MiB.`;
    }
    if (!isAllowedAttachment(file)) {
      return `${file.name} ha un formato non supportato.`;
    }
    total += file.size;
  }

  if (total > maxAttachmentTotalBytes) {
    return 'Gli allegati superano 25 MiB totali.';
  }
  return null;
}

function isAllowedAttachment(file: File): boolean {
  const type = file.type.trim().toLowerCase();
  if (type && allowedAttachmentTypes.has(type)) return true;
  return allowedAttachmentExtensions.has(fileExtension(file.name));
}

function fileExtension(name: string): string {
  const index = name.lastIndexOf('.');
  if (index < 0) return '';
  return name.slice(index).toLowerCase();
}

function totalAttachmentBytes(files: File[]): number {
  return files.reduce((total, file) => total + file.size, 0);
}

function formatFileSize(size: number): string {
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KiB`;
  return `${(size / (1024 * 1024)).toFixed(1)} MiB`;
}

function slugify(value: string) {
  return value
    .toLowerCase()
    .normalize('NFKD')
    .replace(/[\u0300-\u036f]/g, '')
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '') || 'mini-app';
}
