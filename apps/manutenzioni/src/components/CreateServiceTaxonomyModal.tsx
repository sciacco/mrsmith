import { useEffect, useState } from 'react';
import { Button, Modal, useToast } from '@mrsmith/ui';
import type { ReferenceItem } from '../api/types';
import { useConfigMutations } from '../api/queries';
import { errorMessage } from '../lib/format';
import { slugifyName } from '../lib/smartDefaults';
import styles from './CreateServiceTaxonomyModal.module.css';

interface Props {
  open: boolean;
  initialName: string;
  initialDomainId: number | null;
  domains: ReferenceItem[];
  targetTypes: ReferenceItem[];
  onClose: () => void;
  onCreated: (item: ReferenceItem) => void;
}

export function CreateServiceTaxonomyModal({
  open,
  initialName,
  initialDomainId,
  domains,
  targetTypes,
  onClose,
  onCreated,
}: Props) {
  const { toast } = useToast();
  const config = useConfigMutations('service-taxonomy');
  const [nameIt, setNameIt] = useState('');
  const [code, setCode] = useState('');
  const [domainId, setDomainId] = useState<number | null>(null);
  const [targetTypeId, setTargetTypeId] = useState<number | null>(null);
  const [description, setDescription] = useState('');
  const [nameEn, setNameEn] = useState('');
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const [codeTouched, setCodeTouched] = useState(false);

  useEffect(() => {
    if (!open) return;
    setNameIt(initialName);
    setCode(slugifyName(initialName));
    setDomainId(initialDomainId);
    setTargetTypeId(null);
    setDescription('');
    setNameEn('');
    setAdvancedOpen(false);
    setCodeTouched(false);
  }, [open, initialName, initialDomainId]);

  function updateName(value: string) {
    setNameIt(value);
    if (!codeTouched) {
      setCode(slugifyName(value));
    }
  }

  async function submit() {
    const trimmedName = nameIt.trim();
    const trimmedCode = code.trim();
    if (!trimmedName) {
      toast('Inserisci il nome italiano.', 'error');
      return;
    }
    if (!trimmedCode) {
      toast('Codice non valido.', 'error');
      return;
    }
    if (!domainId) {
      toast('Seleziona il dominio tecnico.', 'error');
      return;
    }
    if (!targetTypeId) {
      toast('Seleziona la natura della voce.', 'error');
      return;
    }
    try {
      const item = await config.create.mutateAsync({
        code: trimmedCode,
        name_it: trimmedName,
        name_en: nameEn.trim() || null,
        description: description.trim() || null,
        technical_domain_id: domainId,
        target_type_id: targetTypeId,
        audience: 'internal',
        is_active: true,
      });
      onCreated(item);
      onClose();
      toast(`Voce "${item.name_it}" creata.`);
    } catch (error) {
      toast(errorMessage(error, 'Creazione voce non riuscita.'), 'error');
    }
  }

  const submitting = config.create.isPending;

  return (
    <Modal open={open} onClose={onClose} title="Crea voce di catalogo" size="md">
      <div className={styles.body}>
        <label className={styles.field}>
          <span className={styles.fieldLabel}>Nome italiano *</span>
          <input
            className={styles.input}
            value={nameIt}
            onChange={(event) => updateName(event.target.value)}
            autoFocus
          />
        </label>
        <div className={styles.row}>
          <label className={styles.field}>
            <span className={styles.fieldLabel}>Dominio tecnico *</span>
            <select
              className={styles.input}
              value={domainId ?? ''}
              onChange={(event) =>
                setDomainId(event.target.value ? Number(event.target.value) : null)
              }
            >
              <option value="">Seleziona…</option>
              {domains.map((domain) => (
                <option key={domain.id} value={domain.id}>
                  {domain.name_it}
                </option>
              ))}
            </select>
          </label>
          <label className={styles.field}>
            <span className={styles.fieldLabel}>Natura *</span>
            <select
              className={styles.input}
              value={targetTypeId ?? ''}
              onChange={(event) =>
                setTargetTypeId(event.target.value ? Number(event.target.value) : null)
              }
            >
              <option value="">Seleziona…</option>
              {targetTypes.map((tt) => (
                <option key={tt.id} value={tt.id}>
                  {tt.name_it}
                </option>
              ))}
            </select>
          </label>
        </div>
        <button
          type="button"
          className={styles.advancedToggle}
          onClick={() => setAdvancedOpen((v) => !v)}
        >
          {advancedOpen ? '▾ Nascondi avanzate' : '▸ Avanzate (codice, descrizione)'}
        </button>
        {advancedOpen ? (
          <div className={styles.advanced}>
            <label className={styles.field}>
              <span className={styles.fieldLabel}>Codice (auto)</span>
              <input
                className={styles.input}
                value={code}
                onChange={(event) => {
                  setCode(event.target.value);
                  setCodeTouched(true);
                }}
              />
            </label>
            <label className={styles.field}>
              <span className={styles.fieldLabel}>Nome inglese</span>
              <input
                className={styles.input}
                value={nameEn}
                onChange={(event) => setNameEn(event.target.value)}
              />
            </label>
            <label className={styles.field}>
              <span className={styles.fieldLabel}>Descrizione</span>
              <textarea
                className={styles.textarea}
                value={description}
                onChange={(event) => setDescription(event.target.value)}
                rows={2}
              />
            </label>
          </div>
        ) : null}
      </div>
      <div className={styles.actions}>
        <Button variant="secondary" onClick={onClose} disabled={submitting}>
          Annulla
        </Button>
        <Button onClick={submit} loading={submitting}>
          Crea voce
        </Button>
      </div>
    </Modal>
  );
}
