import { Icon, Modal, useToast } from '@mrsmith/ui';
import { useEffect, useRef, useState, type ChangeEvent, type DragEvent } from 'react';
import { getRdaQuoteThreshold } from '../runtime-config';
import { useDeleteAttachment, useRdaDownloads, useUploadAttachment } from '../api/queries';
import type { AttachmentType, PoAttachment, PoDetail } from '../api/types';
import {
  ATTACHMENT_TYPE_OPTIONS,
  attachmentTypeLabel,
  defaultAttachmentTypeForPOState,
} from '../lib/attachments';
import { downloadBlob, formatDateIT, formatMoney, parseMistraMoney } from '../lib/format';
import { ConfirmDialog } from './ConfirmDialog';

export function AttachmentsTab({ po, editable }: { po: PoDetail; editable: boolean }) {
  const [deleteTarget, setDeleteTarget] = useState<PoAttachment | null>(null);
  const [attachmentType, setAttachmentType] = useState<AttachmentType>(() => defaultAttachmentTypeForPOState(po.state));
  const [preview, setPreview] = useState<{ attachment: PoAttachment; url: string } | null>(null);
  const [previewLoadingId, setPreviewLoadingId] = useState<number | null>(null);
  const uploadFormRef = useRef<HTMLDivElement>(null);
  const attachmentTypeSelectRef = useRef<HTMLSelectElement>(null);
  const previewUrlRef = useRef<string | null>(null);
  const upload = useUploadAttachment();
  const remove = useDeleteAttachment();
  const downloads = useRdaDownloads();
  const { toast } = useToast();
  const poId = po.id;
  const canUpload = po.state === 'DRAFT' || po.state === 'PENDING_VERIFICATION';
  const quoteThreshold = getRdaQuoteThreshold();
  const attachments = po.attachments ?? [];
  const quoteCount = attachments.filter((attachment) => attachment.attachment_type === 'quote').length;
  const quoteRequired = parseMistraMoney(po.total_price) >= quoteThreshold;

  useEffect(() => {
    setAttachmentType(defaultAttachmentTypeForPOState(po.state));
  }, [po.state]);

  useEffect(() => () => releasePreviewUrl(), []);

  function releasePreviewUrl() {
    if (!previewUrlRef.current) return;
    URL.revokeObjectURL(previewUrlRef.current);
    previewUrlRef.current = null;
  }

  function focusUploadForm() {
    uploadFormRef.current?.scrollIntoView({ behavior: 'smooth', block: 'center' });
    attachmentTypeSelectRef.current?.focus({ preventScroll: true });
  }

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

  function handleDrop(event: DragEvent<HTMLLabelElement>) {
    event.preventDefault();
    if (upload.isPending) return;
    void uploadFiles(event.dataTransfer.files);
  }

  async function download(attachment: PoAttachment) {
    try {
      const blob = await downloads.attachment(poId, attachment.id);
      downloadBlob(blob, attachment.file_name || `allegato-${attachment.id}`);
    } catch {
      toast('Download non riuscito', 'error');
    }
  }

  async function previewAttachment(attachment: PoAttachment) {
    setPreviewLoadingId(attachment.id);
    try {
      const blob = await downloads.attachment(poId, attachment.id);
      const url = URL.createObjectURL(blob);
      releasePreviewUrl();
      previewUrlRef.current = url;
      setPreview({ attachment, url });
    } catch {
      toast('Anteprima non riuscita', 'error');
    } finally {
      setPreviewLoadingId(null);
    }
  }

  function closePreview() {
    releasePreviewUrl();
    setPreview(null);
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
      <div className="attachmentBusinessMessage">
        <span className={`badge ${quoteRequired && quoteCount < 2 ? 'warning' : 'success'}`}>
          {quoteRequired ? `${quoteCount}/2 preventivi` : `${attachments.length} allegat${attachments.length === 1 ? 'o' : 'i'}`}
        </span>
        <p className="muted">Per importi maggiori di {formatMoney(quoteThreshold, po.currency)} sono necessari almeno 2 preventivi.</p>
        {canUpload ? (
          <button className="iconButton attachmentBusinessAction" type="button" aria-label="Aggiungi allegato" title="Aggiungi allegato" onClick={focusUploadForm}>
            <Icon name="file-plus" size={17} />
          </button>
        ) : null}
      </div>
      {attachments.length > 0 ? (
        <div className="tableScroll">
          <table className="dataTable">
            <thead>
              <tr><th>File name</th><th>Tipo</th><th>Data</th><th className="actionsCell">Azioni</th></tr>
            </thead>
            <tbody>
              {attachments.map((attachment) => (
                <tr key={attachment.id}>
                  <td>{attachment.file_name ?? attachment.file_id ?? '-'}</td>
                  <td>{attachmentTypeLabel(attachment.attachment_type)}</td>
                  <td>{formatDateIT(attachment.created_at ?? attachment.created)}</td>
                  <td className="actionsCell">
                    <span className="iconActions">
                      <button
                        className={`iconButton ${previewLoadingId === attachment.id ? 'loading' : ''}`}
                        type="button"
                        aria-label="Visualizza allegato"
                        title="Visualizza"
                        disabled={previewLoadingId !== null}
                        onClick={() => void previewAttachment(attachment)}
                      >
                        <Icon name={previewLoadingId === attachment.id ? 'loader' : 'eye'} size={16} />
                      </button>
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
            </tbody>
          </table>
        </div>
      ) : null}
      {canUpload ? (
        <div ref={uploadFormRef} className="attachmentUploadControls detailDropzone">
          <div className="field">
            <label htmlFor="po-attachment-type">Nuovo tipo di documento</label>
            <select
              ref={attachmentTypeSelectRef}
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
          <label
            className={`uploadDrop ${upload.isPending ? 'disabled' : ''}`}
            onDragOver={(event) => event.preventDefault()}
            onDrop={handleDrop}
          >
            <Icon name={upload.isPending ? 'loader' : 'file-up'} size={22} />
            <span>{upload.isPending ? 'Caricamento in corso' : 'Trascina o scegli file'}</span>
            <small>{attachmentTypeLabel(attachmentType)}</small>
            <input type="file" multiple onChange={handleUpload} disabled={upload.isPending} />
          </label>
        </div>
      ) : null}
      <Modal open={preview != null} onClose={closePreview} title={preview?.attachment.file_name ?? 'Visualizza allegato'} size="xwide">
        <div className="attachmentPreviewShell">
          {preview ? (
            <iframe
              className="attachmentPreviewFrame"
              src={preview.url}
              title={preview.attachment.file_name ?? `Allegato ${preview.attachment.id}`}
            />
          ) : null}
        </div>
      </Modal>
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
