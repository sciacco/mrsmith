-- Reset tassonomia servizi/oggetti manutenibili + audience catalogo.
-- Applicare su DB che ha gia' applicato docs/manutenzioni_schema.sql.
-- Il prodotto e' pre-produzione: non sono previste manutenzioni da migrare.

alter table maintenance.service_taxonomy
    add column if not exists target_type_id bigint,
    add column if not exists audience text not null default 'maintenance';

alter table maintenance.service_taxonomy
    drop constraint if exists service_taxonomy_audience_check;
alter table maintenance.service_taxonomy
    add constraint service_taxonomy_audience_check
    check (audience in ('internal','external','both','maintenance'));

insert into maintenance.target_type (code, name_it, name_en, sort_order)
values
    ('site', 'Sito', 'Site', 10),
    ('service', 'Servizio', 'Service', 20),
    ('product', 'Prodotto', 'Product', 30),
    ('platform', 'Piattaforma', 'Platform', 40),
    ('customer', 'Cliente', 'Customer', 50),
    ('order', 'Ordine', 'Order', 60),
    ('asset', 'Asset', 'Asset', 70),
    ('circuit', 'Circuito', 'Circuit', 80),
    ('location', 'Luogo', 'Location', 90),
    ('tenant', 'Tenant', 'Tenant', 100),
    ('other', 'Altro', 'Other', 1000)
on conflict (code) do update set
    name_it = excluded.name_it,
    name_en = excluded.name_en,
    sort_order = excluded.sort_order;

insert into maintenance.technical_domain (code, name_it, name_en, sort_order)
values
    ('applications', 'Applications', 'Applications', 10),
    ('cloud', 'Cloud', 'Cloud', 20),
    ('tlc', 'TLC', 'TLC', 30),
    ('datacenter', 'Datacenter', 'Datacenter', 40),
    ('ms', 'MS', 'MS', 50)
on conflict (code) do update set
    name_it = excluded.name_it,
    name_en = excluded.name_en,
    sort_order = excluded.sort_order;

drop table if exists maintenance._service_taxonomy_seed;
create table maintenance._service_taxonomy_seed (
    code text primary key,
    domain_code text not null,
    target_type_code text not null,
    audience text not null,
    name_it text not null,
    name_en text,
    description text,
    sort_order integer not null
);

insert into maintenance._service_taxonomy_seed (code, domain_code, target_type_code, audience, name_it, name_en, description, sort_order)
values
    ('customer_portal', 'applications', 'service', 'external', 'Customer Portal', 'Customer Portal', NULL, 10),
    ('mistra_gateway', 'applications', 'service', 'both', 'Mistra Gateway', 'Mistra Gateway', 'API gateway per applicazioni interne ed esterne, incluso Customer Portal con SLA stringenti', 20),
    ('mistra_psql_db', 'applications', 'platform', 'both', 'Mistra PSQL DB', 'Mistra PSQL DB', 'Database/logical data store applicativo Mistra; distinto dal cluster HA Patroni gestito da Cloud', 30),
    ('mistra_mysql_db', 'applications', 'platform', 'both', 'Mistra MySQL DB', 'Mistra MySQL DB', 'Database/logical data store applicativo Mistra; distinto dal cluster HA Galera gestito da Cloud', 40),
    ('grappa', 'applications', 'service', 'internal', 'Grappa', 'Grappa', NULL, 50),
    ('keycloak_k8s', 'applications', 'service', 'both', 'Keycloak (k8s)', 'Keycloak (k8s)', 'Auth per app sia interne che esterne', 60),
    ('gitlab', 'applications', 'service', 'internal', 'GitLab', 'GitLab', NULL, 70),
    ('harbor', 'applications', 'service', 'internal', 'Harbor', 'Harbor', 'Container registry', 80),
    ('appsmith', 'applications', 'platform', 'internal', 'Appsmith', 'Appsmith', 'Piattaforma low-code legacy su cui girano molte mini app', 90),
    ('alyante', 'applications', 'service', 'internal', 'Alyante', 'Alyante', NULL, 100),
    ('arxivar', 'applications', 'service', 'internal', 'ARXIVAR', 'ARXIVAR', NULL, 110),
    ('assenzio', 'applications', 'service', 'internal', 'Assenzio', 'Assenzio', NULL, 120),
    ('apicore', 'applications', 'service', 'internal', 'apicore', 'apicore', NULL, 130),
    ('chrono', 'applications', 'service', 'internal', 'chrono', 'chrono', NULL, 140),
    ('hive', 'applications', 'service', 'internal', 'hive', 'hive', NULL, 150),
    ('gestione_portale', 'applications', 'service', 'internal', 'Gestione Portale', 'Portal Management', NULL, 160),
    ('ingressi', 'applications', 'service', 'both', 'INGRESSI', 'INGRESSI', NULL, 170),
    ('cloudstack', 'cloud', 'platform', 'both', 'Cloudstack', 'Cloudstack', 'Piattaforma IaaS + uso interno', 180),
    ('firewall_appliance_clienti', 'cloud', 'platform', 'external', 'Firewall appliance clienti', 'Customer firewall appliances', 'Appliance virtuali pfSense e simili su infrastruttura Cloud', 190),
    ('cpanel', 'cloud', 'platform', 'external', 'cpanel', 'cpanel', 'Hosting clienti', 200),
    ('dns_autoritativi', 'cloud', 'service', 'external', 'DNS Autoritativi', 'Authoritative DNS', 'ns1-ns6', 210),
    ('resolver_dns', 'cloud', 'service', 'external', 'Resolver DNS', 'DNS Resolver', 'Resolver DNS pubblico', 220),
    ('active_directory', 'cloud', 'service', 'internal', 'Active Directory', 'Active Directory', NULL, 230),
    ('timoo_pbx', 'cloud', 'service', 'both', 'TIMOO (PBX)', 'TIMOO (PBX)', 'Uso interno + erogato ai clienti', 240),
    ('bitwarden', 'cloud', 'service', 'internal', 'Bitwarden', 'Bitwarden', NULL, 250),
    ('storage_san_nas', 'cloud', 'asset', 'maintenance', 'Storage SAN/NAS', 'SAN/NAS Storage', 'Dipende da quale volume/storage si tocca', 260),
    ('vmware_iaas', 'cloud', 'platform', 'both', 'VMware IaaS', 'VMware IaaS', NULL, 270),
    ('kvm_iaas', 'cloud', 'platform', 'both', 'KVM IaaS', 'KVM IaaS', NULL, 280),
    ('proxmox_privato', 'cloud', 'platform', 'internal', 'Proxmox Privato', 'Private Proxmox', 'Due cluster interni gestiti da Cloud, dislocati nei datacenter C21 Milano ed E100 Roma', 290),
    ('veeam_backup_infrastrutturale', 'cloud', 'service', 'both', 'Veeam / Backup infrastrutturale', 'Veeam / Infrastructure Backup', NULL, 300),
    ('private_cloud_cloud', 'cloud', 'product', 'external', 'Private Cloud (Cloud)', 'Private Cloud (Cloud)', 'Infrastruttura fisica dedicata a cliente con hypervisor/orchestrator specifico, distinta da MS', 310),
    ('cluster_k8s', 'cloud', 'platform', 'both', 'Cluster k8s', 'k8s Cluster', 'Ospita app sia internal sia external', 320),
    ('patroni', 'cloud', 'platform', 'both', 'Patroni', 'Patroni', 'Cluster HA PostgreSQL gestito da Cloud; puo servire piu app', 330),
    ('galera', 'cloud', 'platform', 'both', 'Galera', 'Galera', 'Cluster HA MySQL gestito da Cloud; puo servire piu app', 340),
    ('db01virt_mysql', 'cloud', 'platform', 'internal', 'db01virt (MySQL)', 'db01virt (MySQL)', 'Database MySQL di Grappa, ospitato come VM su cluster VMware e manutenuto prevalentemente da Cloud', 350),
    ('keycloak_esterno', 'cloud', 'service', 'both', 'Keycloak Esterno', 'External Keycloak', 'Istanza separata da quella su k8s', 360),
    ('switching_core_datacenter', 'tlc', 'asset', 'maintenance', 'Switching core datacenter', 'Datacenter core switching', 'Coppie core / fabric core DC; dipende da quale coppia si tocca', 370),
    ('switching_datacenter_accesso_leaf_tor', 'tlc', 'asset', 'maintenance', 'Switching datacenter accesso/leaf/ToR', 'Datacenter access/leaf/ToR switching', 'Leaf, ToR, switch server-facing, accesso rack', 380),
    ('switching_accesso_distribuzione', 'tlc', 'asset', 'maintenance', 'Switching accesso/distribuzione', 'Access/distribution switching', 'Distribuzione non strettamente DC fabric', 390),
    ('routing_edge_border', 'tlc', 'asset', 'both', 'Routing edge/border', 'Edge/border routing', 'Border router, peering, upstream, internet edge', 400),
    ('routing_backbone_datacenter', 'tlc', 'asset', 'maintenance', 'Routing backbone e datacenter', 'Backbone and datacenter routing', 'Router backbone, inter-DC, routing interno DC', 410),
    ('bras_bng', 'tlc', 'platform', 'external', 'BRAS / BNG', 'BRAS / BNG', 'Piattaforma accesso clienti; impatto su autenticazione/sessioni', 420),
    ('firewall_perimetrale', 'tlc', 'asset', 'both', 'Firewall perimetrale', 'Perimeter firewall', 'Protegge CdLAN ma il down impatta anche clienti', 430),
    ('firewall_multitenant', 'tlc', 'platform', 'external', 'Firewall Multitenant', 'Multitenant firewall', 'Cluster/firewall HA multitenant; manutenzione su tenant, nodo o intero cluster', 440),
    ('radius_applicativo', 'tlc', 'service', 'external', 'RADIUS (applicativo)', 'RADIUS (application)', 'Down: clienti non autenticano', 450),
    ('radius_ui', 'tlc', 'service', 'internal', 'RADIUS UI', 'RADIUS UI', NULL, 460),
    ('ruckus', 'tlc', 'platform', 'internal', 'Ruckus', 'Ruckus', 'Controller wifi corporate', 470),
    ('checkmk', 'tlc', 'service', 'internal', 'CHECKmk', 'CHECKmk', 'Monitoring interno', 480),
    ('desigo', 'datacenter', 'platform', 'internal', 'Desigo', 'Desigo', 'BMS', 490),
    ('lenel', 'datacenter', 'platform', 'internal', 'Lenel', 'Lenel', 'Controllo accessi', 500),
    ('traka', 'datacenter', 'platform', 'internal', 'Traka', 'Traka', 'Gestione chiavi', 510),
    ('power_ups', 'datacenter', 'asset', 'maintenance', 'Power / UPS', 'Power / UPS', 'Dipende dal quadro/impianto toccato', 520),
    ('generatori', 'datacenter', 'asset', 'both', 'Generatori', 'Generators', NULL, 530),
    ('condizionamento', 'datacenter', 'asset', 'maintenance', 'Condizionamento', 'Cooling', NULL, 540),
    ('cablaggio_strutturato_passivo', 'datacenter', 'asset', 'maintenance', 'Cablaggio strutturato passivo', 'Passive structured cabling', 'Permutazioni massive, patch panel, cablaggio fisico non apparati', 550),
    ('illuminazione_sala_locale_tecnico', 'datacenter', 'asset', 'maintenance', 'Illuminazione sala/locale tecnico', 'Room/technical area lighting', 'Illuminazione tecnica di sale DC/locali tecnici', 560),
    ('antincendio_rilevazione_estinzione', 'datacenter', 'asset', 'maintenance', 'Antincendio / rilevazione / estinzione', 'Fire detection / suppression', 'Impianti rilevazione e spegnimento; distinguere dal solo monitoraggio BMS', 570),
    ('sensori_telemetria_ambientale', 'datacenter', 'asset', 'maintenance', 'Sensori e telemetria ambientale', 'Environmental sensors and telemetry', 'Sonde, sensori e punti campo che alimentano BMS/allarmi', 580),
    ('plc_controllori_impianti', 'datacenter', 'asset', 'both', 'PLC / controllori impianti', 'PLC / plant controllers', 'Controllori di campo e automazioni che alimentano o sono supervisionati dal BMS', 590),
    ('facility', 'datacenter', 'location', 'maintenance', 'Facility', 'Facility', 'Infrastruttura strutturale/ambientale non coperta da voci specifiche', 600),
    ('firewall_cliente', 'ms', 'product', 'external', 'Firewall cliente', 'Customer firewall', NULL, 610),
    ('backup_gestito', 'ms', 'product', 'external', 'Backup gestito', 'Managed backup', NULL, 620),
    ('monitoring_gestito', 'ms', 'product', 'external', 'Monitoring gestito', 'Managed monitoring', NULL, 630),
    ('vpn_cliente', 'ms', 'product', 'external', 'VPN cliente', 'Customer VPN', NULL, 640),
    ('infrastrutture_cloud_gestite', 'ms', 'product', 'external', 'Infrastrutture cloud gestite', 'Managed cloud infrastructures', NULL, 650),
    ('private_cloud_ms', 'ms', 'product', 'external', 'Private Cloud (MS)', 'Private Cloud (MS)', NULL, 660);

insert into maintenance.service_taxonomy (
    code,
    technical_domain_id,
    target_type_id,
    audience,
    name_it,
    name_en,
    description,
    sort_order,
    is_active
)
select
    seed.code,
    d.technical_domain_id,
    tt.target_type_id,
    seed.audience,
    seed.name_it,
    seed.name_en,
    seed.description,
    seed.sort_order,
    true
from maintenance._service_taxonomy_seed seed
join maintenance.technical_domain d on d.code = seed.domain_code
join maintenance.target_type tt on tt.code = seed.target_type_code
on conflict (code) do update set
    technical_domain_id = excluded.technical_domain_id,
    target_type_id = excluded.target_type_id,
    audience = excluded.audience,
    name_it = excluded.name_it,
    name_en = excluded.name_en,
    description = excluded.description,
    sort_order = excluded.sort_order,
    is_active = excluded.is_active;

delete from maintenance.service_taxonomy st
where not exists (
    select 1
    from maintenance._service_taxonomy_seed seed
    where seed.code = st.code
);

delete from maintenance.technical_domain td
where td.code not in ('applications', 'cloud', 'tlc', 'datacenter', 'ms');

alter table maintenance.service_taxonomy
    drop constraint if exists service_taxonomy_target_type_id_fkey;
alter table maintenance.service_taxonomy
    add constraint service_taxonomy_target_type_id_fkey
    foreign key (target_type_id)
    references maintenance.target_type(target_type_id);

alter table maintenance.service_taxonomy
    alter column target_type_id set not null,
    alter column audience set default 'maintenance',
    alter column audience set not null;

create index if not exists idx_service_taxonomy_target_type
    on maintenance.service_taxonomy(target_type_id);

drop table if exists maintenance._service_taxonomy_seed;
