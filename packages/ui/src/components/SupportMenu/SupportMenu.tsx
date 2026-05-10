import { useEffect, useMemo, useRef, useState } from 'react';
import { AlertCircle, CheckCircle2, Phone, Send } from 'lucide-react';
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

export function SupportMenu({ appName, support }: SupportMenuProps) {
  const [menuOpen, setMenuOpen] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [message, setMessage] = useState('');
  const [priority, setPriority] = useState<SupportPriority>('normal');
  const [includeContext, setIncludeContext] = useState(true);
  const [submitState, setSubmitState] = useState<SubmitState>({ status: 'idle' });
  const wrapperRef = useRef<HTMLDivElement>(null);
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
      });
      setSubmitState({
        status: 'success',
        requestId: response.id,
        emailNotification: response.emailNotification,
      });
      setMessage('');
      setPriority('normal');
      setIncludeContext(true);
    } catch (error) {
      setSubmitState({
        status: 'error',
        message: error instanceof Error ? error.message : 'Richiesta non inviata.',
      });
    }
  }

  return (
    <div className={styles.wrapper} ref={wrapperRef}>
      <Tooltip content="Operator" placement="bottom">
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

      <Modal open={modalOpen} onClose={closeModal} title="Operator" size="wide" dismissible={submitState.status !== 'submitting'}>
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
  },
): Promise<SupportResponse> {
  const token = await support.getAccessToken?.(30);
  if (!token) {
    throw new Error('Sessione non disponibile.');
  }

  let response = await fetch('/api/support/v1/requests', {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${token}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(body),
  });

  if (response.status === 401 && support.forceRefreshToken) {
    const fresh = await support.forceRefreshToken();
    if (fresh) {
      response = await fetch('/api/support/v1/requests', {
        method: 'POST',
        headers: {
          Authorization: `Bearer ${fresh}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(body),
      });
    }
  }

  if (!response.ok) {
    throw new Error(await supportErrorMessage(response));
  }
  return response.json() as Promise<SupportResponse>;
}

async function supportErrorMessage(response: Response) {
  try {
    const payload = (await response.clone().json()) as { error?: string };
    switch (payload.error) {
      case 'support_database_not_configured':
      case 'support_database_not_ready':
        return 'Supporto non configurato.';
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

function slugify(value: string) {
  return value
    .toLowerCase()
    .normalize('NFKD')
    .replace(/[\u0300-\u036f]/g, '')
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '') || 'mini-app';
}
