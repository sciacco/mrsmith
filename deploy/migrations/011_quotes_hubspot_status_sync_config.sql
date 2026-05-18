-- Runtime switch for the Quotes HubSpot status sync worker.

BEGIN;

INSERT INTO mrsmith.runtime_config (namespace, key, value, description)
VALUES (
  'quotes',
  'hubspot_status_sync',
  '{"enabled": true, "interval_seconds": 300, "batch_size": 50}'::jsonb,
  'Controls the scheduled Quotes HubSpot status synchronization worker.'
)
ON CONFLICT (namespace, key) DO NOTHING;

COMMIT;
