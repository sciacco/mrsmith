import { Button, Icon, Modal, Skeleton, useToast } from '@mrsmith/ui';
import { useCallback, useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useApiClient } from '../../api/client';
import type { MAInitiativeListResponse, MAInitiativeSummary, MASessionListResponse, MASessionSummary } from '../../api/types';
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
  const { toast } = useToast();
  const [sessions, setSessions] = useState<MASessionSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [initiatives, setInitiatives] = useState<MAInitiativeSummary[]>([]);
  const [attachFor, setAttachFor] = useState<MASessionSummary | null>(null);
  const [attachInitiativeId, setAttachInitiativeId] = useState('');
  const [attachBusy, setAttachBusy] = useState(false);

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

  const loadInitiatives = useCallback(async () => {
    try {
      const data = await api.get<MAInitiativeListResponse>('/binocolo/v1/ma/initiatives');
      setInitiatives(data.items);
    } catch (err) {
      setError(errorLabel(err));
    }
  }, [api]);

  useEffect(() => {
    void loadSessions();
    void loadInitiatives();
  }, [loadSessions, loadInitiatives]);

  async function attachInitiative() {
    if (!attachFor || !attachInitiativeId) return;
    setAttachBusy(true);
    try {
      await api.post<void>(`/binocolo/v1/ma/sessions/${attachFor.id}/initiative`, { initiativeId: attachInitiativeId });
      setAttachFor(null);
      setAttachInitiativeId('');
      await loadSessions();
      toast('Ricerca agganciata all’iniziativa.', 'success');
    } catch (err) {
      toast(errorLabel(err), 'error');
    } finally {
      setAttachBusy(false);
    }
  }

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
                <div key={session.id} className={styles.rowCard} role="button" tabIndex={0} onClick={() => navigate(`/ricerche/${session.id}`)}>
                  <span>
                    <span className={styles.inlineActions}>
                      <span className={styles.rowTitle}>{session.title || 'Ricerca senza titolo'}</span>
                      {session.initiativeTitle ? (
                        <span className={`${styles.badge} ${styles.badgeLav}`}>{session.initiativeTitle}</span>
                      ) : (
                        <button
                          type="button"
                          className={styles.linkButton}
                          onClick={(event) => {
                            event.stopPropagation();
                            setAttachFor(session);
                            setAttachInitiativeId('');
                          }}
                        >
                          Aggancia a iniziativa…
                        </button>
                      )}
                    </span>
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
                </div>
              ))}
            </div>
          )}
        </div>
      </section>

      <Modal
        open={attachFor !== null}
        onClose={() => setAttachFor(null)}
        title="Aggancia a iniziativa"
        size="sm"
        dismissible={!attachBusy}
      >
        <div className={styles.stack}>
          <p className={styles.hint}>
            Le aziende con almeno una stella in «{attachFor?.title || 'questa ricerca'}» entreranno nella lavorazione dell'iniziativa scelta.
          </p>
          <select
            className={styles.select}
            value={attachInitiativeId}
            onChange={(event) => setAttachInitiativeId(event.target.value)}
          >
            <option value="">Seleziona iniziativa</option>
            {initiatives.map((item) => (
              <option key={item.id} value={item.id}>{item.title}</option>
            ))}
          </select>
          <div className={styles.modalActions}>
            <Button onClick={() => void attachInitiative()} loading={attachBusy} disabled={!attachInitiativeId}>
              Aggancia
            </Button>
            <Button variant="secondary" onClick={() => setAttachFor(null)} disabled={attachBusy}>
              Annulla
            </Button>
          </div>
        </div>
      </Modal>
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
