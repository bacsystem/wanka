# Errores conocidos: qué revisar antes de pedir la revisión

Lista viva: cada revisión que encuentra un tipo de error nuevo lo agrega aquí. El desarrollador la recorre antes de escribir código y la revisión la usa como mínimo. Entre paréntesis va el PR donde apareció.

## Fallar cerrado
- Un dato ausente, mal formado o de un tipo inesperado **cierra**: un límite ausente vale 0, nunca «sin tope» (#207). Un `1.0` en `jsonb` conserva la escala y Go no lo lee como `int64`: hay que validarlo en el CHECK **y** normalizarlo en el código (#206).
- «Ninguno» se declara con el literal reservado `none`, nunca con `null` ni con un campo vacío (#206).
- Si una lectura de seguridad falla (base caída o error no tipado), la ruta no corre. Se responde 503 o 500, nunca se deja pasar.
- Un error que ningún reintento arregla (configuración, adaptador apagado, entrada inválida) se marca **permanente**; si el contrato dice «todo lo demás puede reintentarse», el llamador reintenta para siempre (#202).
- Un límite contra abuso (intentos, bloqueos, correos) tiene **techo absoluto**, no solo por ventana: 5 cada 15 min sin techo son 480 al día (#203).

## Datos y pertenencia
- Toda clave, caché o respuesta guardada se liga al **principal**, no solo al tenant (replay de idempotencia entre usuarios, #213).
- Una tabla con `tenant_id` sin RLS solo es válida si un ADR la justifica y está en la lista blanca del catálogo, y el código filtra `app.current_tenant()` en cada consulta.
- **Particiones:** sin privilegios para los roles de aplicación, o con su propia RLS completa (#210).
- Las políticas se comparan contra su **texto exacto**, nunca por subcadena: un `OR …` las abre sin que nadie lo note (#210).
- **Grants por columna:** se prueban el `INSERT` y el `UPDATE`, y cada columna que queda fuera del grant. Un `UPDATE` restringido no impide insertar un `created_at` futuro (#203); un `SELECT` por columna no se prueba solo con las columnas permitidas (#213).
- Una lista blanca que justifica una excepción cita un ADR que **existe**; el test lo comprueba contra `docs/architecture/adr/` (#210).
- Una regla de formato que la base puede comprobar (`modulo.recurso:accion`) va también en un `CHECK`, no solo en Go (#212).
- **Secretos en logs y auditoría:** el enmascarado cubre valores anidados (slices, maps, structs: `LogValue` no se aplica ahí) y un `string` o `[]byte` que trae JSON se parsea y se enmascara. La lista de nombres prohibidos incluye los que ya existen en el repo (#202, #212).

## Tiempo, dinero y representación
- Las fechas fiscales van en `America/Lima`. La API recibe un **día civil**, no un `time.Time` cuya zona elige quien llama (#208).
- El dinero se maneja como `decimal`, nunca `float64`, de punta a punta. La escala de salida es fija (`StringFixed`): un `3.750` no puede salir como `3.75` (#208).
- Lo que se serializa también se puede leer de vuelta: test de ida y vuelta por JSON (#208).
- El tope `numeric(14,2)` se comprueba en **todo** importe que se persiste o se envía (subtotal, gravado, descuentos, gratuito…), no solo en líneas y total; la escala de cada valor coincide con su columna del data-model (#204).
- Un descuento nunca sube el IGV ni el total: cada cálculo tiene invariantes con property tests, y los generadores incluyen valores chicos (#204).

## Tests que discriminan
- Cada protección tiene una mutación que pone su test en rojo. Si la mutación pasa en verde, el test no protege nada (#201, #206).
- Un **oráculo** de vectores se escribe desde una especificación en prosa, sin leer la implementación. Si copia la fórmula, también copia el error (#204).
- Los vectores tienen que distinguir las reglas que dicen probar: desempates, modos (#204).
- Los tests de tiempo se saltan con `-race`, y la CI los corre aparte, sin `-race` (#206).
- Las 4 fallas de CI de los lotes 1 y 2 fueron tests de tiempo o de concurrencia: una cota tiene que cumplirse en el runner de la CI, y un test concurrente no lanza más trabajo que el tamaño del pool (#205, #208, #211, #212).
- Los topes tienen su borde fijado: el valor exacto pasa y el siguiente falla (254 y 255, no 310) (#202).
- **Criterios no funcionales:** si la issue pide «medido y reportado», el número va en el cuerpo del PR antes de pedir la revisión, y el test mide la magnitud que el criterio nombra, no una parecida (#206, #211, #212, #213).

## Cableado
- Lo que arranca `serve`, sea el escucha o el bucle, es el mismo objeto que usan las rutas, y hay un test que lo comprueba (#207).
- El orden de los middlewares lo impone el router (`Innermost`), no cada ruta (#213).
- Un job que tiene que seguir corriendo en mora o con la empresa cerrada (por ejemplo, terminar CPE ya emitidos) se declara de forma que el chequeo de módulo no lo bloquee (#207).
- Todo `401` lleva `WWW-Authenticate: Bearer`. Lo pone el escritor de errores, no cada handler (#209).
- Una guarda exportada que el contrato exige (por ejemplo `RejectIdempotencyKey`) se monta en el código de producción: `grep` de su uso fuera de los tests (#213).
- Una caché que se invalida también descarta la lectura en vuelo: una petición que llega después del aviso no se une a la lectura que empezó antes (#206).
- Los timeouts se fijan de forma explícita y configurable, no por el default de una librería (#211).
- Lo que el contrato dice que un componente no importa (un adaptador, un tipo de una librería) lo impone `go-arch-lint`, no la buena voluntad (#202, #211).

## Configuración, CI y documentación
- Cada variable de entorno nueva está en la tabla del README de `apps/api`, en el CHANGELOG y en la sección de despliegue del PR, y sus valores aceptados coinciden con los entornos del ADR 0007 (`staging` incluido) (#202, #211).
- El filtro de rutas de un workflow incluye todo archivo que alimenta un test: si no, un PR que solo cambia ese archivo salta el test (#205).
- Un workflow que dice ser idempotente compara el objeto (el commit de un tag, las dos versiones de `package.json`), no solo si existe (#214).
- Toda afirmación de un documento («lo vigila un test», «la prueba de carga mide…», «ningún rol puede…») tiene el test o la medición que la respalda, y no contradice otra sección del mismo documento ni un ADR (#203, #205, #206, #213, #214).
- Una tabla o un grant nuevos se registran en la matriz del ADR que los gobierna, aunque el PR sea de docs (#203).

## Higiene de la pila
- Los IDs de caso de `rls-check` (`A11`, `K4`…) no se repiten: antes de numerar uno nuevo, busca el siguiente libre (#208).
- Después de integrar la base, se lee el diff del merge en los archivos compartidos (`rls-check`, listas blancas del catálogo): un merge perdió un `ROLLBACK` (#213). Un PR en conflicto (`DIRTY`) no tiene CI sobre su SHA: no se revisa ni se aprueba así (#212).
