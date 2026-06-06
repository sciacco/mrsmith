import { Icon, type IconName } from '@mrsmith/ui';
import type { CSSProperties } from 'react';
import type { EquipmentTypeVisual } from '../../api/types';
import styles from './EquipmentTypeBadge.module.css';

interface EquipmentTypeBadgeProps {
  type?: string | null;
  visual?: EquipmentTypeVisual | null;
  compact?: boolean;
  className?: string;
}

export function EquipmentTypeBadge({ type, visual, compact = false, className }: EquipmentTypeBadgeProps) {
  const resolved = resolveEquipmentTypeVisual(type, visual);
  const cls = [styles.badge, compact ? styles.compact : '', className ?? ''].filter(Boolean).join(' ');

  return (
    <span className={cls} style={equipmentTypeStyle(resolved)} title={resolved.label}>
      <span className={styles.icon}>
        <Icon name={safeIconName(resolved.iconName)} size={compact ? 13 : 14} />
      </span>
      <span className={styles.label}>{resolved.label}</span>
    </span>
  );
}

export function resolveEquipmentTypeVisual(type?: string | null, visual?: EquipmentTypeVisual | null): EquipmentTypeVisual {
  if (visual) return visual;
  const label = type?.trim() || 'Tipo non indicato';
  return {
    type: label,
    label,
    colorHex: '#475569',
    backgroundHex: '#F1F5F9',
    borderHex: '#CBD5E1',
    iconName: 'box',
  };
}

function equipmentTypeStyle(visual: EquipmentTypeVisual): CSSProperties {
  return {
    '--equipment-type-color': visual.colorHex,
    '--equipment-type-bg': visual.backgroundHex,
    '--equipment-type-border': visual.borderHex,
  } as CSSProperties;
}

function safeIconName(iconName: string): IconName {
  switch (iconName) {
    case 'battery-charging':
    case 'box':
    case 'cable':
    case 'cloud':
    case 'database':
    case 'network':
    case 'plug-zap':
    case 'route':
    case 'router':
    case 'server':
    case 'shield':
    case 'wifi':
      return iconName;
    default:
      return 'box';
  }
}
