export interface NavItem {
  label: string;
  path: string;
}

export interface NavSection {
  label: string;
  items: NavItem[];
}

export const afcToolsNavSections: NavSection[] = [
  {
    label: 'Ordini',
    items: [
      { label: 'Ordini di vendita', path: '/ordini-sales' },
      { label: 'Ticket Remote Hands', path: '/ticket-remote-hands' },
      { label: 'Ordini XConnect', path: '/report-xconnect-rh' },
    ],
  },
  {
    label: 'Fatturazione',
    items: [
      { label: 'Consumi Energia Colo', path: '/consumi-energia-colo' },
      { label: 'Transazioni WHMCS', path: '/transazioni-whmcs' },
      { label: 'Fatture Prometeus', path: '/fatture-prometeus' },
    ],
  },
  {
    label: 'Articoli e Cespiti',
    items: [
      { label: 'Nuovi articoli', path: '/nuovi-articoli' },
      { label: 'DDT per cespiti', path: '/report-ddt-cespiti' },
    ],
  },
];
