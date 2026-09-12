import { useState, type FormEvent } from 'react';
import { Button, Modal, MultiSelect, useToast } from '@mrsmith/ui';
import { useAddNeedRequests, useTrainingRequests } from '../../api/queries';
import type { NeedDetail } from '../../api/types';
import { REQUEST_OUTCOME_LABELS } from '../../lib/labels';
import { describeApiError } from '../events/apiErrors';
import { ErrorPanel } from '../events/ErrorPanel';
import form from '../requests/requestShared.module.css';

export function NeedRequestsModal({ need, onClose }: { need: NeedDetail; onClose: () => void }) {
  const requests = useTrainingRequests('all');
  const add = useAddNeedRequests();
  const { toast } = useToast();
  const [requestIds, setRequestIds] = useState<string[]>([]);
  const [error, setError] = useState<string | null>(null);
  const available = (requests.data ?? []).filter((r) => !need.requests.some((n) => n.id === r.id));

  async function submit(event: FormEvent) {
    event.preventDefault();
    setError(null);
    try {
      await add.mutateAsync({ id: need.id, requestIds });
      toast('Richieste collegate');
      onClose();
    } catch (e) { setError(describeApiError(e, 'Collegamento non riuscito')); }
  }

  return (
    <Modal open onClose={onClose} title="Collega richieste" size="lg">
      <form className={`${form.body} ${form.bodyModal}`} onSubmit={submit}>
        <div className={form.field}>Richieste
          <MultiSelect<string> selected={requestIds} onChange={setRequestIds} placeholder="Seleziona richieste…"
            options={available.map((r) => ({ value: r.id, label: `${r.employeeName} · ${r.description} · ${r.outcome ? REQUEST_OUTCOME_LABELS[r.outcome] : r.suspendedAt ? 'Sospesa' : 'Aperta'}` }))} />
        </div>
        {requests.isError && <ErrorPanel message="Richieste non disponibili. Riapri il modulo per riprovare." />}
        {error && <ErrorPanel message={error} />}
        <div className={form.actions}>
          <Button variant="ghost" onClick={onClose}>Annulla</Button>
          <Button type="submit" loading={add.isPending} disabled={!requestIds.length}>Collega richieste</Button>
        </div>
      </form>
    </Modal>
  );
}
