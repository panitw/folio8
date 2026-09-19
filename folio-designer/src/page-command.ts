// SPEC-multi-pages story 2. The three page commands, as opaque Go-defined
// bytes. Each is ONE command, so each is one undo entry. Page indexes are
// 0-based, the numbering `pages[i]` uses; the author reads them as "Page N".
//
// THIS MODULE DECIDES NOTHING. The engine refuses deleting the only page and
// setting page 1's Page Break, and moves content between the one-page and
// pages shapes itself.
import { commandBytes, jsonBoolean, jsonNumber } from './command-json'

// Without `after` the new page goes at the end.
export function addPageCommand(after?: number): ArrayBuffer {
  return commandBytes('addPage', after === undefined ? [] : [['after', jsonNumber(after)]])
}

export function deletePageCommand(page: number): ArrayBuffer {
  return commandBytes('deletePage', [['page', jsonNumber(page)]])
}

export function setPageBreakCommand(page: number, pageBreak: boolean): ArrayBuffer {
  return commandBytes('setPageBreak', [['page', jsonNumber(page)], ['pageBreak', jsonBoolean(pageBreak)]])
}
