import { Button, Icon, useToast } from '@mrsmith/ui';
import { useEffect, useState, type FormEvent } from 'react';
import { useArticleCatalog, useCreateRow, useDeleteRow } from '../api/queries';
import type { Article, PoRow } from '../api/types';
import { apiErrorMessage } from '../lib/api-error';
import { formatMoneyEUR } from '../lib/format';
import { buildRowPayload, rowPreviewTotal } from '../lib/row-payload';
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
  onPreviewTotalChange,
}: {
  poId: number;
  rows: PoRow[];
  editable: boolean;
  onPreviewTotalChange?: (previewTotal: number) => void;
}) {
  const [articleSearch, setArticleSearch] = useState('');
  const [selectedArticle, setSelectedArticle] = useState<Article | null>(null);
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
  const articles = useArticleCatalog();
  const createRow = useCreateRow();
  const remove = useDeleteRow();
  const { toast } = useToast();

  const draft = {
    article: selectedArticle,
    description,
    qty,
    price,
    nrc,
    mrc,
    duration,
    recurrence,
    startAt,
    startDate,
    automaticRenew,
    cancellationAdvice,
  };
  const preview = selectedArticle ? rowPreviewTotal(draft) : 0;
  const selectedType = selectedArticle?.type;

  useEffect(() => {
    onPreviewTotalChange?.(preview);
  }, [onPreviewTotalChange, preview]);

  useEffect(() => () => onPreviewTotalChange?.(0), [onPreviewTotalChange]);

  function reset() {
    setSelectedArticle(null);
    setArticleSearch('');
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

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const body = buildRowPayload(draft);
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
    } catch (error) {
      toast(apiErrorMessage(error, 'Salvataggio non riuscito'), 'error');
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

  function selectArticle(article: Article | null) {
    const previousType = selectedArticle?.type;
    setSelectedArticle(article);
    setValidation(emptyValidation());
    if (!article) return;
    setDescription(article.description ?? article.code);
    if (previousType && previousType !== article.type) {
      if (article.type === 'good') {
        setNrc(0);
        setMrc(0);
        setDuration(12);
        setRecurrence(1);
        setAutomaticRenew(false);
        setCancellationAdvice('');
      } else {
        setPrice(0);
        if (startAt === 'advance_payment') setStartAt('activation_date');
      }
    }
  }

  return (
    <div className="stack">
      {editable ? (
        <form className="rowComposer" onSubmit={(event) => void submit(event)}>
          <div className="articleQuantityRow">
            <div className="field quantityField">
              <label>Quantita</label>
              <input type="number" min="0" step="1" value={qty} onChange={(event) => setQty(Number(event.target.value))} />
              {validation.fieldErrors.qty ? <p className="fieldError">{validation.fieldErrors.qty}</p> : null}
            </div>
            <div className="field">
              <label>Articolo</label>
              <ArticleCombobox
                articles={articles.data ?? []}
                value={selectedArticle}
                search={articleSearch}
                loading={articles.isLoading}
                disabled={articles.isLoading && !selectedArticle}
                onSearchChange={setArticleSearch}
                onChange={selectArticle}
              />
              {validation.fieldErrors.product_code ? <p className="fieldError">{validation.fieldErrors.product_code}</p> : null}
            </div>
          </div>
          {!selectedArticle ? (
            <div className="lineBuilderEmpty wide">
              <Icon name="package" size={22} />
              <strong>Seleziona un articolo</strong>
              <span>Catalogo beni e servizi RDA</span>
            </div>
          ) : (
            <>
          <div className="field wide">
            <label>Descrizione</label>
            <input value={description} onChange={(event) => setDescription(event.target.value)} />
            {validation.fieldErrors.description ? <p className="fieldError">{validation.fieldErrors.description}</p> : null}
          </div>
          {selectedType === 'good' ? (
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
              {selectedType === 'good' ? <option value="advance_payment">Pagamento anticipato</option> : null}
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
          {selectedType === 'service' ? (
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
          <div className="actionRow fullWidth">
            <Button type="submit" leftIcon={<Icon name="plus" />} loading={createRow.isPending}>Aggiungi riga</Button>
          </div>
            </>
          )}
        </form>
      ) : null}

      <div className="tableScroll">
        <table className="dataTable rowTable">
          <thead>
            <tr>
              <th>Riga</th><th>Economia</th><th>Q.ta</th><th>Totale riga</th><th className="actionsCell">Azioni</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={row.id}>
                <td>
                  <div className="rowTitleCell">
                    <strong>{row.description ?? row.product_description ?? '-'}</strong>
                    <span>{row.product_code ?? row.product_description ?? '-'}</span>
                  </div>
                </td>
                <td>
                  <div className="economicBreakdown">
                    <span className={`badge ${row.type === 'good' ? 'success' : 'info'}`}>{row.type === 'good' ? 'Bene' : 'Servizio'}</span>
                    <small>{row.type === 'good' ? `Unitario ${formatMoneyEUR(row.price)}` : `NRC ${formatMoneyEUR(row.activation_fee ?? row.activation_price)} · MRC ${formatMoneyEUR(row.montly_fee ?? row.monthly_fee)}`}</small>
                  </div>
                </td>
                <td>{row.qty ?? '-'}</td>
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
            {rows.length === 0 ? <tr><td colSpan={5} className="emptyInline">Nessuna riga inserita.</td></tr> : null}
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
