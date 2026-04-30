import { useState } from 'react';
import type { PoDetail, ProviderSummary } from '../api/types';
import type { TabBadgeModel } from '../lib/po-detail-view-model';
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
  { id: 'contacts', label: 'Contatti' },
];

export function PoTabs({
  po,
  provider,
  editable,
  header,
  badges,
  onRecipientSelectionChange,
  onSaveRecipients,
}: {
  po: PoDetail;
  provider?: ProviderSummary;
  editable: boolean;
  header: HeaderFormState;
  badges: TabBadgeModel;
  onRecipientSelectionChange?: (ids: number[]) => void;
  onSaveRecipients: (ids: number[]) => void;
}) {
  const [active, setActive] = useState<TabID>('attachments');
  return (
    <section className="surface">
      <div className="tabs">
        {tabs.map((tab) => (
          <button key={tab.id} className={`tab ${active === tab.id ? 'active' : ''}`} type="button" onClick={() => setActive(tab.id)}>
            <span>{tab.label}</span>
            {tab.id === 'attachments' ? <span className="tabBadge">{badges.attachments}</span> : null}
            {tab.id === 'rows' ? <span className="tabBadge">{badges.rows}</span> : null}
            {tab.id === 'notes' && badges.notesDirty ? <span className="tabDot" aria-label="Note modificate" /> : null}
            {tab.id === 'contacts' ? <span className="tabBadge">{badges.contacts}</span> : null}
          </button>
        ))}
      </div>
      <div className="tabBody">
        {active === 'attachments' ? <AttachmentsTab po={po} editable={editable} /> : null}
        {active === 'rows' ? <RowsTab po={po} editable={editable} /> : null}
        {active === 'notes' ? <NotesTab value={header} /> : null}
        {active === 'contacts' ? (
          <ProviderRefTable
            po={po}
            provider={provider}
            editable={editable}
            onSelectionChange={onRecipientSelectionChange}
            onSaveRecipients={onSaveRecipients}
          />
        ) : null}
      </div>
    </section>
  );
}
