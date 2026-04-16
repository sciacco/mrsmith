export interface PagedResponse<T> {
  items: T[];
  page: number;
  page_size: number;
  total: number;
}

export interface Deal {
  id: number;
  codice: string;
  deal_name: string;
  company_name: string | null;
  owner_email: string | null;
  pipeline_label: string | null;
  stage_label: string | null;
  stage_order: number | null;
}

export interface LookupItem {
  id: number;
  nome: string;
}

export interface FattibilitaCounts {
  bozza: number;
  inviata: number;
  sollecitata: number;
  completata: number;
  annullata: number;
  totale: number;
}

export interface RichiestaBase {
  id: number;
  deal_id: number | null;
  data_richiesta: string;
  descrizione: string;
  indirizzo: string;
  stato: string;
  annotazioni_richiedente: string | null;
  annotazioni_carrier: string | null;
  created_by: string | null;
  created_at: string;
  updated_at: string | null;
  fornitori_preferiti: number[];
  codice_deal: string;
  preferred_supplier_names?: string[];
}

export interface RichiestaSummary extends RichiestaBase {
  deal_name: string | null;
  company_name: string | null;
  owner_email: string | null;
  counts: FattibilitaCounts;
}

export interface Fattibilita {
  id: number;
  richiesta_id: number;
  fornitore_id: number;
  fornitore_nome: string;
  data_richiesta: string;
  tecnologia_id: number;
  tecnologia_nome: string;
  descrizione: string | null;
  contatto_fornitore: string | null;
  riferimento_fornitore: string | null;
  stato: string;
  annotazioni: string | null;
  esito_ricevuto_il: string | null;
  da_ordinare: boolean;
  profilo_fornitore: string | null;
  nrc: number | null;
  mrc: number | null;
  durata_mesi: number | null;
  aderenza_budget: number;
  copertura: boolean;
  giorni_rilascio: number | null;
}

export interface RichiestaFull extends RichiestaBase {
  deal: Deal | null;
  deal_name: string | null;
  company_name: string | null;
  owner_email: string | null;
  fattibilita: Fattibilita[];
  counts: FattibilitaCounts;
}

export interface AnalysisTextResponse {
  analysis: string;
}

export interface AnalysisAction {
  azione: string;
  fornitore: string;
  tecnologia?: string;
  motivo: string;
}

export interface AnalysisValutazione {
  fornitore: string;
  tecnologia: string;
  stato: string;
  copertura?: string;
  aderenza_budget?: string;
  durata_mesi?: number;
  giorni_rilascio?: number | null;
  preferito?: boolean;
  criticita?: string;
}

export interface AnalysisJSON {
  azioni_raccomandate: AnalysisAction[];
  valutazioni: AnalysisValutazione[];
}

export interface CreateRichiestaBody {
  deal_id: number;
  indirizzo: string;
  descrizione: string;
  fornitori_preferiti: number[];
}

export interface CreateFattibilitaBody {
  items: Array<{
    fornitore_id: number;
    tecnologia_id: number;
  }>;
}

export interface UpdateRichiestaStatoBody {
  stato: string;
}

export interface UpdateFattibilitaBody {
  descrizione?: string;
  contatto_fornitore?: string;
  riferimento_fornitore?: string;
  stato?: string;
  annotazioni?: string;
  esito_ricevuto_il?: string;
  da_ordinare?: boolean;
  profilo_fornitore?: string;
  nrc?: number;
  mrc?: number;
  durata_mesi?: number;
  aderenza_budget?: number;
  copertura?: boolean;
  giorni_rilascio?: number;
}
