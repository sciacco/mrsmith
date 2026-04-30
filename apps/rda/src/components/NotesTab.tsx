import type { HeaderFormState } from './PoHeaderForm';
import { PoNotesDisclosures } from './PoNotesDisclosures';

export function NotesTab({ value }: { value: HeaderFormState }) {
  return <PoNotesDisclosures note={value.note} description={value.description} />;
}
