-- Site ibridi: global (permanenti, riutilizzabili) + scoped (usa-e-getta, legati a una manutenzione).
-- Applicare su DB che ha gia' applicato docs/manutenzioni_schema.sql.

alter table maintenance.site
    add column if not exists scope text not null default 'global'
        check (scope in ('global','scoped')),
    add column if not exists owner_maintenance_id bigint
        references maintenance.maintenance(maintenance_id) on delete cascade;

alter table maintenance.site
    drop constraint if exists site_scope_consistency;
alter table maintenance.site
    add constraint site_scope_consistency check (
        (scope = 'global' and owner_maintenance_id is null)
        or (scope = 'scoped' and owner_maintenance_id is not null)
    );

-- code e' unico a livello globale solo fra i global; gli scoped vivono in un
-- namespace per-manutenzione. Il vincolo precedente era un UNIQUE sulla colonna:
-- lo sostituiamo con due indici univoci parziali.
alter table maintenance.site drop constraint if exists site_code_key;
drop index if exists maintenance.site_code_key;

create unique index if not exists site_code_global_unique
    on maintenance.site (code)
    where scope = 'global';

create unique index if not exists site_code_scoped_unique
    on maintenance.site (owner_maintenance_id, code)
    where scope = 'scoped';

create index if not exists idx_site_owner_maintenance
    on maintenance.site (owner_maintenance_id)
    where scope = 'scoped';
