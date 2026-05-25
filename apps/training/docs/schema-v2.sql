-- =============================================================================
-- CDLAN Training Management Tool — Data Model
-- Target: PostgreSQL 16+
-- Version: v2
--
-- Changelog v1 -> v2:
--   - Q3 (mandatory training): course.is_compliance_course, recurrence_interval,
--     compliance_framework + tabella mandatory_assignment_rule
--   - Q4 (learning paths): tabelle learning_path, learning_path_step,
--     employee_learning_path (opzionali, non bloccanti per il go-live)
--   - Q6 (employee requests): tabella training_request (wishlist/suggerimenti
--     dei dipendenti, separati dalle enrollment ufficiali)
--   - Q1, Q2: nessuna modifica (no perimetro esterni, no workflow approvazione)
--   - Q5, Q7: TBD (zero impatto schema)
-- =============================================================================

CREATE EXTENSION IF NOT EXISTS pgcrypto;     -- gen_random_uuid()
CREATE EXTENSION IF NOT EXISTS citext;       -- case-insensitive text

CREATE SCHEMA IF NOT EXISTS training;
SET search_path TO training, public;

-- -----------------------------------------------------------------------------
-- ENUMS — stati e tassonomie chiuse
-- -----------------------------------------------------------------------------

CREATE TYPE employee_status        AS ENUM ('active', 'on_leave', 'terminated');
CREATE TYPE course_delivery_mode   AS ENUM ('classroom', 'online_live', 'online_self', 'on_the_job', 'mixed');
CREATE TYPE course_provider_kind   AS ENUM ('internal', 'external');

-- Stati di un'iscrizione (lifecycle). Le transizioni le enforciamo lato applicativo.
-- proposed   = inserita nel piano, non ancora approvata
-- approved   = approvata da People, non ancora iniziata
-- in_progress= il dipendente la sta facendo
-- completed  = conclusa con successo (frequenza/esame come da course)
-- failed     = conclusa ma esame non superato (es. "CCNP BOCCIATO ESAME")
-- cancelled  = annullata prima del termine
-- expired    = pianificata ma mai partita entro l'anno di budget
CREATE TYPE enrollment_status      AS ENUM
  ('proposed', 'approved', 'in_progress', 'completed', 'failed', 'cancelled', 'expired');

-- Esito al conseguimento certificazione
CREATE TYPE award_outcome          AS ENUM
  ('passed_exam',         -- certificazione vera e propria
   'attendance_only');    -- solo attestato di frequenza, no esame

-- Provenienza del dato: utile per audit + per dare "peso" diverso a un
-- self-assessment vs un attestato verificato (vedi colonne "Da Survey" / "A voce")
CREATE TYPE validation_source      AS ENUM
  ('document_verified',   -- attestato caricato e validato
   'declared_survey',     -- dichiarato in survey
   'declared_verbal',     -- dichiarato a voce
   'declared_cv',         -- estratto da CV
   'imported_legacy');    -- importato da Excel o sorgenti legacy, non verificato

CREATE TYPE plan_status            AS ENUM ('draft', 'open', 'frozen', 'closed');

-- -----------------------------------------------------------------------------
-- ANAGRAFICHE (master data)
-- -----------------------------------------------------------------------------

-- Team / area aziendale (APPLICATIONS, CLOUD, DC, PEOPLE, ...).
-- Tabella, non enum: People li gestisce senza migration.
CREATE TABLE team (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code            text NOT NULL UNIQUE,                      -- 'APPLICATIONS'
    name            text NOT NULL,                             -- 'Applications team'
    description     text,
    is_active       boolean NOT NULL DEFAULT true,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);
COMMENT ON TABLE  team IS 'Team / area aziendale (sostituisce la colonna TEAM dell''Excel).';

-- Dipendente. Anagrafica locale alimentata da connettori esterni fuori scope.
CREATE TABLE employee (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id     text,                                      -- id opzionale gestito dal connettore esterno
    first_name      text NOT NULL,
    last_name       text NOT NULL,
    email           citext NOT NULL,
    hire_date       date,
    termination_date date,
    status          employee_status NOT NULL DEFAULT 'active',
    manager_id      uuid REFERENCES employee(id) ON DELETE SET NULL,
    notes           text,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (email)
);
CREATE UNIQUE INDEX idx_employee_external_id ON employee(external_id) WHERE external_id IS NOT NULL;
COMMENT ON COLUMN employee.external_id IS 'ID opzionale gestito da connettori esterni; Training non implementa sync HR.';

-- Membership: un dipendente può appartenere a più team nel tempo.
-- Con tstzrange + exclusion constraint impediamo overlap sullo stesso (employee, team).
CREATE TABLE team_membership (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    employee_id     uuid NOT NULL REFERENCES employee(id) ON DELETE CASCADE,
    team_id         uuid NOT NULL REFERENCES team(id),
    role            text,                                      -- 'member', 'lead', ...
    start_date      timestamptz NOT NULL DEFAULT now(),
    end_date        timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_team_membership_active_uniq
  ON team_membership (employee_id, team_id)
  WHERE (end_date IS NULL);
COMMENT ON TABLE team_membership IS 'Storico appartenenza dipendente-team con range temporale.';

-- Fornitore di formazione (Linux Foundation, eForHum, POLIMI GSoM, ...)
CREATE TABLE vendor (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name            text NOT NULL,
    name_normalized citext NOT NULL,                           -- per dedup, niente "EXTRAORDY" vs "Extraordy"
    website         text,
    notes           text,
    is_active       boolean NOT NULL DEFAULT true,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (name_normalized)
);
COMMENT ON COLUMN vendor.name_normalized IS 'Versione normalizzata case-insensitive per evitare duplicati.';

-- Area formativa / skill (Kubernetes, Linux, CCNA, O365, BPR, AI, ...).
-- Self-referencing per gerarchia opzionale (es. CCNA <- Networking).
CREATE TABLE skill_area (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code            text NOT NULL UNIQUE,                      -- 'KUBERNETES', 'CCNA'
    name            text NOT NULL,
    parent_id       uuid REFERENCES skill_area(id) ON DELETE SET NULL,
    description     text,
    is_active       boolean NOT NULL DEFAULT true,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

-- Catalogo certificazioni (CCNA 200-301, VMCE, RHCSA, ...).
-- Separato dal corso: una certificazione può essere ottenuta tramite più corsi.
CREATE TABLE certification (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code               text NOT NULL UNIQUE,                   -- 'CCNA-200-301'
    name               text NOT NULL,                          -- 'Cisco Certified Network Associate'
    issuer_vendor_id   uuid REFERENCES vendor(id),             -- chi emette (Cisco, RedHat, ...)
    skill_area_id      uuid REFERENCES skill_area(id),
    typical_validity   interval,                               -- es. '3 years'. NULL = senza scadenza.
    description        text,
    is_active          boolean NOT NULL DEFAULT true,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now()
);
COMMENT ON COLUMN certification.typical_validity IS 'Validità tipica (es. 3 anni). La data effettiva sta su certification_award.';

-- Catalogo corsi (istruzione, non istanza).
CREATE TABLE course (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    title               text NOT NULL,
    vendor_id           uuid REFERENCES vendor(id),
    skill_area_id       uuid REFERENCES skill_area(id),
    leads_to_cert_id    uuid REFERENCES certification(id),     -- se il corso prepara a una cert specifica
    delivery_mode       course_delivery_mode NOT NULL DEFAULT 'mixed',
    provider_kind       course_provider_kind NOT NULL DEFAULT 'external',
    default_hours       integer CHECK (default_hours IS NULL OR default_hours > 0),
    default_cost        numeric(10,2) CHECK (default_cost IS NULL OR default_cost >= 0),
    course_url          text,
    description         text,
    -- Q3: formazione obbligatoria
    is_compliance_course boolean NOT NULL DEFAULT false,
    recurrence_interval interval,                              -- es. '3 years' (antincendio), '1 year' (GDPR)
    compliance_framework text,                                 -- 'D.Lgs. 81/08', 'GDPR', 'ISO 27001', ...
    is_active           boolean NOT NULL DEFAULT true,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);
COMMENT ON COLUMN course.default_hours IS 'Ore di default; sovrascrivibili sulla singola enrollment.';
COMMENT ON COLUMN course.default_cost  IS 'Costo di listino; il consuntivo va su enrollment.cost_actual.';
COMMENT ON COLUMN course.is_compliance_course IS 'Se TRUE, il corso è collegato a un framework compliance. L''obbligatorietà per persona deriva dalle regole.';
COMMENT ON COLUMN course.recurrence_interval IS 'Frequenza con cui va ripetuto (es. 3 anni per antincendio). NULL = una tantum.';

-- -----------------------------------------------------------------------------
-- PIANIFICAZIONE E ATTIVITÀ
-- -----------------------------------------------------------------------------

-- Piano formativo annuale (corrisponde al file "PROPOSTA FORMAZIONE 2026").
CREATE TABLE training_plan (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    year            integer NOT NULL UNIQUE CHECK (year BETWEEN 2020 AND 2100),
    status          plan_status NOT NULL DEFAULT 'draft',
    budget_total    numeric(12,2) CHECK (budget_total IS NULL OR budget_total >= 0),
    opened_at       timestamptz,
    frozen_at       timestamptz,
    closed_at       timestamptz,
    notes           text,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

-- Iscrizione: istanza di corso per un dipendente in un piano.
-- È l'entità centrale e quella più "calda" (write-heavy durante la pianificazione).
CREATE TABLE enrollment (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),

    employee_id         uuid NOT NULL REFERENCES employee(id),
    course_id           uuid NOT NULL REFERENCES course(id),
    training_plan_id    uuid NOT NULL REFERENCES training_plan(id),

    status              enrollment_status NOT NULL DEFAULT 'proposed',

    -- Gap analysis (livelli 0..5 tipici, scala definita applicativamente)
    priority            smallint CHECK (priority IS NULL OR priority BETWEEN 1 AND 5),
    level_as_is         smallint CHECK (level_as_is IS NULL OR level_as_is BETWEEN 0 AND 5),
    level_to_be         smallint CHECK (level_to_be IS NULL OR level_to_be BETWEEN 0 AND 5),

    -- Pianificazione vs consuntivo
    planned_start       date,
    planned_end         date,
    actual_start        date,
    actual_end          date,
    hours_planned       integer CHECK (hours_planned IS NULL OR hours_planned > 0),
    hours_actual        integer CHECK (hours_actual IS NULL OR hours_actual >= 0),
    cost_planned        numeric(10,2) CHECK (cost_planned IS NULL OR cost_planned >= 0),
    cost_actual         numeric(10,2) CHECK (cost_actual IS NULL OR cost_actual >= 0),

    -- Snapshot "as-of" del catalogo: una volta che status diventa terminale (completed/failed/expired/cancelled),
    -- valorizziamo questi campi lato applicativo per disaccoppiare lo storico dal catalogo.
    course_title_snapshot   text,
    vendor_name_snapshot    text,

    motivation          text,                                  -- "Motivazione > Obiettivo" dell'Excel
    objective           text,                                  -- "Obiettivo formativo"
    notes               text,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),

    CHECK (planned_end IS NULL OR planned_start IS NULL OR planned_end >= planned_start),
    CHECK (actual_end  IS NULL OR actual_start  IS NULL OR actual_end  >= actual_start),
    CHECK (level_to_be IS NULL OR level_as_is IS NULL OR level_to_be >= level_as_is)
);

CREATE INDEX idx_enrollment_employee     ON enrollment(employee_id);
CREATE INDEX idx_enrollment_plan         ON enrollment(training_plan_id);
CREATE INDEX idx_enrollment_course       ON enrollment(course_id);
-- Indice parziale per le query "cosa sta attualmente succedendo"
CREATE INDEX idx_enrollment_active       ON enrollment(employee_id)
  WHERE status IN ('proposed', 'approved', 'in_progress');

-- -----------------------------------------------------------------------------
-- CERTIFICAZIONI CONSEGUITE
-- -----------------------------------------------------------------------------

-- Conseguimento di una certificazione da parte di un dipendente.
-- Può essere collegato a un'enrollment (se ottenuta tramite tool) oppure no
-- (per le certificazioni storiche importate da Excel/CV).
CREATE TABLE certification_award (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),

    employee_id         uuid NOT NULL REFERENCES employee(id),
    certification_id    uuid NOT NULL REFERENCES certification(id),
    enrollment_id       uuid REFERENCES enrollment(id),         -- NULL = importata o ottenuta fuori dal tool

    outcome             award_outcome NOT NULL,
    awarded_on          date NOT NULL,
    expires_on          date,                                   -- NULL = non scade
    -- Range derivato per query temporali efficienti
    validity            daterange GENERATED ALWAYS AS
                            (daterange(awarded_on, expires_on, '[)')) STORED,

    validation_source   validation_source NOT NULL DEFAULT 'document_verified',
    external_credential_id text,                                -- es. ID credenziale Cisco/Microsoft
    external_credential_url text,
    notes               text,                                   -- "scaduta", "DA CARICARE IN FACTORIAL", ecc.
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),

    CHECK (expires_on IS NULL OR expires_on > awarded_on)
);

CREATE INDEX idx_cert_award_employee      ON certification_award(employee_id);
CREATE INDEX idx_cert_award_certification ON certification_award(certification_id);
-- Indice GIST sul range per query tipo "chi ha la cert X valida a una certa data?"
CREATE INDEX idx_cert_award_validity      ON certification_award USING gist (validity);
-- Indice parziale "certificazioni attualmente valide"
CREATE INDEX idx_cert_award_current
  ON certification_award(employee_id, certification_id)
  WHERE outcome = 'passed_exam'
    AND (expires_on IS NULL OR expires_on > CURRENT_DATE);

-- -----------------------------------------------------------------------------
-- SELF-ASSESSMENT (AS IS)
-- -----------------------------------------------------------------------------

-- Rilevazione livello AS IS di un dipendente su un'area skill (storicizzata).
CREATE TABLE skill_assessment (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    employee_id         uuid NOT NULL REFERENCES employee(id),
    skill_area_id       uuid NOT NULL REFERENCES skill_area(id),
    level               smallint NOT NULL CHECK (level BETWEEN 0 AND 5),
    assessed_on         date NOT NULL DEFAULT CURRENT_DATE,
    source              validation_source NOT NULL DEFAULT 'declared_survey',
    notes               text,
    created_at          timestamptz NOT NULL DEFAULT now(),
    UNIQUE (employee_id, skill_area_id, assessed_on)
);
CREATE INDEX idx_assessment_employee_area ON skill_assessment(employee_id, skill_area_id, assessed_on DESC);

-- -----------------------------------------------------------------------------
-- DOCUMENTI (attestati, certificati, ...)
-- -----------------------------------------------------------------------------

CREATE TABLE document (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    -- Polimorfismo via FK opzionali: il documento appartiene a UNA delle due entità.
    enrollment_id       uuid REFERENCES enrollment(id) ON DELETE CASCADE,
    certification_award_id uuid REFERENCES certification_award(id) ON DELETE CASCADE,

    filename            text NOT NULL,
    storage_key         text NOT NULL,                          -- key su object storage (es. OceanStor Pacific)
    sha256              text NOT NULL,
    mime                text NOT NULL,
    size_bytes          bigint NOT NULL CHECK (size_bytes >= 0),

    uploaded_by         uuid REFERENCES employee(id),
    uploaded_at         timestamptz NOT NULL DEFAULT now(),
    is_validated        boolean NOT NULL DEFAULT false,
    validated_by        uuid REFERENCES employee(id),
    validated_at        timestamptz,

    -- Esattamente uno tra enrollment_id e certification_award_id deve essere valorizzato
    CHECK (
      (enrollment_id IS NOT NULL)::int +
      (certification_award_id IS NOT NULL)::int = 1
    ),
    UNIQUE (sha256, storage_key)
);
CREATE INDEX idx_document_enrollment ON document(enrollment_id) WHERE enrollment_id IS NOT NULL;
CREATE INDEX idx_document_award      ON document(certification_award_id) WHERE certification_award_id IS NOT NULL;

-- -----------------------------------------------------------------------------
-- FORMAZIONE OBBLIGATORIA (Q3) — regole di assegnazione automatica
-- -----------------------------------------------------------------------------

-- Regola: "il corso X è obbligatorio per il team Y (o per tutti)".
-- Un job notturno applicativo legge queste regole, controlla chi non ha
-- una certification_award valida per il corso, e genera enrollment in stato 'proposed'.
CREATE TABLE mandatory_assignment_rule (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    course_id       uuid NOT NULL REFERENCES course(id) ON DELETE CASCADE,
    team_id         uuid REFERENCES team(id) ON DELETE CASCADE,   -- NULL = tutti i dipendenti attivi
    role_filter     text,                                          -- testo libero per ora ('preposto', 'RLS', ...)
    is_active       boolean NOT NULL DEFAULT true,
    notes           text,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    -- Non duplicare la stessa regola attiva
    UNIQUE (course_id, team_id, role_filter)
);
COMMENT ON TABLE mandatory_assignment_rule IS 'Regole di obbligatorietà: il job di compliance le interroga periodicamente.';

-- -----------------------------------------------------------------------------
-- RICHIESTE FORMAZIONE DAI DIPENDENTI (Q6) — wishlist / suggerimenti
-- -----------------------------------------------------------------------------

-- Suggerimento di un dipendente: corso a catalogo o richiesta a testo libero.
-- NON è un'enrollment: vive in una coda separata fino a triage da parte di People.
-- Se accettata, viene convertita in enrollment e il link viene mantenuto.
CREATE TABLE training_request (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    employee_id     uuid NOT NULL REFERENCES employee(id),
    -- Il dipendente può puntare a un corso esistente OPPURE descrivere un'esigenza nuova
    course_id       uuid REFERENCES course(id),
    free_text_title text,
    skill_area_id   uuid REFERENCES skill_area(id),
    motivation      text NOT NULL,
    desired_year    integer CHECK (desired_year IS NULL OR desired_year BETWEEN 2024 AND 2100),
    status          text NOT NULL DEFAULT 'submitted'
                    CHECK (status IN ('submitted','under_review','accepted','rejected','converted')),
    converted_to_enrollment_id uuid REFERENCES enrollment(id),
    review_notes    text,
    reviewed_by     uuid REFERENCES employee(id),
    reviewed_at     timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    -- Almeno uno tra corso a catalogo e testo libero deve essere presente
    CHECK (course_id IS NOT NULL OR free_text_title IS NOT NULL),
    -- Se convertita, deve esistere il link a enrollment
    CHECK (status <> 'converted' OR converted_to_enrollment_id IS NOT NULL)
);
CREATE INDEX idx_training_request_employee ON training_request(employee_id);
CREATE INDEX idx_training_request_open     ON training_request(status, created_at DESC)
  WHERE status IN ('submitted', 'under_review');

-- -----------------------------------------------------------------------------
-- PERCORSI FORMATIVI PLURIENNALI (Q4) — opzionali, nice-to-have
-- -----------------------------------------------------------------------------

-- Percorso strutturato (es. "Senior Network Engineer track").
-- Non vincola il budget annuale, è solo guida di sviluppo professionale.
CREATE TABLE learning_path (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code            text NOT NULL UNIQUE,                      -- 'NETENG-SENIOR'
    name            text NOT NULL,                             -- 'Senior Network Engineer track'
    skill_area_id   uuid REFERENCES skill_area(id),
    description     text,
    is_active       boolean NOT NULL DEFAULT true,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

-- Singolo step di un percorso: può puntare a un corso, a una certificazione, o entrambi.
CREATE TABLE learning_path_step (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    path_id          uuid NOT NULL REFERENCES learning_path(id) ON DELETE CASCADE,
    step_order       smallint NOT NULL,
    course_id        uuid REFERENCES course(id),
    certification_id uuid REFERENCES certification(id),
    is_required      boolean NOT NULL DEFAULT true,
    notes            text,
    UNIQUE (path_id, step_order),
    CHECK (course_id IS NOT NULL OR certification_id IS NOT NULL)
);

-- Iscrizione di un dipendente a un percorso (tracciamento progresso).
-- Il "progresso" si calcola lato applicativo confrontando learning_path_step
-- con enrollment.completed / certification_award.passed_exam del dipendente.
CREATE TABLE employee_learning_path (
    employee_id      uuid NOT NULL REFERENCES employee(id) ON DELETE CASCADE,
    path_id          uuid NOT NULL REFERENCES learning_path(id),
    started_on       date NOT NULL DEFAULT CURRENT_DATE,
    target_completion date,
    completed_on     date,
    notes            text,
    PRIMARY KEY (employee_id, path_id),
    CHECK (target_completion IS NULL OR target_completion >= started_on),
    CHECK (completed_on IS NULL OR completed_on >= started_on)
);

-- -----------------------------------------------------------------------------
-- AUDIT LOG (append-only)
-- -----------------------------------------------------------------------------

CREATE TABLE audit_log (
    id              bigserial PRIMARY KEY,
    occurred_at     timestamptz NOT NULL DEFAULT now(),
    actor_id        uuid REFERENCES employee(id),               -- chi ha fatto l'azione (NULL = sistema/sync)
    entity_type     text NOT NULL,                              -- 'enrollment', 'certification_award', ...
    entity_id       uuid NOT NULL,
    action          text NOT NULL,                              -- 'created', 'updated', 'status_changed', ...
    before_state    jsonb,
    after_state     jsonb,
    correlation_id  uuid                                        -- per raggruppare azioni della stessa transazione
);
CREATE INDEX idx_audit_entity ON audit_log(entity_type, entity_id, occurred_at DESC);
CREATE INDEX idx_audit_actor  ON audit_log(actor_id, occurred_at DESC);
COMMENT ON TABLE audit_log IS 'Log append-only. Mai UPDATE/DELETE. Popolato da trigger applicativo o DB.';

-- -----------------------------------------------------------------------------
-- TRIGGER UTILITY — updated_at
-- -----------------------------------------------------------------------------

CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
  NEW.updated_at := now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DO $$
DECLARE t text;
BEGIN
  FOR t IN
    SELECT tablename FROM pg_tables
    WHERE schemaname = 'training'
      AND tablename IN ('team','employee','vendor','skill_area','certification',
                        'course','training_plan','enrollment','certification_award',
                        'mandatory_assignment_rule','training_request','learning_path')
  LOOP
    EXECUTE format(
      'CREATE TRIGGER trg_%I_updated_at BEFORE UPDATE ON %I
       FOR EACH ROW EXECUTE FUNCTION set_updated_at()', t, t);
  END LOOP;
END $$;

-- -----------------------------------------------------------------------------
-- VISTE PER REPORTING (le query più comuni dell'Excel diventano viste)
-- -----------------------------------------------------------------------------

-- "Foglio Certificazioni" — chi ha cosa, valida o no
CREATE OR REPLACE VIEW v_employee_certifications AS
SELECT
  e.id                AS employee_id,
  e.last_name,
  e.first_name,
  c.code              AS cert_code,
  c.name              AS cert_name,
  ca.outcome,
  ca.awarded_on,
  ca.expires_on,
  CASE
    WHEN ca.expires_on IS NULL                 THEN 'valid_no_expiry'
    WHEN ca.expires_on > CURRENT_DATE          THEN 'valid'
    ELSE 'expired'
  END                 AS current_status,
  ca.validation_source
FROM certification_award ca
JOIN employee e      ON e.id = ca.employee_id
JOIN certification c ON c.id = ca.certification_id;

-- "Per budget" — consuntivo del piano annuale
CREATE OR REPLACE VIEW v_plan_budget AS
SELECT
  tp.year,
  t.code              AS team_code,
  COUNT(*)            AS enrollments_count,
  SUM(COALESCE(en.cost_actual, en.cost_planned, c.default_cost)) AS cost_total,
  SUM(COALESCE(en.hours_actual, en.hours_planned, c.default_hours)) AS hours_total
FROM enrollment en
JOIN training_plan tp ON tp.id = en.training_plan_id
JOIN course        c  ON c.id  = en.course_id
LEFT JOIN team_membership tm
       ON tm.employee_id = en.employee_id
      AND tm.start_date <= CURRENT_TIMESTAMP
      AND (tm.end_date IS NULL OR tm.end_date >= CURRENT_TIMESTAMP)
LEFT JOIN team t      ON t.id  = tm.team_id
GROUP BY tp.year, t.code;

-- Scadenze certificazioni nei prossimi N giorni (parametrizzare in app)
CREATE OR REPLACE VIEW v_expiring_certifications AS
SELECT
  e.id AS employee_id, e.last_name, e.first_name, e.email,
  c.code AS cert_code, c.name AS cert_name,
  ca.expires_on,
  (ca.expires_on - CURRENT_DATE) AS days_to_expiry
FROM certification_award ca
JOIN employee      e ON e.id = ca.employee_id
JOIN certification c ON c.id = ca.certification_id
WHERE ca.outcome = 'passed_exam'
  AND ca.expires_on IS NOT NULL
  AND ca.expires_on > CURRENT_DATE
  AND e.status = 'active';

-- Compliance gap: chi DEVE avere un corso obbligatorio e non ce l'ha valido.
-- Il job notturno usa questa vista per generare enrollment auto in stato 'proposed'.
CREATE OR REPLACE VIEW v_mandatory_compliance_gap AS
WITH active_employees AS (
  SELECT e.id, e.last_name, e.first_name, tm.team_id
  FROM employee e
  LEFT JOIN team_membership tm
         ON tm.employee_id = e.id
        AND tm.start_date <= CURRENT_TIMESTAMP
        AND (tm.end_date IS NULL OR tm.end_date >= CURRENT_TIMESTAMP)
  WHERE e.status = 'active'
),
required AS (
  -- Espandi le regole su tutti i dipendenti applicabili
  SELECT ae.id AS employee_id, ae.last_name, ae.first_name,
         c.id AS course_id, c.title AS course_title,
         c.leads_to_cert_id, c.compliance_framework, c.recurrence_interval
  FROM mandatory_assignment_rule r
  JOIN course c ON c.id = r.course_id AND c.is_active AND c.is_compliance_course
  JOIN active_employees ae
    ON (r.team_id IS NULL OR r.team_id = ae.team_id)
  WHERE r.is_active
)
SELECT
  req.employee_id, req.last_name, req.first_name,
  req.course_id, req.course_title, req.compliance_framework,
  -- Data dell'ultima award valida, se esiste
  (SELECT MAX(ca.awarded_on)
     FROM certification_award ca
    WHERE ca.employee_id = req.employee_id
      AND ca.certification_id = req.leads_to_cert_id
      AND ca.outcome = 'passed_exam'
      AND (ca.expires_on IS NULL OR ca.expires_on > CURRENT_DATE)
  ) AS last_valid_awarded_on,
  -- Stato del gap
  CASE
    WHEN req.leads_to_cert_id IS NULL THEN 'no_cert_linked'   -- corso obbligatorio senza cert: gestire a mano
    WHEN EXISTS (
      SELECT 1 FROM certification_award ca
       WHERE ca.employee_id = req.employee_id
         AND ca.certification_id = req.leads_to_cert_id
         AND ca.outcome = 'passed_exam'
         AND (ca.expires_on IS NULL OR ca.expires_on > CURRENT_DATE)
    ) THEN 'compliant'
    ELSE 'missing_or_expired'
  END AS compliance_status
FROM required req;
