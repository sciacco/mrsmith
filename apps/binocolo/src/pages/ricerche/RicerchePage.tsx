import { Button, Icon, Skeleton } from '@mrsmith/ui';
import { useCallback, useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useApiClient } from '../../api/client';
import type { MASessionListResponse, MASessionSummary } from '../../api/types';
import {
  dateLabel,
  errorLabel,
  numberFormat,
  sessionStatusLabel,
} from './helpers';
import styles from './Ricerche.module.css';

export function RicerchePage() {
  const api = useApiClient();
  const navigate = useNavigate();
  const [sessions, setSessions] = useState<MASessionSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const loadSessions = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await api.get<MASessionListResponse>('/binocolo/v1/ma/sessions?visibility=active');
      setSessions(data.items);
    } catch (err) {
      setError(errorLabel(err));
    } finally {
      setLoading(false);
    }
  }, [api]);

  useEffect(() => {
    void loadSessions();
  }, [loadSessions]);

  return (
    <main className={styles.page}>
      <header className={styles.header}>
        <div>
          <span className={styles.eyebrow}>Binocolo</span>
          <h1>Ricerche</h1>
          <p className={styles.subtitle}>Perimetri M&amp;A, run in corso e shortlist completate.</p>
        </div>
        <div className={styles.inlineActions}>
          <Button variant="secondary" onClick={() => void loadSessions()} loading={loading} leftIcon={<Icon name="refresh-cw" />}>
            Aggiorna
          </Button>
          <Button onClick={() => navigate('/ricerche/nuova')} leftIcon={<Icon name="plus" />}>
            Nuova ricerca
          </Button>
        </div>
      </header>

      {error ? (
        <div className={styles.danger} role="alert">
          <Icon name="triangle-alert" size={18} />
          <span>{error}</span>
        </div>
      ) : null}

      <section className={styles.panel} aria-labelledby="ricerche-title">
        <div className={styles.panelHeader}>
          <div>
            <h2 id="ricerche-title">Attive</h2>
            <p className={styles.hint}>{numberFormat.format(sessions.length)} ricerche disponibili</p>
          </div>
        </div>
        <div className={styles.panelBody}>
          {loading && sessions.length === 0 ? (
            <Skeleton rows={8} />
          ) : sessions.length === 0 ? (
            <EmptyState
              icon="search"
              title="Nessuna ricerca attiva"
              text="Crea una nuova ricerca per definire il perimetro e avviare il gate."
            />
          ) : (
            <div className={styles.indexList}>
              {sessions.map((session) => (
                <button
                  key={session.id}
                  type="button"
                  className={styles.rowCard}
                  onClick={() => navigate(`/ricerche/${session.id}`)}
                >
                  <span>
                    <span className={styles.rowTitle}>{session.title || 'Ricerca senza titolo'}</span>
                    <span className={styles.rowMeta}>
                      {sessionStatusLabel(session.status)}
                      {session.resultCount > 0 ? ` · ${numberFormat.format(session.resultCount)} risultati` : ''}
                      {session.updatedAt ? ` · aggiornata ${dateLabel(session.updatedAt)}` : ''}
                    </span>
                    <span className={styles.promptText}>{session.prompt}</span>
                  </span>
                  <span className={statusClassName(session.status)}>
                    {session.status === 'running' || session.status === 'estimating' ? <span className={styles.pulse} /> : null}
                    {sessionStatusLabel(session.status)}
                  </span>
                </button>
              ))}
            </div>
          )}
        </div>
      </section>
    </main>
  );
}

function EmptyState({ icon, title, text }: { icon: 'search' | 'file-text'; title: string; text: string }) {
  return (
    <div className={styles.emptyState}>
      <span className={styles.emptyIcon}>
        <Icon name={icon} size={28} />
      </span>
      <strong>{title}</strong>
      <p className={styles.hint}>{text}</p>
    </div>
  );
}

function statusClassName(status: MASessionSummary['status']): string {
  const base = styles.statusPill ?? '';
  if (status === 'running' || status === 'estimating') return `${base} ${styles.statusRunning}`;
  if (status === 'completed' || status === 'estimated') return `${base} ${styles.statusDone}`;
  if (status === 'failed') return `${base} ${styles.statusFailed}`;
  return base;
}
