import type { CatalogCourseWithCounts } from '../../api/types';
import styles from './CourseCard.module.css';

interface CourseCardProps {
  course: CatalogCourseWithCounts;
  currentYear: number;
  onOpen: (course: CatalogCourseWithCounts) => void;
}

const DELIVERY_LABEL: Record<string, string> = {
  classroom: 'Aula',
  online_live: 'Online live',
  online_self: 'Self-paced',
  on_the_job: 'On the job',
  mixed: 'Misto',
};

function formatEuro(value: number): string {
  return new Intl.NumberFormat('it-IT', {
    style: 'currency',
    currency: 'EUR',
    maximumFractionDigits: 0,
  }).format(value);
}

export function CourseCard({ course, currentYear, onOpen }: CourseCardProps) {
  return (
    <button
      type="button"
      className={`${styles.card} ${!course.active ? styles.cardArchived : ''}`}
      onClick={() => onOpen(course)}
    >
      <header className={styles.head}>
        <h2 className={styles.title}>{course.title}</h2>
        <span className={`${styles.status} ${course.active ? styles.statusActive : styles.statusArchived}`}>
          {course.active ? 'Attivo' : 'Disattivato'}
        </span>
      </header>
      <p className={styles.meta}>
        {course.skillAreaName && <span>{course.skillAreaName}</span>}
        {course.deliveryMode && (
          <>
            <span className={styles.sep}>·</span>
            <span>{DELIVERY_LABEL[course.deliveryMode] ?? course.deliveryMode}</span>
          </>
        )}
        {course.defaultHours !== undefined && (
          <>
            <span className={styles.sep}>·</span>
            <span className={styles.numeric}>{course.defaultHours}h</span>
          </>
        )}
        {course.defaultCost !== undefined && (
          <>
            <span className={styles.sep}>·</span>
            <span className={styles.numeric}>{formatEuro(course.defaultCost)}</span>
          </>
        )}
      </p>
      {course.vendorName && <p className={styles.vendor}>Fornitore: {course.vendorName}</p>}
      {course.complianceRelated && course.complianceFramework && (
        <p className={styles.compliance}>
          Framework compliance: <strong>{course.complianceFramework}</strong>
        </p>
      )}
      <footer className={styles.foot}>
        <span className={styles.count}>
          <span className={styles.numeric}>{course.enrollments_current_year}</span> iscritti {currentYear}
        </span>
        <span className={styles.sep}>·</span>
        <span className={styles.count}>
          <span className={styles.numeric}>{course.enrollments_completed_historical}</span> completati storico
        </span>
        <span className={styles.chevron}>›</span>
      </footer>
    </button>
  );
}
