import type { IconName } from '@mrsmith/ui';

export interface ReportNavItem {
  label: string;
  path: string;
  icon: IconName;
  desc: string;
}

export interface ReportNavSection {
  label: string;
  items: ReportNavItem[];
}

export const reportNavSections: ReportNavSection[] = [
  {
    label: 'Business',
    items: [
      { label: 'Ordini', path: '/ordini', icon: 'file-text', desc: 'Report ordini per data e stato' },
      { label: 'AOV', path: '/aov', icon: 'bar-chart-2', desc: 'Annual Order Value per tipo, categoria e commerciale' },
      { label: 'Rinnovi in arrivo', path: '/rinnovi-in-arrivo', icon: 'calendar', desc: 'Scadenze contrattuali nei prossimi mesi' },
    ],
  },
  {
    label: 'Operations',
    items: [
      { label: 'Attivazioni in corso', path: '/attivazioni-in-corso', icon: 'clock', desc: 'Ordini confermati con righe da attivare' },
    ],
  },
  {
    label: 'Servizi',
    items: [
      { label: 'Accessi attivi', path: '/accessi-attivi', icon: 'wifi', desc: 'Linee di accesso per tipo e stato' },
      { label: 'Anomalie MOR', path: '/anomalie-mor', icon: 'triangle-alert', desc: 'Anomalie fatturazione telefonica' },
      { label: 'Accounting TIMOO', path: '/accounting-timoo', icon: 'phone', desc: 'Statistiche giornaliere utenti e SE per tenant' },
    ],
  },
];
