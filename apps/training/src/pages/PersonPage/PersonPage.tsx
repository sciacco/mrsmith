// Scheda persona (#158, §Persone 3): dati e appartenenze, iscrizioni
// cross-evento con rimando al dettaglio evento, richieste con rimando al
// drawer di /richieste, coperture delle regole a platea che la includono con
// rimando alla regola. Sola lettura oltre al pulsante di modifica: le azioni
// di dominio (annulla iscrizione, decidi richiesta...) restano nelle loro
// superfici dedicate.

import { useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { Button, Icon, Skeleton, StatusBadge } from '@mrsmith/ui';
import { usePersonDetail } from '../../api/queries';
import { formatDateOnly, formatInstantDate } from '../../components/events/eventFormat';
import { deliveryStatusVariant } from '../../components/events/statusVariants';
import { AssessmentsSection } from '../../components/people/AssessmentsSection';
import { AwardsSection } from '../../components/people/AwardsSection';
import { PersonEditorModal } from '../../components/people/PersonEditorModal';
import { PersonPathsSection } from '../../components/people/PersonPathsSection';
import { DELIVERY_STATUS_LABELS, LEARNING_OUTCOME_LABELS, NEED_LABELS, PERSON_STATUS_LABELS, REQUEST_OUTCOME_LABELS } from '../../lib/labels';
import listStyles from '../RequestsPage/listPage.module.css';
import cardStyles from '../../components/requests/drawerShared.module.css';
import styles from './PersonPage.module.css';

export function PersonPage() {
  const { id = '' } = useParams();
  const detail = usePersonDetail(id);
  const [showEdit, setShowEdit] = useState(false);

  if (detail.isLoading) {
    return (
      <main className={listStyles.page}>
        <Skeleton rows={8} />
      </main>
    );
  }

  if (detail.isError || !detail.data) {
    return (
      <main className={listStyles.page}>
        <Link to="/persone" className={styles.backLink}>
          <Icon name="arrow-left" size={16} /> Persone
        </Link>
        <p className={listStyles.errorNotice}>
          Persona non disponibile: la lettura non è riuscita. Riprovare più tardi o tornare all'elenco.
        </p>
      </main>
    );
  }

  const person = detail.data;

  return (
    <main className={listStyles.page}>
      <Link to="/persone" className={styles.backLink}>
        <Icon name="arrow-left" size={16} /> Persone
      </Link>

      <header className={listStyles.header}>
        <div>
          <h1 className={listStyles.title}>
            {person.lastName} {person.firstName}
          </h1>
          <p className={listStyles.subtitle}>{person.email}</p>
        </div>
        <Button variant="secondary" size="md" onClick={() => setShowEdit(true)}>
          Modifica
        </Button>
      </header>

      <section className={cardStyles.card}>
        <h3 className={cardStyles.cardTitle}>Dati e appartenenze</h3>
        <dl className={cardStyles.grid}>
          <div className={cardStyles.item}>
            <dt>Stato</dt>
            <dd>{PERSON_STATUS_LABELS[person.status] ?? person.status}</dd>
          </div>
          <div className={cardStyles.item}>
            <dt>Gestione</dt>
            <dd>{person.directoryExempt ? 'Manuale' : 'Da directory'}</dd>
          </div>
          <div className={cardStyles.item}>
            <dt>Team</dt>
            <dd>
              {person.teams.length === 0
                ? '—'
                : person.teams.map((t) => (t.role === 'lead' ? `${t.name} (lead)` : t.name)).join(', ')}
            </dd>
          </div>
          <div className={cardStyles.item}>
            <dt>Gruppi locali</dt>
            <dd>{person.groups.length === 0 ? '—' : person.groups.map((g) => g.name).join(', ')}</dd>
          </div>
        </dl>
      </section>

      <section className={cardStyles.card}>
        <h3 className={cardStyles.cardTitle}>Iscrizioni</h3>
        {person.enrollments.length === 0 ? (
          <p>Nessuna iscrizione registrata.</p>
        ) : (
          <div className={listStyles.tableWrap}>
            <table className={listStyles.table}>
              <thead>
                <tr>
                  <th>Corso</th>
                  <th>Stato</th>
                  <th>Esito</th>
                  <th>Periodo</th>
                  <th>Evento</th>
                </tr>
              </thead>
              <tbody>
                {person.enrollments.map((e) => (
                  <tr key={e.enrollmentId}>
                    <td>{e.courseTitle}</td>
                    <td>
                      <StatusBadge
                        value={e.deliveryStatus}
                        label={DELIVERY_STATUS_LABELS[e.deliveryStatus] ?? e.deliveryStatus}
                        variant={deliveryStatusVariant(e.deliveryStatus)}
                      />
                    </td>
                    <td>{e.learningOutcome ? (LEARNING_OUTCOME_LABELS[e.learningOutcome] ?? e.learningOutcome) : '—'}</td>
                    <td>
                      {e.actualStart ? formatInstantDate(e.actualStart) : '—'}
                      {e.actualEnd ? ` – ${formatInstantDate(e.actualEnd)}` : ''}
                    </td>
                    <td>
                      <Link to={`/eventi/${e.eventId}`}>Apri evento</Link>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <AwardsSection personId={person.id} awards={person.awards} enrollments={person.enrollments} />
      <AssessmentsSection personId={person.id} assessments={person.assessments} />
      <PersonPathsSection personId={person.id} paths={person.paths} />

      <section className={cardStyles.card}>
        <h3 className={cardStyles.cardTitle}>Richieste</h3>
        {person.requests.length === 0 ? (
          <p>Nessuna richiesta registrata.</p>
        ) : (
          <div className={listStyles.tableWrap}>
            <table className={listStyles.table}>
              <thead>
                <tr>
                  <th>Corso o titolo</th>
                  <th>Esito</th>
                  <th>Creata il</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {person.requests.map((r) => (
                  <tr key={r.id}>
                    <td>{r.courseTitle || r.freeTextTitle || '—'}</td>
                    <td>{r.outcome ? (REQUEST_OUTCOME_LABELS[r.outcome] ?? r.outcome) : 'Aperta'}</td>
                    <td>{formatInstantDate(r.createdAt)}</td>
                    <td>
                      <Link to={`/richieste?id=${r.id}`}>Apri richiesta</Link>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <section className={cardStyles.card}>
        <h3 className={cardStyles.cardTitle}>Coperture delle regole</h3>
        {person.ruleCoverage.length === 0 ? (
          <p>Nessuna regola a platea la include.</p>
        ) : (
          <div className={listStyles.tableWrap}>
            <table className={listStyles.table}>
              <thead>
                <tr>
                  <th>Regola</th>
                  <th>Natura</th>
                  <th>Copertura</th>
                  <th>Scadenza</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {person.ruleCoverage.map((rc) => (
                  <tr key={rc.ruleId}>
                    <td>{rc.ruleName}</td>
                    <td>{NEED_LABELS[rc.need] ?? rc.need}</td>
                    <td>
                      <StatusBadge
                        value={rc.covered ? 'covered' : 'uncovered'}
                        label={rc.covered ? 'Coperta' : 'Scoperta'}
                        variant={rc.covered ? 'success' : 'warning'}
                      />
                    </td>
                    <td>{formatDateOnly(rc.deadline)}</td>
                    <td>
                      <Link to={`/regole?id=${rc.ruleId}`}>Apri regola</Link>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      {showEdit && (
        <PersonEditorModal
          mode="edit"
          personId={person.id}
          initial={person}
          open={showEdit}
          onClose={() => setShowEdit(false)}
          onSaved={() => setShowEdit(false)}
        />
      )}
    </main>
  );
}
