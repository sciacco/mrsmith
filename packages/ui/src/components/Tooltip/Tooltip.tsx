import {
  cloneElement,
  isValidElement,
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type FocusEvent,
  type MouseEvent,
  type ReactElement,
  type ReactNode,
} from 'react';
import { createPortal } from 'react-dom';
import styles from './Tooltip.module.css';

export type TooltipPlacement = 'top' | 'bottom' | 'left' | 'right';

interface TooltipProps {
  content: ReactNode;
  placement?: TooltipPlacement;
  disabled?: boolean;
  showDelay?: number;
  hideDelay?: number;
  maxWidth?: number;
  children: ReactElement;
}

const GAP = 8;

export function Tooltip({
  content,
  placement = 'top',
  disabled = false,
  showDelay = 400,
  hideDelay = 100,
  maxWidth = 280,
  children,
}: TooltipProps) {
  const triggerRef = useRef<HTMLElement | null>(null);
  const tooltipRef = useRef<HTMLDivElement | null>(null);
  const showTimerRef = useRef<number | null>(null);
  const hideTimerRef = useRef<number | null>(null);

  const [open, setOpen] = useState(false);
  const [coords, setCoords] = useState({ top: 0, left: 0 });

  const clearTimers = useCallback(() => {
    if (showTimerRef.current !== null) {
      window.clearTimeout(showTimerRef.current);
      showTimerRef.current = null;
    }
    if (hideTimerRef.current !== null) {
      window.clearTimeout(hideTimerRef.current);
      hideTimerRef.current = null;
    }
  }, []);

  const openWithDelay = useCallback(() => {
    if (disabled) return;
    clearTimers();
    showTimerRef.current = window.setTimeout(() => setOpen(true), showDelay);
  }, [disabled, clearTimers, showDelay]);

  const closeWithDelay = useCallback(() => {
    clearTimers();
    hideTimerRef.current = window.setTimeout(() => setOpen(false), hideDelay);
  }, [clearTimers, hideDelay]);

  useEffect(() => () => clearTimers(), [clearTimers]);

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false);
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [open]);

  useLayoutEffect(() => {
    if (!open) return;
    const trigger = triggerRef.current;
    const tooltip = tooltipRef.current;
    if (!trigger || !tooltip) return;

    const update = () => {
      const t = trigger.getBoundingClientRect();
      const tip = tooltip.getBoundingClientRect();
      let top = 0;
      let left = 0;
      switch (placement) {
        case 'top':
          top = t.top - tip.height - GAP;
          left = t.left + t.width / 2 - tip.width / 2;
          break;
        case 'bottom':
          top = t.bottom + GAP;
          left = t.left + t.width / 2 - tip.width / 2;
          break;
        case 'left':
          top = t.top + t.height / 2 - tip.height / 2;
          left = t.left - tip.width - GAP;
          break;
        case 'right':
          top = t.top + t.height / 2 - tip.height / 2;
          left = t.right + GAP;
          break;
      }
      const pad = 8;
      left = Math.max(pad, Math.min(left, window.innerWidth - tip.width - pad));
      top = Math.max(pad, Math.min(top, window.innerHeight - tip.height - pad));
      setCoords({ top, left });
    };

    update();
    window.addEventListener('scroll', update, true);
    window.addEventListener('resize', update);
    return () => {
      window.removeEventListener('scroll', update, true);
      window.removeEventListener('resize', update);
    };
  }, [open, placement, content]);

  if (!isValidElement(children)) return children;

  const childProps = children.props as {
    onMouseEnter?: (e: MouseEvent) => void;
    onMouseLeave?: (e: MouseEvent) => void;
    onFocus?: (e: FocusEvent) => void;
    onBlur?: (e: FocusEvent) => void;
    ref?: unknown;
  };

  const handleMouseEnter = (e: MouseEvent) => {
    childProps.onMouseEnter?.(e);
    openWithDelay();
  };
  const handleMouseLeave = (e: MouseEvent) => {
    childProps.onMouseLeave?.(e);
    closeWithDelay();
  };
  const handleFocus = (e: FocusEvent) => {
    childProps.onFocus?.(e);
    openWithDelay();
  };
  const handleBlur = (e: FocusEvent) => {
    childProps.onBlur?.(e);
    closeWithDelay();
  };

  const triggerWithProps = cloneElement(children, {
    ref: (node: HTMLElement | null) => {
      triggerRef.current = node;
      const originalRef = (children as unknown as { ref?: unknown }).ref;
      if (typeof originalRef === 'function') {
        (originalRef as (n: HTMLElement | null) => void)(node);
      } else if (originalRef && typeof originalRef === 'object') {
        (originalRef as { current: HTMLElement | null }).current = node;
      }
    },
    onMouseEnter: handleMouseEnter,
    onMouseLeave: handleMouseLeave,
    onFocus: handleFocus,
    onBlur: handleBlur,
  } as Partial<unknown> as never);

  return (
    <>
      {triggerWithProps}
      {open && !disabled &&
        createPortal(
          <div
            ref={tooltipRef}
            role="tooltip"
            className={`${styles.tooltip} ${styles[placement]}`}
            style={{ top: coords.top, left: coords.left, maxWidth }}
            onMouseEnter={() => clearTimers()}
            onMouseLeave={closeWithDelay}
          >
            {content}
          </div>,
          document.body,
        )}
    </>
  );
}
