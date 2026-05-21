# CDLAN Training Management — Architectural Simplification Plan

This document outlines a pragmatic plan to simplify the training mini-app domain. It strips away academic over-engineering and aligns the codebase with standard, maintainable patterns for internal corporate tooling.

---

## 1. State Machine Redundancy (Tripled Implementations)

### The Over-Engineered State
State transition rules and guards for `enrollment` are currently defined/enforced in three separate layers:
1. An executable TypeScript specification (`state_machines.ts`).
2. A manual mechanical translation of that spec in the Go backend (`state_machine.go`).
3. A PL/pgSQL database trigger (`enrollment_state_trigger.sql` / `validate_enrollment_transition()`).

### The Simplified Approach
* **Single Source of Truth (Backend):** Enforce all business rules and state transition validation exclusively in the Go backend service layer.
* **Database Check Constraint:** Remove the complex trigger and replace it with a simple, standard PostgreSQL `CHECK` constraint (or simple check function) that only prevents invalid transitions at a high level, or rely entirely on database transaction logic.
* **Frontend Flexibility:** The frontend should simply call the backend API and handle standard HTTP validation error codes (e.g. `400 Bad Request` or `422 Unprocessable Entity`), rather than running its own local duplicate state machine engine.

### Impact & Benefits
* **Fast Feature Delivery:** Adding or changing a state transition requires editing exactly **one** file in the Go backend, rather than three files across three different environments (TypeScript, Go, and PL/pgSQL).
* **Elimination of Sync Drift:** Completely removes the risk of the database trigger falling out of sync with the application code.

### Transition Plan
1. Keep `state-machines.md` purely as a developer reference diagram.
2. Maintain the Go state machine in `backend/internal/training/state_machine.go` as the sole runtime validation engine.
3. Keep a basic check trigger in the database to prevent catastrophic direct updates, but do not mirror complex application guards (like role-based checks or plan status checks) in PL/pgSQL.
4. Retire `apps/training/docs/state_machines.ts` or treat it solely as an archived doc.

---

## 2. High Audit Overhead for Minor Typo Corrections

### The Over-Engineered State
Correcting a typographical error or administrative typo (e.g., fixing a certificate's issue date or description) currently requires a dedicated `correct` state machine transition. This transition forces a mandatory reason and logs full-row `before_state` and `after_state` JSON snapshots into the `audit_log` table.

### The Simplified Approach
* **Standard Admin CRUD:** Treat corrections for what they are: standard, authenticated administrative database updates.
* **Simple, Row-Level Audit Log:** If auditing is required, use a simple `last_modified_by` and `last_modified_at` tracking column on critical tables, or insert a simple, generic text record into the `audit_log` (e.g., `"user X updated certificate Y's issue date"`) instead of performing full-row JSONB diffing.

### Impact & Benefits
* **Simpler Admin UI:** Avoids prompting admins with annoying "Enter reason for this correction" popup modals for simple typo fixes.
* **Lower Database Bloat:** Eliminates storing large, duplicate JSON blobs in the `audit_log` table for trivial text edits.
* **Straightforward Go Code:** Eliminates complex state machine wrappers around simple REST `PUT` updates.

### Transition Plan
1. Remove the custom `correct` transition from the state machine specs.
2. Implement standard REST PUT/PATCH endpoints for administrative Master Data tables (`vendor`, `course`, `certification`) and `certification_award` details.
3. Replace JSONB diffing in `audit_log` with a lightweight ledger entry containing `actor_id`, `action` (e.g., `"update_certification"`), `entity_id`, and a short description.

---

## 3. `certification_award` Lifecycle Duplicating `enrollment`

### The Over-Engineered State
The `certification_award` table implements a parallel lifecycle containing an `in_progress` state and a `failed_exam` state, mirroring the operational states of the `enrollment` table.

### The Simplified Approach
* **Leaf-Level Immutability:** A `certification_award` should represent a completed, successful achievement (the earned certificate).
* **Let Enrollment Handle Process:** The "in progress" state and the "failed exam" states belong exclusively to the `enrollment` table. 
* **Simplified Schema:** A `certification_award` is only created when an exam is successfully passed or a certificate is uploaded. It only has one implicit state: active/valid (with an optional expiration date).

### Impact & Benefits
* **No Redundant Records:** Prevents the database from having duplicate "in progress" states for the same course and certificate.
* **Simpler Queries:** Querying active certifications becomes a simple `SELECT ... FROM certification_award WHERE expires_on > NOW()` rather than filtering out in-progress and failed awards.

### Transition Plan
1. Drop the `in_progress` and `failed_exam` outcomes from the `award_outcome` enum.
2. Limit the `certification_award` table to successful achievements (e.g., `passed_exam` or `attendance_only`).
3. Ensure that when a user takes an exam, the progress is tracked via their `enrollment` status (`in_progress` -> `completed` or `failed`). Only upon transition to `completed` is a static `certification_award` row inserted.

---

## 4. Over-Architected HR Sync Framework

### The Over-Engineered State
The architecture specifies a generic multi-source `HRProvider` interface, a concurrent synchronization engine, a complex `hr_source` database enum (`('factorial', 'manual', 'successor')`), and reconciliation scripts to handle dual-system sync scenarios during transitions.

### The Simplified Approach
* **Pragmatic Sync Script:** Write a single, clean cron job or service that reads from the *current* HR system (e.g., Factorial today) and does an upsert on the `employee` table based on a unique identifier (email or employee ID).
* **Decoupled Configuration:** If the HR system changes in the future, simply update the backend sync client credentials/endpoints or swap the sync client implementation. The database schema doesn't need to know or care which provider is active.

### Impact & Benefits
* **Cleaner Database Schema:** Removes the `hr_source` enum and associated conditional columns.
* **Faster Integration:** Eliminates building a massive provider-agnostic framework before the exact API specifications of the future HR system are even known.

### Transition Plan
1. Keep the `employee` table simple and generic, identifying employees by their stable `email` and a generic `external_id`.
2. Remove the `hr_source` enum and the `external_source` column from `employee`.
3. Build a straightforward Factorial integration script. When the successor system is introduced, swap the cron job logic without touching the database schema.

---

## 5. Polymorphic XOR Database Checks

### The Over-Engineered State
The `document` table uses a strict, database-level XOR check constraint to allow a document to belong to either an `enrollment` or a `certification_award`:
```sql
CHECK ((enrollment_id IS NOT NULL)::int + (certification_award_id IS NOT NULL)::int = 1)
```

### The Simplified Approach
* **Standard Foreign Keys:** While the XOR constraint works, it complicates standard cascading deletes and ORM mappings.
* **Simplified Document Association:** Either:
  * Place a nullable `document_id` directly on the `enrollment` and `certification_award` tables (making the document the child leaf).
  * Or maintain the simple nullable foreign key columns on `document` without the strict database-level mathematical constraint check, handling validation in the application logic.

### Impact & Benefits
* **Simplified Schema Relationships:** Simpler entity relationships that map beautifully to typical backend ORMs and serialization libraries.
* **Easier Cascading Deletes:** Deleting an enrollment automatically and cleanly cascades to its documents without constraint violations.

### Transition Plan
1. Keep the `enrollment_id` and `certification_award_id` columns on the `document` table.
2. Drop the mathematical XOR check constraint (`CHECK ((enrollment_id IS NOT NULL)...)`) to allow standard nullable column behaviors.
3. Validate that a document is associated with exactly one entity in the Go service layer.

---

## 6. Scope Creep: Periodic Skill Self-Assessment

### The Over-Engineered State
The schema includes a full-featured `skill_assessment` ledger table where employees rate themselves on skills from 0-5, tracking the historical trend of self-assessments and recording the audit source (e.g., CV vs verbal declaration vs survey).

### The Simplified Approach
* **Defer to Later Phases:** Defer the entire "Skill Competency Matrix" and self-assessment features. Focus entirely on the core business problem: tracking course enrollments, managing plan budgets, and tracking compliance certifications.
* **If Required, Keep It Flat:** If a simple rating is required, keep a single flat rating column or table without a complex historical ledger and verification auditing system.

### Impact & Benefits
* **Reduced Scope:** Cuts down the number of pages, sliders, and charts that need to be built for the initial frontend version.
* **Clear Project Focus:** Keeps the team focused on replacing the Excel-based training budget planning sheets without getting distracted by performance-review features.

### Transition Plan
1. Drop the `skill_assessment` table from the active schema migration for the initial launch.
2. Remove self-assessment pages from the frontend dashboard requirements.
3. Keep the Gerarchic `skill_area` and `certification` catalogo as they are critical for categorizing courses and certifications.
