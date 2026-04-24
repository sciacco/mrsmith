-- Dipendenze iniziali tra servizi catalogo.
-- Applicare dopo docs/manutenzioni_migrations/004_schema_dormant.sql.

alter table maintenance.maintenance_service_taxonomy
    drop constraint if exists maintenance_service_taxonomy_source_check;

alter table maintenance.maintenance_service_taxonomy
    add constraint maintenance_service_taxonomy_source_check
    check (source in ('manual','import','rule','ai_extracted','catalog_mapping','dependency_graph'));

with seed(upstream_code, downstream_code, dependency_type, is_redundant, default_severity) as (
    values
        ('switching_core_datacenter', 'cluster_k8s', 'connects_through', true, 'degraded'),
        ('switching_core_datacenter', 'cloudstack', 'connects_through', true, 'degraded'),
        ('switching_core_datacenter', 'vmware_iaas', 'connects_through', true, 'degraded'),
        ('switching_core_datacenter', 'kvm_iaas', 'connects_through', true, 'degraded'),
        ('switching_core_datacenter', 'proxmox_privato', 'connects_through', true, 'degraded'),
        ('switching_core_datacenter', 'storage_san_nas', 'connects_through', true, 'degraded'),
        ('routing_edge_border', 'dns_autoritativi', 'connects_through', true, 'degraded'),
        ('routing_edge_border', 'customer_portal', 'connects_through', true, 'degraded'),
        ('firewall_perimetrale', 'customer_portal', 'connects_through', true, 'degraded'),
        ('firewall_perimetrale', 'mistra_gateway', 'connects_through', true, 'degraded'),
        ('cluster_k8s', 'customer_portal', 'runs_on', true, 'none'),
        ('cluster_k8s', 'keycloak_k8s', 'runs_on', true, 'none'),
        ('cluster_k8s', 'mistra_gateway', 'runs_on', true, 'none'),
        ('proxmox_privato', 'grappa', 'runs_on', true, 'none'),
        ('vmware_iaas', 'db01virt_mysql', 'runs_on', true, 'none'),
        ('db01virt_mysql', 'grappa', 'consumes', false, 'unavailable'),
        ('storage_san_nas', 'vmware_iaas', 'depends_on', true, 'degraded'),
        ('storage_san_nas', 'kvm_iaas', 'depends_on', true, 'degraded'),
        ('patroni', 'mistra_gateway', 'consumes', true, 'degraded'),
        ('galera', 'mistra_gateway', 'consumes', true, 'degraded'),
        ('mistra_gateway', 'customer_portal', 'consumes', false, 'unavailable'),
        ('cloudstack', 'bitwarden', 'runs_on', false, 'unavailable'),
        ('active_directory', 'keycloak_k8s', 'consumes', false, 'unavailable'),
        ('active_directory', 'keycloak_esterno', 'consumes', false, 'unavailable'),
        ('active_directory', 'gitlab', 'consumes', false, 'unavailable'),
        ('active_directory', 'grappa', 'consumes', false, 'unavailable'),
        ('active_directory', 'gestione_portale', 'consumes', false, 'unavailable'),
        ('active_directory', 'appsmith', 'consumes', false, 'unavailable'),
        ('active_directory', 'bitwarden', 'consumes', false, 'unavailable')
)
insert into maintenance.service_dependency (
    upstream_service_id,
    downstream_service_id,
    dependency_type,
    is_redundant,
    default_severity,
    source,
    is_active
)
select
    upstream.service_taxonomy_id,
    downstream.service_taxonomy_id,
    seed.dependency_type,
    seed.is_redundant,
    seed.default_severity,
    'manual',
    true
from seed
join maintenance.service_taxonomy upstream on upstream.code = seed.upstream_code
join maintenance.service_taxonomy downstream on downstream.code = seed.downstream_code
on conflict (upstream_service_id, downstream_service_id, dependency_type) do update set
    is_redundant = excluded.is_redundant,
    default_severity = excluded.default_severity,
    source = 'manual',
    is_active = true,
    updated_at = now();
