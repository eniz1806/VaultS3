import { useState, useEffect, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'

export function useKeyboardShortcuts() {
  const [showHelp, setShowHelp] = useState(false)
  const navigate = useNavigate()

  const handleKeyDown = useCallback((e: KeyboardEvent) => {
    // Ignore when typing in inputs
    const tag = (e.target as HTMLElement).tagName
    if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') {
      if (e.key === 'Escape') {
        ;(e.target as HTMLElement).blur()
      }
      return
    }

    // Ignore with modifier keys (allow Shift for ?)
    if (e.ctrlKey || e.metaKey || e.altKey) return

    switch (e.key) {
      case '/': {
        e.preventDefault()
        navigate('/search')
        // Focus the search input after navigation
        setTimeout(() => {
          const input = document.querySelector<HTMLInputElement>('input[type="text"], input[type="search"]')
          input?.focus()
        }, 100)
        break
      }
      case '?': {
        e.preventDefault()
        setShowHelp(prev => !prev)
        break
      }
      case 'Escape': {
        setShowHelp(false)
        break
      }
    }
  }, [navigate])

  useEffect(() => {
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [handleKeyDown])

  return { showHelp, setShowHelp }
}

export const shortcuts = [
  { key: '/', description: 'Go to Search' },
  { key: '?', description: 'Toggle shortcut help' },
  { key: 'Esc', description: 'Close modal / blur input' },
]
