export interface MorAnomaly {
  conto: string;
  lastname: string | null;
  firstname: string | null;
  is_da_fatturare: boolean | string | null;
  codice_ordine: string | null;
  serialnumber: string | null;
  periodo_inizio: string | null;
  importo: number | null;
  stato: string | null;
  tipologia: string | null;
  id_cliente: number | null;
  intestazione: string | null;
  ordine_presente: string;
  numero_ordine_corretto: string;
}

export const morAnomaliesKeys = {
  all: ['reports', 'mor-anomalies'] as const,
};
