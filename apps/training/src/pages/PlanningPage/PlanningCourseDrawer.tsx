import { useEffect, type MouseEvent, type ReactNode } from "react";
import { Link } from "react-router-dom";
import {
  Button,
  Drawer,
  SearchInput,
  SingleSelect,
  Skeleton,
  StatusBadge,
} from "@mrsmith/ui";
import { usePlanningCourseDetail, usePlanningItems } from "../../api/queries";
import type {
  PlanningEnrollmentItem,
  PlanningEventItem,
  PlanningItemKind,
  PlanningItemsParams,
  PlanningItemsResponse,
  PlanningReminder,
  PlanningRequestItem,
} from "../../api/types";
import {
  deliveryStatusLabels,
  eventConditions,
  eventState,
  instant,
  localDate,
  n,
  requestState,
} from "./planningFormat";
import styles from "./PlanningPage.module.css";

const sections: Array<[PlanningItemKind, string]> = [
  ["enrollments", "Iscritti"],
  ["requests", "Richieste"],
  ["events", "Eventi"],
  ["reminders", "Promemoria"],
];

interface Props {
  courseId: string | null;
  section: PlanningItemKind;
  params: PlanningItemsParams;
  onParams: (params: Partial<PlanningItemsParams>, reset?: boolean) => void;
  onClose: () => void;
  onEditReminder: (reminder: PlanningReminder, trigger: HTMLElement) => void;
  onExternal: (event: MouseEvent<HTMLAnchorElement>) => void;
  onSwitchCourse: (id: string) => void;
}

export function PlanningCourseDrawer({
  courseId,
  section,
  params,
  onParams,
  onClose,
  onEditReminder,
  onExternal,
  onSwitchCourse,
}: Props) {
  const detail = usePlanningCourseDetail(courseId ?? undefined);
  const items = usePlanningItems(courseId ?? undefined, params);
  const data = items.data;
  const total = data?.total ?? 0;
  const end = Math.min(params.offset + (data?.items.length ?? 0), total);
  const statusOptions = statusOptionsFor(section);
  const needsOffsetCorrection = Boolean(
    data &&
      data.kind === section &&
      params.offset > 0 &&
      params.offset >= data.total,
  );

  useEffect(() => {
    if (!items.isSuccess || !data || !needsOffsetCorrection) return;
    const lastOffset =
      data.total === 0
        ? 0
        : Math.floor((data.total - 1) / params.limit) * params.limit;
    if (params.offset !== lastOffset) onParams({ offset: lastOffset });
  }, [
    data,
    items.isSuccess,
    needsOffsetCorrection,
    onParams,
    params.limit,
    params.offset,
  ]);

  return (
    <Drawer
      open={Boolean(courseId)}
      onClose={onClose}
      size="xl"
      title={detail.data?.course.title ?? "Corso"}
      subtitle={
        detail.data
          ? peopleAndEnrollments(
              detail.data.course.peopleCount,
              detail.data.course.enrollmentsCount,
            )
          : undefined
      }
      footer={
        <DrawerPagination
          adjusting={needsOffsetCorrection}
          end={end}
          params={params}
          section={section}
          total={total}
          onParams={onParams}
        />
      }
    >
      <div className={styles.drawerBody}>
        {detail.isError && (
          <p className={styles.localError} role="alert">
            Impossibile aggiornare l’anteprima del corso. Le sezioni locali
            restano disponibili.{" "}
            <Button size="sm" variant="ghost" onClick={() => detail.refetch()}>
              Riprova
            </Button>
          </p>
        )}
        <div className={styles.sectionTabs} aria-label="Sezioni del corso">
          {sections.map(([kind, label]) => (
            <button
              key={kind}
              className={section === kind ? styles.sectionActive : ""}
              aria-pressed={section === kind}
              onClick={() => onParams({ kind }, true)}
            >
              {label}
            </button>
          ))}
        </div>
        <DrawerFilters
          detail={detail.data}
          params={params}
          section={section}
          statusOptions={statusOptions}
          onParams={onParams}
        />
        <DrawerItems
          data={data}
          error={items.isError}
          loading={items.isLoading}
          section={section}
          onEditReminder={onEditReminder}
          onExternal={onExternal}
          onRetry={() => items.refetch()}
          onSwitchCourse={onSwitchCourse}
        />
      </div>
    </Drawer>
  );
}

function DrawerFilters({
  detail,
  params,
  section,
  statusOptions,
  onParams,
}: {
  detail: ReturnType<typeof usePlanningCourseDetail>["data"];
  params: PlanningItemsParams;
  section: PlanningItemKind;
  statusOptions: Array<{ value: string; label: string }>;
  onParams: Props["onParams"];
}) {
  const needsTeam = section === "enrollments" || section === "requests";
  const needsEvent = section === "enrollments" || section === "reminders";
  return (
    <div className={styles.drawerFilters}>
      <SearchInput
        className={styles.drawerSearch}
        value={params.q}
        onChange={(q) => onParams({ q, offset: 0 })}
        placeholder="Cerca nella sezione…"
      />
      {needsTeam && (
        <SingleSelect
          options={detail?.teamOptions.map(toOption) ?? []}
          selected={params.teamId || null}
          allowClear
          onChange={(teamId) => onParams({ teamId: teamId ?? "", offset: 0 })}
          ariaLabel="Filtra per team"
        />
      )}
      {needsEvent && (
        <SingleSelect
          options={detail?.eventOptions.map(toOption) ?? []}
          selected={params.eventId || null}
          allowClear
          onChange={(eventId) =>
            onParams({ eventId: eventId ?? "", offset: 0 })
          }
          ariaLabel="Filtra per evento"
        />
      )}
      <SingleSelect
        options={statusOptions}
        selected={params.status || null}
        allowClear
        onChange={(status) => onParams({ status: status ?? "", offset: 0 })}
        ariaLabel="Filtra per stato"
      />
    </div>
  );
}

function DrawerItems({
  data,
  error,
  loading,
  section,
  onEditReminder,
  onExternal,
  onRetry,
  onSwitchCourse,
}: {
  data: PlanningItemsResponse | undefined;
  error: boolean;
  loading: boolean;
  section: PlanningItemKind;
  onEditReminder: Props["onEditReminder"];
  onExternal: Props["onExternal"];
  onRetry: () => void;
  onSwitchCourse: (id: string) => void;
}) {
  if (loading && !data) return <Skeleton rows={5} />;
  if (!data || data.kind !== section) {
    return error ? (
      <p className={styles.localError} role="alert">
        Impossibile caricare questa sezione.{" "}
        <Button size="sm" variant="ghost" onClick={onRetry}>
          Riprova
        </Button>
      </p>
    ) : null;
  }

  return (
    <div aria-busy={loading || error}>
      {error && (
        <p className={styles.localError} role="alert">
          Dati non aggiornati: impossibile aggiornare questa sezione.{" "}
          <Button size="sm" variant="ghost" onClick={onRetry}>
            Riprova
          </Button>
        </p>
      )}
      {data.kind === "enrollments" && (
        <ItemList
          items={data.items}
          empty="Nessuna iscrizione corrisponde ai filtri."
          render={(item) => (
            <EnrollmentCard
              key={item.id}
              enrollment={item}
              onExternal={onExternal}
            />
          )}
        />
      )}
      {data.kind === "requests" && (
        <ItemList
          items={data.items}
          empty="Nessuna richiesta corrisponde ai filtri."
          render={(item) => (
            <RequestCard
              key={item.id}
              request={item}
              onExternal={onExternal}
              onSwitchCourse={onSwitchCourse}
            />
          )}
        />
      )}
      {data.kind === "events" && (
        <ItemList
          items={data.items}
          empty="Nessun evento corrisponde ai filtri."
          render={(item) => (
            <EventCard key={item.id} event={item} onExternal={onExternal} />
          )}
        />
      )}
      {data.kind === "reminders" && (
        <ItemList
          items={data.items}
          empty="Nessun promemoria corrisponde ai filtri."
          render={(item) => (
            <ReminderCard
              key={`${item.ownerKind}-${item.ownerId}`}
              reminder={item}
              onEditReminder={onEditReminder}
            />
          )}
        />
      )}
    </div>
  );
}

function ItemList<T>({
  items,
  empty,
  render,
}: {
  items: T[];
  empty: string;
  render: (item: T) => ReactNode;
}) {
  return items.length ? (
    <div className={styles.itemList}>{items.map(render)}</div>
  ) : (
    <p className={styles.sectionEmpty}>{empty}</p>
  );
}

function EnrollmentCard({
  enrollment,
  onExternal,
}: {
  enrollment: PlanningEnrollmentItem;
  onExternal: Props["onExternal"];
}) {
  return (
    <article className={styles.itemCard}>
      <strong>
        <Link to={`/persone/${enrollment.employee.id}`} onClick={onExternal}>
          {enrollment.employee.name}
        </Link>
      </strong>
      <span>
        {enrollment.event.name} ·{" "}
        {enrollment.teams.map((team) => team.name).join(", ") ||
          "Nessun team corrente"}
      </span>
      <StatusBadge
        value={enrollment.deliveryStatus}
        label={deliveryStatusLabels[enrollment.deliveryStatus]}
        variant="neutral"
        dot
      />
      <div>
        <Link
          to={`/eventi/${enrollment.event.id}?highlight=enrollments`}
          onClick={onExternal}
        >
          Apri iscritti dell’evento
        </Link>
        {enrollment.requestIds.map((id) => (
          <Link key={id} to={`/richieste?id=${id}`} onClick={onExternal}>
            Apri richiesta
          </Link>
        ))}
      </div>
    </article>
  );
}

function RequestCard({
  request,
  onExternal,
  onSwitchCourse,
}: {
  request: PlanningRequestItem;
  onExternal: Props["onExternal"];
  onSwitchCourse: (id: string) => void;
}) {
  const areaLevels = request.areas
    .map(
      (area) =>
        `${area.name}: livello ${area.levelCurrent ?? "—"} → ${area.levelTarget ?? "—"}`,
    )
    .join(" · ");
  return (
    <article className={styles.itemCard}>
      <strong>
        <Link to={`/persone/${request.employee.id}`} onClick={onExternal}>
          {request.employee.name}
        </Link>
      </strong>
      <span>
        {request.team.name} ·{" "}
        {request.priority === null
          ? "Nessuna priorità"
          : `Priorità ${request.priority}`}{" "}
        · {requestState(request)}
      </span>
      {areaLevels && <span>{areaLevels}</span>}
      <span>
        Parere TL: {request.tlOpinion ?? "—"} · Decisione People:{" "}
        {request.peopleDecision ?? "—"}
      </span>
      <div>
        <Link to={`/richieste?id=${request.id}`} onClick={onExternal}>
          Apri richiesta
        </Link>
        {request.acceptedCourse && (
          <button
            className={styles.textButton}
            onClick={() => onSwitchCourse(request.acceptedCourse!.id)}
          >
            Corso accolto: {request.acceptedCourse.name}
          </button>
        )}
        {request.acceptedEventId && (
          <Link to={`/eventi/${request.acceptedEventId}`} onClick={onExternal}>
            Evento accolto
          </Link>
        )}
        {request.resultingEnrollmentId && (
          <span>Iscrizione risultante: {request.resultingEnrollmentId}</span>
        )}
      </div>
    </article>
  );
}

function EventCard({
  event,
  onExternal,
}: {
  event: PlanningEventItem;
  onExternal: Props["onExternal"];
}) {
  return (
    <article className={styles.itemCard}>
      <strong>{event.title}</strong>
      <span>
        Origine: {event.origin} · Sessioni: {n(event.sessionsCount)} ·{" "}
        {instant(event.startsAt)} — {instant(event.endsAt)}
        {event.dueOn ? ` · Scadenza ${localDate(event.dueOn)}` : ""}
      </span>
      <span>
        {eventState(event)} · {n(event.enrollmentsCount)} iscrizioni
      </span>
      <span>{eventConditions(event)}</span>
      <div>
        <Link to={`/eventi/${event.id}`} onClick={onExternal}>
          Apri evento
        </Link>
        <Link
          to={`/eventi/${event.id}?highlight=expenses`}
          onClick={onExternal}
        >
          Apri spese
        </Link>
      </div>
    </article>
  );
}

function ReminderCard({
  reminder,
  onEditReminder,
}: {
  reminder: PlanningReminder;
  onEditReminder: Props["onEditReminder"];
}) {
  return (
    <article className={styles.itemCard}>
      <strong>{reminder.ownerLabel}</strong>
      <span>{reminder.text}</span>
      <span>
        {reminder.date ? localDate(reminder.date) : "Senza data"}
        {!reminder.operative ? " · Non operativo per sospensione" : ""}
      </span>
      <Button
        size="sm"
        variant="ghost"
        onClick={(event) => onEditReminder(reminder, event.currentTarget)}
      >
        Modifica promemoria
      </Button>
    </article>
  );
}

function DrawerPagination({
  adjusting,
  end,
  params,
  section,
  total,
  onParams,
}: {
  adjusting: boolean;
  end: number;
  params: PlanningItemsParams;
  section: PlanningItemKind;
  total: number;
  onParams: Props["onParams"];
}) {
  const noun =
    section === "enrollments"
      ? total === 1
        ? "iscrizione"
        : "iscrizioni"
      : total === 1
        ? "elemento"
        : "elementi";
  const interval = total && !adjusting ? `${params.offset + 1}–${end}` : "0";
  return (
    <div className={styles.drawerFooter}>
      <span>
        {adjusting ? "Aggiornamento pagina…" : interval} di {n(total)} {noun}
      </span>
      <div>
        <Button
          size="sm"
          variant="secondary"
          disabled={adjusting || params.offset === 0}
          onClick={() =>
            onParams({ offset: Math.max(0, params.offset - params.limit) })
          }
        >
          Precedente
        </Button>
        <Button
          size="sm"
          variant="secondary"
          disabled={adjusting || end >= total}
          onClick={() => onParams({ offset: params.offset + params.limit })}
        >
          Successiva
        </Button>
      </div>
    </div>
  );
}

function statusOptionsFor(section: PlanningItemKind) {
  if (section === "enrollments")
    return Object.entries(deliveryStatusLabels).map(([value, label]) => ({
      value,
      label,
    }));
  if (section === "requests")
    return [
      { value: "open", label: "Aperte" },
      { value: "suspended", label: "Sospese" },
      { value: "accepted", label: "Accolte" },
      { value: "rejected", label: "Respinte" },
      { value: "withdrawn", label: "Ritirate" },
    ];
  if (section === "events")
    return [
      { value: "operational", label: "Operativi" },
      { value: "terminal", label: "Terminati" },
      { value: "cancelled", label: "Annullati" },
    ];
  return [
    { value: "due", label: "Scaduti o oggi" },
    { value: "future", label: "Futuri" },
    { value: "undated", label: "Senza data" },
    { value: "suspended", label: "Sospesi" },
  ];
}

function peopleAndEnrollments(people: number, enrollments: number) {
  return `${n(people)} ${people === 1 ? "persona" : "persone"} · ${n(enrollments)} ${enrollments === 1 ? "iscrizione" : "iscrizioni"}`;
}

function toOption(ref: { id: string; name: string }) {
  return { value: ref.id, label: ref.name };
}
