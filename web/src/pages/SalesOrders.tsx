import { type Component } from 'solid-js';
import { TransactionPrintWorkspace } from '~/components/transactionPrint/TransactionPrintWorkspace';
import type {
  TransactionSampleDocument,
  TransactionSourceConfig,
} from '~/components/transactionPrint/types';
import { ACCURATE_SOURCE } from '~/domain';

const SAMPLE_SALES_ORDERS: readonly TransactionSampleDocument[] = [
  {
    header: {
      id: 93001,
      number: 'SO-EX-2026-0301',
      date: '14/04/2026',
      description: 'Pesanan Perlengkapan Kantor & Alat Tulis Ritel (Contoh)',
      statusName: 'Disetujui',
    },
    supplierOrParty: 'PT Karya Mandiri Office (Contoh)',
    warehouseName: 'Gudang Utama A',
    items: [
      {
        itemId: 53101,
        itemNo: 'ATK-101',
        itemName: 'Kertas HVS A4 80gsm Rim Isi 500 Lembar',
        upcNo: '8997013300108',
        quantity: 10,
        unitName: 'Pack',
        unitPrice: 52000,
        documentType: ACCURATE_SOURCE.SALES_ORDER,
        documentNumber: 'SO-EX-2026-0301',
      },
      {
        itemId: 53102,
        itemNo: 'ATK-102',
        itemName: 'Pulpen Gel Hitam 0.5mm Kotak Isi 12',
        upcNo: '8997013300207',
        quantity: 6,
        unitName: 'Lusin',
        unitPrice: 34000,
        documentType: ACCURATE_SOURCE.SALES_ORDER,
        documentNumber: 'SO-EX-2026-0301',
      },
      {
        itemId: 53103,
        itemNo: 'ATK-103',
        itemName: 'Map Ordner Bantex Folio Biru 7cm',
        upcNo: '8997013300306',
        quantity: 4,
        unitName: 'Pcs',
        unitPrice: 28500,
        documentType: ACCURATE_SOURCE.SALES_ORDER,
        documentNumber: 'SO-EX-2026-0301',
      },
    ],
  },
  {
    header: {
      id: 93002,
      number: 'SO-EX-2026-0302',
      date: '15/04/2026',
      description: 'Pesanan Perangkat Kasir & Aksesoris POS Toko Cabang',
      statusName: 'Sebagian Dikirim',
    },
    supplierOrParty: 'CV Ritel Jaya Bersama (Contoh)',
    warehouseName: 'Gudang Utara',
    items: [
      {
        itemId: 53201,
        itemNo: 'POS-201',
        itemName: 'Kertas Struk Thermal 80x80mm Pack Isi 10',
        upcNo: '8998824300101',
        quantity: 5,
        unitName: 'Pack',
        unitPrice: 95000,
        documentType: ACCURATE_SOURCE.SALES_ORDER,
        documentNumber: 'SO-EX-2026-0302',
      },
      {
        itemId: 53202,
        itemNo: 'POS-NO-UPC-02',
        itemName: 'Dudukan Scanner Barcode Fleksibel Universal (Tanpa UPC)',
        upcNo: '',
        quantity: 2,
        unitName: 'Unit',
        unitPrice: 125000,
        documentType: ACCURATE_SOURCE.SALES_ORDER,
        documentNumber: 'SO-EX-2026-0302',
      },
    ],
  },
  {
    header: {
      id: 93003,
      number: 'SO-EX-2026-0303',
      date: '16/04/2026',
      description: 'Pesanan Elektronik Kantor & Kabel Jaringan',
      statusName: 'Menunggu Diproses',
    },
    supplierOrParty: 'Toko Sinar Elektro (Contoh)',
    warehouseName: 'Gudang Selatan',
    items: [
      {
        itemId: 53301,
        itemNo: 'ELN-301',
        itemName: 'Mouse Wireless Optical 2.4GHz Hitam',
        upcNo: '8995502301012',
        quantity: 4,
        unitName: 'Unit',
        unitPrice: 89000,
        documentType: ACCURATE_SOURCE.SALES_ORDER,
        documentNumber: 'SO-EX-2026-0303',
      },
      {
        itemId: 53302,
        itemNo: 'ELN-302',
        itemName: 'Kabel LAN Patch Cord Cat6 5 Meter',
        upcNo: '8995502302026',
        quantity: 8,
        unitName: 'Pcs',
        unitPrice: 32000,
        documentType: ACCURATE_SOURCE.SALES_ORDER,
        documentNumber: 'SO-EX-2026-0303',
      },
    ],
  },
  {
    header: {
      id: 93004,
      number: 'SO-EX-2026-0304',
      date: '17/04/2026',
      description: 'Pesanan Stapler, Binder Clip & Perlengkapan Meja',
      statusName: 'Disetujui',
    },
    supplierOrParty: 'PT Global Niaga Sejahtera (Contoh)',
    warehouseName: 'Gudang Utama A',
    items: [
      {
        itemId: 53401,
        itemNo: 'ATK-401',
        itemName: 'Stapler Meja HD-50 + Isi Staples No. 3',
        upcNo: '8996102401015',
        quantity: 3,
        unitName: 'Pack',
        unitPrice: 44000,
        documentType: ACCURATE_SOURCE.SALES_ORDER,
        documentNumber: 'SO-EX-2026-0304',
      },
      {
        itemId: 53402,
        itemNo: 'ATK-402',
        itemName: 'Binder Clip Hitam No. 200 Dus Isi 12',
        upcNo: '8996102402029',
        quantity: 5,
        unitName: 'Dus',
        unitPrice: 26500,
        documentType: ACCURATE_SOURCE.SALES_ORDER,
        documentNumber: 'SO-EX-2026-0304',
      },
    ],
  },
];

const SALES_ORDER_SOURCE_CONFIG: TransactionSourceConfig = {
  source: ACCURATE_SOURCE.SALES_ORDER,
  titleKey: 'transactionPrint.sources.salesOrder.title',
  subtitleKey: 'transactionPrint.sources.salesOrder.subtitle',
  searchPlaceholderKey: 'transactionPrint.sources.salesOrder.searchPlaceholder',
  partyLabelKey: 'transactionPrint.sources.salesOrder.partyLabel',
  sampleDocuments: SAMPLE_SALES_ORDERS,
};

export const SalesOrders: Component = () => {
  return <TransactionPrintWorkspace config={SALES_ORDER_SOURCE_CONFIG} />;
};
