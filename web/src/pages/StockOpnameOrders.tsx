import { type Component } from 'solid-js';
import { TransactionPrintWorkspace } from '~/components/transactionPrint/TransactionPrintWorkspace';
import type {
  TransactionSampleDocument,
  TransactionSourceConfig,
} from '~/components/transactionPrint/types';
import { ACCURATE_SOURCE } from '~/domain';

const SAMPLE_STOCK_OPNAME_ORDERS: readonly TransactionSampleDocument[] = [
  {
    header: {
      id: 95001,
      number: 'SOO-EX-2026-0501',
      date: '14/04/2026',
      description: 'Perintah Opname Perkakas & Kunci Rak A1-A4 (Contoh)',
      statusName: 'Menunggu Opname',
    },
    warehouseName: 'Gudang Utama A',
    items: [
      {
        itemId: 55101,
        itemNo: 'TLS-101',
        itemName: 'Kunci Ring Pas Set 8-24mm Chrome Vanadium',
        upcNo: '8997015500102',
        quantity: 3,
        unitName: 'Pack',
        unitPrice: 195000,
        documentType: ACCURATE_SOURCE.STOCK_OPNAME_ORDER,
        documentNumber: 'SOO-EX-2026-0501',
      },
      {
        itemId: 55102,
        itemNo: 'TLS-102',
        itemName: 'Obeng Plus Minus Magnetik 6 Inch Gagang Karet',
        upcNo: '8997015500201',
        quantity: 8,
        unitName: 'Pcs',
        unitPrice: 28000,
        documentType: ACCURATE_SOURCE.STOCK_OPNAME_ORDER,
        documentNumber: 'SOO-EX-2026-0501',
      },
      {
        itemId: 55103,
        itemNo: 'TLS-103',
        itemName: 'Tang Kombinasi Heavy Duty 8 Inch',
        upcNo: '8997015500300',
        quantity: 5,
        unitName: 'Pcs',
        unitPrice: 54000,
        documentType: ACCURATE_SOURCE.STOCK_OPNAME_ORDER,
        documentNumber: 'SOO-EX-2026-0501',
      },
    ],
  },
  {
    header: {
      id: 95002,
      number: 'SOO-EX-2026-0502',
      date: '15/04/2026',
      description: 'Perintah Opname Baut, Sekrup & Mur Zona Besi',
      statusName: 'Sedang Berlangsung',
    },
    warehouseName: 'Gudang Utara',
    items: [
      {
        itemId: 55201,
        itemNo: 'HDW-201',
        itemName: 'Baut Baja M8 x 40mm + Mur Kotak Isi 100',
        upcNo: '8998826500105',
        quantity: 4,
        unitName: 'Dus',
        unitPrice: 86000,
        documentType: ACCURATE_SOURCE.STOCK_OPNAME_ORDER,
        documentNumber: 'SOO-EX-2026-0502',
      },
      {
        itemId: 55202,
        itemNo: 'HDW-NO-UPC-04',
        itemName: 'Ring Plat Besi M8 Galvanis Pack Isi 200 (Tanpa UPC)',
        upcNo: '',
        quantity: 2,
        unitName: 'Pack',
        unitPrice: 32000,
        documentType: ACCURATE_SOURCE.STOCK_OPNAME_ORDER,
        documentNumber: 'SOO-EX-2026-0502',
      },
    ],
  },
  {
    header: {
      id: 95003,
      number: 'SOO-EX-2026-0503',
      date: '16/04/2026',
      description: 'Perintah Opname Alat Ukur & Keselamatan Kerja',
      statusName: 'Selesai',
    },
    warehouseName: 'Gudang Selatan',
    items: [
      {
        itemId: 55301,
        itemNo: 'TLS-301',
        itemName: 'Meteran Gulung Baja 5 Meter Anti Karat',
        upcNo: '8995504501016',
        quantity: 6,
        unitName: 'Pcs',
        unitPrice: 39000,
        documentType: ACCURATE_SOURCE.STOCK_OPNAME_ORDER,
        documentNumber: 'SOO-EX-2026-0503',
      },
      {
        itemId: 55302,
        itemNo: 'TLS-302',
        itemName: 'Helm Proyek Safety SNI Kuning + Tali Dagu',
        upcNo: '8995504502020',
        quantity: 4,
        unitName: 'Unit',
        unitPrice: 48000,
        documentType: ACCURATE_SOURCE.STOCK_OPNAME_ORDER,
        documentNumber: 'SOO-EX-2026-0503',
      },
    ],
  },
  {
    header: {
      id: 95004,
      number: 'SOO-EX-2026-0504',
      date: '17/04/2026',
      description: 'Perintah Opname Mesin Bor & Mata Gerinda',
      statusName: 'Menunggu Opname',
    },
    warehouseName: 'Gudang Utama A',
    items: [
      {
        itemId: 55401,
        itemNo: 'TLS-401',
        itemName: 'Mata Gerinda Potong Besi 4 Inch Isi 25',
        upcNo: '8996104501019',
        quantity: 3,
        unitName: 'Dus',
        unitPrice: 115000,
        documentType: ACCURATE_SOURCE.STOCK_OPNAME_ORDER,
        documentNumber: 'SOO-EX-2026-0504',
      },
      {
        itemId: 55402,
        itemNo: 'TLS-402',
        itemName: 'Sepatu Safety Kulit Ujung Besi Ukuran 42',
        upcNo: '8996104502023',
        quantity: 2,
        unitName: 'Pasang',
        unitPrice: 285000,
        documentType: ACCURATE_SOURCE.STOCK_OPNAME_ORDER,
        documentNumber: 'SOO-EX-2026-0504',
      },
    ],
  },
];

const STOCK_OPNAME_ORDER_SOURCE_CONFIG: TransactionSourceConfig = {
  source: ACCURATE_SOURCE.STOCK_OPNAME_ORDER,
  titleKey: 'transactionPrint.sources.stockOpnameOrder.title',
  subtitleKey: 'transactionPrint.sources.stockOpnameOrder.subtitle',
  searchPlaceholderKey: 'transactionPrint.sources.stockOpnameOrder.searchPlaceholder',
  sampleDocuments: SAMPLE_STOCK_OPNAME_ORDERS,
};

export const StockOpnameOrders: Component = () => {
  return <TransactionPrintWorkspace config={STOCK_OPNAME_ORDER_SOURCE_CONFIG} />;
};
