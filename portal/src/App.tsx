import { Portal } from './components/Portal';
import { categories } from './data/apps';

export function App() {
  return <Portal categories={categories} />;
}
