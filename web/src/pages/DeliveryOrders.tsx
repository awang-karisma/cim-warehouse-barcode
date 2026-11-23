import { type Component } from 'solid-js';
import { TransactionPrintWorkspace } from '~/components/transactionPrint/TransactionPrintWorkspace';
import type {
  TransactionSampleDocument,
  TransactionSourceConfig,
} from '~/components/transactionPrint/types';
import { ACCURATE_SOURCE } from '~/domain';

const SAMPLE_DELIVERY_ORDERS: readonly TransactionSampleDocument[] = [
  {
    header: {
      id: 94001,
      number: 'DO-EX-2026-0401',
      date: '14/04/2026',
      description: 'Pengiriman Stok Minuman & Snack Supermarket Barat (Contoh)',
      statusName: 'Terkirim',
    },
    supplierOrParty: 'PT Pasar Segar Sentosa (Contoh)',
    warehouseName: 'Gudang Dingin & Kering',
    items: [
      {
        itemId: 54101,
        itemNo: 'KRM-101',
        itemName: 'Biskuit Wafer Coklat 150g Karton Isi 20',
        upcNo: '8997014400105',
        quantity: 6,
        unitName: 'Dus',
        unitPrice: 124000,
        documentType: ACCURATE_SOURCE.DELIVERY_ORDER,
        documentNumber: 'DO-EX-2026-0401',
      },
      {
        itemId: 54102,
        itemNo: 'KRM-102',
        itemName: 'Susu UHT Full Cream 1 Liter Karton Isi 12',
        upcNo: '8997014400204',
        quantity: 4,
        unitName: 'Dus',
        unitPrice: 216000,
        documentType: ACCURATE_SOURCE.DELIVERY_ORDER,
        documentNumber: 'DO-EX-2026-0401',
      },
      {
        itemId: 54103,
        itemNo: 'KRM-103',
        itemName: 'Minyak Goreng Kemasan Pouch 2 Liter Isi 6',
        upcNo: '8997014400303',
        quantity: 3,
        unitName: 'Dus',
        unitPrice: 204000,
        documentType: ACCURATE_SOURCE.DELIVERY_ORDER,
        documentNumber: 'DO-EX-2026-0401',
      },
    ],
  },
  {
    header: {
      id: 94002,
      number: 'DO-EX-2026-0402',
      date: '15/04/2026',
      description: 'Pengiriman Bahan Kebersihan & Sanitasi Outlet',
      statusName: 'Sebagian Terkirim',
    },
    supplierOrParty: 'CV Bersih Mandiri (Contoh)',
    warehouseName: 'Gudang Utama A',
    items: [
      {
        itemId: 54201,
        itemNo: 'CLN-201',
        itemName: 'Cairan Pembersih Lantai Jerigen 5 Liter',
        upcNo: '8998825400108',
        quantity: 5,
        unitName: 'Unit',
        unitPrice: 68000,
        documentType: ACCURATE_SOURCE.DELIVERY_ORDER,
        documentNumber: 'DO-EX-2026-0402',
      },
      {
        itemId: 54202,
        itemNo: 'CLN-NO-UPC-03',
        itemName: 'Kain Pel Microfiber Refill 40cm (Tanpa UPC)',
        upcNo: '',
        quantity: 4,
        unitName: 'Pcs',
        unitPrice: 35000,
        documentType: ACCURATE_SOURCE.DELIVERY_ORDER,
        documentNumber: 'DO-EX-2026-0402',
      },
    ],
  },
  {
    header: {
      id: 94003,
      number: 'DO-EX-2026-0403',
      date: '16/04/2026',
      description: 'Pengiriman Sabun Cuci Tangan & Tisu Dapur Hotel',
      statusName: 'Draf',
    },
    supplierOrParty: 'Hotel Harmoni Nusantara (Contoh)',
    warehouseName: 'Gudang Selatan',
    items: [
      {
        itemId: 54301,
        itemNo: 'CLN-301',
        itemName: 'Tisu Hand Towel Lipat Pack Isi 150 Lembar',
        upcNo: '8995503401019',
        quantity: 10,
        unitName: 'Pack',
        unitPrice: 19500,
        documentType: ACCURATE_SOURCE.DELIVERY_ORDER,
        documentNumber: 'DO-EX-2026-0403',
      },
      {
        itemId: 54302,
        itemNo: 'CLN-302',
        itemName: 'Sabun Cuci Tangan Refill Pouch 450ml',
        upcNo: '8995503402023',
        quantity: 8,
        unitName: 'Pcs',
        unitPrice: 21000,
        documentType: ACCURATE_SOURCE.DELIVERY_ORDER,
        documentNumber: 'DO-EX-2026-0403',
      },
    ],
  },
  {
    header: {
      id: 94004,
      number: 'DO-EX-2026-0404',
      date: '17/04/2026',
      description: 'Pengiriman Paket Kopi & Teh Pantry Perusahaan',
      statusName: 'Terkirim',
    },
    supplierOrParty: 'PT Graha Karya Perkasa (Contoh)',
    warehouseName: 'Gudang Utara',
    items: [
      {
        itemId: 54401,
        itemNo: 'KRM-401',
        itemName: 'Kopi Bubuk Robusta Kemasan 500g',
        upcNo: '8996103401012',
        quantity: 4,
        unitName: 'Pack',
        unitPrice: 58000,
        documentType: ACCURATE_SOURCE.DELIVERY_ORDER,
        documentNumber: 'DO-EX-2026-0404',
      },
      {
        itemId: 54402,
        itemNo: 'KRM-402',
        itemName: 'Gula Pasir Kemasan 1kg Karton Isi 10',
        upcNo: '8996103402026',
        quantity: 2,
        unitName: 'Dus',
        unitPrice: 175000,
        documentType: ACCURATE_SOURCE.DELIVERY_ORDER,
        documentNumber: 'DO-EX-2026-0404',
      },
    ],
  },
];

const DELIVERY_ORDER_SOURCE_CONFIG: TransactionSourceConfig = {
  source: ACCURATE_SOURCE.DELIVERY_ORDER,
  titleKey: 'transactionPrint.sources.deliveryOrder.title',
  subtitleKey: 'transactionPrint.sources.deliveryOrder.subtitle',
  searchPlaceholderKey: 'transactionPrint.sources.deliveryOrder.searchPlaceholder',
  partyLabelKey: 'transactionPrint.sources.deliveryOrder.partyLabel',
  sampleDocuments: SAMPLE_DELIVERY_ORDERS,
};

export const DeliveryOrders: Component = () => {
  return <TransactionPrintWorkspace config={DELIVERY_ORDER_SOURCE_CONFIG} />;
};
