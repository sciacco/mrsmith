import { useLocation, Link } from 'react-router-dom';
import { useRef, useEffect, useState, useCallback } from 'react';
import type { TabNavItem } from '../TabNav/TabNav';
import styles from './TabNavGroup.module.css';

export type TabNavGroupItem = TabNavItem & { path: string };

export interface TabGroup {
  label: string;
  items: TabNavGroupItem[];
}

interface TabNavGroupProps {
  groups: TabGroup[];
}

export function TabNavGroup({ groups }: TabNavGroupProps) {
  const { pathname } = useLocation();
  const containerRef = useRef<HTMLDivElement>(null);
  const groupRefs = useRef<(HTMLDivElement | null)[]>([]);
  const [indicator, setIndicator] = useState({ left: 0, width: 0 });
  const [openIndex, setOpenIndex] = useState<number | null>(null);
  const closeTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const activeGroupIndex = groups.findIndex((group) =>
    group.items.some(
      (item) => pathname === item.path || pathname.startsWith(item.path + '/'),
    ),
  );

  const updateIndicator = useCallback(() => {
    const el = groupRefs.current[activeGroupIndex];
    const container = containerRef.current;
    if (el && container) {
      const containerRect = container.getBoundingClientRect();
      const tabRect = el.getBoundingClientRect();
      setIndicator({
        left: tabRect.left - containerRect.left,
        width: tabRect.width,
      });
    }
  }, [activeGroupIndex]);

  useEffect(() => {
    updateIndicator();
    window.addEventListener('resize', updateIndicator);
    return () => window.removeEventListener('resize', updateIndicator);
  }, [updateIndicator]);

  function handleMouseEnter(index: number) {
    if (closeTimer.current) {
      clearTimeout(closeTimer.current);
      closeTimer.current = null;
    }
    if (groups[index]!.items.length > 1) {
      setOpenIndex(index);
    }
  }

  function handleMouseLeave() {
    closeTimer.current = setTimeout(() => {
      setOpenIndex(null);
      closeTimer.current = null;
    }, 150);
  }

  function handleKeyDown(e: React.KeyboardEvent, index: number) {
    const group = groups[index]!;
    if (e.key === 'Escape') {
      setOpenIndex(null);
    } else if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      if (group.items.length > 1) {
        setOpenIndex(openIndex === index ? null : index);
      }
    }
  }

  return (
    <div className={styles.groups} ref={containerRef}>
      {groups.map((group, i) => {
        const isActive = i === activeGroupIndex;
        const isOpen = openIndex === i;
        const isSingle = group.items.length === 1;

        return (
          <div
            key={group.label}
            ref={(el) => { groupRefs.current[i] = el; }}
            className={`${styles.group} ${isActive ? styles.groupActive : ''}`}
            onMouseEnter={() => handleMouseEnter(i)}
            onMouseLeave={handleMouseLeave}
            onKeyDown={(e) => handleKeyDown(e, i)}
          >
            {isSingle ? (
              <Link to={group.items[0]!.path} className={styles.groupLabel}>
                {group.label}
              </Link>
            ) : (
              <button
                type="button"
                className={styles.groupLabel}
                aria-expanded={isOpen}
                onClick={() => setOpenIndex(isOpen ? null : i)}
              >
                {group.label}
                <svg
                  className={`${styles.chevron} ${isOpen ? styles.chevronOpen : ''}`}
                  width="10"
                  height="10"
                  viewBox="0 0 10 10"
                  fill="none"
                >
                  <path d="M2.5 3.75L5 6.25L7.5 3.75" stroke="currentColor" strokeWidth="1.25" strokeLinecap="round" strokeLinejoin="round" />
                </svg>
              </button>
            )}

            {!isSingle && isOpen && (
              <div
                className={styles.dropdown}
                role="menu"
                onMouseEnter={() => handleMouseEnter(i)}
                onMouseLeave={handleMouseLeave}
              >
                {group.items.map((item) => {
                  const isItemActive =
                    pathname === item.path || pathname.startsWith(item.path + '/');
                  return (
                    <Link
                      key={item.path}
                      to={item.path}
                      role="menuitem"
                      className={`${styles.dropdownItem} ${isItemActive ? styles.dropdownItemActive : ''}`}
                      onClick={() => setOpenIndex(null)}
                    >
                      {item.label}
                    </Link>
                  );
                })}
              </div>
            )}
          </div>
        );
      })}
      {activeGroupIndex >= 0 && (
        <span
          className={styles.indicator}
          style={{ left: indicator.left, width: indicator.width }}
        />
      )}
    </div>
  );
}
