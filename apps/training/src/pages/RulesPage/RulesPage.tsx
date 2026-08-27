// Lista delle regole formative (#157, §Regole 1): natura del bisogno
// derivata dal corso (si mostra, non si sceglie), platea con copertura
// coperte/richieste oppure posizioni, ricorrenza in mesi + àncora, prossima
// tornata, attiva/disattiva diretto dalla riga.

import { useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { Button, Icon, Skeleton, ToggleSwitch, useToast } from '@mrsmith/ui';
import { useSetRuleActive, useTrainingRules } from '../../api/queries';
import type { RuleListRow } from '../../api/types';
import { describeApiError } from '../../components/events/apiErrors';
import { ErrorPanel } from '../../components/events/ErrorPanel';
import { formatDateOnly } from '../../components/events/eventFormat';
import { RuleDetailDrawer } from '../../components/rules/RuleDetailDrawer';
import { RuleEditorModal } from '../../components/rules/RuleEditorModal';
import { NEED_LABELS, POPULATION_KIND_LABELS, RECURRENCE_ANCHOR_LABELS } from '../../lib/labels';
import styles from '../RequestsPage/listPage.module.css';

function populationLabel(row: RuleListRow): string {
  if (row.populationKind) {
    const kind = POPULATION_KIND_LABELS[row.populationKind] ?? row.populationKind;
    return row.populationSize !== undefined ? `${kind} · ${row.coveredCount}/${row.populationSize}` : kind;
  }
  if (row.seatCount !== undefined) return `${row.coveredCount}/${row.seatCount} posizioni`;
  return '—';
}

function recurrenceLabel(row: RuleListRow): string {
  if (!row.recurrenceMonths) return 'Nessuna';
  const anchor = row.recurrenceAnchor ? (RECURRENCE_ANCHOR_LABELS[row.recurrenceAnchor] ?? row.recurrenceAnchor) : '';
  return `${row.recurrenceMonths} mesi · ${anchor}`;
}

export function RulesPage() {
  const { toast } = useToast();
  const [params, setParams] = useSearchParams();
  const selectedId = params.get('id');
  const [showCreate, setShowCreate] = useState(false);
  const [toggleError, setToggleError] = useState<string | null>(null);

  const rules = useTrainingRules();
  const setActive = useSetRuleActive();

  function openDetail(id: string) {
    const next = new URLSearchParams(params);
    next.set('id', id);
    setParams(next, { replace: true });
  }

  function closeDetail() {
    const next = new URLSearchParams(params);
    next.delete('id');
    setParams(next, { replace: true });
  }

  async function toggleActive(row: RuleListRow) {
    setToggleError(null);
    try {
      await setActive.mutateAsync({ id: row.id, active: !row.isActive });
      toast(row.isActive ? 'Regola disattivata' : 'Regola attivata');
    } catch (e) {
      setToggleError(describeApiError(e, `Impossibile ${row.isActive ? 'disattivare' : 'attivare'} la regola`));
    }
  }

  return (
    <main className={styles.page}>
      <header className={styles.header}>
        <div>
          <h1 className={styles.title}>Regole</h1>
          <p className={styles.subtitle}>
            Regole formative: platea o posizioni, obbligatorietà, scadenza e ricorrenza delle tornate.
          </p>
        </div>
        <Button
          variant="primary"
          size="md"
          leftIcon={<Icon name="plus" size={16} />}
          onClick={() => setShowCreate(true)}
        >
          Nuova regola
        </Button>
      </header>

      <ErrorPanel message={toggleError} onDismiss={() => setToggleError(null)} />

      {rules.isLoading ? (
        <Skeleton rows={6} />
      ) : rules.isError ? (
        <p className={styles.errorNotice}>Lettura delle regole non riuscita. Riprovare più tardi.</p>
      ) : (rules.data ?? []).length === 0 ? (
        <div className={styles.empty}>
          <div className={styles.emptyIcon}>
            <Icon name="shield" size={32} />
          </div>
          <p className={styles.emptyTitle}>Nessuna regola</p>
          <p className={styles.emptyDescription}>
            Crea la prima regola per governare obbligatorietà e ricorrenza della formazione.
          </p>
          <Button variant="primary" size="md" onClick={() => setShowCreate(true)}>
            Nuova regola
          </Button>
        </div>
      ) : (
        <div className={styles.tableWrap}>
          <table className={styles.table}>
            <thead>
              <tr>
                <th>Nome</th>
                <th>Corso</th>
                <th>Natura</th>
                <th>Platea / posizioni</th>
                <th>Obbligatoria</th>
                <th>Scadenza</th>
                <th>Ricorrenza</th>
                <th>Prossima tornata</th>
                <th>Attiva</th>
              </tr>
            </thead>
            <tbody>
              {(rules.data ?? []).map((row) => (
                <tr key={row.id} className={styles.row} onClick={() => openDetail(row.id)}>
                  <td>
                    <button
                      type="button"
                      className={styles.rowLink}
                      onClick={(e) => {
                        e.stopPropagation();
                        openDetail(row.id);
                      }}
                    >
                      {row.name}
                    </button>
                  </td>
                  <td>{row.courseTitle}</td>
                  <td>{NEED_LABELS[row.need] ?? row.need}</td>
                  <td>{populationLabel(row)}</td>
                  <td>{row.isMandatory ? 'Sì' : 'No'}</td>
                  <td>{formatDateOnly(row.deadline)}</td>
                  <td>{recurrenceLabel(row)}</td>
                  <td>{row.nextRoundDeadline ? formatDateOnly(row.nextRoundDeadline) : '—'}</td>
                  <td onClick={(e) => e.stopPropagation()}>
                    <ToggleSwitch id={`active-${row.id}`} checked={row.isActive} onChange={() => toggleActive(row)} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {showCreate && (
        <RuleEditorModal
          mode="create"
          open={showCreate}
          onClose={() => setShowCreate(false)}
          onSaved={(id) => {
            setShowCreate(false);
            openDetail(id);
          }}
        />
      )}
      {selectedId && <RuleDetailDrawer id={selectedId} onClose={closeDetail} />}
    </main>
  );
}
