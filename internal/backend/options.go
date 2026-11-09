package backend

// SortMode define cómo se ordenan las páginas dentro del contenedor.
type SortMode string

const (
	// SortNatural coloca "page2.jpg" antes que "page10.jpg". Es el orden que
	// espera la mayoría de cómics y el valor por defecto.
	SortNatural SortMode = "natural"
	// SortLexicographic compara los nombres como cadenas, sin interpretar los
	// tramos numéricos. Útil para archivos cuyo orden real no es numérico.
	SortLexicographic SortMode = "lexicographic"
)

// OpenOptions agrupa los ajustes que afectan a la indexación de un cómic.
//
// El valor cero es válido y reproduce el comportamiento histórico: orden
// natural, filtrado de basura y sin caché en disco.
type OpenOptions struct {
	// Sort elige el orden de las páginas.
	Sort SortMode
	// IncludeJunk conserva entradas ocultas y de sistema (__MACOSX, .DS_Store,
	// carpetas que empiezan por punto…). Por defecto se filtran.
	IncludeJunk bool
	// DiskCache, si no es nil, guarda el índice y las páginas decodificadas
	// entre sesiones.
	DiskCache *DiskCache
	// IntegrityCheck hace una comprobación estructural y por muestreo al abrir.
	// Es rápida (presupuesto de un segundo) y no bloquea la lectura si falla.
	IntegrityCheck bool
}

func normalizeOpenOptions(opts OpenOptions) OpenOptions {
	if opts.Sort != SortLexicographic {
		opts.Sort = SortNatural
	}
	return opts
}

// PageProblem describe una entrada que no se pudo validar al abrir el cómic.
// La lectura continúa: la página afectada mostrará un marcador con reintento.
type PageProblem struct {
	Name   string `json:"name"`
	Detail string `json:"detail"`
}
