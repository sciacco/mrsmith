import { Link } from 'react-router-dom';
import { Icon, type IconName } from '@mrsmith/ui';
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

const sections = [
  {
    label: 'Commerciale',
    cards: [
      { to: '/ordini', icon: 'file-text' as IconName, title: 'Ordini', desc: 'Report ordini per data e stato' },
      { to: '/aov', icon: 'bar-chart-2' as IconName, title: 'AOV', desc: 'Annual Order Value per tipo, categoria e commerciale' },
    ],
  },
  {
    label: 'Rete',
    cards: [
      { to: '/accessi-attivi', icon: 'wifi' as IconName, title: 'Accessi attivi', desc: 'Linee di accesso per tipo e stato' },
      { to: '/attivazioni-in-corso', icon: 'clock' as IconName, title: 'Attivazioni in corso', desc: 'Ordini confermati con righe da attivare' },
    ],
  },
  {
    label: 'Contratti',
    cards: [
      { to: '/rinnovi-in-arrivo', icon: 'calendar' as IconName, title: 'Rinnovi in arrivo', desc: 'Scadenze contrattuali nei prossimi mesi' },
    ],
  },
  {
    label: 'Operativo',
    cards: [
      { to: '/anomalie-mor', icon: 'alert-triangle' as IconName, title: 'Anomalie MOR', desc: 'Anomalie fatturazione telefonica' },
      { to: '/accounting-timoo', icon: 'phone' as IconName, title: 'Accounting TIMOO', desc: 'Statistiche giornaliere utenti e SE per tenant' },
    ],
  },
];

export default function HomePage() {
  return (
    <div className={styles.page}>
      <h1 className={styles.title}>Reports</h1>
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
