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

  // 夜间模式（深底）下，渐变卡片文字保持白色；亮色模式移除 inline 覆盖，回落到 CSS 默认
  if (isDark) {
    root.style.setProperty('--text-color-on-primary', '#FFFFFF')
  } else {
    root.style.removeProperty('--text-color-on-primary')
  }

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


