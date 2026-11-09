package model

type UserRole string

const (
	RoleGuest      UserRole = "guest"
	RoleUser       UserRole = "user"
	RoleAdmin      UserRole = "admin"
	RoleSuperadmin UserRole = "superadmin"
)

type Locale string

const (
	LocaleEnglish    Locale = "en"
	LocaleIndonesian Locale = "id"
)

type BarcodeSymbology string

const (
	BarcodeCode128    BarcodeSymbology = "CODE128"
	BarcodeCode39     BarcodeSymbology = "CODE39"
	BarcodeEAN13      BarcodeSymbology = "EAN13"
	BarcodeQRCode     BarcodeSymbology = "QRCODE"
	BarcodeDataMatrix BarcodeSymbology = "DATAMATRIX"
)

type BarcodeHeight string

const (
	BarcodeHeightCompact BarcodeHeight = "compact"
	BarcodeHeightMedium  BarcodeHeight = "medium"
	BarcodeHeightLarge   BarcodeHeight = "large"
)

type BarcodeFontSize string

const (
	BarcodeFontSmall  BarcodeFontSize = "small"
	BarcodeFontMedium BarcodeFontSize = "medium"
	BarcodeFontLarge  BarcodeFontSize = "large"
)

type BarcodeQuantityMode string

const (
	BarcodeQuantitySingle   BarcodeQuantityMode = "single"
	BarcodeQuantityMultiply BarcodeQuantityMode = "multiply"
)
