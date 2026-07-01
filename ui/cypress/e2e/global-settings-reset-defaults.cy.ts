describe('Global Settings — Reset to Defaults vs. active migrations', () => {
  const namespace = 'migration-system'

  const mockedTimezone = 'America/New_York'
  const mockedNtpServers = '0.pool.ntp.org, 1.pool.ntp.org'
  const mockedDeploymentName = 'custom-deployment'

  const settingsConfigMap = {
    apiVersion: 'v1',
    kind: 'ConfigMap',
    metadata: { name: 'vjailbreak-settings', namespace },
    data: {
      DEPLOYMENT_NAME: mockedDeploymentName,
      TIMEZONE: mockedTimezone,
      NTP_SERVERS: mockedNtpServers
    }
  }

  const runningMigration = {
    metadata: { name: 'mig-in-progress', namespace },
    status: { phase: 'CopyingBlocks' }
  }

  const mockCommon = () => {
    cy.intercept('GET', `**/namespaces/${namespace}/configmaps/vjailbreak-settings`, {
      body: settingsConfigMap
    }).as('getSettingsConfigMap')

    cy.intercept('GET', `**/namespaces/${namespace}/configmaps/pf9-env`, {
      statusCode: 404,
      body: {}
    }).as('getPf9Env')
  }

  it('preserves TIMEZONE and NTP_SERVERS on reset while a migration is running', () => {
    mockCommon()
    cy.intercept('GET', `**/namespaces/${namespace}/migrations`, {
      body: { items: [runningMigration] }
    }).as('getMigrations')

    cy.visit('/dashboard/global-settings')
    cy.wait(['@getSettingsConfigMap', '@getMigrations'])

    cy.get('input[name="DEPLOYMENT_NAME"]').should('have.value', mockedDeploymentName)
    cy.get('[data-testid="global-settings-field-TIMEZONE"] input').should(
      'not.have.value',
      ''
    )

    cy.get('[data-testid="global-settings-reset-defaults"]').click()

    // Unrelated field resets to its default.
    cy.get('input[name="DEPLOYMENT_NAME"]').should('have.value', 'vJailbreak')

    // TIMEZONE (general tab) stays locked to its current value.
    cy.get('[data-testid="global-settings-field-TIMEZONE"] input').should(
      'not.have.value',
      ''
    )

    // NTP_SERVERS (advanced tab) stays locked to its current value.
    cy.get('[data-testid="global-settings-tab-advanced"]').click()
    cy.get('input[name="NTP_SERVERS"]').should('have.value', mockedNtpServers)
  })

  it('resets TIMEZONE and NTP_SERVERS to defaults when no migration is running', () => {
    mockCommon()
    cy.intercept('GET', `**/namespaces/${namespace}/migrations`, {
      body: { items: [] }
    }).as('getMigrations')

    cy.visit('/dashboard/global-settings')
    cy.wait(['@getSettingsConfigMap', '@getMigrations'])

    cy.get('input[name="DEPLOYMENT_NAME"]').should('have.value', mockedDeploymentName)

    cy.get('[data-testid="global-settings-reset-defaults"]').click()

    cy.get('input[name="DEPLOYMENT_NAME"]').should('have.value', 'vJailbreak')
    cy.get('[data-testid="global-settings-field-TIMEZONE"] input').should('have.value', '')

    cy.get('[data-testid="global-settings-tab-advanced"]').click()
    cy.get('input[name="NTP_SERVERS"]').should('have.value', '')
  })
})
