# Novedades de Gutter

Todas las versiones publicadas de Gutter, con los cambios más visibles para
quien usa el lector. El ejecutable de cada versión está en
[Releases](https://github.com/Skeiceee/gutter/releases).

## v1.3.2 — 2026-11-16

Versión de mantenimiento: reorganiza el motor de lectura y aligera el arranque
sin cambiar la experiencia de lectura.

### Arranque

- **Caché de disco en segundo plano**: el recuento inicial de índices, páginas y
  miniaturas ya no se hace en el hilo de UI. En instalaciones con miles de
  archivos la ventana aparece antes.

### Robustez

- **Fuzzing de parsers**: nuevas pruebas de fuzzing para cabeceras de imagen, el
  formato `.gtp` de la caché en disco y la detección/indexado de RAR/ZIP. Los
  *seeds* se ejecutan en cada `go test`; el fuzzing real se lanza con `-fuzz`.
- **Escaneo idempotente**: el recuento de la caché comparte mutex con las
  escrituras, así que no puede duplicar contadores.

### Interno

- **Backend dividido por responsabilidad**: `backend.go` pasa de ~1.470 a ~240
  líneas. El resto vive en `open.go` (contenedor), `page.go` (lectura y
  decodificación), `load.go` (LRU y *single-flight*), `prefetch.go`,
  `thumbnails.go`, `names.go` (filtros y orden) y `cache.go`, sin cambios de API
  ni de comportamiento.

## v1.3.1 — 2026-11-16

Versión de robustez: evita cuelgues y consumo desmedido de memoria con
archivos grandes o dañados, y hace que la apertura y la traducción respondan
siempre a la ventana.

### Robustez

- **Límite de píxeles decodificados**: antes de decodificar una página o una
  miniatura se lee su cabecera y se rechazan las imágenes que declaran más de
  100 megapíxeles. Un PNG de pocos KB con dimensiones absurdas ya no puede
  intentar reservar decenas de GB y tumbar el proceso.

### Cancelación

- **Un único contexto para la ventana**: página, precarga, rejilla, traducción
  y comprobación de actualizaciones se cancelan de verdad al cerrar.
- **RAR sólido**: `Comic.Close()` espera también los recorridos secuenciales de
  precarga y miniaturas, así que ninguna lectura sigue viva sobre un archivo ya
  cerrado.
- **Traducción cancelable**: el OCR y las peticiones de red se cortan al cambiar
  de página, en lugar de seguir trabajando hasta tres minutos después.

### Apertura

- **Indexado en segundo plano**: abrir un `.cbr`/`.cbz` grande ya no congela la
  ventana. La barra inferior muestra «Abriendo cómic…» y `Esc` cancela.
- Si la apertura falla, o se abre otro archivo antes de que termine, el cómic
  que ya estaba en pantalla se conserva.

### Interfaz

- **Diálogos únicos**: pulsar `Ctrl+O` varias veces ya no apila ventanas.
- Cada apertura lleva un número de secuencia: los resultados obsoletos se
  descartan y su archivo se cierra.

## v1.3.0 — 2026-11-14

### Preferencias unificadas

- **Un solo diálogo con pestañas (`Ctrl+,`)**: Lectura, Caché, Traducción e
  Interfaz. Reúne lo que antes estaba repartido entre «Opciones de lectura»,
  «Configuración de OCR» y el bloque de caché de «Acerca de».
- Las opciones de paginación (una página, occidental/manga) se aplican al
  momento y el menú Ver muestra marcas de verificación sincronizadas.
- Nuevas preferencias de interfaz: recordar tamaño de ventana, mostrar el
  indicador de página en modo limpio y buscar actualizaciones al arrancar.

### Versión y actualizaciones

- La versión se inyecta al compilar con `-X` y aparece en «Acerca de» junto al
  commit y la fecha.
- «Ayuda > Buscar actualizaciones…» consulta el último release de GitHub y abre
  la descarga; la comprobación automática es opcional y falla en silencio.

### Calidad

- **`readerState`**: el estado de lectura (cómic, página, mitad, zoom, modo) se
  separa de `gutterApp` y se cubre con pruebas unitarias de navegación.
- **CI**: `gofmt`, `go vet` y `go test -race ./...` en cada push y pull request,
  más un workflow de release por tag que compila Windows, Linux y macOS.
- `build.ps1` acepta `-Version` y toma la versión de `git describe` por defecto.

## v1.2.0 — 2026-11-10

### Rejilla de páginas

- **Vista general con `M`**: muestra todas las páginas del cómic como
  miniaturas y también está disponible en «Ver > Rejilla de páginas (M)».
  Funciona como una capa sobre el lector, así que cubre la barra inferior.
- **Carga perezosa**: solo se piden las miniaturas de la zona visible más dos
  filas de margen; al desplazarse se piden los siguientes bloques. En RAR
  sólidos se genera todo el lote en un único recorrido secuencial para no
  descomprimir el contenedor una vez por fila.
- **Navegación por teclado**: flechas para moverse por filas y columnas,
  `Inicio`/`Fin`, `Av Pág`/`Re Pág`, `Enter` para abrir la página seleccionada
  y `M`/`Esc` para cerrar. El clic sobre una miniatura la abre directamente.
- **Estado por celda**: número de página, marca «DOBLE» en páginas horizontales
  y aviso «No se pudo cargar» con el resto de la rejilla operativa.
- **Tamaño configurable**: 120, 160, 200 o 240 px en «Ver > Opciones de
  lectura…»; se aplica al reabrir la rejilla y se conserva entre sesiones.
- **Miniaturas nítidas**: se generan a 1,5 veces el tamaño lógico multiplicado
  por el DPI de la pantalla y se reducen con `CatmullRom`. Fyne las muestra con
  filtro suave, así que nunca amplía una imagen pequeña ni deja el texto
  emborronado.
- **Caché persistente de miniaturas**: archivos `.gth` en un presupuesto
  separado (25 % del límite de páginas, entre 16 y 128 MiB). Reabrir la rejilla
  evita descomprimir y decodificar de nuevo. «Acerca de» muestra el uso.

## v1.1.1 — 2026-11-09

Versión de corrección sobre v1.1.0.

### Corregido

- **Indicador de página en pantalla completa:** el contador `Página X / N` ahora
  se actualiza al navegar, en lugar de quedarse en la página que estaba visible
  al pulsar `F11`.
- **Entrada y salida del modo limpio:** al salir se restablece el estado interno,
  de modo que la siguiente pulsación de `F11` vuelve a entrar en pantalla
  completa y el indicador se oculta correctamente al recuperar las barras.

## v1.1.0 — 2026-11-09

Versión centrada en el rendimiento y la robustez al abrir cómics grandes,
dañados o en RAR sólido.

### Caché persistente

- **Índice en disco para RAR y ZIP:** al volver a abrir un cómic ya visto no se
  recorre el contenedor ni se repiten el filtrado y el orden. El índice se
  valida por ruta, tamaño, fecha de modificación, orden, filtro e integridad, y
  se descarta solo si algo cambió.
- **Páginas decodificadas entre sesiones:** las páginas se guardan en formato
  `GTP1` (RGBA crudo) dentro de la caché de la aplicación. Se eligió no
  comprimirlas porque descomprimir en cada apertura costaba más que decodificar
  el JPEG original.
- **Límite configurable:** 512 MB por defecto; el tamaño se ajusta y la caché
  se puede vaciar desde «Ayuda > Acerca de». La evicción es aproximadamente LRU
  por fecha de uso.
- **Estadísticas de caché:** el diálogo «Acerca de» muestra el espacio ocupado,
  el número de índices y de páginas guardadas.

### Apertura y precarga

- **Precarga en la dirección de lectura:** tras dibujar una página se preparan
  las tres siguientes y la anterior. Al cambiar de página se cancelan los
  trabajos antiguos para no competir con la lectura visible.
- **RAR sólidos en una sola pasada:** la precarga recorre el contenedor una vez
  para todas las páginas pendientes, en lugar de repetir el recorrido por cada
  una.
- **Progreso y cancelación en RAR sólidos:** la barra inferior muestra el avance
  del recorrido secuencial y `Esc` cancela un salto largo, volviendo a la última
  página realmente dibujada.

### Robustez

- **Verificación rápida de integridad al abrir:** se valida la estructura del
  contenedor y se muestrean la primera, la central y la última página con un
  presupuesto de 1,2 s. En RAR sólidos se omite el muestreo para no descomprimir
  el archivo entero. Los problemas se avisan, pero la lectura continúa.
- **Errores por página sin bloquear:** si una entrada falla, el área de lectura
  muestra un marcador con el detalle y un botón «Reintentar». Las flechas, `G` y
  el resto de la navegación siguen funcionando. Los errores al abrir el archivo
  completo mantienen el diálogo.

### Opciones de lectura

- **Orden de páginas configurable:** natural (por número) o alfabético estricto.
- **Filtro de basura:** omite `__MACOSX`, `.DS_Store` y archivos ocultos.
- Ambas opciones, junto con la verificación de integridad, viven en
  «Ver > Opciones de lectura…». Al aplicarlas se conserva la página visible por
  nombre, no por índice.

### Rendimiento observado

Con un cómic de prueba RAR:

- Reapertura desde el índice en disco: de ~535 ms a ~5 ms.
- Carga de una página desde la caché de disco: ~20–50 ms frente a ~230 ms
  decodificando el JPEG original.

## v1.0.0 — 2026-11-09

Primera versión pública de Gutter.

- Lectura de `.cbr`/`.cbz` (RAR v4/v5 y ZIP) detectados por *magic bytes*.
- Decodificación de JPEG, PNG, GIF, WebP, AVIF, BMP y TIFF.
- Reanudación por cómic, pantalla de bienvenida con recientes, arrastrar y
  soltar, tamaño de ventana recordado y asociación opcional de archivos.
- OCR y traducción de bocadillos bajo demanda.
- Zoom, scroll, modo limpio a pantalla completa, salto de página con `G`, corte
  de doble página y sentido de lectura occidental/manga.
- Caché LRU en RAM, caché de miniaturas y precarga de páginas.
