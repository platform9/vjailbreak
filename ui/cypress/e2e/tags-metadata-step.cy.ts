/**
 * E2E tests for the Tags & Metadata step (step 5) of the migration form.
 *
 * Covers: step placement between Security & Placement and Migration Options,
 * the preserve-source-tags toggle with its preview accordion (fed by
 * VMwareMachine tags/customAttributes), the custom metadata key-value editor,
 * and the section nav marking the step complete once configured.
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

// VM carrying vSphere tags and custom attributes so the preview has content
const vmList = {
  apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
  kind: 'VmwareMachineList',
  metadata: { continue: '', resourceVersion: '1' },
  items: [
    {
      apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
      kind: 'VmwareMachine',
      metadata: {
        name: 'vm-1',
        namespace,
        labels: { 'vjailbreak.k8s.pf9.io/vmware-cluster': vmwareClusterName },
      },
      spec: {
        vms: {
          name: 'VM-1',
          vmid: 'vm-101',
          cpu: 2,
          memory: 4096,
          vmState: 'running',
          osFamily: 'linuxGuest',
          datastores: ['ds-1'],
          disks: ['Hard disk 1'],
          networks: ['net-1'],
          tags: { env: 'production' },
          customAttributes: { Owner: 'alice@corp.com' },
        },
      },
      status: { powerState: 'running', migrated: false },
    },
  ],
}

function setupBaseMocks() {
  cy.intercept('GET', `**/namespaces/${namespace}/vmwarecreds`, {
    body: { items: [vmwareCred] },
  }).as('getVmwareCreds')

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

  cy.intercept('GET', `**/namespaces/${namespace}/rdmdisks**`, {
    body: { items: [] },
  }).as('getRdmDisks')
}

function openFormAndSelectClusters() {
  cy.visit('/dashboard/migrations')

  cy.get('[data-testid="start-migration-button"]').click()
  cy.get('[data-testid="migration-form-drawer"]').should('be.visible')

  cy.get('[data-testid="vmware-cluster-dropdown"]').click()
  cy.get('[role="listbox"]').should('be.visible')
  cy.get('[role="option"]').first().click()

  cy.get('[data-testid="pcd-cluster-dropdown"]').click()
  cy.get('[role="listbox"]').should('be.visible')
  cy.get('[role="option"]').first().click()
}

function scrollToTagsStep() {
  cy.get('[data-testid="migration-form-step-tags-metadata"]').scrollIntoView()
  cy.get('[data-testid="migration-form-tags-metadata-card"]').should('be.visible')
}

// ---------------------------------------------------------------------------

describe('Migration Form — Tags & Metadata Step', () => {
  beforeEach(() => {
    setupBaseMocks()
  })

  it('renders as its own step between Security & Placement and Migration Options', () => {
    openFormAndSelectClusters()
    scrollToTagsStep()

    // Section nav lists it after security and before options
    cy.get('[data-testid="section-nav-item-security"]').should('exist')
    cy.get('[data-testid="section-nav-item-tags-metadata"]').should('exist')
    cy.get('[data-testid="section-nav-item-options"]').should('exist')

    // DOM order: security step above tags step, tags step above options step
    cy.get('[data-testid="migration-form-step-security"]').then(($security) => {
      cy.get('[data-testid="migration-form-step-tags-metadata"]').then(($tags) => {
        cy.get('[data-testid="migration-form-step-options"]').then(($options) => {
          expect($security[0].compareDocumentPosition($tags[0]) & 4, 'security before tags').to.be.greaterThan(0)
          expect($tags[0].compareDocumentPosition($options[0]) & 4, 'tags before options').to.be.greaterThan(0)
        })
      })
    })
  })

  it('toggle reveals the preview accordion and hides it when turned off', () => {
    openFormAndSelectClusters()
    scrollToTagsStep()

    // Off by default — no preview
    cy.get('[data-testid="source-tags-preview"]').should('not.exist')

    cy.get('[data-testid="preserve-source-tags-toggle"]').click()
    cy.get('[data-testid="source-tags-preview"]').should('be.visible')

    cy.get('[data-testid="preserve-source-tags-toggle"]').click()
    cy.get('[data-testid="source-tags-preview"]').should('not.exist')
  })

  it('preview shows the selected VM tags and custom attributes', () => {
    openFormAndSelectClusters()

    // Select the VM so the preview has a row with tag data
    cy.get('[data-testid="migration-form-step-select-vms"]').scrollIntoView()
    cy.get('[data-testid="vms-datagrid"]').should('be.visible')
    cy.get('[data-testid="vms-datagrid"] .MuiDataGrid-row')
      .first()
      .find('input[type="checkbox"]')
      .check({ force: true })

    scrollToTagsStep()
    cy.get('[data-testid="preserve-source-tags-toggle"]').click()

    // Expand the accordion and verify chips
    cy.get('[data-testid="source-tags-preview"]').click()
    cy.get('[data-testid="source-tags-preview"]').within(() => {
      cy.contains('VM-1')
      cy.contains('env: production')
      cy.contains('Owner: alice@corp.com')
    })
  })

  it('custom metadata rows can be added, edited, and removed', () => {
    openFormAndSelectClusters()
    scrollToTagsStep()

    cy.get('[data-testid="custom-metadata-row-0"]').should('not.exist')

    cy.get('[data-testid="add-custom-metadata"]').click()
    cy.get('[data-testid="custom-metadata-row-0"]').should('be.visible')

    cy.get('[data-testid="custom-metadata-row-0"] input').eq(0).type('migrated_by')
    cy.get('[data-testid="custom-metadata-row-0"] input').eq(1).type('vjailbreak')

    cy.get('[data-testid="add-custom-metadata"]').click()
    cy.get('[data-testid="custom-metadata-row-1"]').should('be.visible')

    // Remove the second row
    cy.get('[data-testid="custom-metadata-row-1"] button[aria-label="Remove metadata row"]').click()
    cy.get('[data-testid="custom-metadata-row-1"]').should('not.exist')

    // First row keeps its values
    cy.get('[data-testid="custom-metadata-row-0"] input').eq(0).should('have.value', 'migrated_by')
    cy.get('[data-testid="custom-metadata-row-0"] input').eq(1).should('have.value', 'vjailbreak')
  })

  it('section nav marks the step complete once the toggle is enabled', () => {
    openFormAndSelectClusters()
    scrollToTagsStep()

    // Incomplete before any interaction: chip shows the step number, no check icon
    cy.get('[data-testid="section-nav-item-tags-metadata"] svg[data-testid="CheckIcon"]').should(
      'not.exist'
    )

    cy.get('[data-testid="preserve-source-tags-toggle"]').click()

    cy.get('[data-testid="section-nav-item-tags-metadata"] svg[data-testid="CheckIcon"]').should(
      'exist'
    )
  })
})
