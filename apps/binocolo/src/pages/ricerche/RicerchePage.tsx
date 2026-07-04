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
import type { MASessionVisibility } from '../../api/types';
import styles from './Ricerche.module.css';

const visibilityOptions: { value: MASessionVisibility; label: string }[] = [
  { value: 'active', label: 'Attive' },
  { value: 'archived', label: 'Archiviate' },
  { value: 'deleted', label: 'Cestino' },
];

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
  const [visibility, setVisibility] = useState<MASessionVisibility>('active');
  const [lifecycleBusyId, setLifecycleBusyId] = useState<string | null>(null);
  const [deleteCandidate, setDeleteCandidate] = useState<MASessionSummary | null>(null);
  const [purgeCandidate, setPurgeCandidate] = useState<MASessionSummary | null>(null);

  const loadSessions = useCallback(async (v: MASessionVisibility = visibility) => {
    setLoading(true);
    setError(null);
    try {
      const data = await api.get<MASessionListResponse>(`/binocolo/v1/ma/sessions?visibility=${v}`);
      setSessions(data.items);
    } catch (err) {
      setError(errorLabel(err));
    } finally {
      setLoading(false);
    }
  }, [api, visibility]);

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

  async function archiveSession(id: string) {
    setLifecycleBusyId(id);
    try {
      await api.post<void>(`/binocolo/v1/ma/sessions/${id}/archive`);
      toast('Ricerca archiviata.', 'success');
      await loadSessions();
    } catch (err) {
      toast(errorLabel(err), 'error');
    } finally {
      setLifecycleBusyId(null);
    }
  }

  async function restoreSession(id: string) {
    setLifecycleBusyId(id);
    try {
      await api.post<void>(`/binocolo/v1/ma/sessions/${id}/restore`);
      toast('Ricerca ripristinata.', 'success');
      await loadSessions();
    } catch (err) {
      toast(errorLabel(err), 'error');
    } finally {
      setLifecycleBusyId(null);
    }
  }

  async function trashSession() {
    if (!deleteCandidate) return;
    const id = deleteCandidate.id;
    setLifecycleBusyId(id);
    try {
      await api.delete<void>(`/binocolo/v1/ma/sessions/${id}`);
      toast('Ricerca spostata nel cestino.', 'success');
      setDeleteCandidate(null);
      await loadSessions();
    } catch (err) {
      toast(errorLabel(err), 'error');
    } finally {
      setLifecycleBusyId(null);
    }
  }

  async function purgeSession() {
    if (!purgeCandidate) return;
    const id = purgeCandidate.id;
    setLifecycleBusyId(id);
    try {
      await api.post<void>(`/binocolo/v1/ma/sessions/${id}/purge`);
      toast('Ricerca eliminata definitivamente.', 'success');
      setPurgeCandidate(null);
      await loadSessions();
    } catch (err) {
      toast(errorLabel(err), 'error');
    } finally {
      setLifecycleBusyId(null);
    }
  }

  function handleVisibilityChange(v: MASessionVisibility) {
    setVisibility(v);
    void loadSessions(v);
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
            <h2 id="ricerche-title">
              {visibility === 'active' ? 'Attive' : visibility === 'archived' ? 'Archiviate' : 'Cestino'}
            </h2>
            <p className={styles.hint}>{numberFormat.format(sessions.length)} ricerche disponibili</p>
          </div>
        </div>
        <div className={styles.tabs} role="tablist" aria-label="Filtri visibilità">
          {visibilityOptions.map((opt) => (
            <button
              key={opt.value}
              type="button"
              role="tab"
              aria-selected={visibility === opt.value}
              className={`${styles.tab} ${visibility === opt.value ? styles.tabActive : ''}`}
              onClick={() => handleVisibilityChange(opt.value)}
            >
              {opt.label}
            </button>
          ))}
        </div>
        <div className={styles.panelBody}>
          {loading && sessions.length === 0 ? (
            <Skeleton rows={8} />
          ) : sessions.length === 0 ? (
            <EmptyState
              icon={visibility === 'deleted' ? 'trash' : 'search'}
              title={visibility === 'active' ? 'Nessuna ricerca attiva' : visibility === 'archived' ? 'Nessun archivio' : 'Cestino vuoto'}
              text={visibility === 'active' ? 'Crea una nuova ricerca per definire il perimetro e avviare il gate.' : 'Nessun elemento trovato con questo filtro.'}
            />
          ) : (
            <div className={styles.indexList}>
              {sessions.map((session) => (
                <div key={session.id} className={styles.rowCard} role="button" tabIndex={0} onClick={() => visibility !== 'deleted' && navigate(`/ricerche/${session.id}`)}>
                  <span>
                    <span className={styles.inlineActions}>
                      <span className={styles.rowTitle}>{session.title || 'Ricerca senza titolo'}</span>
                      {session.initiativeTitle ? (
                        <span
                          className={`${styles.badge} ${styles.badgeLav} ${styles.initiativeBadge}`}
                          onClick={(event) => {
                            event.stopPropagation();
                            navigate(`/iniziative/${session.initiativeId}`);
                          }}
                          onKeyDown={(event) => {
                            if (event.key === 'Enter' || event.key === ' ') {
                              event.preventDefault();
                              event.stopPropagation();
                              navigate(`/iniziative/${session.initiativeId}`);
                            }
                          }}
                          role="button"
                          tabIndex={0}
                        >
                          {session.initiativeTitle}
                        </span>
                      ) : visibility === 'active' ? (
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
                      ) : null}
                    </span>
                    <span className={styles.rowMeta}>
                      {sessionStatusLabel(session.status)}
                      {session.resultCount > 0 ? ` · ${numberFormat.format(session.resultCount)} risultati` : ''}
                      {session.updatedAt ? ` · aggiornata ${dateLabel(session.updatedAt)}` : ''}
                    </span>
                    <span className={styles.promptText}>{session.prompt}</span>
                  </span>
                  <div className={styles.inlineActions} onClick={(e) => e.stopPropagation()}>
                    {visibility === 'active' && (
                      <Button
                        variant="secondary"
                        size="sm"
                        onClick={() => void archiveSession(session.id)}
                        loading={lifecycleBusyId === session.id}
                        title="Archivia"
                      >
                        <Icon name="archive" size={14} />
                      </Button>
                    )}
                    {(visibility === 'archived' || visibility === 'deleted') && (
                      <Button
                        variant="secondary"
                        size="sm"
                        onClick={() => void restoreSession(session.id)}
                        loading={lifecycleBusyId === session.id}
                        title="Ripristina"
                      >
                        <Icon name="refresh-cw" size={14} />
                      </Button>
                    )}
                    {visibility !== 'deleted' && (
                      <Button
                        variant="secondary"
                        size="sm"
                        onClick={() => setDeleteCandidate(session)}
                        loading={lifecycleBusyId === session.id}
                        title="Sposta nel cestino"
                      >
                        <Icon name="trash" size={14} />
                      </Button>
                    )}
                    {visibility === 'deleted' && (
                      <Button
                        variant="danger"
                        size="sm"
                        onClick={() => setPurgeCandidate(session)}
                        loading={lifecycleBusyId === session.id}
                        title="Elimina definitivamente"
                      >
                        <Icon name="trash" size={14} />
                      </Button>
                    )}
                  </div>
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

      <Modal
        open={deleteCandidate !== null}
        onClose={() => setDeleteCandidate(null)}
        title="Sposta nel cestino"
        size="sm"
        dismissible={!lifecycleBusyId}
      >
        <div className={styles.stack}>
          <p className={styles.hint}>
            La ricerca «{deleteCandidate?.title}» verrà spostata nel cestino. Potrai ripristinarla entro 30 giorni.
          </p>
          <div className={styles.modalActions}>
            <Button variant="danger" onClick={() => void trashSession()} loading={lifecycleBusyId === deleteCandidate?.id}>
              Sposta nel cestino
            </Button>
            <Button variant="secondary" onClick={() => setDeleteCandidate(null)} disabled={!!lifecycleBusyId}>
              Annulla
            </Button>
          </div>
        </div>
      </Modal>

      <Modal
        open={purgeCandidate !== null}
        onClose={() => setPurgeCandidate(null)}
        title="Elimina definitivamente"
        size="sm"
        dismissible={!lifecycleBusyId}
      >
        <div className={styles.stack}>
          <p className={styles.hint}>
            Sei sicuro di voler eliminare definitivamente la ricerca «{purgeCandidate?.title}»? L'azione non è reversibile.
          </p>
          <div className={styles.modalActions}>
            <Button variant="danger" onClick={() => void purgeSession()} loading={lifecycleBusyId === purgeCandidate?.id}>
              Elimina definitivamente
            </Button>
            <Button variant="secondary" onClick={() => setPurgeCandidate(null)} disabled={!!lifecycleBusyId}>
              Annulla
            </Button>
          </div>
        </div>
      </Modal>
    </main>
  );
}

function EmptyState({ icon, title, text }: { icon: 'search' | 'file-text' | 'trash'; title: string; text: string }) {
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

