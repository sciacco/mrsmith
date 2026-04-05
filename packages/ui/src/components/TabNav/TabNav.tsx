import { useLocation, Link } from 'react-router-dom';
import { useRef, useEffect, useState, useCallback } from 'react';
import styles from './TabNav.module.css';

export interface TabNavItem {
  label: string;
  path: string;
}

interface TabNavProps {
  items: TabNavItem[];
}

export function TabNav({ items }: TabNavProps) {
  const { pathname } = useLocation();
  const containerRef = useRef<HTMLDivElement>(null);
  const tabsRef = useRef<(HTMLAnchorElement | null)[]>([]);
  const [indicator, setIndicator] = useState({ left: 0, width: 0 });

  const activeIndex = items.findIndex((item) =>
    pathname === item.path || pathname.startsWith(item.path + '/'),
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
    <div className={styles.tabs} ref={containerRef}>
      {items.map((item, i) => (
        <Link
          key={item.path}
          to={item.path}
          ref={(el) => { tabsRef.current[i] = el; }}
          className={`${styles.tab} ${i === activeIndex ? styles.active : ''}`}
        >
          {item.label}
        </Link>
      ))}
      {activeIndex >= 0 && (
        <span
          className={styles.indicator}
          style={{ left: indicator.left, width: indicator.width }}
        />
      )}
    </div>
  );
}
