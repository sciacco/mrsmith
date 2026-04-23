import type { AdhocSiteInput, ReferenceItem } from '../api/types';
import shared from '../pages/shared.module.css';

const NEW_SENTINEL = '__new__';

export interface SiteSelectValue {
  site_id: number | null;
  adhoc_site: AdhocSiteInput | null;
}

interface SiteSelectFieldProps {
  label?: string;
  sites: ReferenceItem[];
  value: SiteSelectValue;
  onChange: (next: SiteSelectValue) => void;
  /** Scope del sito attualmente legato alla manutenzione (solo per layout informativo). */
  currentScope?: string | null;
}

export function SiteSelectField({
  label = 'Sito',
  sites,
  value,
  onChange,
  currentScope,
}: SiteSelectFieldProps) {
  const isAdhocMode = value.adhoc_site !== null;
  const selectValue = isAdhocMode
    ? NEW_SENTINEL
    : value.site_id != null
    ? String(value.site_id)
    : '';

  function handleSelect(raw: string) {
    if (raw === NEW_SENTINEL) {
      onChange({
        site_id: null,
        adhoc_site: { name: '', city: null, country_code: null },
      });
      return;
    }
    if (raw === '') {
      onChange({ site_id: null, adhoc_site: null });
      return;
    }
    onChange({ site_id: Number(raw), adhoc_site: null });
  }

  function patchAdhoc(patch: Partial<AdhocSiteInput>) {
    if (!value.adhoc_site) return;
    onChange({
      site_id: null,
      adhoc_site: { ...value.adhoc_site, ...patch },
    });
  }

  const scopedBadge =
    !isAdhocMode && value.site_id != null && currentScope === 'scoped' ? (
      <span
        style={{
          marginLeft: '0.4rem',
          padding: '0.05rem 0.45rem',
          borderRadius: '999px',
          background: 'rgba(56, 189, 248, 0.15)',
          color: 'rgb(125, 211, 252)',
          fontSize: '0.7rem',
          fontWeight: 600,
          textTransform: 'uppercase',
          letterSpacing: '0.04em',
        }}
      >
        Ad-hoc
      </span>
    ) : null;

  return (
    <label className={shared.label}>
      <span>
        {label}
        {scopedBadge}
      </span>
      <select
        className={shared.select}
        value={selectValue}
        onChange={(event) => handleSelect(event.target.value)}
      >
        <option value="">Nessun sito</option>
        {sites.map((item) => (
          <option key={item.id} value={item.id}>
            {item.name_it}
          </option>
        ))}
        <option value={NEW_SENTINEL}>+ Aggiungi un sito solo per questa manutenzione</option>
      </select>
      {isAdhocMode && value.adhoc_site ? (
        <div
          style={{
            display: 'grid',
            gap: '0.5rem',
            padding: '0.75rem',
            border: '1px solid var(--border-muted, #1f2937)',
            borderRadius: '6px',
            marginTop: '0.35rem',
          }}
        >
          <input
            className={shared.field}
            placeholder="Nome del sito (obbligatorio)"
            value={value.adhoc_site.name}
            onChange={(event) => patchAdhoc({ name: event.target.value })}
            autoFocus
          />
          <input
            className={shared.field}
            placeholder="Città"
            value={value.adhoc_site.city ?? ''}
            onChange={(event) => patchAdhoc({ city: event.target.value || null })}
          />
          <input
            className={shared.field}
            placeholder="Country code (ISO 2, es: IT)"
            maxLength={2}
            value={value.adhoc_site.country_code ?? ''}
            onChange={(event) =>
              patchAdhoc({ country_code: event.target.value.toUpperCase() || null })
            }
          />
          <button
            type="button"
            style={{
              justifySelf: 'start',
              fontSize: '0.85rem',
              background: 'transparent',
              border: 'none',
              color: 'var(--color-accent, #60a5fa)',
              cursor: 'pointer',
              padding: 0,
              textDecoration: 'underline',
            }}
            onClick={() => onChange({ site_id: null, adhoc_site: null })}
          >
            Annulla sito ad-hoc
          </button>
        </div>
      ) : null}
    </label>
  );
}
