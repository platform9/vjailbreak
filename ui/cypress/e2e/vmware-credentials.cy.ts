describe('ScaleUp Drawer — Server Group', () => {
  const namespace = 'migration-system'
  const credName = 'test-pcd-creds'

  const masterNode = {
    apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
    kind: 'VjailbreakNode',
    metadata: { name: 'vjailbreak-master', namespace },
    spec: {
      nodeRole: 'master',
      openstackImageID: 'img-abc123',
      openstackFlavorID: 'flavor-001',
      openstackCreds: { kind: 'openstackcreds', name: credName, namespace }
    },
    status: { phase: 'NodeReady', vmIP: '10.0.0.1', openstackUUID: 'uuid-master' }
  }

  const validatedCred = {
    apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
    kind: 'OpenstackCreds',
    metadata: { name: credName, namespace },
    spec: {
      flavors: [
        { id: 'flavor-001', name: 'm1.xlarge', vcpus: 8, ram: 16384, disk: 80 }
      ]
    },
    status: {
      openstackValidationStatus: 'Succeeded',
      openstack: {
        volumeTypes: ['ceph-ssd'],
        securityGroups: [{ id: 'sg-1', name: 'default', requiresIdDisplay: false }],
        serverGroups: [
          { id: 'srv-grp-001', name: 'anti-affinity-agents', policy: 'anti-affinity', members: 0 },
          { id: 'srv-grp-002', name: 'soft-anti-affinity-agents', policy: 'soft-anti-affinity', members: 1 }
        ]
      }
    }
  }

  beforeEach(() => {
    cy.intercept('GET', `**/namespaces/${namespace}/vjailbreaknodes`, {
      body: { items: [masterNode] }
    }).as('getNodes')

    cy.intercept('GET', `**/namespaces/${namespace}/openstackcreds`, {
      body: { items: [validatedCred] }
    }).as('getOpenstackCreds')

    cy.intercept('GET', `**/namespaces/${namespace}/openstackcreds/${credName}`, {
      body: validatedCred
    }).as('getCredDetail')

    cy.visit('/dashboard/agents')
    cy.wait('@getNodes')
  })

  it('renders server group autocomplete after selecting validated creds', () => {
    cy.get('[data-testid="scaleup-open-button"]').click()
    cy.get('[data-testid="scaleup-form"]').should('be.visible')

    // Select OpenStack credential
    cy.wait('@getOpenstackCreds')
    cy.contains(credName).click()
    cy.wait('@getCredDetail')

    // Server group field should appear and be enabled
    cy.get('[data-testid="scaleup-server-group"]').should('exist')
    cy.get('[data-testid="scaleup-server-group"] input').should('not.be.disabled')
  })

  it('submits VjailbreakNode with openstackServerGroup when server group selected', () => {
    cy.intercept('POST', `**/namespaces/${namespace}/vjailbreaknodes`, (req) => {
      req.reply({ statusCode: 201, body: { ...req.body, status: {} } })
    }).as('createNode')

    cy.get('[data-testid="scaleup-open-button"]').click()
    cy.get('[data-testid="scaleup-form"]').should('be.visible')

    cy.wait('@getOpenstackCreds')
    cy.contains(credName).click()
    cy.wait('@getCredDetail')

    // Select a server group
    cy.get('[data-testid="scaleup-server-group"] input').click()
    cy.contains('anti-affinity-agents (anti-affinity)').click()

    // Select flavor
    cy.get('[data-testid="scaleup-submit"]').should('be.disabled')
    // flavor is required — pick via the flavor select
    cy.get('input[name="flavor"]').click({ force: true })
    cy.contains('m1.xlarge').click()

    cy.get('[data-testid="scaleup-submit"]').click()

    cy.wait('@createNode').its('request.body.spec').should((spec) => {
      expect(spec).to.have.property('openstackServerGroup', 'srv-grp-001')
    })
  })

  it('submits VjailbreakNode without openstackServerGroup when no server group selected', () => {
    cy.intercept('POST', `**/namespaces/${namespace}/vjailbreaknodes`, (req) => {
      req.reply({ statusCode: 201, body: { ...req.body, status: {} } })
    }).as('createNode')

    cy.get('[data-testid="scaleup-open-button"]').click()

    cy.wait('@getOpenstackCreds')
    cy.contains(credName).click()
    cy.wait('@getCredDetail')

    // Leave server group empty, select flavor
    cy.get('input[name="flavor"]').click({ force: true })
    cy.contains('m1.xlarge').click()

    cy.get('[data-testid="scaleup-submit"]').click()

    cy.wait('@createNode').its('request.body.spec').should((spec) => {
      expect(spec).to.not.have.property('openstackServerGroup')
    })
  })
})

describe('Add VMware Credentials', () => {
  const namespace = 'migration-system'

  beforeEach(() => {
    // Intercept initial credentials fetch - using wildcard to match potential /dev-api prefix
    cy.intercept('GET', `**/namespaces/${namespace}/vmwarecreds`, {
      body: { items: [] },
    }).as('getVmwareCreds')

    cy.intercept('GET', `**/namespaces/${namespace}/openstackcreds`, {
      body: { items: [] },
    }).as('getOpenstackCreds')

    cy.visit('/dashboard/credentials')
    cy.wait(['@getVmwareCreds', '@getOpenstackCreds'])
  })

  it('should successfully add VMware credentials', () => {
    const credName = 'test-vcenter-creds'
    const vCenterHost = 'vcenter.example.com'
    const datacenter = 'Datacenter-1'
    const username = 'admin@vsphere.local'
    const password = 'securepassword'

    // Open Drawer
    cy.contains('button', 'Add VMware Credentials').click()
    cy.get('[data-testid=vmware-cred-form]').should('be.visible')

    // Fill Form
    cy.get('input[name="credentialName"]').type(credName)
    cy.get('input[name="vcenterHost"]').type(vCenterHost)
    cy.get('input[name="datacenter"]').type(datacenter)
    cy.get('input[name="username"]').type(username)
    cy.get('input[name="password"]').type(password)
    
    // Test toggle
    cy.get('input[name="insecure"]').click({ force: true }) // Check if force needed for switch

    // Visual Snapshot of the filled form
    // Note: ensure consistent rendering (e.g. no animations in progress)
    //cy.matchImageSnapshot('vmware-credentials-drawer-filled')

    // Mock successful creation
    cy.intercept('POST', `**/namespaces/${namespace}/secrets`, {
      statusCode: 201,
      body: {
        metadata: {
          name: `${credName}-vmware-secret`,
          namespace,
        },
      },
    }).as('createSecret')

    cy.intercept('POST', `**/namespaces/${namespace}/vmwarecreds`, {
      statusCode: 201,
      body: {
        metadata: {
          name: credName,
          namespace,
        },
        spec: {
          secretRef: { name: `${credName}-vmware-secret` },
          datacenter: datacenter
        },
        status: {
            vmwareValidationStatus: 'Succeeded'
        }
      },
    }).as('createCreds')

    // Mock validation polling which happens after creation
    cy.intercept('GET', `**/namespaces/${namespace}/vmwarecreds/${credName}`, {
      body: {
        metadata: { name: credName, namespace },
        status: { vmwareValidationStatus: 'Succeeded' }
      }
    }).as('pollCreds')


    // Submit
    cy.get('[data-testid=vmware-cred-submit]').click()

    cy.wait('@createSecret').its('request.body.data').should((data) => {
        expect(data).to.have.property('VCENTER_PASSWORD')
    })
    
    cy.wait('@createCreds')
    
    // Should see success state in drawer (implied by pollCreds returning Succeeded)
    cy.contains('VMware credentials created').scrollIntoView().should('be.visible')
    
    // Wait for drawer to close (it closes after 1.5s on success)
    cy.get('[data-testid=vmware-cred-form]').should('not.exist')
  })

  it('should handle validation failure', () => {
    const credName = 'fail-creds'
    
    cy.contains('button', 'Add VMware Credentials').click()
    
    cy.get('input[name="credentialName"]').type(credName)
    cy.get('input[name="vcenterHost"]').type('fail.com')
    cy.get('input[name="datacenter"]').type('dc')
    cy.get('input[name="username"]').type('user')
    cy.get('input[name="password"]').type('pass')

    // Mock creation success but validation fail
    cy.intercept('POST', `**/namespaces/${namespace}/secrets`, { statusCode: 201, body: {} }).as('createSecret')
    cy.intercept('POST', `**/namespaces/${namespace}/vmwarecreds`, {
        statusCode: 201,
        body: { metadata: { name: credName, namespace } }
    }).as('createCreds')

    cy.intercept('GET', `**/namespaces/${namespace}/vmwarecreds/${credName}`, {
        body: {
          metadata: { name: credName, namespace },
          status: { vmwareValidationStatus: 'Failed', vmwareValidationMessage: 'Invalid credentials' }
        }
    }).as('pollCredsFail')

    cy.get('[data-testid=vmware-cred-submit]').click()
    
    cy.wait('@createSecret')
    cy.wait('@createCreds')
    cy.wait('@pollCredsFail') // might need to wait multiple times or just once depending on interval

    cy.contains('Invalid credentials').should('be.visible')
  })
})
