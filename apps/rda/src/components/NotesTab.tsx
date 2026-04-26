import { Button, Icon } from '@mrsmith/ui';
import type { HeaderFormState } from './PoHeaderForm';

export function NotesTab({
  value,
  editable,
  saving,
  onChange,
  onSave,
}: {
  value: HeaderFormState;
  editable: boolean;
  saving: boolean;
  onChange: (value: HeaderFormState) => void;
  onSave: () => void;
}) {
  return (
    <div className="formGrid">
      <div className="field wide">
        <label>Note fornitore</label>
        <textarea rows={5} value={value.note} disabled={!editable} onChange={(event) => onChange({ ...value, note: event.target.value })} />
      </div>
      <div className="field wide">
        <label>Descrizione interna</label>
        <textarea rows={5} value={value.description} disabled={!editable} onChange={(event) => onChange({ ...value, description: event.target.value })} />
      </div>
      <div className="actionRow fullWidth">
        <Button leftIcon={<Icon name="check" />} disabled={!editable} loading={saving} onClick={onSave}>Salva note</Button>
      </div>
    </div>
  );
}
