import { useMemo, useState } from 'react';
import { Button, SearchInput, SingleSelect, Skeleton } from '@mrsmith/ui';
import { useCatalogCourses, useTrainingLookups } from '../api/queries';
import type { CatalogCourseWithCounts } from '../api/types';
import { CourseCard } from '../components/CourseCard';
import { CourseDetailDrawer } from '../components/CourseDetailDrawer';
import { NewCourseModal } from '../components/NewCourseModal';
import styles from './CatalogPage.module.css';

interface CatalogPageProps {
  isPeopleAdmin: boolean;
}

type StatoFilter = '' | 'attivo' | 'disattivato';

const STATO_OPTIONS = [
  { value: '', label: 'Tutti gli stati' },
  { value: 'attivo', label: 'Attivi' },
  { value: 'disattivato', label: 'Disattivati' },
];

export function CatalogPage({ isPeopleAdmin }: CatalogPageProps) {
  const [search, setSearch] = useState('');
  const [skillArea, setSkillArea] = useState<string | null>(null);
  const [fornitore, setFornitore] = useState<string | null>(null);
  const [stato, setStato] = useState<StatoFilter>('attivo');
  const [openCourse, setOpenCourse] = useState<CatalogCourseWithCounts | null>(null);
  const [newCourseOpen, setNewCourseOpen] = useState(false);

  const lookups = useTrainingLookups(true);
  const catalog = useCatalogCourses(
    {
      skillArea: skillArea ?? undefined,
      fornitore: fornitore ?? undefined,
      stato,
      q: search.trim() || undefined,
    },
    true,
  );

  const skillOptions = useMemo(
    () => [
      { value: '', label: 'Tutte le skill area' },
      ...(lookups.data?.skillAreas ?? []).map((s) => ({ value: s.id, label: s.label })),
    ],
    [lookups.data],
  );
  const vendorOptions = useMemo(
    () => [
      { value: '', label: 'Tutti i fornitori' },
      ...(lookups.data?.vendors ?? []).map((v) => ({ value: v.id, label: v.label })),
    ],
    [lookups.data],
  );

  const courses = catalog.data?.courses ?? [];
  const activeCount = courses.filter((c) => c.active).length;
  const currentYear = new Date().getFullYear();

  return (
    <main className={styles.page}>
      <header className={styles.header}>
        <div>
          <h1 className={styles.title}>Catalogo</h1>
          <p className={styles.subtitle}>
            <span className={styles.numeric}>{activeCount}</span> corsi attivi su {courses.length} totali.
          </p>
        </div>
        {isPeopleAdmin && (
          <Button variant="primary" size="md" onClick={() => setNewCourseOpen(true)}>
            + Nuovo corso
          </Button>
        )}
      </header>

      <div className={styles.filters}>
        <div className={styles.searchWrap}>
          <SearchInput value={search} onChange={setSearch} placeholder="Cerca corso..." />
        </div>
        <SingleSelect
          options={skillOptions}
          selected={skillArea}
          onChange={(v) => setSkillArea(v || null)}
          placeholder="Skill area"
          allowClear
        />
        <SingleSelect
          options={vendorOptions}
          selected={fornitore}
          onChange={(v) => setFornitore(v || null)}
          placeholder="Fornitore"
          allowClear
        />
        <SingleSelect
          options={STATO_OPTIONS}
          selected={stato}
          onChange={(v) => setStato((v ?? '') as StatoFilter)}
          placeholder="Stato"
        />
      </div>

      {catalog.isLoading ? (
        <Skeleton rows={5} />
      ) : courses.length === 0 ? (
        <p className={styles.empty}>Nessun corso trovato con i filtri attivi.</p>
      ) : (
        <div className={styles.grid}>
          {courses.map((course) => (
            <CourseCard
              key={course.id}
              course={course}
              currentYear={currentYear}
              onOpen={(c) => setOpenCourse(c)}
            />
          ))}
        </div>
      )}

      <NewCourseModal
        open={newCourseOpen}
        isPeopleAdmin={isPeopleAdmin}
        onClose={() => setNewCourseOpen(false)}
        onCreated={() => {
          catalog.refetch();
        }}
      />

      <CourseDetailDrawer
        course={openCourse}
        isPeopleAdmin={isPeopleAdmin}
        currentYear={currentYear}
        onClose={() => setOpenCourse(null)}
      />
    </main>
  );
}
