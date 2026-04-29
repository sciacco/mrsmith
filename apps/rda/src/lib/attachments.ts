import type { AttachmentType, PoAttachment } from '../api/types';

export const ATTACHMENT_TYPE_OPTIONS: { value: AttachmentType; label: string }[] = [
  { value: 'quote', label: 'Preventivo' },
  { value: 'transport_document', label: 'Documento di trasporto' },
  { value: 'other', label: 'Altro' },
];

export function isAttachmentType(value: unknown): value is AttachmentType {
  return value === 'quote' || value === 'transport_document' || value === 'other';
}

export function defaultAttachmentTypeForPOState(state?: string | null): AttachmentType {
  return state === 'PENDING_VERIFICATION' ? 'transport_document' : 'quote';
}

export function attachmentTypeLabel(value?: string | null): string {
  if (value === 'quote') return 'Preventivo';
  if (value === 'transport_document') return 'Documento di trasporto';
  return 'Altro';
}

export function countQuoteAttachments(attachments?: Pick<PoAttachment, 'attachment_type'>[] | null): number {
  return (attachments ?? []).filter((attachment) => attachment.attachment_type === 'quote').length;
}
