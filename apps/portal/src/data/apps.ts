import type { App, Category } from '../types';

type AppSeed = {
  name: string;
  icon: string;
  status?: App['status'];
};

const slugify = (value: string) =>
  value
    .toLowerCase()
    .normalize('NFKD')
    .replace(/[\u0300-\u036f]/g, '')
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');

const createApp = (
  workspaceId: string,
  { name, icon, status = 'default' }: AppSeed,
): App => {
  const slug = slugify(name);

  return {
    id: `${workspaceId}-${slug}`,
    name,
    icon,
    href: `/apps/${workspaceId}/${slug}`,
    status,
  };
};

const createCategory = (
  id: string,
  title: string,
  apps: AppSeed[],
): Category => ({
  id,
  title,
  apps: apps.map((app) => createApp(id, app)),
});

export const categories: Category[] = [
  createCategory('acquisti', 'Acquisti', [
    { name: 'Budget Management', icon: 'coins' },
    { name: 'Gestione Utenti', icon: 'users' },
    { name: 'RDA Richieste di Acquisto', icon: 'cart' },
    { name: 'Fornitori', icon: 'handshake' },
  ]),
  createCategory('mkt-sales', 'MKT&Sales', [
    { name: 'Kit e Prodotti', icon: 'package' },
    { name: 'Proposte', icon: 'mail' },
    { name: 'Richieste Fattibilità', icon: 'clipboard' },
    { name: 'Listini e Sconti', icon: 'tag' },
    { name: 'Ordini', icon: 'document' },
  ]),
  createCategory('smart-apps', 'SMART APPS', [
    { name: 'Reports', icon: 'chart' },
    { name: 'Panoramica cliente', icon: 'folder' },
    { name: 'Customer Portal', icon: 'chat' },
    { name: 'Zammù', icon: 'spark' },
    { name: 'Compliance', icon: 'shield' },
    {
      name: 'Customer Portal settings - APPLICATIONS...',
      icon: 'settings',
    },
    { name: 'Nardini', icon: 'briefcase' },
    {
      name: 'NON usare - App di TEST',
      icon: 'spark',
      status: 'test',
    },
    { name: 'AFC Tools', icon: 'settings' },
    { name: 'Coperture', icon: 'shield' },
    { name: 'FDC_playground', icon: 'spark', status: 'test' },
    { name: 'Manutenzioni', icon: 'wrench' },
  ]),
  createCategory('provisioning', 'Provisioning', [
    { name: 'RDF Backend StraFatti', icon: 'database' },
    { name: 'La vendetta di Timoo', icon: 'briefcase' },
    { name: 'S3cchiate di storage', icon: 'launch' },
  ]),
];
