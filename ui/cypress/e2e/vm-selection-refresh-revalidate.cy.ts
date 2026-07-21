/**
 * E2E tests for VM selection step refresh button revalidation flow.
 *
 * Covers: clicking refresh triggers VMware credential revalidation (same API as
 * credentials page), spinner + disabled state during revalidation, VM list
 * refetch on success, and re-enable on error.
 *
 * NOTE: useVmwareRevalidation polls GET .../vmwarecreds/{name} (single credential)
 * not the full list — all polling mocks target the by-name endpoint accordingly.
 */

const namespace = 'migration-system'
const vmwareCredName = 'test-vmware-creds'
const osCredName = 'test-os-creds'
const pcdClusterName = 'test-pcd-cluster'
const vmwareClusterName = 'test-cluster-1'

const vmwareCred = {
  apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
  kind: 'VmwareCreds',
  metadata: { name: vmwareCredName, namespace },
  spec: { datacenter: 'DC1', hostName: 'vcenter.test.com' },
  status: { vmwareValidationStatus: 'Succeeded' },
}

const vmwareCredRevalidating = {
  ...vmwareCred,
  status: { vmwareValidationStatus: 'Revalidating' },
}

const osCred = {
  apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
  kind: 'OpenstackCreds',
  metadata: { name: osCredName, namespace },
  spec: { projectName: 'test-project' },
  status: { openstackValidationStatus: 'Succeeded' },
}

const vmwareCluster = {
  apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
  kind: 'VmwareCluster',
  metadata: {
    name: vmwareClusterName,
    namespace,
    resourceVersion: '1',
    uid: 'uid-1',
    creationTimestamp: '2024-01-01T00:00:00Z',
    generation: 1,
    labels: { 'vjailbreak.k8s.pf9.io/vmwarecreds': vmwareCredName },
    annotations: { 'vjailbreak.k8s.pf9.io/datacenter': 'DC1' },
  },
  spec: { name: 'Cluster1', hosts: [] },
}

const pcdCluster = {
  apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
  kind: 'PCDCluster',
  metadata: {
    name: pcdClusterName,
    namespace,
    resourceVersion: '1',
    uid: 'uid-2',
    creationTimestamp: '2024-01-01T00:00:00Z',
    generation: 1,
    labels: { 'vjailbreak.k8s.pf9.io/openstackcreds': osCredName },
  },
  spec: { clusterName: 'PCD Cluster 1', hosts: [] },
}

const vmList = {
  apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
  kind: 'VmwareMachineList',
  metadata: { continue: '', resourceVersion: '1' },
  items: [
    {
      apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
      kind: 'VmwareMachine',
      metadata: { name: 'vm-1', namespace },
      spec: { vmName: 'VM-1' },
      status: {},
    },
  ],
}

/** Set up all the baseline API mocks required to open the migration form and
 *  reach the VM selection step with both creds validated. */
function setupBaseMocks() {
  // List endpoint — used by credential selectors and CredentialsTable
  cy.intercept('GET', `**/namespaces/${namespace}/vmwarecreds`, {
    body: { items: [vmwareCred] },
  }).as('getVmwareCreds')

  // Single-credential endpoint — used by useVmwareCredentialQuery for polling
  cy.intercept('GET', `**/namespaces/${namespace}/vmwarecreds/${vmwareCredName}`, {
    body: vmwareCred,
  }).as('getVmwareCredByName')

  cy.intercept('GET', `**/namespaces/${namespace}/openstackcreds`, {
    body: { items: [osCred] },
  }).as('getOpenstackCreds')

  cy.intercept('GET', `**/namespaces/${namespace}/openstackcreds/${osCredName}`, {
    body: osCred,
  }).as('getOsCredByName')

  cy.intercept('GET', `**/namespaces/${namespace}/vmwareclusters**`, {
    body: {
      apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
      kind: 'VmwareClusterList',
      metadata: { continue: '', resourceVersion: '1' },
      items: [vmwareCluster],
    },
  }).as('getVmwareClusters')

  cy.intercept('GET', `**/namespaces/${namespace}/pcdclusters`, {
    body: {
      apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
      kind: 'PCDClusterList',
      metadata: { continue: '', resourceVersion: '1' },
      items: [pcdCluster],
    },
  }).as('getPcdClusters')

  cy.intercept('GET', `**/namespaces/${namespace}/vmwaremachines**`, {
    body: vmList,
  }).as('getVmwareMachines')

  cy.intercept('GET', `**/namespaces/${namespace}/migrations**`, {
    body: { items: [] },
  }).as('getMigrations')
}

/** Navigate to the migrations page, open the form, and select both clusters so
 *  the VM selection step is fully enabled. */
function openFormAndSelectClusters() {
  cy.visit('/dashboard/migrations')

  cy.get('[data-testid="start-migration-button"]').click()
  cy.get('[data-testid="migration-form-drawer"]').should('be.visible')

  // Select VMware cluster — opens MUI Select listbox
  cy.get('[data-testid="vmware-cluster-dropdown"]').click()
  cy.get('[role="listbox"]').should('be.visible')
  cy.get('[role="option"]').first().click()

  // Select PCD cluster
  cy.get('[data-testid="pcd-cluster-dropdown"]').click()
  cy.get('[role="listbox"]').should('be.visible')
  cy.get('[role="option"]').first().click()

  // Scroll to and verify VM selection step is visible
  cy.get('[data-testid="migration-form-step-select-vms"]').scrollIntoView()
  cy.get('[data-testid="vms-datagrid"]').should('be.visible')
}

// ---------------------------------------------------------------------------

describe('VM Selection Step — Refresh & Revalidate', () => {
  beforeEach(() => {
    setupBaseMocks()
  })

  it('clicking refresh calls POST revalidate_credentials with VmwareCreds kind', () => {
    cy.intercept('POST', '**/revalidate_credentials', {
      statusCode: 200,
      body: {},
    }).as('revalidate')

    openFormAndSelectClusters()

    cy.get('[data-testid="vm-list-refresh-button"]').should('not.be.disabled').click()

    cy.wait('@revalidate').then((interception) => {
      expect(interception.request.body).to.include({
        name: vmwareCredName,
        namespace,
        kind: 'VmwareCreds',
      })
    })
  })

  it('refresh button spins and is disabled while revalidation is in progress', () => {
    cy.intercept('POST', '**/revalidate_credentials', {
      statusCode: 200,
      body: {},
    }).as('revalidate')

    // Override the single-credential endpoint to return Revalidating status.
    // useVmwareRevalidation polls GET .../vmwarecreds/{name} — NOT the list.
    cy.intercept('GET', `**/namespaces/${namespace}/vmwarecreds/${vmwareCredName}`, {
      body: vmwareCredRevalidating,
    }).as('pollRevalidating')

    openFormAndSelectClusters()

    cy.get('[data-testid="vm-list-refresh-button"]').click()
    cy.wait('@revalidate')

    // Button should be disabled while backend status is Revalidating
    cy.get('[data-testid="vm-list-refresh-button"]').should('be.disabled')

    // The spinner icon animates — MUI applies inline animation style
    cy.get('[data-testid="vm-list-refresh-button"] svg').should(
      'have.css',
      'animation-name'
    )
  })

  it('VM list is refetched after revalidation completes successfully', () => {
    cy.intercept('POST', '**/revalidate_credentials', {
      statusCode: 200,
      body: {},
    }).as('revalidate')

    // The single-credential endpoint (from setupBaseMocks getVmwareCredByName)
    // already returns Succeeded — the hook sees completion when dataUpdatedAt
    // advances after invalidateQueries triggers a fresh fetch.

    // Track vmwaremachines fetches; reply with vmList so the component renders
    let machinesFetchCount = 0
    cy.intercept('GET', `**/namespaces/${namespace}/vmwaremachines**`, (req) => {
      machinesFetchCount++
      req.reply({ body: vmList })
    }).as('vmMachinesRefetch')

    openFormAndSelectClusters()

    cy.get('[data-testid="vm-list-refresh-button"]').click()
    cy.wait('@revalidate')

    // After revalidation completes the hook calls onRevalidationComplete which
    // triggers a VM list refresh — wait for the second fetch
    cy.wait('@vmMachinesRefetch')
    cy.wrap(null).should(() => {
      expect(machinesFetchCount).to.be.greaterThan(1)
    })
  })

  it('refresh button re-enables and spinner stops after revalidation API error', () => {
    cy.intercept('POST', '**/revalidate_credentials', {
      statusCode: 500,
      body: { error: 'Internal server error' },
    }).as('revalidateError')

    openFormAndSelectClusters()

    cy.get('[data-testid="vm-list-refresh-button"]').should('not.be.disabled').click()
    cy.wait('@revalidateError')

    // Button should re-enable after error
    cy.get('[data-testid="vm-list-refresh-button"]').should('not.be.disabled')

    // No animation on the icon after error
    cy.get('[data-testid="vm-list-refresh-button"] svg').should(
      'not.have.css',
      'animation-name',
      'auto'
    )
  })
})
