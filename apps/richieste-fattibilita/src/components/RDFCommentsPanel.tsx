import { Button, Icon, Skeleton, useToast } from '@mrsmith/ui';
import { useMemo, useState, type FormEvent } from 'react';
import { useCreateRichiestaComment, useRDFUserSearch, useRichiestaComments } from '../api/queries';
import type { RDFComment, RDFCommentUser, RDFUser } from '../api/types';
import { copyErrorMessage, formatDateTime } from '../lib/format';
import styles from './RDFCommentsPanel.module.css';

interface RDFCommentsPanelProps {
  richiestaId: number;
}

export function RDFCommentsPanel({ richiestaId }: RDFCommentsPanelProps) {
  const comments = useRichiestaComments(richiestaId);
  const createComment = useCreateRichiestaComment();
  const { toast } = useToast();
  const [value, setValue] = useState('');
  const [selectedMentions, setSelectedMentions] = useState<RDFCommentUser[]>([]);
  const token = trailingMentionToken(value);
  const userSearch = useRDFUserSearch(token ?? '', Boolean(token && token.length > 0));
  const users = useMemo(() => userSearch.data ?? [], [userSearch.data]);
  const items = comments.data?.items ?? [];
  const commentCount = items.length;

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const comment = value.trim();
    if (!comment) return;
    try {
      await createComment.mutateAsync({
        richiestaId,
        comment,
        mentioned_users: selectedMentions,
      });
      setValue('');
      setSelectedMentions([]);
      toast('Commento aggiunto');
    } catch (error) {
      toast(copyErrorMessage(error, 'Commento non salvato.'), 'error');
    }
  }

  function selectMention(user: RDFUser) {
    if (!user.email) return;
    setValue((current) => replaceTrailingMention(current, user.email));
    setSelectedMentions((current) => mergeMentionUsers(current, user));
  }

  function removeMention(user: RDFCommentUser) {
    const key = mentionUserKey(user);
    setSelectedMentions((current) => current.filter((item) => mentionUserKey(item) !== key));
  }

  return (
    <aside className={styles.panel} aria-label="Commenti richiesta">
      <header className={styles.header}>
        <div>
          <h2>Commenti</h2>
          <p>{commentCount === 1 ? '1 commento' : `${commentCount} commenti`}</p>
        </div>
        <span className={styles.headerIcon}>
          <Icon name="mail" size={16} />
        </span>
      </header>

      <div className={styles.body}>
        {comments.isLoading ? (
          <Skeleton rows={5} />
        ) : comments.error ? (
          <div className={styles.stateBlock}>
            <Icon name="triangle-alert" size={20} />
            <p>{copyErrorMessage(comments.error, 'Commenti non disponibili.')}</p>
          </div>
        ) : items.length === 0 ? (
          <div className={styles.stateBlock}>
            <Icon name="mail" size={22} />
            <p>Nessun commento.</p>
          </div>
        ) : (
          <ol className={styles.list}>
            {items.map((comment) => (
              <CommentItem key={comment.id} comment={comment} />
            ))}
          </ol>
        )}
      </div>

      <form className={styles.composer} onSubmit={(event) => void submit(event)}>
        <div className={styles.inputWrap}>
          <textarea
            rows={4}
            value={value}
            disabled={createComment.isPending}
            onChange={(event) => setValue(event.target.value)}
            placeholder="Scrivi un commento..."
          />
          {token && userSearch.error ? (
            <div className={styles.mentionState}>Utenti non disponibili.</div>
          ) : null}
          {token && users.length > 0 ? (
            <div className={styles.mentionMenu}>
              {users.map((user) => (
                <button
                  key={user.id || user.email}
                  type="button"
                  className={styles.mentionOption}
                  onClick={() => selectMention(user)}
                >
                  <strong>{displayUser(user)}</strong>
                  <span>{user.email}</span>
                </button>
              ))}
            </div>
          ) : null}
        </div>

        {selectedMentions.length > 0 ? (
          <div className={styles.selectedMentions} aria-label="Persone menzionate">
            {selectedMentions.map((user) => (
              <button
                key={mentionUserKey(user)}
                type="button"
                className={styles.mentionChip}
                onClick={() => removeMention(user)}
                aria-label={`Rimuovi ${displayUser(user)}`}
              >
                <span>{displayUser(user)}</span>
                <Icon name="x" size={12} />
              </button>
            ))}
          </div>
        ) : null}

        <div className={styles.actions}>
          <Button
            size="sm"
            leftIcon={<Icon name="mail" />}
            loading={createComment.isPending}
            disabled={value.trim() === ''}
            type="submit"
          >
            Commenta
          </Button>
        </div>
      </form>
    </aside>
  );
}

function CommentItem({ comment }: { comment: RDFComment }) {
  const mentionedUsers = comment.mentioned_users ?? [];
  return (
    <li className={styles.item}>
      <span className={styles.avatar}>{initials(comment.author)}</span>
      <article className={styles.message}>
        <header className={styles.meta}>
          <strong>{displayUser(comment.author)}</strong>
          <span>{formatDateTime(comment.created_at)}</span>
        </header>
        <p>{comment.comment}</p>
        {mentionedUsers.length > 0 ? (
          <div className={styles.inlineMentions}>
            {mentionedUsers.map((user) => (
              <span key={mentionUserKey(user)}>{displayUser(user)}</span>
            ))}
          </div>
        ) : null}
      </article>
    </li>
  );
}

function displayUser(user: Partial<RDFCommentUser & RDFUser> | null | undefined): string {
  if (!user) return 'Utente';
  return user.name || [user.first_name, user.last_name].filter(Boolean).join(' ') || user.email || 'Utente';
}

function initials(user: RDFCommentUser): string {
  const label = displayUser(user);
  const parts = label.split(/\s+/).filter(Boolean);
  if (parts.length >= 2) return `${parts[0]?.[0] ?? ''}${parts[1]?.[0] ?? ''}`.toUpperCase();
  return (label[0] ?? '?').toUpperCase();
}

function trailingMentionToken(value: string): string | null {
  const match = value.match(/(^|\s)@([^\s@]*)$/);
  return match?.[2] ?? null;
}

function replaceTrailingMention(value: string, email: string): string {
  return value.replace(/(^|\s)@([^\s@]*)$/, `$1@${email} `);
}

function mergeMentionUsers(current: RDFCommentUser[], next: RDFUser): RDFCommentUser[] {
  const mention = {
    id: next.id,
    subject: next.subject,
    name: displayUser(next),
    email: next.email,
  };
  const key = mentionUserKey(mention);
  if (!key || current.some((user) => mentionUserKey(user) === key)) return current;
  return [...current, mention];
}

function mentionUserKey(user: Pick<RDFCommentUser, 'id' | 'email' | 'subject'>): string {
  if (user.email) return user.email.trim().toLowerCase();
  if (user.subject) return user.subject;
  return user.id;
}
