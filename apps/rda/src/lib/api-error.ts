import { ApiError } from '@mrsmith/api-client';

export function apiErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof ApiError) {
    const body = error.body;
    if (body && typeof body === 'object' && 'error' in body && typeof body.error === 'string') return body.error;
    return error.message;
  }
  return fallback;
}

export type ProviderEmailErrorSection = 'to' | 'cc' | 'documents' | 'subject' | 'introduction' | 'conclusion' | 'global';

export interface ProviderEmailMappedError {
  code: string;
  title: string;
  message: string;
  section: ProviderEmailErrorSection;
  closedAfterFailure: boolean;
  closeModal: boolean;
  refreshDetail: boolean;
}

interface ProviderEmailErrorBody {
  code?: unknown;
  error?: unknown;
  field?: unknown;
}

export function mapProviderEmailError(error: unknown): ProviderEmailMappedError {
  const body = error instanceof ApiError && error.body && typeof error.body === 'object'
    ? error.body as ProviderEmailErrorBody
    : {};
  const code = typeof body.code === 'string' ? body.code : '';
  const serverMessage = typeof body.error === 'string' ? body.error : '';
  const field = typeof body.field === 'string' ? body.field : '';
  const section: ProviderEmailErrorSection = field === 'to_contact_ids'
    ? 'to'
    : field === 'cc_contact_ids'
      ? 'cc'
      : field === 'document_ids'
        ? 'documents'
        : field === 'subject'
          ? 'subject'
          : field === 'introduction'
            ? 'introduction'
            : field === 'conclusion'
              ? 'conclusion'
            : 'global';

  if (code === 'PO_CLOSED_EMAIL_FAILED') {
    return {
      code,
      title: "L'email non è stata inviata",
      message: 'Il PO è stato chiuso correttamente. Controlla i dati e riprova.',
      section: 'global',
      closedAfterFailure: true,
      closeModal: false,
      refreshDetail: true,
    };
  }
  if (code === 'EMAIL_FAILED') {
    return {
      code,
      title: "L'email non è stata inviata",
      message: 'Controlla i dati e riprova.',
      section: 'global',
      closedAfterFailure: false,
      closeModal: false,
      refreshDetail: false,
    };
  }
  if (code === 'ATTACHMENTS_TOO_LARGE') {
    return {
      code,
      title: 'Dimensione allegati non valida',
      message: 'La dimensione complessiva supera 25 MB. Riduci i documenti selezionati e riprova.',
      section: 'documents',
      closedAfterFailure: false,
      closeModal: false,
      refreshDetail: false,
    };
  }
  if (code === 'PO_STATE_CHANGED') {
    return {
      code,
      title: 'Il PO è stato aggiornato',
      message: 'Lo stato del PO è cambiato. Aggiorna il dettaglio prima di continuare.',
      section: 'global',
      closedAfterFailure: false,
      closeModal: false,
      refreshDetail: true,
    };
  }
  if (code === 'PO_NOT_FOUND' || (error instanceof ApiError && error.status === 404)) {
    return {
      code: code || 'PO_NOT_FOUND',
      title: 'PO non disponibile',
      message: 'Il PO non è più disponibile. Il dettaglio verrà aggiornato.',
      section: 'global',
      closedAfterFailure: false,
      closeModal: true,
      refreshDetail: true,
    };
  }
  if (code === 'PO_SEND_FORBIDDEN' || (error instanceof ApiError && error.status === 403)) {
    return {
      code: code || 'PO_SEND_FORBIDDEN',
      title: 'Invio non consentito',
      message: 'Non hai i permessi per inviare questo PO.',
      section: 'global',
      closedAfterFailure: false,
      closeModal: false,
      refreshDetail: false,
    };
  }
  if (code === 'CONTACT_INVALID') {
    return {
      code,
      title: 'Contatto non disponibile',
      message: serverMessage || 'Il contatto selezionato non è più disponibile. Modifica i destinatari e riprova.',
      section: section === 'global' ? 'to' : section,
      closedAfterFailure: false,
      closeModal: false,
      refreshDetail: false,
    };
  }
  if (code === 'DOCUMENT_INVALID') {
    return {
      code,
      title: 'Documento non disponibile',
      message: serverMessage || 'Un documento selezionato non è più disponibile. Modifica gli allegati e riprova.',
      section: 'documents',
      closedAfterFailure: false,
      closeModal: false,
      refreshDetail: false,
    };
  }

  return {
    code: code || 'UNKNOWN',
    title: 'Invio non riuscito',
    message: serverMessage || 'Controlla i dati e riprova.',
    section,
    closedAfterFailure: false,
    closeModal: false,
    refreshDetail: false,
  };
}

export function conformityConfirmationErrorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    const body = error.body;
    if (body && typeof body === 'object' && 'error' in body && typeof body.error === 'string') {
      const upstreamMessage = body.error.toLowerCase();
      if (upstreamMessage.includes('ddt') || upstreamMessage.includes('delivery note')) {
        return 'Impossibile confermare la conformità. Carica il DDT per gli articoli di tipo bene e riprova.';
      }
    }
  }
  return apiErrorMessage(error, 'Conferma della conformità non riuscita');
}
