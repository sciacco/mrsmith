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
    label: 'Billing',
    items: [
      { label: 'Transazioni WHMCS', path: '/transazioni-whmcs' },
      { label: 'Fatture Prometeus', path: '/fatture-prometeus' },
      { label: 'Nuovi articoli', path: '/nuovi-articoli' },
      { label: 'DDT per cespiti', path: '/report-ddt-cespiti' },
    ],
  },
  {
    label: 'Ordini & XConnect',
    items: [
      { label: 'Ordini Sales', path: '/ordini-sales' },
      { label: 'Ticket Remote Hands', path: '/ticket-remote-hands' },
      { label: 'Ordini XConnect', path: '/report-xconnect-rh' },
    ],
  },
  {
    label: 'Energia',
    items: [
      { label: 'Consumi Energia Colo', path: '/consumi-energia-colo' },
    ],
  },
];
