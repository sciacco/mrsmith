import { SingleSelect } from '@mrsmith/ui';
import type { Article } from '../api/types';

export function ArticleCombobox({
  articles,
  value,
  disabled,
  onChange,
}: {
  articles: Article[];
  value: string;
  disabled?: boolean;
  onChange: (value: string) => void;
}) {
  return (
    <SingleSelect<string>
      options={articles.map((article) => ({
        value: article.code,
        label: article.description ? `${article.description} · ${article.code}` : article.code,
      }))}
      selected={value || null}
      disabled={disabled}
      placeholder="Seleziona articolo"
      onChange={(next) => onChange(next ?? '')}
    />
  );
}
