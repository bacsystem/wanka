package domain

import (
	"context"
	"errors"
	"regexp"
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

// Errores tipados del dominio de empresas y numeración correlativa (#33, #47).
var (
	// ErrSeriesNotFound indica que la serie solicitada no existe.
	ErrSeriesNotFound = errors.New("series not found")

	// ErrSeriesInactive indica que la serie existe pero no está habilitada.
	ErrSeriesInactive = errors.New("series is inactive")

	// ErrSeriesNotSynced indica que una serie CPE aún no ha completado la sincronización inicial con Khipu.
	ErrSeriesNotSynced = errors.New("series is not synced with provider")

	// ErrSeriesConflict indica que la serie está congelada por discrepancia con el proveedor.
	ErrSeriesConflict = errors.New("series is in conflict")

	// ErrGREUsesProviderNumbering indica que la Guía de Remisión (09) utiliza numeración del proveedor (ADR 0013 §5).
	ErrGREUsesProviderNumbering = errors.New("gre uses provider numbering")

	// ErrSeriesAlreadyFrozen indica que la serie ya se encontraba congelada.
	ErrSeriesAlreadyFrozen = errors.New("series is already frozen")

	// ErrSiteNotFound indica que el local o sede no existe.
	ErrSiteNotFound = errors.New("site not found")

	// ErrCashRegisterNotFound indica que la caja registradora no existe.
	ErrCashRegisterNotFound = errors.New("cash register not found")

	// ErrDuplicateCode indica que el código ya se encuentra registrado en la empresa.
	ErrDuplicateCode = errors.New("code already in use")

	// ErrDuplicateAnnex indica que el anexo SUNAT ya se encuentra asignado a otro local de la empresa.
	ErrDuplicateAnnex = errors.New("sunat annex already in use")

	// ErrDuplicateSeries indica que la serie ya existe para ese tipo en la empresa.
	ErrDuplicateSeries = errors.New("series already in use")

	// ErrAnnexImmutableWithSyncedSeries indica que no se puede cambiar el anexo de un local con series sincronizadas.
	ErrAnnexImmutableWithSyncedSeries = errors.New("cannot change sunat annex of site with synced series")

	// ErrCPERequiresSunatAnnex indica que un comprobante electrónico exige un local con anexo SUNAT.
	ErrCPERequiresSunatAnnex = errors.New("cpe series requires site with sunat annex")

	// ErrSiteInactive indica que el local está inactivo.
	ErrSiteInactive = errors.New("site is inactive")

	// ErrCashRegisterInactive indica que la caja está inactiva.
	ErrCashRegisterInactive = errors.New("cash register is inactive")

	// ErrCashRegisterSiteMismatch indica que la caja registradora pertenece a otro local.
	ErrCashRegisterSiteMismatch = errors.New("cash register belongs to another site")

	// ErrLastNumberImmutableWhenSynced indica que el correlativo no puede modificarse una vez sincronizado o en conflicto.
	ErrLastNumberImmutableWhenSynced = errors.New("last number cannot be changed when synced or in conflict")

	// ErrSiteImmutableWhenSynced indica que el local no puede modificarse si la serie ya está sincronizada con Khipu.
	ErrSiteImmutableWhenSynced = errors.New("site cannot be changed when series is synced")

	// ErrInvalidCode indica que el código no cumple con el formato permitido.
	ErrInvalidCode = errors.New("invalid code format")

	// ErrInvalidSeriesFormat indica que el código de serie no cumple el formato reglamentario para su tipo.
	ErrInvalidSeriesFormat = errors.New("invalid series format for document type")

	// ErrInvalidAnnexFormat indica que el código de anexo SUNAT no tiene el formato de 4 dígitos.
	ErrInvalidAnnexFormat = errors.New("invalid sunat annex format")

	// ErrPlanLimitReached indica que se ha alcanzado el límite permitido de sedes para el plan contratado.
	ErrPlanLimitReached = errors.New("plan limit reached")
)

var (
	reFactura    = regexp.MustCompile(`^F[A-Z0-9]{3}$`)
	reBoleta     = regexp.MustCompile(`^B[A-Z0-9]{3}$`)
	reNotas      = regexp.MustCompile(`^[FB][A-Z0-9]{3}$`)
	reGuia       = regexp.MustCompile(`^T[A-Z0-9]{3}$`)
	reInterno    = regexp.MustCompile(`^[A-Z0-9]{2,10}$`)
	reAnexoSunat = regexp.MustCompile(`^[0-9]{4}$`)
)

// ValidateSeriesFormat valida el formato del código de la serie según su tipo de documento.
func ValidateSeriesFormat(docType, series string) error {
	var valid bool
	switch docType {
	case DocTypeFactura:
		valid = reFactura.MatchString(series)
	case DocTypeBoleta:
		valid = reBoleta.MatchString(series)
	case DocTypeNotaCredito, DocTypeNotaDebito:
		valid = reNotas.MatchString(series)
	case DocTypeGuiaRemision:
		valid = reGuia.MatchString(series)
	default:
		valid = reInterno.MatchString(series)
	}
	if !valid {
		return ErrInvalidSeriesFormat
	}
	return nil
}

// ValidateSunatAnnex valida que el anexo SUNAT esté formado por exactamente 4 dígitos.
func ValidateSunatAnnex(annex *string) error {
	if annex == nil || *annex == "" {
		return nil
	}
	if !reAnexoSunat.MatchString(*annex) {
		return ErrInvalidAnnexFormat
	}
	return nil
}

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

// Site representa una sede o local de la empresa en org.sites.
type Site struct {
	ID                    pgtype.UUID
	TenantID              pgtype.UUID
	Code                  string
	Name                  string
	SunatAnnex            *string
	Address               *string
	Active                bool
	Version               int64
	ProviderSyncedVersion *int64
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// CashRegister modela una caja registradora en org.cash_registers.
type CashRegister struct {
	ID        pgtype.UUID
	TenantID  pgtype.UUID
	SiteID    pgtype.UUID
	Code      string
	Active    bool
	CreatedAt time.Time
	UpdatedAt time.Time
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

// RemoteSeries representa la información de numeración y anexo reportada por Khipu.
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

// ProviderClient define el cliente para consultar el estado de series en Khipu durante la reactivación.
type ProviderClient interface {
	GetSeries(ctx context.Context, tenantID pgtype.UUID) ([]RemoteSeries, error)
}
