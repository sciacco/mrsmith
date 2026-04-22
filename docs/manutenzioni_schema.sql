create schema if not exists maintenance;

-- =========================
-- 1) ANAGRAFICHE MINIME
-- =========================

create table maintenance.site (
    site_id              bigint generated always as identity primary key,
    code                 text not null unique,            -- es: C21, E100, PRATO
    name                 text not null,                   -- es: Data Center C21
    city                 text,
    country_code         char(2),
    is_active            boolean not null default true,
    metadata             jsonb not null default '{}'::jsonb
);

create table maintenance.technical_domain (
    technical_domain_id  bigint generated always as identity primary key,
    code                 text not null unique,
    name_it              text not null,
    name_en              text,
    description          text,
    synonyms             text[] not null default '{}',
    sort_order           integer not null default 100,
    is_active            boolean not null default true,
    metadata             jsonb not null default '{}'::jsonb,

    check (code ~ '^[a-z][a-z0-9_]*$')
);

create table maintenance.maintenance_kind (
    maintenance_kind_id  bigint generated always as identity primary key,
    code                 text not null unique,
    name_it              text not null,
    name_en              text,
    description          text,
    synonyms             text[] not null default '{}',
    sort_order           integer not null default 100,
    is_active            boolean not null default true,
    metadata             jsonb not null default '{}'::jsonb,

    check (code ~ '^[a-z][a-z0-9_]*$')
);

create table maintenance.customer_scope (
    customer_scope_id    bigint generated always as identity primary key,
    code                 text not null unique,
    name_it              text not null,
    name_en              text,
    description          text,
    synonyms             text[] not null default '{}',
    sort_order           integer not null default 100,
    is_active            boolean not null default true,
    metadata             jsonb not null default '{}'::jsonb,

    check (code ~ '^[a-z][a-z0-9_]*$')
);

create table maintenance.reason_class (
    reason_class_id      bigint generated always as identity primary key,
    code                 text not null unique,
    name_it              text not null,
    name_en              text,
    description          text,
    synonyms             text[] not null default '{}',
    sort_order           integer not null default 100,
    is_active            boolean not null default true,
    metadata             jsonb not null default '{}'::jsonb,

    check (code ~ '^[a-z][a-z0-9_]*$')
);

create table maintenance.impact_effect (
    impact_effect_id     bigint generated always as identity primary key,
    code                 text not null unique,
    name_it              text not null,
    name_en              text,
    description          text,
    synonyms             text[] not null default '{}',
    sort_order           integer not null default 100,
    is_active            boolean not null default true,
    metadata             jsonb not null default '{}'::jsonb,

    check (code ~ '^[a-z][a-z0-9_]*$')
);

create table maintenance.quality_flag (
    quality_flag_id      bigint generated always as identity primary key,
    code                 text not null unique,
    name_it              text not null,
    name_en              text,
    description          text,
    synonyms             text[] not null default '{}',
    sort_order           integer not null default 100,
    is_active            boolean not null default true,
    metadata             jsonb not null default '{}'::jsonb,

    check (code ~ '^[a-z][a-z0-9_]*$')
);

create table maintenance.target_type (
    target_type_id       bigint generated always as identity primary key,
    code                 text not null unique,
    name_it              text not null,
    name_en              text,
    description          text,
    synonyms             text[] not null default '{}',
    sort_order           integer not null default 100,
    is_active            boolean not null default true,
    metadata             jsonb not null default '{}'::jsonb,

    check (code ~ '^[a-z][a-z0-9_]*$')
);

create table maintenance.notice_channel (
    notice_channel_id    bigint generated always as identity primary key,
    code                 text not null unique,
    name_it              text not null,
    name_en              text,
    description          text,
    synonyms             text[] not null default '{}',
    sort_order           integer not null default 100,
    is_active            boolean not null default true,
    metadata             jsonb not null default '{}'::jsonb,

    check (code ~ '^[a-z][a-z0-9_]*$')
);

create table maintenance.service_taxonomy (
    service_taxonomy_id  bigint generated always as identity primary key,
    code                 text not null unique,            -- es: veeam_cloud_connect
    technical_domain_id  bigint not null references maintenance.technical_domain(technical_domain_id),
    name_it              text not null,
    name_en              text,
    description          text,
    synonyms             text[] not null default '{}',
    sort_order           integer not null default 100,
    is_active            boolean not null default true,
    metadata             jsonb not null default '{}'::jsonb,

    check (code ~ '^[a-z][a-z0-9_]*$')
);

create index idx_service_taxonomy_domain on maintenance.service_taxonomy(technical_domain_id);

insert into maintenance.technical_domain (code, name_it, name_en, sort_order)
values
    ('network', 'Network', 'Network', 10),
    ('cloud', 'Cloud', 'Cloud', 20),
    ('facility', 'Facility', 'Facility', 30),
    ('storage', 'Storage', 'Storage', 40),
    ('power', 'Power', 'Power', 50),
    ('portal', 'Portale', 'Portal', 60),
    ('backup', 'Backup', 'Backup', 70),
    ('voice', 'Voce', 'Voice', 80),
    ('firewall', 'Firewall', 'Firewall', 90),
    ('security', 'Sicurezza', 'Security', 100),
    ('other', 'Altro', 'Other', 1000)
on conflict (code) do update set
    name_it = excluded.name_it,
    name_en = excluded.name_en,
    sort_order = excluded.sort_order;

insert into maintenance.maintenance_kind (code, name_it, name_en, sort_order)
values
    ('planned', 'Programmata', 'Planned', 10),
    ('extraordinary', 'Straordinaria', 'Extraordinary', 20),
    ('emergency', 'Emergenza', 'Emergency', 30),
    ('corrective', 'Correttiva', 'Corrective', 40),
    ('preventive', 'Preventiva', 'Preventive', 50),
    ('other', 'Altro', 'Other', 1000)
on conflict (code) do update set
    name_it = excluded.name_it,
    name_en = excluded.name_en,
    sort_order = excluded.sort_order;

insert into maintenance.customer_scope (code, name_it, name_en, sort_order)
values
    ('internal', 'Interno', 'Internal', 10),
    ('single_customer', 'Singolo cliente', 'Single customer', 20),
    ('customer_subset', 'Sottoinsieme clienti', 'Customer subset', 30),
    ('all_customers', 'Tutti i clienti', 'All customers', 40),
    ('unknown', 'Sconosciuto', 'Unknown', 1000)
on conflict (code) do update set
    name_it = excluded.name_it,
    name_en = excluded.name_en,
    sort_order = excluded.sort_order;

insert into maintenance.reason_class (code, name_it, name_en, sort_order)
values
    ('topology_change', 'Cambio topologia', 'Topology change', 10),
    ('migration', 'Migrazione', 'Migration', 20),
    ('platform_upgrade', 'Aggiornamento piattaforma', 'Platform upgrade', 30),
    ('infrastructure_upgrade', 'Aggiornamento infrastruttura', 'Infrastructure upgrade', 40),
    ('network_retermination', 'Riterminazione rete', 'Network retermination', 50),
    ('security_patch', 'Patch di sicurezza', 'Security patch', 60),
    ('electrical_maintenance', 'Manutenzione elettrica', 'Electrical maintenance', 70),
    ('corrective_maintenance', 'Manutenzione correttiva', 'Corrective maintenance', 80),
    ('firmware_upgrade', 'Aggiornamento firmware', 'Firmware upgrade', 90),
    ('other', 'Altro', 'Other', 1000)
on conflict (code) do update set
    name_it = excluded.name_it,
    name_en = excluded.name_en,
    sort_order = excluded.sort_order;

insert into maintenance.impact_effect (code, name_it, name_en, sort_order)
values
    ('no_impact', 'Nessun impatto', 'No impact', 10),
    ('service_unavailable', 'Servizio indisponibile', 'Service unavailable', 20),
    ('partial_unavailability', 'Indisponibilita parziale', 'Partial unavailability', 30),
    ('degraded_service', 'Servizio degradato', 'Degraded service', 40),
    ('management_plane_unavailable', 'Piano di gestione indisponibile', 'Management plane unavailable', 50),
    ('configuration_unavailable', 'Configurazione indisponibile', 'Configuration unavailable', 60),
    ('provisioning_unavailable', 'Provisioning indisponibile', 'Provisioning unavailable', 70),
    ('other', 'Altro', 'Other', 1000)
on conflict (code) do update set
    name_it = excluded.name_it,
    name_en = excluded.name_en,
    sort_order = excluded.sort_order;

insert into maintenance.quality_flag (code, name_it, name_en, sort_order)
values
    ('unresolved_placeholder', 'Placeholder non risolto', 'Unresolved placeholder', 10),
    ('inconsistent_content', 'Contenuto incoerente', 'Inconsistent content', 20),
    ('malformed_translation', 'Traduzione malformata', 'Malformed translation', 30),
    ('ambiguous_service_scope', 'Ambito servizio ambiguo', 'Ambiguous service scope', 40),
    ('generic_subject', 'Oggetto generico', 'Generic subject', 50),
    ('excessive_boilerplate', 'Boilerplate eccessivo', 'Excessive boilerplate', 60),
    ('missing_site', 'Sito mancante', 'Missing site', 70),
    ('missing_reason', 'Motivo mancante', 'Missing reason', 80),
    ('missing_duration', 'Durata mancante', 'Missing duration', 90),
    ('missing_start', 'Inizio mancante', 'Missing start', 100),
    ('missing_end', 'Fine mancante', 'Missing end', 110),
    ('duration_text_inconsistent', 'Durata incoerente', 'Duration text inconsistent', 120),
    ('start_end_mismatch', 'Inizio e fine incoerenti', 'Start/end mismatch', 130),
    ('malformed_english_text', 'Testo inglese malformato', 'Malformed English text', 140),
    ('html_noise_present', 'Rumore HTML presente', 'HTML noise present', 150),
    ('generic_salutation', 'Saluto generico', 'Generic salutation', 160),
    ('hardcoded_customer_name', 'Nome cliente hardcoded', 'Hardcoded customer name', 170),
    ('typo_in_subject', 'Errore nell oggetto', 'Typo in subject', 180),
    ('translation_inconsistent', 'Traduzione incoerente', 'Translation inconsistent', 190),
    ('weak_customer_scope', 'Ambito clienti debole', 'Weak customer scope', 200),
    ('weak_domain_classification', 'Classificazione dominio debole', 'Weak domain classification', 210),
    ('weak_linking_candidate', 'Candidato collegamento debole', 'Weak linking candidate', 220),
    ('other', 'Altro', 'Other', 1000)
on conflict (code) do update set
    name_it = excluded.name_it,
    name_en = excluded.name_en,
    sort_order = excluded.sort_order;

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
    ('other', 'Altro', 'Other', 1000)
on conflict (code) do update set
    name_it = excluded.name_it,
    name_en = excluded.name_en,
    sort_order = excluded.sort_order;

insert into maintenance.notice_channel (code, name_it, name_en, sort_order)
values
    ('email', 'Email', 'Email', 10),
    ('portal_banner', 'Banner portale', 'Portal banner', 20),
    ('sms', 'SMS', 'SMS', 30),
    ('api', 'API', 'API', 40),
    ('other', 'Altro', 'Other', 1000)
on conflict (code) do update set
    name_it = excluded.name_it,
    name_en = excluded.name_en,
    sort_order = excluded.sort_order;

insert into maintenance.service_taxonomy (code, technical_domain_id, name_it, name_en, sort_order)
select seed.code, d.technical_domain_id, seed.name_it, seed.name_en, seed.sort_order
from (
    values
        ('point_to_point_transport', 'network', 'Trasporto punto-punto', 'Point-to-point transport', 10),
        ('virtual_machine', 'cloud', 'Macchina virtuale', 'Virtual machine', 20),
        ('veeam_cloud_connect', 'backup', 'Veeam Cloud Connect', 'Veeam Cloud Connect', 30),
        ('customer_portal', 'portal', 'Portale cliente', 'Customer portal', 40),
        ('hosting', 'cloud', 'Hosting', 'Hosting', 50),
        ('iaas', 'cloud', 'IaaS', 'IaaS', 60),
        ('voip_pbx', 'voice', 'PBX VoIP', 'VoIP PBX', 70),
        ('cloud_director', 'cloud', 'Cloud Director', 'Cloud Director', 80),
        ('electrical_infrastructure', 'power', 'Infrastruttura elettrica', 'Electrical infrastructure', 90),
        ('cloudstack', 'cloud', 'CloudStack', 'CloudStack', 100),
        ('firewall_infrastructure', 'firewall', 'Infrastruttura firewall', 'Firewall infrastructure', 110)
) as seed(code, domain_code, name_it, name_en, sort_order)
join maintenance.technical_domain d on d.code = seed.domain_code
where true
on conflict (code) do update set
    technical_domain_id = excluded.technical_domain_id,
    name_it = excluded.name_it,
    name_en = excluded.name_en,
    sort_order = excluded.sort_order;

-- =========================
-- 2) OGGETTO MANUTENZIONE
-- =========================

create table maintenance.maintenance (
    maintenance_id       bigint generated always as identity primary key,

    code                 text unique,                     -- es: MNT-2026-000123
    title_it             text not null,
    title_en             text,

    description_it       text,
    description_en       text,

    maintenance_kind_id  bigint not null references maintenance.maintenance_kind(maintenance_kind_id),
    technical_domain_id  bigint not null references maintenance.technical_domain(technical_domain_id),
    customer_scope_id    bigint not null references maintenance.customer_scope(customer_scope_id),

    status               text not null check (
                            status in ('draft','announced','approved','scheduled','in_progress','completed','cancelled','superseded')
                         ) default 'draft',

    site_id              bigint references maintenance.site(site_id),

    reason_it            text,
    reason_en            text,

    residual_service_it  text,                            -- es: "Durante la manutenzione è garantito il Ramo A"
    residual_service_en  text,

    owner_admin_id       bigint,                          -- FK verso la vostra tabella admin/users
    created_by_admin_id  bigint,
    updated_by_admin_id  bigint,

    created_at           timestamptz not null default now(),
    updated_at           timestamptz not null default now(),

    metadata             jsonb not null default '{}'::jsonb
);

create index idx_maintenance_status on maintenance.maintenance(status);
create index idx_maintenance_kind on maintenance.maintenance(maintenance_kind_id);
create index idx_maintenance_technical_domain on maintenance.maintenance(technical_domain_id);
create index idx_maintenance_customer_scope on maintenance.maintenance(customer_scope_id);
create index idx_maintenance_site on maintenance.maintenance(site_id);

create table maintenance.maintenance_service_taxonomy (
    maintenance_service_taxonomy_id bigint generated always as identity primary key,
    maintenance_id                  bigint not null references maintenance.maintenance(maintenance_id) on delete cascade,
    service_taxonomy_id             bigint not null references maintenance.service_taxonomy(service_taxonomy_id),

    source                          text not null check (
                                        source in ('manual','import','rule','ai_extracted','catalog_mapping')
                                    ) default 'manual',
    confidence                      numeric(5,4) check (confidence is null or (confidence >= 0 and confidence <= 1)),
    is_primary                      boolean not null default false,
    metadata                        jsonb not null default '{}'::jsonb,

    unique (maintenance_id, service_taxonomy_id)
);

create index idx_maintenance_service_taxonomy_maintenance on maintenance.maintenance_service_taxonomy(maintenance_id);
create index idx_maintenance_service_taxonomy_service on maintenance.maintenance_service_taxonomy(service_taxonomy_id);

create table maintenance.maintenance_reason_class (
    maintenance_reason_class_id bigint generated always as identity primary key,
    maintenance_id              bigint not null references maintenance.maintenance(maintenance_id) on delete cascade,
    reason_class_id             bigint not null references maintenance.reason_class(reason_class_id),

    source                      text not null check (
                                    source in ('manual','import','rule','ai_extracted','catalog_mapping')
                                ) default 'manual',
    confidence                  numeric(5,4) check (confidence is null or (confidence >= 0 and confidence <= 1)),
    is_primary                  boolean not null default false,
    metadata                    jsonb not null default '{}'::jsonb,

    unique (maintenance_id, reason_class_id)
);

create index idx_maintenance_reason_class_maintenance on maintenance.maintenance_reason_class(maintenance_id);
create index idx_maintenance_reason_class_reason on maintenance.maintenance_reason_class(reason_class_id);

create table maintenance.maintenance_impact_effect (
    maintenance_impact_effect_id bigint generated always as identity primary key,
    maintenance_id               bigint not null references maintenance.maintenance(maintenance_id) on delete cascade,
    impact_effect_id             bigint not null references maintenance.impact_effect(impact_effect_id),

    source                       text not null check (
                                     source in ('manual','import','rule','ai_extracted','catalog_mapping')
                                 ) default 'manual',
    confidence                   numeric(5,4) check (confidence is null or (confidence >= 0 and confidence <= 1)),
    is_primary                   boolean not null default false,
    metadata                     jsonb not null default '{}'::jsonb,

    unique (maintenance_id, impact_effect_id)
);

create index idx_maintenance_impact_effect_maintenance on maintenance.maintenance_impact_effect(maintenance_id);
create index idx_maintenance_impact_effect_effect on maintenance.maintenance_impact_effect(impact_effect_id);

create table maintenance.maintenance_quality_flag (
    maintenance_quality_flag_id bigint generated always as identity primary key,
    maintenance_id              bigint not null references maintenance.maintenance(maintenance_id) on delete cascade,
    quality_flag_id             bigint not null references maintenance.quality_flag(quality_flag_id),

    source                      text not null check (
                                    source in ('manual','import','rule','ai_extracted','catalog_mapping')
                                ) default 'manual',
    confidence                  numeric(5,4) check (confidence is null or (confidence >= 0 and confidence <= 1)),
    metadata                    jsonb not null default '{}'::jsonb,

    unique (maintenance_id, quality_flag_id)
);

create index idx_maintenance_quality_flag_maintenance on maintenance.maintenance_quality_flag(maintenance_id);
create index idx_maintenance_quality_flag_flag on maintenance.maintenance_quality_flag(quality_flag_id);

-- =========================
-- 3) FINESTRE DI MANUTENZIONE
--    consente cancellazioni/riprogrammazioni
-- =========================

create table maintenance.maintenance_window (
    maintenance_window_id        bigint generated always as identity primary key,
    maintenance_id               bigint not null references maintenance.maintenance(maintenance_id) on delete cascade,

    seq_no                       integer not null,  -- 1,2,3... per ripianificazioni successive

    window_status                text not null check (
                                    window_status in ('planned','cancelled','superseded','executed')
                                 ) default 'planned',

    scheduled_start_at           timestamptz not null,
    scheduled_end_at             timestamptz not null,

    expected_downtime_minutes    integer check (expected_downtime_minutes is null or expected_downtime_minutes >= 0),

    actual_start_at              timestamptz,
    actual_end_at                timestamptz,
    actual_downtime_minutes      integer check (actual_downtime_minutes is null or actual_downtime_minutes >= 0),

    cancellation_reason_it       text,
    cancellation_reason_en       text,

    announced_at                 timestamptz,       -- prima comunicazione di quella finestra
    last_notice_at               timestamptz,

    created_at                   timestamptz not null default now(),

    unique (maintenance_id, seq_no),

    check (scheduled_end_at > scheduled_start_at),
    check (
        actual_start_at is null
        or actual_end_at is null
        or actual_end_at > actual_start_at
    )
);

create index idx_maintenance_window_maintenance on maintenance.maintenance_window(maintenance_id);
create index idx_maintenance_window_start on maintenance.maintenance_window(scheduled_start_at);
create index idx_maintenance_window_status on maintenance.maintenance_window(window_status);

-- =========================
-- 4) EVENTI DEL CICLO DI VITA
-- =========================

create table maintenance.maintenance_event (
    maintenance_event_id         bigint generated always as identity primary key,
    maintenance_id               bigint not null references maintenance.maintenance(maintenance_id) on delete cascade,
    maintenance_window_id        bigint references maintenance.maintenance_window(maintenance_window_id) on delete set null,

    event_type                   text not null check (
                                    event_type in (
                                        'created',
                                        'classified',
                                        'announced',
                                        'reminder_sent',
                                        'rescheduled',
                                        'cancelled',
                                        'started',
                                        'updated',
                                        'completed',
                                        'analysis_enriched',
                                        'impact_recomputed'
                                    )
                                 ),

    actor_type                   text not null check (
                                    actor_type in ('user','system','ai','import')
                                 ),

    actor_admin_id               bigint,
    event_at                     timestamptz not null default now(),

    summary                      text,
    payload                      jsonb not null default '{}'::jsonb
);

create index idx_maintenance_event_maintenance on maintenance.maintenance_event(maintenance_id);
create index idx_maintenance_event_window on maintenance.maintenance_event(maintenance_window_id);
create index idx_maintenance_event_type on maintenance.maintenance_event(event_type);
create index idx_maintenance_event_at on maintenance.maintenance_event(event_at);

-- =========================
-- 5) TARGET / OGGETTI IMPATTATI
--    layer generico, senza imporre subito una CMDB
-- =========================

create table maintenance.maintenance_target (
    maintenance_target_id        bigint generated always as identity primary key,
    maintenance_id               bigint not null references maintenance.maintenance(maintenance_id) on delete cascade,

    target_type_id               bigint not null references maintenance.target_type(target_type_id),

    ref_table                    text,              -- es: products, orders, customers, assets
    ref_id                       bigint,            -- id nella tabella sorgente, se esiste
    external_key                 text,              -- es: seriale, codice servizio, SN, UUID

    display_name                 text not null,
    source                       text not null check (
                                    source in ('manual','import','rule','ai_extracted','catalog_mapping')
                                 ) default 'manual',

    confidence                   numeric(5,4) check (confidence is null or (confidence >= 0 and confidence <= 1)),
    is_primary                   boolean not null default false,
    metadata                     jsonb not null default '{}'::jsonb
);

create index idx_maintenance_target_maintenance on maintenance.maintenance_target(maintenance_id);
create index idx_maintenance_target_type on maintenance.maintenance_target(target_type_id);
create index idx_maintenance_target_ref on maintenance.maintenance_target(ref_table, ref_id);

-- =========================
-- 6) CLIENTI IMPATTATI DERIVATI
--    risultato del motore di impatto
-- =========================

create table maintenance.maintenance_impacted_customer (
    maintenance_impacted_customer_id  bigint generated always as identity primary key,
    maintenance_id                    bigint not null references maintenance.maintenance(maintenance_id) on delete cascade,

    customer_id                       bigint not null,  -- FK futura verso anagrafica clienti
    order_id                          bigint,
    service_id                        bigint,

    impact_scope                      text not null check (
                                         impact_scope in ('direct','indirect','possible')
                                      ) default 'possible',

    derivation_source                 text not null check (
                                         derivation_source in ('manual','rule','ai','hybrid')
                                      ) default 'rule',

    confidence                        numeric(5,4) check (confidence is null or (confidence >= 0 and confidence <= 1)),
    reason                            text,
    metadata                          jsonb not null default '{}'::jsonb,

    created_at                        timestamptz not null default now()
);

create index idx_impacted_customer_maintenance on maintenance.maintenance_impacted_customer(maintenance_id);
create index idx_impacted_customer_customer on maintenance.maintenance_impacted_customer(customer_id);
create index idx_impacted_customer_order on maintenance.maintenance_impacted_customer(order_id);

-- =========================
-- 7) COMUNICAZIONI / NOTIFICHE
-- =========================

create table maintenance.notice (
    notice_id                     bigint generated always as identity primary key,
    maintenance_id                bigint not null references maintenance.maintenance(maintenance_id) on delete cascade,
    maintenance_window_id         bigint references maintenance.maintenance_window(maintenance_window_id) on delete set null,

    notice_type                   text not null check (
                                    notice_type in (
                                        'announcement',
                                        'reminder',
                                        'reschedule',
                                        'cancellation',
                                        'start',
                                        'completion',
                                        'internal_update'
                                    )
                                 ),

    audience                      text not null check (
                                    audience in ('internal','external')
                                 ),

    notice_channel_id             bigint not null references maintenance.notice_channel(notice_channel_id),

    template_code                 text,              -- es: external_power_announcement_v1
    template_version              integer,

    generation_source             text not null check (
                                    generation_source in ('manual','ai','system','import')
                                 ) default 'manual',

    send_status                   text not null check (
                                    send_status in ('draft','ready','sent','failed','suppressed')
                                 ) default 'draft',

    scheduled_send_at             timestamptz,
    sent_at                       timestamptz,

    created_by_admin_id           bigint,
    created_at                    timestamptz not null default now(),

    metadata                      jsonb not null default '{}'::jsonb
);

create index idx_notice_maintenance on maintenance.notice(maintenance_id);
create index idx_notice_window on maintenance.notice(maintenance_window_id);
create index idx_notice_type on maintenance.notice(notice_type);
create index idx_notice_channel on maintenance.notice(notice_channel_id);
create index idx_notice_send_status on maintenance.notice(send_status);

create table maintenance.notice_locale (
    notice_locale_id              bigint generated always as identity primary key,
    notice_id                     bigint not null references maintenance.notice(notice_id) on delete cascade,

    locale                        text not null check (locale in ('it','en')),
    subject                       text not null,
    body_html                     text,
    body_text                     text,

    unique (notice_id, locale)
);

create table maintenance.notice_quality_flag (
    notice_quality_flag_id       bigint generated always as identity primary key,
    notice_id                    bigint not null references maintenance.notice(notice_id) on delete cascade,
    quality_flag_id              bigint not null references maintenance.quality_flag(quality_flag_id),

    source                       text not null check (
                                    source in ('manual','import','rule','ai_extracted','catalog_mapping')
                                 ) default 'manual',
    confidence                   numeric(5,4) check (confidence is null or (confidence >= 0 and confidence <= 1)),
    metadata                     jsonb not null default '{}'::jsonb,

    unique (notice_id, quality_flag_id)
);

create index idx_notice_quality_flag_notice on maintenance.notice_quality_flag(notice_id);
create index idx_notice_quality_flag_flag on maintenance.notice_quality_flag(quality_flag_id);

create view maintenance.v_current_window as
select distinct on (mw.maintenance_id)
    mw.maintenance_id,
    mw.maintenance_window_id,
    mw.seq_no,
    mw.window_status,
    mw.scheduled_start_at,
    mw.scheduled_end_at,
    mw.expected_downtime_minutes
from maintenance.maintenance_window mw
order by mw.maintenance_id, mw.seq_no desc;
