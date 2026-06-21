import { ApiError } from '@mrsmith/api-client';

/**
 * Map of stable backend error codes → Italian user-facing messages.
 * Codes are emitted by backend handlers (see backend/internal/raenad).
 * Keep codes stable; only edit the human-readable text here.
 */
const codeMessages: Record<string, string> = {
  // Structural / transport
  invalid_payload: 'Dati non validi.',
  invalid_json: 'Richiesta non valida.',

  // Validation field codes (returned as details[] under error=validation_failed)
  required: 'campo obbligatorio',
  invalid_date: 'data non valida',
  invalid_line_type: 'tipo riga non valido',
  invalid_decimal: 'valore numerico non valido',
  invalid_vat_code: 'codice IVA non valido',
  no_economic_line: 'inserisci almeno una riga articolo con quantità, prezzo e IVA',

  // Domain error codes (top-level error)
  payment_method_not_found: 'Metodo di pagamento non trovato.',
  quote_not_found: 'Preventivo non trovato.',
  raenad_config_not_configured: 'Configurazione pipeline non disponibile.',

  // Generic
  internal_server_error: 'Errore del server. Riprova più tardi.',
};

interface ValidationDetail {
  field: string;
  code: string;
}

/** Human-readable label for a field path emitted by the backend. */
const fieldLabels: Record<string, string> = {
  hubspot_company_id: 'Cliente',
  'customer.name': 'Cliente',
  document_date: 'Data documento',
  description: 'Oggetto / Descrizione Breve',
  payment_method_code: 'Metodo di pagamento',
  lines: 'Righe',
};

function labelForField(field: string): string {
  if (fieldLabels[field]) return fieldLabels[field];
  const linesMatch = field.match(/^lines\[(\d+)\]\.(.+)$/);
  if (linesMatch && linesMatch[1] && linesMatch[2]) {
    const idx = parseInt(linesMatch[1], 10) + 1;
    const sub = linesMatch[2];
    const subLabels: Record<string, string> = {
      line_type: 'tipo riga',
      qta: 'quantità',
      unit_price: 'prezzo unitario',
      iva_percent_snapshot: 'aliquota IVA',
      purchase_unit_price: 'prezzo di carico',
      cod_iva: 'codice IVA',
    };
    return `Riga ${idx} · ${subLabels[sub] ?? sub}`;
  }
  return field;
}

/**
 * Returns a user-facing message for an API error.
 *
 * Handles two shapes:
 * - { error: "validation_failed", details: [{field, code}, ...] } → joins per-field messages
 * - { error: "<code>" } → mapped via codeMessages
 */
export function apiErrorMessage(error: unknown, fallback: string): string {
  if (!(error instanceof ApiError)) {
    return fallback;
  }

  const body = error.body as
    | { error?: string; details?: ValidationDetail[]; message?: string }
    | undefined;

  if (body?.error === 'validation_failed' && Array.isArray(body.details) && body.details.length > 0) {
    return body.details
      .map((d) => {
        const msg = codeMessages[d.code] ?? d.code;
        return `${labelForField(d.field)}: ${msg}`;
      })
      .join(' · ');
  }

  if (body?.error && codeMessages[body.error]) {
    return codeMessages[body.error]!;
  }

  if (body?.message && typeof body.message === 'string') {
    return body.message;
  }

  if (error.status === 403) return 'Operazione non consentita.';
  if (error.status === 401) return 'Accesso richiesto.';

  return fallback;
}
