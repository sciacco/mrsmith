import shared from '../pages/shared.module.css';

export function RequiredMark() {
  return (
    <>
      <span className={shared.requiredDot} aria-hidden="true" />
      <span className={shared.visuallyHidden}>obbligatorio</span>
    </>
  );
}
