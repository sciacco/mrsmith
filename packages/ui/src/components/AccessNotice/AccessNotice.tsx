import { useEffect, useState } from 'react';
import { ArrowLeft, Loader2, Lock, LogIn, ShieldAlert } from 'lucide-react';
import styles from './AccessNotice.module.css';

export type AccessNoticeState = 'loading' | 'reauthenticating' | 'unauthenticated' | 'forbidden';
const DEFAULT_TRANSIENT_DEFER_MS = 450;

interface AccessNoticeCopy {
  eyebrow: string;
  title: string;
  message: string;
}

interface AccessNoticeProps {
  state: AccessNoticeState;
  title?: string;
  message?: string;
  portalHref?: string;
  deferTransientMs?: number;
}

const copyByState: Record<AccessNoticeState, AccessNoticeCopy> = {
  loading: {
    eyebrow: 'Apertura applicazione',
    title: 'Preparazione area di lavoro',
    message: 'Caricamento della sessione in corso.',
  },
  reauthenticating: {
    eyebrow: 'Autenticazione',
    title: 'Sessione in ripristino',
    message: 'Reindirizzamento in corso.',
  },
  unauthenticated: {
    eyebrow: 'Autenticazione',
    title: 'Accesso richiesto',
    message: 'La sessione Keycloak non è disponibile. Ricarica la pagina o riapri l\'app dal portale.',
  },
  forbidden: {
    eyebrow: 'Autorizzazione',
    title: 'Accesso non assegnato',
    message: 'La sessione Keycloak è valida, ma questa applicazione non è assegnata al profilo.',
  },
};

function NoticeIcon({ state }: { state: AccessNoticeState }) {
  if (state === 'loading') {
    return <Loader2 className={styles.spinner} size={22} strokeWidth={1.8} aria-hidden="true" />;
  }
  if (state === 'reauthenticating') {
    return <LogIn size={22} strokeWidth={1.8} aria-hidden="true" />;
  }
  if (state === 'forbidden') {
    return <ShieldAlert size={22} strokeWidth={1.8} aria-hidden="true" />;
  }
  return <Lock size={22} strokeWidth={1.8} aria-hidden="true" />;
}

export function AccessNotice({
  state,
  title,
  message,
  portalHref = '/',
  deferTransientMs = DEFAULT_TRANSIENT_DEFER_MS,
}: AccessNoticeProps) {
  const copy = copyByState[state];
  const isTransient = state === 'loading' || state === 'reauthenticating';
  const [visibleTransientState, setVisibleTransientState] = useState<AccessNoticeState | null>(() =>
    isTransient && deferTransientMs <= 0 ? state : null,
  );

  useEffect(() => {
    if (!isTransient) {
      setVisibleTransientState(null);
      return undefined;
    }

    if (deferTransientMs <= 0) {
      setVisibleTransientState(state);
      return undefined;
    }

    setVisibleTransientState(null);
    const timeout = window.setTimeout(() => {
      setVisibleTransientState(state);
    }, deferTransientMs);

    return () => {
      window.clearTimeout(timeout);
    };
  }, [deferTransientMs, isTransient, state]);

  const notice = (
    <section
      className={styles.notice}
      role={isTransient ? 'status' : 'alert'}
      aria-live={isTransient ? 'polite' : 'assertive'}
    >
      <div className={styles.iconWrap}>
        <NoticeIcon state={state} />
      </div>
      <p className={styles.eyebrow}>{copy.eyebrow}</p>
      <h1>{title ?? copy.title}</h1>
      <p className={styles.message}>{message ?? copy.message}</p>
      {state === 'forbidden' && (
        <a className={styles.action} href={portalHref}>
          <ArrowLeft size={16} strokeWidth={1.9} aria-hidden="true" />
          <span>Torna al portale</span>
        </a>
      )}
    </section>
  );

  if (isTransient) {
    return (
      <div className={styles.transientSurface} aria-hidden={visibleTransientState !== state}>
        {visibleTransientState === state ? notice : null}
      </div>
    );
  }

  return notice;
}
