import { useEffect, useMemo, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { Button, Icon, Skeleton, useToast } from '@mrsmith/ui';
import type { Segnalazione, SegnalazioneState } from '../../api/types';
import { useSegnalazioneMutations, useSegnalazioni } from '../../hooks/useSegnalazioni';
import { errorLabel } from '../ricerche/helpers';
import { SegnalazioneModal } from '../../components/segnalazioni/SegnalazioneModal';
import { SegnalazioniBoard } from './SegnalazioniBoard';
import { SegnalazioneDrawer } from './SegnalazioneDrawer';
import board from '../iniziative/board/board.module.css';
import styles from './Segnalazioni.module.css';

function matches(item: Segnalazione, needle: string): boolean {
  if (!needle) return true;
  const haystack = [item.name, item.website, item.location, item.fiscalId, item.contacts, item.notes, item.createdByEmail ?? '']
    .join('\n')
    .toLowerCase();
  return haystack.includes(needle);
}

export function SegnalazioniPage() {
  const query = useSegnalazioni();
  const { setState } = useSegnalazioneMutations();
  const { toast } = useToast();
  const [searchParams, setSearchParams] = useSearchParams();
  const [search, setSearch] = useState('');
  const [createOpen, setCreateOpen] = useState(false);

  const items = query.data ?? [];
  const selectedId = searchParams.get('id');
  const selected = selectedId ? items.find((item) => item.id === selectedId) ?? null : null;

  // Link condivisibile: /segnalazioni?id=<uuid>. Se l'id non esiste, avviso e
  // rimozione del parametro una volta arrivati i dati.
  useEffect(() => {
    if (!selectedId || !query.data || selected) return;
    toast('Segnalazione non trovata.', 'error');
    setSearchParams((current) => { current.delete('id'); return current; }, { replace: true });
  }, [selectedId, query.data, selected, toast, setSearchParams]);

  const open = (item: Segnalazione) => setSearchParams((current) => { current.set('id', item.id); return current; });
  const close = () => setSearchParams((current) => { current.delete('id'); return current; });

  const visible = useMemo(() => {
    const needle = search.trim().toLowerCase();
    return items.filter((item) => matches(item, needle));
  }, [items, search]);

  const move = (item: Segnalazione, state: SegnalazioneState) => {
    setState.mutate({ id: item.id, state }, { onError: (error) => toast(errorLabel(error), 'error') });
  };

  return (
    <main className={board.page}>
      <div className={board.board}>
        <div className={board.topbar}>
          <div className={board.titleBlock}>
            <span className={board.kicker}>Segnalazioni · {items.length} totali</span>
            <h1 className={board.title}>Segnalazioni</h1>
          </div>
          <div className={board.viewMatrix}>
            <label className={board.search}>
              <Icon name="search" size={14} />
              <input placeholder="Cerca nelle segnalazioni…" value={search} onChange={(event) => setSearch(event.target.value)} aria-label="Cerca nelle segnalazioni" />
            </label>
            <Button leftIcon={<Icon name="plus" />} onClick={() => setCreateOpen(true)}>Nuova segnalazione</Button>
          </div>
        </div>

        {query.isLoading ? (
          <Skeleton rows={6} />
        ) : query.isError ? (
          <div className={styles.errorBox} role="alert">
            <Icon name="triangle-alert" size={18} />
            <span>{errorLabel(query.error)}</span>
            <Button variant="secondary" size="sm" onClick={() => void query.refetch()}>Riprova</Button>
          </div>
        ) : items.length === 0 ? (
          <div className={styles.empty}>
            <div className={styles.emptyIcon}><Icon name="file-plus" size={32} /></div>
            <p className={styles.emptyTitle}>Nessuna segnalazione</p>
            <p className={styles.emptyText}>Registra la prima azienda o opportunità con le informazioni che conosci.</p>
            <Button leftIcon={<Icon name="plus" />} onClick={() => setCreateOpen(true)}>Nuova segnalazione</Button>
          </div>
        ) : visible.length === 0 ? (
          <div className={board.colEmpty} style={{ padding: 'var(--space-16) 0' }}>
            Nessuna segnalazione corrisponde alla ricerca.{' '}
            <button type="button" className={board.actionLink} style={{ border: 'none', background: 'none', cursor: 'pointer' }} onClick={() => setSearch('')}>
              Azzera ricerca
            </button>
          </div>
        ) : (
          <SegnalazioniBoard items={visible} onOpen={open} onMove={move} />
        )}
      </div>

      <SegnalazioneModal open={createOpen} onClose={() => setCreateOpen(false)} />
      {selected ? <SegnalazioneDrawer item={selected} onClose={close} onSaved={() => toast('Contenuti salvati', 'success')} /> : null}
    </main>
  );
}
