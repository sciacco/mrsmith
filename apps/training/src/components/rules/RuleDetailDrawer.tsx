// Dettaglio regola formativa (#157, §Regole 3): platea risolta con coperti
// per persona (o posizioni richieste/coperte), scadenza e prossima tornata,
// tornate esistenti con rimando al dettaglio evento. «Crea evento della
// tornata» è offerta solo su regola attiva (rule_inactive è quindi escluso
// per costruzione); i due 409 raggiungibili dall'operatore — no_next_round,
// round_already_exists — restano mostrati per intero dal backend, nessuna
// previsione locale di quando la tornata è disponibile.

import { useState } from 'react';
import { Link } from 'react-router-dom';
import { Button, Drawer, Skeleton, StatusBadge, ToggleSwitch, useToast } from '@mrsmith/ui';
import { useCreateRuleEvent, useRuleDetail, useSetRuleActive } from '../../api/queries';
import { describeApiError } from '../events/apiErrors';
import { ErrorPanel } from '../events/ErrorPanel';
import { formatDateOnly } from '../events/eventFormat';
import { NEED_LABELS, RECURRENCE_ANCHOR_LABELS } from '../../lib/labels';
import { RuleEditorModal } from './RuleEditorModal';
import styles from '../requests/drawerShared.module.css';
import localStyles from './RuleDetailDrawer.module.css';

interface RuleDetailDrawerProps {
  id: string;
  onClose: () => void;
}

export function RuleDetailDrawer({ id, onClose }: RuleDetailDrawerProps) {
  const { toast } = useToast();
  const detail = useRuleDetail(id);
  const setActive = useSetRuleActive();
  const createRuleEvent = useCreateRuleEvent();

  const [showEdit, setShowEdit] = useState(false);
  const [activeError, setActiveError] = useState<string | null>(null);
  const [roundError, setRoundError] = useState<string | null>(null);
  const [createdRound, setCreatedRound] = useState<{ id: string; enrollmentsCreated: number } | null>(null);

  const rule = detail.data;

  async function toggleActive() {
    if (!rule) return;
    setActiveError(null);
    try {
      await setActive.mutateAsync({ id: rule.id, active: !rule.isActive });
      toast(rule.isActive ? 'Regola disattivata' : 'Regola attivata');
    } catch (e) {
      setActiveError(describeApiError(e, `Impossibile ${rule.isActive ? 'disattivare' : 'attivare'} la regola`));
    }
  }

  async function createRound() {
    if (!rule) return;
    setRoundError(null);
    setCreatedRound(null);
    try {
      const response = await createRuleEvent.mutateAsync(rule.id);
      setCreatedRound({ id: response.id, enrollmentsCreated: response.enrollmentsCreated });
      toast(`Tornata creata: ${response.enrollmentsCreated} iscrizioni generate`);
    } catch (e) {
      setRoundError(describeApiError(e, 'Creazione della tornata non riuscita'));
    }
  }

  return (
    <>
      <Drawer
        open
        onClose={onClose}
        title={rule?.name ?? 'Regola'}
        subtitle={rule?.courseTitle}
        size="lg"
        footer={
          rule ? (
            <div className={styles.footerActions}>
              <Button variant="ghost" size="md" onClick={() => setShowEdit(true)}>
                Modifica
              </Button>
              <Button
                variant="primary"
                size="md"
                loading={createRuleEvent.isPending}
                disabled={!rule.isActive}
                onClick={createRound}
              >
                Crea evento della tornata
              </Button>
            </div>
          ) : undefined
        }
      >
        <div className={styles.body}>
          {detail.isLoading && <Skeleton rows={6} />}
          {detail.isError && (
            <p className={styles.errorNotice}>{describeApiError(detail.error, 'Lettura della regola non riuscita')}</p>
          )}
          {rule && (
            <>
              <section className={styles.card}>
                <div className={localStyles.cardHeader}>
                  <h3 className={styles.cardTitle}>Regola</h3>
                  <ToggleSwitch id={`rule-active-${rule.id}`} checked={rule.isActive} onChange={toggleActive} label="Attiva" />
                </div>
                <ErrorPanel message={activeError} onDismiss={() => setActiveError(null)} />
                <dl className={styles.grid}>
                  <div className={styles.item}>
                    <dt>Natura del bisogno</dt>
                    <dd>{NEED_LABELS[rule.need] ?? rule.need}</dd>
                  </div>
                  <div className={styles.item}>
                    <dt>Obbligatoria</dt>
                    <dd>{rule.isMandatory ? 'Sì' : 'No'}</dd>
                  </div>
                  <div className={styles.item}>
                    <dt>Scadenza</dt>
                    <dd>{formatDateOnly(rule.deadline)}</dd>
                  </div>
                  <div className={styles.item}>
                    <dt>Prossima tornata</dt>
                    <dd>{rule.nextRoundDeadline ? formatDateOnly(rule.nextRoundDeadline) : '—'}</dd>
                  </div>
                  <div className={styles.item}>
                    <dt>Ricorrenza</dt>
                    <dd>
                      {rule.recurrenceMonths
                        ? `${rule.recurrenceMonths} mesi · ${
                            rule.recurrenceAnchor
                              ? (RECURRENCE_ANCHOR_LABELS[rule.recurrenceAnchor] ?? rule.recurrenceAnchor)
                              : ''
                          }`
                        : 'Nessuna'}
                    </dd>
                  </div>
                  <div className={styles.item}>
                    <dt>Note</dt>
                    <dd>{rule.notes || '—'}</dd>
                  </div>
                </dl>
              </section>

              <section className={styles.card}>
                <h3 className={styles.cardTitle}>{rule.population ? 'Platea' : 'Posizioni'}</h3>
                {rule.seats && (
                  <p>
                    Coperte {rule.seats.covered} di {rule.seats.requested} richieste.
                  </p>
                )}
                {rule.population && (
                  <div className={localStyles.tableWrap}>
                    <table className={localStyles.miniTable}>
                      <thead>
                        <tr>
                          <th>Persona</th>
                          <th>Copertura</th>
                        </tr>
                      </thead>
                      <tbody>
                        {rule.population.members.map((m) => (
                          <tr key={m.employeeId}>
                            <td>{m.name}</td>
                            <td>
                              <StatusBadge
                                value={m.covered ? 'covered' : 'uncovered'}
                                label={m.covered ? 'Coperta' : 'Scoperta'}
                                variant={m.covered ? 'success' : 'warning'}
                              />
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
              </section>

              <section className={styles.card}>
                <h3 className={styles.cardTitle}>Tornate</h3>
                {createdRound && (
                  <p className={localStyles.createdBanner}>
                    Tornata creata ({createdRound.enrollmentsCreated} iscrizioni) ·{' '}
                    <Link to={`/eventi/${createdRound.id}`}>Apri evento</Link>
                  </p>
                )}
                <ErrorPanel message={roundError} onDismiss={() => setRoundError(null)} />
                {rule.rounds.length === 0 ? (
                  <p>Nessuna tornata registrata.</p>
                ) : (
                  <div className={localStyles.tableWrap}>
                    <table className={localStyles.miniTable}>
                      <thead>
                        <tr>
                          <th>Evento</th>
                          <th>Scadenza tornata</th>
                          <th>Iscrizioni</th>
                          <th>Annullata</th>
                        </tr>
                      </thead>
                      <tbody>
                        {rule.rounds.map((round) => (
                          <tr key={round.eventId}>
                            <td>
                              <Link to={`/eventi/${round.eventId}`}>Apri evento</Link>
                            </td>
                            <td>{round.ruleDeadline ? formatDateOnly(round.ruleDeadline) : '—'}</td>
                            <td>{round.enrollmentsCount}</td>
                            <td>{round.cancelled ? 'Sì' : 'No'}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
              </section>
            </>
          )}
        </div>
      </Drawer>

      {rule && showEdit && (
        <RuleEditorModal
          mode="edit"
          ruleId={rule.id}
          initial={rule}
          open={showEdit}
          onClose={() => setShowEdit(false)}
          onSaved={() => setShowEdit(false)}
        />
      )}
    </>
  );
}
