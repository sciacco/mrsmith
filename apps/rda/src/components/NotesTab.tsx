import type { HeaderFormState } from './PoHeaderForm';

function noteValue(value: string): string {
  return value.trim() || '-';
}

export function NotesTab({ value }: { value: HeaderFormState }) {
  return (
    <div className="summaryNotesGrid notesReadOnly">
      <div className="summaryItem wide">
        <span>Note fornitore</span>
        <strong>{noteValue(value.note)}</strong>
      </div>
      <div className="summaryItem wide">
        <span>Descrizione ad uso interno</span>
        <strong>{noteValue(value.description)}</strong>
      </div>
    </div>
  );
}
