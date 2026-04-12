import { useEditor, EditorContent } from '@tiptap/react';
import StarterKit from '@tiptap/starter-kit';
import Underline from '@tiptap/extension-underline';
import Link from '@tiptap/extension-link';
import TextAlign from '@tiptap/extension-text-align';
import Highlight from '@tiptap/extension-highlight';
import TextStyle from '@tiptap/extension-text-style';
import Color from '@tiptap/extension-color';
import Placeholder from '@tiptap/extension-placeholder';
import { useEffect, useCallback, useState, useRef } from 'react';
import { Icon } from '@mrsmith/ui';
import styles from './RichTextEditor.module.css';

interface RichTextEditorProps {
  value: string;
  onChange: (html: string) => void;
  placeholder?: string;
  disabled?: boolean;
  standalone?: boolean;
}

const TEXT_COLORS = [
  '', '#1e293b', '#dc2626', '#ea580c', '#ca8a04',
  '#16a34a', '#2563eb', '#7c3aed', '#db2777', '#6b7280',
];

const HIGHLIGHT_COLORS = [
  '', '#fef08a', '#bbf7d0', '#bfdbfe',
  '#fbcfe8', '#fed7aa', '#e9d5ff', '#fecaca',
];

const COLOR_TITLES: Record<string, string> = {
  '': 'Predefinito',
  '#1e293b': 'Scuro',
  '#dc2626': 'Rosso',
  '#ea580c': 'Arancione',
  '#ca8a04': 'Giallo',
  '#16a34a': 'Verde',
  '#2563eb': 'Blu',
  '#7c3aed': 'Viola',
  '#db2777': 'Rosa',
  '#6b7280': 'Grigio',
  '#fef08a': 'Giallo',
  '#bbf7d0': 'Verde',
  '#bfdbfe': 'Blu',
  '#fbcfe8': 'Rosa',
  '#fed7aa': 'Arancione',
  '#e9d5ff': 'Viola',
  '#fecaca': 'Rosso',
};

export function RichTextEditor({ value, onChange, disabled, standalone, placeholder }: RichTextEditorProps) {
  const [colorOpen, setColorOpen] = useState(false);
  const popoverRef = useRef<HTMLDivElement>(null);

  const editor = useEditor({
    extensions: [
      StarterKit,
      Underline,
      Link.configure({ openOnClick: false, HTMLAttributes: { rel: 'noopener noreferrer' } }),
      TextAlign.configure({ types: ['heading', 'paragraph'] }),
      Highlight.configure({ multicolor: true }),
      TextStyle,
      Color,
      Placeholder.configure({ placeholder: placeholder ?? '' }),
    ],
    content: value,
    editable: !disabled,
    onUpdate: ({ editor: e }) => {
      onChange(e.getHTML());
    },
  });

  useEffect(() => {
    if (editor && value !== editor.getHTML()) {
      editor.commands.setContent(value, false);
    }
  }, [value, editor]);

  useEffect(() => {
    if (!colorOpen) return;
    const handleClick = (e: MouseEvent) => {
      if (popoverRef.current && !popoverRef.current.contains(e.target as Node)) {
        setColorOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, [colorOpen]);

  const setLink = useCallback(() => {
    if (!editor) return;
    const prev = editor.getAttributes('link').href as string | undefined;
    const url = window.prompt('URL', prev ?? 'https://');
    if (url === null) return;
    if (url === '') {
      editor.chain().focus().extendMarkRange('link').unsetLink().run();
    } else {
      editor.chain().focus().extendMarkRange('link').setLink({ href: url }).run();
    }
  }, [editor]);

  if (!editor) return null;

  const activeTextColor = (editor.getAttributes('textStyle').color as string) ?? '';
  const activeHighlightColor = (editor.getAttributes('highlight').color as string) ?? '';

  return (
    <div className={`${styles.wrap} ${standalone ? styles.standalone : ''} ${disabled ? styles.disabled : ''}`}>
      <div className={styles.toolbar}>
        {/* ── Headings ── */}
        <button
          type="button"
          className={`${styles.toolBtn} ${editor.isActive('heading', { level: 2 }) ? styles.toolBtnActive : ''}`}
          onClick={() => editor.chain().focus().toggleHeading({ level: 2 }).run()}
          title="Titolo 2"
          aria-label="Titolo 2"
        >
          <Icon name="heading2" size={16} />
        </button>
        <button
          type="button"
          className={`${styles.toolBtn} ${editor.isActive('heading', { level: 3 }) ? styles.toolBtnActive : ''}`}
          onClick={() => editor.chain().focus().toggleHeading({ level: 3 }).run()}
          title="Titolo 3"
          aria-label="Titolo 3"
        >
          <Icon name="heading3" size={16} />
        </button>
        <button
          type="button"
          className={`${styles.toolBtn} ${editor.isActive('heading', { level: 4 }) ? styles.toolBtnActive : ''}`}
          onClick={() => editor.chain().focus().toggleHeading({ level: 4 }).run()}
          title="Titolo 4"
          aria-label="Titolo 4"
        >
          <Icon name="heading4" size={16} />
        </button>

        <div className={styles.separator} />

        {/* ── Text formatting ── */}
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

        <div className={styles.separator} />

        {/* ── Color popover ── */}
        <div className={styles.popoverAnchor} ref={popoverRef}>
          <button
            type="button"
            className={`${styles.toolBtn} ${colorOpen ? styles.toolBtnActive : ''}`}
            onClick={() => setColorOpen(o => !o)}
            title="Colori"
            aria-label="Colori"
          >
            <span className={styles.colorIndicator}>
              A
              <span
                className={styles.colorBar}
                style={{ background: activeTextColor || 'var(--color-text)' }}
              />
            </span>
          </button>
          {colorOpen && (
            <div className={styles.colorPopover}>
              <span className={styles.colorLabel}>Colore</span>
              <div className={styles.colorGrid}>
                {TEXT_COLORS.map(c => (
                  <button
                    key={`t-${c}`}
                    type="button"
                    className={`${styles.swatch} ${activeTextColor === c ? styles.swatchActive : ''}`}
                    title={COLOR_TITLES[c] ?? c}
                    onClick={() => {
                      if (c) {
                        editor.chain().focus().setColor(c).run();
                      } else {
                        editor.chain().focus().unsetColor().run();
                      }
                    }}
                  >
                    {c ? (
                      <span className={styles.swatchFill} style={{ background: c }} />
                    ) : (
                      <span className={styles.swatchReset}>
                        <Icon name="x" size={12} />
                      </span>
                    )}
                  </button>
                ))}
              </div>

              <span className={styles.colorLabel}>Sfondo</span>
              <div className={styles.colorGrid}>
                {HIGHLIGHT_COLORS.map(c => (
                  <button
                    key={`h-${c}`}
                    type="button"
                    className={`${styles.swatch} ${activeHighlightColor === c ? styles.swatchActive : ''}`}
                    title={COLOR_TITLES[c] ?? c}
                    onClick={() => {
                      if (c) {
                        editor.chain().focus().toggleHighlight({ color: c }).run();
                      } else {
                        editor.chain().focus().unsetHighlight().run();
                      }
                    }}
                  >
                    {c ? (
                      <span className={styles.swatchFill} style={{ background: c }} />
                    ) : (
                      <span className={styles.swatchReset}>
                        <Icon name="x" size={12} />
                      </span>
                    )}
                  </button>
                ))}
              </div>
            </div>
          )}
        </div>

        <div className={styles.separator} />

        {/* ── Lists ── */}
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

        <div className={styles.separator} />

        {/* ── Alignment ── */}
        <button
          type="button"
          className={`${styles.toolBtn} ${editor.isActive({ textAlign: 'left' }) ? styles.toolBtnActive : ''}`}
          onClick={() => editor.chain().focus().setTextAlign('left').run()}
          title="Allinea a sinistra"
          aria-label="Allinea a sinistra"
        >
          <Icon name="align-left" size={16} />
        </button>
        <button
          type="button"
          className={`${styles.toolBtn} ${editor.isActive({ textAlign: 'center' }) ? styles.toolBtnActive : ''}`}
          onClick={() => editor.chain().focus().setTextAlign('center').run()}
          title="Centra"
          aria-label="Centra"
        >
          <Icon name="align-center" size={16} />
        </button>
        <button
          type="button"
          className={`${styles.toolBtn} ${editor.isActive({ textAlign: 'right' }) ? styles.toolBtnActive : ''}`}
          onClick={() => editor.chain().focus().setTextAlign('right').run()}
          title="Allinea a destra"
          aria-label="Allinea a destra"
        >
          <Icon name="align-right" size={16} />
        </button>

        <div className={styles.separator} />

        {/* ── Link + clear ── */}
        <button
          type="button"
          className={`${styles.toolBtn} ${editor.isActive('link') ? styles.toolBtnActive : ''}`}
          onClick={setLink}
          title="Link"
          aria-label="Link"
        >
          <Icon name="link" size={16} />
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
