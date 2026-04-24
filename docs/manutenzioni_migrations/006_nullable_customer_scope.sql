-- Ambito clienti opzionale per bozze e manutenzioni annullate.
-- Applicare su DB che ha gia' applicato docs/manutenzioni_schema.sql.

alter table maintenance.maintenance
    alter column customer_scope_id drop not null;

alter table maintenance.maintenance
    drop constraint if exists maintenance_customer_scope_required_for_active_status;

alter table maintenance.maintenance
    add constraint maintenance_customer_scope_required_for_active_status
    check (
        customer_scope_id is not null
        or status in ('draft','cancelled')
    );
