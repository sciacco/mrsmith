import { ApiError } from '@mrsmith/api-client';
import { Button, Icon, Modal, useToast } from '@mrsmith/ui';
import { useEffect, useMemo, useState, type FormEvent } from 'react';
import { useArticleCatalog, useCreateRow, useReplaceRow } from '../api/queries';
import type { Article, PoRow } from '../api/types';
import { apiErrorMessage } from '../lib/api-error';
import { formatMoney } from '../lib/format';
import { buildRowPayload, draftFromPoRow, emptyRowDraft, rowPreviewTotal, type RowPayloadDraft } from '../lib/row-payload';
import { firstError, validateRow, type ValidationResult } from '../lib/validation';
import { ArticleCombobox } from './ArticleCombobox';

function emptyValidation(): ValidationResult {
  return { fieldErrors: {}, formErrors: [] };
}

function isReplaceDeleteFailure(error: unknown): boolean {
  if (!(error instanceof ApiError)) return false;
  const body = error.body;
  return typeof body === 'object' && body !== null && 'code' in body && body.code === 'ROW_REPLACE_DELETE_FAILED';
}

export function RowModal({
  poId,
  currency,
  open,
  row,
  onClose,
}: {
  poId: number;
  currency?: string | null;
  open: boolean;
  row?: PoRow | null;
  onClose: () => void;
}) {
  const [articleSearch, setArticleSearch] = useState('');
  const [draft, setDraft] = useState<RowPayloadDraft>(() => emptyRowDraft());
  const [validation, setValidation] = useState<ValidationResult>(emptyValidation);
  const articles = useArticleCatalog();
  const createRow = useCreateRow();
  const replaceRow = useReplaceRow();
  const { toast } = useToast();

  const catalog = useMemo(() => articles.data ?? [], [articles.data]);
  const editing = row != null;
  const preview = draft.article ? rowPreviewTotal(draft) : 0;
  const selectedType = draft.article?.type;
  const saving = createRow.isPending || replaceRow.isPending;

  useEffect(() => {
    if (!open) return;
    setArticleSearch('');
    setValidation(emptyValidation());
    setDraft(row ? draftFromPoRow(row, catalog) : emptyRowDraft());
  }, [catalog, open, row]);

  function updateDraft<K extends keyof RowPayloadDraft>(key: K, value: RowPayloadDraft[K]) {
    setDraft((current) => ({ ...current, [key]: value }));
    setValidation(emptyValidation());
  }

  function selectArticle(article: Article | null) {
    const previousType = draft.article?.type;
    setValidation(emptyValidation());
    setDraft((current) => {
      if (!article) return { ...current, article: null };
      const next: RowPayloadDraft = {
        ...current,
        article,
        description: article.description ?? article.code,
      };
      if (previousType && previousType !== article.type) {
        if (article.type === 'good') {
          next.nrc = 0;
          next.mrc = 0;
          next.duration = 12;
          next.recurrence = 1;
          next.automaticRenew = false;
          next.cancellationAdvice = '';
        } else {
          next.price = 0;
          if (next.startAt === 'advance_payment') next.startAt = 'activation_date';
        }
      }
      return next;
    });
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
      if (row) {
        await replaceRow.mutateAsync({ id: poId, rowId: row.id, body });
        toast('Riga aggiornata');
      } else {
        await createRow.mutateAsync({ id: poId, body });
        toast('Riga aggiunta');
      }
      setDraft(emptyRowDraft());
      onClose();
    } catch (error) {
      toast(apiErrorMessage(error, 'Salvataggio non riuscito'), isReplaceDeleteFailure(error) ? 'warning' : 'error');
      if (isReplaceDeleteFailure(error)) onClose();
    }
  }

  return (
    <Modal open={open} onClose={onClose} title={editing ? 'Modifica riga' : 'Nuova riga PO'} size="wide">
      <form className="formGrid three" onSubmit={(event) => void submit(event)}>
        <div className="articleQuantityRow">
          <div className="field quantityField">
            <label>Quantita</label>
            <input type="number" min="0" step="1" value={draft.qty} onChange={(event) => updateDraft('qty', Number(event.target.value))} />
            {validation.fieldErrors.qty ? <p className="fieldError">{validation.fieldErrors.qty}</p> : null}
          </div>
          <div className="field">
            <label>Articolo</label>
            <ArticleCombobox
              articles={catalog}
              value={draft.article}
              search={articleSearch}
              loading={articles.isLoading}
              disabled={articles.isLoading && !draft.article}
              onSearchChange={setArticleSearch}
              onChange={selectArticle}
            />
            {validation.fieldErrors.product_code ? <p className="fieldError">{validation.fieldErrors.product_code}</p> : null}
          </div>
        </div>
        {!draft.article ? (
          <div className="lineBuilderEmpty wide">
            <Icon name="package" size={22} />
            <strong>Seleziona un articolo</strong>
            <span>Catalogo beni e servizi RDA</span>
          </div>
        ) : (
          <>
            <div className="field wide">
              <label>Descrizione</label>
              <input value={draft.description} onChange={(event) => updateDraft('description', event.target.value)} />
              {validation.fieldErrors.description ? <p className="fieldError">{validation.fieldErrors.description}</p> : null}
            </div>
            {selectedType === 'good' ? (
              <div className="field">
                <label>Costo unitario</label>
                <input type="number" min="0" step="0.01" value={draft.price} onChange={(event) => updateDraft('price', Number(event.target.value))} />
                {validation.fieldErrors.price ? <p className="fieldError">{validation.fieldErrors.price}</p> : null}
              </div>
            ) : (
              <>
                <div className="field">
                  <label>NRC</label>
                  <input type="number" min="0" step="0.01" value={draft.nrc} onChange={(event) => updateDraft('nrc', Number(event.target.value))} />
                </div>
                <div className="field">
                  <label>MRC</label>
                  <input type="number" min="0" step="0.01" value={draft.mrc} onChange={(event) => updateDraft('mrc', Number(event.target.value))} />
                </div>
                <div className="field">
                  <label>Durata mesi</label>
                  <input type="number" min="1" value={draft.duration} onChange={(event) => updateDraft('duration', Number(event.target.value))} />
                  {validation.fieldErrors.initial_subscription_months ? <p className="fieldError">{validation.fieldErrors.initial_subscription_months}</p> : null}
                </div>
                <div className="field">
                  <label>Ricorrenza mesi</label>
                  <select value={draft.recurrence} onChange={(event) => updateDraft('recurrence', Number(event.target.value))}>
                    {[1, 3, 6, 12].map((value) => (
                      <option key={value} value={value}>
                        {value}
                      </option>
                    ))}
                  </select>
                </div>
              </>
            )}
            <div className="field">
              <label>Decorrenza</label>
              <select value={draft.startAt} onChange={(event) => updateDraft('startAt', event.target.value)}>
                <option value="activation_date">Data attivazione</option>
                {selectedType === 'good' ? <option value="advance_payment">Pagamento anticipato</option> : null}
                <option value="specific_date">Data specifica</option>
              </select>
            </div>
            {draft.startAt === 'specific_date' ? (
              <div className="field">
                <label>Data decorrenza</label>
                <input type="date" value={draft.startDate} onChange={(event) => updateDraft('startDate', event.target.value)} />
                {validation.fieldErrors.start_at_date ? <p className="fieldError">{validation.fieldErrors.start_at_date}</p> : null}
              </div>
            ) : null}
            {selectedType === 'service' ? (
              <>
                <label className="field checkboxField">
                  <span>Rinnovo automatico</span>
                  <input type="checkbox" checked={draft.automaticRenew} onChange={(event) => updateDraft('automaticRenew', event.target.checked)} />
                </label>
                {draft.automaticRenew ? (
                  <div className="field">
                    <label>Preavviso disdetta</label>
                    <input value={draft.cancellationAdvice} onChange={(event) => updateDraft('cancellationAdvice', event.target.value)} />
                    {validation.fieldErrors.cancellation_advice ? <p className="fieldError">{validation.fieldErrors.cancellation_advice}</p> : null}
                  </div>
                ) : null}
              </>
            ) : null}
            {validation.formErrors.length ? <p className="fieldError fullWidth">{validation.formErrors[0]}</p> : null}
            <p className="muted fullWidth">Anteprima totale: {formatMoney(preview, currency)}. Il totale finale e calcolato dal servizio RDA.</p>
            <div className="modalActions fullWidth">
              <Button variant="secondary" onClick={onClose}>
                Annulla
              </Button>
              <Button type="submit" leftIcon={<Icon name={editing ? 'check' : 'plus'} />} loading={saving}>
                {editing ? 'Salva modifiche' : 'Aggiungi riga'}
              </Button>
            </div>
          </>
        )}
      </form>
    </Modal>
  );
}
