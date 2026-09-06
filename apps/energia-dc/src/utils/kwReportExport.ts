import { createElement } from 'react';
import { createRoot } from 'react-dom/client';
import { flushSync } from 'react-dom';
import {
  KWReportChart,
  type KWReportChartSpec,
} from '../components/KWReportChart';
import type { KWReport } from '../api/types';

export function reportPeriod(report: Pick<KWReport, 'year' | 'month'>) {
  return report.month
    ? `${String(report.month).padStart(2, '0')}/${report.year}`
    : String(report.year);
}

// Only presentation-safe fields cross into export. No request/filter object is accepted.
export function reportCharts(report: KWReport): KWReportChartSpec[] {
  const context = {
    customer: report.customer.name,
    period: reportPeriod(report),
  };
  return [
    {
      ...context,
      key: `cliente-${report.customer.id}`,
      title: 'Totale cliente',
      series: report.series,
    },
    ...report.rooms.flatMap((room) => [
      {
        ...context,
        key: `sala-${room.id}`,
        title: `Totale sala · ${room.name}`,
        series: room.series,
      },
      ...room.racks.map((rack) => ({
        ...context,
        key: `sala-${room.id}-rack-${rack.id}`,
        title: `${room.name} · ${rack.name}`,
        series: rack.series,
      })),
    ]),
  ];
}

function filename(value: string) {
  return (
    value
      .replace(/[^\p{L}\p{N}_-]+/gu, '-')
      .replace(/^-|-$/g, '')
      .slice(0, 140) || 'grafico'
  );
}

function download(url: string, name: string) {
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = name;
  anchor.click();
}

async function chartImage(spec: KWReportChartSpec): Promise<HTMLCanvasElement> {
  const host = document.createElement('div');
  host.setAttribute('aria-hidden', 'true');
  Object.assign(host.style, {
    position: 'fixed',
    left: '-12000px',
    top: '0',
    width: '1100px',
    height: '420px',
    pointerEvents: 'none',
  });
  document.body.append(host);
  const root = createRoot(host);
  let url: string | undefined;
  try {
    await document.fonts.ready;
    flushSync(() =>
      root.render(
        createElement(KWReportChart, { series: spec.series, fixed: true }),
      ),
    );
    const svg = host.querySelector('svg');
    if (!svg) throw new Error('chart_not_rendered');
    const clone = svg.cloneNode(true) as SVGElement;
    const originals = [svg, ...svg.querySelectorAll('*')];
    const copies = [clone, ...clone.querySelectorAll('*')];
    originals.forEach((node, index) => {
      const computed = getComputedStyle(node);
      const target = copies[index] as SVGElement;
      [
        'fill',
        'stroke',
        'stroke-width',
        'font-family',
        'font-size',
        'font-weight',
        'opacity',
      ].forEach((property) => {
        target.style.setProperty(property, computed.getPropertyValue(property));
      });
    });
    clone.setAttribute('xmlns', 'http://www.w3.org/2000/svg');
    url = URL.createObjectURL(
      new Blob([new XMLSerializer().serializeToString(clone)], {
        type: 'image/svg+xml;charset=utf-8',
      }),
    );
    const image = new Image();
    await new Promise<void>((resolve, reject) => {
      image.onload = () => resolve();
      image.onerror = () => reject(new Error('chart_image_failed'));
      image.src = url!;
    });
    const canvas = document.createElement('canvas');
    const context = canvas.getContext('2d');
    if (!context) throw new Error('canvas_unavailable');
    const theme = getComputedStyle(document.documentElement);
    const font = theme.getPropertyValue('--font-sans');
    // Wrap all identifying text without clipping long customer/room/rack names.
    context.font = `24px ${font}`;
    const lines: string[] = [];
    const captions = [
      spec.title,
      spec.customer,
      `${spec.period} · Potenza media (kW)`,
    ];
    if (!spec.series.some((point) => point.kilowatt !== null))
      captions.push('Nessuna lettura disponibile');
    for (const text of captions) {
      let line = '';
      for (const character of text) {
        if (context.measureText(line + character).width > 1036 && line) {
          lines.push(line);
          line = '';
        }
        line += character;
      }
      lines.push(line);
    }
    const headingHeight = 48 + lines.length * 32;
    canvas.width = 2200;
    canvas.height = (headingHeight + 420) * 2;
    context.scale(2, 2);
    context.fillStyle = theme.getPropertyValue('--color-bg-elevated').trim();
    context.fillRect(0, 0, 1100, headingHeight + 420);
    context.fillStyle = theme.getPropertyValue('--color-text').trim();
    context.font = `24px ${font}`;
    lines.forEach((line, index) => context.fillText(line, 32, 40 + index * 32));
    context.drawImage(image, 0, headingHeight, 1100, 420);
    return canvas;
  } finally {
    if (url) URL.revokeObjectURL(url);
    root.unmount();
    host.remove();
  }
}

export async function exportChartPNG(spec: KWReportChartSpec) {
  const canvas = await chartImage(spec);
  const blob = await new Promise<Blob>((resolve, reject) =>
    canvas.toBlob(
      (value) => (value ? resolve(value) : reject(new Error('png_failed'))),
      'image/png',
    ),
  );
  const url = URL.createObjectURL(blob);
  try {
    download(
      url,
      `${filename(`${spec.customer}-${spec.period}-${spec.key}`)}.png`,
    );
  } finally {
    window.setTimeout(() => URL.revokeObjectURL(url), 1000);
  }
}

export async function exportReportPDF(report: KWReport) {
  const { jsPDF } = await import('jspdf');
  const pdf = new jsPDF({ orientation: 'landscape', unit: 'mm', format: 'a4' });
  pdf.setProperties({
    title: `Grafici cliente - ${report.customer.name} - ${reportPeriod(report)}`,
    subject: 'Potenza media (kW)',
    creator: 'Energia in DC',
  });
  const charts = reportCharts(report);
  for (let index = 0; index < charts.length; index++) {
    const canvas = await chartImage(charts[index]!);
    if (index > 0) pdf.addPage();
    const width = Math.min(277, (190 * canvas.width) / canvas.height);
    const height = (width * canvas.height) / canvas.width;
    pdf.addImage(
      canvas.toDataURL('PNG'),
      'PNG',
      10,
      10,
      width,
      height,
      undefined,
      'FAST',
    );
  }
  pdf.save(
    `${filename(`${report.customer.name}-${reportPeriod(report)}-grafici`)}.pdf`,
  );
}
