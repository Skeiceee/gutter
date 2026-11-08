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
| `help.go` | Menú Archivo/Ver/Traducción/Ayuda, hoja de atajos (`F1`, `?`), "Acerca de". |
| `hud.go` | Indicador flotante de página en pantalla completa y selector `G`. |
| `library.go` | Progreso persistente, lista de recientes, pantalla de bienvenida. |
| `navigation.go` | Zoom, scroll, paginación, prefetch de páginas adyacentes, `updateControls`, `clampZoom`, `updateZoomLabel`. |
| `run.go` | `Run()`: crea la aplicación Fyne, instala el tema, configura `OnStarted` y lanza `ShowAndRun()`. |
| `translate_ui.go` | Activación de OCR/traducción bajo demanda (`T`), caché por página, diálogo de configuración. |
| `ui.go` | `showPage`, `reportLoadError`, `toggleCleanMode`, `handleKey`, `buildBottomBar`, `onReaderResize`, `updateZoomLabel`. |

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
decodificación multiformato (JPEG, PNG, GIF, WebP, AVIF, BMP, TIFF) y caché LRU
de 96 MiB con *single-flight* y prefetch.

- Detección por *magic bytes*: `Rar!` para RAR, `PK` para ZIP.
- Corte de doble página con `SubImage` compactado a `*image.RGBA`.
- Caché LRU de 96 MiB; caché de miniaturas independiente de 24 MiB.

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
        ├─► OpenComic(path)          internal/backend
        │      └─► detectArchiveKind (magic bytes)
        │      └─► openRARComic / openZIPComic
        │      └─► ReadComicIndex (orden natural)
        │
        ├─► restoreProgress(comic)   library.go (prefs)
        │
        └─► showPage()               ui.go
               │
               ├─► comic.GetPageContext (backend)
               │      ├─► readEntry (RAR/ZIP)
               │      ├─► toRGBA (normaliza a image.RGBA)
               │      └─► CropHalf (si doble página)
               │
               ├─► applyPageZoom (zoom 65% ancho por defecto)
               │
               └─► prefetchAdjacent (página anterior + siguiente)
```

---

## Reglas de diseño

1. **Un solo archivo en raíz**: `main.go` es el único punto de entrada. Todo el resto vive bajo `internal/`.
2. **Paquetes privados**: todo lo bajo `internal/` es inaccesible desde fuera del módulo `gutter`.
3. **Estado compartido mínimo**: `gutterApp` concentra el estado; los paquetes `backend` y `ocr` no saben de la UI.
4. **Un solo diálogo a la vez**: `showDialog` / `hideActiveDialog` evitan apilar ventanas al pulsar F1/Ctrl+O repetidamente.
5. **Caché LRU de 96 MiB**: las páginas decodificadas se mantienen en memoria para evitar recompresión al navegar.
6. **Zoom por defecto al 65%**: la página ocupa el 65% del ancho disponible, dejando aire a los lados.

---

## Construcción

```powershell
# Windows (recomendado: LLVM-MinGW / clang)
.\build.ps1

# Linux / macOS
CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o gutter .
```

**Pruebas:**
```bash
CGO_ENABLED=1 go test ./...
```

---

## Licencia

Código y comentarios en español. Proyecto de código abierto.