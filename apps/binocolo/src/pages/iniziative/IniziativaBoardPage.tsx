import { useParams } from 'react-router-dom';

// Placeholder: la board (kanban/tabella/drawer) arriva con F2.
export function IniziativaBoardPage() {
  const { id } = useParams<{ id: string }>();
  return (
    <main>
      <p>Board iniziativa {id} — in arrivo.</p>
    </main>
  );
}
