package jobs

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Variables de entorno del worker. Los valores no se repiten en los errores.
const (
	// EnvShutdownTimeout: cuánto espera el worker a los jobs en curso tras SIGTERM,
	// como duración de Go ("30s"). Railway corta con SIGKILL al vencer su plazo de
	// drenaje (RAILWAY_DEPLOYMENT_DRAINING_SECONDS): hay que subirlo por encima de este valor.
	EnvShutdownTimeout = "JOBS_SHUTDOWN_TIMEOUT"
	// envConcurrencyPrefix + el nombre de la cola en mayúsculas, p. ej. JOBS_CONCURRENCY_CPE.
	envConcurrencyPrefix = "JOBS_CONCURRENCY_"

	DefaultShutdownTimeout = 30 * time.Second
	maxShutdownTimeout     = 10 * time.Minute
	maxConcurrency         = 1000
)

// Config es la configuración del worker.
type Config struct {
	// Concurrency es el máximo de jobs en ejecución por cola y por réplica.
	Concurrency map[string]int
	// ShutdownTimeout es el plazo que se espera a los jobs en curso al detenerse;
	// vencido, se cancela su contexto.
	ShutdownTimeout time.Duration
	// RetryBase y RetryMax acotan el backoff exponencial; cero usa el valor por defecto.
	RetryBase, RetryMax time.Duration
}

// ConfigFromEnv lee la configuración del worker de getenv (os.Getenv en producción).
// Todos los problemas salen juntos.
func ConfigFromEnv(getenv func(string) string) (Config, error) {
	cfg := Config{Concurrency: maps.Clone(DefaultConcurrency), ShutdownTimeout: DefaultShutdownTimeout}
	var problems []string
	for _, q := range slices.Sorted(maps.Keys(DefaultConcurrency)) {
		name := envConcurrencyPrefix + strings.ToUpper(q)
		if v := getenv(name); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 || n > maxConcurrency {
				problems = append(problems, fmt.Sprintf("environment variable %s must be an integer between 1 and %d", name, maxConcurrency))
				continue
			}
			cfg.Concurrency[q] = n
		}
	}
	if v := getenv(EnvShutdownTimeout); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d < time.Second || d > maxShutdownTimeout {
			problems = append(problems, fmt.Sprintf("environment variable %s must be a duration between 1s and %s", EnvShutdownTimeout, maxShutdownTimeout))
		} else {
			cfg.ShutdownTimeout = d
		}
	}
	if len(problems) > 0 {
		return Config{}, fmt.Errorf("%s", strings.Join(problems, "; "))
	}
	return cfg, nil
}
