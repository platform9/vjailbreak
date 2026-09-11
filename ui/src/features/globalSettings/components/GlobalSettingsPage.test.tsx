import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { vi } from 'vitest'
import GlobalSettingsPage from './GlobalSettingsPage'
import * as settingsApi from 'src/api/settings/settings'
import * as helpersApi from 'src/api/helpers'
import * as aiAnalysis from 'src/api/ai/aiAnalysis'
import * as proxyCredentials from 'src/api/proxy/proxyCredentials'

vi.mock('src/api/settings/settings', () => ({
  VERSION_CONFIG_MAP_NAME: 'vjailbreak-settings',
  VERSION_NAMESPACE: 'migration-system',
  getSettingsConfigMap: vi.fn(),
  updateSettingsConfigMap: vi.fn(),
  applyTimeSettings: vi.fn()
}))

vi.mock('src/api/helpers', () => ({
  getPf9EnvConfig: vi.fn(),
  injectEnvVariables: vi.fn()
}))

vi.mock('src/api/ai/aiAnalysis', () => ({
  getAIKeyStatus: vi.fn(),
  saveAIKey: vi.fn()
}))

vi.mock('src/api/proxy/proxyCredentials', () => ({
  getProxyCredsStatus: vi.fn(),
  saveProxyCreds: vi.fn(),
  deleteProxyCreds: vi.fn()
}))

vi.mock('src/api/vddk', () => ({
  uploadVddkFile: vi.fn()
}))

vi.mock('src/hooks/api/useVddkStatusQuery', () => ({
  useVddkStatusQuery: () => ({
    data: { uploaded: false, path: '', version: '' },
    refetch: vi.fn()
  })
}))

vi.mock('src/hooks/api/useMigrationsQuery', () => ({
  useMigrationsQuery: () => ({ data: [] })
}))

const mockedSettings = settingsApi as unknown as {
  getSettingsConfigMap: ReturnType<typeof vi.fn>
  updateSettingsConfigMap: ReturnType<typeof vi.fn>
  applyTimeSettings: ReturnType<typeof vi.fn>
}
const mockedHelpers = helpersApi as unknown as {
  getPf9EnvConfig: ReturnType<typeof vi.fn>
  injectEnvVariables: ReturnType<typeof vi.fn>
}
const mockedAi = aiAnalysis as unknown as {
  getAIKeyStatus: ReturnType<typeof vi.fn>
  saveAIKey: ReturnType<typeof vi.fn>
}
const mockedProxyCreds = proxyCredentials as unknown as {
  getProxyCredsStatus: ReturnType<typeof vi.fn>
  saveProxyCreds: ReturnType<typeof vi.fn>
  deleteProxyCreds: ReturnType<typeof vi.fn>
}

const PROXY_ENABLED_ENV = {
  http_proxy: 'http://proxy.local:3128',
  https_proxy: 'http://proxy.local:3128',
  no_proxy: 'localhost,127.0.0.1'
}

const setupDefaultMocks = () => {
  mockedSettings.getSettingsConfigMap.mockResolvedValue({ data: {} })
  mockedSettings.updateSettingsConfigMap.mockResolvedValue({})
  mockedSettings.applyTimeSettings.mockResolvedValue(undefined)
  mockedHelpers.getPf9EnvConfig.mockResolvedValue({ data: { ...PROXY_ENABLED_ENV } })
  mockedHelpers.injectEnvVariables.mockResolvedValue({ success: true, message: 'ok' })
  mockedAi.getAIKeyStatus.mockResolvedValue({ configured: false })
  mockedProxyCreds.getProxyCredsStatus.mockResolvedValue({ configured: false, https_override: false })
  mockedProxyCreds.saveProxyCreds.mockResolvedValue({ configured: true, https_override: false })
  mockedProxyCreds.deleteProxyCreds.mockResolvedValue({ configured: false, https_override: false })
}

const renderPage = () => render(
  <MemoryRouter>
    <GlobalSettingsPage />
  </MemoryRouter>
)

const goToNetworkTab = async () => {
  const tab = await screen.findByTestId('global-settings-tab-network')
  fireEvent.click(tab)
  await screen.findByTestId('global-settings-toggle-PROXY_AUTH_ENABLED')
}

const clickToggle = (testId: string) => {
  const el = screen.getByTestId(testId)
  const input = el.tagName === 'INPUT' ? el : el.querySelector('input')
  fireEvent.click((input ?? el) as Element)
}

const fillTextField = (label: string, value: string) => {
  const field = screen.getByLabelText(label)
  fireEvent.change(field, { target: { value } })
}

describe('GlobalSettingsPage - proxy authentication', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setupDefaultMocks()
  })

  it('shows the auth toggle on the network tab, off by default', async () => {
    renderPage()
    await goToNetworkTab()

    expect(screen.queryByLabelText('Proxy Username')).not.toBeInTheDocument()
  })

  it('reveals shared username/password fields when the auth toggle is turned on', async () => {
    renderPage()
    await goToNetworkTab()

    clickToggle('global-settings-toggle-PROXY_AUTH_ENABLED')

    expect(await screen.findByLabelText('Proxy Username')).toBeInTheDocument()
    expect(screen.getByLabelText('Proxy Password')).toBeInTheDocument()
    expect(screen.queryByLabelText('HTTPS Proxy Username')).not.toBeInTheDocument()
  })

  it('reveals HTTPS-specific fields only when the override toggle is on', async () => {
    renderPage()
    await goToNetworkTab()
    clickToggle('global-settings-toggle-PROXY_AUTH_ENABLED')
    await screen.findByLabelText('Proxy Username')

    clickToggle('global-settings-toggle-PROXY_AUTH_HTTPS_OVERRIDE')

    expect(await screen.findByLabelText('HTTPS Proxy Username')).toBeInTheDocument()
    expect(screen.getByLabelText('HTTPS Proxy Password')).toBeInTheDocument()
  })

  it('shows the configured status message from the status endpoint on load', async () => {
    mockedProxyCreds.getProxyCredsStatus.mockResolvedValue({ configured: true, https_override: false })
    renderPage()
    await goToNetworkTab()

    expect(await screen.findByText('Proxy credentials are configured.')).toBeInTheDocument()
  })

  it('shows the split-credentials status message when https_override is true', async () => {
    mockedProxyCreds.getProxyCredsStatus.mockResolvedValue({ configured: true, https_override: true })
    renderPage()
    await goToNetworkTab()

    expect(
      await screen.findByText('HTTP and HTTPS proxy credentials are configured separately.')
    ).toBeInTheDocument()
  })

  it('blocks save with a validation error when auth is enabled but no credentials are entered', async () => {
    renderPage()
    await goToNetworkTab()
    clickToggle('global-settings-toggle-PROXY_AUTH_ENABLED')
    await screen.findByLabelText('Proxy Username')

    fireEvent.click(screen.getByTestId('global-settings-save'))

    expect(
      await screen.findByText(
        'Proxy username and password are required when proxy authentication is enabled.'
      )
    ).toBeInTheDocument()
    expect(mockedProxyCreds.saveProxyCreds).not.toHaveBeenCalled()
    expect(mockedHelpers.injectEnvVariables).not.toHaveBeenCalled()
  })

  it('blocks save when HTTPS override is on but its fields are left empty', async () => {
    renderPage()
    await goToNetworkTab()
    clickToggle('global-settings-toggle-PROXY_AUTH_ENABLED')
    await screen.findByLabelText('Proxy Username')
    fillTextField('Proxy Username', 'proxyuser')
    fillTextField('Proxy Password', 'proxypass')
    clickToggle('global-settings-toggle-PROXY_AUTH_HTTPS_OVERRIDE')
    await screen.findByLabelText('HTTPS Proxy Username')

    fireEvent.click(screen.getByTestId('global-settings-save'))

    expect(
      await screen.findByText(
        'Both HTTPS proxy username and password are required when using different HTTPS credentials.'
      )
    ).toBeInTheDocument()
    expect(mockedProxyCreds.saveProxyCreds).not.toHaveBeenCalled()
  })

  it('saves shared credentials before injecting env variables, in that order', async () => {
    renderPage()
    await goToNetworkTab()
    clickToggle('global-settings-toggle-PROXY_AUTH_ENABLED')
    await screen.findByLabelText('Proxy Username')
    fillTextField('Proxy Username', 'proxyuser')
    fillTextField('Proxy Password', 'proxypass')

    fireEvent.click(screen.getByTestId('global-settings-save'))

    await waitFor(() => expect(mockedProxyCreds.saveProxyCreds).toHaveBeenCalledTimes(1))
    expect(mockedProxyCreds.saveProxyCreds).toHaveBeenCalledWith({
      username: 'proxyuser',
      password: 'proxypass',
      https_override: false,
      https_username: undefined,
      https_password: undefined
    })
    await waitFor(() => expect(mockedHelpers.injectEnvVariables).toHaveBeenCalledTimes(1))

    const credsCallOrder = mockedProxyCreds.saveProxyCreds.mock.invocationCallOrder[0]
    const injectCallOrder = mockedHelpers.injectEnvVariables.mock.invocationCallOrder[0]
    expect(credsCallOrder).toBeLessThan(injectCallOrder)
  })

  it('sends distinct HTTPS credentials when the override is used', async () => {
    renderPage()
    await goToNetworkTab()
    clickToggle('global-settings-toggle-PROXY_AUTH_ENABLED')
    await screen.findByLabelText('Proxy Username')
    fillTextField('Proxy Username', 'proxyuser')
    fillTextField('Proxy Password', 'proxypass')
    clickToggle('global-settings-toggle-PROXY_AUTH_HTTPS_OVERRIDE')
    await screen.findByLabelText('HTTPS Proxy Username')
    fillTextField('HTTPS Proxy Username', 'httpsuser')
    fillTextField('HTTPS Proxy Password', 'httpspass')

    fireEvent.click(screen.getByTestId('global-settings-save'))

    await waitFor(() => expect(mockedProxyCreds.saveProxyCreds).toHaveBeenCalledTimes(1))
    expect(mockedProxyCreds.saveProxyCreds).toHaveBeenCalledWith({
      username: 'proxyuser',
      password: 'proxypass',
      https_override: true,
      https_username: 'httpsuser',
      https_password: 'httpspass'
    })
  })

  it('does not call saveProxyCreds again on save when already configured and fields are left blank', async () => {
    mockedProxyCreds.getProxyCredsStatus.mockResolvedValue({ configured: true, https_override: false })
    renderPage()
    await goToNetworkTab()
    await screen.findByText('Proxy credentials are configured.')

    fireEvent.click(screen.getByTestId('global-settings-save'))

    await waitFor(() => expect(mockedHelpers.injectEnvVariables).toHaveBeenCalledTimes(1))
    expect(mockedProxyCreds.saveProxyCreds).not.toHaveBeenCalled()
    expect(mockedProxyCreds.deleteProxyCreds).not.toHaveBeenCalled()
  })

  it('clears the secret when auth is turned off after being configured', async () => {
    mockedProxyCreds.getProxyCredsStatus.mockResolvedValue({ configured: true, https_override: false })
    renderPage()
    await goToNetworkTab()
    await screen.findByText('Proxy credentials are configured.')

    clickToggle('global-settings-toggle-PROXY_AUTH_ENABLED')
    await waitFor(() => expect(screen.queryByLabelText('Proxy Username')).not.toBeInTheDocument())

    fireEvent.click(screen.getByTestId('global-settings-save'))

    await waitFor(() => expect(mockedProxyCreds.deleteProxyCreds).toHaveBeenCalledTimes(1))
    await waitFor(() => expect(mockedHelpers.injectEnvVariables).toHaveBeenCalledTimes(1))
    expect(mockedProxyCreds.saveProxyCreds).not.toHaveBeenCalled()

    const deleteCallOrder = mockedProxyCreds.deleteProxyCreds.mock.invocationCallOrder[0]
    const injectCallOrder = mockedHelpers.injectEnvVariables.mock.invocationCallOrder[0]
    expect(deleteCallOrder).toBeLessThan(injectCallOrder)
  })

  it('does not call injectEnvVariables when saving credentials fails', async () => {
    mockedProxyCreds.saveProxyCreds.mockRejectedValue(new Error('secret write failed'))
    renderPage()
    await goToNetworkTab()
    clickToggle('global-settings-toggle-PROXY_AUTH_ENABLED')
    await screen.findByLabelText('Proxy Username')
    fillTextField('Proxy Username', 'proxyuser')
    fillTextField('Proxy Password', 'proxypass')

    fireEvent.click(screen.getByTestId('global-settings-save'))

    await waitFor(() => expect(mockedProxyCreds.saveProxyCreds).toHaveBeenCalledTimes(1))
    expect(
      await screen.findByText(
        'Settings saved, but saving proxy credentials failed. Proxy environment variables were not updated — please retry.'
      )
    ).toBeInTheDocument()
    expect(mockedHelpers.injectEnvVariables).not.toHaveBeenCalled()
  })
})

const goToIntervalsTab = async () => {
  const tab = await screen.findByTestId('global-settings-tab-retry')
  fireEvent.click(tab)
  await screen.findByLabelText('Log Retention (hours)')
}

describe('GlobalSettingsPage - log retention', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setupDefaultMocks()
  })

  it('blocks save when log retention is below the 24-hour minimum', async () => {
    renderPage()
    await goToIntervalsTab()
    fillTextField('Log Retention (hours)', '12')

    fireEvent.click(screen.getByTestId('global-settings-save'))

    expect(
      await screen.findByText('Enter a whole number of hours, minimum 24.')
    ).toBeInTheDocument()
    expect(mockedSettings.updateSettingsConfigMap).not.toHaveBeenCalled()
  })

  it('allows save at the 24-hour minimum', async () => {
    renderPage()
    await goToIntervalsTab()
    fillTextField('Log Retention (hours)', '24')

    fireEvent.click(screen.getByTestId('global-settings-save'))

    await waitFor(() => expect(mockedSettings.updateSettingsConfigMap).toHaveBeenCalled())
  })

  it('allows save above the 24-hour minimum', async () => {
    renderPage()
    await goToIntervalsTab()
    fillTextField('Log Retention (hours)', '72')

    fireEvent.click(screen.getByTestId('global-settings-save'))

    await waitFor(() => expect(mockedSettings.updateSettingsConfigMap).toHaveBeenCalled())
  })
})
