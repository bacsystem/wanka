# Errores conocidos: qué revisar antes de pedir la revisión

Lista viva: cada revisión que encuentra un tipo de error nuevo lo agrega aquí. El desarrollador la recorre antes de escribir código y la revisión la usa como mínimo. Entre paréntesis va el PR donde apareció.

## Fallar cerrado
- Un dato ausente, mal formado o de un tipo inesperado **cierra**: un límite ausente vale 0, nunca «sin tope» (#207). Un `1.0` en `jsonb` conserva la escala y Go no lo lee como `int64`: hay que validarlo en el CHECK **y** normalizarlo en el código (#206).
- «Ninguno» se declara con el literal reservado `none`, nunca con `null` ni con un campo vacío (#206).
- Si una lectura de seguridad falla (base caída o error no tipado), la ruta no corre. Se responde 503 o 500, nunca se deja pasar.
- Cerrar es negar la petición, no destruir estado: ante un dato desconocido no se revoca ni se borra nada (#176).
- Un error que ningún reintento arregla (configuración, adaptador apagado, entrada inválida) se marca **permanente**; si el contrato dice «todo lo demás puede reintentarse», el llamador reintenta para siempre (#202).
- Un límite contra abuso (intentos, bloqueos, correos) tiene **techo absoluto**, no solo por ventana: 5 cada 15 min sin techo son 480 al día (#203).
- Una consulta de seguridad cancelada o vencida por timeout también cierra: `503`, nunca un `200` vacío (#248).
- **Respuestas que deben ser iguales** (credencial mala, cuenta inexistente o bloqueada) hacen el mismo trabajo: las mismas transacciones y sentencias, y el hash ficticio en **todo** camino de error, también con un hash guardado ilegible. Lo prueba un test determinista que cuenta ese trabajo, más una sonda de tiempo por HTTP con los parámetros de producción y los casos intercalados; una cota de 0,6x a 1,6x no lo distingue (#251).

## Datos y pertenencia
- Toda clave, caché o respuesta guardada se liga al **principal**, no solo al tenant (replay de idempotencia entre usuarios, #213).
- Una tabla con `tenant_id` sin RLS solo es válida si un ADR la justifica y está en la lista blanca del catálogo, y el código filtra `app.current_tenant()` en cada consulta.
- **Particiones:** sin privilegios para los roles de aplicación, o con su propia RLS completa (#210).
- Las políticas se comparan contra su **texto exacto**, nunca por subcadena: un `OR …` las abre sin que nadie lo note (#210).
- **Grants por columna:** se prueban el `INSERT` y el `UPDATE`, y cada columna que queda fuera del grant. Un `UPDATE` restringido no impide insertar un `created_at` futuro (#203); un `SELECT` por columna no se prueba solo con las columnas permitidas (#213).
- Una lista blanca que justifica una excepción cita un ADR que **existe**; el test lo comprueba contra `docs/architecture/adr/` (#210).
- Una guarda de `DELETE` (o de cualquier sentencia) cubre todas las que el rol tiene: con `UPDATE` de fila completa, un cambio de estado logra lo mismo que el borrado (#230, #245). Las columnas congeladas de una fila se expresan como «todo menos lo permitido» (`to_jsonb(NEW) - …`), así las columnas futuras nacen congeladas (#245).
- Un permiso reservado («solo el dueño») se protege en cada vía que lo otorga: asignar un rol, editarlo y crearlo (#230).
- Una guarda sobre una cadena de conexión (solo loopback, solo local) valida la configuración que arma el driver (`pgconn.ParseConfig`: host principal y alternos), no el texto de la URL: `?host=`, `?service=` y el DNS cambian el destino (#253).
- Una regla de formato que la base puede comprobar (`modulo.recurso:accion`) va también en un `CHECK`, no solo en Go (#212).
- **Secretos en logs y auditoría:** el enmascarado cubre valores anidados (slices, maps, structs: `LogValue` no se aplica ahí) y un `string` o `[]byte` que trae JSON se parsea y se enmascara. La lista de nombres prohibidos incluye los que ya existen en el repo (#202, #212).

## Tiempo, dinero y representación
- Las fechas fiscales van en `America/Lima`. La API recibe un **día civil**, no un `time.Time` cuya zona elige quien llama (#208).
- El dinero se maneja como `decimal`, nunca `float64`, de punta a punta. La escala de salida es fija (`StringFixed`): un `3.750` no puede salir como `3.75` (#208).
- Lo que se serializa también se puede leer de vuelta: test de ida y vuelta por JSON (#208).
- El tope `numeric(14,2)` se comprueba en **todo** importe que se persiste o se envía (subtotal, gravado, descuentos, gratuito…), no solo en líneas y total; la escala de cada valor coincide con su columna del data-model (#204).
- Un descuento nunca sube el IGV ni el total: cada cálculo tiene invariantes con property tests, y los generadores incluyen valores chicos (#204).
- Una longitud se cuenta en la misma unidad en todas las capas: puntos de código, como JSON Schema y `utf8.RuneCountInString`, nunca `.length` de UTF-16 (#254).

## Tests que discriminan
- Cada protección tiene una mutación que pone su test en rojo. Si la mutación pasa en verde, el test no protege nada (#201, #206).
- Un **oráculo** de vectores se escribe desde una especificación en prosa, sin leer la implementación. Si copia la fórmula, también copia el error (#204).
- Los vectores tienen que distinguir las reglas que dicen probar: desempates, modos (#204).
- Los tests de tiempo se saltan con `-race`, y la CI los corre aparte, sin `-race` (#206).
- Las 4 fallas de CI de los lotes 1 y 2 fueron tests de tiempo o de concurrencia: una cota tiene que cumplirse en el runner de la CI, y un test concurrente no lanza más trabajo que el tamaño del pool (#205, #208, #211, #212).
- Los topes tienen su borde fijado: el valor exacto pasa y el siguiente falla (254 y 255, no 310) (#202).
- **Criterios no funcionales:** si la issue pide «medido y reportado», el número va en el cuerpo del PR antes de pedir la revisión, y el test mide la magnitud que el criterio nombra, no una parecida (#206, #211, #212, #213).
- Un test de conjuntos (permisos de un alcance, catálogos, listas blancas) compara contra una **lista escrita a mano**, nunca contra lo que da la misma fórmula del código: si la copia, fija el defecto (#218).
- Las semillas de un test no se arman con la constante que el test vigila: una retención de 7 días sembrada con `UserTokenRetention` sigue en verde si la constante pasa a 6 (#244).
- Un hallazgo que se cierra con una mutación o una corrida queda abierto hasta ejecutarla. Una mutación «razonada» no cierra nada: en #244 se razonó roja y corrida quedó verde.
- Una aserción que no puede fallar no prueba nada: con `enable_seqscan = off` nunca aparece un `Seq Scan` (#245).
- **Tests concurrentes:** el invariante sale del diseño, con sus reinicios. Un ingreso correcto que devuelve el cobro duplica el máximo, de 5 a 10 (#251). El pool del test fija un `AcquireTimeout` explícito y amplio. Un fallo intermitente se reproduce de forma determinista antes de arreglarlo; nunca se reintenta hasta que pase (#251).
- Un reloj falso falsea solo lo que el test usa (`setInterval`, `Date`), no todos los timers (#254).
- Un E2E que la CI saltea (porque falta la API u otro servicio) cuenta como no corrido: el PR lo dice y abre la issue que lo hace correr (#254, #255).

## Cableado
- Lo que arranca `serve`, sea el escucha o el bucle, es el mismo objeto que usan las rutas, y hay un test que lo comprueba (#207).
- El orden de los middlewares lo impone el router (`Innermost`), no cada ruta (#213).
- Un job que tiene que seguir corriendo en mora o con la empresa cerrada (por ejemplo, terminar CPE ya emitidos) se declara de forma que el chequeo de módulo no lo bloquee (#207).
- Todo `401` lleva `WWW-Authenticate: Bearer`. Lo pone el escritor de errores, no cada handler (#209).
- Una guarda exportada que el contrato exige (por ejemplo `RejectIdempotencyKey`) se monta en el código de producción: `grep` de su uso fuera de los tests (#213).
- Una caché que se invalida también descarta la lectura en vuelo: una petición que llega después del aviso no se une a la lectura que empezó antes (#206).
- Los timeouts se fijan de forma explícita y configurable, no por el default de una librería (#211).
- Lo que el contrato dice que un componente no importa (un adaptador, un tipo de una librería) lo impone `go-arch-lint`, no la buena voluntad (#202, #211).
- Un test de cableado (paridad router ↔ contrato, smoke del router) monta **exactamente** lo que monta `serve` (`defaultWiring`), no un `wiring{}` vacío (#218).
- Un chequeo de paridad compara todas las dimensiones que pide el criterio: método, ruta, módulo, permiso **y clase** (#218).
- Una guarda que promete detectar una lista de nombres prueba cada nombre con una mutación (#219).
- Un job periódico de la cola corre también al arrancar el worker (`RunOnStart`): River OSS no persiste el próximo turno, y uno diario no corre nunca si el worker se reinicia antes de 24 h (#244).
- Una columna de estado nueva (contador, racha, estado de la empresa) se agrega a toda siembra y todo reinicio (`dev-seed`, siembra de E2E), con un test por columna (#253, #254).
- Un route handler público lee el cuerpo con tope mientras llega (`Content-Length` y conteo en el stream) y el framework tiene su propio límite: un `req.text()` antes del tope deja que un POST de 50 MB llene la memoria (#254).
- Nombres, `Path` y atributos de cookies y cabeceras salen del contrato (ADR, `conventions.md`), y un test escrito a mano los fija (#254).
- **Frontend:** ninguna ruta fuera de la lista de habilitadas responde, ni con sesión ni sin ella. Un matcher del `proxy` que excluye las rutas con punto deja pasar los segmentos dinámicos (#243), y las pantallas mock de `OPEN_AREAS` respondían 200 sin sesión (#254). Lo prueba una sonda sobre cada ruta de `app/` con valores de ejemplo en los segmentos.

## Configuración, CI y documentación
- Cada variable de entorno nueva está en la tabla del README de `apps/api`, en el CHANGELOG y en la sección de despliegue del PR, y sus valores aceptados coinciden con los entornos del ADR 0007 (`staging` incluido) (#202, #211).
- El filtro de rutas de un workflow incluye todo archivo que alimenta un test: si no, un PR que solo cambia ese archivo salta el test (#205).
- Un workflow que dice ser idempotente compara el objeto (el commit de un tag, las dos versiones de `package.json`), no solo si existe (#214).
- Toda afirmación de un documento («lo vigila un test», «la prueba de carga mide…», «ningún rol puede…») tiene el test o la medición que la respalda, y no contradice otra sección del mismo documento ni un ADR (#203, #205, #206, #213, #214).
- Una tabla o un grant nuevos se registran en la matriz del ADR que los gobierna, aunque el PR sea de docs (#203).
- Una herramienta de CI se fija por lockfile o por checksum, con una versión de **al menos 14 días** desde su publicación; el runtime (Node) sale de `.nvmrc`, nunca del runner (#219).
- El script que usa una herramienta distingue sus exit codes: «encontró un problema» no es lo mismo que «no pudo correr» (#219).
- El cuerpo del PR no afirma que la CI corrió algo hasta que `gh pr checks` lo muestra; un PR `DIRTY` no corre ningún check (#219).
- Un contenedor efímero de test (`rls-check`, carga, servicios de un solo uso) pone el directorio de datos en `tmpfs` y se borra con sus volúmenes (`docker rm -fv`, `down -v`): los volúmenes anónimos llenaron el disco y tiraron Docker (#252).
- Una receta de `make` que usa secretos no se imprime (`@`) y no los pasa por argv (#252).
- Corregir un ADR aceptado lleva la nota fechada del §9 del README de arquitectura y la decide el architect; nunca se reescribe la decisión en su lugar sin nota (#218).

## Higiene de la pila
- Después de integrar la base, `git diff origin/main -- CHANGELOG.md` muestra solo las líneas del PR, agregadas bajo `[Unreleased]`: tras un release, un merge limpio, con `union` o sin él, puede mover entradas hacia la sección publicada o desde ella (skills #33).
- Los IDs de caso de `rls-check` (`A11`, `K4`…) no se repiten: antes de numerar uno nuevo, busca el siguiente libre (#208).
- Después de integrar la base, se lee el diff del merge en los archivos compartidos (`rls-check`, listas blancas del catálogo): un merge perdió un `ROLLBACK` (#213). Un PR en conflicto (`DIRTY`) no tiene CI sobre su SHA: no se revisa ni se aprueba así (#212).
- Después de integrar la base, el código generado se regenera y se commitea (`pnpm --filter web gen:api`, el servidor de OpenAPI): la base pudo cambiar el contrato (#254).
- GitHub no aplica `merge=union`: un PR que solo choca en el CHANGELOG igual figura en conflicto y necesita un merge local. Después del merge se mira que `[Unreleased]` no repita encabezados: tres `### Changed` llegaron a `main` en la ola 1, lote 1.
