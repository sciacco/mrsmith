import { ArrowLeft, Loader2, Lock, LogIn, ShieldAlert } from 'lucide-react';
import styles from './AccessNotice.module.css';

export type AccessNoticeState = 'loading' | 'reauthenticating' | 'unauthenticated' | 'forbidden';

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
}

const copyByState: Record<AccessNoticeState, AccessNoticeCopy> = {
  loading: {
    eyebrow: 'Autenticazione',
    title: 'Verifica accesso',
    message: 'Controllo del profilo in corso.',
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

export function AccessNotice({ state, title, message, portalHref = '/' }: AccessNoticeProps) {
  const copy = copyByState[state];
  const isTransient = state === 'loading' || state === 'reauthenticating';

  return (
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
}
