describe('Global Settings — Host Entries tab', () => {
  const namespace = 'migration-system'

  const existingEntries = [{ ip: '10.0.0.5', hostnames: ['esxi01.corp.local', 'esxi01'] }]

  const settingsConfigMap = {
    apiVersion: 'v1',
    kind: 'ConfigMap',
    metadata: { name: 'vjailbreak-settings', namespace },
    data: {
      AGENT_HOST_ENTRIES: JSON.stringify(existingEntries)
    }
  }

  const mockCommon = () => {
    cy.intercept('GET', `**/namespaces/${namespace}/configmaps/vjailbreak-settings`, {
      body: settingsConfigMap
    }).as('getSettingsConfigMap')

    cy.intercept('GET', `**/namespaces/${namespace}/configmaps/pf9-env`, {
      statusCode: 404,
      body: {}
    }).as('getPf9Env')

    cy.intercept('GET', `**/namespaces/${namespace}/migrations`, {
      body: { items: [] }
    }).as('getMigrations')
  }

  const openHostEntriesTab = () => {
    cy.visit('/dashboard/global-settings')
    cy.wait(['@getSettingsConfigMap', '@getMigrations'])
    cy.get('[data-testid="global-settings-tab-hosts"]').click()
  }

  it('shows a single description and the Add Entry button inside the table header', () => {
    mockCommon()
    openHostEntriesTab()

    // Description rendered exactly once (page-level helper, no duplicate inside the tab).
    cy.get('p:contains("Custom hostname-to-IP mappings")').should('have.length', 1)
    cy.contains('p', 'Custom hostname-to-IP mappings').should(
      'contain.text',
      'Supports ESXi hosts, vCenter, PCD, and OpenStack endpoints'
    )
    // Table itself carries no description text.
    cy.get('table').should('not.contain.text', 'Custom hostname-to-IP mappings')

    // Add Entry button lives in the table header row and uses the contained variant.
    cy.get('[data-testid="host-entries-add-btn"]')
      .should('be.visible')
      .and('have.class', 'MuiButton-contained')
      .closest('thead')
      .should('exist')
  })

  it('adds a new entry via the header button and saves it to AGENT_HOST_ENTRIES', () => {
    mockCommon()
    cy.intercept('PUT', `**/namespaces/${namespace}/configmaps/vjailbreak-settings`, (req) => {
      req.reply({ body: req.body })
    }).as('putSettingsConfigMap')

    openHostEntriesTab()

    // Existing entry from the ConfigMap is listed.
    cy.contains('td', '10.0.0.5').should('be.visible')
    cy.contains('td', 'esxi01.corp.local, esxi01').should('be.visible')

    cy.get('[data-testid="host-entries-add-btn"]').click()
    // Button disabled while the inline add row is open.
    cy.get('[data-testid="host-entries-add-btn"]').should('be.disabled')

    cy.get('[data-testid="host-entry-new-ip"]').type('192.168.1.100')
    cy.get('[data-testid="host-entry-new-hostnames"]').type('vcenter.corp.local, vcenter')
    cy.get('[data-testid="host-entry-add-confirm"]').click()

    cy.contains('td', '192.168.1.100').should('be.visible')
    cy.get('[data-testid="host-entries-add-btn"]').should('not.be.disabled')

    cy.get('[data-testid="global-settings-save"]').click()

    cy.wait('@putSettingsConfigMap').then(({ request }) => {
      const saved = JSON.parse(request.body.data.AGENT_HOST_ENTRIES)
      expect(saved).to.deep.equal([
        { ip: '10.0.0.5', hostnames: ['esxi01.corp.local', 'esxi01'] },
        { ip: '192.168.1.100', hostnames: ['vcenter.corp.local', 'vcenter'] }
      ])
    })
  })

  it('validates input and supports edit and delete from the actions column', () => {
    mockCommon()
    openHostEntriesTab()

    // Invalid IP is rejected with an inline error.
    cy.get('[data-testid="host-entries-add-btn"]').click()
    cy.get('[data-testid="host-entry-new-ip"]').type('not-an-ip')
    cy.get('[data-testid="host-entry-new-hostnames"]').type('host1')
    cy.get('[data-testid="host-entry-add-confirm"]').click()
    cy.contains('Invalid IP address').should('be.visible')
    cy.get('[data-testid="host-entry-add-cancel"]').click()

    // Edit the existing entry.
    cy.get('[data-testid="host-entry-edit-0"]').click()
    cy.get('[data-testid="host-entry-edit-ip"]').clear().type('10.0.0.6')
    cy.get('[data-testid="host-entry-edit-save"]').click()
    cy.contains('td', '10.0.0.6').should('be.visible')

    // Delete it — empty state returns.
    cy.get('[data-testid="host-entry-delete-0"]').click()
    cy.contains('No host entries configured').should('be.visible')
  })
})
