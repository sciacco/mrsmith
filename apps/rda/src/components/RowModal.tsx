import { Button, Icon, Modal, useToast } from '@mrsmith/ui';
import { useState } from 'react';
import { useArticleCatalog, useCreateRow } from '../api/queries';
import type { Article } from '../api/types';
import { apiErrorMessage } from '../lib/api-error';
import { formatMoneyEUR } from '../lib/format';
import { buildRowPayload, rowPreviewTotal } from '../lib/row-payload';
import { firstError, validateRow } from '../lib/validation';
import { ArticleCombobox } from './ArticleCombobox';

export function RowModal({ poId, open, onClose }: { poId: number; open: boolean; onClose: () => void }) {
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
  const articles = useArticleCatalog();
  const createRow = useCreateRow();
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
  }

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const body = buildRowPayload(draft);
    const message = firstError(validateRow(body));
    if (message) {
      toast(message, 'warning');
      return;
    }
    try {
      await createRow.mutateAsync({ id: poId, body });
      toast('Riga aggiunta');
      reset();
      onClose();
    } catch (error) {
      toast(apiErrorMessage(error, 'Salvataggio non riuscito'), 'error');
    }
  }

  function selectArticle(article: Article | null) {
    const previousType = selectedArticle?.type;
    setSelectedArticle(article);
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
    <Modal open={open} onClose={onClose} title="Nuova riga PO" size="wide">
      <form className="formGrid three" onSubmit={(event) => void submit(event)}>
        <div className="articleQuantityRow">
          <div className="field quantityField">
            <label>Quantita</label>
            <input type="number" min="0" step="1" value={qty} onChange={(event) => setQty(Number(event.target.value))} />
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
        </div>
        {selectedType === 'good' ? (
          <div className="field">
            <label>Costo unitario</label>
            <input type="number" min="0" step="0.01" value={price} onChange={(event) => setPrice(Number(event.target.value))} />
          </div>
        ) : (
          <>
            <div className="field"><label>NRC</label><input type="number" min="0" step="0.01" value={nrc} onChange={(event) => setNrc(Number(event.target.value))} /></div>
            <div className="field"><label>MRC</label><input type="number" min="0" step="0.01" value={mrc} onChange={(event) => setMrc(Number(event.target.value))} /></div>
            <div className="field"><label>Durata mesi</label><input type="number" min="1" value={duration} onChange={(event) => setDuration(Number(event.target.value))} /></div>
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
          <div className="field"><label>Data decorrenza</label><input type="date" value={startDate} onChange={(event) => setStartDate(event.target.value)} /></div>
        ) : null}
        {selectedType === 'service' ? (
          <>
            <label className="field"><span>Rinnovo automatico</span><input type="checkbox" checked={automaticRenew} onChange={(event) => setAutomaticRenew(event.target.checked)} /></label>
            {automaticRenew ? (
              <div className="field"><label>Preavviso disdetta</label><input value={cancellationAdvice} onChange={(event) => setCancellationAdvice(event.target.value)} /></div>
            ) : null}
          </>
        ) : null}
        <p className="muted fullWidth">Anteprima totale: {formatMoneyEUR(preview)}. Il totale finale e calcolato dal servizio RDA.</p>
        <div className="modalActions fullWidth">
          <Button variant="secondary" onClick={onClose}>Annulla</Button>
          <Button type="submit" leftIcon={<Icon name="plus" />} loading={createRow.isPending}>Aggiungi riga</Button>
        </div>
          </>
        )}
      </form>
    </Modal>
  );
}
