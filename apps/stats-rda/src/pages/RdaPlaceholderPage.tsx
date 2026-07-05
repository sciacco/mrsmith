import { Icon } from '@mrsmith/ui';

export function RdaPlaceholderPage() {
  return (
    <main className="statsRdaPage">
      <section className="surface placeholderPage">
        <div className="placeholderIcon">
          <Icon name="shopping-cart" size={36} />
        </div>
        <h1 className="placeholderTitle">RDA — in costruzione</h1>
        <p className="placeholderDesc">
          Qui troverai la ricerca e la consultazione delle RDA create nel sistema corrente.
          Questa sezione sarà disponibile nei prossimi rilasci.
        </p>
      </section>
    </main>
  );
}
