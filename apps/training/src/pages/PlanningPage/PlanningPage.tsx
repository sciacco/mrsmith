import { useEffect, useRef, useState } from "react";
import type { MouseEvent } from "react";
import { Link, useSearchParams } from "react-router-dom";
import {
  Button,
  Icon,
  SearchInput,
  SingleSelect,
  Skeleton,
  VisuallyHidden,
} from "@mrsmith/ui";
import {
  usePlanning,
  usePlanningCourseDetail,
  usePlanningFilters,
} from "../../api/queries";
import type {
  CourseSummary,
  PlanningItemKind,
  PlanningItemsParams,
  PlanningListParams,
  PlanningReminder,
  PlanningView,
} from "../../api/types";
import {
  deliverySummary,
  eventConditions,
  eventState,
  localDate,
  n,
  reminderDate,
  reminderMonthDay,
  requestState,
} from "./planningFormat";
import { PlanningCourseDrawer } from "./PlanningCourseDrawer";
import { PlanningReminderModal } from "./PlanningReminderModal";
import styles from "./PlanningPage.module.css";

const views: Array<[PlanningView, string]> = [
  ["operative", "Operativa"],
  ["reminders", "In scadenza"],
  ["suspended", "Sospese"],
  ["history", "Storico"],
];
const itemKinds: PlanningItemKind[] = [
  "requests",
  "enrollments",
  "events",
  "reminders",
];
const defaults: PlanningListParams = {
  view: "operative",
  q: "",
  tag: "",
  employeeId: "",
  teamId: "",
  skillAreaId: "",
  limit: 25,
  offset: 0,
};

function isView(value: string | null): value is PlanningView {
  return views.some(([view]) => view === value);
}
function isItemKind(value: string | null): value is PlanningItemKind {
  return itemKinds.some((kind) => kind === value);
}
function numberParam(value: string | null) {
  const parsed = Number(value);
  return Number.isInteger(parsed) && parsed > 0 ? parsed : 0;
}

function parseList(params: URLSearchParams): PlanningListParams {
  const view = params.get("view");
  return {
    ...defaults,
    view: isView(view) ? view : "operative",
    q: params.get("q") ?? "",
    tag: params.get("tag") ?? "",
    employeeId: params.get("employeeId") ?? "",
    teamId: params.get("teamId") ?? "",
    skillAreaId: params.get("skillAreaId") ?? "",
    limit: params.get("limit") === "50" ? 50 : 25,
    offset: numberParam(params.get("offset")),
  };
}

function parseDrawer(params: URLSearchParams): PlanningItemsParams {
  const kind = params.get("section");
  return {
    kind: isItemKind(kind) ? kind : "enrollments",
    q: params.get("itemQ") ?? "",
    teamId: params.get("itemTeam") ?? "",
    eventId: params.get("itemEvent") ?? "",
    status: params.get("itemStatus") ?? "",
    limit: 25,
    offset: numberParam(params.get("itemOffset")),
  };
}

function normalizedUrl(params: URLSearchParams) {
  const sorted = [...params.entries()].sort(([left], [right]) =>
    left.localeCompare(right),
  );
  const query = new URLSearchParams(sorted).toString();
  return `${window.location.pathname}${query ? `?${query}` : ""}`;
}

export function PlanningPage() {
  const [params, setParams] = useSearchParams();
  const filters = parseList(params);
  const itemsParams = parseDrawer(params);
  const list = usePlanning(filters);
  const filterOptions = usePlanningFilters();
  const expanded = params.get("expanded");
  const drawer = params.get("drawer");
  const expandedDetail = usePlanningCourseDetail(expanded ?? undefined);
  const headingRef = useRef<HTMLHeadingElement>(null);
  const reminderTrigger = useRef<HTMLElement | null>(null);
  const restoredKey = useRef<string | null>(null);
  const [search, setSearch] = useState(filters.q);
  const [showFilters, setShowFilters] = useState(
    Boolean(filters.employeeId || filters.teamId || filters.skillAreaId),
  );
  const [reminder, setReminder] = useState<PlanningReminder | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  useEffect(() => setSearch(filters.q), [filters.q]);
  useEffect(() => {
    const timer = window.setTimeout(() => {
      if (search.trim() !== filters.q)
        updateList({ q: search.trim(), offset: 0 }, true);
    }, 250);
    return () => window.clearTimeout(timer);
  }, [search, filters.q]);

  useEffect(() => {
    const key = `training-planning-scroll:${normalizedUrl(params)}`;
    if (list.isLoading || !list.data || restoredKey.current === key) return;
    const saved = sessionStorage.getItem(key);
    if (saved) {
      const context: unknown = JSON.parse(saved);
      if (isScrollContext(context) && context.url === normalizedUrl(params))
        requestAnimationFrame(() => window.scrollTo(0, context.position));
      sessionStorage.removeItem(key);
    }
    restoredKey.current = key;
  }, [list.data, list.isLoading, params]);

  useEffect(() => {
    if (
      !expanded ||
      !list.data ||
      list.data.items.some((course) => course.id === expanded)
    )
      return;
    const next = new URLSearchParams(params);
    next.delete("expanded");
    setParams(next, { replace: true });
    setNotice(
      "Il corso non rientra più in questa vista. Il drawer resta consultabile fino alla chiusura.",
    );
  }, [expanded, list.data, params, setParams]);

  useEffect(() => {
    if (!list.data || filters.offset < list.data.total) return;
    updateList(
      {
        offset: Math.max(
          0,
          Math.floor((list.data.total - 1) / filters.limit) * filters.limit,
        ),
      },
      true,
    );
  }, [filters.limit, filters.offset, list.data?.total]);

  function updateList(next: Partial<PlanningListParams>, replace = false) {
    const merged = { ...filters, ...next };
    const query = new URLSearchParams();
    Object.entries(merged).forEach(([key, value]) => {
      if (
        value !== "" &&
        value !== 0 &&
        !(key === "view" && value === "operative") &&
        !(key === "limit" && value === 25)
      )
        query.set(key, String(value));
    });
    [
      "expanded",
      "drawer",
      "section",
      "itemQ",
      "itemTeam",
      "itemEvent",
      "itemStatus",
      "itemOffset",
    ].forEach((key) => {
      const value = params.get(key);
      if (value) query.set(key, value);
    });
    setParams(query, { replace });
  }

  function updateDrawer(next: Partial<PlanningItemsParams>, reset = false) {
    const query = new URLSearchParams(params);
    const merged: PlanningItemsParams = reset
      ? {
          kind: itemsParams.kind,
          q: "",
          teamId: "",
          eventId: "",
          status: "",
          limit: 25,
          offset: 0,
          ...next,
        }
      : { ...itemsParams, ...next };
    if (reset)
      ["itemQ", "itemTeam", "itemEvent", "itemStatus", "itemOffset"].forEach(
        (key) => query.delete(key),
      );
    query.set("section", merged.kind);
    const entries: Array<
      [
        keyof Pick<
          PlanningItemsParams,
          "q" | "teamId" | "eventId" | "status" | "offset"
        >,
        string,
      ]
    > = [
      ["q", "itemQ"],
      ["teamId", "itemTeam"],
      ["eventId", "itemEvent"],
      ["status", "itemStatus"],
      ["offset", "itemOffset"],
    ];
    entries.forEach(([key, urlKey]) =>
      merged[key]
        ? query.set(urlKey, String(merged[key]))
        : query.delete(urlKey),
    );
    setParams(query, { replace: true });
  }

  function openDrawer(id: string, section: PlanningItemKind = "enrollments") {
    const query = new URLSearchParams(params);
    query.set("drawer", id);
    query.set("section", section);
    ["itemQ", "itemTeam", "itemEvent", "itemStatus", "itemOffset"].forEach(
      (key) => query.delete(key),
    );
    setParams(query);
  }

  function closeDrawer() {
    const query = new URLSearchParams(params);
    [
      "drawer",
      "section",
      "itemQ",
      "itemTeam",
      "itemEvent",
      "itemStatus",
      "itemOffset",
    ].forEach((key) => query.delete(key));
    setParams(query);
  }

  function saveExternalContext(event: MouseEvent<HTMLAnchorElement>) {
    const anchor = event.currentTarget;
    if (anchor.pathname === window.location.pathname) return;
    const key = `training-planning-scroll:${normalizedUrl(params)}`;
    sessionStorage.setItem(
      key,
      JSON.stringify({ url: normalizedUrl(params), position: window.scrollY }),
    );
  }

  function editReminder(value: PlanningReminder, trigger: HTMLElement) {
    reminderTrigger.current = trigger;
    setReminder(value);
  }

  function reminderCompleted(message: string) {
    setNotice(message);
    requestAnimationFrame(() => {
      if (reminderTrigger.current?.isConnected) reminderTrigger.current.focus();
      else headingRef.current?.focus();
    });
  }

  const active = Boolean(
    filters.q ||
      filters.tag ||
      filters.employeeId ||
      filters.teamId ||
      filters.skillAreaId,
  );
  const total = list.data?.total ?? 0;
  const end = Math.min(filters.offset + (list.data?.items.length ?? 0), total);

  return (
    <div className={styles.page}>
      <header className={styles.header}>
        <div>
          <h1 ref={headingRef} tabIndex={-1}>
            Pianificazione
          </h1>
          <p>
            {filters.view === "operative"
              ? "Attività formative da seguire per corso."
              : filters.view === "reminders"
                ? "Corsi con un richiamo di promemoria odierno o scaduto."
                : `Corsi nella vista ${views.find(([view]) => view === filters.view)?.[1].toLowerCase()}.`}
          </p>
        </div>
        <div className={styles.headerActions}>
          <span>Oggi {localDate(list.data?.today)}</span>
          {(list.data?.dueCoursesTotal ?? 0) > 0 && (
            <Button
              size="sm"
              variant="secondary"
              onClick={() => updateList({ view: "reminders", offset: 0 })}
            >
              {n(list.data!.dueCoursesTotal)} corsi in scadenza
            </Button>
          )}
        </div>
      </header>
      <section className={styles.panel} aria-busy={list.isFetching}>
        <div className={styles.lenses}>
          {views.map(([view, label]) => (
            <button
              key={view}
              aria-pressed={filters.view === view}
              className={filters.view === view ? styles.lensActive : ""}
              onClick={() => updateList({ view, offset: 0 })}
            >
              {label}
            </button>
          ))}
        </div>
        <div className={styles.toolbar}>
          <SearchInput
            className={styles.planningSearch}
            value={search}
            onChange={setSearch}
            placeholder="Cerca corso, persona o team…"
          />
          <SingleSelect
            options={
              filterOptions.data?.tags.map((tag) => ({
                value: tag,
                label: tag,
              })) ?? []
            }
            selected={filters.tag || null}
            allowClear
            onChange={(tag) => updateList({ tag: tag ?? "", offset: 0 })}
            ariaLabel="Filtra per tag"
          />
          <Button
            variant="secondary"
            leftIcon={<Icon name="filter" size={16} />}
            onClick={() => setShowFilters((value) => !value)}
          >
            Filtri
          </Button>
        </div>
        {showFilters && (
          <div className={styles.advanced}>
            <SingleSelect
              options={filterOptions.data?.people.map(toOption) ?? []}
              selected={filters.employeeId || null}
              allowClear
              onChange={(employeeId) =>
                updateList({ employeeId: employeeId ?? "", offset: 0 })
              }
              ariaLabel="Filtra per persona"
            />
            <SingleSelect
              options={filterOptions.data?.teams.map(toOption) ?? []}
              selected={filters.teamId || null}
              allowClear
              onChange={(teamId) =>
                updateList({ teamId: teamId ?? "", offset: 0 })
              }
              ariaLabel="Filtra per team"
            />
            <SingleSelect
              options={filterOptions.data?.areas.map(toOption) ?? []}
              selected={filters.skillAreaId || null}
              allowClear
              onChange={(skillAreaId) =>
                updateList({ skillAreaId: skillAreaId ?? "", offset: 0 })
              }
              ariaLabel="Filtra per area"
            />
          </div>
        )}
        <FilterChips
          filters={filters}
          options={filterOptions.data}
          onChange={updateList}
        />
        <p className={styles.scopeNote}>
          I filtri selezionano i corsi; i conteggi comprendono tutto il corso.
        </p>
        {notice && (
          <p className={styles.notice} role="status">
            {notice}
          </p>
        )}
        {list.isLoading ? (
          <Skeleton rows={6} />
        ) : list.isError && !list.data ? (
          <Error retry={() => list.refetch()} />
        ) : total === 0 ? (
          <>
            <Empty
              filtered={active}
              onClear={() => updateList(defaults)}
              onExternal={saveExternalContext}
            />
            {list.isError && (
              <StaleError onRetry={() => list.refetch()} />
            )}
          </>
        ) : (
          <>
            <div className={styles.tableWrap}>
              <table className={styles.table}>
                <VisuallyHidden as="caption">
                  Corsi in pianificazione
                </VisuallyHidden>
                <thead>
                  <tr>
                    <th>Corso e persone</th>
                    <th>Avanzamento</th>
                    <th>Acquisto</th>
                    <th>Promemoria</th>
                  </tr>
                </thead>
                <tbody>
                  {list.data?.items.map((course) => (
                    <CourseRow
                      key={course.id}
                      course={course}
                      view={filters.view}
                      expanded={expanded === course.id}
                      detail={expandedDetail}
                      onExpand={() => {
                        const query = new URLSearchParams(params);
                        expanded === course.id
                          ? query.delete("expanded")
                          : query.set("expanded", course.id);
                        setParams(query);
                      }}
                      onDrawer={openDrawer}
                      onReminder={editReminder}
                      onTag={(tag) => updateList({ tag, offset: 0 })}
                      onExternal={saveExternalContext}
                    />
                  ))}
                </tbody>
              </table>
            </div>
            {list.isError && (
              <StaleError onRetry={() => list.refetch()} />
            )}
            <div className={styles.pagination}>
              <span>
                {filters.offset + 1}–{end} di {n(total)} corsi
              </span>
              <Button
                size="sm"
                variant="secondary"
                disabled={!filters.offset}
                onClick={() =>
                  updateList({
                    offset: Math.max(0, filters.offset - filters.limit),
                  })
                }
              >
                Precedente
              </Button>
              <Button
                size="sm"
                variant="secondary"
                disabled={end >= total}
                onClick={() =>
                  updateList({ offset: filters.offset + filters.limit })
                }
              >
                Successiva
              </Button>
            </div>
          </>
        )}
      </section>
      <PlanningCourseDrawer
        courseId={drawer}
        section={itemsParams.kind}
        params={itemsParams}
        onParams={updateDrawer}
        onClose={closeDrawer}
        onEditReminder={editReminder}
        onExternal={saveExternalContext}
        onSwitchCourse={(id) => {
          const query = new URLSearchParams(params);
          query.set("drawer", id);
          [
            "itemQ",
            "itemTeam",
            "itemEvent",
            "itemStatus",
            "itemOffset",
          ].forEach((key) => query.delete(key));
          setParams(query);
        }}
      />
      <PlanningReminderModal
        reminder={reminder}
        onClose={() => setReminder(null)}
        onCompleted={reminderCompleted}
      />
    </div>
  );
}

function FilterChips({
  filters,
  options,
  onChange,
}: {
  filters: PlanningListParams;
  options: ReturnType<typeof usePlanningFilters>["data"];
  onChange: (next: Partial<PlanningListParams>) => void;
}) {
  const chips = [
    { key: "q", label: filters.q },
    { key: "tag", label: filters.tag },
    {
      key: "employeeId",
      label:
        options?.people.find((item) => item.id === filters.employeeId)?.name ??
        filters.employeeId,
    },
    {
      key: "teamId",
      label:
        options?.teams.find((item) => item.id === filters.teamId)?.name ??
        filters.teamId,
    },
    {
      key: "skillAreaId",
      label:
        options?.areas.find((item) => item.id === filters.skillAreaId)?.name ??
        filters.skillAreaId,
    },
  ].filter((chip) => chip.label);
  if (!chips.length) return null;
  return (
    <div className={styles.chips} aria-label="Filtri applicati">
      {chips.map((chip) => (
        <button
          key={chip.key}
          onClick={() => onChange({ [chip.key]: "", offset: 0 })}
        >
          Rimuovi filtro {chip.label} <Icon name="x" size={14} />
        </button>
      ))}
    </div>
  );
}

// Contatore "Vedi tutti" per lente: mostra solo i promemoria pertinenti alla
// lente attiva (issue §9). In Storico le attività escluse per sospensione non
// vengono mostrate, quindi il contatore è zero.
function lensReminderTotal(course: CourseSummary, view: PlanningView) {
  switch (view) {
    case "operative":
      return course.operativeReminderCount;
    case "reminders":
      return course.dueReminderCount;
    case "suspended":
      return course.reminderCount - course.operativeReminderCount;
    case "history":
      return 0;
  }
}

function CourseRow({
  course,
  view,
  expanded,
  detail,
  onExpand,
  onDrawer,
  onReminder,
  onTag,
  onExternal,
}: {
  course: CourseSummary;
  view: PlanningView;
  expanded: boolean;
  detail: ReturnType<typeof usePlanningCourseDetail>;
  onExpand: () => void;
  onDrawer: (id: string, section?: PlanningItemKind) => void;
  onReminder: (reminder: PlanningReminder, trigger: HTMLElement) => void;
  onTag: (tag: string) => void;
  onExternal: (event: MouseEvent<HTMLAnchorElement>) => void;
}) {
  const reminder = course.primaryReminder;
  const reminderTotal = lensReminderTotal(course, view);
  const date = reminder && reminderMonthDay(reminder.date);
  return (
    <>
      <tr className={expanded ? styles.selectedRow : ""}>
        <td>
          <div className={styles.courseIdentity}>
            <button
              className={styles.courseButton}
              onClick={onExpand}
              aria-expanded={expanded}
              aria-label={`${expanded ? "Chiudi" : "Espandi"} ${course.title}`}
            >
              <Icon
                name={expanded ? "chevron-up" : "chevron-right"}
                size={16}
              />
              <span>
                <strong>{course.title}</strong>
                <small>
                  {n(course.peopleCount)}{" "}
                  {course.peopleCount === 1 ? "persona" : "persone"}
                  {course.enrollmentsCount !== course.peopleCount &&
                    ` · ${n(course.enrollmentsCount)} iscrizioni`}
                </small>
              </span>
            </button>
            <Link
              className={styles.catalogLink}
              to={`/catalogo?id=${course.id}`}
              onClick={onExternal}
            >
              Apri corso<VisuallyHidden>: {course.title}</VisuallyHidden>
            </Link>
          </div>
          <span className={styles.tags}>
            {course.tags.map((tag) => (
              <button key={tag} onClick={() => onTag(tag)}>
                {tag}
              </button>
            ))}
            {course.courseSuspended && <em>Sospeso</em>}
            {course.priority !== null && (
              <span
                className={styles.priority}
                aria-label={`Priorità delle richieste ${course.priority}`}
              >
                P{course.priority}
              </span>
            )}
          </span>
        </td>
        <td>
          {course.requestsCount > 0 && (
            <span>
              Richieste: {n(course.operativeRequestsCount)} da trattare
              {course.suspendedRequestsCount
                ? ` · ${n(course.suspendedRequestsCount)} sospese`
                : ""}
            </span>
          )}
          <span>Iscrizioni: {deliverySummary(course.enrollmentsByStatus)}</span>
        </td>
        <td>
          {course.economic.distinctPOCount ? (
            <>
              <span>
                {n(course.economic.approved)} RDA approvate ·{" "}
                {n(course.economic.pending)} in approvazione ·{" "}
                {n(course.economic.rejected)} respinte
              </span>
              <span>
                {n(course.economic.coveredEnrollments)} iscrizioni coperte
                esplicitamente
              </span>
            </>
          ) : (
            <span>Nessuna RDA collegata</span>
          )}
        </td>
        <td>
          {reminder ? (
            <div className={styles.reminder}>
              <div
                className={`${styles.dateTile} ${reminder.timing === "overdue" || reminder.timing === "today" ? styles.due : ""}`}
              >
                {date ? (
                  <>
                    <b>{date.day}</b>
                    <small>{date.label}</small>
                  </>
                ) : (
                  <Icon name="clock" size={16} />
                )}
              </div>
              <div>
                <span>{reminder.text}</span>
                <small>
                  {reminder.ownerLabel} · {reminderDate(reminder)}
                </small>
                <button
                  className={styles.textButton}
                  onClick={(event) => onReminder(reminder, event.currentTarget)}
                >
                  Modifica promemoria
                </button>
                {reminderTotal > 1 && (
                  <button
                    className={styles.textButton}
                    onClick={() => onDrawer(course.id, "reminders")}
                  >
                    Vedi tutti ({n(reminderTotal)})
                  </button>
                )}
              </div>
            </div>
          ) : reminderTotal > 0 ? (
            <button
              className={styles.textButton}
              onClick={() => onDrawer(course.id, "reminders")}
            >
              Vedi tutti ({n(reminderTotal)})
            </button>
          ) : (
            <span>Nessun promemoria</span>
          )}
        </td>
      </tr>
      {expanded && (
        <tr className={styles.expansion}>
          <td colSpan={4}>
            <Preview
              course={course}
              detail={detail}
              onDrawer={onDrawer}
              onExternal={onExternal}
            />
          </td>
        </tr>
      )}
    </>
  );
}

function Preview({
  course,
  detail,
  onDrawer,
  onExternal,
}: {
  course: CourseSummary;
  detail: ReturnType<typeof usePlanningCourseDetail>;
  onDrawer: (id: string, section?: PlanningItemKind) => void;
  onExternal: (event: MouseEvent<HTMLAnchorElement>) => void;
}) {
  if (detail.isLoading && !detail.data) return <Skeleton rows={2} />;
  if (!detail.data)
    return detail.isError ? (
      <p className={styles.localError} role="alert">
        Impossibile caricare il dettaglio.{" "}
        <Button size="sm" variant="ghost" onClick={() => detail.refetch()}>
          Riprova
        </Button>
      </p>
    ) : null;
  return (
    <div
      className={styles.preview}
      aria-busy={detail.isFetching || detail.isError}
    >
      {detail.isError && (
        <p className={styles.localError} role="alert">
          Dati non aggiornati: impossibile aggiornare il dettaglio.{" "}
          <Button size="sm" variant="ghost" onClick={() => detail.refetch()}>
            Riprova
          </Button>
        </p>
      )}
      <section>
        <h2>
          Richieste{" "}
          <small>
            Mostrate {detail.data.requestsPreview.length} di{" "}
            {course.requestsCount}
          </small>
        </h2>
        {detail.data.requestsPreview.length ? (
          <>
            {detail.data.requestsPreview.map((request) => (
              <p key={request.id}>
                <Link to={`/richieste?id=${request.id}`} onClick={onExternal}>
                  {request.employee.name}
                </Link>{" "}
                · {requestState(request)}
              </p>
            ))}
            {course.requestsCount > detail.data.requestsPreview.length && (
              <Button
                size="sm"
                variant="ghost"
                onClick={() => onDrawer(course.id, "requests")}
              >
                Vedi tutte ({n(course.requestsCount)})
              </Button>
            )}
          </>
        ) : (
          <p>Nessuna richiesta collegata</p>
        )}
      </section>
      <section>
        <h2>
          Eventi{" "}
          <small>
            Mostrati {detail.data.eventsPreview.length} di {course.eventsCount}
          </small>
        </h2>
        {detail.data.eventsPreview.length ? (
          <>
            {detail.data.eventsPreview.map((event) => (
              <p key={event.id}>
                <Link to={`/eventi/${event.id}`} onClick={onExternal}>
                  {event.title}
                </Link>{" "}
                · {eventState(event)} · {eventConditions(event)}
              </p>
            ))}
            {course.eventsCount > detail.data.eventsPreview.length && (
              <Button
                size="sm"
                variant="ghost"
                onClick={() => onDrawer(course.id, "events")}
              >
                Vedi tutti ({n(course.eventsCount)})
              </Button>
            )}
          </>
        ) : (
          <p>Nessun evento collegato</p>
        )}
      </section>
      <Button
        size="sm"
        variant="secondary"
        onClick={() => onDrawer(course.id, "enrollments")}
        disabled={!course.enrollmentsCount}
      >
        Vedi iscritti ({n(course.enrollmentsCount)})
      </Button>
    </div>
  );
}

function StaleError({ onRetry }: { onRetry: () => void }) {
  return (
    <p className={styles.staleError} role="status">
      Dati non aggiornati: impossibile aggiornare la pianificazione.{" "}
      <Button size="sm" variant="ghost" onClick={onRetry}>
        Riprova
      </Button>
    </p>
  );
}
function Error({ retry }: { retry: () => void }) {
  return (
    <div className={styles.empty}>
      <Icon name="x" size={32} />
      <strong>Impossibile caricare la pianificazione</strong>
      <Button variant="secondary" onClick={retry}>
        Riprova
      </Button>
    </div>
  );
}
function Empty({
  filtered,
  onClear,
  onExternal,
}: {
  filtered: boolean;
  onClear: () => void;
  onExternal: (event: MouseEvent<HTMLAnchorElement>) => void;
}) {
  return (
    <div className={styles.empty}>
      <Icon name="list-ordered" size={32} />
      <strong>
        {filtered
          ? "Nessun corso corrisponde ai filtri"
          : "Nessuna attività da seguire"}
      </strong>
      {filtered ? (
        <Button variant="secondary" onClick={onClear}>
          Rimuovi filtri
        </Button>
      ) : (
        <Link to="/catalogo" onClick={onExternal}>
          Apri Catalogo
        </Link>
      )}
    </div>
  );
}
function isScrollContext(
  value: unknown,
): value is { url: string; position: number } {
  return (
    typeof value === "object" &&
    value !== null &&
    "url" in value &&
    "position" in value &&
    typeof value.url === "string" &&
    typeof value.position === "number"
  );
}

function toOption(item: { id: string; name: string }) {
  return { value: item.id, label: item.name };
}
