import { useEffect, useRef } from 'react';
import styles from './MatrixBackground.module.css';

type MatrixBackgroundProps = {
  /** Characters per second drop speed — higher = faster */
  speed?: number;
  /** Fraction of columns active per frame (0–1) */
  density?: number;
  /** Canvas opacity (0–1) */
  opacity?: number;
  /** Character set to rain */
  charset?: string;
};

const DEFAULT_CHARSET =
  'アイウエオカキクケコサシスセソタチツテトナニヌネノハヒフヘホマミムメモヤユヨラリルレロワヲン0123456789ABCDEF';
const FONT_SIZE = 14;

export function MatrixBackground({
  speed = 33,
  density = 0.975,
  opacity = 0.12,
  charset = DEFAULT_CHARSET,
}: MatrixBackgroundProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    let columns: number;
    let drops: number[];

    function resize() {
      canvas!.width = window.innerWidth;
      canvas!.height = window.innerHeight;
      columns = Math.floor(canvas!.width / FONT_SIZE);
      drops = Array.from({ length: columns }, () => Math.random() * -100);
    }

    function draw() {
      ctx!.fillStyle = 'rgba(0, 0, 0, 0.05)';
      ctx!.fillRect(0, 0, canvas!.width, canvas!.height);

      ctx!.fillStyle = '#00ff41';
      ctx!.font = `${FONT_SIZE}px monospace`;

      for (let i = 0; i < columns; i++) {
        const char = charset[Math.floor(Math.random() * charset.length)]!;
        const x = i * FONT_SIZE;
        const y = drops[i]! * FONT_SIZE;

        ctx!.globalAlpha = 0.6 + Math.random() * 0.4;
        ctx!.fillText(char, x, y);

        if (y > canvas!.height && Math.random() > density) {
          drops[i] = 0;
        }
        drops[i]!++;
      }
      ctx!.globalAlpha = 1;
    }

    resize();
    window.addEventListener('resize', resize);

    const mq = window.matchMedia('(prefers-reduced-motion: reduce)');
    let interval: ReturnType<typeof setInterval> | null = null;

    function start() {
      if (!mq.matches) {
        interval = setInterval(draw, speed);
      }
    }

    function stop() {
      if (interval) clearInterval(interval);
      ctx!.clearRect(0, 0, canvas!.width, canvas!.height);
    }

    const handleMotionChange = () => {
      if (mq.matches) stop();
      else start();
    };

    mq.addEventListener('change', handleMotionChange);
    start();

    return () => {
      stop();
      window.removeEventListener('resize', resize);
      mq.removeEventListener('change', handleMotionChange);
    };
  }, [speed, density, charset]);

  return (
    <canvas
      ref={canvasRef}
      className={styles.canvas}
      style={{ opacity }}
      aria-hidden="true"
    />
  );
}
