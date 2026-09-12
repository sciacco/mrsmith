import { useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { Button, Icon, SearchInput, SingleSelect, Skeleton } from '@mrsmith/ui';
import { formatLocalDate, formatNumber } from '@mrsmith/format';
import { useNeeds, useSaveNeed } from '../../api/queries';
import type { Need, NeedStatus } from '../../api/types';
import { NEED_STATES, needInput } from '../../lib/needs';
import { NeedEditorModal } from '../../components/needs/NeedEditorModal';
import { ErrorPanel } from '../../components/events/ErrorPanel';
import { describeApiError } from '../../components/events/apiErrors';
// Stessa board di Binocolo; qui il cambio stato è esplicito, senza trascinamento.
import board from '../../../../binocolo/src/pages/iniziative/board/board.module.css';
import styles from './NeedsPage.module.css';

export function NeedsPage() {
  const needs = useNeeds();
  const save = useSaveNeed();
  const navigate = useNavigate();
  const [creating, setCreating] = useState(false);
  const [query, setQuery] = useState('');
  const [error, setError] = useState<string | null>(null);
  const visible = (needs.data ?? []).filter((n) =>
    `${n.description} ${n.notes ?? ''} ${n.skillAreas.map((a) => a.name).join(' ')}`.toLocaleLowerCase('it').includes(query.toLocaleLowerCase('it')),
  );

  async function move(need: Need, status: NeedStatus | null) {
    if (!status || status === need.status) return;
    setError(null);
    try { await save.mutateAsync({ id: need.id, input: { ...needInput(need), status } }); }
    catch (e) { setError(describeApiError(e, 'Cambio stato non riuscito')); }
  }

  return (
    <div className={styles.page}>
      <header className={styles.header}>
        <h1 className={styles.title}>Esigenze formative</h1>
        <Button onClick={() => setCreating(true)} leftIcon={<Icon name="plus" size={16} />}>Nuova esigenza</Button>
      </header>
      {needs.isPending ? <Skeleton rows={5} /> : needs.isError ? (
        <div><ErrorPanel message={describeApiError(needs.error, 'Esigenze non disponibili')} /><Button variant="secondary" onClick={() => void needs.refetch()}>Riprova</Button></div>
      ) : (needs.data ?? []).length === 0 ? (
        <div className={styles.empty}>
          <div className={styles.emptyIcon}><Icon name="target" size={32} /></div>
          <strong>Nessuna esigenza formativa</strong>
          <p className={styles.meta}>Crea un’esigenza e raccogli i corsi candidati. Puoi collegare le richieste in seguito.</p>
          <Button onClick={() => setCreating(true)}>Nuova esigenza</Button>
        </div>
      ) : (
        <>
          <SearchInput value={query} onChange={setQuery} placeholder="Cerca esigenza o area…" />
          {error && <ErrorPanel message={error} />}
          <div className={`${board.cols} ${styles.columns}`}>
            {NEED_STATES.map((state) => {
              const items = visible.filter((n) => n.status === state.value);
              return (
                <section key={state.value} className={`${board.kcol} ${styles.column} ${styles[state.value]}`} aria-label={state.label}>
                  <div className={board.kcolHead}>
                    <span className={board.dot} aria-hidden="true" />
                    <span className={board.nm}>{state.label}</span>
                    <span className={board.cnt}>{formatNumber(items.length)}</span>
                  </div>
                  <div className={board.kcolBody}>
                    {items.map((need) => (
                      <article key={need.id} className={`${board.kcard} ${styles.card}`}>
                        <Link to={`/esigenze/${need.id}`} className={styles.cardTitle}>{need.description}</Link>
                        <p className={styles.meta}>{formatNumber(need.candidatesCount)} candidati · {formatNumber(need.requestsCount)} richieste</p>
                        {need.finalCourseTitle && <p className={styles.meta}>Definitivo: {need.finalCourseTitle}</p>}
                        {need.reminderText && <p className={styles.meta}>{need.reminderAt ? `${formatLocalDate(need.reminderAt)} · ` : ''}{need.reminderText}</p>}
                        <SingleSelect<NeedStatus> ariaLabel={`Stato · ${need.description}`} selected={need.status} options={NEED_STATES} onChange={(status) => void move(need, status)} disabled={save.isPending} />
                      </article>
                    ))}
                    {!items.length && <p className={styles.meta}>Nessuna esigenza</p>}
                  </div>
                </section>
              );
            })}
          </div>
          {query && !visible.length && <Button variant="secondary" onClick={() => setQuery('')}>Cancella ricerca</Button>}
        </>
      )}
      {creating && <NeedEditorModal onClose={() => setCreating(false)} onCreated={(id) => navigate(`/esigenze/${id}`)} />}
    </div>
  );
}
