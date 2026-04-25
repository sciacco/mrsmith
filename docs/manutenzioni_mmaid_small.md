# Schema ER compatto — maintenance

```mermaid
erDiagram
    %% ===== LOOKUP TABLES =====
    technical_domain {
        bigint PK "technical_domain_id"
        text UK "code"
    }
    maintenance_kind {
        bigint PK "maintenance_kind_id"
        text UK "code"
    }
    customer_scope {
        bigint PK "customer_scope_id"
        text UK "code"
    }
    reason_class {
        bigint PK "reason_class_id"
        text UK "code"
    }
    impact_effect {
        bigint PK "impact_effect_id"
        text UK "code"
    }
    quality_flag {
        bigint PK "quality_flag_id"
        text UK "code"
    }
    target_type {
        bigint PK "target_type_id"
        text UK "code"
    }
    notice_channel {
        bigint PK "notice_channel_id"
        text UK "code"
    }

    %% ===== SERVICE TAXONOMY =====
    service_taxonomy {
        bigint PK "service_taxonomy_id"
        text UK "code"
        bigint FK "technical_domain_id"
        bigint FK "target_type_id"
    }
    service_dependency {
        bigint PK "service_dependency_id"
        bigint FK "upstream_service_id"
        bigint FK "downstream_service_id"
    }
    llm_model {
        text PK "scope"
    }

    %% ===== CORE TABLES =====
    site {
        bigint PK "site_id"
        text "code"
        bigint FK "owner_maintenance_id"FK "maintenance"
    }
    maintenance {
        bigint PK "maintenance_id"
        text UK "code"
        bigint FK "maintenance_kind_id"
        bigint FK "technical_domain_id"
        bigint FK "customer_scope_id"FK "site_id"
    }

    %% ===== M-N BRIDGE TABLES =====
    maintenance_service_taxonomy {
        bigint PK
        bigint FK "maintenance_id"
        bigint FK "service_taxonomy_id"
    }
    maintenance_reason_class {
        bigint PK
        bigint FK "maintenance_id"
        bigint FK "reason_class_id"
    }
    maintenance_impact_effect {
        bigint PK
        bigint FK "maintenance_id"
        bigint FK "impact_effect_id"
    }
    maintenance_quality_flag {
        bigint PK
        bigint FK "maintenance_id"
        bigint FK "quality_flag_id"
    }

    %% ===== WINDOWS & EVENTS =====
    maintenance_window {
        bigint PK "maintenance_window_id"
        bigint FK "maintenance_id"
        integer "seq_no"
    }
    maintenance_event {
        bigint PK "maintenance_event_id"
        bigint FK "maintenance_id"
        bigint FK "maintenance_window_id"
    }

    %% ===== TARGETS =====
    maintenance_target {
        bigint PK "maintenance_target_id"
        bigint FK "maintenance_id"
        bigint FK "target_type_id"
        bigint FK "service_taxonomy_id"
    }
    maintenance_impacted_customer {
        bigint PK
        bigint FK "maintenance_id"
    }

    %% ===== NOTICES =====
    notice {
        bigint PK "notice_id"
        bigint FK "maintenance_id"
        bigint FK "maintenance_window_id"
        bigint FK "notice_channel_id"
    }
    notice_locale {
        bigint PK
        bigint FK "notice_id"
    }
    notice_quality_flag {
        bigint PK
        bigint FK "notice_id"
        bigint FK "quality_flag_id"
    }

    %% ===== RELATIONSHIPS =====
    maintenance }o--|| maintenance_kind : ""
    maintenance }o--|| technical_domain : ""
    maintenance }o--o| customer_scope : ""
    maintenance }o--o| site : ""
    maintenance }o--o| llm_model : ""

    site }o--o| maintenance : "owner_maintenance"

    maintenance ||--o{ maintenance_window : ""
    maintenance ||--o{ maintenance_event : ""
    maintenance ||--o{ maintenance_service_taxonomy : ""
    maintenance ||--o{ maintenance_reason_class : ""
    maintenance ||--o{ maintenance_impact_effect : ""
    maintenance ||--o{ maintenance_quality_flag : ""
    maintenance ||--o{ maintenance_target : ""
    maintenance ||--o{ maintenance_impacted_customer : ""
    maintenance ||--o{ notice : ""

    service_taxonomy }o--|| technical_domain : ""
    service_taxonomy }o--|| target_type : ""
    service_taxonomy ||--o{ service_dependency : "upstream"
    service_taxonomy ||--o{ service_dependency : "downstream"

    maintenance_service_taxonomy }o--|| service_taxonomy : ""
    maintenance_reason_class }o--|| reason_class : ""
    maintenance_impact_effect }o--|| impact_effect : ""
    maintenance_quality_flag }o--|| quality_flag : ""

    maintenance_event }o--o| maintenance_window : ""
    maintenance_target }o--|| target_type : ""
    maintenance_target }o--o| service_taxonomy : ""

    notice }o--o| maintenance_window : ""
    notice }o--|| notice_channel : ""
    notice_locale }o--|| notice : ""
    notice_quality_flag }o--|| notice : ""
    notice_quality_flag }o--|| quality_flag : ""
```

---

## Schema riassuntivo

```
LOOKUP TABLES (8 tabelle codice/descrizione)
├── technical_domain ──────────────────────┐
├── maintenance_kind ──────────────────────┤
├── customer_scope ────────────────────────┤
├── reason_class ─────────────────────────┤
├── impact_effect ────────────────────────┤
├── quality_flag ──────────────────────────┤
├── target_type ──────────────────────────┤
└── notice_channel ───────────────────────┘

SERVICE TAXONOMY (catalogo servizi + dipendenze)
├── service_taxonomy ────── FK → technical_domain
│                          FK → target_type
├── service_dependency ─── FK ×2 → service_taxonomy (upstream/downstream)
└── llm_model ──────────── configurazione IA

CORE
├── site ─────────────────── FK → maintenance (owner_maintenance_id, nullable)
└── maintenance ──────────── FK → maintenance_kind
                             FK → technical_domain
                             FK → customer_scope (nullable per draft/cancelled)
                             FK → site (nullable)

BRIDGE TABLES (classificazioni M-N con attributi)
├── maintenance_service_taxonomy ─ FK → maintenance + service_taxonomy
├── maintenance_reason_class ───── FK → maintenance + reason_class
├── maintenance_impact_effect ──── FK → maintenance + impact_effect
└── maintenance_quality_flag ───── FK → maintenance + quality_flag

WINDOWS & EVENTS
├── maintenance_window ──── FK → maintenance (cascade)
│                           seq_no per ripianificazioni
└── maintenance_event ───── FK → maintenance (cascade)
                             FK → maintenance_window (set null)

TARGETS
├── maintenance_target ──── FK → maintenance (cascade)
│                           FK → target_type
│                           FK → service_taxonomy (nullable)
│                           ref_table/ref_id per integrazione CMDB
└── maintenance_impacted_customer ─ FK → maintenance (cascade)

NOTICES
├── notice ───────────────── FK → maintenance (cascade)
│                           FK → maintenance_window (nullable)
│                           FK → notice_channel
├── notice_locale ────────── FK → notice (cascade)
│                           locale: it/en
└── notice_quality_flag ──── FK → notice (cascade)
                             FK → quality_flag
```

---

## Legenda cardinalità

| Simbolo | Significato |
|---------|-------------|
| `||` | esattamente uno (one) |
| `o|` | zero o uno (zero-or-one) |
| `}|` | uno o molti (one-or-many) |
| `o}|` | zero o molti (zero-or-many) |
| `o--` | relazione (nessuna cardinalità forzata) |
