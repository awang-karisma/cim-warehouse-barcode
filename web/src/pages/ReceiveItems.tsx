import { type Component } from 'solid-js';
import { TransactionPrintWorkspace } from '~/components/transactionPrint/TransactionPrintWorkspace';
import type {
  TransactionSampleDocument,
  TransactionSourceConfig,
} from '~/components/transactionPrint/types';
import { ACCURATE_SOURCE } from '~/domain';

const SAMPLE_RECEIVE_ITEMS: readonly TransactionSampleDocument[] = [
  {
    header: {
      id: 92001,
      number: 'RI-EX-2026-0201',
      date: '14/04/2026',
      description: 'Penerimaan Kemasan Kardus & Lakban Masuk (Contoh)',
      statusName: 'Terproses',
    },
    supplierOrParty: 'PT Kemasan Nusantara (Contoh)',
    warehouseName: 'Gudang Utama A',
    items: [
      {
        itemId: 52101,
        itemNo: 'RCV-101',
        itemName: 'Lakban Packing Coklat 48mm x 90 Yard',
        upcNo: '8997012200101',
        quantity: 6,
        unitName: 'Roll',
        unitPrice: 14000,
        documentType: ACCURATE_SOURCE.RECEIVE_ITEM,
        documentNumber: 'RI-EX-2026-0201',
      },
      {
        itemId: 52102,
        itemNo: 'RCV-102',
        itemName: 'Kardus Gelombang 40x30x25 cm Double Wall',
        upcNo: '8997012200200',
        quantity: 4,
        unitName: 'Dus',
        unitPrice: 82000,
        documentType: ACCURATE_SOURCE.RECEIVE_ITEM,
        documentNumber: 'RI-EX-2026-0201',
      },
      {
        itemId: 52103,
        itemNo: 'RCV-103',
        itemName: 'Stretch Film Bening 50cm x 300m 17 Micron',
        upcNo: '8997012200309',
        quantity: 2,
        unitName: 'Roll',
        unitPrice: 105000,
        documentType: ACCURATE_SOURCE.RECEIVE_ITEM,
        documentNumber: 'RI-EX-2026-0201',
      },
    ],
  },
  {
    header: {
      id: 92002,
      number: 'RI-EX-2026-0202',
      date: '15/04/2026',
      description: 'Penerimaan Material Label & Plastik Shrink Gudang Utara',
      statusName: 'Sebagian Diproses',
    },
    supplierOrParty: 'CV Plastik Sentosa (Contoh)',
    warehouseName: 'Gudang Utara',
    items: [
      {
        itemId: 52201,
        itemNo: 'RCV-201',
        itemName: 'Plastik Shrink POF Roll 35cm x 1500m',
        upcNo: '8998823200104',
        quantity: 3,
        unitName: 'Roll',
        unitPrice: 275000,
        documentType: ACCURATE_SOURCE.RECEIVE_ITEM,
        documentNumber: 'RI-EX-2026-0202',
      },
      {
        itemId: 52202,
        itemNo: 'RCV-NO-UPC-01',
        itemName: 'Kertas Label Barcode Direct Thermal 100x150mm (Tanpa UPC)',
        upcNo: '',
        quantity: 5,
        unitName: 'Pack',
        unitPrice: 54000,
        documentType: ACCURATE_SOURCE.RECEIVE_ITEM,
        documentNumber: 'RI-EX-2026-0202',
      },
    ],
  },
  {
    header: {
      id: 92003,
      number: 'RI-EX-2026-0203',
      date: '16/04/2026',
      description: 'Penerimaan Tali Strapping & Segel Kemasan',
      statusName: 'Draf',
    },
    supplierOrParty: 'PT Mitra Packing Abadi (Contoh)',
    warehouseName: 'Gudang Selatan',
    items: [
      {
        itemId: 52301,
        itemNo: 'RCV-301',
        itemName: 'Tali Strapping Band PP Kuning 12mm x 1000m',
        upcNo: '8995501301015',
        quantity: 2,
        unitName: 'Roll',
        unitPrice: 118000,
        documentType: ACCURATE_SOURCE.RECEIVE_ITEM,
        documentNumber: 'RI-EX-2026-0203',
      },
      {
        itemId: 52302,
        itemNo: 'RCV-302',
        itemName: 'Klem Besi Strapping Band 12mm Isi 1000',
        upcNo: '8995501302029',
        quantity: 3,
        unitName: 'Pack',
        unitPrice: 46000,
        documentType: ACCURATE_SOURCE.RECEIVE_ITEM,
        documentNumber: 'RI-EX-2026-0203',
      },
    ],
  },
  {
    header: {
      id: 92004,
      number: 'RI-EX-2026-0204',
      date: '17/04/2026',
      description: 'Penerimaan Kotak Arsip & Kantong Packing Zona B',
      statusName: 'Terproses',
    },
    supplierOrParty: 'UD Karton Sejahtera (Contoh)',
    warehouseName: 'Gudang Utama A',
    items: [
      {
        itemId: 52401,
        itemNo: 'RCV-401',
        itemName: 'Polymailer Hitam Waterproof 30x40 cm Isi 100',
        upcNo: '8996101401018',
        quantity: 8,
        unitName: 'Pack',
        unitPrice: 38500,
        documentType: ACCURATE_SOURCE.RECEIVE_ITEM,
        documentNumber: 'RI-EX-2026-0204',
      },
      {
        itemId: 52402,
        itemNo: 'RCV-402',
        itemName: 'Karton Partisi Sekat Botol 12 Slot',
        upcNo: '8996101402022',
        quantity: 4,
        unitName: 'Dus',
        unitPrice: 64000,
        documentType: ACCURATE_SOURCE.RECEIVE_ITEM,
        documentNumber: 'RI-EX-2026-0204',
      },
    ],
  },
];

const RECEIVE_ITEM_SOURCE_CONFIG: TransactionSourceConfig = {
  source: ACCURATE_SOURCE.RECEIVE_ITEM,
  titleKey: 'transactionPrint.sources.receiveItem.title',
  subtitleKey: 'transactionPrint.sources.receiveItem.subtitle',
  searchPlaceholderKey: 'transactionPrint.sources.receiveItem.searchPlaceholder',
  partyLabelKey: 'transactionPrint.sources.receiveItem.partyLabel',
  sampleDocuments: SAMPLE_RECEIVE_ITEMS,
};

export const ReceiveItems: Component = () => {
  return <TransactionPrintWorkspace config={RECEIVE_ITEM_SOURCE_CONFIG} />;
};
