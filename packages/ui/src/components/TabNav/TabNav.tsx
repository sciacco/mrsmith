import { useLocation, Link } from 'react-router-dom';
import { useRef, useEffect, useState, useCallback, type ReactNode } from 'react';
import styles from './TabNav.module.css';

export interface TabNavItem {
  label: string;
  path?: string;
  key?: string;
  icon?: ReactNode;
}

export type TabNavDotIndicator = 'warning' | 'danger' | null | undefined;

interface TabNavProps {
  items: TabNavItem[];
  activeKey?: string;
  onTabChange?: (key: string) => void;
  dotIndicator?: Record<string, TabNavDotIndicator>;
}

function itemKey(item: TabNavItem): string {
  return item.key ?? item.path ?? item.label;
}

export function TabNav({ items, activeKey, onTabChange, dotIndicator }: TabNavProps) {
  const { pathname } = useLocation();
  const containerRef = useRef<HTMLDivElement>(null);
  const tabsRef = useRef<(HTMLElement | null)[]>([]);
  const [indicator, setIndicator] = useState({ left: 0, width: 0 });

  const isControlled = activeKey !== undefined;

  const activeIndex = isControlled
    ? items.findIndex((item) => itemKey(item) === activeKey)
    : items.findIndex(
        (item) =>
          item.path !== undefined &&
          (pathname === item.path || pathname.startsWith(item.path + '/')),
      );

  const updateIndicator = useCallback(() => {
    const el = tabsRef.current[activeIndex];
    const container = containerRef.current;
    if (el && container) {
      const containerRect = container.getBoundingClientRect();
      const tabRect = el.getBoundingClientRect();
      setIndicator({
        left: tabRect.left - containerRect.left,
        width: tabRect.width,
      });
    }
  }, [activeIndex]);

  useEffect(() => {
    updateIndicator();
    window.addEventListener('resize', updateIndicator);
    return () => window.removeEventListener('resize', updateIndicator);
  }, [updateIndicator]);

  return (
    <div className={styles.tabs} ref={containerRef} role="tablist">
      {items.map((item, i) => {
        const key = itemKey(item);
        const active = i === activeIndex;
        const dot = dotIndicator?.[key];
        const className = `${styles.tab} ${active ? styles.active : ''}`;

        const content = (
          <>
            {item.icon && <span className={styles.icon}>{item.icon}</span>}
            <span className={styles.label}>{item.label}</span>
            {dot && (
              <span
                className={`${styles.dot} ${dot === 'danger' ? styles.dotDanger : styles.dotWarning}`}
                aria-hidden="true"
              />
            )}
          </>
        );

        if (item.path && !isControlled) {
          return (
            <Link
              key={key}
              to={item.path}
              ref={(el) => {
                tabsRef.current[i] = el;
              }}
              className={className}
              role="tab"
              aria-selected={active}
            >
              {content}
            </Link>
          );
        }

        return (
          <button
            key={key}
            type="button"
            ref={(el) => {
              tabsRef.current[i] = el;
            }}
            className={className}
            role="tab"
            aria-selected={active}
            onClick={() => onTabChange?.(key)}
          >
            {content}
          </button>
        );
      })}
      {activeIndex >= 0 && (
        <span
          className={styles.indicator}
          style={{ left: indicator.left, width: indicator.width }}
        />
      )}
    </div>
  );
}
