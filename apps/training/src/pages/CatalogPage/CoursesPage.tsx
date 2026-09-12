// Catalogo · Corsi (#158, §Catalogo 4-5, scomposta dalle sottoviste di
// CatalogPage): elenco corsi con filtro "Da revisionare" (corsi inattivi importati
// dal sync formativo Factorial e, da #170, filtro per tag), ricerca locale e
// dettaglio via ?id=.

import { useMemo, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { Button, Icon, SearchInput, SingleSelect, Skeleton, StatusBadge, TableToolbar } from '@mrsmith/ui';
import { useTrainingCourses } from '../../api/queries';
import type { CourseListRow } from '../../api/types';
import { CourseDetailDrawer } from '../../components/catalog/CourseDetailDrawer';
import { CourseEditorModal } from '../../components/catalog/CourseEditorModal';
import { ReminderBadge } from '../../components/reminders/ReminderBadge';
import { DELIVERY_MODE_LABELS, PROVIDER_KIND_LABELS } from '../../lib/labels';
import listStyles from '../RequestsPage/listPage.module.css';
import styles from './CoursesPage.module.css';

// Filtro di stato dei Corsi. «Da revisionare» assorbe l'interruttore precedente
// (corsi importati dal sync e mai attivati): era un caso a sé accanto ai
// filtri, qui è uno stato come gli altri. Nessuna selezione = tutti i corsi,
// così all'apertura la lista resta quella di prima.
const COURSE_STATUS_OPTIONS = [
  { value: 'attivi', label: 'Attivi' },
  { value: 'archiviati', label: 'Archiviati' },
  { value: 'da-revisionare', label: 'Da revisionare' },
];

// Riga secondaria della cella Corso: prima area di competenza con il conteggio
// delle altre, poi la certificazione a cui il corso porta. Sono dati di
// lunghezza variabile: come colonne dettavano la larghezza della tabella,
// qui qualificano il titolo e si restringono dai filtri.
function courseSubtitle(row: CourseListRow): string | null {
  const parts: string[] = [];
  const [firstArea, ...otherAreas] = row.skillAreas;
  if (firstArea) {
    parts.push(otherAreas.length > 0 ? `${firstArea.name} +${otherAreas.length}` : firstArea.name);
  }
  if (row.leadsToCertName) parts.push(`→ ${row.leadsToCertName}`);
  return parts.length > 0 ? parts.join(' · ') : null;
}

export function CoursesPage() {
  const [params, setParams] = useSearchParams();
  const selectedId = params.get('id');
  // #170: filtri dei soli Corsi, persistiti nell'URL (parametro assente/vuoto = nessun filtro).
  const selectedTag = params.get('tag') || null;
  const selectedArea = params.get('area') || null;
  const selectedStatus = params.get('stato') || null;

  const [showCreate, setShowCreate] = useState(false);
  const [query, setQuery] = useState('');

  const courses = useTrainingCourses();
  // Opzioni dei tag calcolate su TUTTI i corsi caricati: non si restringono
  // quando cambiano il filtro attivo o il tag selezionato. Dedup esatta,
  // esclusi i tag vuoti/solo spazi, ordinamento con locale italiano; il tag
  // nell'URL senza corrispondenze resta selezionabile per essere visibile e azzerabile.
  const tagOptions = useMemo(() => {
    const rows = courses.data ?? [];
    const tags = [...new Set(rows.flatMap((c) => c.tags).filter((t) => t.trim().length > 0))];
    tags.sort(new Intl.Collator('it').compare);
    if (selectedTag && !tags.includes(selectedTag)) tags.unshift(selectedTag);
    return tags.map((tag) => ({ value: tag, label: tag }));
  }, [courses.data, selectedTag]);

  // Aree di competenza presenti nel catalogo, dedotte dai corsi caricati:
  // stessa regola dei tag, così il filtro non si restringe da solo.
  const areaOptions = useMemo(() => {
    const byId = new Map<string, string>();
    for (const c of courses.data ?? []) {
      for (const a of c.skillAreas) byId.set(a.id, a.name);
    }
    const areas = [...byId].map(([value, label]) => ({ value, label }));
    areas.sort((a, b) => new Intl.Collator('it').compare(a.label, b.label));
    return areas;
  }, [courses.data]);

  const filtered = useMemo(() => {
    const needle = query.trim().toLowerCase();
    const rows = courses.data ?? [];
    return rows.filter((c) => {
      if (selectedStatus === 'attivi' && !c.active) return false;
      if (selectedStatus === 'archiviati' && c.active) return false;
      if (selectedStatus === 'da-revisionare' && !(!c.active && c.factorialTrainingId)) return false;
      if (selectedTag && !c.tags.includes(selectedTag)) return false;
      if (selectedArea && !c.skillAreas.some((a) => a.id === selectedArea)) return false;
      if (!needle) return true;
      return [c.title, c.vendorName, ...c.skillAreas.map((a) => a.name)]
        .filter((v): v is string => Boolean(v))
        .some((v) => v.toLowerCase().includes(needle));
    });
  }, [courses.data, query, selectedArea, selectedStatus, selectedTag]);

  const activeFilterCount = [selectedTag, selectedArea, selectedStatus].filter(Boolean).length;
  const hasCourseFilters = activeFilterCount > 0 || query.trim().length > 0;

  function setCourseFilter(name: 'tag' | 'area' | 'stato', next: string | null) {
    const nextParams = new URLSearchParams(params);
    if (next) nextParams.set(name, next);
    else nextParams.delete(name);
    setParams(nextParams, { replace: true });
  }

  // Azzera ricerca e filtri dei Corsi lasciando intatti gli altri parametri.
  function resetCourseFilters() {
    const nextParams = new URLSearchParams(params);
    nextParams.delete('tag');
    nextParams.delete('area');
    nextParams.delete('stato');
    setQuery('');
    setParams(nextParams, { replace: true });
  }

  function openDetail(id: string) {
    const nextParams = new URLSearchParams(params);
    nextParams.set('id', id);
    setParams(nextParams, { replace: true });
  }

  function closeDetail() {
    const nextParams = new URLSearchParams(params);
    nextParams.delete('id');
    setParams(nextParams, { replace: true });
  }

  return (
    <main className={listStyles.page}>
      <header className={listStyles.header}>
        <div>
          <h1 className={listStyles.title}>Corsi</h1>
          <p className={listStyles.subtitle}>Catalogo corsi: erogazione, area di competenza e stato.</p>
        </div>
      </header>

      <div className={styles.toolbarRow}>
        <TableToolbar
          className={styles.toolbar}
          activeFilterCount={activeFilterCount}
          filters={
            <>
              <SingleSelect
                options={areaOptions}
                selected={selectedArea}
                onChange={(v) => setCourseFilter('area', v)}
                placeholder="Area di competenza"
                allowClear
                clearLabel="Tutte le aree"
                ariaLabel="Filtra per area di competenza"
              />
              <SingleSelect
                options={tagOptions}
                selected={selectedTag}
                onChange={(v) => setCourseFilter('tag', v)}
                placeholder="Tag"
                allowClear
                clearLabel="Tutti i tag"
                ariaLabel="Filtra per tag"
              />
              <SingleSelect
                options={COURSE_STATUS_OPTIONS}
                selected={selectedStatus}
                onChange={(v) => setCourseFilter('stato', v)}
                placeholder="Stato"
                allowClear
                clearLabel="Tutti gli stati"
                ariaLabel="Filtra per stato"
              />
            </>
          }
        >
          <SearchInput
            value={query}
            onChange={setQuery}
            placeholder="Cerca per titolo, area o fornitore..."
          />
        </TableToolbar>
        <Button variant="primary" size="md" leftIcon={<Icon name="plus" size={16} />} onClick={() => setShowCreate(true)}>
          Nuovo corso
        </Button>
      </div>

      {courses.isLoading ? (
        <Skeleton rows={6} />
      ) : courses.isError ? (
        <p className={listStyles.errorNotice}>Lettura dei corsi non riuscita. Riprovare più tardi.</p>
      ) : (courses.data ?? []).length === 0 ? (
        <div className={listStyles.empty}>
          <div className={listStyles.emptyIcon}>
            <Icon name="package" size={32} />
          </div>
          <p className={listStyles.emptyTitle}>Nessun corso a catalogo</p>
          <p className={listStyles.emptyDescription}>Crea il primo corso o attendi il sync formativo da Factorial.</p>
          <Button variant="primary" size="md" onClick={() => setShowCreate(true)}>
            Nuovo corso
          </Button>
        </div>
      ) : filtered.length === 0 ? (
        <div className={listStyles.empty}>
          <p className={listStyles.emptyTitle}>Nessun corso corrisponde alla ricerca</p>
          <p className={listStyles.emptyDescription}>Modifica o azzera ricerca e filtri per vedere altri corsi.</p>
          {hasCourseFilters && (
            <Button variant="secondary" size="md" onClick={resetCourseFilters}>
              Azzera filtri
            </Button>
          )}
        </div>
      ) : (
        <div className={listStyles.tableWrap}>
          <table className={listStyles.table}>
            <thead>
              <tr>
                <th className={listStyles.colPrimary}>Corso</th>
                <th>Fornitore</th>
                <th>Modalità</th>
                <th className={listStyles.cellNum}>Ore</th>
                <th>Stato</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((row: CourseListRow) => {
                const subtitle = courseSubtitle(row);
                return (
                  <tr key={row.id} className={listStyles.row} onClick={() => openDetail(row.id)}>
                    <td className={listStyles.wrapCell}>
                      <button
                        type="button"
                        className={listStyles.rowLink}
                        onClick={(e) => {
                          e.stopPropagation();
                          openDetail(row.id);
                        }}
                      >
                        {row.title}
                      </button>
                      {subtitle && <span className={listStyles.cellSecondary}>{subtitle}</span>}
                    </td>
                    <td>
                      {row.vendorName ? (
                        <span className={listStyles.truncate} title={row.vendorName}>
                          {row.vendorName}
                        </span>
                      ) : (
                        // Senza fornitore resta il tipo di erogazione (Interna/Esterna):
                        // de-enfatizzato, perché non è una ragione sociale.
                        <span className={listStyles.mutedCell}>
                          {PROVIDER_KIND_LABELS[row.providerKind] ?? '—'}
                        </span>
                      )}
                    </td>
                    <td>{DELIVERY_MODE_LABELS[row.deliveryMode] ?? row.deliveryMode}</td>
                    <td className={listStyles.cellNum}>{row.defaultHours ?? '—'}</td>
                    <td>
                      {/* Solo le eccezioni: un corso attivo e senza segnalazioni lascia
                          la cella vuota, così le righe da guardare si trovano a colpo d'occhio. */}
                      <span className={listStyles.inlineBadges}>
                        {row.suspendedAt && <StatusBadge value="suspended" label="Sospeso" variant="warning" />}
                        {!row.active &&
                          (row.factorialTrainingId ? (
                            <StatusBadge value="to_review" label="Da revisionare" variant="warning" />
                          ) : (
                            <StatusBadge value="archived" label="Archiviato" variant="neutral" />
                          ))}
                        {row.complianceRelated && (
                          <StatusBadge
                            value="compliance"
                            label={row.complianceFramework || 'Compliance'}
                            variant="warning"
                          />
                        )}
                        <ReminderBadge
                          text={row.reminderText}
                          date={row.reminderAt}
                          neutral={Boolean(row.suspendedAt)}
                        />
                      </span>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      {showCreate && (
        <CourseEditorModal
          mode="create"
          open={showCreate}
          onClose={() => setShowCreate(false)}
          onSaved={(id) => {
            setShowCreate(false);
            openDetail(id);
          }}
        />
      )}
      {selectedId && <CourseDetailDrawer id={selectedId} onClose={closeDetail} />}
    </main>
  );
}
