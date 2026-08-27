// Editor anagrafico del percorso (#162, §Catalogo 5): creazione e modifica
// di codice/nome/area/descrizione/attivo. Passi, assegnatari e progresso
// vivono nel dettaglio (PathDetailDrawer.tsx). File proprio per evitare un
// ciclo di import tra PathsCatalogSection.tsx e PathDetailDrawer.tsx, che lo
// usano entrambi.

import { useState } from 'react';
import { Button, Modal, SingleSelect, ToggleSwitch, VisuallyHidden, useToast } from '@mrsmith/ui';
import { useTrainingSkillAreas, useUpsertPath } from '../../api/queries';
import type { PathInput, PathListRow } from '../../api/types';
import { describeApiError } from '../events/apiErrors';
import { ErrorPanel } from '../events/ErrorPanel';
import formStyles from '../requests/requestShared.module.css';

interface PathEditorModalProps {
  mode: 'create' | 'edit';
  pathId?: string;
  initial?: PathListRow;
  open: boolean;
  onClose: () => void;
  onSaved: (id: string) => void;
}

export function PathEditorModal({ mode, pathId, initial, open, onClose, onSaved }: PathEditorModalProps) {
  const { toast } = useToast();
  const skillAreas = useTrainingSkillAreas();
  const upsert = useUpsertPath();

  const [code, setCode] = useState(initial?.code ?? '');
  const [name, setName] = useState(initial?.name ?? '');
  const [skillAreaId, setSkillAreaId] = useState(initial?.skillAreaId ?? '');
  const [description, setDescription] = useState(initial?.description ?? '');
  const [active, setActive] = useState(initial?.active ?? true);
  const [error, setError] = useState<string | null>(null);

  if (!open) return null;

  const canSubmit = code.trim() !== '' && name.trim() !== '';

  async function submit() {
    if (!canSubmit) return;
    setError(null);
    const input: PathInput = {
      code: code.trim(),
      name: name.trim(),
      skillAreaId: skillAreaId || undefined,
      description: description.trim() || undefined,
      active,
    };
    try {
      const response = await upsert.mutateAsync({ id: pathId, input });
      toast(mode === 'create' ? 'Percorso creato' : 'Percorso aggiornato');
      if (response.id) onSaved(response.id);
      else if (pathId) onSaved(pathId);
    } catch (e) {
      setError(describeApiError(e, 'Salvataggio non riuscito'));
    }
  }

  return (
    <Modal open={open} onClose={onClose} title={mode === 'create' ? 'Nuovo percorso' : 'Modifica percorso'} size="md">
      <div className={formStyles.body}>
        <div className={formStyles.row}>
          <label className={formStyles.field}>
            <span className={formStyles.labelHead}>
              Codice
              <span className={formStyles.requiredMarker} aria-hidden="true" />
              <VisuallyHidden>obbligatorio</VisuallyHidden>
            </span>
            <input className={formStyles.input} value={code} onChange={(e) => setCode(e.target.value)} />
          </label>
          <label className={formStyles.field}>
            <span className={formStyles.labelHead}>
              Nome
              <span className={formStyles.requiredMarker} aria-hidden="true" />
              <VisuallyHidden>obbligatorio</VisuallyHidden>
            </span>
            <input className={formStyles.input} value={name} onChange={(e) => setName(e.target.value)} />
          </label>
        </div>
        <label className={formStyles.field}>
          Area di competenza
          <SingleSelect
            options={(skillAreas.data ?? []).filter((a) => a.active).map((a) => ({ value: a.id, label: a.name }))}
            selected={skillAreaId || null}
            onChange={(v) => setSkillAreaId(v ?? '')}
            placeholder="Nessuna"
            allowClear
            searchable
          />
        </label>
        <label className={formStyles.field}>
          Descrizione
          <textarea className={formStyles.textarea} value={description} onChange={(e) => setDescription(e.target.value)} rows={2} />
        </label>
        <div className={formStyles.field}>
          Attivo
          <ToggleSwitch id="path-active" checked={active} onChange={setActive} />
        </div>
        <ErrorPanel message={error} onDismiss={() => setError(null)} />
        <div className={formStyles.actions}>
          <Button variant="ghost" size="md" onClick={onClose} disabled={upsert.isPending}>
            Annulla
          </Button>
          <Button variant="primary" size="md" loading={upsert.isPending} disabled={!canSubmit} onClick={submit}>
            {mode === 'create' ? 'Crea percorso' : 'Salva modifiche'}
          </Button>
        </div>
      </div>
    </Modal>
  );
}
