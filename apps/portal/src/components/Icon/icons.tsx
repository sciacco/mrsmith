import type { JSX } from 'react';

const svgProps = {
  viewBox: '0 0 48 48',
  fill: 'none',
  stroke: 'currentColor',
  strokeWidth: 1.5,
  strokeLinecap: 'round' as const,
  strokeLinejoin: 'round' as const,
};

export const icons: Record<string, JSX.Element> = {
  chart: (
    <svg {...svgProps}>
      <rect x="6" y="34" width="6" height="8" rx="1" />
      <rect x="15" y="26" width="6" height="16" rx="1" />
      <rect x="24" y="18" width="6" height="24" rx="1" />
      <rect x="33" y="10" width="6" height="32" rx="1" />
      <path d="M8 12 L18 16 L28 8 L38 4" />
      <circle cx="38" cy="4" r="2" />
    </svg>
  ),

  funnel: (
    <svg {...svgProps}>
      <path d="M8 6 L40 6 L32 20 L32 38 L16 42 L16 20 Z" />
      <line x1="8" y1="6" x2="40" y2="6" />
      <line x1="14" y1="14" x2="34" y2="14" />
    </svg>
  ),

  document: (
    <svg {...svgProps}>
      <path d="M12 4 L32 4 L38 10 L38 44 L10 44 L10 4 Z" />
      <polyline points="32 4 32 10 38 10" />
      <line x1="16" y1="18" x2="32" y2="18" />
      <line x1="16" y1="24" x2="32" y2="24" />
      <line x1="16" y1="30" x2="28" y2="30" />
      <line x1="16" y1="36" x2="24" y2="36" />
    </svg>
  ),

  'file-text': (
    <svg {...svgProps}>
      <path d="M12 4 L32 4 L38 10 L38 44 L10 44 L10 4 Z" />
      <polyline points="32 4 32 10 38 10" />
      <line x1="16" y1="14" x2="32" y2="14" />
      <line x1="16" y1="20" x2="32" y2="20" />
      <line x1="16" y1="26" x2="26" y2="26" />
      <line x1="16" y1="32" x2="30" y2="32" />
      <line x1="16" y1="38" x2="22" y2="38" />
    </svg>
  ),

  handshake: (
    <svg {...svgProps}>
      <path d="M6 28 C6 28 10 22 16 22 C18 22 20 24 22 24" />
      <path d="M42 28 C42 28 38 22 32 22 C30 22 28 24 26 24" />
      <path d="M22 24 Q24 26 26 24" />
      <path d="M6 28 L6 34 Q6 36 8 36 L18 36" />
      <path d="M42 28 L42 34 Q42 36 40 36 L30 36" />
      <line x1="18" y1="36" x2="30" y2="36" />
      <circle cx="14" cy="14" r="5" />
      <circle cx="34" cy="14" r="5" />
    </svg>
  ),

  cart: (
    <svg {...svgProps}>
      <circle cx="18" cy="40" r="3" />
      <circle cx="36" cy="40" r="3" />
      <path d="M4 4 L10 4 L16 30 L40 30 L44 12 L14 12" />
      <line x1="20" y1="20" x2="40" y2="20" />
    </svg>
  ),

  chat: (
    <svg {...svgProps}>
      <path d="M6 8 L42 8 Q44 8 44 10 L44 30 Q44 32 42 32 L28 32 L20 40 L20 32 L6 32 Q4 32 4 30 L4 10 Q4 8 6 8 Z" />
      <line x1="14" y1="16" x2="34" y2="16" />
      <line x1="14" y1="22" x2="30" y2="22" />
      <line x1="14" y1="28" x2="24" y2="28" />
    </svg>
  ),

  star: (
    <svg {...svgProps}>
      <polygon points="24,4 29,18 44,18 32,27 36,42 24,33 12,42 16,27 4,18 19,18" />
    </svg>
  ),

  coins: (
    <svg {...svgProps}>
      <ellipse cx="18" cy="15" rx="8" ry="4" />
      <path d="M10 15 V24 C10 26.5 13.6 28 18 28 C22.4 28 26 26.5 26 24 V15" />
      <ellipse cx="30" cy="27" rx="8" ry="4" />
      <path d="M22 27 V35 C22 37.5 25.6 39 30 39 C34.4 39 38 37.5 38 35 V27" />
    </svg>
  ),

  users: (
    <svg {...svgProps}>
      <circle cx="17" cy="16" r="5" />
      <circle cx="31" cy="18" r="4" />
      <path d="M8 36 C8 30.5 12.5 26 18 26 H20 C25.5 26 30 30.5 30 36" />
      <path d="M25 35 C25 30.8 28.2 28 32.5 28 H33 C37.2 28 40 30.8 40 35" />
    </svg>
  ),

  package: (
    <svg {...svgProps}>
      <path d="M24 6 L38 13 V35 L24 42 L10 35 V13 Z" />
      <path d="M24 6 V42" />
      <path d="M10 13 L24 20 L38 13" />
    </svg>
  ),

  mail: (
    <svg {...svgProps}>
      <rect x="6" y="10" width="36" height="28" rx="3" />
      <path d="M8 14 L24 27 L40 14" />
    </svg>
  ),

  clipboard: (
    <svg {...svgProps}>
      <rect x="12" y="8" width="24" height="32" rx="3" />
      <path d="M18 8 V6 H30 V8" />
      <line x1="18" y1="18" x2="30" y2="18" />
      <line x1="18" y1="25" x2="30" y2="25" />
      <line x1="18" y1="32" x2="26" y2="32" />
    </svg>
  ),

  tag: (
    <svg {...svgProps}>
      <path d="M8 22 L22 8 H38 V24 L24 38 Z" />
      <circle cx="31" cy="15" r="2.5" />
    </svg>
  ),

  folder: (
    <svg {...svgProps}>
      <path d="M6 14 H18 L22 18 H42 V34 C42 36.2 40.2 38 38 38 H10 C7.8 38 6 36.2 6 34 Z" />
      <path d="M6 14 V12 C6 9.8 7.8 8 10 8 H18 L22 12 H38 C40.2 12 42 13.8 42 16 V18" />
    </svg>
  ),

  spark: (
    <svg {...svgProps}>
      <path d="M24 6 L27 17 L38 20 L27 23 L24 34 L21 23 L10 20 L21 17 Z" />
      <path d="M35 9 L36.5 13 L40.5 14.5 L36.5 16 L35 20 L33.5 16 L29.5 14.5 L33.5 13 Z" />
    </svg>
  ),

  shield: (
    <svg {...svgProps}>
      <path d="M24 5 L38 10 V22 C38 30 32.5 37.2 24 41 C15.5 37.2 10 30 10 22 V10 Z" />
      <path d="M18 22 L22 26 L30 18" />
    </svg>
  ),

  settings: (
    <svg {...svgProps}>
      <circle cx="24" cy="24" r="5" />
      <path d="M24 8 V13" />
      <path d="M24 35 V40" />
      <path d="M40 24 H35" />
      <path d="M13 24 H8" />
      <path d="M35.3 12.7 L31.8 16.2" />
      <path d="M16.2 31.8 L12.7 35.3" />
      <path d="M35.3 35.3 L31.8 31.8" />
      <path d="M16.2 16.2 L12.7 12.7" />
    </svg>
  ),

  briefcase: (
    <svg {...svgProps}>
      <rect x="8" y="14" width="32" height="22" rx="3" />
      <path d="M18 14 V10 C18 8.9 18.9 8 20 8 H28 C29.1 8 30 8.9 30 10 V14" />
      <path d="M8 24 H40" />
    </svg>
  ),

  wrench: (
    <svg {...svgProps}>
      <path d="M29 8 C31.5 10.5 31.8 14.3 30 17 L20 27 L14 21 L24 11 C26.7 9.2 30.5 9.5 33 12" />
      <path d="M12 29 L19 36" />
      <path d="M10 38 L16 32" />
    </svg>
  ),

  database: (
    <svg {...svgProps}>
      <ellipse cx="24" cy="10" rx="12" ry="5" />
      <path d="M12 10 V22 C12 24.8 17.4 27 24 27 C30.6 27 36 24.8 36 22 V10" />
      <path d="M12 22 V34 C12 36.8 17.4 39 24 39 C30.6 39 36 36.8 36 34 V22" />
    </svg>
  ),

  launch: (
    <svg {...svgProps}>
      <path d="M18 14 H10 V38 H34 V30" />
      <path d="M22 10 H38 V26" />
      <path d="M38 10 L20 28" />
    </svg>
  ),
};
