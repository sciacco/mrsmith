import { useEffect } from 'react';

interface ShortcutHandlers {
  onSave?: () => void;
  onPublish?: () => void;
  onTabSwitch?: (tab: number) => void;
}

export function useKeyboardShortcuts({ onSave, onPublish, onTabSwitch }: ShortcutHandlers) {
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      // Skip when typing in inputs
      const target = e.target as HTMLElement;
      const isInput = target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable;

      if ((e.metaKey || e.ctrlKey) && e.key === 's') {
        e.preventDefault();
        onSave?.();
        return;
      }

      if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
        e.preventDefault();
        onPublish?.();
        return;
      }

      if (!isInput && onTabSwitch) {
        const num = parseInt(e.key);
        if (num >= 1 && num <= 4) {
          onTabSwitch(num - 1);
        }
      }
    };

    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [onSave, onPublish, onTabSwitch]);
}
