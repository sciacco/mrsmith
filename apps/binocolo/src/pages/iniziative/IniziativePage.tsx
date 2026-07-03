import { Button, Icon, Modal, Skeleton, useToast } from '@mrsmith/ui';
import { useCallback, useEffect, useState, type ReactNode } from 'react';
import { useNavigate } from 'react-router-dom';
import { useApiClient } from '../../api/client';
import type { MAInitiative, MAInitiativeListResponse, MAInitiativeSummary } from '../../api/types';
import { relativeDate, errorLabel } from '../ricerche/helpers';
import styles from './Iniziative.module.css';

const STATE_ORDER: Array<{ key: string; label: string }> = [
  { key: 'da_contattare', label: 'Da contattare' },
  { key: 'contattata', label: 'Contattata' },
  { key: 'in_dialogo', label: 'In dialogo' },
  { key: 'approfondimento', label: 'Approfondimento' },
  { key: 'offerta', label: 'Offerta' },
  { key: 'chiusa', label: 'Chiuse' },
];

export function IniziativePage() {
  const api = useApiClient();
  const navigate = useNavigate();
  const { toast } = useToast();
  const [items, setItems] = useState<MAInitiativeSummary[]>([]);
  const [showArchived, setShowArchived] = useState(false);
  const [archivedCount, setArchivedCount] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [modalOpen, setModalOpen] = useState(false);
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);
  const [lifecycleBusy, setLifecycleBusy] = useState<string | null>(null);

  const load = useCallback(
    async (archived: boolean) => {
      setLoading(true);
      setError(null);
      try {
        const query = archived ? '?archived=1' : '';
        const data = await api.get<MAInitiativeListResponse>(`/binocolo/v1/ma/initiatives${query}`);
        setItems(data.items);
        if (!archived) {
          void api
            .get<MAInitiativeListResponse>('/binocolo/v1/ma/initiatives?archived=1')
            .then((archivedData) => setArchivedCount(archivedData.items.length))
            .catch(() => setArchivedCount(null));
        }
      } catch (err) {
        setError(errorLabel(err));
      } finally {
        setLoading(false);
      }
    },
    [api],
  );

  useEffect(() => {
    void load(showArchived);
  }, [load, showArchived]);

  const openCreateModal = () => {
    setTitle('');
    setDescription('');
    setCreateError(null);
    setModalOpen(true);
  };

  const submitCreate = async () => {
    const trimmedTitle = title.trim();
    if (!trimmedTitle) {
      setCreateError('Il titolo è obbligatorio.');
      return;
    }
    setCreating(true);
    setCreateError(null);
    try {
      const created = await api.post<MAInitiative>('/binocolo/v1/ma/initiatives', {
        title: trimmedTitle,
        description: description.trim(),
      });
      setModalOpen(false);
      navigate(`/iniziative/${created.id}`);
    } catch (err) {
      setCreateError(errorLabel(err));
    } finally {
      setCreating(false);
    }
  };

  return (
    <main className={styles.page}>
      <header className={styles.header}>
        <div>
          <span className={styles.eyebrow}>Binocolo</span>
          <h1>Iniziative</h1>
          <p className={styles.subtitle}>
            Un&rsquo;iniziativa raccoglie le ricerche di uno stesso obiettivo di acquisizione e le aziende in
            lavorazione.
          </p>
        </div>
        <div className={styles.inlineActions}>
          <Button onClick={openCreateModal} leftIcon={<Icon name="plus" />}>
            Nuova iniziativa
          </Button>
        </div>
      </header>

      {error ? (
        <div className={styles.danger} role="alert">
          <Icon name="triangle-alert" size={18} />
          <span>{error}</span>
        </div>
      ) : null}

      {loading && items.length === 0 ? (
        <Skeleton rows={4} />
      ) : items.length === 0 ? (
        showArchived ? (
          <EmptyState
            title="Nessuna iniziativa archiviata."
            text="Le iniziative archiviate restano consultabili qui."
          />
        ) : (
          <EmptyState
            title="Nessuna iniziativa."
            text="Le iniziative raccolgono le ricerche di uno stesso obiettivo di acquisizione e le aziende promosse in lavorazione."
            action={
              <Button onClick={openCreateModal} leftIcon={<Icon name="plus" />}>
                Nuova iniziativa
              </Button>
            }
          />
        )
      ) : (
        <div className={styles.list}>
          {items.map((item) => (
            <IniziativaCard
              key={item.id}
              item={item}
              onOpen={() => navigate(`/iniziative/${item.id}`)}
              onArchive={item.archivedAt ? undefined : async () => {
                setLifecycleBusy(item.id);
                try {
                  await api.post(`/binocolo/v1/ma/initiatives/${item.id}/archive`, {});
                  toast('Iniziativa archiviata.', 'success');
                  await load(showArchived);
                } catch (err) {
                  toast(errorLabel(err), 'error');
                } finally {
                  setLifecycleBusy(null);
                }
              }}
              onRestore={item.archivedAt ? async () => {
                setLifecycleBusy(item.id);
                try {
                  await api.post(`/binocolo/v1/ma/initiatives/${item.id}/restore`, {});
                  toast('Iniziativa ripristinata.', 'success');
                  await load(showArchived);
                } catch (err) {
                  toast(errorLabel(err), 'error');
                } finally {
                  setLifecycleBusy(null);
                }
              } : undefined}
              busy={lifecycleBusy === item.id}
            />
          ))}
        </div>
      )}

      {!showArchived ? (
        <p className={styles.archiveLink}>
          <button type="button" className={styles.linkButton} onClick={() => setShowArchived(true)}>
            Archivio{archivedCount != null ? ` (${archivedCount})` : ''}
          </button>
        </p>
      ) : (
        <p className={styles.archiveLink}>
          <button type="button" className={styles.linkButton} onClick={() => setShowArchived(false)}>
            Torna alle iniziative attive
          </button>
        </p>
      )}

      <Modal open={modalOpen} onClose={() => setModalOpen(false)} title="Nuova iniziativa">
        <p className={styles.modalSub}>Un obiettivo largo, non una ricerca: le ricerche si agganciano dopo.</p>
        <div className={styles.field}>
          <label htmlFor="initiative-title">Titolo</label>
          <input
            id="initiative-title"
            className={styles.input}
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            maxLength={120}
            placeholder="Acquisizione MSP 2026"
          />
        </div>
        <div className={styles.field}>
          <label htmlFor="initiative-description">Descrizione (opzionale)</label>
          <textarea
            id="initiative-description"
            className={styles.textarea}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            maxLength={500}
            placeholder="Perimetro e razionale in una riga…"
          />
        </div>
        {createError ? (
          <div className={styles.danger} role="alert">
            <Icon name="triangle-alert" size={18} />
            <span>{createError}</span>
          </div>
        ) : null}
        <div className={styles.modalActions}>
          <Button onClick={() => void submitCreate()} loading={creating}>
            Crea
          </Button>
          <Button variant="secondary" onClick={() => setModalOpen(false)}>
            Annulla
          </Button>
        </div>
      </Modal>
    </main>
  );
}

function IniziativaCard({ item, onOpen, onArchive, onRestore, busy }: { item: MAInitiativeSummary; onOpen: () => void; onArchive?: () => void; onRestore?: () => void; busy?: boolean }) {
  const isArchived = !!item.archivedAt;
  return (
    <div className={styles.card}>
      <div className={styles.cardTop}>
        <button type="button" className={styles.cardTitle} onClick={onOpen}>
          {item.title}
        </button>
        <span className={styles.hint}>aggiornata {relativeDate(item.updatedAt)}</span>
      </div>
      {item.description ? <p className={styles.cardDesc}>{item.description}</p> : null}
      <div className={styles.stateLine}>
        {STATE_ORDER.map((state) => {
          const count = item.counts[state.key] ?? 0;
          return (
            <button
              key={state.key}
              type="button"
              className={`${styles.statePill}${count > 0 ? ` ${styles.statePillActive}` : ''}`}
              onClick={onOpen}
              title={`Vai al board filtrato su ${state.label}`}
            >
              {state.label} <b>{count}</b>
            </button>
          );
        })}
      </div>
      <div className={styles.cardFoot}>
        <span>
          {item.sessionCount} {item.sessionCount === 1 ? 'ricerca agganciata' : 'ricerche agganciate'}
        </span>
        {item.lastActivityEvent ? (
          <>
            <span>&middot;</span>
            <span>Ultima attività: {item.lastActivityEvent}</span>
          </>
        ) : null}
        {isArchived && onRestore ? (
          <>
            <span className={styles.hint}>&middot;</span>
            <Button variant="secondary" size="sm" onClick={onRestore} loading={busy}>
              Ripristina
            </Button>
          </>
        ) : null}
        {!isArchived && onArchive ? (
          <>
            <span className={styles.hint}>&middot;</span>
            <Button variant="secondary" size="sm" onClick={onArchive} loading={busy}>
              Archivia
            </Button>
          </>
        ) : null}
      </div>
    </div>
  );
}

function EmptyState({ title, text, action }: { title: string; text: string; action?: ReactNode }) {
  return (
    <div className={styles.emptyState}>
      <span className={styles.emptyIcon}>
        <Icon name="search" size={28} />
      </span>
      <strong>{title}</strong>
      <p className={styles.hint}>{text}</p>
      {action}
    </div>
  );
}
