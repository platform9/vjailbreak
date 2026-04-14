import { useCallback, useEffect, useRef, useState } from 'react'

export type MigrationFormSectionId =
  | 'source-destination'
  | 'select-vms'
  | 'map-resources'
  | 'security'
  | 'options'

export function useMigrationFormSections({ open }: { open: boolean }) {
  const contentRootRef = useRef<HTMLDivElement | null>(null)
  const section1Ref = useRef<HTMLDivElement | null>(null)
  const section2Ref = useRef<HTMLDivElement | null>(null)
  const section3Ref = useRef<HTMLDivElement | null>(null)
  const section4Ref = useRef<HTMLDivElement | null>(null)
  const section5Ref = useRef<HTMLDivElement | null>(null)

  const [activeSectionId, setActiveSectionId] = useState<MigrationFormSectionId>('source-destination')

  const scrollToSection = useCallback((id: MigrationFormSectionId) => {
    const map: Record<MigrationFormSectionId, React.RefObject<HTMLDivElement | null>> = {
      'source-destination': section1Ref,
      'select-vms': section2Ref,
      'map-resources': section3Ref,
      security: section4Ref,
      options: section5Ref
    }

    const el = map[id]?.current
    if (!el) return
    el.scrollIntoView({ behavior: 'smooth', block: 'start' })
    setActiveSectionId(id)
  }, [])

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
      const nodes = [
        section1Ref.current,
        section2Ref.current,
        section3Ref.current,
        section4Ref.current,
        section5Ref.current
      ].filter(Boolean) as HTMLDivElement[]

      if (!root || nodes.length === 0) {
        rafId = requestAnimationFrame(init)
        return
      }

      const idByNode = new Map<Element, MigrationFormSectionId>([
        [section1Ref.current as HTMLDivElement, 'source-destination'],
        [section2Ref.current as HTMLDivElement, 'select-vms'],
        [section3Ref.current as HTMLDivElement, 'map-resources'],
        [section4Ref.current as HTMLDivElement, 'security'],
        [section5Ref.current as HTMLDivElement, 'options']
      ])

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

  return {
    contentRootRef,
    section1Ref,
    section2Ref,
    section3Ref,
    section4Ref,
    section5Ref,
    activeSectionId,
    setActiveSectionId,
    scrollToSection
  }
}
