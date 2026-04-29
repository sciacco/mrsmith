import { Icon, useToast } from '@mrsmith/ui';
import { useEffect, useState, type ChangeEvent } from 'react';
import { getRdaQuoteThreshold } from '../runtime-config';
import { useDeleteAttachment, useRdaDownloads, useUploadAttachment } from '../api/queries';
import type { AttachmentType, PoAttachment, PoDetail } from '../api/types';
import {
  ATTACHMENT_TYPE_OPTIONS,
  attachmentTypeLabel,
  defaultAttachmentTypeForPOState,
} from '../lib/attachments';
import { downloadBlob, formatDateIT, formatMoney } from '../lib/format';
import { ConfirmDialog } from './ConfirmDialog';

export function AttachmentsTab({ po, editable }: { po: PoDetail; editable: boolean }) {
  const [deleteTarget, setDeleteTarget] = useState<PoAttachment | null>(null);
  const [attachmentType, setAttachmentType] = useState<AttachmentType>(() => defaultAttachmentTypeForPOState(po.state));
  const upload = useUploadAttachment();
  const remove = useDeleteAttachment();
  const downloads = useRdaDownloads();
  const { toast } = useToast();
  const poId = po.id;
  const canUpload = po.state === 'DRAFT' || po.state === 'PENDING_VERIFICATION';
  const quoteThreshold = getRdaQuoteThreshold();

  useEffect(() => {
    setAttachmentType(defaultAttachmentTypeForPOState(po.state));
  }, [po.state]);

  async function uploadFiles(files: FileList | null) {
    if (!files) return;
    const selectedType = attachmentType;
    try {
      for (const file of Array.from(files)) {
        await upload.mutateAsync({ id: poId, file, attachmentType: selectedType });
      }
      toast('Allegati caricati');
    } catch {
      toast('Caricamento non riuscito', 'error');
    }
  }

  function handleUpload(event: ChangeEvent<HTMLInputElement>) {
    const input = event.currentTarget;
    void uploadFiles(input.files).finally(() => {
      input.value = '';
    });
  }

  async function download(attachment: PoAttachment) {
    try {
      const blob = await downloads.attachment(poId, attachment.id);
      downloadBlob(blob, attachment.file_name || `allegato-${attachment.id}`);
    } catch {
      toast('Download non riuscito', 'error');
    }
  }

  async function confirmDelete() {
    if (!deleteTarget) return;
    try {
      await remove.mutateAsync({ id: poId, attachmentId: deleteTarget.id });
      toast('Allegato eliminato');
      setDeleteTarget(null);
    } catch {
      toast('Eliminazione non riuscita', 'error');
    }
  }

  return (
    <div className="stack">
      <p className="muted">Per importi maggiori di {formatMoney(quoteThreshold, po.currency)} sono necessari almeno 2 allegati di tipo Preventivo.</p>
      {canUpload ? (
        <div className="attachmentUploadControls compact">
          <div className="field">
            <label htmlFor="po-attachment-type">Tipo documento</label>
            <select
              id="po-attachment-type"
              value={attachmentType}
              disabled={upload.isPending}
              onChange={(event) => setAttachmentType(event.target.value as AttachmentType)}
            >
              {ATTACHMENT_TYPE_OPTIONS.map((option) => (
                <option key={option.value} value={option.value}>{option.label}</option>
              ))}
            </select>
          </div>
          <div className="field">
            <label>Carica allegati</label>
            <input type="file" multiple onChange={handleUpload} disabled={upload.isPending} />
          </div>
        </div>
      ) : null}
      <div className="tableScroll">
        <table className="dataTable">
          <thead>
            <tr><th>File name</th><th>Tipo</th><th>Data</th><th className="actionsCell">Azioni</th></tr>
          </thead>
          <tbody>
            {(po.attachments ?? []).map((attachment) => (
              <tr key={attachment.id}>
                <td>{attachment.file_name ?? attachment.file_id ?? '-'}</td>
                <td>{attachmentTypeLabel(attachment.attachment_type)}</td>
                <td>{formatDateIT(attachment.created_at ?? attachment.created)}</td>
                <td className="actionsCell">
                  <span className="iconActions">
                    <button className="iconButton" type="button" aria-label="Scarica allegato" title="Scarica" onClick={() => void download(attachment)}>
                      <Icon name="download" size={16} />
                    </button>
                    <button
                      className="iconButton dangerButton"
                      type="button"
                      aria-label="Elimina allegato"
                      title="Elimina"
                      disabled={!editable}
                      onClick={() => setDeleteTarget(attachment)}
                    >
                      <Icon name="trash" size={16} />
                    </button>
                  </span>
                </td>
              </tr>
            ))}
            {(po.attachments ?? []).length === 0 ? <tr><td colSpan={4} className="emptyInline">Nessun allegato presente.</td></tr> : null}
          </tbody>
        </table>
      </div>
      <ConfirmDialog
        open={deleteTarget != null}
        title="Elimina allegato"
        message="Confermi eliminazione dell'allegato selezionato?"
        confirmLabel="Elimina"
        danger
        loading={remove.isPending}
        onClose={() => setDeleteTarget(null)}
        onConfirm={() => void confirmDelete()}
      />
    </div>
  );
}
