-- Grappa DCIM room canvas — representative island placement.
-- Target database: Grappa MySQL 5.6.29.
-- Stores operator-authored, representative (non-metric) coordinates for each islet
-- on its room canvas. Kept separate from dcim_layout_blocks so any islet can be
-- placed (even without an imported block) and a layout re-import never overwrites
-- the authored positions. Coordinates are logical units of a virtual room plane.

CREATE TABLE IF NOT EXISTS dcim_room_islet_layout (
  datacenter_id INT(10) NOT NULL,
  islet_id INT NOT NULL,
  x DOUBLE NOT NULL COMMENT 'representative x in virtual room units',
  y DOUBLE NOT NULL COMMENT 'representative y in virtual room units',
  rotation INT NOT NULL DEFAULT 0 COMMENT 'degrees, 0/90/180/270',

  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

  PRIMARY KEY (datacenter_id, islet_id),
  KEY idx_dcim_room_islet_layout_islet (islet_id),

  CONSTRAINT fk_dcim_room_islet_layout_datacenter
    FOREIGN KEY (datacenter_id)
    REFERENCES datacenter (id_datacenter)
    ON DELETE CASCADE
    ON UPDATE NO ACTION,

  CONSTRAINT fk_dcim_room_islet_layout_islet
    FOREIGN KEY (islet_id)
    REFERENCES islets (id)
    ON DELETE CASCADE
    ON UPDATE NO ACTION
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
