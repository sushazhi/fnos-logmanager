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

export function applyThemeColor(color: string): void {
  const root = document.documentElement
  root.style.setProperty('--primary-color', color)

  const darkerColor = adjustColor(color, -20)
  root.style.setProperty('--primary-gradient', `linear-gradient(135deg, ${color} 0%, ${darkerColor} 100%)`)
  root.style.setProperty('--primary-hover', adjustColor(color, -15))
  root.style.setProperty('--primary-pressed', adjustColor(color, -30))

  const hsl = hexToHSL(color)
  const hue = hsl.h

  const card1Color = hslToHex((hue + 0) % 360, Math.min(hsl.s * 0.6, 60), Math.max(hsl.l, 50))
  const card2Color = hslToHex((hue + 60) % 360, Math.min(hsl.s * 0.6, 60), Math.max(hsl.l, 50))
  const card3Color = hslToHex((hue + 120) % 360, Math.min(hsl.s * 0.6, 60), Math.max(hsl.l, 50))
  const card4Color = hslToHex((hue + 180) % 360, Math.min(hsl.s * 0.6, 60), Math.max(hsl.l, 50))

  root.style.setProperty('--card-color-1', card1Color)
  root.style.setProperty('--card-color-1-light', hslToHex((hue + 0) % 360, Math.min(hsl.s * 0.5, 50), Math.max(hsl.l + 10, 60)))
  root.style.setProperty('--card-color-2', card2Color)
  root.style.setProperty('--card-color-2-light', hslToHex((hue + 60) % 360, Math.min(hsl.s * 0.5, 50), Math.max(hsl.l + 10, 60)))
  root.style.setProperty('--card-color-3', card3Color)
  root.style.setProperty('--card-color-3-light', hslToHex((hue + 120) % 360, Math.min(hsl.s * 0.5, 50), Math.max(hsl.l + 10, 60)))
  root.style.setProperty('--card-color-4', card4Color)
  root.style.setProperty('--card-color-4-light', hslToHex((hue + 180) % 360, Math.min(hsl.s * 0.5, 50), Math.max(hsl.l + 10, 60)))
}


