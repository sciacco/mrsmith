import { useEffect, useRef, useState } from 'react';
import { Button, Icon, useToast } from '@mrsmith/ui';
import type { MATag } from '../../api/types';
import { maTagErrorMessage, useMATagCompanyMutations } from '../../hooks/useMATags';
import { MATagChips } from './MATagChips';
import styles from './CompanyTagEditor.module.css';

interface CompanyTagEditorProps {
  companyKey: string;
  tags: MATag[];
  /** Apre il modal di assegnazione/creazione: lo stato vive nel consumatore. */
  onAddTag: () => void;
  /** Scala compatta (drawer della card): chip sm, comando ghost. */
  compact?: boolean;
  /** aria-label del gruppo; default «Tag dell’azienda». */
  groupLabel?: string;
  className?: string;
}

/**
 * Riga dei tag aziendali condivisi con i due comandi che agiscono sulla SOLA
 * associazione: «Aggiungi tag» (delega al consumatore) e la rimozione sulla
 * chip. Il tag resta sempre nel catalogo e sulle altre aziende.
 *
 * Un unico posto per il comportamento, condiviso da scheda azienda e drawer
 * della card (kanban), così pending, errore persistente e focus da tastiera
 * non divergono tra le due superfici:
 * - pending: spinner sulla chip + comandi disabilitati;
 * - errore: box persistente accanto alla riga con «Riprova» e dismiss, oltre
 *   al toast (UI-UX §14.3: il toast da solo non basta);
 * - focus: dopo una rimozione riuscita la chip viene smontata e il focus
 *   cadrebbe su <body>: va alla chip subentrata nella stessa posizione o, se
 *   non c'è, ad «Aggiungi tag», e solo se il focus era davvero caduto.
 */
export function CompanyTagEditor({ companyKey, tags, onAddTag, compact = false, groupLabel = 'Tag dell’azienda', className }: CompanyTagEditorProps) {
  const { unassign } = useMATagCompanyMutations(companyKey);
  const { toast } = useToast();
  const [removingTagId, setRemovingTagId] = useState<string | null>(null);
  // Errore PERSISTENTE della rimozione (UI-UX §14.3): conserva tag e messaggio
  // per «Riprova» e resta in vista finché una nuova azione non riesce o l'utente
  // non lo dismissa esplicitamente.
  const [unassignError, setUnassignError] = useState<{ tagId: string; message: string } | null>(null);
  const rowRef = useRef<HTMLDivElement | null>(null);
  const addButtonRef = useRef<HTMLButtonElement | null>(null);
  // Rimozione riuscita di cui attendere la sparizione dall'elenco per
  // riposizionare il focus da tastiera.
  const pendingFocusRef = useRef<{ tagId: string; index: number } | null>(null);

  const unassignTag = async (tagId: string) => {
    if (!companyKey) return;
    // Posizione della chip da rimuovere: la chip subentrata alla stessa
    // posizione riceverà il focus dopo il successo (uso da tastiera).
    const tagIndex = tags.findIndex((tag) => tag.id === tagId);
    setRemovingTagId(tagId);
    try {
      await unassign.mutateAsync(tagId);
      setUnassignError(null); // pulizia a azione riuscita
      pendingFocusRef.current = { tagId, index: tagIndex };
      toast('Tag rimosso.', 'success');
    } catch (error) {
      const message = maTagErrorMessage(error);
      setUnassignError({ tagId, message });
      toast(message, 'error');
    } finally {
      setRemovingTagId(null);
    }
  };

  useEffect(() => {
    const pending = pendingFocusRef.current;
    if (!pending) return;
    if (tags.some((tag) => tag.id === pending.tagId)) return; // aggiornamento non ancora arrivato
    pendingFocusRef.current = null;
    if (document.activeElement !== document.body) return;
    const container = rowRef.current;
    const removeButtons = container
      ? Array.from(container.querySelectorAll<HTMLButtonElement>('button[data-tag-remove]'))
      : [];
    const nextButton =
      pending.index >= 0 && pending.index < removeButtons.length ? removeButtons[pending.index] : undefined;
    if (nextButton && !nextButton.disabled) {
      nextButton.focus();
      return;
    }
    addButtonRef.current?.focus();
  }, [tags]);

  const failedTagName = unassignError ? tags.find((tag) => tag.id === unassignError.tagId)?.name : undefined;

  return (
    <div className={[styles.wrap, className ?? ''].filter(Boolean).join(' ')}>
      <div ref={rowRef} className={styles.row} role="group" aria-label={groupLabel}>
        <MATagChips
          tags={tags}
          size={compact ? 'sm' : 'md'}
          onRemove={(tagId) => void unassignTag(tagId)}
          removeDisabled={unassign.isPending}
          removingTagId={removingTagId}
        />
        <Button
          ref={addButtonRef}
          size="sm"
          variant={compact ? 'ghost' : 'secondary'}
          onClick={onAddTag}
          disabled={unassign.isPending}
          leftIcon={<Icon name="plus" size={14} />}
        >
          Aggiungi tag
        </Button>
      </div>
      {unassignError ? (
        <div className={styles.error} role="alert">
          <Icon name="triangle-alert" size={16} aria-hidden="true" />
          <p>
            Rimozione del tag{failedTagName ? ` «${failedTagName}»` : ''} non riuscita: {unassignError.message}
          </p>
          <div className={styles.errorActions}>
            <Button
              size="sm"
              variant="secondary"
              onClick={() => void unassignTag(unassignError.tagId)}
              loading={unassign.isPending && removingTagId === unassignError.tagId}
              disabled={unassign.isPending}
            >
              Riprova
            </Button>
            <button
              type="button"
              className={styles.errorDismiss}
              aria-label="Nascondi l’errore"
              disabled={unassign.isPending}
              onClick={() => setUnassignError(null)}
            >
              <Icon name="x" size={14} aria-hidden="true" />
            </button>
          </div>
        </div>
      ) : null}
    </div>
  );
}
