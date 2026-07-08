import { getBlob } from '../axios'
import { VJAILBREAK_DEFAULT_NAMESPACE } from '../constants'

const DEBUG_BUNDLE_PATH = '/dev-api/sdk/vpw/v1/debug-bundle'

export const downloadDebugBundle = async (
  migrationName?: string,
  namespace = VJAILBREAK_DEFAULT_NAMESPACE,
  podName?: string
): Promise<void> => {
  // The backend needs migration or pod; with only a pod it resolves the
  // migration via spec.podRef.
  const params: Record<string, string> = { namespace }
  if (migrationName) params.migration = migrationName
  if (podName) params.pod = podName

  const response = await getBlob({
    endpoint: DEBUG_BUNDLE_PATH,
    config: { params },
  })
  const cd = response.headers['content-disposition'] as string | undefined
  const filename =
    cd?.match(/filename="([^"]+)"/)?.[1] ??
    `${migrationName || podName || 'migration'}-debug-bundle.tar.gz`
  const objectUrl = URL.createObjectURL(response.data)
  const a = document.createElement('a')
  a.href = objectUrl
  a.download = filename
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(objectUrl)
}
