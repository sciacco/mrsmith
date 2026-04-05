import { useState, useEffect, useRef } from 'react';
import styles from './HomePage.module.css';

interface ThresholdInputProps {
  onChange: (value: number | null) => void;
}

export function ThresholdInput({ onChange }: ThresholdInputProps) {
  const [value, setValue] = useState(80);
  const timerRef = useRef<ReturnType<typeof setTimeout>>();

  function handleSlide(next: number) {
    setValue(next);
    clearTimeout(timerRef.current);
    timerRef.current = setTimeout(() => {
      onChange(next);
    }, 300);
  }

  useEffect(() => {
    return () => clearTimeout(timerRef.current);
  }, []);

  return (
    <div className={styles.thresholdWrap}>
      <input
        type="range"
        min={10}
        max={100}
        step={5}
        value={value}
        onChange={(e) => handleSlide(Number(e.target.value))}
        className={styles.thresholdSlider}
      />
      <span className={styles.thresholdValue}>{value}%</span>
    </div>
  );
}
