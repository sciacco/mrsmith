interface NoteSection {
  id: string;
  label: string;
  value: string;
}

function normalizeNote(value?: string | null): string {
  return value?.trim() ?? '';
}

export function PoNotesDisclosures({
  note,
  description,
}: {
  note?: string | null;
  description?: string | null;
}) {
  const sections: NoteSection[] = [
    { id: 'supplier-note', label: 'Note fornitore', value: normalizeNote(note) },
    { id: 'internal-description', label: 'Descrizione ad uso interno', value: normalizeNote(description) },
  ].filter((section) => section.value !== '');

  if (sections.length === 0) return null;

  return (
    <div className="poNotesDisclosures">
      {sections.map((section) => (
        <details key={section.id} className="poNoteDisclosure">
          <summary>
            <span>{section.label}</span>
          </summary>
          <p>{section.value}</p>
        </details>
      ))}
    </div>
  );
}
