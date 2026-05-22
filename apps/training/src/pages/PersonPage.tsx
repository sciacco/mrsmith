import { useMemo, useState } from 'react';
import { Link, useParams, useSearchParams } from 'react-router-dom';
import { ApiError } from '@mrsmith/api-client';
import { Button, Skeleton, useToast } from '@mrsmith/ui';
import { useCreateEnrollment, usePersonProfile, useTrainingLookups } from '../api/queries';
import type {
  CertificationRow,
  PersonHistoryYearRow,
  PersonProfile,
  PersonSkillArea,
  PersonSuggestion,
  PlanEnrollment,
} from '../api/types';
import { EnrollmentDrawer } from '../components/EnrollmentDrawer';
import { PersonEditModal } from '../components/PersonEditModal';
import { classifyAlertLevel } from '../lib/alertLevel';
import styles from './PersonPage.module.css';

interface PersonPageProps {
  isPeopleAdmin: boolean;
}

function initialsOf(name: string): string {
  return name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part.charAt(0).toUpperCase())
    .join('');
}

function formatDate(value: string | undefined): string {
  if (!value) return '';
  return value.slice(0, 10);
}

function formatMoney(value: number | undefined): string {
  if (value === undefined || value === 0) return '—';
  return new Intl.NumberFormat('it-IT', { style: 'currency', currency: 'EUR', maximumFractionDigits: 0 }).format(value);
}

function apiErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof ApiError) {
    const body = error.body;
    if (body && typeof body === 'object' && 'message' in body && typeof body.message === 'string') return body.message;
  }
  return fallback;
}

export function PersonPage({ isPeopleAdmin }: PersonPageProps) {
  const { id } = useParams<{ id: string }>();
  const [params] = useSearchParams();
  const year = params.get('year') ?? String(new Date().getFullYear());
  const profile = usePersonProfile(id, year, isPeopleAdmin);
  const lookups = useTrainingLookups(isPeopleAdmin);
  const createEnrollment = useCreateEnrollment(isPeopleAdmin);
  const { toast } = useToast();
  const [openEnrollmentId, setOpenEnrollmentId] = useState<string | null>(null);
  const [editOpen, setEditOpen] = useState(false);

  const data = profile.data;

  const openEnrollment = useMemo(() => {
    if (!data || !openEnrollmentId) return null;
    return data.enrollments_current_year.find((e) => e.id === openEnrollmentId) ?? null;
  }, [data, openEnrollmentId]);

  if (!isPeopleAdmin) {
    return <main className={styles.page}><p>Accesso riservato al team People.</p></main>;
  }

  if (profile.isLoading || !data) {
    return <main className={styles.page}><Skeleton rows={6} /></main>;
  }

  function planFromSuggestion(suggestion: PersonSuggestion) {
    const courseId = suggestion.recommended_courses[0]?.id;
    if (!courseId || !id) return;
    const planId = lookups.data?.plans?.find((plan) => plan.label.startsWith(year) && plan.active)?.id;
    if (!planId) {
      toast(`Piano ${year} non trovato`, 'warning');
      return;
    }
    createEnrollment.mutate(
      { employeeId: id, courseId, trainingPlanId: planId },
      {
        onSuccess: () => toast('Iscrizione creata in stato proposta'),
        onError: (error) => toast(apiErrorMessage(error, 'Creazione iscrizione non riuscita'), 'error'),
      },
    );
  }

  const enrollmentAlerts = data.enrollments_current_year.reduce(
    (acc, e) => {
      const level = classifyAlertLevel(e);
      acc[level] = (acc[level] ?? 0) + 1;
      return acc;
    },
    {} as Record<string, number>,
  );

  return (
    <main className={styles.page}>
      <header className={styles.hero}>
        <div className={styles.heroLeft}>
          <div className={styles.avatar} aria-hidden>{initialsOf(data.identity_min.name)}</div>
          <div className={styles.heroText}>
            <h1>{data.identity_min.name}</h1>
            <p className={styles.heroMeta}>
              {data.identity_min.email} · {data.identity_min.team_name || 'Senza team'}
            </p>
            <p className={styles.heroState}>
              {data.compliance.open_gaps.length > 0 && (
                <span className={styles.heroCritical}>🔴 {data.compliance.open_gaps.length} gap</span>
              )}
              <span>{data.enrollments_current_year.length} iscrizioni {year}</span>
              {data.compliance.expiring_certs[0] && (
                <span>Prossima cert {formatDate(data.compliance.expiring_certs[0].expiresOn)}</span>
              )}
              {enrollmentAlerts.critical && <span>🔴 {enrollmentAlerts.critical}</span>}
              {enrollmentAlerts.warning && <span>🟡 {enrollmentAlerts.warning}</span>}
            </p>
          </div>
        </div>
        <div className={styles.heroActions}>
          <Button variant="secondary" size="sm" onClick={() => setEditOpen(true)}>
            Modifica
          </Button>
          <Link to="/persone" className={styles.backLink}>← Directory</Link>
        </div>
      </header>

      <div className={styles.grid}>
        <ComplianceSection profile={data} />
        <EnrollmentsSection profile={data} year={year} onOpen={(eId) => setOpenEnrollmentId(eId)} />
        <CertificationsSection profile={data} />
        <SkillMatrixSection profile={data} />
        <HistorySection profile={data} />
        <SuggestionsSection profile={data} onPlan={planFromSuggestion} pending={createEnrollment.isPending} />
      </div>

      <EnrollmentDrawer enrollment={openEnrollment} isPeopleAdmin={isPeopleAdmin} onClose={() => setOpenEnrollmentId(null)} />
      <PersonEditModal
        open={editOpen}
        profile={data}
        teams={lookups.data?.teams ?? []}
        onClose={() => setEditOpen(false)}
      />
    </main>
  );
}

function ComplianceSection({ profile }: { profile: PersonProfile }) {
  const { compliance } = profile;
  return (
    <section className={styles.section}>
      <header className={styles.sectionHead}>
        <h2>Compliance</h2>
        <span className={styles.pct}>{Math.round(compliance.coverage_pct)}% copertura</span>
      </header>
      {compliance.mandatory_rules.length === 0 ? (
        <p className={styles.muted}>Nessun corso obbligatorio applicabile.</p>
      ) : (
        <ul className={styles.list}>
          {compliance.mandatory_rules.map((rule) => (
            <li key={rule.course_id} className={styles.complianceRow}>
              <span className={styles.complianceTitle}>{rule.course_title}</span>
              {rule.compliance_framework && <span className={styles.complianceTag}>{rule.compliance_framework}</span>}
              <span className={`${styles.complianceStatus} ${styles[`status_${rule.status}`]}`}>
                {rule.status === 'compliant' ? 'Coperta' : rule.status === 'missing_or_expired' ? 'Da pianificare' : 'Da verificare'}
              </span>
              {rule.last_valid_awarded_on && <span className={styles.complianceDate}>dal {formatDate(rule.last_valid_awarded_on)}</span>}
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

function EnrollmentsSection({ profile, year, onOpen }: { profile: PersonProfile; year: string; onOpen: (id: string) => void }) {
  const enrollments = profile.enrollments_current_year;
  return (
    <section className={styles.section}>
      <header className={styles.sectionHead}>
        <h2>Iscrizioni {year}</h2>
        <span className={styles.pct}>{enrollments.length}</span>
      </header>
      {enrollments.length === 0 ? (
        <p className={styles.muted}>Nessuna iscrizione per quest'anno.</p>
      ) : (
        <ul className={styles.list}>
          {enrollments.map((enrollment: PlanEnrollment) => {
            const level = classifyAlertLevel(enrollment);
            return (
              <li key={enrollment.id} className={styles.enrollmentRow}>
                <button type="button" className={styles.enrollmentBtn} onClick={() => onOpen(enrollment.id)}>
                  <span className={`${styles.dot} ${styles[`alert_${level}`]}`} aria-hidden />
                  <span className={styles.enrollmentTitle}>{enrollment.courseTitle}</span>
                  <span className={styles.enrollmentMeta}>
                    {enrollment.vendorName && <span>{enrollment.vendorName}</span>}
                    <span>{formatDate(enrollment.plannedStart)}</span>
                    <span className={styles.enrollmentStatus}>{enrollment.status}</span>
                  </span>
                </button>
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}

function CertificationsSection({ profile }: { profile: PersonProfile }) {
  const certs = profile.certifications;
  return (
    <section className={styles.section}>
      <header className={styles.sectionHead}>
        <h2>Certificazioni</h2>
        <span className={styles.pct}>{certs.length}</span>
      </header>
      {certs.length === 0 ? (
        <p className={styles.muted}>Nessuna certificazione registrata.</p>
      ) : (
        <ul className={styles.list}>
          {certs.map((cert: CertificationRow) => (
            <li key={cert.awardId} className={styles.certRow}>
              <span className={styles.certCode}>{cert.certificationCode}</span>
              <span className={styles.certName}>{cert.certificationName}</span>
              <span className={`${styles.certStatus} ${styles[`certStatus_${cert.currentStatus}`]}`}>
                {cert.currentStatus === 'valid' || cert.currentStatus === 'valid_no_expiry' ? 'Valida' : 'Scaduta'}
              </span>
              {cert.expiresOn && <span className={styles.certDate}>scad. {formatDate(cert.expiresOn)}</span>}
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

function SkillMatrixSection({ profile }: { profile: PersonProfile }) {
  const areas = profile.skill_areas;
  return (
    <section className={styles.section}>
      <header className={styles.sectionHead}>
        <h2>Skill area</h2>
      </header>
      {areas.length === 0 ? (
        <p className={styles.muted}>Nessuna competenza derivata da corsi completati.</p>
      ) : (
        <ul className={styles.list}>
          {areas.map((area: PersonSkillArea) => (
            <li key={area.skill_area_id} className={styles.skillRow}>
              <span className={styles.skillName}>{area.name}</span>
              <span className={`${styles.skillLevel} ${styles[`level_${area.derived_level}`]}`}>{area.derived_level}</span>
              <span className={styles.skillEvidence}>
                {area.evidence.courses_completed.length} corsi · {area.evidence.certs.length} cert
              </span>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

function HistorySection({ profile }: { profile: PersonProfile }) {
  const history = profile.history_by_year;
  return (
    <section className={styles.section}>
      <header className={styles.sectionHead}>
        <h2>Storico formativo</h2>
      </header>
      {history.length === 0 ? (
        <p className={styles.muted}>Nessuno storico disponibile.</p>
      ) : (
        <table className={styles.historyTable}>
          <thead>
            <tr>
              <th>Anno</th>
              <th>Completate</th>
              <th>Non superate</th>
              <th>Ore</th>
              <th>Costo</th>
            </tr>
          </thead>
          <tbody>
            {history.map((row: PersonHistoryYearRow) => (
              <tr key={row.year}>
                <td>{row.year}</td>
                <td>{row.completed_count}</td>
                <td>{row.failed_count}</td>
                <td>{row.hours_total || '—'}</td>
                <td>{formatMoney(row.cost_total)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </section>
  );
}

function SuggestionsSection({ profile, onPlan, pending }: { profile: PersonProfile; onPlan: (suggestion: PersonSuggestion) => void; pending: boolean }) {
  const suggestions = profile.suggestions;
  return (
    <section className={`${styles.section} ${styles.sectionFull}`}>
      <header className={styles.sectionHead}>
        <h2>Suggerimenti</h2>
      </header>
      {suggestions.length === 0 ? (
        <p className={styles.muted}>Nessun gap aperto: niente da suggerire.</p>
      ) : (
        <ul className={styles.list}>
          {suggestions.map((suggestion: PersonSuggestion, index: number) => {
            const course = suggestion.recommended_courses[0];
            return (
              <li key={index} className={styles.suggestionRow}>
                <div className={styles.suggestionInfo}>
                  <strong>{suggestion.gap.description}</strong>
                  {course && <span className={styles.suggestionCourse}>→ {course.title}</span>}
                </div>
                <Button variant="secondary" size="sm" disabled={pending || !course} onClick={() => onPlan(suggestion)}>
                  Pianifica
                </Button>
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}
