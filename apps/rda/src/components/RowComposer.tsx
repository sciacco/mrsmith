import { Button, Icon, useToast } from '@mrsmith/ui';
import { useMemo, useState, type FormEvent } from 'react';
import { useArticles, useCreateRow, useDeleteRow } from '../api/queries';
import type { PoRow, RowPayload } from '../api/types';
import { formatMoneyEUR } from '../lib/format';
import { firstError, validateRow, type ValidationResult } from '../lib/validation';
import { ArticleCombobox } from './ArticleCombobox';
import { ConfirmDialog } from './ConfirmDialog';

function emptyValidation(): ValidationResult {
  return { fieldErrors: {}, formErrors: [] };
}

export function RowComposer({
  poId,
  rows,
  editable,
}: {
  poId: number;
  rows: PoRow[];
  editable: boolean;
}) {
  const [type, setType] = useState<'good' | 'service'>('service');
  const [articleCode, setArticleCode] = useState('');
  const [description, setDescription] = useState('');
  const [qty, setQty] = useState(1);
  const [price, setPrice] = useState(0);
  const [nrc, setNrc] = useState(0);
  const [mrc, setMrc] = useState(0);
  const [duration, setDuration] = useState(12);
  const [recurrence, setRecurrence] = useState(1);
  const [startAt, setStartAt] = useState('activation_date');
  const [startDate, setStartDate] = useState('');
  const [automaticRenew, setAutomaticRenew] = useState(false);
  const [cancellationAdvice, setCancellationAdvice] = useState('');
  const [validation, setValidation] = useState<ValidationResult>(emptyValidation);
  const [deleteTarget, setDeleteTarget] = useState<PoRow | null>(null);
  const articles = useArticles(type, '');
  const createRow = useCreateRow();
  const remove = useDeleteRow();
  const { toast } = useToast();

  const selectedArticle = (articles.data ?? []).find((article) => article.code === articleCode);
  const preview = useMemo(() => {
    if (type === 'good') return price * qty;
    return mrc * qty * duration + nrc * qty;
  }, [duration, mrc, nrc, price, qty, type]);

  function reset() {
    setArticleCode('');
    setDescription('');
    setQty(1);
    setPrice(0);
    setNrc(0);
    setMrc(0);
    setDuration(12);
    setRecurrence(1);
    setStartAt('activation_date');
    setStartDate('');
    setAutomaticRenew(false);
    setCancellationAdvice('');
    setValidation(emptyValidation());
  }

  function buildPayload(): RowPayload {
    return {
      type,
      description: description.trim(),
      qty,
      product_code: articleCode,
      product_description: selectedArticle?.description ?? '',
      ...(type === 'good' ? { price } : { montly_fee: mrc, activation_price: nrc }),
      payment_detail: {
        start_at: startAt,
        ...(startAt === 'specific_date' ? { start_at_date: startDate } : {}),
        ...(type === 'service' ? { month_recursion: recurrence } : {}),
      },
      ...(type === 'service'
        ? {
            renew_detail: {
              initial_subscription_months: duration,
              automatic_renew: automaticRenew,
              cancellation_advice: cancellationAdvice,
            },
          }
        : {}),
    };
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const body = buildPayload();
    const result = validateRow(body);
    setValidation(result);
    const message = firstError(result);
    if (message) {
      toast(message, 'warning');
      return;
    }
    try {
      await createRow.mutateAsync({ id: poId, body });
      toast('Riga aggiunta');
      reset();
    } catch {
      toast('Salvataggio non riuscito', 'error');
    }
  }

  async function confirmDelete() {
    if (!deleteTarget) return;
    try {
      await remove.mutateAsync({ id: poId, rowId: deleteTarget.id });
      toast('Riga eliminata');
      setDeleteTarget(null);
    } catch {
      toast('Eliminazione non riuscita', 'error');
    }
  }

  function selectArticle(code: string) {
    setArticleCode(code);
    const article = (articles.data ?? []).find((item) => item.code === code);
    if (article?.description && !description.trim()) setDescription(article.description);
  }

  function switchType(next: 'good' | 'service') {
    setType(next);
    setArticleCode('');
    setValidation(emptyValidation());
  }

  return (
    <div className="stack">
      {editable ? (
        <form className="rowComposer" onSubmit={(event) => void submit(event)}>
          <div className="field">
            <label>Tipo riga</label>
            <select value={type} onChange={(event) => switchType(event.target.value as 'good' | 'service')}>
              <option value="service">Servizio</option>
              <option value="good">Bene</option>
            </select>
          </div>
          <div className="field wide">
            <label>Articolo</label>
            <ArticleCombobox articles={articles.data ?? []} value={articleCode} disabled={articles.isLoading} onChange={selectArticle} />
            {validation.fieldErrors.product_code ? <p className="fieldError">{validation.fieldErrors.product_code}</p> : null}
          </div>
          <div className="field">
            <label>Quantita</label>
            <input type="number" min="0" step="1" value={qty} onChange={(event) => setQty(Number(event.target.value))} />
            {validation.fieldErrors.qty ? <p className="fieldError">{validation.fieldErrors.qty}</p> : null}
          </div>
          <div className="field wide">
            <label>Descrizione</label>
            <input value={description} onChange={(event) => setDescription(event.target.value)} />
            {validation.fieldErrors.description ? <p className="fieldError">{validation.fieldErrors.description}</p> : null}
          </div>
          {type === 'good' ? (
            <div className="field">
              <label>Costo unitario</label>
              <input type="number" min="0" step="0.01" value={price} onChange={(event) => setPrice(Number(event.target.value))} />
              {validation.fieldErrors.price ? <p className="fieldError">{validation.fieldErrors.price}</p> : null}
            </div>
          ) : (
            <>
              <div className="field"><label>Costo una tantum</label><input type="number" min="0" step="0.01" value={nrc} onChange={(event) => setNrc(Number(event.target.value))} /></div>
              <div className="field"><label>Canone mensile</label><input type="number" min="0" step="0.01" value={mrc} onChange={(event) => setMrc(Number(event.target.value))} /></div>
              <div className="field">
                <label>Durata mesi</label>
                <input type="number" min="1" value={duration} onChange={(event) => setDuration(Number(event.target.value))} />
                {validation.fieldErrors.initial_subscription_months ? <p className="fieldError">{validation.fieldErrors.initial_subscription_months}</p> : null}
              </div>
              <div className="field">
                <label>Ricorrenza mesi</label>
                <select value={recurrence} onChange={(event) => setRecurrence(Number(event.target.value))}>
                  {[1, 3, 6, 12].map((value) => <option key={value} value={value}>{value}</option>)}
                </select>
              </div>
            </>
          )}
          <div className="field">
            <label>Decorrenza</label>
            <select value={startAt} onChange={(event) => setStartAt(event.target.value)}>
              <option value="activation_date">Data attivazione</option>
              {type === 'good' ? <option value="advance_payment">Pagamento anticipato</option> : null}
              <option value="specific_date">Data specifica</option>
            </select>
          </div>
          {startAt === 'specific_date' ? (
            <div className="field">
              <label>Data decorrenza</label>
              <input type="date" value={startDate} onChange={(event) => setStartDate(event.target.value)} />
              {validation.fieldErrors.start_at_date ? <p className="fieldError">{validation.fieldErrors.start_at_date}</p> : null}
            </div>
          ) : null}
          {type === 'service' ? (
            <>
              <label className="field checkboxField">
                <span>Rinnovo automatico</span>
                <input type="checkbox" checked={automaticRenew} onChange={(event) => setAutomaticRenew(event.target.checked)} />
              </label>
              {automaticRenew ? (
                <div className="field">
                  <label>Preavviso disdetta</label>
                  <input value={cancellationAdvice} onChange={(event) => setCancellationAdvice(event.target.value)} />
                  {validation.fieldErrors.cancellation_advice ? <p className="fieldError">{validation.fieldErrors.cancellation_advice}</p> : null}
                </div>
              ) : null}
            </>
          ) : null}
          {validation.formErrors.length ? <p className="fieldError wide">{validation.formErrors[0]}</p> : null}
          <div className="composerTotal">
            <span>Anteprima totale</span>
            <strong>{formatMoneyEUR(preview)}</strong>
          </div>
          <div className="actionRow fullWidth">
            <Button type="submit" leftIcon={<Icon name="plus" />} loading={createRow.isPending}>Aggiungi riga</Button>
          </div>
        </form>
      ) : null}

      <div className="tableScroll">
        <table className="dataTable">
          <thead>
            <tr>
              <th>Descrizione</th><th>Costo unitario</th><th>Canone mensile</th><th>Q.ta</th><th>Tipo</th><th>Totale riga</th><th className="actionsCell">Azioni</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={row.id}>
                <td>{row.description ?? row.product_description ?? '-'}</td>
                <td>{formatMoneyEUR(row.type === 'good' ? row.price : row.activation_fee ?? row.activation_price)}</td>
                <td>{formatMoneyEUR(row.montly_fee ?? row.monthly_fee)}</td>
                <td>{row.qty ?? '-'}</td>
                <td>{row.type === 'good' ? 'Bene' : 'Servizio'}</td>
                <td>{formatMoneyEUR(row.total_price)}</td>
                <td className="actionsCell">
                  <button
                    className="iconButton dangerButton"
                    type="button"
                    aria-label="Elimina riga"
                    title="Elimina"
                    disabled={!editable}
                    onClick={() => setDeleteTarget(row)}
                  >
                    <Icon name="trash" size={16} />
                  </button>
                </td>
              </tr>
            ))}
            {rows.length === 0 ? <tr><td colSpan={7} className="emptyInline">Aggiungi almeno una riga.</td></tr> : null}
          </tbody>
        </table>
      </div>

      <ConfirmDialog
        open={deleteTarget != null}
        title="Elimina riga"
        message="Confermi eliminazione della riga selezionata?"
        confirmLabel="Elimina"
        danger
        loading={remove.isPending}
        onClose={() => setDeleteTarget(null)}
        onConfirm={() => void confirmDelete()}
      />
    </div>
  );
}
