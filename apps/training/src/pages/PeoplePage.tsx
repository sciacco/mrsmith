import { useMemo, useState } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { ApiError } from '@mrsmith/api-client';
import { Button, Icon, Modal, SearchInput, SingleSelect, Skeleton, useToast } from '@mrsmith/ui';
import {
  useBulkAssignEnrollment,
  useCustomGroups,
  usePeopleDirectory,
  useTrainingLookups,
} from '../api/queries';
import type { PersonSummary } from '../api/types';
import { BulkActionBar } from '../components/BulkActionBar';
import { PersonCreateModal } from '../components/PersonCreateModal';
import { PersonRow } from '../components/PersonRow';
import styles from './PeoplePage.module.css';

interface PeoplePageProps {
  isPeopleAdmin: boolean;
}

const CHIP_PRESETS = [
  { value: '', label: 'Tutti', tone: 'neutral' },
  { value: 'compliance_gap', label: 'Obblighi da gestire', tone: 'red' },
  { value: 'scadenze_imminenti', label: 'Scadenze entro 60 giorni', tone: 'yellow' },
  { value: 'failed_recente', label: 'Esiti negativi', tone: 'yellow' },
  { value: 'senza_formazione_attiva', label: 'Senza corsi attivi', tone: 'neutral' },
];

const VALID_FILTERS = new Set(CHIP_PRESETS.map((preset) => preset.value).filter(Boolean));

const EMPTY_STATE_BY_FILTER: Record<string, string> = {
  compliance_gap: 'Nessuna persona ha obblighi formativi scoperti per i filtri selezionati.',
  scadenze_imminenti: 'Nessuna certificazione o ricorrenza formativa è in scadenza nei prossimi 60 giorni.',
  failed_recente: 'Nessuna iscrizione risulta con esito negativo nel piano selezionato.',
  senza_formazione_attiva: 'Tutte le persone filtrate hanno almeno un corso attivo nel piano selezionato.',
};

function apiErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof ApiError) {
    const body = error.body;
    if (body && typeof body === 'object' && 'message' in body && typeof body.message === 'string') return body.message;
  }
  return fallback;
}

export function PeoplePage({ isPeopleAdmin }: PeoplePageProps) {
  const [params, setParams] = useSearchParams();
  const navigate = useNavigate();
  const { toast } = useToast();

  const year = params.get('year') ?? String(new Date().getFullYear());
  const team = params.get('team') ?? '';
  const group = params.get('group') ?? '';
  const rawFilter = params.get('filter') ?? '';
  const filter = VALID_FILTERS.has(rawFilter) ? rawFilter : '';
  const q = params.get('q') ?? '';
  const sort = params.get('sort') === 'priority' ? 'priority' : 'alpha';

  const directory = usePeopleDirectory({ year, team, group, filter, q }, isPeopleAdmin);
  const lookups = useTrainingLookups(isPeopleAdmin);
  const groups = useCustomGroups({ status: 'attivo' }, isPeopleAdmin);
  const bulkAssign = useBulkAssignEnrollment(isPeopleAdmin);

  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [assignOpen, setAssignOpen] = useState(false);
  const [assignDraft, setAssignDraft] = useState({ courseId: '', plannedStart: '', plannedEnd: '' });

  function updateParam(key: string, value: string | null, replace = true) {
    setParams((previous) => {
      const next = new URLSearchParams(previous);
      if (value && value !== '') next.set(key, value);
      else next.delete(key);
      return next;
    }, { replace });
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
        sourceCustomGroupId: selectedGroup ? selectedGroup.id : undefined,
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
  const groupOptions = useMemo(
    () => [
      { value: '', label: 'Tutti i gruppi' },
      ...(groups.data?.groups ?? []).map((item) => ({ value: item.id, label: `${item.name} (${item.member_count})` })),
    ],
    [groups.data],
  );
  const selectedGroup = (groups.data?.groups ?? []).find((item) => item.id === group);
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
          <p className={styles.subtitle}>Directory dipendenti con priorità formative e azioni da gestire.</p>
        </div>
        <div className={styles.headerActions}>
          <Link to="/persone/gruppi" className={styles.manageLink}>
            Gestisci gruppi
          </Link>
          <SearchInput value={q} onChange={(value) => updateParam('q', value)} placeholder="Cerca per nome o email" />
          <Button variant="primary" onClick={() => setCreateOpen(true)} leftIcon={<Icon name="plus" size={16} />}>
            Nuova persona
          </Button>
        </div>
      </header>

      <div className={styles.toolbar}>
        <div className={styles.chips}>
          {CHIP_PRESETS.map((preset) => (
            <button
              key={preset.value || 'all'}
              type="button"
              className={`${styles.chip} ${styles[`chip_${preset.tone}`]} ${filter === preset.value ? styles.chipActive : ''}`}
              onClick={() => updateParam('filter', preset.value || null, false)}
            >
              <span className={styles.chipDot} aria-hidden />
              {preset.label}
            </button>
          ))}
        </div>
        <div className={styles.sortRow}>
          <SingleSelect
            options={groupOptions}
            selected={group || null}
            onChange={(value) => updateParam('group', value || null)}
            placeholder="Gruppo"
            allowClear
          />
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

      {selectedGroup && filteredPeople.length > 0 && (
        <div className={styles.groupStrip}>
          <span>{selectedGroup.name}</span>
          <button
            type="button"
            onClick={() => setSelected(new Set(filteredPeople.map((person) => person.id)))}
          >
            Seleziona gruppo
          </button>
        </div>
      )}

      {filteredPeople.length === 0 ? (
        <div className={styles.empty}>
          {EMPTY_STATE_BY_FILTER[filter] ?? 'Nessuna persona corrisponde ai filtri.'}
        </div>
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

      <PersonCreateModal
        open={createOpen}
        teams={lookups.data?.teams ?? []}
        onClose={() => setCreateOpen(false)}
        onCreated={(id) => navigate(`/persone/${id}`)}
      />

      <Modal open={assignOpen} onClose={() => setAssignOpen(false)} title="Assegna corso obbligatorio" size="md">
        <form
          className={styles.modalForm}
          onSubmit={(event) => {
            event.preventDefault();
            submitAssign();
          }}
        >
          <p className={styles.modalText}>
            {selected.size} persone selezionate · anno {year}
            {selectedGroup ? ` · ${selectedGroup.name}` : ''}
          </p>
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
