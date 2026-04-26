import type { ProviderReference } from '../api/types';
import { referenceTypeLabel } from '../lib/provider-refs';

export function RecipientsList({ recipients }: { recipients?: ProviderReference[] }) {
  if (!recipients?.length) {
    return <p className="muted">Se non viene selezionato alcun contatto, verra utilizzato il referente di qualifica.</p>;
  }
  return (
    <div className="stack">
      {recipients.map((recipient) => (
        <div key={recipient.id ?? recipient.email} className="readonly field fullWidth">
          <strong>{recipient.email ?? '-'}</strong>
          <small>
            {[recipient.first_name, recipient.last_name].filter(Boolean).join(' ') || '-'} · {recipient.phone || '-'} ·{' '}
            {referenceTypeLabel(recipient.reference_type)}
          </small>
        </div>
      ))}
    </div>
  );
}
