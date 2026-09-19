export type ShortcutHints = Readonly<{ save: string; undo: string; redo: string; delete: string; duplicate: string; copy: string; paste: string; selectAll: string; nudge: string; preview: string; snap: string }>

export function isMacPlatform(platform = typeof navigator === 'undefined' ? '' : navigator.platform): boolean {
  return /mac/i.test(platform)
}

export function shortcutHintsFor(mac = isMacPlatform()): ShortcutHints {
  return mac
    ? { save: '⌘S', undo: '⌘Z', redo: '⇧⌘Z', delete: 'Delete', duplicate: '⌘D', copy: '⌘C', paste: '⌘V', selectAll: '⌘A', nudge: 'Arrow keys', preview: '⌥P', snap: '⌥S' }
    : { save: 'Ctrl+S', undo: 'Ctrl+Z', redo: 'Ctrl+Y', delete: 'Delete', duplicate: 'Ctrl+D', copy: 'Ctrl+C', paste: 'Ctrl+V', selectAll: 'Ctrl+A', nudge: 'Arrow keys', preview: 'Alt+P', snap: 'Alt+S' }
}

export function primaryModifier(event: Pick<KeyboardEvent, 'metaKey' | 'ctrlKey'>, mac: boolean): boolean {
  return mac ? event.metaKey && !event.ctrlKey : event.ctrlKey && !event.metaKey
}
