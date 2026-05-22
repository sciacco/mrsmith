import { useState } from 'react';
import { Link } from 'react-router-dom';
import { ApiError } from '@mrsmith/api-client';
import { Button, Drawer, SearchInput, SingleSelect, Skeleton, useToast } from '@mrsmith/ui';
import {
  useCreateCustomGroup,
  useCustomGroups,
  useDeleteCustomGroup,
  useTrainingLookups,
  useUpdateCustomGroup,
} from '../api/queries';
import type { CustomGroup, CustomGroupInput } from '../api/types';
import { MemberMultiSelect } from '../components/MemberMultiSelect';
import styles from './GroupsPage.module.css';

type StatusFilter = '' | 'attivo' | 'disattivato';
type DrawerState = { mode: 'closed' } | { mode: 'new'; draft: CustomGroupInput } | { mode: 'edit'; group: CustomGroup; draft: CustomGroupInput };

const STATUS_OPTIONS = [
  { value: '', label: 'Tutti gli stati' },
  { value: 'attivo', label: 'Attivi' },
  { value: 'disattivato', label: 'Disattivati' },
];

function emptyDraft(): CustomGroupInput {
  return { name: '', description: '', active: true, member_ids: [] };
}

function groupToDraft(group: CustomGroup): CustomGroupInput {
  return {
    name: group.name,
    description: group.description ?? '',
    active: group.active,
    member_ids: (group.members ?? []).map((member) => member.id),
  };
}

function errorMessage(error: unknown, fallback: string): string {
  if (error instanceof ApiError) {
    const body = error.body as { message?: string } | undefined;
    return body?.message ?? fallback;
  }
  return error instanceof Error ? error.message : fallback;
}

export function GroupsPage({ isPeopleAdmin }: { isPeopleAdmin: boolean }) {
  const { toast } = useToast();
  const [status, setStatus] = useState<StatusFilter>('attivo');
  const [search, setSearch] = useState('');
  const [memberSearch, setMemberSearch] = useState('');
  const [drawer, setDrawer] = useState<DrawerState>({ mode: 'closed' });

  const groups = useCustomGroups({ status, q: search.trim() || undefined }, isPeopleAdmin);
  const lookups = useTrainingLookups(isPeopleAdmin);
  const createGroup = useCreateCustomGroup();
  const updateGroup = useUpdateCustomGroup();
  const deleteGroup = useDeleteCustomGroup();

  const list = groups.data?.groups ?? [];
  const drawerDraft = drawer.mode === 'closed' ? null : drawer.draft;
  const canSubmit = Boolean(drawerDraft?.name.trim());

  if (!isPeopleAdmin) {
    return <main className={styles.page}><p className={styles.muted}>Accesso riservato al team People.</p></main>;
  }

  function updateDraft(patch: Partial<CustomGroupInput>) {
    if (drawer.mode === 'closed') return;
    const draft = { ...drawer.draft, ...patch };
    setDrawer(drawer.mode === 'new' ? { mode: 'new', draft } : { ...drawer, draft });
  }

  function saveGroup() {
    if (!drawerDraft || !canSubmit) return;
    const body = {
      ...drawerDraft,
      name: drawerDraft.name.trim(),
      description: drawerDraft.description?.trim() || undefined,
    };
    if (drawer.mode === 'new') {
      createGroup.mutate(body, {
        onSuccess: () => {
          toast('Gruppo creato');
          setDrawer({ mode: 'closed' });
        },
        onError: (error) => toast(errorMessage(error, 'Gruppo non salvato'), 'error'),
      });
      return;
    }
    if (drawer.mode === 'edit') {
      updateGroup.mutate(
        { id: drawer.group.id, body },
        {
          onSuccess: () => {
            toast('Gruppo aggiornato');
            setDrawer({ mode: 'closed' });
          },
          onError: (error) => toast(errorMessage(error, 'Gruppo non aggiornato'), 'error'),
        },
      );
    }
  }

  function removeGroup(group: CustomGroup) {
    if (!window.confirm(`Eliminare ${group.name}?`)) return;
    deleteGroup.mutate(group.id, {
      onSuccess: () => {
        toast('Gruppo eliminato');
        setDrawer({ mode: 'closed' });
      },
      onError: (error) => toast(errorMessage(error, 'Gruppo non eliminato'), 'error'),
    });
  }

  return (
    <main className={styles.page}>
      <header className={styles.header}>
        <div>
          <h1 className={styles.title}>Gruppi formativi</h1>
          <p className={styles.subtitle}>
            {list.length} gruppi · {list.reduce((sum, group) => sum + group.member_count, 0)} appartenenze
          </p>
        </div>
        <div className={styles.headerActions}>
          <Link to="/persone" className={styles.backLink}>Directory</Link>
          <Button
            variant="primary"
            size="md"
            onClick={() => {
              setMemberSearch('');
              setDrawer({ mode: 'new', draft: emptyDraft() });
            }}
          >
            + Nuovo gruppo
          </Button>
        </div>
      </header>

      <div className={styles.filters}>
        <div className={styles.searchWrap}>
          <SearchInput value={search} onChange={setSearch} placeholder="Cerca gruppo" />
        </div>
        <SingleSelect
          options={STATUS_OPTIONS}
          selected={status}
          onChange={(value) => setStatus((value ?? '') as StatusFilter)}
          placeholder="Stato"
        />
      </div>

      {groups.isLoading ? (
        <Skeleton rows={6} />
      ) : list.length === 0 ? (
        <div className={styles.empty}>Nessun gruppo formativo. I gruppi servono per popolazioni ad-hoc.</div>
      ) : (
        <div className={styles.grid}>
          {list.map((group) => (
            <button
              key={group.id}
              type="button"
              className={`${styles.card} ${!group.active ? styles.disabled : ''}`}
              onClick={() => {
                setMemberSearch('');
                setDrawer({ mode: 'edit', group, draft: groupToDraft(group) });
              }}
            >
              <span className={styles.cardTitle}>{group.name}</span>
              <span className={styles.cardMeta}>{group.description || 'Popolazione ad-hoc'}</span>
              <span className={styles.cardFooter}>
                <span>{group.member_count} persone</span>
                <span>Usato in {group.used_by?.length ?? 0}</span>
              </span>
            </button>
          ))}
        </div>
      )}

      <Drawer
        open={drawer.mode !== 'closed'}
        onClose={() => setDrawer({ mode: 'closed' })}
        size="lg"
        title={drawer.mode === 'new' ? 'Nuovo gruppo' : drawer.mode === 'edit' ? drawer.group.name : ''}
        subtitle={drawer.mode === 'edit' ? `${drawer.group.member_count} persone` : undefined}
        footer={
          drawer.mode !== 'closed' && (
            <div className={styles.drawerFooter}>
              <div className={styles.drawerLeft}>
                {drawer.mode === 'edit' && (drawer.group.used_by?.length ?? 0) > 0 && (
                  <span>Usato in {drawer.group.used_by?.length}</span>
                )}
              </div>
              <div className={styles.drawerActions}>
                {drawer.mode === 'edit' && (
                  <Button
                    variant="danger"
                    size="md"
                    onClick={() => removeGroup(drawer.group)}
                    loading={deleteGroup.isPending}
                  >
                    Elimina
                  </Button>
                )}
                <Button variant="ghost" size="md" onClick={() => setDrawer({ mode: 'closed' })}>Annulla</Button>
                <Button
                  variant="primary"
                  size="md"
                  onClick={saveGroup}
                  loading={createGroup.isPending || updateGroup.isPending}
                  disabled={!canSubmit}
                >
                  Salva
                </Button>
              </div>
            </div>
          )
        }
      >
        {drawer.mode !== 'closed' && (
          <div className={styles.drawerBody}>
            <label className={styles.field}>
              <span>Nome gruppo</span>
              <input
                value={drawer.draft.name}
                onChange={(event) => updateDraft({ name: event.target.value })}
                placeholder="Es. Cybersecurity 2026"
              />
            </label>
            <label className={styles.field}>
              <span>Descrizione</span>
              <textarea
                rows={3}
                value={drawer.draft.description ?? ''}
                onChange={(event) => updateDraft({ description: event.target.value })}
                placeholder="Ambito o criterio operativo"
              />
            </label>
            <label className={styles.toggle}>
              <input
                type="checkbox"
                checked={drawer.draft.active ?? true}
                onChange={(event) => updateDraft({ active: event.target.checked })}
              />
              <span>Gruppo attivo</span>
            </label>
            <div className={styles.field}>
              <span>Persone</span>
              <MemberMultiSelect
                people={lookups.data?.employees ?? []}
                selectedIds={drawer.draft.member_ids}
                query={memberSearch}
                onQueryChange={setMemberSearch}
                onChange={(member_ids) => updateDraft({ member_ids })}
              />
            </div>
            {drawer.mode === 'edit' && (drawer.group.used_by?.length ?? 0) > 0 && (
              <div className={styles.usedBy}>
                <h3>Usato in</h3>
                <ul>
                  {drawer.group.used_by?.map((usage, index) => (
                    <li key={`${usage.kind}-${usage.id ?? index}`}>
                      {usage.label}{usage.count ? ` · ${usage.count}` : ''}
                    </li>
                  ))}
                </ul>
              </div>
            )}
          </div>
        )}
      </Drawer>
    </main>
  );
}
