import { useToast } from '@mrsmith/ui';
import { usePostComment } from '../api/queries';
import type { PoComment } from '../api/types';
import { formatDateTimeIT } from '../lib/format';
import { MentionInput } from './MentionInput';

function commentText(comment: PoComment): string {
  return comment.comment ?? comment.comment_text ?? '';
}

function initials(comment: PoComment): string {
  const first = comment.user?.first_name?.[0] ?? comment.user?.name?.[0] ?? comment.user?.email?.[0] ?? '?';
  const last = comment.user?.last_name?.[0] ?? '';
  return `${first}${last}`.toUpperCase();
}

function CommentItem({ comment }: { comment: PoComment }) {
  return (
    <div className="commentItem">
      <span className="avatar">{initials(comment)}</span>
      <div className="commentBody">
        <div className="commentMeta">
          <strong>{[comment.user?.first_name, comment.user?.last_name].filter(Boolean).join(' ') || comment.user?.email || 'Utente'}</strong>
          <span>{formatDateTimeIT(comment.created_at ?? comment.created)}</span>
        </div>
        <p>{commentText(comment)}</p>
        {(comment.replies ?? []).map((reply) => (
          <div key={reply.id} className="commentItem" style={{ marginTop: '0.65rem' }}>
            <span className="avatar">{initials(reply)}</span>
            <div className="commentBody">
              <div className="commentMeta">
                <strong>{reply.user?.email ?? 'Utente'}</strong>
                <span>{formatDateTimeIT(reply.created_at ?? reply.created)}</span>
              </div>
              <p>{commentText(reply)}</p>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

export function CommentsPanel({ poId, comments }: { poId: number; comments: PoComment[] }) {
  const post = usePostComment();
  const { toast } = useToast();

  async function submit(comment: string) {
    try {
      await post.mutateAsync({ id: poId, comment });
      toast('Commento aggiunto');
    } catch {
      toast('Commento non salvato', 'error');
    }
  }

  return (
    <aside className="surface commentsPanel">
      <details className="commentsDetails" open>
        <summary>
          <span>
            <strong>Commenti</strong>
            <small>Discussione sulla richiesta.</small>
          </span>
        </summary>
        <div className="commentList">
          {comments.map((comment) => <CommentItem key={comment.id} comment={comment} />)}
          {comments.length === 0 ? <p className="muted">Nessun commento presente.</p> : null}
        </div>
        <MentionInput submitting={post.isPending} onSubmit={(comment) => void submit(comment)} />
      </details>
    </aside>
  );
}
