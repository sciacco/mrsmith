// Spese dell'evento: voci con riferimento PO (letto live da Arak), stato
// economico, budget di competenza e iscrizioni coperte. Prezzo pattuito
// (evento) e costo PO (spesa) restano fatti distinti — questa sezione mostra
// solo il secondo. Gli errori po_not_found/po_reference_ambiguous/
// event_expense_po_duplicate si mostrano sul campo del riferimento PO (#156).

import { useState } from 'react';
import { formatCurrency } from '@mrsmith/format';
import { Button, Icon, Modal, MultiSelect, StatusBadge, useToast, VisuallyHidden } from '@mrsmith/ui';
import { useCreateExpense, useDeleteExpense, useReplaceExpenseEnrollments, useReplaceExpensePO } from '../../api/queries';
import type { EnrollmentDetail, EventExpense } from '../../api/types';
import { apiErrorCode, describeApiError } from '../../components/events/apiErrors';
import { ErrorPanel } from '../../components/events/ErrorPanel';
import { economicStateVariant } from '../../components/events/statusVariants';
import { ECONOMIC_STATE_LABELS } from '../../lib/labels';
import styles from './EventDetailPage.module.css';

const PO_FIELD_ERROR_CODES = new Set(['po_not_found', 'po_reference_ambiguous', 'event_expense_po_duplicate']);

interface ExpensesSectionProps {
  eventId: string;
  expenses: EventExpense[];
  enrollments: EnrollmentDetail[];
  highlighted: boolean;
}

function enrollmentNames(ids: string[], enrollments: EnrollmentDetail[]): string {
  if (ids.length === 0) return '—';
  const names = ids.map((id) => enrollments.find((e) => e.id === id)?.employeeName ?? id);
  return names.join(', ');
}

export function ExpensesSection({ eventId, expenses, enrollments, highlighted }: ExpensesSectionProps) {
  const [showAdd, setShowAdd] = useState(false);
  const [replaceTarget, setReplaceTarget] = useState<EventExpense | null>(null);
  const [coverageTarget, setCoverageTarget] = useState<EventExpense | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<EventExpense | null>(null);

  return (
    <section id="section-expenses" className={`${styles.section} ${highlighted ? styles.sectionHighlighted : ''}`}>
      <div className={styles.sectionHeader}>
        <h2 className={styles.sectionTitle}>Spese</h2>
        <Button variant="secondary" size="sm" leftIcon={<Icon name="plus" size={14} />} onClick={() => setShowAdd(true)}>
          Aggiungi voce
        </Button>
      </div>

      {expenses.length === 0 ? (
        <p className={styles.emptyNotice}>Nessuna voce di spesa collegata a questo evento.</p>
      ) : (
        <div className={styles.tableWrap}>
          <table className={styles.table}>
            <thead>
              <tr>
                <th>PO</th>
                <th className={styles.numCell}>Costo PO</th>
                <th>Stato economico</th>
                <th>Budget di competenza</th>
                <th>Iscrizioni coperte</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {expenses.map((expense) => (
                <tr key={expense.id}>
                  <td>{expense.poCode || expense.poId}</td>
                  <td className={styles.numCell}>
                    {formatCurrency(Number(expense.totalPrice), expense.currency || 'EUR') ?? expense.totalPrice}
                  </td>
                  <td>
                    <StatusBadge
                      value={expense.economicState}
                      label={ECONOMIC_STATE_LABELS[expense.economicState]}
                      variant={economicStateVariant(expense.economicState)}
                      tooltip={`Stato PO in Arak: ${expense.rawState}`}
                    />
                  </td>
                  <td>{expense.budget.name || '—'}</td>
                  <td>{enrollmentNames(expense.enrollmentIds, enrollments)}</td>
                  <td className={styles.rowActions}>
                    <button type="button" className={styles.linkButton} onClick={() => setReplaceTarget(expense)}>
                      Sostituisci PO
                    </button>
                    <button type="button" className={styles.linkButton} onClick={() => setCoverageTarget(expense)}>
                      Coperture
                    </button>
                    <button type="button" className={styles.linkButtonDanger} onClick={() => setDeleteTarget(expense)}>
                      Scollega
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <AddExpenseModal
        open={showAdd}
        eventId={eventId}
        enrollments={enrollments}
        onClose={() => setShowAdd(false)}
      />
      {replaceTarget && (
        <ReplacePoModal key={replaceTarget.id} expense={replaceTarget} onClose={() => setReplaceTarget(null)} />
      )}
      {coverageTarget && (
        <CoverageModal
          key={coverageTarget.id}
          expense={coverageTarget}
          enrollments={enrollments}
          onClose={() => setCoverageTarget(null)}
        />
      )}
      {deleteTarget && <UnlinkModal key={deleteTarget.id} expense={deleteTarget} onClose={() => setDeleteTarget(null)} />}
    </section>
  );
}

// PoReferenceField: campo unico condiviso da "Aggiungi voce" e "Sostituisci
// PO". L'errore sul riferimento (PO non trovato/ambiguo/gia collegato) si
// mostra sotto il campo; ogni altro errore resta al chiamante.
function PoReferenceField({
  value,
  onChange,
  fieldError,
}: {
  value: string;
  onChange: (v: string) => void;
  fieldError: string | null;
}) {
  return (
    <label className={styles.field}>
      <span className={styles.labelHead}>
        Riferimento PO
        <span className={styles.requiredMarker} aria-hidden="true" />
        <VisuallyHidden>obbligatorio</VisuallyHidden>
      </span>
      <input
        className={styles.input}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder="ID o codice PO"
        required
        aria-invalid={fieldError ? 'true' : undefined}
      />
      <span className={styles.helperText}>ID numerico oppure codice PO.</span>
      {fieldError && <ErrorPanel message={fieldError} />}
    </label>
  );
}

function AddExpenseModal({
  open,
  eventId,
  enrollments,
  onClose,
}: {
  open: boolean;
  eventId: string;
  enrollments: EnrollmentDetail[];
  onClose: () => void;
}) {
  const { toast } = useToast();
  const createExpense = useCreateExpense();
  const [poReference, setPoReference] = useState('');
  const [enrollmentIds, setEnrollmentIds] = useState<string[]>([]);
  const [fieldError, setFieldError] = useState<string | null>(null);
  const [generalError, setGeneralError] = useState<string | null>(null);

  if (!open) return null;

  const options = enrollments.map((e) => ({ value: e.id, label: e.employeeName }));

  function handleClose() {
    setPoReference('');
    setEnrollmentIds([]);
    setFieldError(null);
    setGeneralError(null);
    onClose();
  }

  async function submit() {
    setFieldError(null);
    setGeneralError(null);
    try {
      await createExpense.mutateAsync({ eventId, input: { poReference, enrollmentIds } });
      toast('Voce di spesa aggiunta');
      handleClose();
    } catch (error) {
      const code = apiErrorCode(error);
      if (code && PO_FIELD_ERROR_CODES.has(code)) setFieldError(describeApiError(error, 'Riferimento non valido'));
      else setGeneralError(describeApiError(error, 'Aggiunta non riuscita'));
    }
  }

  return (
    <Modal open={open} onClose={handleClose} title="Aggiungi voce di spesa" size="md">
      <div className={styles.formBody}>
        <PoReferenceField value={poReference} onChange={setPoReference} fieldError={fieldError} />
        <label className={styles.field}>
          Iscrizioni coperte
          <MultiSelect options={options} selected={enrollmentIds} onChange={setEnrollmentIds} placeholder="Nessuna" />
        </label>
        <ErrorPanel message={generalError} />
        <div className={styles.actions}>
          <Button variant="ghost" size="md" onClick={handleClose} disabled={createExpense.isPending}>
            Annulla
          </Button>
          <Button
            variant="primary"
            size="md"
            loading={createExpense.isPending}
            disabled={poReference.trim() === ''}
            onClick={submit}
          >
            Aggiungi
          </Button>
        </div>
      </div>
    </Modal>
  );
}

function ReplacePoModal({ expense, onClose }: { expense: EventExpense; onClose: () => void }) {
  const { toast } = useToast();
  const replacePo = useReplaceExpensePO();
  const [poReference, setPoReference] = useState('');
  const [fieldError, setFieldError] = useState<string | null>(null);
  const [generalError, setGeneralError] = useState<string | null>(null);

  function handleClose() {
    setPoReference('');
    setFieldError(null);
    setGeneralError(null);
    onClose();
  }

  async function submit() {
    setFieldError(null);
    setGeneralError(null);
    try {
      await replacePo.mutateAsync({ id: expense.id, input: { poReference } });
      toast('PO sostituito');
      handleClose();
    } catch (error) {
      const code = apiErrorCode(error);
      if (code && PO_FIELD_ERROR_CODES.has(code)) setFieldError(describeApiError(error, 'Riferimento non valido'));
      else setGeneralError(describeApiError(error, 'Sostituzione non riuscita'));
    }
  }

  return (
    <Modal open onClose={handleClose} title="Sostituisci PO" size="sm">
      <div className={styles.formBody}>
        <p className={styles.helperText}>PO attuale: {expense.poCode || expense.poId}. Le coperture restano invariate.</p>
        <PoReferenceField value={poReference} onChange={setPoReference} fieldError={fieldError} />
        <ErrorPanel message={generalError} />
        <div className={styles.actions}>
          <Button variant="ghost" size="md" onClick={handleClose} disabled={replacePo.isPending}>
            Annulla
          </Button>
          <Button
            variant="primary"
            size="md"
            loading={replacePo.isPending}
            disabled={poReference.trim() === ''}
            onClick={submit}
          >
            Sostituisci
          </Button>
        </div>
      </div>
    </Modal>
  );
}

function CoverageModal({
  expense,
  enrollments,
  onClose,
}: {
  expense: EventExpense;
  enrollments: EnrollmentDetail[];
  onClose: () => void;
}) {
  const { toast } = useToast();
  const replaceCoverage = useReplaceExpenseEnrollments();
  const [enrollmentIds, setEnrollmentIds] = useState<string[]>(expense.enrollmentIds);
  const [error, setError] = useState<string | null>(null);

  const options = enrollments.map((e) => ({ value: e.id, label: e.employeeName }));

  function handleClose() {
    setError(null);
    onClose();
  }

  async function submit() {
    setError(null);
    try {
      await replaceCoverage.mutateAsync({ id: expense.id, input: { enrollmentIds } });
      toast('Coperture aggiornate');
      handleClose();
    } catch (submitError) {
      setError(describeApiError(submitError, 'Aggiornamento non riuscito'));
    }
  }

  return (
    <Modal open onClose={handleClose} title="Modifica coperture" size="sm">
      <div className={styles.formBody}>
        <label className={styles.field}>
          Iscrizioni coperte
          <MultiSelect options={options} selected={enrollmentIds} onChange={setEnrollmentIds} placeholder="Nessuna" />
        </label>
        <ErrorPanel message={error} />
        <div className={styles.actions}>
          <Button variant="ghost" size="md" onClick={handleClose} disabled={replaceCoverage.isPending}>
            Annulla
          </Button>
          <Button variant="primary" size="md" loading={replaceCoverage.isPending} onClick={submit}>
            Salva coperture
          </Button>
        </div>
      </div>
    </Modal>
  );
}

function UnlinkModal({ expense, onClose }: { expense: EventExpense; onClose: () => void }) {
  const { toast } = useToast();
  const deleteExpense = useDeleteExpense();
  const [error, setError] = useState<string | null>(null);

  async function confirm() {
    setError(null);
    try {
      await deleteExpense.mutateAsync(expense.id);
      toast('Voce di spesa scollegata');
      onClose();
    } catch (submitError) {
      setError(describeApiError(submitError, 'Operazione non riuscita'));
    }
  }

  return (
    <Modal open onClose={onClose} title="Scollega spesa" size="sm">
      <div className={styles.formBody}>
        <p>Scollegare il PO {expense.poCode || expense.poId} da questo evento?</p>
        <ErrorPanel message={error} />
        <div className={styles.actions}>
          <Button variant="ghost" size="md" onClick={onClose} disabled={deleteExpense.isPending}>
            Annulla
          </Button>
          <Button variant="danger" size="md" loading={deleteExpense.isPending} onClick={confirm}>
            Scollega
          </Button>
        </div>
      </div>
    </Modal>
  );
}
