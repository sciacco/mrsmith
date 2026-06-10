// Genera offerta-template.docx: template Carbone per i PDF delle offerte aenad.
// Layout replicato dalle stampe Easyfatt "SHELLI CDLAN offerta" (esempi in artifacts/aenad/).
// Tag Carbone: https://carbone.io/documentation.html (versione API 4).
import {
  AlignmentType,
  BorderStyle,
  Document,
  Footer,
  Header,
  ImageRun,
  PageNumber,
  Packer,
  Paragraph,
  ShadingType,
  Table,
  TableCell,
  TableRow,
  TextRun,
  VerticalAlign,
  WidthType,
  convertMillimetersToTwip as mm,
} from 'docx';
import { readFileSync, writeFileSync } from 'node:fs';

const RED = 'FB1830'; // rosso barre (campionato dal render dei PDF originali)
const BAR_GRAY = 'F2F2F2'; // grigio barra titolo
const TEXT_GRAY = '808080'; // etichette secondarie ("Destinatario", "Firma del Cliente")
const BORDER_GRAY = 'BFBFBF'; // bordi tabella righe
const FONT = 'Calibri';

const CONTENT_MM = 176; // A4 210mm - margini 17mm per lato

const noBorder = { style: BorderStyle.NONE, size: 0, color: 'FFFFFF' };
const noBorders = {
  top: noBorder, bottom: noBorder, left: noBorder, right: noBorder,
  insideHorizontal: noBorder, insideVertical: noBorder,
};
const grayBorder = { style: BorderStyle.SINGLE, size: 4, color: BORDER_GRAY };
const redBorder = { style: BorderStyle.SINGLE, size: 6, color: RED };

const text = (value, opts = {}) =>
  new TextRun({ text: value, font: FONT, size: 18, ...opts });

const para = (children, opts = {}) =>
  new Paragraph({ spacing: { before: 0, after: 0 }, children, ...opts });

// Run "tecnico" che ospita solo tag Carbone (es. :drop): font minimo per non
// influire sul layout; Carbone rimuove il testo del tag al render.
const tag = (value) => text(value, { size: 2 });

const cellMargins = { top: 20, bottom: 20, left: 60, right: 60 };

function cell(children, opts = {}) {
  return new TableCell({
    children,
    margins: cellMargins,
    verticalAlign: VerticalAlign.CENTER,
    ...opts,
  });
}

const png = (file) => readFileSync(new URL(`./assets/${file}`, import.meta.url));

// ---------------------------------------------------------------------------
// Header di pagina: loghi + barra titolo (si ripete su ogni pagina)
// ---------------------------------------------------------------------------

const logoTable = new Table({
  width: { size: mm(CONTENT_MM), type: WidthType.DXA },
  columnWidths: [mm(CONTENT_MM / 2), mm(CONTENT_MM / 2)],
  borders: {
    ...noBorders,
    bottom: { style: BorderStyle.SINGLE, size: 4, color: 'D9D9D9' },
  },
  rows: [
    new TableRow({
      children: [
        cell([
          para([
            new ImageRun({
              type: 'png',
              data: png('shelli-logo.png'),
              transformation: { width: 240, height: 31 }, // 2.50in x 0.32in
            }),
          ]),
        ], { margins: { top: 120, bottom: 80, left: 0, right: 0 } }),
        cell([
          para([
            new ImageRun({
              type: 'png',
              data: png('cdlan-logo.png'),
              transformation: { width: 155, height: 63 }, // 1.62in x 0.66in
            }),
          ], { alignment: AlignmentType.RIGHT }),
        ], { margins: { top: 40, bottom: 40, left: 0, right: 0 } }),
      ],
    }),
  ],
});

const titleBar = new Table({
  width: { size: mm(CONTENT_MM), type: WidthType.DXA },
  columnWidths: [mm(78), mm(34), mm(28), mm(36)],
  borders: noBorders,
  rows: [
    new TableRow({
      children: [
        cell([para([])], { shading: { type: ShadingType.CLEAR, fill: BAR_GRAY } }),
        cell([para([text('Offerta', { bold: true, size: 22 })])], {
          shading: { type: ShadingType.CLEAR, fill: BAR_GRAY },
        }),
        cell([para([text('nr. ', { size: 20 }), text('{d.numero}', { bold: true, size: 20 })])], {
          shading: { type: ShadingType.CLEAR, fill: BAR_GRAY },
        }),
        cell([para([text('del ', { size: 20 }), text('{d.data}', { bold: true, size: 20 })])], {
          shading: { type: ShadingType.CLEAR, fill: BAR_GRAY },
        }),
      ],
    }),
  ],
});

const pageHeader = new Header({
  children: [logoTable, para([]), titleBar],
});

// ---------------------------------------------------------------------------
// Footer di pagina: blocco legale CDLAN (immagine originale) + numero pagina
// ---------------------------------------------------------------------------

const pageFooter = new Footer({
  children: [
    para([
      new ImageRun({
        type: 'png',
        data: png('footer-legale.png'),
        transformation: { width: 666, height: 87 }, // ~176mm x 23mm
      }),
    ]),
    para([
      text('Pag. ', { size: 16, color: TEXT_GRAY }),
      new TextRun({ children: [PageNumber.CURRENT], font: FONT, size: 16, color: TEXT_GRAY, bold: true }),
    ]),
  ],
});

// ---------------------------------------------------------------------------
// Blocco destinatario (solo prima pagina, nel corpo)
// ---------------------------------------------------------------------------

const destinatario = [
  para([text('Destinatario', { size: 16, color: TEXT_GRAY })], { indent: { left: mm(2) } }),
  para([text('{d.destinatario.nome}', { size: 22 })], { indent: { left: mm(4) } }),
  para([text('{d.destinatario.indirizzo}', { size: 22 })], { indent: { left: mm(4) } }),
  para([text('{d.destinatario.capCittaProv}', { size: 22 })], { indent: { left: mm(4) } }),
  para([]),
  para([text('{d.destinatario.rigaFiscale}', { size: 20 })], { indent: { left: mm(4) } }),
];

// ---------------------------------------------------------------------------
// Tabella righe documento
// ---------------------------------------------------------------------------

const COLS_MM = [16, 72, 11, 15, 19, 13, 20, 10];

const headerLabels = ['Codice', 'Descrizione', '', 'Quantità', 'Prezzo', 'Sconto', 'Importo', 'Iva'];
const headerAlign = [
  AlignmentType.LEFT, AlignmentType.LEFT, AlignmentType.CENTER, AlignmentType.CENTER,
  AlignmentType.RIGHT, AlignmentType.CENTER, AlignmentType.RIGHT, AlignmentType.RIGHT,
];

const rowsHeader = new TableRow({
  tableHeader: true,
  children: headerLabels.map((label, i) =>
    cell([para([text(label, { bold: true, color: 'FFFFFF', size: 16 })], { alignment: headerAlign[i] })], {
      shading: { type: ShadingType.CLEAR, fill: RED },
      borders: { top: redBorder, bottom: redBorder, left: redBorder, right: redBorder },
      margins: { ...cellMargins, left: 40, right: 40 },
    })),
});

const rowTags = [
  '{d.righe[i].codice}',
  '{d.righe[i].descrizione:html}',
  '{d.righe[i].udm}',
  '{d.righe[i].qta}',
  '{d.righe[i].prezzo}',
  '{d.righe[i].sconto}',
  '{d.righe[i].importo}',
  '{d.righe[i].iva}',
];
const rowAlign = headerAlign;

const repeatRow = new TableRow({
  children: rowTags.map((rowTag, i) =>
    cell([para([text(rowTag, i === 2 ? { size: 14 } : {})], { alignment: rowAlign[i] })], {
      verticalAlign: VerticalAlign.TOP,
      borders: {
        top: noBorder, bottom: noBorder,
        left: i === 0 ? grayBorder : { style: BorderStyle.SINGLE, size: 2, color: BORDER_GRAY },
        right: i === rowTags.length - 1 ? grayBorder : { style: BorderStyle.SINGLE, size: 2, color: BORDER_GRAY },
      },
    })),
});

const repeatMarkerRow = new TableRow({
  children: rowTags.map((_, i) =>
    cell([para([text(i === 0 ? '{d.righe[i+1].codice}' : '')])], {
      borders: {
        top: noBorder, bottom: grayBorder,
        left: i === 0 ? grayBorder : { style: BorderStyle.SINGLE, size: 2, color: BORDER_GRAY },
        right: i === rowTags.length - 1 ? grayBorder : { style: BorderStyle.SINGLE, size: 2, color: BORDER_GRAY },
      },
    })),
});

const itemsTable = new Table({
  width: { size: mm(CONTENT_MM), type: WidthType.DXA },
  columnWidths: COLS_MM.map((w) => mm(w)),
  borders: noBorders,
  rows: [rowsHeader, repeatRow, repeatMarkerRow],
});

// ---------------------------------------------------------------------------
// Barra pagamento + box totali
// ---------------------------------------------------------------------------

const pagamentoBar = new Table({
  width: { size: mm(96), type: WidthType.DXA },
  columnWidths: [mm(58), mm(38)],
  borders: noBorders,
  rows: [
    new TableRow({
      children: [
        cell([para([text('Pagamento', { bold: true, color: 'FFFFFF' })])], {
          shading: { type: ShadingType.CLEAR, fill: RED },
        }),
        cell([
          para([
            text('{d.mostraCondizioni:ifEQ(true):show(Acconto):elseShow( )}', {
              bold: true, color: 'FFFFFF',
            }),
          ], { alignment: AlignmentType.CENTER }),
        ], { shading: { type: ShadingType.CLEAR, fill: RED } }),
      ],
    }),
    new TableRow({
      children: [
        new TableCell({
          children: [para([]), para([text('{d.pagamento}')]), para([]), para([])],
          margins: cellMargins,
          columnSpan: 2,
          borders: { top: noBorder, bottom: redBorder, left: redBorder, right: redBorder },
        }),
      ],
    }),
  ],
});

const totaliBox = new Table({
  width: { size: mm(74), type: WidthType.DXA },
  columnWidths: [mm(40), mm(34)],
  borders: { ...noBorders, top: grayBorder, bottom: grayBorder, left: grayBorder, right: grayBorder },
  rows: [
    new TableRow({
      children: [
        cell([para([text('Tot. imponibile', { size: 20 })])]),
        cell([para([text('{d.totali.imponibile}', { size: 20 })], { alignment: AlignmentType.RIGHT })]),
      ],
    }),
    new TableRow({
      children: [
        cell([para([text('Tot. Iva', { size: 20 })])]),
        cell([para([text('{d.totali.iva}', { size: 20 })], { alignment: AlignmentType.RIGHT })]),
      ],
    }),
    new TableRow({
      children: [cell([para([])], { columnSpan: 2 })],
    }),
    new TableRow({
      children: [
        cell([para([text('Tot. documento', { bold: true, size: 22 })])]),
        cell([para([text('{d.totali.documento}', { bold: true, size: 22 })], { alignment: AlignmentType.RIGHT })]),
      ],
    }),
  ],
});

const pagamentoTotali = new Table({
  width: { size: mm(CONTENT_MM), type: WidthType.DXA },
  columnWidths: [mm(98), mm(78)],
  borders: noBorders,
  rows: [
    new TableRow({
      children: [
        cell([pagamentoBar, para([])], { verticalAlign: VerticalAlign.TOP, margins: { top: 0, bottom: 0, left: 0, right: 0 } }),
        cell([totaliBox, para([])], { verticalAlign: VerticalAlign.TOP, margins: { top: 0, bottom: 0, left: 0, right: 0 } }),
      ],
    }),
  ],
});

// ---------------------------------------------------------------------------
// Chiusura variante completa (condizioni di fornitura, doppia firma)
// ---------------------------------------------------------------------------

const firmaCell = (children) =>
  new TableCell({
    children,
    margins: cellMargins,
    verticalAlign: VerticalAlign.TOP,
    borders: { top: redBorder, bottom: redBorder, left: { style: BorderStyle.SINGLE, size: 2, color: BORDER_GRAY }, right: redBorder },
  });

const condizioniBox = new Table({
  width: { size: mm(CONTENT_MM), type: WidthType.DXA },
  columnWidths: [mm(116), mm(60)],
  borders: noBorders,
  rows: [
    new TableRow({
      cantSplit: true,
      children: [
        new TableCell({
          children: [
            para([
              tag('{d.mostraCondizioni:ifEQ(false):drop(table)}'),
              text('Con la sottoscrizione del presente Ordine, il Cliente dichiara di aver preso visione e di accettare integralmente le Condizioni di fornitura relative al Servizio:', { size: 15 }),
            ]),
            para([text('DOMINI - disponibili al link :', { size: 14 })], { indent: { left: mm(2) } }),
            para([text('https://www.cdlan.it/hubfs/CondizioniDOMINI.pdf', { size: 14 })], { indent: { left: mm(3) } }),
            para([text('PEC - disponibili al link :', { size: 14 })], { indent: { left: mm(2) } }),
            para([text('https://www.cdlan.it/hubfs/CondizioniPEC.pdf', { size: 14 })], { indent: { left: mm(3) } }),
            para([text('Vendita Hardware e Software - disponibili al link :', { size: 14 })], { indent: { left: mm(2) } }),
            para([text('https://www.cdlan.it/hubfs/CondizioniVenditaHWSW.pdf', { size: 14 })], { indent: { left: mm(3) } }),
          ],
          margins: cellMargins,
          borders: { top: redBorder, bottom: redBorder, left: redBorder, right: { style: BorderStyle.SINGLE, size: 2, color: BORDER_GRAY } },
        }),
        firmaCell([
          para([text('Firma del Cliente', { size: 16, color: TEXT_GRAY, bold: true })]),
          para([]), para([]), para([]),
          para([text('________________________________', { size: 16, color: TEXT_GRAY })], { alignment: AlignmentType.CENTER }),
        ]),
      ],
    }),
  ],
});

const clausoleBox = new Table({
  width: { size: mm(CONTENT_MM), type: WidthType.DXA },
  columnWidths: [mm(64), mm(52), mm(60)],
  borders: noBorders,
  rows: [
    new TableRow({
      cantSplit: true,
      children: [
        new TableCell({
          children: [
            para([
              tag('{d.mostraCondizioni:ifEQ(false):drop(table)}'),
              text('Ai sensi degli articoli 1341 e 1342 c.c., il Cliente dichiara altresì di accettare integralmente le CLAUSOLE AD APPROVAZIONE SPECIFICA riportate nelle Condizioni di fornitura.', { size: 14 }),
            ]),
            para([]),
            para([text('Privacy policy disponibile al link:', { size: 13 })]),
            para([text('https://www.cdlan.it/privacy-policy', { size: 13 })]),
          ],
          margins: cellMargins,
          verticalAlign: VerticalAlign.TOP,
          borders: { top: redBorder, bottom: redBorder, left: redBorder, right: noBorder },
        }),
        new TableCell({
          children: [
            para([]), para([]), para([]),
            para([
              text('Data e luogo ', { size: 14, bold: true, color: TEXT_GRAY }),
              text('___________________________', { size: 14, color: TEXT_GRAY }),
            ]),
          ],
          margins: cellMargins,
          verticalAlign: VerticalAlign.BOTTOM,
          borders: { top: redBorder, bottom: redBorder, left: noBorder, right: { style: BorderStyle.SINGLE, size: 2, color: BORDER_GRAY } },
        }),
        firmaCell([
          para([text('Firma del Cliente', { size: 16, color: TEXT_GRAY, bold: true })]),
          para([]), para([]), para([]),
          para([text('________________________________', { size: 16, color: TEXT_GRAY })], { alignment: AlignmentType.CENTER }),
        ]),
      ],
    }),
  ],
});

// ---------------------------------------------------------------------------
// Chiusura variante semplice (solo privacy + firma)
// ---------------------------------------------------------------------------

const privacyBox = new Table({
  width: { size: mm(CONTENT_MM), type: WidthType.DXA },
  columnWidths: [mm(116), mm(60)],
  borders: noBorders,
  rows: [
    new TableRow({
      cantSplit: true,
      children: [
        new TableCell({
          children: [
            para([
              tag('{d.mostraCondizioni:ifEQ(true):drop(table)}'),
              text('Privacy policy disponibile al link:', { size: 13 }),
            ]),
            para([text('https://www.cdlan.it/privacy-policy', { size: 13 })]),
            para([]), para([]), para([]),
          ],
          margins: cellMargins,
          verticalAlign: VerticalAlign.TOP,
          borders: { top: redBorder, bottom: redBorder, left: redBorder, right: { style: BorderStyle.SINGLE, size: 2, color: BORDER_GRAY } },
        }),
        firmaCell([
          para([text('Firma del Cliente', { size: 16, color: TEXT_GRAY, bold: true })]),
          para([]), para([]), para([]),
          para([text('________________________________', { size: 16, color: TEXT_GRAY })], { alignment: AlignmentType.CENTER }),
        ]),
      ],
    }),
  ],
});

// ---------------------------------------------------------------------------
// Documento
// ---------------------------------------------------------------------------

const doc = new Document({
  styles: {
    default: {
      document: { run: { font: FONT, size: 18 } },
    },
  },
  sections: [
    {
      properties: {
        page: {
          size: { width: mm(210), height: mm(297) },
          margin: { top: mm(44), bottom: mm(36), left: mm(17), right: mm(17) },
        },
      },
      headers: { default: pageHeader },
      footers: { default: pageFooter },
      children: [
        ...destinatario,
        para([]),
        itemsTable,
        para([]),
        pagamentoTotali,
        para([]),
        // Variante completa: condizioni di fornitura + clausole + doppia firma.
        // Le tabelle si auto-rimuovono via {d.mostraCondizioni:ifEQ(..):drop(table)}.
        condizioniBox,
        para([tag('{d.mostraCondizioni:ifEQ(false):drop(p)}')]),
        clausoleBox,
        // Variante semplice: solo privacy + firma
        privacyBox,
      ],
    },
  ],
});

const buffer = await Packer.toBuffer(doc);
writeFileSync(new URL('./offerta-template.docx', import.meta.url), buffer);
console.log('offerta-template.docx generato');
