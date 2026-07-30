import { VisuallyHidden } from '@mrsmith/ui';
import shared from '../pages/shared.module.css';

export function RequiredMark() {
  return (
    <>
      <span className={shared.requiredDot} aria-hidden="true" />
      <VisuallyHidden>obbligatorio</VisuallyHidden>
    </>
  );
}
