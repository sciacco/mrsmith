import { Tooltip } from '@mrsmith/ui';

// Marker 🔒 "hidden in prod" — la mappa diretta della curation Fase 1.
// Convenzione PRD §3: puntino discreto (~7px grigio) + tooltip in hover.
// Visibile a chiunque con ruolo Binocolo: non è un flag dev-only.
//
// Usiamo il Tooltip del design system (non l'attributo `title` nativo) perché
// il tooltip nativo del browser ha un ritardo di ~1–2s e su elementi piccoli
// spesso non si attiva — il Tooltip renderizza un popover affidabile.

export function HiddenField({ label }: { label: string }) {
  return (
    <Tooltip content={`${label} — oggi non visibile all'analista`} placement="top" showDelay={150}>
      <span
        className="ti-hide"
        role="img"
        aria-label={`${label} — oggi non visibile all'analista`}
      />
    </Tooltip>
  );
}
