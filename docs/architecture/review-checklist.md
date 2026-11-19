# Errores conocidos: qué revisar antes de pedir la revisión

Lista viva: cada revisión que encuentra un tipo de error nuevo lo agrega aquí. El desarrollador la recorre antes de escribir código y la revisión la usa como mínimo. Entre paréntesis va el PR donde apareció.

## Fallar cerrado
- Un dato ausente, mal formado o de un tipo inesperado **cierra**: un límite ausente vale 0, nunca «sin tope» (#207). Un `1.0` en `jsonb` conserva la escala y Go no lo lee como `int64`: hay que validarlo en el CHECK **y** normalizarlo en el código (#206).
- «Ninguno» se declara con el literal reservado `none`, nunca con `null` ni con un campo vacío (#206).
- Si una lectura de seguridad falla (base caída o error no tipado), la ruta no corre. Se responde 503 o 500, nunca se deja pasar.
- Cerrar es negar la petición, no destruir estado: ante un dato desconocido no se revoca ni se borra nada (#176).

## Datos y pertenencia
- Toda clave, caché o respuesta guardada se liga al **principal**, no solo al tenant (replay de idempotencia entre usuarios, #213).
- Una tabla con `tenant_id` sin RLS solo es válida si un ADR la justifica y está en la lista blanca del catálogo, y el código filtra `app.current_tenant()` en cada consulta.
- **Particiones:** sin privilegios para los roles de aplicación, o con su propia RLS completa (#210).
- Las políticas se comparan contra su **texto exacto**, nunca por subcadena: un `OR …` las abre sin que nadie lo note (#210).

## Tiempo, dinero y representación
- Las fechas fiscales van en `America/Lima`. La API recibe un **día civil**, no un `time.Time` cuya zona elige quien llama (#208).
- El dinero se maneja como `decimal`, nunca `float64`, de punta a punta. La escala de salida es fija (`StringFixed`): un `3.750` no puede salir como `3.75` (#208).
- Lo que se serializa también se puede leer de vuelta: test de ida y vuelta por JSON (#208).
- Un descuento nunca sube el IGV ni el total: cada cálculo tiene invariantes con property tests, y los generadores incluyen valores chicos (#204).

## Tests que discriminan
- Cada protección tiene una mutación que pone su test en rojo. Si la mutación pasa en verde, el test no protege nada (#201, #206).
- Un **oráculo** de vectores se escribe desde una especificación en prosa, sin leer la implementación. Si copia la fórmula, también copia el error (#204).
- Los vectores tienen que distinguir las reglas que dicen probar: desempates, modos (#204).
- Los tests de tiempo se saltan con `-race`, y la CI los corre aparte, sin `-race` (#206).

## Cableado
- Lo que arranca `serve`, sea el escucha o el bucle, es el mismo objeto que usan las rutas, y hay un test que lo comprueba (#207).
- El orden de los middlewares lo impone el router (`Innermost`), no cada ruta (#213).
- Un job que tiene que seguir corriendo en mora o con la empresa cerrada (por ejemplo, terminar CPE ya emitidos) se declara de forma que el chequeo de módulo no lo bloquee (#207).
- Todo `401` lleva `WWW-Authenticate: Bearer`. Lo pone el escritor de errores, no cada handler (#209).

## Higiene de la pila
- Los IDs de caso de `rls-check` (`A11`, `K4`…) no se repiten: antes de numerar uno nuevo, busca el siguiente libre (#208).
