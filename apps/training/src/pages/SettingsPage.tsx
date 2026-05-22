import { ApiError } from '@mrsmith/api-client';
import {
  Button,
  Drawer,
  Icon,
  SearchInput,
  SingleSelect,
  Skeleton,
  TabNav,
  ToggleSwitch,
  useToast,
  type IconName,
} from '@mrsmith/ui';
import { useMemo, useState, type FormEvent, type ReactNode } from 'react';
import { useSearchParams } from 'react-router-dom';
import {
  useCreateTrainingMasterData,
  useTrainingWorkspace,
  useUpdateTrainingMasterData,
  type TrainingMasterDataKind,
} from '../api/queries';
import type {
  CatalogCertificationRow,
  SkillAreaRow,
  TeamRow,
  VendorRow,
} from '../api/types';
import styles from './SettingsPage.module.css';

type SectionKey = 'team' | 'skill-area' | 'vendors' | 'certifications';
type StatusFilter = '' | 'attivo' | 'disattivato';

interface BaseDraft {
  active: boolean;
}

interface TeamDraft extends BaseDraft {
  code: string;
  name: string;
  description: string;
}

interface SkillAreaDraft extends BaseDraft {
  code: string;
  name: string;
  parentId: string;
  description: string;
}

interface VendorDraft extends BaseDraft {
  name: string;
  website: string;
  notes: string;
}

interface CertificationDraft extends BaseDraft {
  code: string;
  name: string;
  issuerVendorId: string;
  skillAreaId: string;
  typicalValidityMonths: string;
  description: string;
}

interface MasterItem {
  id: string;
  active: boolean;
}

interface FieldRenderProps<T extends MasterItem, D extends BaseDraft> {
  draft: D;
  item: T | null;
  prefix: string;
  updateDraft: (patch: Partial<D>) => void;
}

interface SectionDefinition<T extends MasterItem, D extends BaseDraft> {
  key: SectionKey;
  kind: TrainingMasterDataKind;
  label: string;
  title: string;
  subtitle: string;
  icon: IconName;
  newLabel: string;
  singular: string;
  plural: string;
  searchPlaceholder: string;
  emptyLabel: string;
  createdToast: string;
  updatedToast: string;
  emptyDraft: () => D;
  itemToDraft: (item: T) => D;
  buildPayload: (draft: D, item: T | null) => Record<string, unknown>;
  validate: (draft: D, item: T | null) => string | null;
  getTitle: (item: T) => string;
  getCode?: (item: T) => string | undefined;
  getMeta: (item: T) => string[];
  searchText: (item: T) => string;
  renderFields: (props: FieldRenderProps<T, D>) => ReactNode;
}

type DrawerState<T extends MasterItem, D extends BaseDraft> =
  | { mode: 'closed' }
  | { mode: 'new'; draft: D }
  | { mode: 'edit'; item: T; draft: D };

const SECTION_TABS = [
  { key: 'team', label: 'Team' },
  { key: 'skill-area', label: 'Skill area' },
  { key: 'vendors', label: 'Fornitori' },
  { key: 'certifications', label: 'Certificazioni' },
] satisfies Array<{ key: SectionKey; label: string }>;

const STATUS_OPTIONS = [
  { value: '', label: 'Tutti gli stati' },
  { value: 'attivo', label: 'Attivi' },
  { value: 'disattivato', label: 'Disattivati' },
];

const INITIAL_SEARCH: Record<SectionKey, string> = {
  team: '',
  'skill-area': '',
  vendors: '',
  certifications: '',
};

const INITIAL_STATUS: Record<SectionKey, StatusFilter> = {
  team: 'attivo',
  'skill-area': 'attivo',
  vendors: 'attivo',
  certifications: 'attivo',
};

export function SettingsPage({ isPeopleAdmin }: { isPeopleAdmin: boolean }) {
  const [params, setParams] = useSearchParams();
  const workspace = useTrainingWorkspace(isPeopleAdmin);
  const createMasterData = useCreateTrainingMasterData(isPeopleAdmin);
  const updateMasterData = useUpdateTrainingMasterData(isPeopleAdmin);
  const [searchBySection, setSearchBySection] = useState<Record<SectionKey, string>>(INITIAL_SEARCH);
  const [statusBySection, setStatusBySection] = useState<Record<SectionKey, StatusFilter>>(INITIAL_STATUS);

  const activeSection = toSectionKey(params.get('sezione')) ?? 'team';
  const masterData = workspace.data?.masterData;
  const teams = masterData?.teams ?? [];
  const skillAreas = masterData?.skillAreas ?? [];
  const vendors = masterData?.vendors ?? [];
  const certifications = masterData?.certifications ?? [];

  const skillAreaOptions = useMemo(
    () => skillAreas.map((area) => ({ value: area.id, label: labelWithCode(area.code, area.name), active: area.active })),
    [skillAreas],
  );
  const vendorOptions = useMemo(
    () => vendors.map((vendor) => ({ value: vendor.id, label: vendor.name, active: vendor.active })),
    [vendors],
  );

  const sectionTabs = useMemo(() => SECTION_TABS.map(({ key, label }) => ({ key, label })), []);
  const isSaving = createMasterData.isPending || updateMasterData.isPending;

  if (!isPeopleAdmin) {
    return (
      <main className={styles.page}>
        <p className={styles.muted}>Accesso riservato al team People.</p>
      </main>
    );
  }

  function changeSection(key: string) {
    const nextSection = toSectionKey(key) ?? 'team';
    setParams((previous) => {
      const next = new URLSearchParams(previous);
      if (nextSection === 'team') next.delete('sezione');
      else next.set('sezione', nextSection);
      return next;
    }, { replace: true });
  }

  function updateSearch(section: SectionKey, value: string) {
    setSearchBySection((current) => ({ ...current, [section]: value }));
  }

  function updateStatus(section: SectionKey, value: StatusFilter) {
    setStatusBySection((current) => ({ ...current, [section]: value }));
  }

  async function createItem(kind: TrainingMasterDataKind, body: Record<string, unknown>) {
    await createMasterData.mutateAsync({ kind, body });
  }

  async function updateItem(kind: TrainingMasterDataKind, id: string, body: Record<string, unknown>) {
    await updateMasterData.mutateAsync({ kind, id, body });
  }

  const commonProps = {
    loading: workspace.isLoading,
    error: workspace.error,
    search: searchBySection[activeSection],
    status: statusBySection[activeSection],
    onSearchChange: (value: string) => updateSearch(activeSection, value),
    onStatusChange: (value: StatusFilter) => updateStatus(activeSection, value),
    onCreate: createItem,
    onUpdate: updateItem,
    isSaving,
  };

  const teamDefinition: SectionDefinition<TeamRow, TeamDraft> = {
    key: 'team',
    kind: 'teams',
    label: 'Team',
    title: 'Team',
    subtitle: 'Strutture usate per persone, pianificazione e regole.',
    icon: 'user',
    newLabel: 'Nuovo team',
    singular: 'team',
    plural: 'team',
    searchPlaceholder: 'Cerca team',
    emptyLabel: 'Nessun team trovato.',
    createdToast: 'Team creato',
    updatedToast: 'Team aggiornato',
    emptyDraft: () => ({ code: '', name: '', description: '', active: true }),
    itemToDraft: (item) => ({
      code: item.code,
      name: item.name,
      description: item.description ?? '',
      active: item.active,
    }),
    buildPayload: (draft, item) => ({
      code: codeFromNameOrExisting(draft.name, 'TEAM', item?.code),
      name: draft.name.trim(),
      description: emptyToUndefined(draft.description),
      active: draft.active,
    }),
    validate: (draft) => {
      if (!draft.name.trim()) return 'Nome obbligatorio.';
      return null;
    },
    getTitle: (item) => item.name,
    getCode: (item) => item.code,
    getMeta: (item) => [item.description || 'Nessuna descrizione'],
    searchText: (item) => joinSearch(item.code, item.name, item.description),
    renderFields: ({ draft, item, prefix, updateDraft }) => (
      <>
        {item ? (
          <div className={styles.gridTwo}>
            <ReadOnlyCodeField label="Codice" value={item.code} />
            <TextField id={`${prefix}-name`} label="Nome" value={draft.name} onChange={(name) => updateDraft({ name })} required />
          </div>
        ) : (
          <TextField id={`${prefix}-name`} label="Nome" value={draft.name} onChange={(name) => updateDraft({ name })} required />
        )}
        <TextareaField
          id={`${prefix}-description`}
          label="Descrizione"
          value={draft.description}
          onChange={(description) => updateDraft({ description })}
        />
        <ActiveField id={`${prefix}-active`} active={draft.active} onChange={(active) => updateDraft({ active })} />
      </>
    ),
  };

  const skillAreaDefinition: SectionDefinition<SkillAreaRow, SkillAreaDraft> = {
    key: 'skill-area',
    kind: 'skill-areas',
    label: 'Skill area',
    title: 'Skill area',
    subtitle: 'Aree competenza collegate a corsi, persone e certificazioni.',
    icon: 'git-branch',
    newLabel: 'Nuova skill area',
    singular: 'skill area',
    plural: 'skill area',
    searchPlaceholder: 'Cerca skill area',
    emptyLabel: 'Nessuna skill area trovata.',
    createdToast: 'Skill area creata',
    updatedToast: 'Skill area aggiornata',
    emptyDraft: () => ({ code: '', name: '', parentId: '', description: '', active: true }),
    itemToDraft: (item) => ({
      code: item.code,
      name: item.name,
      parentId: item.parentId ?? '',
      description: item.description ?? '',
      active: item.active,
    }),
    buildPayload: (draft, item) => ({
      code: codeFromNameOrExisting(draft.name, 'AREA', item?.code),
      name: draft.name.trim(),
      parentId: draft.parentId || undefined,
      description: emptyToUndefined(draft.description),
      active: draft.active,
    }),
    validate: (draft, item) => {
      if (!draft.name.trim()) return 'Nome obbligatorio.';
      if (item && draft.parentId === item.id) return "L'area padre deve essere diversa dalla skill area corrente.";
      return null;
    },
    getTitle: (item) => item.name,
    getCode: (item) => item.code,
    getMeta: (item) => [
      item.parentLabel ? `Area padre: ${item.parentLabel}` : 'Nessuna area padre',
      item.description ?? '',
    ].filter(Boolean),
    searchText: (item) => joinSearch(item.code, item.name, item.parentLabel, item.description),
    renderFields: ({ draft, item, prefix, updateDraft }) => {
      const options = skillAreaOptions
        .filter((option) => option.value !== item?.id && (option.active || option.value === draft.parentId))
        .map(({ value, label }) => ({ value, label }));
      return (
        <>
          {item ? (
            <div className={styles.gridTwo}>
              <ReadOnlyCodeField label="Codice" value={item.code} />
              <TextField id={`${prefix}-name`} label="Nome" value={draft.name} onChange={(name) => updateDraft({ name })} required />
            </div>
          ) : (
            <TextField id={`${prefix}-name`} label="Nome" value={draft.name} onChange={(name) => updateDraft({ name })} required />
          )}
          <SelectField
            id={`${prefix}-parent`}
            label="Area padre"
            options={options}
            selected={draft.parentId || null}
            onChange={(parentId) => updateDraft({ parentId: parentId ?? '' })}
            placeholder="Nessuna area padre"
            allowClear
          />
          <TextareaField
            id={`${prefix}-description`}
            label="Descrizione"
            value={draft.description}
            onChange={(description) => updateDraft({ description })}
          />
          <ActiveField id={`${prefix}-active`} active={draft.active} onChange={(active) => updateDraft({ active })} />
        </>
      );
    },
  };

  const vendorDefinition: SectionDefinition<VendorRow, VendorDraft> = {
    key: 'vendors',
    kind: 'vendors',
    label: 'Fornitori',
    title: 'Fornitori',
    subtitle: 'Soggetti che erogano o certificano percorsi formativi.',
    icon: 'package',
    newLabel: 'Nuovo fornitore',
    singular: 'fornitore',
    plural: 'fornitori',
    searchPlaceholder: 'Cerca fornitore',
    emptyLabel: 'Nessun fornitore trovato.',
    createdToast: 'Fornitore creato',
    updatedToast: 'Fornitore aggiornato',
    emptyDraft: () => ({ name: '', website: '', notes: '', active: true }),
    itemToDraft: (item) => ({
      name: item.name,
      website: item.website ?? '',
      notes: item.notes ?? '',
      active: item.active,
    }),
    buildPayload: (draft) => ({
      name: draft.name.trim(),
      website: emptyToUndefined(draft.website),
      notes: emptyToUndefined(draft.notes),
      active: draft.active,
    }),
    validate: (draft) => {
      if (!draft.name.trim()) return 'Nome obbligatorio.';
      return null;
    },
    getTitle: (item) => item.name,
    getMeta: (item) => [item.website ?? '', item.notes ?? ''].filter(Boolean),
    searchText: (item) => joinSearch(item.name, item.website, item.notes),
    renderFields: ({ draft, prefix, updateDraft }) => (
      <>
        <TextField id={`${prefix}-name`} label="Nome" value={draft.name} onChange={(name) => updateDraft({ name })} required />
        <TextField id={`${prefix}-website`} label="Sito" value={draft.website} onChange={(website) => updateDraft({ website })} type="url" />
        <TextareaField id={`${prefix}-notes`} label="Note" value={draft.notes} onChange={(notes) => updateDraft({ notes })} />
        <ActiveField id={`${prefix}-active`} active={draft.active} onChange={(active) => updateDraft({ active })} />
      </>
    ),
  };

  const certificationDefinition: SectionDefinition<CatalogCertificationRow, CertificationDraft> = {
    key: 'certifications',
    kind: 'certifications',
    label: 'Certificazioni',
    title: 'Certificazioni',
    subtitle: 'Qualifiche riconosciute collegate a skill area e fornitore.',
    icon: 'file-check',
    newLabel: 'Nuova certificazione',
    singular: 'certificazione',
    plural: 'certificazioni',
    searchPlaceholder: 'Cerca certificazione',
    emptyLabel: 'Nessuna certificazione trovata.',
    createdToast: 'Certificazione creata',
    updatedToast: 'Certificazione aggiornata',
    emptyDraft: () => ({
      code: '',
      name: '',
      issuerVendorId: '',
      skillAreaId: '',
      typicalValidityMonths: '',
      description: '',
      active: true,
    }),
    itemToDraft: (item) => ({
      code: item.code,
      name: item.name,
      issuerVendorId: item.issuerVendorId ?? '',
      skillAreaId: item.skillAreaId ?? '',
      typicalValidityMonths: item.typicalValidityMonths !== undefined ? String(item.typicalValidityMonths) : '',
      description: item.description ?? '',
      active: item.active,
    }),
    buildPayload: (draft, item) => ({
      code: codeFromNameOrExisting(draft.name, 'CERT', item?.code),
      name: draft.name.trim(),
      issuerVendorId: draft.issuerVendorId || undefined,
      skillAreaId: draft.skillAreaId || undefined,
      typicalValidityMonths: parseOptionalPositiveInteger(draft.typicalValidityMonths),
      description: emptyToUndefined(draft.description),
      active: draft.active,
    }),
    validate: (draft) => {
      if (!draft.name.trim()) return 'Nome obbligatorio.';
      if (draft.typicalValidityMonths.trim() && parseOptionalPositiveInteger(draft.typicalValidityMonths) === undefined) {
        return 'La validità deve essere un numero intero di mesi.';
      }
      return null;
    },
    getTitle: (item) => item.name,
    getCode: (item) => item.code,
    getMeta: (item) => [
      item.issuerVendorName ?? '',
      item.skillAreaLabel ?? '',
      item.typicalValidityMonths ? `${item.typicalValidityMonths} mesi` : '',
      item.description ?? '',
    ].filter(Boolean),
    searchText: (item) => joinSearch(
      item.code,
      item.name,
      item.issuerVendorName,
      item.skillAreaLabel,
      item.description,
    ),
    renderFields: ({ draft, item, prefix, updateDraft }) => {
      const activeVendors = vendorOptions
        .filter((option) => option.active || option.value === draft.issuerVendorId)
        .map(({ value, label }) => ({ value, label }));
      const activeSkillAreas = skillAreaOptions
        .filter((option) => option.active || option.value === draft.skillAreaId)
        .map(({ value, label }) => ({ value, label }));
      return (
        <>
          {item ? (
            <div className={styles.gridTwo}>
              <ReadOnlyCodeField label="Codice" value={item.code} />
              <TextField id={`${prefix}-name`} label="Nome" value={draft.name} onChange={(name) => updateDraft({ name })} required />
            </div>
          ) : (
            <TextField id={`${prefix}-name`} label="Nome" value={draft.name} onChange={(name) => updateDraft({ name })} required />
          )}
          <div className={styles.gridTwo}>
            <SelectField
              id={`${prefix}-issuer`}
              label="Ente certificatore"
              options={activeVendors}
              selected={draft.issuerVendorId || null}
              onChange={(issuerVendorId) => updateDraft({ issuerVendorId: issuerVendorId ?? '' })}
              placeholder="Nessun ente"
              allowClear
            />
            <SelectField
              id={`${prefix}-skill-area`}
              label="Skill area"
              options={activeSkillAreas}
              selected={draft.skillAreaId || null}
              onChange={(skillAreaId) => updateDraft({ skillAreaId: skillAreaId ?? '' })}
              placeholder="Nessuna skill area"
              allowClear
            />
          </div>
          <TextField
            id={`${prefix}-validity`}
            label="Validità tipica (mesi)"
            value={draft.typicalValidityMonths}
            onChange={(typicalValidityMonths) => updateDraft({ typicalValidityMonths })}
            type="number"
            min={1}
          />
          <TextareaField
            id={`${prefix}-description`}
            label="Descrizione"
            value={draft.description}
            onChange={(description) => updateDraft({ description })}
          />
          <ActiveField id={`${prefix}-active`} active={draft.active} onChange={(active) => updateDraft({ active })} />
        </>
      );
    },
  };

  function renderActiveSection() {
    switch (activeSection) {
      case 'team':
        return <MasterDataSection key="team" definition={teamDefinition} items={teams} {...commonProps} />;
      case 'skill-area':
        return <MasterDataSection key="skill-area" definition={skillAreaDefinition} items={skillAreas} {...commonProps} />;
      case 'vendors':
        return <MasterDataSection key="vendors" definition={vendorDefinition} items={vendors} {...commonProps} />;
      case 'certifications':
        return <MasterDataSection key="certifications" definition={certificationDefinition} items={certifications} {...commonProps} />;
    }
  }

  return (
    <main className={styles.page}>
      <header className={styles.header}>
        <div>
          <h1 className={styles.title}>Impostazioni</h1>
          <p className={styles.subtitle}>Anagrafiche formative per catalogo, pianificazione e compliance.</p>
        </div>
      </header>

      <div className={styles.sectionNav}>
        <TabNav items={sectionTabs} activeKey={activeSection} onTabChange={changeSection} />
      </div>

      {workspace.error ? (
        <StateBlock title="Impostazioni non disponibili" text={apiErrorMessage(workspace.error, 'Le anagrafiche non possono essere caricate.')} />
      ) : !workspace.isLoading && !masterData ? (
        <StateBlock title="Impostazioni non disponibili" text="Le anagrafiche sono riservate al team People." />
      ) : (
        renderActiveSection()
      )}
    </main>
  );
}

interface MasterDataSectionProps<T extends MasterItem, D extends BaseDraft> {
  definition: SectionDefinition<T, D>;
  items: T[];
  loading: boolean;
  error: unknown;
  search: string;
  status: StatusFilter;
  onSearchChange: (value: string) => void;
  onStatusChange: (value: StatusFilter) => void;
  onCreate: (kind: TrainingMasterDataKind, body: Record<string, unknown>) => Promise<void>;
  onUpdate: (kind: TrainingMasterDataKind, id: string, body: Record<string, unknown>) => Promise<void>;
  isSaving: boolean;
}

function MasterDataSection<T extends MasterItem, D extends BaseDraft>({
  definition,
  items,
  loading,
  error,
  search,
  status,
  onSearchChange,
  onStatusChange,
  onCreate,
  onUpdate,
  isSaving,
}: MasterDataSectionProps<T, D>) {
  const { toast } = useToast();
  const [drawer, setDrawer] = useState<DrawerState<T, D>>({ mode: 'closed' });
  const query = normalize(search);
  const visibleItems = [...items]
    .filter((item) => {
      if (status === 'attivo' && !item.active) return false;
      if (status === 'disattivato' && item.active) return false;
      if (!query) return true;
      return normalize(definition.searchText(item)).includes(query);
    })
    .sort((a, b) => definition.getTitle(a).localeCompare(definition.getTitle(b), 'it'));

  const draft = drawer.mode === 'closed' ? null : drawer.draft;
  const editingItem = drawer.mode === 'edit' ? drawer.item : null;
  const validationMessage = draft ? definition.validate(draft, editingItem) : null;
  const formId = `${definition.key}-settings-form`;

  function updateDraft(patch: Partial<D>) {
    setDrawer((current) => {
      if (current.mode === 'closed') return current;
      const nextDraft = { ...current.draft, ...patch };
      return current.mode === 'new'
        ? { mode: 'new', draft: nextDraft }
        : { mode: 'edit', item: current.item, draft: nextDraft };
    });
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (drawer.mode === 'closed') return;
    const message = definition.validate(drawer.draft, drawer.mode === 'edit' ? drawer.item : null);
    if (message) {
      toast(message, 'warning');
      return;
    }

    try {
      const item = drawer.mode === 'edit' ? drawer.item : null;
      const body = definition.buildPayload(drawer.draft, item);
      if (drawer.mode === 'new') {
        await onCreate(definition.kind, body);
        toast(definition.createdToast);
      } else {
        await onUpdate(definition.kind, drawer.item.id, body);
        toast(definition.updatedToast);
      }
      setDrawer({ mode: 'closed' });
    } catch (saveError) {
      toast(apiErrorMessage(saveError, 'Salvataggio non riuscito.'), 'error');
    }
  }

  return (
    <section className={styles.section} aria-label={definition.title}>
      <div className={styles.sectionHeader}>
        <div>
          <h2 className={styles.sectionTitle}>{definition.title}</h2>
          <p className={styles.sectionSubtitle}>
            {definition.subtitle} {countLabel(visibleItems.length, definition.singular, definition.plural)} visibili.
          </p>
        </div>
        <Button
          variant="primary"
          size="md"
          leftIcon={<Icon name="plus" size={16} />}
          onClick={() => setDrawer({ mode: 'new', draft: definition.emptyDraft() })}
        >
          {definition.newLabel}
        </Button>
      </div>

      <div className={styles.filters}>
        <div className={styles.searchWrap}>
          <SearchInput value={search} onChange={onSearchChange} placeholder={definition.searchPlaceholder} />
        </div>
        <SingleSelect
          options={STATUS_OPTIONS}
          selected={status}
          onChange={(value) => onStatusChange((value ?? '') as StatusFilter)}
          placeholder="Stato"
        />
      </div>

      <div className={styles.masterPanel}>
        {loading ? (
          <div className={styles.panelBody}>
            <Skeleton rows={8} />
          </div>
        ) : error ? (
          <StateBlock title="Anagrafiche non disponibili" text={apiErrorMessage(error, 'Elenco non disponibile.')} />
        ) : visibleItems.length === 0 ? (
          <StateBlock title={definition.emptyLabel} text="Modifica i filtri o crea una nuova voce." />
        ) : (
          <>
            <div className={styles.tableHeader}>
              <span>Voce</span>
              <span>Stato</span>
            </div>
            <div className={styles.list} role="list">
              {visibleItems.map((item) => (
                <button
                  key={item.id}
                  type="button"
                  className={styles.row}
                  onClick={() => setDrawer({ mode: 'edit', item, draft: definition.itemToDraft(item) })}
                  aria-label={`Modifica ${definition.getTitle(item)}`}
                >
                  <span className={styles.rowIcon}>
                    <Icon name={definition.icon} size={17} />
                  </span>
                  <span className={styles.rowMain}>
                    <span className={styles.rowTitleLine}>
                      <span className={styles.rowTitle}>{definition.getTitle(item)}</span>
                      {definition.getCode?.(item) ? <span className={styles.codeBadge}>{definition.getCode?.(item)}</span> : null}
                    </span>
                    <span className={styles.rowMeta}>{definition.getMeta(item).slice(0, 3).join(' · ') || 'Nessuna nota'}</span>
                  </span>
                  <span className={`${styles.statusBadge} ${item.active ? styles.statusActive : styles.statusInactive}`}>
                    {item.active ? 'Attivo' : 'Disattivato'}
                  </span>
                  <Icon name="chevron-right" size={16} className={styles.chevron} />
                </button>
              ))}
            </div>
          </>
        )}
      </div>

      <Drawer
        open={drawer.mode !== 'closed'}
        onClose={() => setDrawer({ mode: 'closed' })}
        size="lg"
        title={drawer.mode === 'new' ? definition.newLabel : drawer.mode === 'edit' ? `Modifica ${definition.singular}` : ''}
        subtitle={drawer.mode === 'edit' ? definition.getTitle(drawer.item) : undefined}
        footer={
          drawer.mode !== 'closed' && (
            <div className={styles.drawerFooter}>
              <Button variant="ghost" size="md" onClick={() => setDrawer({ mode: 'closed' })}>
                Annulla
              </Button>
              <Button
                type="submit"
                form={formId}
                variant="primary"
                size="md"
                loading={isSaving}
                disabled={Boolean(validationMessage)}
              >
                Salva
              </Button>
            </div>
          )
        }
      >
        {drawer.mode !== 'closed' && (
          <form id={formId} className={styles.drawerForm} onSubmit={handleSubmit}>
            {definition.renderFields({
              draft: drawer.draft,
              item: drawer.mode === 'edit' ? drawer.item : null,
              prefix: `${definition.key}-${drawer.mode}`,
              updateDraft,
            })}
          </form>
        )}
      </Drawer>
    </section>
  );
}

function TextField({
  id,
  label,
  value,
  onChange,
  required,
  type = 'text',
  min,
}: {
  id: string;
  label: string;
  value: string;
  onChange: (value: string) => void;
  required?: boolean;
  type?: 'text' | 'url' | 'number';
  min?: number;
}) {
  return (
    <label className={styles.field} htmlFor={id}>
      <span className={styles.label}>
        {label}
        {required ? <span className={styles.required} aria-hidden="true" /> : null}
      </span>
      <input
        id={id}
        className={styles.input}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        required={required}
        type={type}
        min={min}
      />
    </label>
  );
}

function TextareaField({
  id,
  label,
  value,
  onChange,
}: {
  id: string;
  label: string;
  value: string;
  onChange: (value: string) => void;
}) {
  return (
    <label className={styles.field} htmlFor={id}>
      <span className={styles.label}>{label}</span>
      <textarea
        id={id}
        className={styles.textarea}
        rows={3}
        value={value}
        onChange={(event) => onChange(event.target.value)}
      />
    </label>
  );
}

function ReadOnlyCodeField({ label, value }: { label: string; value: string }) {
  return (
    <div className={styles.field}>
      <span className={styles.label}>{label}</span>
      <span className={styles.readOnlyCodeBadge}>{value}</span>
    </div>
  );
}

function SelectField({
  id,
  label,
  options,
  selected,
  onChange,
  placeholder,
  allowClear,
}: {
  id: string;
  label: string;
  options: Array<{ value: string; label: string }>;
  selected: string | null;
  onChange: (value: string | null) => void;
  placeholder: string;
  allowClear?: boolean;
}) {
  return (
    <div className={styles.field}>
      <span id={`${id}-label`} className={styles.label}>{label}</span>
      <SingleSelect
        options={options}
        selected={selected}
        onChange={onChange}
        placeholder={placeholder}
        allowClear={allowClear}
      />
    </div>
  );
}

function ActiveField({
  id,
  active,
  onChange,
}: {
  id: string;
  active: boolean;
  onChange: (active: boolean) => void;
}) {
  return (
    <div className={styles.activeField}>
      <span className={styles.label}>Stato</span>
      <ToggleSwitch id={id} checked={active} onChange={onChange} label={active ? 'Attivo' : 'Disattivato'} />
    </div>
  );
}

function StateBlock({ title, text }: { title: string; text: string }) {
  return (
    <div className={styles.stateBlock}>
      <Icon name="info" size={18} />
      <div>
        <p className={styles.stateTitle}>{title}</p>
        <p className={styles.stateText}>{text}</p>
      </div>
    </div>
  );
}

function toSectionKey(value: string | null): SectionKey | null {
  switch (value) {
    case 'team':
    case 'skill-area':
    case 'vendors':
    case 'certifications':
      return value;
    default:
      return null;
  }
}

function countLabel(count: number, singular: string, plural: string): string {
  return `${count} ${count === 1 ? singular : plural}`;
}

function labelWithCode(code: string, name: string): string {
  return `${code} - ${name}`;
}

function joinSearch(...parts: Array<string | undefined>): string {
  return parts.filter(Boolean).join(' ');
}

function normalize(value: string): string {
  return value.trim().toLowerCase();
}

function emptyToUndefined(value: string): string | undefined {
  const trimmed = value.trim();
  return trimmed || undefined;
}

function codeFromNameOrExisting(name: string, fallback: string, existingCode?: string): string {
  return existingCode ?? importCodeFromName(name, fallback);
}

function importCodeFromName(name: string, fallback: string): string {
  const cleanName = cleanImportLabel(name);
  if (!cleanName) return fallback;
  const parts: string[] = [];
  let lastUnderscore = false;

  for (const character of cleanName.toUpperCase()) {
    if (/[\p{L}\p{N}]/u.test(character)) {
      parts.push(character);
      lastUnderscore = false;
      continue;
    }
    if (!lastUnderscore) {
      parts.push('_');
      lastUnderscore = true;
    }
  }

  const code = parts.join('').replace(/^_+|_+$/g, '');
  return code || fallback;
}

function cleanImportLabel(value: string): string {
  const cleaned = value.trim().split(/\s+/).filter(Boolean).join(' ');
  return cleaned === '/' ? '' : cleaned;
}

function parseOptionalPositiveInteger(value: string): number | undefined {
  const trimmed = value.trim();
  if (!trimmed) return undefined;
  const parsed = Number(trimmed);
  if (!Number.isInteger(parsed) || parsed <= 0) return undefined;
  return parsed;
}

function apiErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof ApiError) {
    const body = error.body;
    if (body && typeof body === 'object' && 'message' in body && typeof body.message === 'string') {
      return body.message;
    }
  }
  return error instanceof Error ? error.message : fallback;
}
