-- Grappa DCIM equipment type visual lookup.
-- Target database: Grappa MySQL 5.6.29.
-- The legacy apparato.type column remains a varchar, but MrSmith create/update
-- accepts only active values from this lookup.

CREATE TABLE IF NOT EXISTS dcim_equipment_type_visuals (
  type_value VARCHAR(80) NOT NULL,
  label VARCHAR(120) NOT NULL,
  color_hex CHAR(7) NOT NULL,
  background_hex CHAR(7) NOT NULL,
  border_hex CHAR(7) NOT NULL,
  icon_name VARCHAR(60) NOT NULL,
  sort_order INT NOT NULL DEFAULT 0,
  active TINYINT(1) NOT NULL DEFAULT 1,

  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

  PRIMARY KEY (type_value),
  KEY idx_dcim_equipment_type_visuals_active (active, sort_order, label)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT IGNORE INTO dcim_equipment_type_visuals
  (type_value, label, color_hex, background_hex, border_hex, icon_name, sort_order, active)
VALUES
  ('Access Point', 'Access Point', '#0284C7', '#E0F2FE', '#7DD3FC', 'wifi', 10, 1),
  ('Ats', 'Ats', '#D97706', '#FEF3C7', '#FBBF24', 'plug-zap', 20, 1),
  ('Cassetto Ottico', 'Cassetto Ottico', '#0D9488', '#CCFBF1', '#5EEAD4', 'cable', 30, 1),
  ('Cluster Virtualizzazione', 'Cluster Virtualizzazione', '#7C3AED', '#F3E8FF', '#C4B5FD', 'cloud', 40, 1),
  ('Firewall', 'Firewall', '#DC2626', '#FEE2E2', '#FCA5A5', 'shield', 50, 1),
  ('Housing', 'Housing', '#475569', '#F1F5F9', '#CBD5E1', 'box', 60, 1),
  ('Passacavo', 'Passacavo', '#0891B2', '#CFFAFE', '#67E8F9', 'route', 70, 1),
  ('Router', 'Router', '#2563EB', '#DBEAFE', '#93C5FD', 'router', 80, 1),
  ('Server', 'Server', '#059669', '#D1FAE5', '#6EE7B7', 'server', 90, 1),
  ('Storage', 'Storage', '#4F46E5', '#E0E7FF', '#A5B4FC', 'database', 100, 1),
  ('Switch', 'Switch', '#65A30D', '#ECFCCB', '#BEF264', 'network', 110, 1),
  ('UPS', 'UPS', '#EA580C', '#FFEDD5', '#FDBA74', 'battery-charging', 120, 1);
