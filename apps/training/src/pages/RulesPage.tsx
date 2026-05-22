import { useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { ApiError } from '@mrsmith/api-client';
import { Button, Drawer, SearchInput, SingleSelect, Skeleton, useToast } from '@mrsmith/ui';
import {
  useCreateMandatoryRule,
  useCustomGroups,
  useDeleteMandatoryRule,
  useMandatoryRules,
  useTrainingLookups,
  useUpdateMandatoryRule,
} from '../api/queries';
import type { MandatoryRule, MandatoryRuleInput, PopulationKind } from '../api/types';
import { RuleForm } from '../components/RuleForm';
import styles from './RulesPage.module.css';

type StatusFilter = '' | 'attiva' | 'disattivata';
type DrawerState = { mode: 'closed' } | { mode: 'new'; draft: MandatoryRuleInput } | { mode: 'edit'; rule: MandatoryRule; draft: MandatoryRuleInput };

const STATUS_OPTIONS = [
  { value: '', label: 'Tutti gli stati' },
  { value: 'attiva', label: 'Attive' },
  { value: 'disattivata', label: 'Disattivate' },
];

const POPULATION_OPTIONS: Array<{ value: '' | PopulationKind; label: string }> = [
  { value: '', label: 'Tutte le popolazioni' },
  { value: 'all', label: 'Tutte' },
  { value: 'team', label: 'Team' },
  { value: 'skill_area', label: 'Skill area' },
  { value: 'custom_group', label: 'Gruppo' },
];

function emptyDraft(): MandatoryRuleInput {
  return {
    name: '',
    course_id: '',
    population_target: { kind: 'all' },
    active: true,
    notes: '',
  };
}

function ruleToDraft(rule: MandatoryRule): MandatoryRuleInput {
  return {
    name: rule.name,
    course_id: rule.course_id,
    population_target: {
      kind: rule.population_target.kind,
      id: rule.population_target.id,
    },
    active: rule.active,
    notes: rule.notes ?? '',
  };
}

function errorMessage(error: unknown, fallback: string): string {
  if (error instanceof ApiError) {
    const body = error.body as { message?: string } | undefined;
    return body?.message ?? fallback;
  }
  return error instanceof Error ? error.message : fallback;
}

function coverage(rule: MandatoryRule): string {
  return `${Math.round(rule.coverage_pct)}%`;
}

function populationLabel(rule: MandatoryRule): string {
  return rule.population_target.label || (rule.population_target.kind === 'all' ? 'Tutte le persone' : 'Popolazione');
}

export function RulesPage({ isPeopleAdmin }: { isPeopleAdmin: boolean }) {
  const { toast } = useToast();
  const [status, setStatus] = useState<StatusFilter>('attiva');
  const [populationKind, setPopulationKind] = useState<'' | PopulationKind>('');
  const [search, setSearch] = useState('');
  const [drawer, setDrawer] = useState<DrawerState>({ mode: 'closed' });

  const rules = useMandatoryRules({ status, populationKind, q: search.trim() || undefined }, isPeopleAdmin);
  const lookups = useTrainingLookups(isPeopleAdmin);
  const groups = useCustomGroups({ status: 'attivo' }, isPeopleAdmin);
  const createRule = useCreateMandatoryRule();
  const updateRule = useUpdateMandatoryRule();
  const deleteRule = useDeleteMandatoryRule();

  const list = rules.data?.rules ?? [];
  const activeCount = list.filter((rule) => rule.active).length;
  const gapCount = list.reduce((sum, rule) => sum + rule.gap_count, 0);

  const drawerDraft = drawer.mode === 'closed' ? null : drawer.draft;
  const canSubmit = Boolean(drawerDraft?.name.trim() && drawerDraft.course_id && drawerDraft.population_target.kind);

  const titleByCourse = useMemo(() => {
    const map = new Map<string, string>();
    (lookups.data?.courses ?? []).forEach((course) => map.set(course.id, course.label));
    return map;
  }, [lookups.data]);

  if (!isPeopleAdmin) {
    return <main className={styles.page}><p className={styles.muted}>Accesso riservato al team People.</p></main>;
  }

  function saveRule() {
    if (!drawerDraft || !canSubmit) return;
    const body = {
      ...drawerDraft,
      name: drawerDraft.name.trim(),
      notes: drawerDraft.notes?.trim() || undefined,
    };
    if (drawer.mode === 'new') {
      createRule.mutate(body, {
        onSuccess: (response) => {
          toast(response.warnings?.includes('coverage_gap') ? 'Regola salvata con copertura da completare' : 'Regola creata');
          setDrawer({ mode: 'closed' });
        },
        onError: (error) => toast(errorMessage(error, 'Regola non salvata'), 'error'),
      });
      return;
    }
    if (drawer.mode === 'edit') {
      updateRule.mutate(
        { id: drawer.rule.id, body },
        {
          onSuccess: (response) => {
            toast(response.warnings?.includes('coverage_gap') ? 'Regola aggiornata con copertura da completare' : 'Regola aggiornata');
            setDrawer({ mode: 'closed' });
          },
          onError: (error) => toast(errorMessage(error, 'Regola non aggiornata'), 'error'),
        },
      );
    }
  }

  function removeRule(rule: MandatoryRule) {
    if (!window.confirm(`Eliminare ${rule.name}?`)) return;
    deleteRule.mutate(rule.id, {
      onSuccess: () => {
        toast('Regola eliminata');
        setDrawer({ mode: 'closed' });
      },
      onError: (error) => toast(errorMessage(error, 'Regola non eliminata'), 'error'),
    });
  }

  return (
    <main className={styles.page}>
      <header className={styles.header}>
        <div>
          <h1 className={styles.title}>Regole</h1>
          <p className={styles.subtitle}>
            {activeCount} attive · {gapCount} persone interessate da coprire
          </p>
        </div>
        <div className={styles.headerActions}>
          <Link to="/compliance" className={styles.backLink}>Copertura</Link>
          <Button variant="primary" size="md" onClick={() => setDrawer({ mode: 'new', draft: emptyDraft() })}>
            + Nuova regola
          </Button>
        </div>
      </header>

      <div className={styles.filters}>
        <div className={styles.searchWrap}>
          <SearchInput value={search} onChange={setSearch} placeholder="Cerca regola o corso" />
        </div>
        <SingleSelect
          options={STATUS_OPTIONS}
          selected={status}
          onChange={(value) => setStatus((value ?? '') as StatusFilter)}
          placeholder="Stato"
        />
        <SingleSelect
          options={POPULATION_OPTIONS}
          selected={populationKind}
          onChange={(value) => setPopulationKind((value ?? '') as '' | PopulationKind)}
          placeholder="Popolazione"
        />
      </div>

      {rules.isLoading ? (
        <Skeleton rows={6} />
      ) : list.length === 0 ? (
        <div className={styles.empty}>
          {status === 'attiva'
            ? 'Nessuna regola attiva. Crea la prima per iniziare.'
            : 'Nessuna regola trovata.'}
        </div>
      ) : (
        <div className={styles.list}>
          {list.map((rule) => (
            <button
              key={rule.id}
              type="button"
              className={`${styles.ruleRow} ${styles[`severity_${rule.severity}`]} ${!rule.active ? styles.disabled : ''}`}
              onClick={() => setDrawer({ mode: 'edit', rule, draft: ruleToDraft(rule) })}
            >
              <span className={styles.ruleMain}>
                <span className={styles.ruleTitle}>{rule.name}</span>
                <span className={styles.ruleMeta}>
                  {titleByCourse.get(rule.course_id) ?? rule.course_title} · {populationLabel(rule)}
                </span>
              </span>
              <span className={styles.ruleNumbers}>
                <span>{coverage(rule)}</span>
                <small>{rule.covered_count}/{rule.target_count}</small>
              </span>
              <span className={styles.gapBadge}>{rule.gap_count} gap</span>
            </button>
          ))}
        </div>
      )}

      <Drawer
        open={drawer.mode !== 'closed'}
        onClose={() => setDrawer({ mode: 'closed' })}
        size="lg"
        title={drawer.mode === 'new' ? 'Nuova regola' : drawer.mode === 'edit' ? drawer.rule.name : ''}
        subtitle={drawer.mode === 'edit' ? `${coverage(drawer.rule)} copertura · ${drawer.rule.gap_count} gap` : undefined}
        footer={
          drawer.mode !== 'closed' && (
            <div className={styles.drawerFooter}>
              <div className={styles.drawerLeft}>
                {drawer.mode === 'edit' && (drawer.rule.used_by?.length ?? 0) > 0 && (
                  <span>Usato in {drawer.rule.used_by?.reduce((sum, item) => sum + (item.count ?? 1), 0)}</span>
                )}
              </div>
              <div className={styles.drawerActions}>
                {drawer.mode === 'edit' && (
                  <Button
                    variant="danger"
                    size="md"
                    onClick={() => removeRule(drawer.rule)}
                    loading={deleteRule.isPending}
                  >
                    Elimina
                  </Button>
                )}
                <Button variant="ghost" size="md" onClick={() => setDrawer({ mode: 'closed' })}>Annulla</Button>
                <Button
                  variant="primary"
                  size="md"
                  onClick={saveRule}
                  loading={createRule.isPending || updateRule.isPending}
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
            <RuleForm
              value={drawer.draft}
              courses={lookups.data?.courses ?? []}
              teams={lookups.data?.teams ?? []}
              skillAreas={lookups.data?.skillAreas ?? []}
              groups={groups.data?.groups ?? []}
              onChange={(draft) => setDrawer(drawer.mode === 'new' ? { mode: 'new', draft } : { ...drawer, draft })}
            />
            {drawer.mode === 'edit' && (
              <div className={styles.impact}>
                <h3>Persone interessate</h3>
                <p>{drawer.rule.target_count} in popolazione · {drawer.rule.gap_count} da coprire</p>
                {drawer.rule.gaps && drawer.rule.gaps.length > 0 && (
                  <ul>
                    {drawer.rule.gaps.map((gap) => (
                      <li key={gap.employee_id}>{gap.employee_name}</li>
                    ))}
                  </ul>
                )}
              </div>
            )}
          </div>
        )}
      </Drawer>
    </main>
  );
}
