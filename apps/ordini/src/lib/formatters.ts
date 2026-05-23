import type { OrderState } from '../api/types';

export function isBlank(value: unknown): boolean {
  return value == null || (typeof value === 'string' && value.trim() === '');
}

export function formatEmpty(value: string | number | null | undefined): string {
  if (value == null) return '—';
  const text = String(value).trim();
  return text === '' ? '—' : text;
}

export function formatDate(value: string | null | undefined): string {
  if (!value) return '—';
  const [datePart] = value.split('T');
  if (!datePart) return '—';
  const date = new Date(`${datePart}T00:00:00`);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat('it-IT', { day: '2-digit', month: '2-digit', year: 'numeric' }).format(date);
}

export function dateInputValue(value: string | null | undefined): string {
  if (!value) return '';
  return value.split('T')[0] ?? '';
}

export function formatMoney(value: number | null | undefined, currency = 'EUR'): string {
  if (value == null || Number.isNaN(value)) return '—';
  const normalizedCurrency = currency === 'EURO' || currency === '' ? 'EUR' : currency;
  try {
    return new Intl.NumberFormat('it-IT', { style: 'currency', currency: normalizedCurrency }).format(value);
  } catch {
    return new Intl.NumberFormat('it-IT', { minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(value);
  }
}

export function formatSiNo(value: number | string | null | undefined): string {
  if (value == null || value === '') return '—';
  return Number(value) !== 0 ? 'Sì' : 'No';
}

export function formatStato(value: OrderState | null | undefined): string {
  if (!value) return '—';
  return String(value).toUpperCase();
}

export function formatTipoProposta(value: string | null | undefined): string {
  switch ((value ?? '').toUpperCase()) {
    case 'A':
      return 'Sostituzione';
    case 'N':
      return 'Nuovo';
    case 'R':
      return 'Rinnovo';
    default:
      return formatEmpty(value);
  }
}

export function formatTipoDoc(value: string | null | undefined): string {
  switch ((value ?? '').toUpperCase()) {
    case 'TSC-ORDINE-RIC':
      return 'Ordine ricorrente';
    case 'TSC-ORDINE':
      return 'Ordine spot';
    default:
      return formatEmpty(value);
  }
}

export function formatFatturazione(value: string | null | undefined): string {
  switch (String(value ?? '').trim()) {
    case '1':
      return 'Mensile';
    case '2':
      return 'Bimestrale';
    case '3':
      return 'Trimestrale';
    case '4':
    case '5':
      return 'Quadrimestrale';
    case '6':
      return 'Semestrale';
    case '12':
      return 'Annuale';
    default:
      return formatEmpty(value);
  }
}

export function formatFatturazioneAtt(value: string | null | undefined): string {
  switch (String(value ?? '').trim()) {
    case '1':
      return "All'ordine";
    case '2':
      return "All'attivazione";
    default:
      return formatEmpty(value);
  }
}

export function formatDurRin(value: string | null | undefined): string {
  switch (String(value ?? '').trim()) {
    case '1':
      return 'Mensile';
    case '2':
      return 'Bimestrale';
    case '3':
      return 'Trimestrale';
    case '4':
      return 'Quadrimestrale';
    case '6':
      return 'Semestrale';
    case '12':
      return 'Annuale';
    default:
      return formatEmpty(value);
  }
}

export function formatIsColo(value: string | null | undefined): string {
  if (!value || value === '0') return 'Altre soluzioni';
  return value;
}

export function formatServiceTypes(serviceType: string | null | undefined, isColo: string | null | undefined): string {
  if (isColo && isColo !== '0') return isColo;
  return formatEmpty(serviceType);
}

export function orderCode(ndoc: string | null | undefined, anno: number | null | undefined): string {
  if (!ndoc && !anno) return '—';
  if (!ndoc) return String(anno);
  if (!anno) return ndoc;
  return `${ndoc}/${anno}`;
}
