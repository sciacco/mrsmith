export interface DocumentTypeOption {
  tipoDoc: string;
  label: string;
}

export interface AenadDocument {
  IDDoc: number;
  TipoDoc: string | null;
  IDAnagr: number | null;
  Anagr_Nome: string | null;
  CodDest_IDAnagr: number | null;
  CodDest: string | null;
  Data: string | null;
  Num: number | null;
  DataDoc: string | null;
  NumDoc: string | null;
  DescDoc: string | null;
  TotNetto: number | null;
  TotDoc: number | null;
  TotPrezzoAcquisto: number | null;
  TotGuadagno: number | null;
  Pagamento: string | null;
  Pagam_CoordBancarie: string | null;
  NoteInterne: string | null;
  Anagr_Indirizzo: string | null;
  Anagr_Cap: string | null;
  Anagr_Citta: string | null;
  Anagr_Prov: string | null;
  Anagr_Nazione: string | null;
  Anagr_CodiceFiscale: string | null;
  Anagr_PartitaIva: string | null;
  Anagr_DestNome: string | null;
  Anagr_DestIndirizzo: string | null;
  Anagr_DestCap: string | null;
  Anagr_DestCitta: string | null;
  Anagr_DestProv: string | null;
  Anagr_DestNazione: string | null;
}

export interface AenadDocumentsPage {
  items: AenadDocument[];
  total: number;
  page: number;
  pageSize: number;
}

export interface ArchiveDocumentFilters {
  tipoDoc: string;
  dateFrom: string;
  dateTo: string;
  page: number;
  pageSize: number;
}
