# Feature: mercadona-cli (séptima cadena)

**Estado:** en curso · **Creado:** 2026-11-08 · **Rama:** feat/mercadona

Mercadona entra en el agregador con el mismo patrón que las otras seis cadenas: paquete
propio, binario propio, el mismo sobre `contract.Result`, su fila en la tabla de
capacidades y su marca en la web. El catálogo se lee de su índice público de Algolia, que
se verificó en vivo antes de escribir código.

## Specs

- **S1** — «vamos a incluir mercadona en los supermercados.» Mercadona es la **séptima
  cadena**: `internal/mercadona`, `cmd/mercadona`, fila propia en `internal/capabilities`,
  alta en las listas de cadenas de `cmd/multisuper` y `cmd/web`, tarjeta con su marca en la
  web y su fila en el README. El JSON de salida es el mismo `contract.Result` que imprimen
  los otros seis binarios.
- **S2** — «creo que hay un cli en github marcadona-cli que pede servirnos de punto de
  inicio». `ivorpad/mercadona-cli` se usa como **punto de partida documental** (endpoints,
  nombres de índice, credenciales públicas del SPA y semántica de `price_instructions`); su
  código no se importa ni se copia, se reescribe en el patrón del repo. Sin dependencias
  nuevas: solo la biblioteca estándar, como el resto de las cadenas.
- **S3** — Mismo contrato de invocación que la familia: `mercadona search <term>
  [--limit N] [--wh code] [--json]`. Con `--json` imprime el sobre `contract.Result` y
  **solo eso** en stdout; errores y logs a stderr. Códigos de salida: `0` éxito, `1` error
  de obtención o de parseo, `2` error de uso (falta el término, `--limit` fuera de 1..100,
  bandera desconocida). El término es la frase completa, y una ayuda (`-h`/`--help`) sale
  con `0` en cualquier posición.
- **S4** — Ningún dato inventado. El precio de venta es `price_instructions.unit_price`; el
  €/unidad es `reference_price` con la unidad de `reference_format` (`kg` → kg, `L` → L,
  otra cosa → sin unidad, nunca convertida a ojo); el tamaño es `unit_size` + `size_format`
  tal como se publican. Si el índice no publica precio, el precio queda vacío.
- **S5** — El precio es **por almacén**: el índice de Algolia es
  `products_prod_<almacén>_es` y cada almacén tiene su catálogo y sus precios. `--wh` elige
  el almacén y su valor por defecto es `mad1`; la fila de `capabilities` lo declara (modo
  `warehouse`, `price_scoped: true`) para que la web no prometa un precio nacional.
- **S6** — Las credenciales de Algolia son las **públicas y de solo búsqueda** que el SPA de
  `tienda.mercadona.es` sirve a cualquier visitante, igual que el caso de ALDI. Si el
  almacén rota su app id o su clave, la búsqueda falla con un error explícito (nunca con
  una lista vacía) y el README dice cómo recuperarlas del bundle.
- **S7** — Lo que el índice **no** publica se declara, no se rellena: la búsqueda no trae
  EAN (solo `GET /api/products/<id>/` lo publica, y no se paga una petición por resultado)
  ni estado de stock (`published` es publicación en catálogo, no existencias), así que `ean`
  queda ausente y `available` desconocido. El README lo dice.
- **S8** — La web muestra la marca oficial de Mercadona dentro de la misma caja 106 × 30 que
  las otras seis, sin recolorear, recortar ni deformar, con su procedencia anotada en el
  README. Los rótulos de la página no pueden afirmar «seis» cadenas.

## Tareas

- [x] **T1** `internal/mercadona`: cliente de búsqueda contra el índice público y mapeo de
      `price_instructions` a `contract.Hit`, con fixtures reales. Inline. Commit `feat(mercadona): …`
- [x] **T2** `cmd/mercadona`: `search`, `--limit`, `--wh`, `--json`, ayuda y códigos de
      salida de S3. Inline. Commit `feat(mercadona): add the mercadona CLI`
- [x] **T3** Alta en el agregador: `internal/capabilities`, `cmd/multisuper`, `cmd/web` y los
      tests que fijan la tabla y el recuento. Commit `feat(aggregate): …`
- [x] **T4** Web: nombre, marca en el sprite, color de cadena y el chip de zona que hoy dice
      el código postal de DIA. Commit `feat(web): …`
- [x] **T5** README: cadena, endpoint, alcance del precio, límites y lo que no publica.
      Commit `docs(readme): …`
- [x] **T6** Capturas de `docs/web/` al día: la de las tarjetas ya no puede enseñar seis.
      Regeneradas contra el servidor vivo con Chrome por CDP (navegar, buscar «leche entera»,
      fijar una línea por cadena en cadenas que confirman el producto por id, recortar cada
      sección). Sin script en el repo: es una herramienta de un solo uso. Inline.

## Entregas

- **E1** `94be2ce` feat(mercadona): leer el catálogo del almacén por su índice público de
  Algolia — T1. El hook de commits cuenta 1044 líneas, pero ~170 son los dos fixtures JSON
  reales que fijan el mapeo: el código revisable son el paquete (~330 líneas) y su test
  (~380). Se deja como una sola entrega porque el test y el mapeo son la misma pieza.
- **E2** `de68c1f` feat(mercadona): añadir el binario mercadona con el contrato de la
  familia — T2.
- **E3** `68aa3d0` feat(aggregate): mercadona entra en la tabla de cadenas y en el fan-out
  — T3.
- **E4** `1f9441d` feat(web): la séptima tarjeta, con la marca oficial de Mercadona — T4.
- **E7** `4170f73` docs(web): capturas nuevas, con la séptima tarjeta — T6. Regeneradas
  desde la web viva; la comparación enseña tres cestas de cadenas que confirman el id y no
  la de CARREFOUR, que cotiza un tercero.
- **E6** `ec51309` fix(multisuper): la ayuda de --only nombra también mercadona — hallazgo
  posterior a la entrega. `chains()` registraba la cadena pero las dos ayudas llevaban la
  lista escrita a mano: `--only mercadona` funcionaba y no estaba documentado. La lista
  sale ahora de `capabilities.Stores()` y un test compara ayudas, registro y tabla.
- **E5** `a153480` docs(readme): siete cadenas y la letra pequeña de Mercadona — T5. Delegado a un
  worker con superficies acotadas a `README.md`; revisado por el padre: 14 menciones a
  mercadona, ningún recuento de seis, `go test ./...` en verde. La captura de la web sigue
  mostrando seis tarjetas y se deja como pendiente declarado, no como si estuviera al día.

## Log

- **L1** (literal) «vamos a incluir mercadona en los supermercados. creo que hay un cli en
  github marcadona-cli que pede servirnos de punto de inicio»
- **L2** Evidencia verificada en vivo (2026-11-08) antes de escribir código:
  `POST https://7uzjkl1dj0-dsn.algolia.net/1/indexes/products_prod_mad1_es/query` con las
  cabeceras del SPA responde **200** y devuelve `hits` con `id`, `display_name`, `brand`,
  `packaging`, `share_url`, `thumbnail`, `categories` anidadas y `price_instructions`
  (`unit_price`, `reference_price`, `reference_format`, `unit_size`, `size_format`,
  `bulk_price`, `is_pack`). `GET https://tienda.mercadona.es/api/products/<id>/?wh=mad1`
  responde 200 e incluye `ean`; la búsqueda no lo trae. Un `curl` normal (sin huella TLS
  especial) basta: el fingerprint de navegador que usa `mercadona-cli` no es necesario para
  leer el índice.
- **L5** Hallazgo del humano el 2026-11-08, con razón: la comprobación diferencial ya
  enseñaba ese texto (`--only S,... query only these chains: ...`) y se leyó como
  diferencia esperada en vez de como defecto. La ayuda de `--only` no debe escribirse a
  mano: sale de la misma tabla que registra las cadenas (E6).
- **L4** Verificación independiente (gentle-ai-verify, rama `feat/mercadona`), veredicto
  **se sostiene** en las cuatro afirmaciones:
  (1) el par `appId`/clave está en el bundle público de la tienda
  (`/v10116/index-kL_Zoh0P.js`, 5,2 MB, el SPA lo usa tal cual) y la clave no escribe:
  `DELETE /1/indexes/products_prod_mad1_es/<id>` → `403 Method not allowed with this API
  key`; `partialUpdateObjectNoCreate` → `400 Not enough rights to update an object`;
  `GET /1/keys` → `403`;
  (2) no hay otra credencial, token, cookie ni sesión en el diff, y los fixtures solo
  llevan miniaturas `imgix` y campos de catálogo;
  (3) 21 valores de `--wh` (`../`, `?`, `#`, saltos de línea, NUL, mayúsculas, `%2F`) se
  rechazan sin llegar a la red, y solo un `^[a-z][a-z0-9]{1,11}$` produce petición;
  (4) ningún precio inventado: `azafrán` sale con precio y **sin** €/unidad porque su
  `reference_format` es `100 g`, que el contrato no sabe nombrar.
  Sin verificar, declarado: si la clave permite `addObject` (no se probó: crearía un
  registro que la propia clave no podría borrar) y el 404 real de un almacén inexistente
  (solo cubierto por el servidor falso).
- **L3** Decisión: `ivorpad/mercadona-cli` no se importa (módulo ajeno, `internal/` no es
  importable) y su cliente habla la API privada con token; aquí solo se necesita la lectura
  anónima del índice, que es lo que la búsqueda de esta app hace. Su código de descubrimiento
  de credenciales del SPA se documenta en el README como plan de recuperación ante rotación.
