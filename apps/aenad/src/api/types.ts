export interface DocumentTypeOption {
  tipoDoc: string;
  label: string;
}

export interface AenadDocument {
  IDDoc: number;
  TipoDoc: string | null;
  IDAnagr: number | null;
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
