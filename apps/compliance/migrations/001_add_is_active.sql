-- Add soft-delete support to origins table.
-- Manual execution required; no migration runner.
ALTER TABLE dns_bl_method ADD COLUMN IF NOT EXISTS is_active BOOLEAN DEFAULT TRUE;
