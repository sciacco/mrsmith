-- 143_training_technical_skill_areas.sql
-- Vocabolario delle aree di competenza tecniche ratificato ad agosto 2026:
-- le 12 aree esistenti (da Factorial) sono tutte soft skill; queste 16
-- coprono le esigenze tecniche della proposta formazione. Idempotente:
-- riesecuzione senza effetto sulle aree gia' presenti.

BEGIN;

INSERT INTO training.skill_area (code, name) VALUES
  ('reti-tlc', 'Reti & TLC'),
  ('sistemi-automazione', 'Sistemi & Automazione'),
  ('cloud-virtualizzazione', 'Cloud & Virtualizzazione'),
  ('microsoft-365-postazioni', 'Microsoft 365 & Postazioni'),
  ('cybersecurity', 'Cybersecurity'),
  ('backup-data-protection', 'Backup & Data Protection'),
  ('data-center-impianti', 'Data Center & Impianti'),
  ('cablaggio-fibra-ottica', 'Cablaggio & Fibra ottica'),
  ('dati-database', 'Dati & Database'),
  ('sviluppo-software', 'Sviluppo software'),
  ('intelligenza-artificiale', 'Intelligenza Artificiale'),
  ('marketing-comunicazione-digitale', 'Marketing & Comunicazione digitale'),
  ('vendite-offerta-cdlan', 'Vendite & Offerta CDLAN'),
  ('compliance-sgi-esg', 'Compliance, SGI & ESG'),
  ('amministrazione-finanza', 'Amministrazione & Finanza'),
  ('persone-leadership', 'Persone & Leadership')
ON CONFLICT (code) DO NOTHING;

COMMIT;
