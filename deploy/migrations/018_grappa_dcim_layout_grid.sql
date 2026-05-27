-- Grappa DCIM layout grid Step 1.
-- Target database: Grappa MySQL 5.6.29.
-- This migration is additive and intentionally stores the grid payload as LONGTEXT,
-- not JSON, for MySQL 5.6 compatibility.

CREATE TABLE IF NOT EXISTS dcim_layout_blocks (
  id INT NOT NULL AUTO_INCREMENT,
  datacenter_id INT(10) NOT NULL,
  islet_id INT NULL,

  datacenter_name_snapshot VARCHAR(250) NOT NULL,
  datacenter_kind VARCHAR(20) NOT NULL COMMENT 'room, mmr',
  islet_name_snapshot VARCHAR(80) NOT NULL,
  block_key VARCHAR(160) NOT NULL,
  block_title VARCHAR(160) NOT NULL,
  display_order INT NOT NULL DEFAULT 0,
  layout_width VARCHAR(80) NULL,

  schema_version VARCHAR(40) NOT NULL DEFAULT 'layout-grid-v1',
  layout_json LONGTEXT NOT NULL COMMENT 'layout-grid-v1 JSON: grid, source metadata, render hints.',
  source_checksum CHAR(64) NULL,
  active TINYINT(1) NOT NULL DEFAULT 1,

  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

  PRIMARY KEY (id),
  UNIQUE KEY uk_dcim_layout_blocks_datacenter_block (datacenter_id, block_key),
  KEY idx_dcim_layout_blocks_datacenter (datacenter_id, active, display_order),
  KEY idx_dcim_layout_blocks_islet (islet_id),
  KEY idx_dcim_layout_blocks_kind (datacenter_kind),

  CONSTRAINT fk_dcim_layout_blocks_datacenter
    FOREIGN KEY (datacenter_id)
    REFERENCES datacenter (id_datacenter)
    ON DELETE NO ACTION
    ON UPDATE NO ACTION,

  CONSTRAINT fk_dcim_layout_blocks_islet
    FOREIGN KEY (islet_id)
    REFERENCES islets (id)
    ON DELETE SET NULL
    ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS dcim_layout_block_plenums (
  id INT NOT NULL AUTO_INCREMENT,
  layout_block_id INT NOT NULL,
  datacenter_id INT(10) NOT NULL,
  plenum_id INT NOT NULL,
  row_index INT NOT NULL,
  col_index INT NOT NULL,
  plenum_type VARCHAR(45) NULL,
  label VARCHAR(80) NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

  PRIMARY KEY (id),
  UNIQUE KEY uk_dcim_layout_block_plenums_cell (layout_block_id, row_index, col_index),
  KEY idx_dcim_layout_block_plenums_block (layout_block_id),
  KEY idx_dcim_layout_block_plenums_plenum (plenum_id, datacenter_id),

  CONSTRAINT fk_dcim_layout_block_plenums_block
    FOREIGN KEY (layout_block_id)
    REFERENCES dcim_layout_blocks (id)
    ON DELETE CASCADE
    ON UPDATE NO ACTION,

  CONSTRAINT fk_dcim_layout_block_plenums_plenum
    FOREIGN KEY (plenum_id, datacenter_id)
    REFERENCES plenums (id, datacenter_id)
    ON DELETE RESTRICT
    ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
