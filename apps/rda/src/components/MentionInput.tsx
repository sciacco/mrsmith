import { Button, Icon } from '@mrsmith/ui';
import { useMemo, useState } from 'react';
import { useUserSearch } from '../api/queries';
import { replaceTrailingMention, trailingMentionToken } from '../lib/mentions';

export function MentionInput({
  disabled,
  submitting,
  onSubmit,
}: {
  disabled?: boolean;
  submitting?: boolean;
  onSubmit: (comment: string) => void;
}) {
  const [value, setValue] = useState('');
  const token = trailingMentionToken(value);
  const search = useUserSearch(token ?? '', Boolean(token));
  const users = useMemo(() => search.data ?? [], [search.data]);

  function submit() {
    const trimmed = value.trim();
    if (!trimmed) return;
    onSubmit(trimmed);
    setValue('');
  }

  return (
    <div className="mentionInput">
      <textarea rows={4} value={value} disabled={disabled} onChange={(event) => setValue(event.target.value)} placeholder="Scrivi un commento..." />
      {token && users.length > 0 ? (
        <div className="mentionMenu">
          {users.map((user) => (
            <button
              key={user.id ?? user.email}
              className="mentionOption"
              type="button"
              onClick={() => setValue((current) => replaceTrailingMention(current, user.email ?? ''))}
            >
              <strong>{[user.first_name, user.last_name].filter(Boolean).join(' ') || user.name || user.email}</strong>
              <small>{user.email}</small>
            </button>
          ))}
        </div>
      ) : null}
      <div className="actionRow">
        <Button size="sm" leftIcon={<Icon name="mail" />} loading={submitting} disabled={disabled || value.trim() === ''} onClick={submit}>
          Commenta
        </Button>
      </div>
    </div>
  );
}
