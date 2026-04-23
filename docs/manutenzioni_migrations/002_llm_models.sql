-- Modelli LLM configurabili per le automazioni Manutenzioni.
-- Applicare su DB che ha gia' applicato docs/manutenzioni_schema.sql.

create table if not exists maintenance.llm_model (
    scope                         text primary key
                                  check (scope ~ '^[a-z][a-z0-9_]*$'),
    model                         text not null
                                  check (btrim(model) <> '')
);

insert into maintenance.llm_model (scope, model)
values
    ('default', 'google/gemini-2.5-flash-lite-preview-06-17'),
    ('assistance_draft', 'google/gemini-2.5-flash-lite-preview-06-17')
on conflict (scope) do nothing;
