package httpx

import (
	"net/http"
	"path"
	"strings"
	"time"
)

// Plazos por defecto (api/conventions.md §7).
const (
	// DefaultHandlerTimeout es el plazo de un handler normal.
	DefaultHandlerTimeout = 10 * time.Second
	// DefaultStreamTimeout es el presupuesto de una ruta de streaming, contado
	// desde que llega la petición.
	DefaultStreamTimeout = 25 * time.Second
)

// Middleware envuelve un handler. Es la forma de los pasos propios de una ruta
// (sesión, limitador, permisos).
type Middleware = func(http.Handler) http.Handler

// RouterOptions configura NewRouter; los ceros son los plazos del contrato.
type RouterOptions struct {
	HandlerTimeout time.Duration
	StreamTimeout  time.Duration
}

// Router es el enrutador de la API sobre http.ServeMux. Una ruta es método +
// camino ("GET /v1/items/{id}"); lo que no está declarado, incluido un método
// distinto y OPTIONS, responde 404 not_found RFC 9457 (nunca el 404 ni el 405
// en texto de ServeMux, y no hay CORS, conventions.md §2 y §4). Cada ruta corre
// con el plazo del handler o, si es Streaming, con el presupuesto de streaming.
type Router struct {
	mux  *http.ServeMux
	opts RouterOptions
}

// NewRouter devuelve un Router vacío.
func NewRouter(o RouterOptions) *Router {
	if o.HandlerTimeout <= 0 {
		o.HandlerTimeout = DefaultHandlerTimeout
	}
	if o.StreamTimeout <= 0 {
		o.StreamTimeout = DefaultStreamTimeout
	}
	return &Router{mux: http.NewServeMux(), opts: o}
}

type route struct {
	streaming bool
	mw        []Middleware
	inner     []Middleware
}

// RouteOption ajusta una ruta en Handle.
type RouteOption func(*route)

// Streaming marca una ruta de descarga o exporte: no pasa por el plazo de 10 s
// ni guarda la respuesta en memoria, puede hacer Flush y su contexto vence a
// los 25 s desde la llegada. Si vence antes del primer byte responde 503
// overloaded; si vence durante el envío corta la conexión.
func Streaming() RouteOption { return func(r *route) { r.streaming = true } }

// Use agrega middlewares a la ruta, el primero es el más externo. Corren dentro
// del plazo (sesión, limitador y demás ven el contexto con plazo) y antes del handler.
func Use(mw ...Middleware) RouteOption { return func(r *route) { r.mw = append(r.mw, mw...) } }

// Innermost agrega middlewares que se aplican más cerca del handler que todos los
// Use, sin importar el orden de las opciones: corren después de la sesión, los
// limitadores y todo paso de ruta. El primero es el más externo de los innermost.
// Es el lugar de la idempotencia, que debe ser lo último antes del handler.
func Innermost(mw ...Middleware) RouteOption {
	return func(r *route) { r.inner = append(r.inner, mw...) }
}

// Handle registra h para pattern, que debe ser "MÉTODO /camino". Un patrón sin
// método o de subárbol (termina en "/" o en "...}") es un error de programación y entra en pánico.
func (rt *Router) Handle(pattern string, h http.Handler, opts ...RouteOption) {
	method, p, ok := strings.Cut(pattern, " ")
	if !ok || method == "" || strings.ToUpper(method) != method || !strings.HasPrefix(p, "/") || strings.HasSuffix(p, "/") || strings.HasSuffix(p, "...}") {
		// Un patrón de subárbol ("/x/" o "/{p...}") hace que ServeMux redirija
		// "/x" con un 301 HTML, que no es RFC 9457.
		panic("httpx: route pattern must be \"METHOD /path\" without a trailing slash or a \"...}\" wildcard, got " + pattern)
	}
	var ro route
	for _, o := range opts {
		o(&ro)
	}
	for i := len(ro.inner) - 1; i >= 0; i-- {
		h = ro.inner[i](h)
	}
	for i := len(ro.mw) - 1; i >= 0; i-- {
		h = ro.mw[i](h)
	}
	if ro.streaming {
		h = streamBudget(h, rt.opts.StreamTimeout)
	} else {
		h = handlerDeadline(h, rt.opts.HandlerTimeout)
	}
	rt.mux.Handle(pattern, h)
}

// HandleFunc es Handle con una función.
func (rt *Router) HandleFunc(pattern string, f http.HandlerFunc, opts ...RouteOption) {
	rt.Handle(pattern, f, opts...)
}

// ServeHTTP despacha r o responde 404 not_found.
func (rt *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Un camino no canónico lo redirigiría ServeMux con un cuerpo HTML: aquí es 404.
	if path.Clean(r.URL.Path) != r.URL.Path {
		WriteProblem(w, r, "not_found", "")
		return
	}
	if _, pattern := rt.mux.Handler(r); pattern == "" {
		WriteProblem(w, r, "not_found", "")
		return
	}
	rt.mux.ServeHTTP(w, r)
}
