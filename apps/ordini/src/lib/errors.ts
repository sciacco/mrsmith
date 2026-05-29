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
  not_converted_from_quote: 'Questo ordine non è collegato a una proposta.',
  order_has_erp_rows: 'L\'ordine risulta già presente in ERP e non può essere rimosso.',
  order_has_signed_pdf: 'L\'ordine ha già un documento firmato e non può essere rimosso.',
  bridge_delete_failed: 'Conversione annullata, ma il collegamento con la proposta richiede una verifica.',
  hubspot_cleanup_failed: 'Conversione annullata, ma PDF o nota HubSpot richiedono una verifica.',
  hubspot_not_configured: 'Conversione annullata, ma HubSpot non era disponibile per rimuovere PDF e nota.',
  db_failed: 'Operazione non riuscita.',
  db_commit_failed: 'Salvataggio non completato.',
  gw_pdf_malformed: 'Documento non disponibile in questo momento.',
  alyante_database_not_configured: 'Dati ERP non disponibili in questo momento.',
  mistra_database_not_configured: 'Collegamento proposta non disponibile in questo momento.',
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
