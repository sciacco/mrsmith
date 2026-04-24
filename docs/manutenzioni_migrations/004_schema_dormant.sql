-- Schema dormiente per dipendenze catalogo servizi.
-- Applicare dopo docs/manutenzioni_migrations/003_service_taxonomy_audience.sql.
-- Nessuna modifica a workflow, UI o payload REST: i nuovi campi restano
-- retrocompatibili grazie a default e nullable.

alter table maintenance.maintenance_service_taxonomy
    add column if not exists role text not null default 'operated',
    add column if not exists expected_severity text not null default 'unavailable',
    add column if not exists expected_audience text;

alter table maintenance.maintenance_service_taxonomy
    drop constraint if exists maintenance_service_taxonomy_role_check,
    drop constraint if exists maintenance_service_taxonomy_expected_severity_check,
    drop constraint if exists maintenance_service_taxonomy_expected_audience_check;

alter table maintenance.maintenance_service_taxonomy
    add constraint maintenance_service_taxonomy_role_check
    check (role in ('operated','dependent')),
    add constraint maintenance_service_taxonomy_expected_severity_check
    check (expected_severity in ('none','degraded','unavailable')),
    add constraint maintenance_service_taxonomy_expected_audience_check
    check (expected_audience is null or expected_audience in ('internal','external','both'));

alter table maintenance.maintenance_target
    add column if not exists service_taxonomy_id bigint;

alter table maintenance.maintenance_target
    drop constraint if exists maintenance_target_service_taxonomy_id_fkey;

alter table maintenance.maintenance_target
    add constraint maintenance_target_service_taxonomy_id_fkey
    foreign key (service_taxonomy_id)
    references maintenance.service_taxonomy(service_taxonomy_id)
    on delete set null;

create index if not exists idx_maintenance_target_service_taxonomy
    on maintenance.maintenance_target(service_taxonomy_id);

create table if not exists maintenance.service_dependency (
    service_dependency_id bigint generated always as identity primary key,
    upstream_service_id   bigint not null references maintenance.service_taxonomy(service_taxonomy_id) on delete restrict,
    downstream_service_id bigint not null references maintenance.service_taxonomy(service_taxonomy_id) on delete restrict,
    dependency_type       text not null
                          check (dependency_type in ('runs_on','connects_through','consumes','depends_on')),
    is_redundant          boolean not null default false,
    default_severity      text not null default 'unavailable'
                          check (default_severity in ('none','degraded','unavailable')),
    source                text not null default 'manual'
                          check (source in ('manual','ai_suggested','usage_suggested','import_suggested')),
    is_active             boolean not null default true,
    metadata              jsonb not null default '{}'::jsonb,
    created_at            timestamptz not null default now(),
    updated_at            timestamptz not null default now(),

    unique (upstream_service_id, downstream_service_id, dependency_type),
    check (upstream_service_id <> downstream_service_id)
);

create index if not exists idx_service_dependency_upstream_active
    on maintenance.service_dependency(upstream_service_id)
    where is_active;

create index if not exists idx_service_dependency_downstream_active
    on maintenance.service_dependency(downstream_service_id)
    where is_active;
