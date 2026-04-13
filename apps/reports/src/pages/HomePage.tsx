import { Link } from 'react-router-dom';
import { Icon, type IconName } from '@mrsmith/ui';
import shared from './shared.module.css';
import styles from './HomePage.module.css';

interface CardProps {
  icon: IconName;
  title: string;
  desc: string;
}

function Card({ icon, title, desc }: CardProps) {
  return (
    <div className={styles.card}>
      <div className={styles.iconWrap}>
        <Icon name={icon} size={20} />
      </div>
      <div className={styles.cardTitle}>{title}</div>
      <div className={styles.cardDesc}>{desc}</div>
    </div>
  );
}

const sections: { label: string; cards: { to: string; icon: IconName; title: string; desc: string }[] }[] = [
  {
    label: 'Commerciale',
    cards: [
      { to: '/ordini', icon: 'file-text', title: 'Ordini', desc: 'Report ordini per data e stato' },
      { to: '/aov', icon: 'bar-chart-2', title: 'AOV', desc: 'Annual Order Value per tipo, categoria e commerciale' },
    ],
  },
  {
    label: 'Rete',
    cards: [
      { to: '/accessi-attivi', icon: 'wifi', title: 'Accessi attivi', desc: 'Linee di accesso per tipo e stato' },
      { to: '/attivazioni-in-corso', icon: 'clock', title: 'Attivazioni in corso', desc: 'Ordini confermati con righe da attivare' },
    ],
  },
  {
    label: 'Contratti',
    cards: [
      { to: '/rinnovi-in-arrivo', icon: 'calendar', title: 'Rinnovi in arrivo', desc: 'Scadenze contrattuali nei prossimi mesi' },
    ],
  },
  {
    label: 'Operativo',
    cards: [
      { to: '/anomalie-mor', icon: 'triangle-alert', title: 'Anomalie MOR', desc: 'Anomalie fatturazione telefonica' },
      { to: '/accounting-timoo', icon: 'phone', title: 'Accounting TIMOO', desc: 'Statistiche giornaliere utenti e SE per tenant' },
    ],
  },
];

export default function HomePage() {
  return (
    <div className={shared.page}>
      <h1 className={shared.title}>Reports</h1>
      {sections.map((section) => (
        <section key={section.label} className={styles.section}>
          <h2 className={styles.sectionTitle}>{section.label}</h2>
          <div className={styles.grid}>
            {section.cards.map((card) => (
              <Link key={card.to} to={card.to} className={styles.cardLink}>
                <Card icon={card.icon} title={card.title} desc={card.desc} />
              </Link>
            ))}
          </div>
        </section>
      ))}
    </div>
  );
}
