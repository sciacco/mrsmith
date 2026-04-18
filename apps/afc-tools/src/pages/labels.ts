// Code → business label maps used on Dettaglio ordini (spec §B.2.7).
// Preserved 1:1 from the Appsmith ternaries, with one deliberate fix
// (decision A.5.1a: code 400 now correctly maps to "SDD FM").

export function tipodocLabel(code: string | null | undefined): string {
  return code === 'TSC-ORDINE-RIC' ? 'Ordine ricorrente' : 'Ordine Spot';
}

export function tipoOrdLabel(code: string | null | undefined): string {
  switch (code) {
    case 'N': return 'Nuovo';
    case 'A': return 'Sostituzione';
    case 'R': return 'Rinnovo';
    default: return '';
  }
}

// NB: Quadrimestrale = 4 on cdlan_dur_rin (here) but 5 on cdlan_int_fatturazione
// (server-side CASE in the Order query). Divergence preserved (decision A.5.1b).
export function durRinLabel(code: number | null | undefined): string {
  switch (code) {
    case 1: return 'Mensile';
    case 2: return 'Bimestrale';
    case 3: return 'Trimestrale';
    case 4: return 'Quadrimestrale';
    case 6: return 'Semestrale';
    case 12: return 'Annuale';
    default: return '';
  }
}

export function tacitoRinLabel(code: number | null | undefined): string {
  return code === 1 ? 'Sì' : 'No';
}

// paymentTermsLabel: 18 payment-term codes from cdlan_cod_termini_pag.
// Deliberate fix (A.5.1a): the Appsmith ternary contained a typo that made
// code 400 unreachable ("Order.data[0].Order == 400"); corrected here.
export function paymentTermsLabel(code: number | null | undefined): string {
  switch (code) {
    case 301: return 'Vista fattura';
    case 303: return 'BB FM';
    case 304: return 'BB Vista fattura';
    case 311: return 'BB 30ggDF';
    case 312: return 'BB 30ggFM';
    case 313: return 'BB 60ggDF';
    case 314: return 'BB 60ggFM';
    case 315: return 'BB 90ggDF';
    case 316: return 'BB 90ggFM';
    case 318: return 'BB 120ggFM';
    case 400: return 'SDD FM'; // fixed (was unreachable in Appsmith)
    case 402: return 'SDD 30ggDF';
    case 403: return 'SDD 30ggFM';
    case 404: return 'SDD 60ggDF';
    case 405: return 'SDD 60ggFM';
    case 406: return 'SDD 90ggDF';
    case 407: return 'SDD 90ggFM';
    case 409: return 'SDD DFFM';
    default: return '';
  }
}
