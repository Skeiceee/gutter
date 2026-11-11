# Arquitectura de Gutter

## Visión general

Gutter es un lector de cómics `.cbr`/`.cbz` escrito en Go con Fyne 2.8.2.
Su objetivo es abrir archivos grandes (RAR5, ZIP, cientos de páginas) sin
extraerlos completamente: indexa el contenedor, descompone solo la página
solicitada y mantiene una caché LRU en RAM.

La aplicación sigue una **arquitectura por capas** con paquetes internos
autocontenidos bajo `internal/`, y un paquete `app` que orquesta la UI.

---

## Paquetes

### `main` (raíz del módulo)

- `main.go`: único punto de entrada. Llama a `app.Run()`.
- Contiene únicamente `func main()`. Toda la lógica vive en `internal/app`.

### `internal/app`

Orquesta la ventana Fyne y toda la lógica de lectura.

| Archivo | Responsabilidad |
|---------|-----------------|
| `app.go` | Struct `gutterApp`, `NewGutterApp()`, `Run()`, `FitWindowToPrimaryScreen()`. Estado global de la app (página actual, zoom, modo limpio, etc.). |
| `dialogs.go` | `showDialog`, `hideActiveDialog`, `openFile`, `LoadComic`, `cancelPageWork`, `cancelLoading`, `isComicPath`, `loadBrandResource`. |
| `grid.go` | Rejilla de páginas: celdas de miniatura, carga perezosa por bloques, navegación por teclado, selección y estados de error. |
| `help.go` | Menú Archivo/Ver/Traducción/Ayuda, hoja de atajos (`F1`, `?`), "Acerca de". |
| `hud.go` | Indicador flotante de página en pantalla completa y selector `G`. |
| `library.go` | Progreso persistente, lista de recientes, pantalla de bienvenida. |
| `navigation.go` | Zoom, scroll, paginación, precarga de las tres páginas siguientes y la anterior, `updateControls`, `clampZoom`, `updateZoomLabel`. |
| `run.go` | `Run()`: crea la aplicación Fyne, instala el tema, configura `OnStarted` y lanza `ShowAndRun()`. |
| `settings.go` | Preferencias de lectura (orden, filtro, integridad), caché de disco, diálogo de opciones y estadísticas de «Acerca de». |
| `translate_ui.go` | Activación de OCR/traducción bajo demanda (`T`), caché por página, diálogo de configuración. |
| `ui.go` | `showPage`, `reportLoadError`, marcador de página fallida, `toggleCleanMode`, `handleKey`, `buildBottomBar`, `onReaderResize`, `updateZoomLabel`. |

**Flujo de arranque:**

```
main.Run()                      ← main.go llama a app.Run()
  └─ app.NewWithID(...)
  └─ theme.NewGutterTheme()
  └─ NewGutterApp(a)            ← crea ventana, menú, HUD, controles
  └─ a.Lifecycle().SetOnStarted()
        └─ FitWindowToPrimaryScreen()
        └─ LoadComic(initialComic)  (si se pasó por CLI)
  └─ app.Run()                  ← window.ShowAndRun()
```

### `internal/backend`

Índice de archivos `.cbr`/`.cbz` (RAR v4/v5 y ZIP), extracción selectiva de páginas,
decodificación multiformato (JPEG, PNG, GIF, WebP, AVIF, BMP, TIFF), caché LRU
de 96 MiB con *single-flight*, precarga y caché persistente opcional.

| Archivo | Responsabilidad |
|---------|-----------------|
| `backend.go` | Motor de lectura, índice RAR/ZIP, `PrefetchContext`, `ThumbnailsContext`, estadísticas de memoria y limpieza de cachés. |
| `disk_cache.go` | Índice `.json` por tamaño/fecha/configuración, páginas `.gtp` en RGBA crudo y miniaturas `.gth`; evicción LRU y presupuestos separados. |
| `integrity.go` | Comprobación estructural y por muestreo con presupuesto de 1,2 s; devuelve `PageProblem`. |
| `options.go` | `OpenOptions`, `SortMode` (natural/lexicográfico) y `PageProblem`. |

- Detección por *magic bytes*: `Rar!` para RAR, `PK` para ZIP.
- Corte de doble página con `SubImage` compactado a `*image.RGBA`.
- Caché LRU de 96 MiB; caché de miniaturas independiente de 24 MiB.
- `ThumbnailsContext` entrega varias miniaturas por nombre. En RAR sólidos
  recorre el contenedor una sola vez y descarta las entradas que no se piden.
- Las miniaturas se guardan en `.gth` con la misma cabecera que las páginas,
  en un directorio y presupuesto aparte (25 % del límite de páginas, entre 16 y
  128 MiB). La clave incluye el tamaño máximo para no mezclar resoluciones.
- La rejilla pide las miniaturas a 1,5 × tamaño lógico × escala de pantalla y
  las reduce con `CatmullRom`. Así el marco nunca tiene que ampliarlas y el
  texto de las páginas se mantiene legible.
- `OpenComic` conserva el comportamiento histórico (memoria, orden natural); la
  app usa `OpenComicWithOptions` para activar caché, filtro e integridad.

### `internal/ocr`

Motor de OCR y traducción.

- **OCR**: Windows integrado (`Windows.Media.Ocr` vía PowerShell) y Tesseract CLI
  (Linux/macOS) como respaldo.
- **Traducción**: MyMemory (por defecto, sin clave), LibreTranslate o DeepL.
- **Overlay**: capa transparente con bocadillos traducidos y fuente CJK.

### `internal/theme`

Paleta de colores oficial de Gutter y `fyne.Theme` con variante oscura fija:

| Elemento | Color |
|----------|-------|
| Fondo (`ColorNameBackground`) | `#121212` |
| Paneles/barras (`ColorNameHeaderBackground`, etc.) | `#1E1E1E` |
| Botones (`ColorNameButton`) | `#252526` |
| Hover (`ColorNameHover`) | `#333333` |
| Acento (`ColorNamePrimary`) | `#007ACC` |
| Texto claro | blanco/gris claro |

### `internal/ui`

Layouts personalizados y contenedores de Fyne:

| Archivo | Qué contiene |
|---------|--------------|
| `layouts.go` | `PageLayout`, `RootLayout`, `BottomBarLayout`, `BottomCenterLayout`, `BottomShadowLayout`, `ReaderLayout`. |
| `scroll.go` | `ScrollSinSombras`, `NewScrollSinSombras()`: `Scroll` de Fyne sin sombras internas de borde (eliminan la franja despegada de la barra inferior). |

### `internal/cursor`

`CleanCursor`: rectángulo transparente que implementa `desktop.Cursorable` para
ocultar el puntero en modo lectura limpia.

### `internal/screen`

`PrimaryWorkArea()`: obtiene el área útil del monitor principal (sin barra de
tareas) vía GLFW en escritorio; devuelve cero en móvil/web.

### `internal/associate`

Asociación de archivos `.cbr`/`.cbz` con Gutter en Windows (registro en
`HKCU\Software\Classes`). Solo afecta al usuario actual y pide confirmación.

---

## Flujo de datos al abrir un cómic

```
Usuario selecciona .cbr/.cbz
        │
        ▼
openFile (dialogs.go)
        │
        ▼
LoadComic(path) (dialogs.go)
        │
        ├─► OpenComicWithOptions(path, opts)   internal/backend
        │      └─► detectArchiveKind (magic bytes)
        │      └─► DiskCache.LoadIndex (tamaño + fecha + configuración)
        │      └─► openRARComicWithOptions / openZIPComicWithOptions
        │      └─► finishIndex (orden natural o lexicográfico)
        │      └─► quickIntegrityCheck (si está activa)
        │
        ├─► restoreProgress(comic)   library.go (prefs)
        │
        └─► showPage()               ui.go
               │
               ├─► comic.PageContext (backend)
               │      ├─► DiskCache.LoadPage (si existe)
               │      ├─► readEntry (RAR/ZIP)
               │      ├─► toRGBA (normaliza a image.RGBA)
               │      └─► CropHalf (si doble página)
               │
               ├─► applyPageZoom (zoom 65% ancho por defecto)
               │
               └─► prefetchAdjacent (tres siguientes + anterior)
```

### Flujo de la rejilla de páginas

```
M / Ver > Rejilla de páginas
        │
        ▼
grid.go: showThumbnailGrid
        │
        ├─► crea una celda por página y la capa sobre RootLayout
        ├─► espera al primer layout (60 ms) para conocer columnas y scroll
        └─► loadGridVisible
               │
               ├─► ZIP / RAR normal: lote de la zona visible + 2 filas
               └─► RAR sólido: lote completo (0..n-1) en una sola pasada
                      │
                      ▼
               Comic.ThumbnailsContext (backend.go)
                      ├─► memoria (LRU de miniaturas)
                      ├─► disco (.gth, presupuesto separado)
                      └─► readEntry + image.Decode + makeThumbnail
                             │
                             ▼
               callback → fyne.Do → applyGridThumbnail
                      ├─► celda lista, «DOBLE» o error
                      └─► updateGridStatus
```

Al cerrar (`M`, `Esc`, `Enter` o cambio de cómic) se cancela el contexto, se
incrementa `gridSeq` para descartar resultados en vuelo y se vacía la capa.

---

## Reglas de diseño

1. **Un solo archivo en raíz**: `main.go` es el único punto de entrada. Todo el resto vive bajo `internal/`.
2. **Paquetes privados**: todo lo bajo `internal/` es inaccesible desde fuera del módulo `gutter`.
3. **Estado compartido mínimo**: `gutterApp` concentra el estado; los paquetes `backend` y `ocr` no saben de la UI.
4. **Un solo diálogo a la vez**: `showDialog` / `hideActiveDialog` evitan apilar ventanas al pulsar F1/Ctrl+O repetidamente.
5. **Caché LRU de 96 MiB**: las páginas decodificadas se mantienen en memoria para evitar recompresión al navegar.
6. **Zoom por defecto al 65%**: la página ocupa el 65% del ancho disponible, dejando aire a los lados.
7. **La caché en disco es opcional y best-effort**: un fallo de escritura nunca impide leer; la app puede funcionar sin directorio de caché.
8. **La lectura nunca se bloquea por una página rota**: los errores de página se muestran en el área de lectura con «Reintentar»; solo los fallos al abrir usan diálogo.
9. **La rejilla no bloquea la lectura**: cancela sus cargas al cerrarse, descarta lotes antiguos por secuencia y pide solo la zona visible, salvo en RAR sólidos donde un único recorrido secuencial es más barato.

---

## Construcción

```powershell
# Windows (recomendado: LLVM-MinGW / clang)
.\build.ps1

# Linux / macOS
CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o gutter .
```

**Icono y recursos de Windows:** `assets/gutter.ico` y `assets/gutter.rc`
contienen el icono y las propiedades del archivo. Se compilan una sola vez al
objeto `rsrc_windows_amd64.syso`, que Go incrusta automáticamente al compilar
para Windows amd64:

```powershell
cd assets
windres -i gutter.rc -o ..\rsrc_windows_amd64.syso
```

**Pruebas:**
```bash
CGO_ENABLED=1 go test ./...
```

---

## Licencia

Código y comentarios en español. Proyecto de código abierto.