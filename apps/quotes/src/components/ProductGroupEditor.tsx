import { useId, useState, type ReactNode } from 'react';
import { Icon } from '@mrsmith/ui';
import type { DocumentType, ProductGroup, ProductVariant } from '../api/types';
import type { KitEditorForm } from '../hooks/useKitEditorForm';
import styles from './ProductGroupEditor.module.css';

interface ProductGroupEditorProps {
  group: ProductGroup;
  documentType: DocumentType;
  form: KitEditorForm;
  isMissing: boolean;
}

function normalize(s: string): string {
  return s
    .toLowerCase()
    .replace(/[()[\],.\-_/]/g, ' ')
    .replace(/\s+/g, ' ')
    .trim();
}

function isRedundant(groupName: string, variantName: string): boolean {
  const g = normalize(groupName);
  const v = normalize(variantName);
  if (!g || !v) return false;
  return g === v || g.includes(v) || v.includes(g);
}

function formatPrice(value: number): string {
  return value.toFixed(2);
}

export function ProductGroupEditor({ group, documentType, form, isMissing }: ProductGroupEditorProps) {
  const uid = useId();
  const radioName = `group-${uid}`;
  const isSpot = documentType === 'TSC-ORDINE';

  const onlyVariant: ProductVariant | null =
    group.products.length === 1 ? group.products[0] ?? null : null;
  const flatCollapsed =
    onlyVariant !== null &&
    !group.required &&
    isRedundant(group.group_name, onlyVariant.product_name);

  const [open, setOpen] = useState(true);
  const noneSelected = !group.products.some(p => form.state.get(p.id)?.included);

  if (flatCollapsed && onlyVariant) {
    return (
      <ProductRow
        variant={onlyVariant}
        form={form}
        groupName={group.group_name}
        controlType="checkbox"
        isSpot={isSpot}
        isMissing={false}
      />
    );
  }

  return (
    <section
      className={`${styles.section} ${isMissing ? styles.sectionMissing : ''}`}
      aria-labelledby={`${uid}-title`}
    >
      <header className={styles.eyebrow}>
        <button
          type="button"
          className={styles.eyebrowToggle}
          onClick={() => setOpen(o => !o)}
          aria-expanded={open}
        >
          <span className={`${styles.chevron} ${open ? styles.chevronOpen : ''}`} aria-hidden="true">
            <Icon name="chevron-right" size={12} />
          </span>
          <span id={`${uid}-title`} className={styles.eyebrowName}>
            {group.group_name}
          </span>
        </button>
        {group.required && <span className={styles.eyebrowMeta}>· Obbligatorio</span>}
        {isMissing && <span className={styles.eyebrowMissing}>· Selezione richiesta</span>}
      </header>

      {open && <div className={styles.sectionRows}>
        {!group.required && (
          <NoneRow
            checked={noneSelected}
            name={radioName}
            onSelect={() => form.setIncludedForGroup(group.group_name, null)}
          />
        )}
        {group.products.map(p => (
          <ProductRow
            key={p.id}
            variant={p}
            form={form}
            groupName={group.group_name}
            controlType="radio"
            radioName={radioName}
            isSpot={isSpot}
            isMissing={isMissing}
          />
        ))}
      </div>}
    </section>
  );
}

interface NoneRowProps {
  checked: boolean;
  name: string;
  onSelect: () => void;
}

function NoneRow({ checked, name, onSelect }: NoneRowProps) {
  return (
    <div className={`${styles.row} ${styles.rowNone} ${checked ? styles.rowActive : ''}`}>
      <label className={styles.rowMain}>
        <input
          type="radio"
          className={styles.control}
          name={name}
          checked={checked}
          onChange={onSelect}
        />
        <span className={styles.noneLabel}>Non incluso</span>
      </label>
    </div>
  );
}

interface ProductRowProps {
  variant: ProductVariant;
  form: KitEditorForm;
  groupName: string;
  controlType: 'checkbox' | 'radio';
  radioName?: string;
  isSpot: boolean;
  isMissing: boolean;
}

function ProductRow({
  variant,
  form,
  groupName,
  controlType,
  radioName,
  isSpot,
  isMissing,
}: ProductRowProps) {
  const entry = form.state.get(variant.id);
  if (!entry) return null;
  const active = entry.included;
  const displayMrc = isSpot ? 0 : variant.mrc;

  const handleToggle = () => {
    if (controlType === 'checkbox') {
      form.setIncludedForGroup(groupName, active ? null : variant.id);
    } else {
      form.setIncludedForGroup(groupName, variant.id);
    }
  };

  return (
    <div
      className={`${styles.row} ${active ? styles.rowActive : ''} ${isMissing ? styles.rowMissing : ''}`}
    >
      <label className={styles.rowMain}>
        <input
          type={controlType}
          className={styles.control}
          name={controlType === 'radio' ? radioName : undefined}
          checked={active}
          onChange={handleToggle}
        />
        <div className={styles.nameCell}>
          <span className={styles.name}>{variant.product_name}</span>
          <span className={styles.code}>{variant.product_code}</span>
        </div>
        <div className={styles.prices}>
          <PriceSlot label="NRC" value={variant.nrc} dimmed={!active} />
          <PriceSlot label="MRC" value={displayMrc} dimmed={!active} />
        </div>
      </label>

      {active && (
        <div className={styles.expand}>
          <div className={styles.fields}>
            <Field label="Quantità">
              <input
                type="number"
                className={styles.input}
                min={variant.minimum > 0 ? variant.minimum : 0}
                max={variant.maximum || undefined}
                value={entry.quantity}
                onChange={e => form.setProductField(variant.id, 'quantity', Number(e.target.value))}
              />
            </Field>
            <Field label="NRC">
              <input
                type="number"
                step="0.01"
                className={styles.input}
                value={entry.nrc}
                onChange={e => form.setProductField(variant.id, 'nrc', Number(e.target.value))}
              />
            </Field>
            <Field label="MRC">
              <input
                type="number"
                step="0.01"
                className={`${styles.input} ${isSpot ? styles.inputDisabled : ''}`}
                value={isSpot ? 0 : entry.mrc}
                disabled={isSpot}
                onChange={e => form.setProductField(variant.id, 'mrc', Number(e.target.value))}
                title={isSpot ? 'MRC non disponibile per ordini spot' : undefined}
              />
            </Field>
          </div>
          <Field label="Descrizione aggiuntiva">
            <textarea
              className={styles.textarea}
              rows={2}
              placeholder="Personalizza la descrizione per questo cliente…"
              value={entry.extended_description ?? ''}
              onChange={e =>
                form.setProductField(
                  variant.id,
                  'extended_description',
                  e.target.value === '' ? null : e.target.value,
                )
              }
            />
          </Field>
        </div>
      )}
    </div>
  );
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className={styles.field}>
      <span className={styles.fieldLabel}>{label}</span>
      {children}
    </label>
  );
}

function PriceSlot({ label, value, dimmed }: { label: string; value: number; dimmed: boolean }) {
  return (
    <span className={`${styles.priceSlot} ${dimmed ? styles.priceDimmed : ''}`}>
      <span className={styles.priceLabel}>{label}</span>
      <span className={styles.priceValue}>{formatPrice(value)}</span>
    </span>
  );
}
