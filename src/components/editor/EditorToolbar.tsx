import { type Editor, useEditorState } from '@tiptap/react';
import { useState, type ReactNode } from 'react';
import { safeLink } from './extensions';

type IconName =
  | 'bold'
  | 'italic'
  | 'strike'
  | 'code'
  | 'link'
  | 'bullet'
  | 'ordered'
  | 'quote'
  | 'codeBlock'
  | 'divider'
  | 'undo'
  | 'redo'
  | 'image'
  | 'library';
const paths: Record<IconName, ReactNode> = {
  bold: <path d="M8 5h5a3.5 3.5 0 0 1 0 7H8zm0 7h6a3.5 3.5 0 0 1 0 7H8z" />,
  italic: (
    <>
      <path d="M10 5h7M7 19h7M14 5 10 19" />
    </>
  ),
  strike: (
    <>
      <path d="M7 8c.4-2 2.2-3 5-3 2.5 0 4.2.8 5 2.2M7 16c.8 2 2.5 3 5 3 2.8 0 4.7-1.1 5-3M4 12h16" />
    </>
  ),
  code: <path d="m8.5 9-3 3 3 3m7-6 3 3-3 3m-2-8-3 10" />,
  link: (
    <>
      <path d="m9.5 14.5-1 1a3.5 3.5 0 0 1-5-5l2-2a3.5 3.5 0 0 1 5 0" />
      <path d="m14.5 9.5 1-1a3.5 3.5 0 0 1 5 5l-2 2a3.5 3.5 0 0 1-5 0M8 12h8" />
    </>
  ),
  bullet: (
    <>
      <path d="M9 7h11M9 12h11M9 17h11" />
      <circle cx="4.5" cy="7" r="1" fill="currentColor" stroke="none" />
      <circle cx="4.5" cy="12" r="1" fill="currentColor" stroke="none" />
      <circle cx="4.5" cy="17" r="1" fill="currentColor" stroke="none" />
    </>
  ),
  ordered: (
    <>
      <path d="M10 7h10M10 12h10M10 17h10M4 5v4M3 9h3M3 13h2a1 1 0 0 1 0 2H3l3 3H3" />
    </>
  ),
  quote: (
    <>
      <path d="M5 8h5v5H7c0 2.5-1 4-3 5M14 8h5v5h-3c0 2.5-1 4-3 5" />
    </>
  ),
  codeBlock: (
    <>
      <rect x="3" y="4" width="18" height="16" rx="2" />
      <path d="m9 9-3 3 3 3m6-6 3 3-3 3" />
    </>
  ),
  divider: <path d="M4 12h16" />,
  undo: (
    <>
      <path d="m9 7-5 5 5 5" />
      <path d="M4 12h9a6 6 0 0 1 6 6" />
    </>
  ),
  redo: (
    <>
      <path d="m15 7 5 5-5 5" />
      <path d="M20 12h-9a6 6 0 0 0-6 6" />
    </>
  ),
  image: (
    <>
      <rect x="3" y="5" width="18" height="14" rx="2" />
      <path d="m3 16 5-5 4 4 2-2 7 6" />
    </>
  ),
  library: (
    <>
      <rect x="4" y="4" width="16" height="16" rx="2" />
      <path d="M8 4v16M4 9h4m-4 6h4m8-11v16m0-11h4m-4 6h4" />
    </>
  ),
};
function ToolIcon({ name }: { name: IconName }) {
  return (
    <svg
      aria-hidden="true"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      {paths[name]}
    </svg>
  );
}
function ToolButton({
  label,
  icon,
  active = false,
  disabled = false,
  onClick,
}: {
  label: string;
  icon: IconName;
  active?: boolean;
  disabled?: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      className={`editor-tool ${active ? 'is-active' : ''}`}
      aria-label={label}
      title={label}
      aria-pressed={active || undefined}
      disabled={disabled}
      onClick={onClick}
    >
      <ToolIcon name={icon} />
      <span className="editor-tooltip" role="tooltip">
        {label}
      </span>
    </button>
  );
}

export function EditorToolbar({
  editor,
  onUpload,
  onLibrary,
}: {
  editor: Editor | null;
  onUpload: (file: File) => void;
  onLibrary: () => void;
}) {
  const [linkOpen, setLinkOpen] = useState(false);
  const [href, setHref] = useState('');
  const editorState = useEditorState({
    editor,
    selector: ({ editor: current, transactionNumber }) => ({
      transactionNumber,
      hasTextSelection:
        current !== null && !current.state.selection.empty && !current.isActive('image'),
    }),
  });
  if (!editor) return null;
  const block = editor.isActive('heading', { level: 2 })
    ? 'h2'
    : editor.isActive('heading', { level: 3 })
      ? 'h3'
      : 'paragraph';
  const applyLink = () => {
    const value = safeLink(href);
    if (value) editor.chain().focus().extendMarkRange('link').setLink({ href: value }).run();
    else editor.chain().focus().unsetLink().run();
    setLinkOpen(false);
  };
  return (
    <>
      <div className="editor-commandbar" role="toolbar" aria-label="Document formatting">
        <div className="editor-toolgroup editor-block-picker">
          <label htmlFor="editor-block-style" className="sr-only">
            Text style
          </label>
          <select
            id="editor-block-style"
            value={block}
            onChange={(event) => {
              const value = event.target.value;
              if (value === 'h2') editor.chain().focus().setHeading({ level: 2 }).run();
              else if (value === 'h3') editor.chain().focus().setHeading({ level: 3 }).run();
              else editor.chain().focus().setParagraph().run();
            }}
          >
            <option value="paragraph">Paragraph</option>
            <option value="h2">Heading 2</option>
            <option value="h3">Heading 3</option>
          </select>
        </div>
        <div className="editor-toolgroup">
          <ToolButton
            label="Bold (Ctrl+B)"
            icon="bold"
            active={editor.isActive('bold')}
            onClick={() => editor.chain().focus().toggleBold().run()}
          />
          <ToolButton
            label="Italic (Ctrl+I)"
            icon="italic"
            active={editor.isActive('italic')}
            onClick={() => editor.chain().focus().toggleItalic().run()}
          />
          <ToolButton
            label="Strikethrough"
            icon="strike"
            active={editor.isActive('strike')}
            onClick={() => editor.chain().focus().toggleStrike().run()}
          />
          <ToolButton
            label="Inline code"
            icon="code"
            active={editor.isActive('code')}
            onClick={() => editor.chain().focus().toggleCode().run()}
          />
          <ToolButton
            label="Add link"
            icon="link"
            active={editor.isActive('link')}
            onClick={() => {
              setHref(editor.getAttributes('link').href ?? '');
              setLinkOpen((v) => !v);
            }}
          />
        </div>
        <div className="editor-toolgroup">
          <ToolButton
            label="Bulleted list"
            icon="bullet"
            active={editor.isActive('bulletList')}
            onClick={() => editor.chain().focus().toggleBulletList().run()}
          />
          <ToolButton
            label="Numbered list"
            icon="ordered"
            active={editor.isActive('orderedList')}
            onClick={() => editor.chain().focus().toggleOrderedList().run()}
          />
          <ToolButton
            label="Blockquote"
            icon="quote"
            active={editor.isActive('blockquote')}
            onClick={() => editor.chain().focus().toggleBlockquote().run()}
          />
          <ToolButton
            label="Code block"
            icon="codeBlock"
            active={editor.isActive('codeBlock')}
            onClick={() => editor.chain().focus().toggleCodeBlock().run()}
          />
          <ToolButton
            label="Divider"
            icon="divider"
            onClick={() => editor.chain().focus().setHorizontalRule().run()}
          />
        </div>
        <div className="editor-toolgroup editor-media-tools">
          <label className="editor-tool" aria-label="Upload image" title="Upload image">
            <ToolIcon name="image" />
            <span className="editor-tooltip" role="tooltip">
              Upload image
            </span>
            <input
              type="file"
              accept="image/jpeg,image/png"
              className="sr-only"
              onChange={(event) => {
                const file = event.target.files?.[0];
                if (file) onUpload(file);
                event.currentTarget.value = '';
              }}
            />
          </label>
          <ToolButton label="Media library" icon="library" onClick={onLibrary} />
        </div>
        <div className="editor-toolgroup editor-history-tools">
          <ToolButton
            label="Undo"
            icon="undo"
            disabled={!editor.can().undo()}
            onClick={() => editor.chain().focus().undo().run()}
          />
          <ToolButton
            label="Redo"
            icon="redo"
            disabled={!editor.can().redo()}
            onClick={() => editor.chain().focus().redo().run()}
          />
        </div>
        {linkOpen && (
          <form
            className="editor-link-popover"
            onSubmit={(event) => {
              event.preventDefault();
              applyLink();
            }}
          >
            <div>
              <label htmlFor="editor-link">Link destination</label>
              <p>Paste a safe web, email, or internal link.</p>
            </div>
            <input
              id="editor-link"
              autoFocus
              value={href}
              onChange={(event) => setHref(event.target.value)}
              placeholder="https://example.com"
            />
            <button type="submit" className="link-apply">
              Apply
            </button>
            <button
              type="button"
              className="link-remove"
              onClick={() => {
                setHref('');
                editor.chain().focus().unsetLink().run();
                setLinkOpen(false);
              }}
            >
              Remove
            </button>
          </form>
        )}
      </div>
      {editorState?.hasTextSelection && (
        <div className="editor-selectionbar" role="toolbar" aria-label="Selection formatting">
          <span>Selection</span>
          <ToolButton
            label="Bold"
            icon="bold"
            active={editor.isActive('bold')}
            onClick={() => editor.chain().focus().toggleBold().run()}
          />
          <ToolButton
            label="Italic"
            icon="italic"
            active={editor.isActive('italic')}
            onClick={() => editor.chain().focus().toggleItalic().run()}
          />
          <ToolButton
            label="Link"
            icon="link"
            active={editor.isActive('link')}
            onClick={() => setLinkOpen(true)}
          />
        </div>
      )}
    </>
  );
}
