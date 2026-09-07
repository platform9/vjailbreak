import { describe, expect, it } from 'vitest'
import { validateProxyAuthFields, type ProxyAuthFieldsInput } from './helpers'

const base: ProxyAuthFieldsInput = {
  enabled: false,
  configured: false,
  username: '',
  password: '',
  httpsOverride: false,
  httpsUsername: '',
  httpsPassword: ''
}

describe('validateProxyAuthFields', () => {
  it('passes when auth is disabled, regardless of other fields', () => {
    expect(validateProxyAuthFields({ ...base, enabled: false })).toBeNull()
  })

  it('requires credentials when newly enabled and nothing is configured yet', () => {
    expect(validateProxyAuthFields({ ...base, enabled: true })).toBe(
      'Proxy username and password are required when proxy authentication is enabled.'
    )
  })

  it('passes when enabled, already configured, and fields are left blank (no change)', () => {
    expect(validateProxyAuthFields({ ...base, enabled: true, configured: true })).toBeNull()
  })

  it('rejects a username with no password', () => {
    expect(
      validateProxyAuthFields({ ...base, enabled: true, username: 'proxyuser', password: '' })
    ).toBe('Both proxy username and password are required to update credentials.')
  })

  it('rejects a password with no username', () => {
    expect(
      validateProxyAuthFields({ ...base, enabled: true, username: '', password: 'proxypass' })
    ).toBe('Both proxy username and password are required to update credentials.')
  })

  it('passes with a valid shared username/password and no HTTPS override', () => {
    expect(
      validateProxyAuthFields({
        ...base,
        enabled: true,
        username: 'proxyuser',
        password: 'proxypass'
      })
    ).toBeNull()
  })

  it('requires HTTPS-specific credentials when override is on, even with valid shared creds', () => {
    expect(
      validateProxyAuthFields({
        ...base,
        enabled: true,
        username: 'proxyuser',
        password: 'proxypass',
        httpsOverride: true
      })
    ).toBe(
      'Both HTTPS proxy username and password are required when using different HTTPS credentials.'
    )
  })

  it('rejects a partially-filled HTTPS override pair', () => {
    expect(
      validateProxyAuthFields({
        ...base,
        enabled: true,
        username: 'proxyuser',
        password: 'proxypass',
        httpsOverride: true,
        httpsUsername: 'httpsuser',
        httpsPassword: ''
      })
    ).toBe(
      'Both HTTPS proxy username and password are required when using different HTTPS credentials.'
    )
  })

  it('passes with a complete HTTPS override pair', () => {
    expect(
      validateProxyAuthFields({
        ...base,
        enabled: true,
        username: 'proxyuser',
        password: 'proxypass',
        httpsOverride: true,
        httpsUsername: 'httpsuser',
        httpsPassword: 'httpspass'
      })
    ).toBeNull()
  })

  it('treats whitespace-only fields as blank', () => {
    expect(
      validateProxyAuthFields({ ...base, enabled: true, username: '   ', password: '   ' })
    ).toBe('Proxy username and password are required when proxy authentication is enabled.')
  })

  it('requires the shared pair when only an HTTPS override field was touched, even if already configured', () => {
    expect(
      validateProxyAuthFields({
        ...base,
        enabled: true,
        configured: true,
        httpsOverride: true,
        httpsUsername: 'httpsuser'
      })
    ).toBe('Both proxy username and password are required to update credentials.')
  })
})
