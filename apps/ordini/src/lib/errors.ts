import { ApiError } from '@mrsmith/api-client';

const messages: Record<string, string> = {
  invalid_order_id: 'Ordine non valido.',
  invalid_row_id: 'Riga non valida.',
  invalid_request_body: 'Dati non validi.',
  order_not_found: 'Ordine non trovato.',
  row_not_found: 'Riga non trovata.',
  customer_not_found: 'Ragione sociale non trovata.',
  forbidden: 'Operazione non consentita.',
  role_insufficient: 'Operazione non consentita.',
  wrong_state: 'Operazione non disponibile per lo stato attuale.',
  precondition_missing: 'Completa i dati richiesti prima di procedere.',
  missing_confirmation_date: 'Inserisci la data conferma prima di procedere.',
  invalid_confirmation_date: 'La data conferma non è valida.',
  invalid_activation_date: 'La data attivazione non è valida.',
  missing_customer: 'Seleziona la ragione sociale prima di procedere.',
  missing_pdf: 'Seleziona il PDF firmato prima di procedere.',
  invalid_pdf: 'Il file selezionato non è un PDF valido.',
  gateway_not_configured: 'Non è possibile completare l\'operazione in questo momento.',
  gateway_error: 'Non è possibile completare l\'operazione in questo momento.',
  arxivar_upload_failed: 'Ordine inviato, ma caricamento documento non completato.',
  db_failed: 'Operazione non riuscita.',
  db_commit_failed: 'Salvataggio non completato.',
  gw_pdf_malformed: 'Documento non disponibile in questo momento.',
  alyante_database_not_configured: 'Elenco clienti non disponibile in questo momento.',
  vodka_database_not_configured: 'Ordini non disponibili in questo momento.',
};

export function apiErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof ApiError) {
    const body = error.body;
    if (body && typeof body === 'object' && 'error' in body && typeof body.error === 'string') {
      return messages[body.error] ?? fallback;
    }
    if (error.status === 403) return 'Operazione non consentita.';
    if (error.status === 401) return 'Accesso richiesto.';
  }
  return fallback;
}
