import { useEffect, RefObject } from 'react'

interface SectionRefEntry {
  ref: RefObject<HTMLDivElement | null>
  id: string
}

interface UseSectionTrackingParams {
  open: boolean
  contentRootRef: RefObject<HTMLDivElement | null>
  sections: SectionRefEntry[]
  setActiveSectionId: (id: string) => void
}

export function useSectionTracking({
  open,
  contentRootRef,
  sections,
  setActiveSectionId
}: UseSectionTrackingParams): void {
  useEffect(() => {
    if (!open) return

    let cancelled = false
    let observer: IntersectionObserver | undefined
    let rafId: number | undefined

    const init = () => {
      if (cancelled) {
        if (rafId) cancelAnimationFrame(rafId)
        return
      }

      const root = contentRootRef.current?.parentElement ?? undefined
      const nodes = sections
        .map((s) => s.ref.current)
        .filter(Boolean) as HTMLDivElement[]

      if (!root || nodes.length === 0) {
        rafId = requestAnimationFrame(init)
        return
      }

      const idByNode = new Map<Element, string>(
        sections.map((s) => [s.ref.current as HTMLDivElement, s.id])
      )

      observer = new IntersectionObserver(
        (entries) => {
          const visible = entries
            .filter((e) => e.isIntersecting)
            .sort((a, b) => (b.intersectionRatio ?? 0) - (a.intersectionRatio ?? 0))[0]

          if (!visible) return
          const id = idByNode.get(visible.target)
          if (id) setActiveSectionId(id)
        },
        {
          root,
          threshold: [0.2, 0.35, 0.5, 0.65]
        }
      )

      nodes.forEach((n) => observer?.observe(n))
    }

    rafId = requestAnimationFrame(init)

    return () => {
      cancelled = true
      if (rafId) cancelAnimationFrame(rafId)
      if (observer) observer.disconnect()
    }
  }, [open])
}