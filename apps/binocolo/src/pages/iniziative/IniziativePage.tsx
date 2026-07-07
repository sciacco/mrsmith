import { Button, Icon, Modal, Skeleton, useToast } from '@mrsmith/ui';
import { useCallback, useEffect, useState, type ReactNode } from 'react';
import { useNavigate } from 'react-router-dom';
import { useApiClient } from '../../api/client';
import type { MAInitiative, MAInitiativeListResponse, MAInitiativeSummary } from '../../api/types';
import { relativeDate, errorLabel } from '../ricerche/helpers';
import styles from './Iniziative.module.css';

type InitiativeVisibility = 'active' | 'archived' | 'deleted';

const visibilityOptions: { value: InitiativeVisibility; label: string }[] = [
  { value: 'active', label: 'Attive' },
  { value: 'archived', label: 'Archiviate' },
  { value: 'deleted', label: 'Cestino' },
];

const visibilityTitle: Record<InitiativeVisibility, string> = {
  active: 'Attive',
  archived: 'Archiviate',
  deleted: 'Cestino',
};

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
  const [visibility, setVisibility] = useState<InitiativeVisibility>('active');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [modalOpen, setModalOpen] = useState(false);
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);
  const [lifecycleBusy, setLifecycleBusy] = useState<string | null>(null);
  const [deleteCandidate, setDeleteCandidate] = useState<MAInitiativeSummary | null>(null);
  const [purgeCandidate, setPurgeCandidate] = useState<MAInitiativeSummary | null>(null);
  const [purgeConfirm, setPurgeConfirm] = useState('');
  const [renameCandidate, setRenameCandidate] = useState<MAInitiativeSummary | null>(null);
  const [renameTitle, setRenameTitle] = useState('');
  const [renameError, setRenameError] = useState<string | null>(null);
  const [renaming, setRenaming] = useState(false);

  const load = useCallback(
    async (v: InitiativeVisibility) => {
      setLoading(true);
      setError(null);
      try {
        const data = await api.get<MAInitiativeListResponse>(`/binocolo/v1/ma/initiatives?visibility=${v}`);
        setItems(data.items);
      } catch (err) {
        setError(errorLabel(err));
      } finally {
        setLoading(false);
      }
    },
    [api],
  );

  useEffect(() => {
    void load(visibility);
  }, [load, visibility]);

  function handleVisibilityChange(next: InitiativeVisibility) {
    setVisibility(next);
  }

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
    if (trimmedTitle.length > 120) {
      setCreateError('Il titolo può contenere al massimo 120 caratteri.');
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

  async function archiveInitiative(id: string) {
    setLifecycleBusy(id);
    try {
      await api.post<void>(`/binocolo/v1/ma/initiatives/${id}/archive`, {});
      toast('Iniziativa archiviata.', 'success');
      await load(visibility);
    } catch (err) {
      toast(errorLabel(err), 'error');
    } finally {
      setLifecycleBusy(null);
    }
  }

  async function restoreInitiative(id: string) {
    setLifecycleBusy(id);
    try {
      await api.post<void>(`/binocolo/v1/ma/initiatives/${id}/restore`, {});
      toast('Iniziativa ripristinata.', 'success');
      await load(visibility);
    } catch (err) {
      toast(errorLabel(err), 'error');
    } finally {
      setLifecycleBusy(null);
    }
  }

  async function trashInitiative() {
    if (!deleteCandidate) return;
    const id = deleteCandidate.id;
    setLifecycleBusy(id);
    try {
      await api.delete<void>(`/binocolo/v1/ma/initiatives/${id}`);
      toast('Iniziativa spostata nel cestino.', 'success');
      setDeleteCandidate(null);
      await load(visibility);
    } catch (err) {
      toast(errorLabel(err), 'error');
    } finally {
      setLifecycleBusy(null);
    }
  }

  async function purgeInitiative() {
    if (!purgeCandidate || purgeConfirm !== 'ELIMINA') return;
    const id = purgeCandidate.id;
    setLifecycleBusy(id);
    try {
      await api.post<void>(`/binocolo/v1/ma/initiatives/${id}/purge`, {});
      toast('Iniziativa eliminata definitivamente.', 'success');
      setPurgeCandidate(null);
      setPurgeConfirm('');
      await load(visibility);
    } catch (err) {
      toast(errorLabel(err), 'error');
    } finally {
      setLifecycleBusy(null);
    }
  }

  function openRenameModal(item: MAInitiativeSummary) {
    setRenameCandidate(item);
    setRenameTitle(item.title);
    setRenameError(null);
  }

  async function renameInitiative() {
    if (!renameCandidate) return;
    const trimmedTitle = renameTitle.trim();
    if (!trimmedTitle) {
      setRenameError('Il titolo è obbligatorio.');
      return;
    }
    if (trimmedTitle.length > 120) {
      setRenameError('Il titolo può contenere al massimo 120 caratteri.');
      return;
    }
    setRenaming(true);
    setRenameError(null);
    try {
      await api.patch<MAInitiative>(`/binocolo/v1/ma/initiatives/${renameCandidate.id}`, { title: trimmedTitle });
      toast('Titolo aggiornato.', 'success');
      setRenameCandidate(null);
      await load(visibility);
    } catch (err) {
      setRenameError(errorLabel(err));
    } finally {
      setRenaming(false);
    }
  }

  const emptyCopy =
    visibility === 'active'
      ? {
          icon: 'search' as const,
          title: 'Nessuna iniziativa.',
          text: 'Le iniziative raccolgono le ricerche di uno stesso obiettivo di acquisizione e le aziende promosse in lavorazione.',
        }
      : visibility === 'archived'
        ? { icon: 'archive' as const, title: 'Nessuna iniziativa archiviata.', text: 'Le iniziative archiviate restano consultabili qui.' }
        : { icon: 'trash' as const, title: 'Cestino vuoto.', text: 'Nessuna iniziativa nel cestino.' };

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

      <section className={styles.panel} aria-labelledby="iniziative-title">
        <div className={styles.panelHeader}>
          <div>
            <h2 id="iniziative-title">{visibilityTitle[visibility]}</h2>
            <p className={styles.hint}>Gestisci visibilità e titolo delle iniziative.</p>
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
          {loading && items.length === 0 ? (
            <Skeleton rows={4} />
          ) : items.length === 0 ? (
            <EmptyState
              icon={emptyCopy.icon}
              title={emptyCopy.title}
              text={emptyCopy.text}
              action={
                visibility === 'active' ? (
                  <Button onClick={openCreateModal} leftIcon={<Icon name="plus" />}>
                    Nuova iniziativa
                  </Button>
                ) : undefined
              }
            />
          ) : (
            <div className={styles.list}>
              {items.map((item) => (
                <IniziativaCard
                  key={item.id}
                  item={item}
                  visibility={visibility}
                  onOpen={() => navigate(`/iniziative/${item.id}`)}
                  onArchive={() => void archiveInitiative(item.id)}
                  onRestore={() => void restoreInitiative(item.id)}
                  onTrash={() => setDeleteCandidate(item)}
                  onPurge={() => {
                    setPurgeCandidate(item);
                    setPurgeConfirm('');
                  }}
                  onRename={() => openRenameModal(item)}
                  busy={lifecycleBusy === item.id}
                />
              ))}
            </div>
          )}
        </div>
      </section>

      <Modal open={modalOpen} onClose={() => setModalOpen(false)} title="Nuova iniziativa">
        <p className={styles.modalSub}>Un obiettivo largo, non una ricerca: le ricerche entrano qui quando condividono lo stesso filone.</p>
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

      <Modal open={renameCandidate !== null} onClose={() => setRenameCandidate(null)} title="Rinomina iniziativa" size="sm" dismissible={!renaming}>
        <div className={styles.field}>
          <label htmlFor="initiative-rename-title">Titolo</label>
          <input
            id="initiative-rename-title"
            className={styles.input}
            value={renameTitle}
            onChange={(event) => setRenameTitle(event.target.value)}
            maxLength={120}
          />
        </div>
        {renameError ? (
          <div className={styles.danger} role="alert">
            <Icon name="triangle-alert" size={18} />
            <span>{renameError}</span>
          </div>
        ) : null}
        <div className={styles.modalActions}>
          <Button onClick={() => void renameInitiative()} loading={renaming}>
            Salva modifiche
          </Button>
          <Button variant="secondary" onClick={() => setRenameCandidate(null)} disabled={renaming}>
            Annulla
          </Button>
        </div>
      </Modal>

      <Modal open={deleteCandidate !== null} onClose={() => setDeleteCandidate(null)} title="Sposta nel cestino" size="sm" dismissible={!lifecycleBusy}>
        <div className={styles.stack}>
          <p className={styles.hint}>
            L&rsquo;iniziativa «{deleteCandidate?.title}» verrà spostata nel cestino. Le ricerche collegate non vengono eliminate.
          </p>
          <div className={styles.modalActions}>
            <Button variant="danger" onClick={() => void trashInitiative()} loading={lifecycleBusy === deleteCandidate?.id}>
              Sposta nel cestino
            </Button>
            <Button variant="secondary" onClick={() => setDeleteCandidate(null)} disabled={!!lifecycleBusy}>
              Annulla
            </Button>
          </div>
        </div>
      </Modal>

      <Modal open={purgeCandidate !== null} onClose={() => setPurgeCandidate(null)} title="Elimina definitivamente" size="sm" dismissible={!lifecycleBusy}>
        <div className={styles.stack}>
          <p className={styles.hint}>
            Sei sicuro di voler eliminare definitivamente l&rsquo;iniziativa «{purgeCandidate?.title}»? L&rsquo;azione non è reversibile.
          </p>
          <div className={styles.field}>
            <label htmlFor="initiative-purge-confirm">Scrivi ELIMINA per confermare</label>
            <input
              id="initiative-purge-confirm"
              className={styles.input}
              value={purgeConfirm}
              onChange={(event) => setPurgeConfirm(event.target.value)}
              autoComplete="off"
            />
          </div>
          <div className={styles.modalActions}>
            <Button
              variant="danger"
              onClick={() => void purgeInitiative()}
              loading={lifecycleBusy === purgeCandidate?.id}
              disabled={purgeConfirm !== 'ELIMINA'}
            >
              Elimina definitivamente
            </Button>
            <Button variant="secondary" onClick={() => setPurgeCandidate(null)} disabled={!!lifecycleBusy}>
              Annulla
            </Button>
          </div>
        </div>
      </Modal>
    </main>
  );
}

function IniziativaCard({
  item,
  visibility,
  onOpen,
  onArchive,
  onRestore,
  onTrash,
  onPurge,
  onRename,
  busy,
}: {
  item: MAInitiativeSummary;
  visibility: InitiativeVisibility;
  onOpen: () => void;
  onArchive: () => void;
  onRestore: () => void;
  onTrash: () => void;
  onPurge: () => void;
  onRename: () => void;
  busy?: boolean;
}) {
  return (
    <div className={styles.card}>
      <div className={styles.cardTop}>
        <button type="button" className={styles.cardTitle} onClick={visibility === 'deleted' ? undefined : onOpen} disabled={visibility === 'deleted'}>
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
              disabled={visibility === 'deleted'}
              title={`Vai al board filtrato su ${state.label}`}
            >
              {state.label} <b>{count}</b>
            </button>
          );
        })}
      </div>
      <div className={styles.cardFoot}>
        <span>
          {item.sessionCount} {item.sessionCount === 1 ? 'ricerca collegata' : 'ricerche collegate'}
        </span>
        {item.lastActivityEvent ? (
          <>
            <span>&middot;</span>
            <span>Ultima attività: {item.lastActivityEvent}</span>
          </>
        ) : null}
        <span className={styles.actionSpacer} />
        {visibility === 'active' ? (
          <>
            <Button variant="secondary" size="sm" onClick={onRename} disabled={busy}>
              Rinomina
            </Button>
            <Button variant="secondary" size="sm" onClick={onArchive} loading={busy}>
              Archivia
            </Button>
            <Button variant="secondary" size="sm" onClick={onTrash} loading={busy}>
              Sposta nel cestino
            </Button>
          </>
        ) : null}
        {visibility === 'archived' ? (
          <Button variant="secondary" size="sm" onClick={onRestore} loading={busy}>
            Ripristina
          </Button>
        ) : null}
        {visibility === 'deleted' ? (
          <>
            <Button variant="secondary" size="sm" onClick={onRestore} loading={busy}>
              Ripristina
            </Button>
            <Button variant="danger" size="sm" onClick={onPurge} loading={busy}>
              Elimina definitivamente
            </Button>
          </>
        ) : null}
      </div>
    </div>
  );
}

function EmptyState({ icon, title, text, action }: { icon: 'search' | 'archive' | 'trash'; title: string; text: string; action?: ReactNode }) {
  return (
    <div className={styles.emptyState}>
      <span className={styles.emptyIcon}>
        <Icon name={icon} size={28} />
      </span>
      <strong>{title}</strong>
      <p className={styles.hint}>{text}</p>
      {action}
    </div>
  );
}
