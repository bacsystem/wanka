package domain

import (
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// Tipos de documento soportados por org.document_series (#33, #47).
const (
	DocTypeFactura      = "01"
	DocTypeBoleta       = "03"
	DocTypeNotaCredito  = "07"
	DocTypeNotaDebito   = "08"
	DocTypeGuiaRemision = "09"
	DocTypeProforma     = "PRO"
	DocTypeOrdenCompra  = "OC"
	DocTypeCierreZ      = "Z"
	DocTypeOrdenTrabajo = "OT"
)

// Estados de sincronización con el proveedor (Khipu).
const (
	ProviderStatusNotApplicable = "not_applicable"
	ProviderStatusPending       = "pending"
	ProviderStatusSynced        = "synced"
	ProviderStatusConflict      = "conflict"
)

// Errores tipados del dominio de numeración correlativa (#47).
var (
	// ErrSeriesNotFound indica que la serie solicitada no existe para la caja/tipo o ID especificado.
	ErrSeriesNotFound = errors.New("series not found")

	// ErrSeriesInactive indica que la serie existe pero no está habilitada (active = false).
	ErrSeriesInactive = errors.New("series is inactive")

	// ErrSeriesNotSynced indica que una serie CPE aún no ha completado la sincronización inicial con Khipu.
	ErrSeriesNotSynced = errors.New("series is not synced with provider")

	// ErrSeriesConflict indica que la serie está congelada por discrepancia con el proveedor.
	ErrSeriesConflict = errors.New("series is in conflict")

	// ErrGREUsesProviderNumbering indica que la Guía de Remisión (09) utiliza numeración del proveedor (ADR 0013 §5).
	ErrGREUsesProviderNumbering = errors.New("gre uses provider numbering")

	// ErrSeriesAlreadyFrozen indica que la serie ya se encontraba congelada.
	ErrSeriesAlreadyFrozen = errors.New("series is already frozen")
)

// IsCPEDocType determina si el tipo de comprobante está sujeto a emisión electrónica SUNAT.
func IsCPEDocType(docType string) bool {
	switch docType {
	case DocTypeFactura, DocTypeBoleta, DocTypeNotaCredito, DocTypeNotaDebito, DocTypeGuiaRemision:
		return true
	default:
		return false
	}
}

// ComputeProviderStatus calcula el valor de providerStatus según el tipo de documento y marcas temporales.
func ComputeProviderStatus(docType string, syncedAt, conflictAt *time.Time) string {
	if !IsCPEDocType(docType) {
		return ProviderStatusNotApplicable
	}
	if conflictAt != nil {
		return ProviderStatusConflict
	}
	if syncedAt != nil {
		return ProviderStatusSynced
	}
	return ProviderStatusPending
}

// AssignedNumber representa un número correlativo asignado de forma consecutiva e inmutable.
type AssignedNumber struct {
	SeriesID pgtype.UUID
	DocType  string
	Series   string
	Number   int64
}

// DocumentSeries modela una serie de comprobantes o documentos internos en org.document_series.
type DocumentSeries struct {
	ID                 pgtype.UUID
	TenantID           pgtype.UUID
	SiteID             pgtype.UUID
	CashRegisterID     pgtype.UUID
	DocType            string
	Series             string
	LastNumber         int64
	Active             bool
	ProviderSyncedAt   *time.Time
	ProviderConflictAt *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
	ProviderStatus     string
}

// RemoteSeries representa la información de numeración y anexo reportada por Khipu durante la conciliación.
type RemoteSeries struct {
	DocType      string
	Series       string
	UltimoNumero int64
	Anexo        string
}

// ReconcileReport contiene los resultados agregados de la conciliación diaria de series con Khipu.
type ReconcileReport struct {
	Processed   int
	FrozenCount int
	Alerts      []string
}
