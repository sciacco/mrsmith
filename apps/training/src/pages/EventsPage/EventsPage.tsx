import { useMemo, useState } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { formatNumber } from '@mrsmith/format';
import { Button, Icon, SearchInput, Skeleton, SingleSelect, TableToolbar, useToast } from '@mrsmith/ui';
import { useCreateEvent, useTrainingEvents, useTrainingLookups } from '../../api/queries';
import type { EventInput, EventListRow } from '../../api/types';
import { EventConditionBadges } from '../../components/events/EventConditionBadges';
import { EventFormModal } from '../../components/events/EventFormModal';
import { ReminderBadge } from '../../components/reminders/ReminderBadge';
import { describeApiError } from '../../components/events/apiErrors';
import { formatInstantDate } from '../../components/events/eventFormat';
import {
  DEFAULT_EVENT_FILTERS,
  EVENT_CONDITIONS,
  type EventCondition,
  eventFiltersToParams,
  filterEvents,
  isEventFiltersActive,
  parseEventFilters,
} from '../../lib/eventFilters';
import { EVENT_CONDITION_BADGE_LABELS } from '../../lib/labels';
import styles from './EventsPage.module.css';

const CONDITION_LABELS: Record<EventCondition, string> = {
  all: 'Tutte le condizioni',
  ...EVENT_CONDITION_BADGE_LABELS,
};

export function EventsPage() {
  const navigate = useNavigate();
  const { toast } = useToast();
  const [params, setParams] = useSearchParams();
  const filters = parseEventFilters(params);

  const events = useTrainingEvents();
  const lookups = useTrainingLookups();
  const createEvent = useCreateEvent();
  const [showCreate, setShowCreate] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);

  function setFilters(next: typeof filters) {
    setParams(eventFiltersToParams(next, params), { replace: true });
  }

  const filtered = useMemo(() => filterEvents(events.data ?? [], filters), [events.data, filters]);
  const courseOptions = useMemo(() => {
    const seen = new Map<string, string>();
    for (const row of events.data ?? []) seen.set(row.courseId, row.courseTitle);
    return [...seen.entries()].map(([value, label]) => ({ value, label })).sort((a, b) => a.label.localeCompare(b.label));
  }, [events.data]);

  async function handleCreate(input: EventInput) {
    setCreateError(null);
    try {
      const response = await createEvent.mutateAsync(input);
      setShowCreate(false);
      toast('Evento creato');
      if (response.id) navigate(`/eventi/${response.id}`);
    } catch (error) {
      setCreateError(describeApiError(error, 'Creazione non riuscita'));
    }
  }

  const active = isEventFiltersActive(filters);

  return (
    <main className={styles.page}>
      <header className={styles.header}>
        <div>
          <h1 className={styles.title}>Eventi</h1>
          <p className={styles.subtitle}>Tornate di formazione: sessioni, iscritti e spese collegate.</p>
        </div>
        <Button variant="primary" size="md" leftIcon={<Icon name="plus" size={16} />} onClick={() => setShowCreate(true)}>
          Nuovo evento
        </Button>
      </header>

      <TableToolbar
        activeFilterCount={(filters.condition !== 'all' ? 1 : 0) + (filters.courseId !== '' ? 1 : 0)}
        filters={
          <>
            <SingleSelect
              options={EVENT_CONDITIONS.map((c) => ({ value: c, label: CONDITION_LABELS[c] }))}
              selected={filters.condition}
              onChange={(v) => setFilters({ ...filters, condition: v ?? 'all' })}
              placeholder="Condizione"
              ariaLabel="Filtra per condizione operativa"
            />
            <SingleSelect
              options={courseOptions}
              selected={filters.courseId || null}
              onChange={(v) => setFilters({ ...filters, courseId: v ?? '' })}
              placeholder="Corso"
              allowClear
              ariaLabel="Filtra per corso"
            />
          </>
        }
      >
        <SearchInput
          value={filters.q}
          onChange={(q) => setFilters({ ...filters, q })}
          placeholder="Cerca per titolo, corso o fornitore..."
        />
      </TableToolbar>

      {events.isLoading ? (
        <Skeleton rows={6} />
      ) : events.isError ? (
        <p className={styles.errorNotice}>Lettura degli eventi non riuscita. Riprovare più tardi.</p>
      ) : (events.data ?? []).length === 0 ? (
        <div className={styles.empty}>
          <div className={styles.emptyIcon}>
            <Icon name="calendar" size={32} />
          </div>
          <p className={styles.emptyTitle}>Nessun evento</p>
          <p className={styles.emptyDescription}>Crea il primo evento per avviare una tornata di formazione.</p>
          <Button variant="primary" size="md" onClick={() => setShowCreate(true)}>
            Nuovo evento
          </Button>
        </div>
      ) : filtered.length === 0 ? (
        <div className={styles.empty}>
          <p className={styles.emptyTitle}>Nessun evento corrisponde ai filtri</p>
          {active && (
            <Button variant="ghost" size="sm" onClick={() => setFilters(DEFAULT_EVENT_FILTERS)}>
              Azzera filtri
            </Button>
          )}
        </div>
      ) : (
        <div className={styles.tableWrap}>
          <table className={styles.table}>
            <thead>
              <tr>
                <th>Titolo</th>
                <th>Corso</th>
                <th>Fornitore</th>
                <th className={styles.numCell}>Sessioni</th>
                <th className={styles.numCell}>Iscritti</th>
                <th className={styles.numCell}>di cui annullate</th>
                <th>Condizione</th>
                <th>Creato il</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((row: EventListRow) => (
                <tr key={row.id} className={styles.row} onClick={() => navigate(`/eventi/${row.id}`)}>
                  <td className={styles.wrapCell}>
                    <span className={styles.inlineBadges}>
                      <Link
                        to={`/eventi/${row.id}`}
                        className={styles.rowLink}
                        onClick={(e) => e.stopPropagation()}
                      >
                        {row.title}
                      </Link>
                      <ReminderBadge text={row.reminderText} date={row.reminderAt} />
                    </span>
                  </td>
                  <td>{row.courseTitle}</td>
                  <td>{row.vendorName || '—'}</td>
                  <td className={styles.numCell}>{formatNumber(row.sessionsCount)}</td>
                  <td className={styles.numCell}>{formatNumber(row.enrollmentsCount)}</td>
                  <td className={styles.numCell}>{formatNumber(row.cancelledEnrollmentsCount)}</td>
                  <td>
                    <EventConditionBadges flags={row.flags} />
                  </td>
                  <td>{formatInstantDate(row.createdAt)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {showCreate && (
        <EventFormModal
          key="create"
          open={showCreate}
          mode="create"
          courses={lookups.data?.courses ?? []}
          vendors={lookups.data?.vendors ?? []}
          pending={createEvent.isPending}
          error={createError}
          onSubmit={handleCreate}
          onClose={() => {
            setShowCreate(false);
            setCreateError(null);
          }}
        />
      )}
    </main>
  );
}
