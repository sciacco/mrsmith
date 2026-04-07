import type { ReactNode } from 'react';
import styles from './TableToolbar.module.css';

interface TableToolbarProps {
  children: ReactNode;
  className?: string;
}

export function TableToolbar({ children, className }: TableToolbarProps) {
  return (
    <div className={`${styles.toolbar} ${className ?? ''}`}>
      {children}
    </div>
  );
}
