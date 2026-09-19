// Keep PDF.js CMaps and standard fonts in Vite's immutable asset graph.
export const pdfjsRuntimeAssets = import.meta.glob('./runtime/pdfjs-*/**/*', { eager: true, query: '?url', import: 'default' })
export const pdfjsViewerAssets = { cMapUrl: '/assets/pdfjs-cmaps-63e94418f6a31380c20a/', standardFontDataUrl: '/assets/pdfjs-standard-fonts-235124a44157171aab33/', cMapPacked: true } as const
