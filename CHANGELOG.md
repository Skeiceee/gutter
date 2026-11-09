# Novedades de Gutter

Todas las versiones publicadas de Gutter, con los cambios más visibles para
quien usa el lector. El ejecutable de cada versión está en
[Releases](https://github.com/Skeiceee/gutter/releases).

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
