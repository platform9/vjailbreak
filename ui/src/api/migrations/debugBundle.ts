import { getBlob } from '../axios'
import { VJAILBREAK_DEFAULT_NAMESPACE } from '../constants'

const DEBUG_BUNDLE_PATH = '/dev-api/sdk/vpw/v1/debug-bundle'

export const downloadDebugBundle = async (
  migrationName: string,
  namespace = VJAILBREAK_DEFAULT_NAMESPACE
): Promise<void> => {
  const response = await getBlob({
    endpoint: DEBUG_BUNDLE_PATH,
    config: { params: { migration: migrationName, namespace } },
  })
  const cd = response.headers['content-disposition'] as string | undefined
  const filename =
    cd?.match(/filename="([^"]+)"/)?.[1] ?? `${migrationName}-debug-bundle.txt`
  const objectUrl = URL.createObjectURL(response.data)
  const a = document.createElement('a')
  a.href = objectUrl
  a.download = filename
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(objectUrl)
}
