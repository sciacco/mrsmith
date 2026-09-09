import { Button, Icon, Modal, Skeleton, useToast, VisuallyHidden } from '@mrsmith/ui';
import { useEffect, useMemo, useState, type FormEvent } from 'react';
import type { MATag } from '../../api/types';
import { maTagErrorMessage, useMATagCatalog, useMATagCompanyMutations } from '../../hooks/useMATags';
import { errorLabel } from '../../pages/ricerche/helpers';
import styles from './AddCompanyTagModal.module.css';

interface AddCompanyTagModalProps {
  open: boolean;
  onClose: () => void;
  companyKey: string;
  /** Tag già sull'azienda: non sono selezionabili (niente doppie associazioni). */
  currentTags: MATag[];
}

/**
 * «Aggiungi tag» dalla scheda azienda (issue #198). Un solo modal per i due
 * percorsi, con transizione ESPLICITA: il catalogo condiviso si percorre con il
 * comando «Assegna» per riga; la creazione parte solo dal comando «Crea nuovo
 * tag», che mostra il form dedicato. La rimozione non vive qui: resta sulla
 * chip in testata. Rinomina ed eliminazione globale non esistono in scheda
 * (vivranno in Strumenti → Gestione tag).
 *
 * Ogni mutazione passa da useMATagCompanyMutations: le invalidazioni di
 * catalogo, overview, elenco, board e pipeline sono centralizzate nell'hook.
 */
export function AddCompanyTagModal({ open, onClose, companyKey, currentTags }: AddCompanyTagModalProps) {
  const catalog = useMATagCatalog();
  const { createAndAssign, assign } = useMATagCompanyMutations(companyKey);
  const { toast } = useToast();

  const [mode, setMode] = useState<'list' | 'create'>('list');
  const [name, setName] = useState('');
  const [nameError, setNameError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  // Stato pulito a ogni apertura: il modal non conserva bozze tra aziende.
  // A ogni apertura rilegge anche il catalogo condiviso: il modal resta
  // MONTATO (la pagina lo rende sempre), quindi refetchOnMount non interviene
  // sulle riaperture e un collega può aver cambiato il catalogo nel frattempo
  // (issue #198). L'effetto scatta solo sulla transizione open false→true e
  // il refetch non tocca `open`: nessun loop né doppio refetch. La politica
  // centrale maTagQueryPolicy (mount/focus) resta invariata.
  useEffect(() => {
    if (!open) return;
    setMode('list');
    setName('');
    setNameError(null);
    setActionError(null);
    void catalog.refetch();
  }, [open, catalog.refetch]);

  const assignedIds = useMemo(() => new Set(currentTags.map((tag) => tag.id)), [currentTags]);
  const sortedCatalog = useMemo(
    () => [...(catalog.data ?? [])].sort((a, b) => a.name.localeCompare(b.name, 'it')),
    [catalog.data],
  );
  const busy = assign.isPending || createAndAssign.isPending;

  const handleAssign = async (tag: MATag) => {
    setActionError(null);
    try {
      await assign.mutateAsync(tag.id);
      toast('Tag assegnato.', 'success');
      onClose();
    } catch (error) {
      setActionError(maTagErrorMessage(error));
    }
  };

  const submitCreate = async (event?: FormEvent) => {
    event?.preventDefault();
    setActionError(null);
    const trimmed = name.trim();
    if (!trimmed) {
      setNameError('Inserisci un nome per il tag.');
      return;
    }
    // Regola di dominio: il confronto ignora spazi esterni e maiuscole, come il
    // indice univoco del backend («Cloud» e « cloud » sono lo stesso tag).
    const equivalent = [...sortedCatalog, ...currentTags].some((tag) => tag.name.trim().toLowerCase() === trimmed.toLowerCase());
    if (equivalent) {
      setNameError('Esiste già un tag con questo nome.');
      return;
    }
    try {
      await createAndAssign.mutateAsync(trimmed);
      toast('Tag creato e assegnato.', 'success');
      onClose();
    } catch (error) {
      setActionError(maTagErrorMessage(error));
    }
  };

  const backToList = () => {
    setMode('list');
    setName('');
    setNameError(null);
    setActionError(null);
  };

  return (
    <Modal open={open} onClose={onClose} title="Aggiungi tag" size="sm" dismissible={!busy}>
      {mode === 'list' ? (
        <div className={styles.body}>
          <p className={styles.intro}>Scegli un tag del catalogo condiviso da assegnare all’azienda.</p>

          {catalog.isLoading ? (
            <Skeleton rows={3} />
          ) : catalog.isError ? (
            <div className={styles.errorBox} role="alert">
              <Icon name="triangle-alert" size={16} />
              <span>{errorLabel(catalog.error)}</span>
              <Button size="sm" variant="secondary" onClick={() => void catalog.refetch()}>
                Riprova
              </Button>
            </div>
          ) : sortedCatalog.length === 0 ? (
            <p className={styles.emptyNote}>Il catalogo condiviso è vuoto: crea il primo tag per iniziare a classificare le aziende.</p>
          ) : (
            <ul className={styles.catalog}>
              {sortedCatalog.map((tag) => {
                const assigned = assignedIds.has(tag.id);
                return (
                  <li key={tag.id} className={styles.option}>
                    <span className={styles.optionName}>{tag.name}</span>
                    {assigned ? (
                      <span className={styles.assignedBadge}>Assegnato</span>
                    ) : (
                      <Button
                        variant="secondary"
                        size="sm"
                        loading={assign.isPending && assign.variables === tag.id}
                        disabled={busy}
                        onClick={() => void handleAssign(tag)}
                      >
                        Assegna
                      </Button>
                    )}
                  </li>
                );
              })}
            </ul>
          )}

          {actionError ? (
            <p className={styles.formError} role="alert">
              {actionError}
            </p>
          ) : null}

          {/* Transizione esplicita alla creazione: mai un campo implicito che
              cambia semantica mentre si digita. */}
          <div className={styles.createRow}>
            <Button
              variant="secondary"
              size="sm"
              disabled={busy}
              onClick={() => {
                setMode('create');
                setActionError(null);
              }}
              leftIcon={<Icon name="plus" size={14} />}
            >
              Crea nuovo tag
            </Button>
          </div>
        </div>
      ) : (
        <form className={styles.body} onSubmit={(event) => void submitCreate(event)} noValidate>
          <label htmlFor="ma-new-tag-name" className={styles.fieldLabel}>
            <span className={styles.labelText}>
              Nome del tag
              <span className={styles.requiredDot} aria-hidden="true" />
              <VisuallyHidden>obbligatorio</VisuallyHidden>
            </span>
            <input
              id="ma-new-tag-name"
              type="text"
              value={name}
              autoFocus
              required
              placeholder="Es. Passaggio generazionale"
              aria-invalid={nameError ? true : undefined}
              aria-describedby={nameError ? 'ma-new-tag-name-error' : undefined}
              onChange={(event) => {
                setName(event.target.value);
                setNameError(null);
              }}
            />
          </label>
          <p className={styles.helper}>Il tag entra nel catalogo condiviso e resta disponibile per tutto il team.</p>
          {nameError ? (
            <p id="ma-new-tag-name-error" className={styles.formError} role="alert">
              {nameError}
            </p>
          ) : null}
          {actionError ? (
            <p className={styles.formError} role="alert">
              {actionError}
            </p>
          ) : null}
          <div className={styles.modalActions}>
            <Button type="button" variant="secondary" onClick={backToList} disabled={createAndAssign.isPending}>
              Annulla
            </Button>
            <Button type="submit" variant="primary" loading={createAndAssign.isPending}>
              Crea e assegna
            </Button>
          </div>
        </form>
      )}
    </Modal>
  );
}
