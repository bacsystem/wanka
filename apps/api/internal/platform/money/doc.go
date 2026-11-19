// Package money define Amount, el decimal con escala fija que usa todo el dinero,
// las cantidades y los tipos de cambio del backend (ADR 0008, data-model §2).
//
// No existe ninguna conversión desde o hacia float64: ni en la API pública ni en JSON.
package money
