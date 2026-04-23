import { SingleSelect } from '@mrsmith/ui';
import type { AdhocSiteInput, ReferenceItem } from '../api/types';
import shared from '../pages/shared.module.css';

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

  function enterAdhoc() {
    onChange({
      site_id: null,
      adhoc_site: { name: '', city: null, country_code: null },
    });
  }

  function exitAdhoc() {
    onChange({ site_id: null, adhoc_site: null });
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
      <span className={shared.siteScopedBadge}>Ad-hoc</span>
    ) : null;

  const options = sites.map((item) => ({ value: item.id, label: item.name_it }));

  return (
    <div className={shared.fieldGroup}>
      <span className={shared.fieldLabel}>
        {label}
        {scopedBadge}
      </span>

      {isAdhocMode && value.adhoc_site ? (
        <div className={shared.siteAdhocBlock}>
          <div className={shared.siteAdhocHeader}>
            <span className={shared.siteAdhocTitle}>Sito ad-hoc</span>
            <button type="button" className={shared.siteAdhocBackLink} onClick={exitAdhoc}>
              ← Torna all'elenco siti
            </button>
          </div>
          <input
            className={shared.field}
            placeholder="Nome del sito"
            value={value.adhoc_site.name}
            onChange={(event) => patchAdhoc({ name: event.target.value })}
            autoFocus
          />
          <div className={shared.siteAdhocGrid}>
            <input
              className={shared.field}
              placeholder="Città"
              value={value.adhoc_site.city ?? ''}
              onChange={(event) => patchAdhoc({ city: event.target.value || null })}
            />
            <input
              className={shared.field}
              placeholder="Paese (ISO 2)"
              maxLength={2}
              value={value.adhoc_site.country_code ?? ''}
              onChange={(event) =>
                patchAdhoc({ country_code: event.target.value.toUpperCase() || null })
              }
            />
          </div>
          <span className={shared.fieldHelper}>
            Il sito resterà agganciato solo a questa manutenzione.
          </span>
        </div>
      ) : (
        <>
          <SingleSelect<number>
            options={options}
            selected={value.site_id}
            onChange={(next) => onChange({ site_id: next, adhoc_site: null })}
            placeholder="Cerca un sito…"
            allowClear
          />
          <span className={shared.siteAdhocLink}>
            Il sito non è in elenco?{' '}
            <button type="button" onClick={enterAdhoc}>
              Aggiungi un sito ad-hoc
            </button>
          </span>
        </>
      )}
    </div>
  );
}
