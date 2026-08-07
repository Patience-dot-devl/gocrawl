import { useMemo, useState } from 'react'

export type SortDir = 'asc' | 'desc'

// Shared column-sort behavior for the issues and history tables: click a header to sort
// ascending, click again to reverse, click a different header to sort by that one instead.
export function useSort<T>(
  rows: T[],
  accessor: (row: T, key: string) => string | number,
  initial?: { key: string; dir?: SortDir },
) {
  const [key, setKey] = useState<string | null>(initial?.key ?? null)
  const [dir, setDir] = useState<SortDir>(initial?.dir ?? 'asc')

  function toggle(nextKey: string) {
    if (key === nextKey) {
      setDir((d) => (d === 'asc' ? 'desc' : 'asc'))
    } else {
      setKey(nextKey)
      setDir('asc')
    }
  }

  const sorted = useMemo(() => {
    if (!key) return rows
    const factor = dir === 'asc' ? 1 : -1
    return [...rows].sort((a, b) => {
      const x = accessor(a, key)
      const y = accessor(b, key)
      if (x < y) return -factor
      if (x > y) return factor
      return 0
    })
  }, [rows, accessor, key, dir])

  function dirFor(columnKey: string): SortDir | undefined {
    return key === columnKey ? dir : undefined
  }

  return { sorted, toggle, dirFor }
}
