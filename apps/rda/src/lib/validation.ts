import type { CreatePOPayload, RowPayload } from '../api/types';
import { RDA_CURRENCIES } from './format';

export interface ValidationResult {
  fieldErrors: Record<string, string>;
  formErrors: string[];
}

export function ok(): ValidationResult {
  return { fieldErrors: {}, formErrors: [] };
}

export function validateNewPO(body: CreatePOPayload): ValidationResult {
  const result = ok();
  if (!body.budget_id) result.fieldErrors.budget_id = 'Seleziona un budget';
  if (!body.provider_id) result.fieldErrors.provider_id = 'Seleziona un fornitore';
  if (!body.payment_method) result.fieldErrors.payment_method = 'Seleziona un metodo di pagamento';
  if (!RDA_CURRENCIES.includes(body.currency)) result.fieldErrors.currency = 'Seleziona una valuta valida';
  if (!body.project.trim()) result.fieldErrors.project = 'Inserisci il progetto';
  if (body.project.trim().length > 50) result.fieldErrors.project = 'Massimo 50 caratteri';
  if (!body.object.trim()) result.fieldErrors.object = "Inserisci l'oggetto";
  if (body.budget_id && Boolean(body.cost_center) === Boolean(body.budget_user_id)) {
    result.formErrors.push('Il budget deve indicare un solo centro di costo o utente.');
  }
  return result;
}

export function validateRow(body: RowPayload): ValidationResult {
  const result = ok();
  if (!body.product_code) result.fieldErrors.product_code = 'Seleziona un articolo';
  if (!body.description.trim()) result.fieldErrors.description = 'Inserisci la descrizione';
  if (body.qty <= 0) result.fieldErrors.qty = 'Inserisci una quantita maggiore di zero';
  if (body.type === 'good' && (body.price ?? 0) <= 0) result.fieldErrors.price = 'Inserisci un costo unitario';
  if (body.type === 'service' && (body.monthly_fee ?? body.montly_fee ?? 0) <= 0 && (body.activation_price ?? 0) <= 0) {
    result.formErrors.push('Inserisci almeno MRC o NRC.');
  }
  if (body.payment_detail.start_at === 'specific_date' && !body.payment_detail.start_at_date) {
    result.fieldErrors.start_at_date = 'Inserisci la data di decorrenza';
  }
  if (body.type === 'service' && !body.renew_detail?.initial_subscription_months) {
    result.fieldErrors.initial_subscription_months = 'Inserisci la durata iniziale';
  }
  if (body.renew_detail?.automatic_renew && !body.renew_detail.cancellation_advice?.trim()) {
    result.fieldErrors.cancellation_advice = 'Inserisci il preavviso';
  }
  return result;
}

export function firstError(result: ValidationResult): string | null {
  return result.formErrors[0] ?? Object.values(result.fieldErrors)[0] ?? null;
}
