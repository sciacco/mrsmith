import { useCallback, useEffect, useRef, useState } from 'react';

type TabKey = 'header' | 'kits' | 'notes' | 'contacts';

export function useDirtyState() {
  const [dirtyTabs, setDirtyTabs] = useState<Record<TabKey, boolean>>({
    header: false,
    kits: false,
    notes: false,
    contacts: false,
  });

  const isDirty = Object.values(dirtyTabs).some(Boolean);

  const markDirty = useCallback((tab: TabKey) => {
    setDirtyTabs(prev => ({ ...prev, [tab]: true }));
  }, []);

  const markClean = useCallback((tab?: TabKey) => {
    if (tab) {
      setDirtyTabs(prev => ({ ...prev, [tab]: false }));
    } else {
      setDirtyTabs({ header: false, kits: false, notes: false, contacts: false });
    }
  }, []);

  // Track snapshot for comparison
  const snapshotRef = useRef<string>('');

  const setSnapshot = useCallback((data: unknown) => {
    snapshotRef.current = JSON.stringify(data);
  }, []);

  const hasChanged = useCallback((data: unknown) => {
    return JSON.stringify(data) !== snapshotRef.current;
  }, []);

  // beforeunload protection
  useEffect(() => {
    if (!isDirty) return;
    const handler = (e: BeforeUnloadEvent) => {
      e.preventDefault();
    };
    window.addEventListener('beforeunload', handler);
    return () => window.removeEventListener('beforeunload', handler);
  }, [isDirty]);

  return { isDirty, dirtyTabs, markDirty, markClean, setSnapshot, hasChanged };
}
