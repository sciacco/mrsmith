import { useId } from 'react';
import { Icon } from '@mrsmith/ui';
import type { DocumentType, ProductGroup } from '../api/types';
import type { KitEditorForm } from '../hooks/useKitEditorForm';
import styles from './ProductGroupEditor.module.css';

interface ProductGroupEditorProps {
  group: ProductGroup;
  documentType: DocumentType;
  form: KitEditorForm;
  isMissing: boolean;
}

export function ProductGroupEditor({ group, documentType, form, isMissing }: ProductGroupEditorProps) {
  const uid = useId();
  const isSpot = documentType === 'TSC-ORDINE';
  const radioName = `group-${uid}`;

  const noneSelected = !group.products.some(p => form.state.get(p.id)?.included);
  // Single-variant optional groups collapse the "Non incluso" row into a single checkbox.
  const singleCheckbox = group.products.length === 1 && !group.required;

  return (
    <section
      className={`${styles.group} ${isMissing ? styles.missing : ''}`}
      aria-labelledby={`${uid}-title`}
    >
      <header className={styles.header}>
        <h3 id={`${uid}-title`} className={styles.title}>
          {group.group_name}
        </h3>
        <div className={styles.badges}>
          {group.required && <span className={styles.badgeRequired}>Obbligatorio</span>}
          {isMissing && (
            <span className={styles.badgeMissing}>
              <Icon name="triangle-alert" size={12} /> Selezione richiesta
            </span>
          )}
        </div>
      </header>

      <ul className={styles.variants}>
        {!group.required && !singleCheckbox && (
          <li className={`${styles.variant} ${noneSelected ? styles.variantActive : ''}`}>
            <label className={styles.variantHead}>
              <input
                type="radio"
                className={styles.radio}
                name={radioName}
                checked={noneSelected}
                onChange={() => form.setIncludedForGroup(group.group_name, null)}
              />
              <span className={styles.variantName}>Non incluso</span>
            </label>
          </li>
        )}

        {group.products.map(p => {
          const entry = form.state.get(p.id);
          if (!entry) return null;
          const active = entry.included;
          return (
            <li
              key={p.id}
              className={`${styles.variant} ${active ? styles.variantActive : styles.variantDimmed}`}
            >
              <label className={styles.variantHead}>
                <input
                  type={singleCheckbox ? 'checkbox' : 'radio'}
                  className={styles.radio}
                  name={singleCheckbox ? undefined : radioName}
                  checked={active}
                  onChange={() =>
                    form.setIncludedForGroup(group.group_name, active ? null : p.id)
                  }
                />
                <div className={styles.variantInfo}>
                  <span className={styles.variantName}>{p.product_name}</span>
                  <span className={styles.variantCode}>{p.product_code}</span>
                </div>
              </label>

              {active && (
                <div className={styles.fields}>
                  <div className={styles.field}>
                    <label className={styles.fieldLabel}>Quantità</label>
                    <input
                      type="number"
                      className={styles.input}
                      min={p.minimum > 0 ? p.minimum : 0}
                      max={p.maximum || undefined}
                      value={entry.quantity}
                      onChange={e => form.setProductField(p.id, 'quantity', Number(e.target.value))}
                    />
                  </div>
                  <div className={styles.field}>
                    <label className={styles.fieldLabel}>NRC</label>
                    <input
                      type="number"
                      step="0.01"
                      className={styles.input}
                      value={entry.nrc}
                      onChange={e => form.setProductField(p.id, 'nrc', Number(e.target.value))}
                    />
                  </div>
                  <div className={styles.field}>
                    <label className={styles.fieldLabel}>MRC</label>
                    <input
                      type="number"
                      step="0.01"
                      className={`${styles.input} ${isSpot ? styles.inputDisabled : ''}`}
                      value={isSpot ? 0 : entry.mrc}
                      disabled={isSpot}
                      onChange={e => form.setProductField(p.id, 'mrc', Number(e.target.value))}
                      title={isSpot ? 'MRC non disponibile per ordini spot' : undefined}
                    />
                  </div>
                  <div className={`${styles.field} ${styles.fieldFull}`}>
                    <label className={styles.fieldLabel}>Descrizione aggiuntiva</label>
                    <textarea
                      className={styles.textarea}
                      rows={2}
                      placeholder="Personalizza la descrizione per questo cliente…"
                      value={entry.extended_description ?? ''}
                      onChange={e =>
                        form.setProductField(
                          p.id,
                          'extended_description',
                          e.target.value === '' ? null : e.target.value,
                        )
                      }
                    />
                  </div>
                </div>
              )}
            </li>
          );
        })}
      </ul>
    </section>
  );
}
