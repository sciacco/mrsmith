import { useState } from 'react';
import type { PoDetail, ProviderSummary } from '../api/types';
import { AttachmentsTab } from './AttachmentsTab';
import type { HeaderFormState } from './PoHeaderForm';
import { NotesTab } from './NotesTab';
import { ProviderRefTable } from './ProviderRefTable';
import { RowsTab } from './RowsTab';

type TabID = 'attachments' | 'rows' | 'notes' | 'contacts';

const tabs: { id: TabID; label: string }[] = [
  { id: 'attachments', label: 'Allegati' },
  { id: 'rows', label: 'Righe PO' },
  { id: 'notes', label: 'Note' },
  { id: 'contacts', label: 'Contatti Fornitore' },
];

export function PoTabs({
  po,
  provider,
  editable,
  header,
  saving,
  onHeaderChange,
  onSaveHeader,
  onSaveRecipients,
}: {
  po: PoDetail;
  provider?: ProviderSummary;
  editable: boolean;
  header: HeaderFormState;
  saving: boolean;
  onHeaderChange: (value: HeaderFormState) => void;
  onSaveHeader: () => void;
  onSaveRecipients: (ids: number[]) => void;
}) {
  const [active, setActive] = useState<TabID>('attachments');
  return (
    <section className="surface">
      <div className="tabs">
        {tabs.map((tab) => (
          <button key={tab.id} className={`tab ${active === tab.id ? 'active' : ''}`} type="button" onClick={() => setActive(tab.id)}>
            {tab.label}
          </button>
        ))}
      </div>
      <div className="tabBody">
        {active === 'attachments' ? <AttachmentsTab po={po} editable={editable} /> : null}
        {active === 'rows' ? <RowsTab po={po} editable={editable} /> : null}
        {active === 'notes' ? (
          <NotesTab value={header} editable={editable} saving={saving} onChange={onHeaderChange} onSave={onSaveHeader} />
        ) : null}
        {active === 'contacts' ? (
          <ProviderRefTable po={po} provider={provider} editable={editable} onSaveRecipients={onSaveRecipients} />
        ) : null}
      </div>
    </section>
  );
}
