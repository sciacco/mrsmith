import { useEditor, EditorContent } from '@tiptap/react';
import StarterKit from '@tiptap/starter-kit';
import Underline from '@tiptap/extension-underline';
import { useEffect } from 'react';
import { Icon } from '@mrsmith/ui';
import styles from './RichTextEditor.module.css';

interface RichTextEditorProps {
  value: string;
  onChange: (html: string) => void;
  placeholder?: string;
  disabled?: boolean;
}

export function RichTextEditor({ value, onChange, disabled }: RichTextEditorProps) {
  const editor = useEditor({
    extensions: [StarterKit, Underline],
    content: value,
    editable: !disabled,
    onUpdate: ({ editor: e }) => {
      onChange(e.getHTML());
    },
  });

  // Sync external value changes
  useEffect(() => {
    if (editor && value !== editor.getHTML()) {
      editor.commands.setContent(value, false);
    }
  }, [value, editor]);

  if (!editor) return null;

  return (
    <div className={`${styles.wrap} ${disabled ? styles.disabled : ''}`}>
      <div className={styles.toolbar}>
        <button
          type="button"
          className={`${styles.toolBtn} ${editor.isActive('bold') ? styles.toolBtnActive : ''}`}
          onClick={() => editor.chain().focus().toggleBold().run()}
          title="Grassetto"
        >
          B
        </button>
        <button
          type="button"
          className={`${styles.toolBtn} ${editor.isActive('italic') ? styles.toolBtnActive : ''}`}
          onClick={() => editor.chain().focus().toggleItalic().run()}
          title="Corsivo"
          style={{ fontStyle: 'italic' }}
        >
          I
        </button>
        <button
          type="button"
          className={`${styles.toolBtn} ${editor.isActive('underline') ? styles.toolBtnActive : ''}`}
          onClick={() => editor.chain().focus().toggleUnderline().run()}
          title="Sottolineato"
          style={{ textDecoration: 'underline' }}
        >
          U
        </button>
        <button
          type="button"
          className={`${styles.toolBtn} ${editor.isActive('bulletList') ? styles.toolBtnActive : ''}`}
          onClick={() => editor.chain().focus().toggleBulletList().run()}
          title="Elenco puntato"
          aria-label="Elenco puntato"
        >
          <Icon name="list" size={16} />
        </button>
        <button
          type="button"
          className={`${styles.toolBtn} ${editor.isActive('orderedList') ? styles.toolBtnActive : ''}`}
          onClick={() => editor.chain().focus().toggleOrderedList().run()}
          title="Elenco numerato"
          aria-label="Elenco numerato"
        >
          <Icon name="list-ordered" size={16} />
        </button>
        <button
          type="button"
          className={styles.toolBtn}
          onClick={() => editor.chain().focus().clearNodes().unsetAllMarks().run()}
          title="Cancella formattazione"
          aria-label="Cancella formattazione"
        >
          <Icon name="remove-formatting" size={16} />
        </button>
      </div>
      <EditorContent editor={editor} className={styles.editor} />
    </div>
  );
}
