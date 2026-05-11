import { Button, Icon } from '@mrsmith/ui';
import { useMemo, useState } from 'react';
import { useUserSearch } from '../api/queries';
import type { CommentMentionUser, RdaUser } from '../api/types';
import { hasMentionToken, replaceTrailingMention, trailingMentionToken } from '../lib/mentions';

export function MentionInput({
  disabled,
  submitting,
  onSubmit,
}: {
  disabled?: boolean;
  submitting?: boolean;
  onSubmit: (comment: string, mentionedUsers: CommentMentionUser[]) => void;
}) {
  const [value, setValue] = useState('');
  const [selectedMentions, setSelectedMentions] = useState<CommentMentionUser[]>([]);
  const token = trailingMentionToken(value);
  const search = useUserSearch(token ?? '', Boolean(token));
  const users = useMemo(() => search.data ?? [], [search.data]);

  function submit() {
    const trimmed = value.trim();
    if (!trimmed) return;
    const mentionedUsers = selectedMentions.filter((user) => user.email && hasMentionToken(trimmed, user.email));
    onSubmit(trimmed, mentionedUsers);
    setValue('');
    setSelectedMentions([]);
  }

  function selectMention(user: RdaUser) {
    if (!user.email) return;
    setValue((current) => replaceTrailingMention(current, user.email ?? ''));
    setSelectedMentions((current) => mergeMentionUsers(current, user));
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
              onClick={() => selectMention(user)}
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

function mergeMentionUsers(current: CommentMentionUser[], next: RdaUser): CommentMentionUser[] {
  const nextKey = mentionUserKey(next);
  if (!nextKey || current.some((user) => mentionUserKey(user) === nextKey)) return current;
  return [...current, next];
}

function mentionUserKey(user: CommentMentionUser): string {
  if (user.email) return user.email.trim().toLowerCase();
  if (user.id != null) return String(user.id);
  return '';
}
