interface HSL {
  h: number
  s: number
  l: number
}

function adjustColor(hex: string, amount: number): string {
  const color = hex.replace('#', '')
  let r = parseInt(color.substring(0, 2), 16)
  let g = parseInt(color.substring(2, 4), 16)
  let b = parseInt(color.substring(4, 6), 16)

  r = Math.max(0, Math.min(255, r + amount))
  g = Math.max(0, Math.min(255, g + amount))
  b = Math.max(0, Math.min(255, b + amount))

  return `#${r.toString(16).padStart(2, '0')}${g.toString(16).padStart(2, '0')}${b.toString(16).padStart(2, '0')}`
}

function hexToHSL(hex: string): HSL {
  const color = hex.replace('#', '')
  const r = parseInt(color.substring(0, 2), 16) / 255
  const g = parseInt(color.substring(2, 4), 16) / 255
  const b = parseInt(color.substring(4, 6), 16) / 255

  const max = Math.max(r, g, b)
  const min = Math.min(r, g, b)
  let h = 0, s = 0
  const l = (max + min) / 2

  if (max !== min) {
    const d = max - min
    s = l > 0.5 ? d / (2 - max - min) : d / (max + min)
    switch (max) {
      case r: h = ((g - b) / d + (g < b ? 6 : 0)) / 6; break
      case g: h = ((b - r) / d + 2) / 6; break
      case b: h = ((r - g) / d + 4) / 6; break
    }
  }

  return {
    h: Math.round(h * 360),
    s: Math.round(s * 100),
    l: Math.round(l * 100)
  }
}

function hslToHex(h: number, s: number, l: number): string {
  s /= 100
  l /= 100

  const a = s * Math.min(l, 1 - l)
  const f = (n: number): number => {
    const k = (n + h / 30) % 12
    return l - a * Math.max(Math.min(k - 3, 9 - k, 1), -1)
  }

  const r = Math.round(f(0) * 255)
  const g = Math.round(f(8) * 255)
  const b = Math.round(f(4) * 255)

  return `#${r.toString(16).padStart(2, '0')}${g.toString(16).padStart(2, '0')}${b.toString(16).padStart(2, '0')}`
}

export function applyThemeColor(color: string, theme?: 'dark' | 'light'): void {
  const root = document.documentElement

  // 未显式传入主题时，从 DOM 当前类推断（夜间模式会设置 dark-theme 类）
  const isDark = theme
    ? theme === 'dark'
    : root.classList.contains('dark-theme')

  root.style.setProperty('--primary-color', color)

  const darkerColor = adjustColor(color, -20)
  root.style.setProperty('--primary-gradient', `linear-gradient(135deg, ${color} 0%, ${darkerColor} 100%)`)
  root.style.setProperty('--primary-hover', adjustColor(color, -15))
  root.style.setProperty('--primary-pressed', adjustColor(color, -30))

  const hsl = hexToHSL(color)
  const hue = hsl.h

  // 卡片基础亮度：
  //  - 亮色模式：保持原有莫兰迪浅色（>=50%），白色文字
  //  - 夜间模式：使用较深底色（<=34%），保证白色文字清晰可读
  const baseLight = isDark ? Math.min(hsl.l, 34) : Math.max(hsl.l, 50)
  const lightOffset = isDark ? 8 : 10

  const card1Color = hslToHex((hue + 0) % 360, Math.min(hsl.s * 0.6, 60), baseLight)
  const card2Color = hslToHex((hue + 60) % 360, Math.min(hsl.s * 0.6, 60), baseLight)
  const card3Color = hslToHex((hue + 120) % 360, Math.min(hsl.s * 0.6, 60), baseLight)
  const card4Color = hslToHex((hue + 180) % 360, Math.min(hsl.s * 0.6, 60), baseLight)

  root.style.setProperty('--card-color-1', card1Color)
  root.style.setProperty('--card-color-1-light', hslToHex((hue + 0) % 360, Math.min(hsl.s * 0.5, 50), Math.min(baseLight + lightOffset, 60)))
  root.style.setProperty('--card-color-2', card2Color)
  root.style.setProperty('--card-color-2-light', hslToHex((hue + 60) % 360, Math.min(hsl.s * 0.5, 50), Math.min(baseLight + lightOffset, 60)))
  root.style.setProperty('--card-color-3', card3Color)
  root.style.setProperty('--card-color-3-light', hslToHex((hue + 120) % 360, Math.min(hsl.s * 0.5, 50), Math.min(baseLight + lightOffset, 60)))
  root.style.setProperty('--card-color-4', card4Color)
  root.style.setProperty('--card-color-4-light', hslToHex((hue + 180) % 360, Math.min(hsl.s * 0.5, 50), Math.min(baseLight + lightOffset, 60)))

  // 主色可被用户任意选择，而「主色上的文字色」此前是写死的白色：
  // 一旦用户选了偏亮的主色（浅色模式下尤其明显），实心按钮就会变成
  // 白字浅底、几乎无法辨认。这里按主色亮度动态选择前景色，保证对比度。
  const onPrimary = readableOnColor(color)
  root.style.setProperty('--text-color-on-primary', onPrimary)

  // 鸿蒙 7.0 动态光效：主色辉光跟随用户主题色
  const glow = hexToRgba(color, 0.35)
  const glowStrong = hexToRgba(color, 0.55)
  root.style.setProperty('--glow-primary', glow)
  root.style.setProperty('--glow-primary-strong', glowStrong)
}

function hexToRgba(hex: string, alpha: number): string {
  const color = hex.replace('#', '')
  const r = parseInt(color.substring(0, 2), 16)
  const g = parseInt(color.substring(2, 4), 16)
  const b = parseInt(color.substring(4, 6), 16)
  return `rgba(${r}, ${g}, ${b}, ${alpha})`
}

/**
 * 为主色挑选对比度更高的前景色。
 *
 * 白色文字在深色底上可读、在浅色底上不可读，反之亦然。这里不做
 * 「亮度阈值」判断——中间色调（如琥珀 #E8B339、绿 #4CAF50）用阈值
 * 很容易误判——而是**分别计算黑白两种前景的实际对比度，取更高者**，
 * 这是 WCAG 对比度公式的直接应用，对任意主色都成立。
 */
function readableOnColor(hex: string): string {
  const background = relativeLuminance(hex)
  const white = 1
  const dark = relativeLuminance('#182431')

  const contrastWithWhite = contrastRatio(background, white)
  const contrastWithDark = contrastRatio(background, dark)

  return contrastWithDark > contrastWithWhite ? '#182431' : '#FFFFFF'
}

/** WCAG 相对对比度： (L1 + 0.05) / (L2 + 0.05)，L 为相对亮度。 */
function contrastRatio(a: number, b: number): number {
  const lighter = Math.max(a, b)
  const darker = Math.min(a, b)
  return (lighter + 0.05) / (darker + 0.05)
}

function relativeLuminance(hex: string): number {
  const color = hex.replace('#', '')
  if (color.length !== 6) return 0
  const channels = [0, 2, 4].map((i) => {
    const value = parseInt(color.substring(i, i + 2), 16) / 255
    return value <= 0.03928 ? value / 12.92 : Math.pow((value + 0.055) / 1.055, 2.4)
  })
  return 0.2126 * channels[0] + 0.7152 * channels[1] + 0.0722 * channels[2]
}


