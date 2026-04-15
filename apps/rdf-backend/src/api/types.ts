export interface Supplier {
  id: number;
  nome: string;
}

export interface SupplierListResponse {
  items: Supplier[];
  total: number;
}

export interface SupplierCreateInput {
  nome: string;
}

export interface SupplierUpdateInput {
  nome?: string;
}

export interface ErrorResponse {
  error?: string;
}
