import { useMemo, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { ApiError } from '@mrsmith/api-client';
import { Button, Modal, SearchInput, SingleSelect, Skeleton, useToast } from '@mrsmith/ui';
import {
  useBulkAssignEnrollment,
  usePeopleDirectory,
  useTrainingLookups,
} from '../api/queries';
import type { PersonSummary } from '../api/types';
import { BulkActionBar } from '../components/BulkActionBar';
import { PersonRow } from '../components/PersonRow';
import styles from './PeoplePage.module.css';

interface PeoplePageProps {
  isPeopleAdmin: boolean;
}

const CHIP_PRESETS: Array<{ value: string; label: string }> = [
  { value: '', label: 'Tutti' },
  { value: 'a_norma', label: 'A norma' },
  { value: 'con_gap', label: 'Con gap' },
  { value: 'senza_piano', label: 'Senza piano' },
  { value: 'nuovo_assunto', label: 'Nuovi assunti' },
];

function apiErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof ApiError) {
    const body = error.body;
    if (body && typeof body === 'object' && 'message' in body && typeof body.message === 'string') return body.message;
  }
  return fallback;
}

export function PeoplePage({ isPeopleAdmin }: PeoplePageProps) {
  const [params, setParams] = useSearchParams();
  const { toast } = useToast();

  const year = params.get('year') ?? String(new Date().getFullYear());
  const team = params.get('team') ?? '';
  const filter = params.get('filter') ?? '';
  const q = params.get('q') ?? '';
  const sort = params.get('sort') ?? 'alpha';

  const directory = usePeopleDirectory({ year, team, filter, q }, isPeopleAdmin);
  const lookups = useTrainingLookups(isPeopleAdmin);
  const bulkAssign = useBulkAssignEnrollment(isPeopleAdmin);

  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [assignOpen, setAssignOpen] = useState(false);
  const [assignDraft, setAssignDraft] = useState({ courseId: '', plannedStart: '', plannedEnd: '' });

  function updateParam(key: string, value: string | null) {
    setParams((previous) => {
      const next = new URLSearchParams(previous);
      if (value && value !== '') next.set(key, value);
      else next.delete(key);
      return next;
    }, { replace: true });
  }

  const people = directory.data ?? [];

  const filteredPeople = useMemo(() => {
    let result = people;
    if (q.trim()) {
      const needle = q.trim().toLowerCase();
      result = result.filter((p) => `${p.name} ${p.email}`.toLowerCase().includes(needle));
    }
    if (sort === 'priority') {
      result = [...result].sort((a, b) => b.priority_score - a.priority_score || a.name.localeCompare(b.name));
    } else {
      result = [...result].sort((a, b) => a.name.localeCompare(b.name));
    }
    return result;
  }, [people, q, sort]);

  if (!isPeopleAdmin) {
    return <main className={styles.page}><p className={styles.muted}>Accesso riservato al team People.</p></main>;
  }

  function toggleSelect(id: string) {
    setSelected((previous) => {
      const next = new Set(previous);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  function toggleExpand(id: string) {
    setExpandedId((previous) => (previous === id ? null : id));
  }

  function clearSelection() {
    setSelected(new Set());
  }

  function openAssign() {
    setAssignDraft({ courseId: '', plannedStart: '', plannedEnd: '' });
    setAssignOpen(true);
  }

  function submitAssign() {
    if (!assignDraft.courseId) {
      toast('Seleziona un corso', 'warning');
      return;
    }
    const ids = Array.from(selected);
    bulkAssign.mutate(
      {
        employeeIds: ids,
        courseId: assignDraft.courseId,
        planParams: {
          year: Number(year),
          plannedStart: assignDraft.plannedStart || undefined,
          plannedEnd: assignDraft.plannedEnd || undefined,
          mandatory: true,
        },
      },
      {
        onSuccess: (response) => {
          if (response.failed > 0) {
            toast(`${response.created} create, ${response.failed} fallite`, 'warning');
          } else {
            toast(`${response.created} iscrizioni create`);
          }
          clearSelection();
          setAssignOpen(false);
        },
        onError: (error) => toast(apiErrorMessage(error, 'Assegnazione massiva non riuscita'), 'error'),
      },
    );
  }

  const courses = lookups.data?.courses ?? [];
  const courseOptions = useMemo(
    () => courses.filter((c) => c.active).map((c) => ({ value: c.id, label: c.label })),
    [courses],
  );

  if (directory.isLoading) {
    return (
      <main className={styles.page}>
        <Skeleton rows={8} />
      </main>
    );
  }

  return (
    <main className={styles.page}>
      <header className={styles.header}>
        <div>
          <h1>Persone</h1>
          <p className={styles.subtitle}>Directory dipendenti con stato formativo aggregato.</p>
        </div>
        <SearchInput value={q} onChange={(value) => updateParam('q', value)} placeholder="Cerca per nome o email" />
      </header>

      <div className={styles.toolbar}>
        <div className={styles.chips}>
          {CHIP_PRESETS.map((preset) => (
            <button
              key={preset.value || 'all'}
              type="button"
              className={`${styles.chip} ${filter === preset.value ? styles.chipActive : ''}`}
              onClick={() => updateParam('filter', preset.value || null)}
            >
              {preset.label}
            </button>
          ))}
        </div>
        <div className={styles.sortRow}>
          <span className={styles.sortLabel}>Ordina</span>
          <button
            type="button"
            className={`${styles.sortBtn} ${sort === 'alpha' ? styles.sortActive : ''}`}
            onClick={() => updateParam('sort', null)}
          >
            A-Z
          </button>
          <button
            type="button"
            className={`${styles.sortBtn} ${sort === 'priority' ? styles.sortActive : ''}`}
            onClick={() => updateParam('sort', 'priority')}
          >
            Priorità
          </button>
        </div>
      </div>

      <div className={styles.counter}>{filteredPeople.length} persone</div>

      {filteredPeople.length === 0 ? (
        <div className={styles.empty}>Nessuna persona corrisponde ai filtri.</div>
      ) : (
        <div className={styles.list}>
          {filteredPeople.map((person: PersonSummary) => (
            <PersonRow
              key={person.id}
              person={person}
              selected={selected.has(person.id)}
              expanded={expandedId === person.id}
              onToggleSelect={() => toggleSelect(person.id)}
              onToggleExpand={() => toggleExpand(person.id)}
            />
          ))}
        </div>
      )}

      <BulkActionBar selectedCount={selected.size} onClear={clearSelection}>
        <Button variant="primary" size="sm" onClick={openAssign}>
          Assegna corso obbligatorio
        </Button>
      </BulkActionBar>

      <Modal open={assignOpen} onClose={() => setAssignOpen(false)} title="Assegna corso obbligatorio" size="md">
        <form
          className={styles.modalForm}
          onSubmit={(event) => {
            event.preventDefault();
            submitAssign();
          }}
        >
          <p className={styles.modalText}>{selected.size} persone selezionate · anno {year}</p>
          <label>
            <span>Corso</span>
            <SingleSelect
              options={courseOptions}
              selected={assignDraft.courseId || null}
              onChange={(v) => setAssignDraft((d) => ({ ...d, courseId: v ? String(v) : '' }))}
              placeholder="Seleziona"
              searchable
            />
          </label>
          <div className={styles.modalGrid}>
            <label>
              <span>Inizio previsto</span>
              <input type="date" value={assignDraft.plannedStart} onChange={(e) => setAssignDraft((d) => ({ ...d, plannedStart: e.target.value }))} />
            </label>
            <label>
              <span>Fine prevista</span>
              <input type="date" value={assignDraft.plannedEnd} onChange={(e) => setAssignDraft((d) => ({ ...d, plannedEnd: e.target.value }))} />
            </label>
          </div>
          <div className={styles.modalActions}>
            <Button type="button" variant="secondary" onClick={() => setAssignOpen(false)}>Annulla</Button>
            <Button type="submit" variant="primary" disabled={bulkAssign.isPending}>Assegna</Button>
          </div>
        </form>
      </Modal>
    </main>
  );
}
