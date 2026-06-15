export interface OpenAPIITEnvelope<T> {
  data: T;
  success: boolean;
  message: string;
  error: number | null;
}

export interface Province {
  sigla: string;
  provincia: string;
  superficie: number;
  residenti: number;
  num_comuni: number;
  istat: string;
  regione: string;
}
