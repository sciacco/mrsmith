CREATE TABLE IF NOT EXISTS coperture.network_coverage_operators (
  id integer PRIMARY KEY,
  name text NOT NULL,
  logo_url text NOT NULL
);

INSERT INTO coperture.network_coverage_operators (id, name, logo_url)
VALUES
  (1, 'TIM', 'https://static.cdlan.business/x/logo_tim.png'),
  (2, 'Fastweb', 'https://static.cdlan.business/x/logo_fastweb.png'),
  (3, 'OpenFiber', 'https://static.cdlan.business/x/logo_openfiber.png'),
  (4, 'OpenFiber CD', 'https://static.cdlan.business/x/logo_openfiberCD.png')
ON CONFLICT (id) DO UPDATE
SET
  name = EXCLUDED.name,
  logo_url = EXCLUDED.logo_url;
