// Package components son los modelos compartidos del contrato (errores, paginación, dinero,
// cabeceras, comprobantes), generados de openapi/components/*.yaml. Cada archivo tiene su
// cfg-<nombre>.yaml y todos van al mismo paquete.
package components

//go:generate go tool oapi-codegen -config cfg-money.yaml ../../../openapi/components/money.yaml
//go:generate go tool oapi-codegen -config cfg-documents.yaml ../../../openapi/components/documents.yaml
//go:generate go tool oapi-codegen -config cfg-pagination.yaml ../../../openapi/components/pagination.yaml
//go:generate go tool oapi-codegen -config cfg-problem.yaml ../../../openapi/components/problem.yaml
//go:generate go tool oapi-codegen -config cfg-headers.yaml ../../../openapi/components/headers.yaml
