package permission

// Código de cada permiso de tenant, en el orden de api/resources.md. Los módulos usan estas
// constantes (un código inexistente no compila); la paridad con resources.md y con la migración
// la prueba catalog_test.go. Para agregar uno: primero resources.md y su migración (identidad),
// luego aquí, en la constante y en all.
const (
	// identidad
	IdentidadUsersRead            Code = "identidad.users:read"
	IdentidadUsersCreate          Code = "identidad.users:create"
	IdentidadUsersUpdate          Code = "identidad.users:update"
	IdentidadRolesRead            Code = "identidad.roles:read"
	IdentidadRolesCreate          Code = "identidad.roles:create"
	IdentidadRolesUpdate          Code = "identidad.roles:update"
	IdentidadSessionsRead         Code = "identidad.sessions:read"
	IdentidadSessionsVoid         Code = "identidad.sessions:void"
	IdentidadSupportAccessRead    Code = "identidad.support_access:read"
	IdentidadSupportAccessApprove Code = "identidad.support_access:approve"

	// empresas
	EmpresasConfigRead       Code = "empresas.config:read"
	EmpresasConfigUpdate     Code = "empresas.config:update"
	EmpresasSitesRead        Code = "empresas.sites:read"
	EmpresasSitesCreate      Code = "empresas.sites:create"
	EmpresasSitesUpdate      Code = "empresas.sites:update"
	EmpresasSeriesRead       Code = "empresas.series:read"
	EmpresasSeriesCreate     Code = "empresas.series:create"
	EmpresasSeriesUpdate     Code = "empresas.series:update"
	EmpresasEinvoicingRead   Code = "empresas.einvoicing:read"
	EmpresasEinvoicingUpdate Code = "empresas.einvoicing:update"

	// clientes
	ClientesCustomersRead   Code = "clientes.customers:read"
	ClientesCustomersCreate Code = "clientes.customers:create"
	ClientesCustomersUpdate Code = "clientes.customers:update"
	ClientesCustomersExport Code = "clientes.customers:export"
	ClientesCustomersImport Code = "clientes.customers:import"

	// catalogo
	CatalogoItemsRead        Code = "catalogo.items:read"
	CatalogoItemsCreate      Code = "catalogo.items:create"
	CatalogoItemsUpdate      Code = "catalogo.items:update"
	CatalogoItemsImport      Code = "catalogo.items:import"
	CatalogoItemsExport      Code = "catalogo.items:export"
	CatalogoCategoriesRead   Code = "catalogo.categories:read"
	CatalogoCategoriesCreate Code = "catalogo.categories:create"
	CatalogoCategoriesUpdate Code = "catalogo.categories:update"
	CatalogoPricesRead       Code = "catalogo.prices:read"
	CatalogoPricesUpdate     Code = "catalogo.prices:update"

	// inventario
	InventarioWarehousesRead         Code = "inventario.warehouses:read"
	InventarioWarehousesCreate       Code = "inventario.warehouses:create"
	InventarioWarehousesUpdate       Code = "inventario.warehouses:update"
	InventarioStockRead              Code = "inventario.stock:read"
	InventarioStockUpdate            Code = "inventario.stock:update"
	InventarioStockExport            Code = "inventario.stock:export"
	InventarioTransfersRead          Code = "inventario.transfers:read"
	InventarioTransfersCreate        Code = "inventario.transfers:create"
	InventarioTransfersUpdate        Code = "inventario.transfers:update"
	InventarioCountsRead             Code = "inventario.counts:read"
	InventarioCountsCreate           Code = "inventario.counts:create"
	InventarioCountsUpdate           Code = "inventario.counts:update"
	InventarioCountsApprove          Code = "inventario.counts:approve"
	InventarioConsignmentsRead       Code = "inventario.consignments:read"
	InventarioConsignmentsCreate     Code = "inventario.consignments:create"
	InventarioConsignmentsUpdate     Code = "inventario.consignments:update"
	InventarioSupplierReturnsRead    Code = "inventario.supplier_returns:read"
	InventarioSupplierReturnsCreate  Code = "inventario.supplier_returns:create"
	InventarioSupplierReturnsApprove Code = "inventario.supplier_returns:approve"

	// ventas
	VentasPosRead          Code = "ventas.pos:read"
	VentasPosCreate        Code = "ventas.pos:create"
	VentasDiscountsCreate  Code = "ventas.discounts:create"
	VentasDiscountsApprove Code = "ventas.discounts:approve"
	VentasQuotesRead       Code = "ventas.quotes:read"
	VentasQuotesCreate     Code = "ventas.quotes:create"
	VentasQuotesUpdate     Code = "ventas.quotes:update"
	VentasQuotesSend       Code = "ventas.quotes:send"
	VentasReturnsCreate    Code = "ventas.returns:create"
	VentasReturnsApprove   Code = "ventas.returns:approve"

	// facturacion
	FacturacionCpeRead           Code = "facturacion.cpe:read"
	FacturacionCpeCreate         Code = "facturacion.cpe:create"
	FacturacionCpeVoid           Code = "facturacion.cpe:void"
	FacturacionCpeRetry          Code = "facturacion.cpe:retry"
	FacturacionCpeSend           Code = "facturacion.cpe:send"
	FacturacionCpeExport         Code = "facturacion.cpe:export"
	FacturacionCreditNotesCreate Code = "facturacion.credit_notes:create"
	FacturacionDebitNotesCreate  Code = "facturacion.debit_notes:create"
	FacturacionGreRead           Code = "facturacion.gre:read"
	FacturacionGreCreate         Code = "facturacion.gre:create"
	FacturacionGreUpdate         Code = "facturacion.gre:update"

	// compras
	ComprasSuppliersRead   Code = "compras.suppliers:read"
	ComprasSuppliersCreate Code = "compras.suppliers:create"
	ComprasSuppliersUpdate Code = "compras.suppliers:update"
	ComprasOrdersRead      Code = "compras.orders:read"
	ComprasOrdersCreate    Code = "compras.orders:create"
	ComprasOrdersUpdate    Code = "compras.orders:update"
	ComprasDocumentsRead   Code = "compras.documents:read"
	ComprasDocumentsCreate Code = "compras.documents:create"
	ComprasDocumentsUpdate Code = "compras.documents:update"
	ComprasDocumentsExport Code = "compras.documents:export"

	// finanzas
	FinanzasCashRead          Code = "finanzas.cash:read"
	FinanzasCashReadAll       Code = "finanzas.cash:read_all"
	FinanzasCashCreate        Code = "finanzas.cash:create"
	FinanzasCashUpdate        Code = "finanzas.cash:update"
	FinanzasCashApprove       Code = "finanzas.cash:approve"
	FinanzasCashExport        Code = "finanzas.cash:export"
	FinanzasReceivablesRead   Code = "finanzas.receivables:read"
	FinanzasReceivablesCreate Code = "finanzas.receivables:create"
	FinanzasReceivablesUpdate Code = "finanzas.receivables:update"
	FinanzasPayablesRead      Code = "finanzas.payables:read"
	FinanzasPayablesCreate    Code = "finanzas.payables:create"
	FinanzasPayablesUpdate    Code = "finanzas.payables:update"

	// reportes
	ReportesDashboardRead    Code = "reportes.dashboard:read"
	ReportesManagementRead   Code = "reportes.management:read"
	ReportesManagementExport Code = "reportes.management:export"
	ReportesTaxRead          Code = "reportes.tax:read"
	ReportesTaxExport        Code = "reportes.tax:export"

	// agenda
	AgendaAppointmentsRead   Code = "agenda.appointments:read"
	AgendaAppointmentsCreate Code = "agenda.appointments:create"
	AgendaAppointmentsUpdate Code = "agenda.appointments:update"

	// integraciones
	IntegracionesConfigRead   Code = "integraciones.config:read"
	IntegracionesConfigUpdate Code = "integraciones.config:update"

	// auditoria
	AuditoriaEventsRead   Code = "auditoria.events:read"
	AuditoriaEventsExport Code = "auditoria.events:export"

	// notificaciones
	NotificacionesRemindersRead   Code = "notificaciones.reminders:read"
	NotificacionesRemindersUpdate Code = "notificaciones.reminders:update"

	// lubricentro
	LubricentroVehiclesRead        Code = "lubricentro.vehicles:read"
	LubricentroVehiclesCreate      Code = "lubricentro.vehicles:create"
	LubricentroVehiclesUpdate      Code = "lubricentro.vehicles:update"
	LubricentroServiceOrdersRead   Code = "lubricentro.service_orders:read"
	LubricentroServiceOrdersCreate Code = "lubricentro.service_orders:create"
	LubricentroServiceOrdersUpdate Code = "lubricentro.service_orders:update"
	LubricentroServiceOrdersVoid   Code = "lubricentro.service_orders:void"

	// libreria
	LibreriaBooksRead         Code = "libreria.books:read"
	LibreriaBooksCreate       Code = "libreria.books:create"
	LibreriaBooksUpdate       Code = "libreria.books:update"
	LibreriaBooksImport       Code = "libreria.books:import"
	LibreriaSchoolListsRead   Code = "libreria.school_lists:read"
	LibreriaSchoolListsCreate Code = "libreria.school_lists:create"
	LibreriaSchoolListsUpdate Code = "libreria.school_lists:update"

	// odontologia
	OdontologiaRecordsRead   Code = "odontologia.records:read"
	OdontologiaRecordsUpdate Code = "odontologia.records:update"

	// veterinaria
	VeterinariaRecordsRead    Code = "veterinaria.records:read"
	VeterinariaRecordsUpdate  Code = "veterinaria.records:update"
	VeterinariaHospitalRead   Code = "veterinaria.hospital:read"
	VeterinariaHospitalUpdate Code = "veterinaria.hospital:update"
	VeterinariaGroomingRead   Code = "veterinaria.grooming:read"
	VeterinariaGroomingUpdate Code = "veterinaria.grooming:update"

	// restaurante
	RestauranteTablesRead         Code = "restaurante.tables:read"
	RestauranteTablesUpdate       Code = "restaurante.tables:update"
	RestauranteOrdersRead         Code = "restaurante.orders:read"
	RestauranteOrdersCreate       Code = "restaurante.orders:create"
	RestauranteOrdersUpdate       Code = "restaurante.orders:update"
	RestauranteReservationsRead   Code = "restaurante.reservations:read"
	RestauranteReservationsCreate Code = "restaurante.reservations:create"
	RestauranteReservationsUpdate Code = "restaurante.reservations:update"

	// retail
	RetailVariantsRead   Code = "retail.variants:read"
	RetailVariantsUpdate Code = "retail.variants:update"
)

// all lista todas las constantes de arriba. TestConstantsAreListed falla si falta alguna.
var all = [...]Code{
	IdentidadUsersRead,
	IdentidadUsersCreate,
	IdentidadUsersUpdate,
	IdentidadRolesRead,
	IdentidadRolesCreate,
	IdentidadRolesUpdate,
	IdentidadSessionsRead,
	IdentidadSessionsVoid,
	IdentidadSupportAccessRead,
	IdentidadSupportAccessApprove,
	EmpresasConfigRead,
	EmpresasConfigUpdate,
	EmpresasSitesRead,
	EmpresasSitesCreate,
	EmpresasSitesUpdate,
	EmpresasSeriesRead,
	EmpresasSeriesCreate,
	EmpresasSeriesUpdate,
	EmpresasEinvoicingRead,
	EmpresasEinvoicingUpdate,
	ClientesCustomersRead,
	ClientesCustomersCreate,
	ClientesCustomersUpdate,
	ClientesCustomersExport,
	ClientesCustomersImport,
	CatalogoItemsRead,
	CatalogoItemsCreate,
	CatalogoItemsUpdate,
	CatalogoItemsImport,
	CatalogoItemsExport,
	CatalogoCategoriesRead,
	CatalogoCategoriesCreate,
	CatalogoCategoriesUpdate,
	CatalogoPricesRead,
	CatalogoPricesUpdate,
	InventarioWarehousesRead,
	InventarioWarehousesCreate,
	InventarioWarehousesUpdate,
	InventarioStockRead,
	InventarioStockUpdate,
	InventarioStockExport,
	InventarioTransfersRead,
	InventarioTransfersCreate,
	InventarioTransfersUpdate,
	InventarioCountsRead,
	InventarioCountsCreate,
	InventarioCountsUpdate,
	InventarioCountsApprove,
	InventarioConsignmentsRead,
	InventarioConsignmentsCreate,
	InventarioConsignmentsUpdate,
	InventarioSupplierReturnsRead,
	InventarioSupplierReturnsCreate,
	InventarioSupplierReturnsApprove,
	VentasPosRead,
	VentasPosCreate,
	VentasDiscountsCreate,
	VentasDiscountsApprove,
	VentasQuotesRead,
	VentasQuotesCreate,
	VentasQuotesUpdate,
	VentasQuotesSend,
	VentasReturnsCreate,
	VentasReturnsApprove,
	FacturacionCpeRead,
	FacturacionCpeCreate,
	FacturacionCpeVoid,
	FacturacionCpeRetry,
	FacturacionCpeSend,
	FacturacionCpeExport,
	FacturacionCreditNotesCreate,
	FacturacionDebitNotesCreate,
	FacturacionGreRead,
	FacturacionGreCreate,
	FacturacionGreUpdate,
	ComprasSuppliersRead,
	ComprasSuppliersCreate,
	ComprasSuppliersUpdate,
	ComprasOrdersRead,
	ComprasOrdersCreate,
	ComprasOrdersUpdate,
	ComprasDocumentsRead,
	ComprasDocumentsCreate,
	ComprasDocumentsUpdate,
	ComprasDocumentsExport,
	FinanzasCashRead,
	FinanzasCashReadAll,
	FinanzasCashCreate,
	FinanzasCashUpdate,
	FinanzasCashApprove,
	FinanzasCashExport,
	FinanzasReceivablesRead,
	FinanzasReceivablesCreate,
	FinanzasReceivablesUpdate,
	FinanzasPayablesRead,
	FinanzasPayablesCreate,
	FinanzasPayablesUpdate,
	ReportesDashboardRead,
	ReportesManagementRead,
	ReportesManagementExport,
	ReportesTaxRead,
	ReportesTaxExport,
	AgendaAppointmentsRead,
	AgendaAppointmentsCreate,
	AgendaAppointmentsUpdate,
	IntegracionesConfigRead,
	IntegracionesConfigUpdate,
	AuditoriaEventsRead,
	AuditoriaEventsExport,
	NotificacionesRemindersRead,
	NotificacionesRemindersUpdate,
	LubricentroVehiclesRead,
	LubricentroVehiclesCreate,
	LubricentroVehiclesUpdate,
	LubricentroServiceOrdersRead,
	LubricentroServiceOrdersCreate,
	LubricentroServiceOrdersUpdate,
	LubricentroServiceOrdersVoid,
	LibreriaBooksRead,
	LibreriaBooksCreate,
	LibreriaBooksUpdate,
	LibreriaBooksImport,
	LibreriaSchoolListsRead,
	LibreriaSchoolListsCreate,
	LibreriaSchoolListsUpdate,
	OdontologiaRecordsRead,
	OdontologiaRecordsUpdate,
	VeterinariaRecordsRead,
	VeterinariaRecordsUpdate,
	VeterinariaHospitalRead,
	VeterinariaHospitalUpdate,
	VeterinariaGroomingRead,
	VeterinariaGroomingUpdate,
	RestauranteTablesRead,
	RestauranteTablesUpdate,
	RestauranteOrdersRead,
	RestauranteOrdersCreate,
	RestauranteOrdersUpdate,
	RestauranteReservationsRead,
	RestauranteReservationsCreate,
	RestauranteReservationsUpdate,
	RetailVariantsRead,
	RetailVariantsUpdate,
}
