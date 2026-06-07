import { useState, useRef, useEffect, useLayoutEffect, ChangeEvent, ClipboardEvent } from 'react';
import { createPortal } from 'react-dom';
import styles from './PhoneInput.module.css';

const Flags: Record<string, React.ReactNode> = {
  IT: (
    <svg viewBox="0 0 3 2" width="20" height="14" style={{ borderRadius: '2px', display: 'block' }} aria-hidden="true">
      <rect width="1" height="2" fill="#009246" />
      <rect x="1" width="1" height="2" fill="#ffffff" />
      <rect x="2" width="1" height="2" fill="#ce2b37" />
    </svg>
  ),
  US: (
    <svg viewBox="0 0 741 390" width="20" height="14" style={{ borderRadius: '2px', display: 'block' }} aria-hidden="true">
      <rect width="741" height="390" fill="#b22234" />
      <path d="M0,30h741M0,90h741M0,150h741M0,210h741M0,270h741M0,330h741" stroke="#ffffff" strokeWidth="30" />
      <rect width="296.4" height="210" fill="#3c3b6e" />
      <circle cx="24.7" cy="21" r="5" fill="#fff" />
      <circle cx="74.1" cy="21" r="5" fill="#fff" />
      <circle cx="123.5" cy="21" r="5" fill="#fff" />
      <circle cx="172.9" cy="21" r="5" fill="#fff" />
      <circle cx="222.3" cy="21" r="5" fill="#fff" />
      <circle cx="271.7" cy="21" r="5" fill="#fff" />
      <circle cx="49.4" cy="42" r="5" fill="#fff" />
      <circle cx="98.8" cy="42" r="5" fill="#fff" />
      <circle cx="148.2" cy="42" r="5" fill="#fff" />
      <circle cx="197.6" cy="42" r="5" fill="#fff" />
      <circle cx="247.0" cy="42" r="5" fill="#fff" />
      <circle cx="24.7" cy="63" r="5" fill="#fff" />
      <circle cx="74.1" cy="63" r="5" fill="#fff" />
      <circle cx="123.5" cy="63" r="5" fill="#fff" />
      <circle cx="172.9" cy="63" r="5" fill="#fff" />
      <circle cx="222.3" cy="63" r="5" fill="#fff" />
      <circle cx="271.7" cy="63" r="5" fill="#fff" />
      <circle cx="49.4" cy="84" r="5" fill="#fff" />
      <circle cx="98.8" cy="84" r="5" fill="#fff" />
      <circle cx="148.2" cy="84" r="5" fill="#fff" />
      <circle cx="197.6" cy="84" r="5" fill="#fff" />
      <circle cx="247.0" cy="84" r="5" fill="#fff" />
      <circle cx="24.7" cy="105" r="5" fill="#fff" />
      <circle cx="74.1" cy="105" r="5" fill="#fff" />
      <circle cx="123.5" cy="105" r="5" fill="#fff" />
      <circle cx="172.9" cy="105" r="5" fill="#fff" />
      <circle cx="222.3" cy="105" r="5" fill="#fff" />
      <circle cx="271.7" cy="105" r="5" fill="#fff" />
      <circle cx="49.4" cy="126" r="5" fill="#fff" />
      <circle cx="98.8" cy="126" r="5" fill="#fff" />
      <circle cx="148.2" cy="126" r="5" fill="#fff" />
      <circle cx="197.6" cy="126" r="5" fill="#fff" />
      <circle cx="247.0" cy="126" r="5" fill="#fff" />
      <circle cx="24.7" cy="147" r="5" fill="#fff" />
      <circle cx="74.1" cy="147" r="5" fill="#fff" />
      <circle cx="123.5" cy="147" r="5" fill="#fff" />
      <circle cx="172.9" cy="147" r="5" fill="#fff" />
      <circle cx="222.3" cy="147" r="5" fill="#fff" />
      <circle cx="271.7" cy="147" r="5" fill="#fff" />
      <circle cx="49.4" cy="168" r="5" fill="#fff" />
      <circle cx="98.8" cy="168" r="5" fill="#fff" />
      <circle cx="148.2" cy="168" r="5" fill="#fff" />
      <circle cx="197.6" cy="168" r="5" fill="#fff" />
      <circle cx="247.0" cy="168" r="5" fill="#fff" />
      <circle cx="24.7" cy="189" r="5" fill="#fff" />
      <circle cx="74.1" cy="189" r="5" fill="#fff" />
      <circle cx="123.5" cy="189" r="5" fill="#fff" />
      <circle cx="172.9" cy="189" r="5" fill="#fff" />
      <circle cx="222.3" cy="189" r="5" fill="#fff" />
      <circle cx="271.7" cy="189" r="5" fill="#fff" />
    </svg>
  ),
  DE: (
    <svg viewBox="0 0 5 3" width="20" height="14" style={{ borderRadius: '2px', display: 'block' }} aria-hidden="true">
      <rect width="5" height="1" fill="#000000" />
      <rect y="1" width="5" height="1" fill="#dd0000" />
      <rect y="2" width="5" height="1" fill="#ffce00" />
    </svg>
  ),
  FR: (
    <svg viewBox="0 0 3 2" width="20" height="14" style={{ borderRadius: '2px', display: 'block' }} aria-hidden="true">
      <rect width="1" height="2" fill="#002395" />
      <rect x="1" width="1" height="2" fill="#ffffff" />
      <rect x="2" width="1" height="2" fill="#ed2939" />
    </svg>
  ),
  GB: (
    <svg viewBox="0 0 50 30" width="20" height="14" style={{ borderRadius: '2px', display: 'block' }} aria-hidden="true">
      <rect width="50" height="30" fill="#012169" />
      <path d="M0,0 L50,30 M50,0 L0,30" stroke="#ffffff" strokeWidth="6" />
      <path d="M0,0 L50,30 M50,0 L0,30" stroke="#c8102e" strokeWidth="4" />
      <path d="M0,0 L25,15 M50,30 L25,15" stroke="#c8102e" strokeWidth="2" />
      <path d="M50,0 L25,15 M0,30 L25,15" stroke="#c8102e" strokeWidth="2" />
      <path d="M25,0 v30 M0,15 h50" stroke="#ffffff" strokeWidth="10" />
      <path d="M25,0 v30 M0,15 h50" stroke="#c8102e" strokeWidth="6" />
    </svg>
  ),
  ES: (
    <svg viewBox="0 0 3 2" width="20" height="14" style={{ borderRadius: '2px', display: 'block' }} aria-hidden="true">
      <rect width="3" height="2" fill="#aa151b" />
      <rect y="0.5" width="3" height="1" fill="#f1bf00" />
    </svg>
  ),
  CH: (
    <svg viewBox="0 0 20 14" width="20" height="14" style={{ borderRadius: '2px', display: 'block' }} aria-hidden="true">
      <rect width="20" height="14" fill="#d52b1e" />
      <path d="M10,3 v8 M6,7 h8" stroke="#ffffff" strokeWidth="2.5" strokeLinecap="square" />
    </svg>
  ),
  AT: (
    <svg viewBox="0 0 3 2" width="20" height="14" style={{ borderRadius: '2px', display: 'block' }} aria-hidden="true">
      <rect width="3" height="2" fill="#c8102e" />
      <rect y="0.666" width="3" height="0.666" fill="#ffffff" />
    </svg>
  ),
};

const GlobeIcon = (
  <svg viewBox="0 0 16 16" width="20" height="14" fill="none" stroke="currentColor" strokeWidth="1.5" style={{ display: 'block' }} aria-hidden="true">
    <circle cx="8" cy="8" r="6.25" />
    <path d="M1.75 8h12.5M8 1.75a10 10 0 0 0 0 12.5M8 1.75a10 10 0 0 1 0 12.5" />
  </svg>
);

interface Country {
  code: string;
  dial: string;
  name: string;
}

const COUNTRIES: Country[] = [
  { code: 'IT', dial: '+39', name: 'Italia' },
  { code: 'US', dial: '+1', name: 'Stati Uniti' },
  { code: 'DE', dial: '+49', name: 'Germania' },
  { code: 'FR', dial: '+33', name: 'Francia' },
  { code: 'GB', dial: '+44', name: 'Regno Unito' },
  { code: 'ES', dial: '+34', name: 'Spagna' },
  { code: 'CH', dial: '+41', name: 'Svizzera' },
  { code: 'AT', dial: '+43', name: 'Austria' },
  { code: 'BE', dial: '+32', name: 'Belgio' },
  { code: 'NL', dial: '+31', name: 'Paesi Bassi' },
  { code: 'PT', dial: '+351', name: 'Portogallo' },
  { code: 'SE', dial: '+46', name: 'Svezia' },
  { code: 'NO', dial: '+47', name: 'Norvegia' },
  { code: 'FI', dial: '+358', name: 'Finlandia' },
  { code: 'DK', dial: '+45', name: 'Danimarca' },
  { code: 'IE', dial: '+353', name: 'Irlanda' },
  { code: 'GR', dial: '+30', name: 'Grecia' },
  { code: 'PL', dial: '+48', name: 'Polonia' },
  { code: 'RO', dial: '+40', name: 'Romania' },
  { code: 'HR', dial: '+385', name: 'Croazia' },
  { code: 'OTHER', dial: '+', name: 'Altro / Internazionale' },
];

const DROPDOWN_GAP = 6;
const VIEWPORT_PAD = 8;
const DROPDOWN_MAX_HEIGHT = 220;

export interface PhoneInputProps {
  value: string; // Saved as full E.164 string (e.g. "+393331234567")
  onChange: (value: string) => void;
  label?: string;
  disabled?: boolean;
  required?: boolean;
  error?: string;
  id?: string;
  name?: string;
}

const findCountryByValue = (val: string): Country => {
  const defaultCountry = COUNTRIES[0] || { code: 'IT', dial: '+39', name: 'Italia' };
  if (!val) return defaultCountry;
  const matched = [...COUNTRIES]
    .sort((a, b) => b.dial.length - a.dial.length)
    .find((c) => c.code !== 'OTHER' && val.startsWith(c.dial));
  return matched || (val.startsWith('+') ? COUNTRIES.find((c) => c.code === 'OTHER') || defaultCountry : defaultCountry);
};

export function PhoneInput({
  value,
  onChange,
  label,
  disabled = false,
  required = false,
  error,
  id,
  name,
}: PhoneInputProps) {
  const [selectedCountry, setSelectedCountry] = useState<Country>(() => findCountryByValue(value));
  const [dropdownOpen, setDropdownOpen] = useState(false);
  const [focused, setFocused] = useState(false);
  const [coords, setCoords] = useState({ top: 0, left: 0, width: 0, placeTop: false });

  const containerRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const dropdownRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  // Sync selected country if value changes from outside
  useEffect(() => {
    if (value) {
      const matched = [...COUNTRIES]
        .sort((a, b) => b.dial.length - a.dial.length)
        .find((c) => c.code !== 'OTHER' && value.startsWith(c.dial));
      if (matched) {
        if (matched.code !== selectedCountry.code) {
          setSelectedCountry(matched);
        }
      } else if (value.startsWith('+') && selectedCountry.code !== 'OTHER') {
        const otherCountry = COUNTRIES.find((c) => c.code === 'OTHER');
        if (otherCountry) {
          setSelectedCountry(otherCountry);
        }
      }
    }
  }, [value, selectedCountry.code]);

  // Handle click outside to close dropdown
  useEffect(() => {
    if (!dropdownOpen) return;
    function handleClickOutside(e: MouseEvent) {
      const target = e.target as Node;
      if (
        !triggerRef.current?.contains(target) &&
        !dropdownRef.current?.contains(target)
      ) {
        setDropdownOpen(false);
      }
    }
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, [dropdownOpen]);

  // Handle Escape key to close dropdown
  useEffect(() => {
    if (!dropdownOpen) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setDropdownOpen(false);
        triggerRef.current?.focus();
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [dropdownOpen]);

  // Update dropdown coordinates relative to the trigger button
  useLayoutEffect(() => {
    if (!dropdownOpen) return;
    const trigger = triggerRef.current;
    if (!trigger) return;

    const update = () => {
      const rect = trigger.getBoundingClientRect();
      const spaceBelow = window.innerHeight - rect.bottom - VIEWPORT_PAD;
      const spaceAbove = rect.top - VIEWPORT_PAD;
      const placeTop = spaceBelow < DROPDOWN_MAX_HEIGHT && spaceAbove > spaceBelow;
      const top = placeTop
        ? rect.top - DROPDOWN_GAP
        : rect.bottom + DROPDOWN_GAP;
      
      const dropdownEl = dropdownRef.current;
      const renderedWidth = dropdownEl ? dropdownEl.offsetWidth : 280;
      const dropdownWidth = Math.max(260, renderedWidth);
      const maxLeft = window.innerWidth - dropdownWidth - VIEWPORT_PAD;
      const left = Math.max(VIEWPORT_PAD, Math.min(rect.left, maxLeft));

      setCoords({ top, left, width: dropdownWidth, placeTop });
    };

    update();
    window.addEventListener('scroll', update, true);
    window.addEventListener('resize', update);
    return () => {
      window.removeEventListener('scroll', update, true);
      window.removeEventListener('resize', update);
    };
  }, [dropdownOpen]);

  // Split value to display local number without prefix
  const dialCode = selectedCountry.dial;
  const localNumber = value.startsWith(dialCode) ? value.slice(dialCode.length) : value;

  const handleNumberChange = (e: ChangeEvent<HTMLInputElement>) => {
    const cleanDigits = e.target.value.replace(/\D/g, '');
    onChange(cleanDigits ? `${selectedCountry.dial}${cleanDigits}` : '');
  };

  const handlePaste = (e: ClipboardEvent<HTMLInputElement>) => {
    const text = e.clipboardData.getData('Text').trim();
    const matched = COUNTRIES.find(
      (c) => c.code !== 'OTHER' && (text.startsWith(c.dial) || text.startsWith(`00${c.dial.slice(1)}`))
    );
    if (matched) {
      e.preventDefault();
      setSelectedCountry(matched);
      const dial = matched.dial;
      const cleanText = text.startsWith('00')
        ? text.slice(2 + dial.length - 1)
        : text.slice(dial.length);
      const cleanDigits = cleanText.replace(/\D/g, '');
      onChange(cleanDigits ? `${dial}${cleanDigits}` : '');
    } else if (text.startsWith('+') || text.startsWith('00')) {
      e.preventDefault();
      const otherCountry = COUNTRIES.find((c) => c.code === 'OTHER');
      if (otherCountry) {
        setSelectedCountry(otherCountry);
        const prefixLength = text.startsWith('00') ? 2 : 1;
        const cleanDigits = text.slice(prefixLength).replace(/\D/g, '');
        onChange(cleanDigits ? `+${cleanDigits}` : '');
      }
    }
  };

  const selectCountry = (country: Country) => {
    setSelectedCountry(country);
    setDropdownOpen(false);
    const cleanDigits = localNumber.replace(/\D/g, '');
    onChange(cleanDigits ? `${country.dial}${cleanDigits}` : '');
    inputRef.current?.focus();
  };

  const renderFlag = (code: string) => {
    if (code === 'OTHER') return GlobeIcon;
    if (code in Flags) return Flags[code];
    return <span className={styles.isoBadge}>{code}</span>;
  };

  const isFocused = focused || dropdownOpen;
  const renderInline = Boolean(containerRef.current?.closest('dialog[open]'));

  const dropdown = (
    <div
      ref={dropdownRef}
      className={`${styles.dropdown} ${renderInline ? styles.dropdownInline : ''}`}
      style={
        renderInline
          ? coords.placeTop
            ? { bottom: `calc(100% + ${DROPDOWN_GAP}px)` }
            : { top: `calc(100% + ${DROPDOWN_GAP}px)` }
          : {
              top: coords.top,
              left: coords.left,
              width: coords.width,
              transform: coords.placeTop ? 'translateY(-100%)' : undefined,
            }
      }
    >
      <div className={styles.options}>
        {COUNTRIES.map((country) => (
          <button
            key={country.code}
            type="button"
            className={`${styles.option} ${country.code === selectedCountry.code ? styles.optionSelected : ''}`}
            onClick={() => selectCountry(country)}
          >
            <span className={styles.optionFlag}>{renderFlag(country.code)}</span>
            <span className={styles.optionName}>{country.name}</span>
            <span className={styles.optionDial}>{country.dial}</span>
          </button>
        ))}
      </div>
    </div>
  );

  return (
    <div ref={containerRef} className={`${styles.field} ${error ? styles.fieldError : ''}`}>
      {label && (
        <label htmlFor={id} className={styles.label}>
          {label}
          {required && <span className={styles.requiredMarker} aria-hidden="true" />}
        </label>
      )}
      <div
        className={`${styles.inputContainer} ${isFocused ? styles.focused : ''} ${
          disabled ? styles.disabled : ''
        }`}
      >
        <button
          ref={triggerRef}
          type="button"
          className={styles.prefixSelector}
          onClick={() => {
            if (!disabled) setDropdownOpen(!dropdownOpen);
          }}
          disabled={disabled}
          aria-haspopup="listbox"
          aria-expanded={dropdownOpen}
          aria-label="Seleziona prefisso internazionale"
        >
          <span className={styles.flagWrap}>{renderFlag(selectedCountry.code)}</span>
          <span className={styles.dialCode}>{selectedCountry.dial}</span>
          <span className={`${styles.arrow} ${dropdownOpen ? styles.arrowOpen : ''}`}>▼</span>
        </button>

        <div className={styles.divider} />

        <input
          ref={inputRef}
          id={id}
          type="tel"
          className={styles.numberInput}
          value={localNumber}
          onChange={handleNumberChange}
          onPaste={handlePaste}
          onFocus={() => setFocused(true)}
          onBlur={() => setFocused(false)}
          disabled={disabled}
          placeholder={selectedCountry.code === 'OTHER' ? 'Prefisso + Numero' : '333 123 4567'}
        />
      </div>
      {name && <input type="hidden" name={name} value={value} />}
      {error && <span className={styles.errorText}>{error}</span>}

      {dropdownOpen && !disabled && (renderInline ? dropdown : createPortal(dropdown, document.body))}
    </div>
  );
}
