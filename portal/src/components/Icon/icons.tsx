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
};
