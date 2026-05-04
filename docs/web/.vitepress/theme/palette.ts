// Single source-of-truth synthwave palette, shared with the Go TUI.
// Edits MUST happen in /internal/ui/theme/palette.json — both the binary
// and this docs site read from that one file.
import palette from '../../../../internal/ui/theme/palette.json'

export type PaletteKey =
  | 'NeonPink'
  | 'NeonCyan'
  | 'NeonMagenta'
  | 'NeonLime'
  | 'NeonPurple'
  | 'NeonOrange'
  | 'NeonYellow'
  | 'NeonBlue'
  | 'HotPink'
  | 'Black'
  | 'White'
  | 'Dim'
  | 'DarkBg'

export const colors = palette as Record<PaletteKey, string>
export default colors
