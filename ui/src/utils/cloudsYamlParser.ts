/**
 * Client-side parser for the OpenStack clouds.yaml credential format.
 *
 * Mirrors the controller-side parser in
 * k8s/migration/pkg/utils/clouds_yaml.go so the UI can validate operator
 * input, surface inline errors, and populate the cloud-name selector + auth
 * method badge before submission.
 */
import yaml from 'js-yaml'

export type CloudsYamlAuthType =
  | 'v3password'
  | 'password'
  | 'v3applicationcredential'
  | string

export interface CloudsYamlAuth {
  auth_url?: string
  username?: string
  password?: string
  application_credential_id?: string
  application_credential_secret?: string
  project_name?: string
  project_id?: string
  user_domain_name?: string
  project_domain_name?: string
  [k: string]: unknown
}

export interface CloudsYamlEntry {
  auth_type?: CloudsYamlAuthType
  auth?: CloudsYamlAuth
  region_name?: string
  interface?: string
  verify?: boolean
  cacert?: string
  compute_api_version?: string
  volume_api_version?: string
  image_api_version?: string
  network_api_version?: string
  identity_api_version?: string
  [k: string]: unknown
}

export interface CloudsYamlFile {
  clouds?: Record<string, CloudsYamlEntry>
}

export interface ParseSuccess {
  ok: true
  raw: string
  parsed: CloudsYamlFile
  cloudNames: string[]
}

export interface ParseFailure {
  ok: false
  raw: string
  error: string
  // Best-effort line/column from js-yaml's YAMLException.
  line?: number
  column?: number
}

export type ParseResult = ParseSuccess | ParseFailure

/**
 * Parse clouds.yaml content. On success, returns the parsed structure plus the
 * list of top-level cloud entry names. On failure, returns an error message
 * suitable for inline display (with line/column when available).
 */
export function parseCloudsYAML(input: string): ParseResult {
  try {
    const parsed = yaml.load(input, { schema: yaml.JSON_SCHEMA }) as
      | CloudsYamlFile
      | null
      | undefined

    if (!parsed || typeof parsed !== 'object') {
      return {
        ok: false,
        raw: input,
        error: 'Expected a YAML mapping with a top-level "clouds" key.',
      }
    }
    if (!parsed.clouds || typeof parsed.clouds !== 'object') {
      return {
        ok: false,
        raw: input,
        error: 'Missing top-level "clouds:" mapping.',
      }
    }

    const cloudNames = Object.keys(parsed.clouds)
    if (cloudNames.length === 0) {
      return {
        ok: false,
        raw: input,
        error: '"clouds:" mapping contains no cloud entries.',
      }
    }

    return { ok: true, raw: input, parsed, cloudNames }
  } catch (e) {
    const yamlErr = e as { reason?: string; mark?: { line: number; column: number } }
    return {
      ok: false,
      raw: input,
      error: yamlErr.reason ?? (e instanceof Error ? e.message : String(e)),
      line: yamlErr.mark?.line,
      column: yamlErr.mark?.column,
    }
  }
}

/**
 * Detect the auth method declared by a cloud entry. Mirrors the controller's
 * allowlist: empty / v3password / password / v3applicationcredential.
 * Anything else is reported as "unsupported" so the UI can surface a warning.
 */
export type AuthMethodKind =
  | 'password'
  | 'applicationCredential'
  | 'unsupported'

export function detectAuthMethod(entry?: CloudsYamlEntry): AuthMethodKind {
  if (!entry) {
    return 'unsupported'
  }
  const t = entry.auth_type ?? ''
  if (t === '' || t === 'password' || t === 'v3password') {
    return 'password'
  }
  if (t === 'v3applicationcredential') {
    return 'applicationCredential'
  }
  return 'unsupported'
}

/**
 * Return a copy of the cloud entry with secret-bearing fields masked, suitable
 * for display in a post-parse summary. The operator can confirm the parsed
 * structure without seeing the actual secret value.
 */
export function maskSecrets(entry: CloudsYamlEntry): CloudsYamlEntry {
  if (!entry.auth) {
    return entry
  }
  const masked: CloudsYamlAuth = { ...entry.auth }
  if (masked.password !== undefined) {
    masked.password = '••••••••'
  }
  if (masked.application_credential_secret !== undefined) {
    masked.application_credential_secret = '••••••••'
  }
  return { ...entry, auth: masked }
}
