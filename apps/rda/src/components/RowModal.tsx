import { Button, Icon, Modal, useToast } from '@mrsmith/ui';
import { useMemo, useState } from 'react';
import { useArticles, useCreateRow } from '../api/queries';
import type { RowPayload } from '../api/types';
import { formatMoneyEUR } from '../lib/format';
import { firstError, validateRow } from '../lib/validation';

export function RowModal({ poId, open, onClose }: { poId: number; open: boolean; onClose: () => void }) {
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
  const articles = useArticles(type, '');
  const createRow = useCreateRow();
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
  }

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const body: RowPayload = {
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
    } catch {
      toast('Salvataggio non riuscito', 'error');
    }
  }

  return (
    <Modal open={open} onClose={onClose} title="Nuova riga PO" size="wide">
      <form className="formGrid three" onSubmit={(event) => void submit(event)}>
        <div className="field">
          <label>Tipo</label>
          <select value={type} onChange={(event) => setType(event.target.value as 'good' | 'service')}>
            <option value="service">Servizio</option>
            <option value="good">Bene</option>
          </select>
        </div>
        <div className="field">
          <label>Articolo</label>
          <select value={articleCode} onChange={(event) => setArticleCode(event.target.value)}>
            <option value="">Seleziona articolo</option>
            {(articles.data ?? []).map((article) => (
              <option key={article.code} value={article.code}>{article.description ?? article.code}</option>
            ))}
          </select>
        </div>
        <div className="field">
          <label>Quantita</label>
          <input type="number" min="0" step="1" value={qty} onChange={(event) => setQty(Number(event.target.value))} />
        </div>
        <div className="field wide">
          <label>Descrizione</label>
          <input value={description} onChange={(event) => setDescription(event.target.value)} />
        </div>
        {type === 'good' ? (
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
            {type === 'good' ? <option value="advance_payment">Pagamento anticipato</option> : null}
            <option value="specific_date">Data specifica</option>
          </select>
        </div>
        {startAt === 'specific_date' ? (
          <div className="field"><label>Data decorrenza</label><input type="date" value={startDate} onChange={(event) => setStartDate(event.target.value)} /></div>
        ) : null}
        {type === 'service' ? (
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
      </form>
    </Modal>
  );
}
