export interface MorAnomaly {
  conto: string;
  lastname: string | null;
  firstname: string | null;
  is_da_fatturare: string | null;
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

export interface TimooDailyStat {
  tenant_id: number;
  tenant_name: string;
  day: string;
  users: number;
  service_extensions: number;
}

export interface PendingActivation {
  ragione_sociale: string;
  numero_ordine: string;
  data_documento: string | null;
  durata_servizio: string | null;
  durata_rinnovo: string | null;
  sost_ord: string | null;
  sostituito_da: string | null;
  storico: string | null;
  numero_azienda: string;
}

export interface ActivationRow {
  descrizione_long: string | null;
  quantita: number | null;
  nrc: number | null;
  mrc: number | null;
  totale_mrc: number | null;
  stato_riga: string | null;
  serialnumber: string | null;
  note_legali: string | null;
}

export interface RenewalSummary {
  ragione_sociale: string;
  rinnovi_dal: string | null;
  rinnovi_al: string | null;
  numero_ordini: number;
  servizi_attivi: number;
  ordini_servizi: string;
  senza_tacito_rinnovo: boolean;
  canoni: number | null;
  numero_azienda: string;
}

export interface RenewalRow {
  nome_testata_ordine: string;
  stato_ordine: string | null;
  descrizione_long: string | null;
  quantita: number | null;
  nrc: number | null;
  mrc: number | null;
  stato_riga: string | null;
  serialnumber: string | null;
  note_legali: string | null;
  data_attivazione: string | null;
  durata_servizio: string | null;
  durata_rinnovo: string | null;
  durata: string | null;
  prossimo_rinnovo: string | null;
  sost_ord: string | null;
  sostituito_da: string | null;
  tacito_rinnovo: number | null;
}

export interface AovByTypeRow {
  anno: string | null;
  mese: string | null;
  tipo_ordine: string | null;
  numero_ordini: number;
  valore_aov: number | null;
  totale_mrc: number | null;
  totale_nrc: number | null;
}

export interface AovByCategoryRow {
  anno: string | null;
  mese: string | null;
  categoria: string | null;
  numero_ordini: number;
  valore_aov: number | null;
  totale_mrc: number | null;
  totale_nrc: number | null;
}

export interface AovBySalesRow {
  anno: string | null;
  commerciale: string | null;
  tipo_ordine: string | null;
  numero_ordini: number;
  valore_aov: number | null;
  totale_mrc: number | null;
  totale_nrc: number | null;
}

export interface AovDetailRow {
  tipo_documento: string | null;
  anno: string | null;
  mese: string | null;
  nome_testata_ordine: string | null;
  tipo_ordine: string | null;
  sost_ord: string | null;
  commerciale: string | null;
  totale_mrc: number | null;
  totale_nrc: number | null;
  totale_mrc_odv_sost: number | null;
  totale_mrc_new: number | null;
  valore_aov: number | null;
}

export interface AovPreviewResponse {
  byType: AovByTypeRow[];
  byCategory: AovByCategoryRow[];
  bySales: AovBySalesRow[];
  detail: AovDetailRow[];
}

export interface OrderRow {
  ragione_sociale: string;
  stato_ordine: string;
  numero_ordine: string;
  descrizione_long: string | null;
  quantita: number | null;
  nrc: number | null;
  mrc: number | null;
  totale_mrc: number | null;
  numero_azienda: string;
  data_documento: string | null;
  stato_riga: string | null;
  data_ultima_fatt: string | null;
  serialnumber: string | null;
  metodo_pagamento: string | null;
  durata_servizio: string | null;
  durata_rinnovo: string | null;
  data_cessazione: string | null;
  data_attivazione: string | null;
  note_legali: string | null;
  sost_ord: string | null;
  sostituito_da: string | null;
  progressivo_riga: number | null;
}

export interface ActiveLineRow {
  ragione_sociale: string;
  tipo_conn: string | null;
  fornitore: string | null;
  provincia: string | null;
  comune: string | null;
  tipo: string | null;
  profilo_commerciale: string | null;
  macro: string | null;
  intestatario: string | null;
  ordine: string | null;
  fatturato_fino_al: string | null;
  stato_riga: string | null;
  stato_ordine: string | null;
  stato: string | null;
  id: number;
  codice_ordine: string | null;
  serialnumber: string | null;
  id_anagrafica: string | null;
  quantita: number | null;
  canone: number | null;
}
