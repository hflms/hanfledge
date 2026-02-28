'use client';

import { useRef, useEffect, useState, useCallback } from 'react';
import styles from './GeometryCanvas.module.css';

interface Point { x: number; y: number; }

interface GeometryCanvasProps {
  width?: number;
  height?: number;
  readOnly?: boolean;
}

type Tool = 'point' | 'line' | 'circle' | 'select';

export function GeometryCanvas({ width = 600, height = 400, readOnly = false }: GeometryCanvasProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const [tool, setTool] = useState<Tool>('point');
  const [points, setPoints] = useState<Point[]>([]);

  const draw = useCallback(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    // Clear
    ctx.fillStyle = '#11111b';
    ctx.fillRect(0, 0, width, height);

    // Draw grid
    ctx.strokeStyle = '#2a2a3e';
    ctx.lineWidth = 0.5;
    for (let x = 0; x <= width; x += 20) {
      ctx.beginPath();
      ctx.moveTo(x, 0);
      ctx.lineTo(x, height);
      ctx.stroke();
    }
    for (let y = 0; y <= height; y += 20) {
      ctx.beginPath();
      ctx.moveTo(0, y);
      ctx.lineTo(width, y);
      ctx.stroke();
    }

    // Draw points
    ctx.fillStyle = '#89b4fa';
    for (const p of points) {
      ctx.beginPath();
      ctx.arc(p.x, p.y, 4, 0, Math.PI * 2);
      ctx.fill();
    }

    // Draw lines between consecutive points
    if (points.length >= 2) {
      ctx.strokeStyle = '#a6e3a1';
      ctx.lineWidth = 2;
      ctx.beginPath();
      ctx.moveTo(points[0].x, points[0].y);
      for (let i = 1; i < points.length; i++) {
        ctx.lineTo(points[i].x, points[i].y);
      }
      ctx.stroke();
    }
  }, [points, width, height]);

  useEffect(() => { draw(); }, [draw]);

  const handleClick = useCallback(
    (e: React.MouseEvent<HTMLCanvasElement>) => {
      if (readOnly) return;
      const rect = canvasRef.current?.getBoundingClientRect();
      if (!rect) return;
      const x = e.clientX - rect.left;
      const y = e.clientY - rect.top;
      setPoints((prev) => [...prev, { x, y }]);
    },
    [readOnly],
  );

  const clearCanvas = useCallback(() => { setPoints([]); }, []);

  const tools: { id: Tool; label: string }[] = [
    { id: 'point', label: '点' },
    { id: 'line', label: '线段' },
    { id: 'circle', label: '圆' },
    { id: 'select', label: '选择' },
  ];

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <span className={styles.icon}>△</span>
        <span className={styles.title}>几何画板</span>
      </div>
      <div className={styles.toolbar}>
        {tools.map((t) => (
          <button
            key={t.id}
            className={`${styles.toolBtn} ${tool === t.id ? styles.active : ''}`}
            onClick={() => setTool(t.id)}
            disabled={readOnly}
          >
            {t.label}
          </button>
        ))}
        <button className={styles.clearBtn} onClick={clearCanvas} disabled={readOnly}>
          清除
        </button>
      </div>
      <div className={styles.canvasWrapper}>
        <canvas
          ref={canvasRef}
          width={width}
          height={height}
          className={styles.canvas}
          onClick={handleClick}
        />
      </div>
      <div className={styles.footer}>
        <span className={styles.hint}>已绘制 {points.length} 个点</span>
      </div>
    </div>
  );
}
