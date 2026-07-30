import {
  createElement,
  forwardRef,
  type ComponentPropsWithRef,
  type ComponentPropsWithoutRef,
  type ElementType,
  type JSX,
  type ReactElement,
} from 'react';
import styles from './VisuallyHidden.module.css';

type IntrinsicElement = keyof JSX.IntrinsicElements;
type PolymorphicRef<T extends IntrinsicElement> = ComponentPropsWithRef<T>['ref'];

export type VisuallyHiddenProps<T extends IntrinsicElement = 'span'> = {
  as?: T;
} & Omit<ComponentPropsWithoutRef<T>, 'as'>;

type VisuallyHiddenComponent = <T extends IntrinsicElement = 'span'>(
  props: VisuallyHiddenProps<T> & { ref?: PolymorphicRef<T> },
) => ReactElement | null;

function VisuallyHiddenInner<T extends IntrinsicElement = 'span'>(
  { as, className, ...rest }: VisuallyHiddenProps<T>,
  ref: PolymorphicRef<T>,
) {
  const Component: ElementType = as ?? 'span';
  const classes = [styles.visuallyHidden, className ?? ''].filter(Boolean).join(' ');

  return createElement(Component, { ...rest, ref, className: classes });
}

export const VisuallyHidden = forwardRef(VisuallyHiddenInner) as VisuallyHiddenComponent;
