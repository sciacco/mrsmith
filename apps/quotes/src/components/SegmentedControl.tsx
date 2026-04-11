import { useRef } from 'react';
import styles from './SegmentedControl.module.css';

interface SegmentedControlOption<T extends string> {
  value: T;
  label: string;
  disabled?: boolean;
}

interface SegmentedControlProps<T extends string> {
  value: T;
  onChange: (value: T) => void;
  options: SegmentedControlOption<T>[];
  'aria-label'?: string;
  size?: 'sm' | 'md';
}

export function SegmentedControl<T extends string>({
  value,
  onChange,
  options,
  'aria-label': ariaLabel,
  size = 'md',
}: SegmentedControlProps<T>) {
  const rootRef = useRef<HTMLDivElement | null>(null);

  const enabledValues = options.filter(o => !o.disabled).map(o => o.value);

  const moveFocus = (direction: 1 | -1) => {
    if (enabledValues.length === 0) return;
    const currentIndex = enabledValues.indexOf(value);
    const nextIndex =
      currentIndex < 0
        ? 0
        : (currentIndex + direction + enabledValues.length) % enabledValues.length;
    const nextValue = enabledValues[nextIndex];
    if (nextValue === undefined) return;
    onChange(nextValue);
    requestAnimationFrame(() => {
      const btn = rootRef.current?.querySelector<HTMLButtonElement>(
        `[data-segment-value="${nextValue}"]`,
      );
      btn?.focus();
    });
  };

  return (
    <div
      ref={rootRef}
      role="radiogroup"
      aria-label={ariaLabel}
      className={`${styles.root} ${size === 'sm' ? styles.sizeSm : styles.sizeMd}`}
    >
      {options.map(opt => {
        const selected = opt.value === value;
        return (
          <button
            key={opt.value}
            type="button"
            role="radio"
            aria-checked={selected}
            disabled={opt.disabled}
            data-segment-value={opt.value}
            className={`${styles.segment} ${selected ? styles.selected : ''}`}
            tabIndex={selected ? 0 : -1}
            onClick={() => !opt.disabled && onChange(opt.value)}
            onKeyDown={e => {
              if (e.key === 'ArrowRight' || e.key === 'ArrowDown') {
                e.preventDefault();
                moveFocus(1);
              } else if (e.key === 'ArrowLeft' || e.key === 'ArrowUp') {
                e.preventDefault();
                moveFocus(-1);
              }
            }}
          >
            {opt.label}
          </button>
        );
      })}
    </div>
  );
}
