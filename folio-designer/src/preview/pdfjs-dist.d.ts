declare module 'pdfjs-dist/build/pdf.mjs' {
  export type RenderTask = { promise: Promise<void>; cancel(): void }
  export type PDFPageProxy = { getViewport(options: { scale: number }): { width: number; height: number }; render(options: { canvasContext: CanvasRenderingContext2D; viewport: { width: number; height: number } }): RenderTask }
  export type PDFDocumentProxy = { numPages: number; getPage(page: number): Promise<PDFPageProxy>; destroy(): Promise<void> }
  export type PDFDocumentLoadingTask = { promise: Promise<PDFDocumentProxy>; destroy(): Promise<void> }
  export const GlobalWorkerOptions: { workerSrc: string }
  export function getDocument(options: unknown): PDFDocumentLoadingTask
}
