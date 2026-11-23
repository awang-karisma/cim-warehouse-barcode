import { type Component } from 'solid-js';
import { TransactionPrintWorkspace } from '~/components/transactionPrint/TransactionPrintWorkspace';
import type {
  TransactionSampleDocument,
  TransactionSourceConfig,
} from '~/components/transactionPrint/types';
import { ACCURATE_SOURCE } from '~/domain';

const SAMPLE_STOCK_OPNAME_RESULTS: readonly TransactionSampleDocument[] = [
  {
    header: {
      id: 96001,
      number: 'SOR-EX-2026-0601',
      date: '14/04/2026',
      description: 'Hasil Opname Komponen Sensor & Relay Otomasi (Contoh)',
      statusName: 'Disetujui',
    },
    warehouseName: 'Gudang Utama A',
    items: [
      {
        itemId: 56101,
        itemNo: 'CMP-101',
        itemName: 'Sensor Proximity Induktif PNP NO 12-24VDC',
        upcNo: '8997016600109',
        quantity: 4,
        unitName: 'Pcs',
        unitPrice: 112000,
        documentType: ACCURATE_SOURCE.STOCK_OPNAME_RESULT,
        documentNumber: 'SOR-EX-2026-0601',
      },
      {
        itemId: 56102,
        itemNo: 'CMP-102',
        itemName: 'Relay Kontaktor 24VDC 8 Pin + Socket',
        upcNo: '8997016600208',
        quantity: 6,
        unitName: 'Unit',
        unitPrice: 48000,
        documentType: ACCURATE_SOURCE.STOCK_OPNAME_RESULT,
        documentNumber: 'SOR-EX-2026-0601',
      },
      {
        itemId: 56103,
        itemNo: 'CMP-103',
        itemName: 'MCB 1 Phase 16A 6kA Standar SNI',
        upcNo: '8997016600307',
        quantity: 5,
        unitName: 'Pcs',
        unitPrice: 64000,
        documentType: ACCURATE_SOURCE.STOCK_OPNAME_RESULT,
        documentNumber: 'SOR-EX-2026-0601',
      },
    ],
  },
  {
    header: {
      id: 96002,
      number: 'SOR-EX-2026-0602',
      date: '15/04/2026',
      description: 'Hasil Opname Suku Cadang Conveyor & Roller Gudang',
      statusName: 'Terproses',
    },
    warehouseName: 'Gudang Utara',
    items: [
      {
        itemId: 56201,
        itemNo: 'SPR-201',
        itemName: 'Bearing Bola 6204-2RS Seal Karet Baja',
        upcNo: '8998827600102',
        quantity: 10,
        unitName: 'Pcs',
        unitPrice: 34500,
        documentType: ACCURATE_SOURCE.STOCK_OPNAME_RESULT,
        documentNumber: 'SOR-EX-2026-0602',
      },
      {
        itemId: 56202,
        itemNo: 'SPR-NO-UPC-05',
        itemName: 'V-Belt Karet Tipe A-42 Mesin Pengemas (Tanpa UPC)',
        upcNo: '',
        quantity: 3,
        unitName: 'Pcs',
        unitPrice: 52000,
        documentType: ACCURATE_SOURCE.STOCK_OPNAME_RESULT,
        documentNumber: 'SOR-EX-2026-0602',
      },
    ],
  },
  {
    header: {
      id: 96003,
      number: 'SOR-EX-2026-0603',
      date: '16/04/2026',
      description: 'Hasil Opname Adaptor Power Supply & Kabel Kontrol',
      statusName: 'Draf',
    },
    warehouseName: 'Gudang Selatan',
    items: [
      {
        itemId: 56301,
        itemNo: 'CMP-301',
        itemName: 'Power Supply Switching 24V 5A DIN Rail',
        upcNo: '8995505601013',
        quantity: 2,
        unitName: 'Unit',
        unitPrice: 175000,
        documentType: ACCURATE_SOURCE.STOCK_OPNAME_RESULT,
        documentNumber: 'SOR-EX-2026-0603',
      },
      {
        itemId: 56302,
        itemNo: 'CMP-302',
        itemName: 'Terminal Block Kabel 12 Pin 15A',
        upcNo: '8995505602027',
        quantity: 7,
        unitName: 'Pack',
        unitPrice: 29000,
        documentType: ACCURATE_SOURCE.STOCK_OPNAME_RESULT,
        documentNumber: 'SOR-EX-2026-0603',
      },
    ],
  },
  {
    header: {
      id: 96004,
      number: 'SOR-EX-2026-0604',
      date: '17/04/2026',
      description: 'Hasil Opname Komponen Scanner & Printer Barcode',
      statusName: 'Disetujui',
    },
    warehouseName: 'Gudang Utama A',
    items: [
      {
        itemId: 56401,
        itemNo: 'SPR-401',
        itemName: 'Platen Roller Karet Pengganti Printer Thermal 4 Inch',
        upcNo: '8996105601016',
        quantity: 2,
        unitName: 'Unit',
        unitPrice: 165000,
        documentType: ACCURATE_SOURCE.STOCK_OPNAME_RESULT,
        documentNumber: 'SOR-EX-2026-0604',
      },
      {
        itemId: 56402,
        itemNo: 'SPR-402',
        itemName: 'Kabel USB Coiled Scanner Barcode 2 Meter',
        upcNo: '8996105602020',
        quantity: 4,
        unitName: 'Pcs',
        unitPrice: 65000,
        documentType: ACCURATE_SOURCE.STOCK_OPNAME_RESULT,
        documentNumber: 'SOR-EX-2026-0604',
      },
    ],
  },
];

const STOCK_OPNAME_RESULT_SOURCE_CONFIG: TransactionSourceConfig = {
  source: ACCURATE_SOURCE.STOCK_OPNAME_RESULT,
  titleKey: 'transactionPrint.sources.stockOpnameResult.title',
  subtitleKey: 'transactionPrint.sources.stockOpnameResult.subtitle',
  searchPlaceholderKey: 'transactionPrint.sources.stockOpnameResult.searchPlaceholder',
  sampleDocuments: SAMPLE_STOCK_OPNAME_RESULTS,
};

export const StockOpnameResults: Component = () => {
  return <TransactionPrintWorkspace config={STOCK_OPNAME_RESULT_SOURCE_CONFIG} />;
};
