// Light / dark theme. The initial value is set by an inline script in index.html (no flash);
// this hook just reads and flips it, remembering the choice.

import { useCallback, useState } from 'react'

export type Theme = 'light' | 'dark'

export function useTheme() {
  const [theme, setTheme] = useState<Theme>(() =>
    document.documentElement.dataset.theme === 'dark' ? 'dark' : 'light',
  )
  const toggle = useCallback(() => {
    const next: Theme = theme === 'dark' ? 'light' : 'dark'
    document.documentElement.dataset.theme = next
    document.querySelector('meta[name=theme-color]')?.setAttribute('content', next === 'dark' ? '#121311' : '#f6f4ef')
    try { localStorage.setItem('theme', next) } catch { /* private mode */ }
    setTheme(next)
  }, [theme])
  return { theme, toggle }
}
