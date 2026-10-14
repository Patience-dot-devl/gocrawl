import { useEffect } from 'react'

// Sets document.title to "<title> - gocrawl" while the calling view is on screen, so browser
// history, tabs and screen-reader window lists tell the views apart (WCAG 2.4.2).
export function usePageTitle(title: string): void {
  useEffect(() => {
    document.title = `${title} - gocrawl`
  }, [title])
}
