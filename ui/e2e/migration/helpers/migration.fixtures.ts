// Mock data for migration E2E tests.
// Shapes mirror the CRD types in src/api/migrations/model.ts and
// src/api/migration-templates/model.ts — keep in sync if those change.

export const NS = 'migration-system'
const API_VERSION = 'vjailbreak.k8s.pf9.io/v1alpha1'

// ─── Helpers ──────────────────────────────────────────────────────────────────

function baseMeta(name: string) {
  return {
    name,
    namespace: NS,
    uid: `uid-${name}`,
    resourceVersion: '1000',
    generation: 1,
    creationTimestamp: '2026-05-20T10:00:00Z',
    annotations: {},
    labels: {},
  }
}

function listMeta() {
  return { continue: '', resourceVersion: '9999' }
}

// ─── Migration objects ────────────────────────────────────────────────────────

export const MOCK_MIGRATION_PENDING = {
  apiVersion: API_VERSION,
  kind: 'Migration',
  metadata: { ...baseMeta('test-vm-1-migration'), labels: { migrationplan: 'test-plan-1' } },
  spec: {
    migrationPlan: 'test-plan-1',
    podRef: 'v2v-helper-test-1',
    vmName: 'test-vm-1',
  },
  status: {
    phase: 'Pending',
    conditions: [],
  },
}

export const MOCK_MIGRATION_RUNNING = {
  apiVersion: API_VERSION,
  kind: 'Migration',
  metadata: { ...baseMeta('test-vm-2-migration'), labels: { migrationplan: 'test-plan-1' } },
  spec: {
    migrationPlan: 'test-plan-1',
    podRef: 'v2v-helper-test-2',
    vmName: 'test-vm-2',
  },
  status: {
    phase: 'CopyingBlocks',
    conditions: [
      {
        lastTransitionTime: '2026-05-20T10:05:00Z',
        message: 'Copying disk 0',
        reason: 'Migration',
        status: 'True',
        type: 'Running',
      },
    ],
  },
}

export const MOCK_MIGRATION_SUCCEEDED = {
  apiVersion: API_VERSION,
  kind: 'Migration',
  metadata: { ...baseMeta('test-vm-3-migration'), labels: { migrationplan: 'test-plan-2' } },
  spec: {
    migrationPlan: 'test-plan-2',
    podRef: 'v2v-helper-test-3',
    vmName: 'test-vm-3',
  },
  status: {
    phase: 'Succeeded',
    conditions: [
      {
        lastTransitionTime: '2026-05-20T11:00:00Z',
        message: 'Migrating VM from VMware to OpenStack',
        reason: 'Migration',
        status: 'True',
        type: 'Succeeded',
      },
    ],
  },
}

export const MOCK_MIGRATION_FAILED = {
  apiVersion: API_VERSION,
  kind: 'Migration',
  metadata: { ...baseMeta('test-vm-4-migration'), labels: { migrationplan: 'test-plan-3' } },
  spec: {
    migrationPlan: 'test-plan-3',
    podRef: 'v2v-helper-test-4',
    vmName: 'test-vm-4',
  },
  status: {
    phase: 'Failed',
    conditions: [
      {
        lastTransitionTime: '2026-05-20T10:30:00Z',
        message: 'Disk copy failed: connection timeout',
        reason: 'Migration',
        status: 'False',
        type: 'Failed',
      },
    ],
  },
}

export const MOCK_MIGRATION_AWAITING_CUTOVER = {
  apiVersion: API_VERSION,
  kind: 'Migration',
  metadata: { ...baseMeta('test-vm-5-migration'), labels: { migrationplan: 'test-plan-4' } },
  spec: {
    migrationPlan: 'test-plan-4',
    podRef: 'v2v-helper-test-5',
    vmName: 'test-vm-5',
    initiateCutover: true,
  },
  status: {
    phase: 'AwaitingAdminCutOver',
    conditions: [
      {
        lastTransitionTime: '2026-05-20T10:45:00Z',
        message: 'Awaiting admin cutover trigger',
        reason: 'Migration',
        status: 'True',
        type: 'Paused',
      },
    ],
  },
}

export const MOCK_MIGRATIONS_LIST = {
  apiVersion: API_VERSION,
  kind: 'MigrationList',
  metadata: listMeta(),
  items: [
    MOCK_MIGRATION_PENDING,
    MOCK_MIGRATION_RUNNING,
    MOCK_MIGRATION_SUCCEEDED,
    MOCK_MIGRATION_FAILED,
    MOCK_MIGRATION_AWAITING_CUTOVER,
  ],
}

export const MOCK_MIGRATIONS_LIST_EMPTY = {
  apiVersion: API_VERSION,
  kind: 'MigrationList',
  metadata: listMeta(),
  items: [],
}

// ─── Migration Plans ──────────────────────────────────────────────────────────

export const MOCK_MIGRATION_PLAN_1 = {
  apiVersion: API_VERSION,
  kind: 'MigrationPlan',
  metadata: baseMeta('test-plan-1'),
  spec: {
    migrationStrategy: { type: 'cold' },
    migrationTemplate: 'test-template-abc123',
    retry: false,
    virtualMachines: [['test-vm-1', 'test-vm-2']],
  },
  status: {
    migrationStatus: 'Running',
    migrationMessage: '',
  },
}

export const MOCK_MIGRATION_PLAN_2 = {
  apiVersion: API_VERSION,
  kind: 'MigrationPlan',
  metadata: baseMeta('test-plan-2'),
  spec: {
    migrationStrategy: { type: 'cold' },
    migrationTemplate: 'test-template-abc123',
    retry: false,
    virtualMachines: [['test-vm-3']],
  },
  status: { migrationStatus: 'Succeeded', migrationMessage: '' },
}

export const MOCK_MIGRATION_PLAN_3 = {
  apiVersion: API_VERSION,
  kind: 'MigrationPlan',
  metadata: baseMeta('test-plan-3'),
  spec: {
    migrationStrategy: { type: 'cold' },
    migrationTemplate: 'test-template-abc123',
    retry: false,
    virtualMachines: [['test-vm-4']],
  },
  status: { migrationStatus: 'Failed', migrationMessage: '' },
}

export const MOCK_MIGRATION_PLAN_5 = {
  apiVersion: API_VERSION,
  kind: 'MigrationPlan',
  metadata: baseMeta('test-plan-5'),
  spec: {
    migrationStrategy: { type: 'cold' },
    migrationTemplate: 'test-template-abc123',
    retry: false,
    virtualMachines: [['test-vm-6']],
  },
  status: { migrationStatus: 'Pending', migrationMessage: '' },
}

export const MOCK_MIGRATION_PLANS_LIST = {
  apiVersion: API_VERSION,
  kind: 'MigrationPlanList',
  metadata: listMeta(),
  items: [MOCK_MIGRATION_PLAN_1],
}

export const MOCK_MIGRATION_PLANS_LIST_EMPTY = {
  apiVersion: API_VERSION,
  kind: 'MigrationPlanList',
  metadata: listMeta(),
  items: [],
}

// ─── VMware Credentials ───────────────────────────────────────────────────────

export const MOCK_VMWARE_CRED_1 = {
  apiVersion: API_VERSION,
  kind: 'VMwareCreds',
  metadata: baseMeta('vcenter-cred-1'),
  spec: {
    secretRef: { name: 'vcenter-cred-1-secret' },
    datacenter: 'DC1',
    hostName: 'vcenter.example.com',
  },
  status: {
    vmwareValidationStatus: 'Succeeded',
    vmwareValidationMessage: 'Validated successfully',
  },
}

export const MOCK_VMWARE_CREDS_LIST = {
  apiVersion: API_VERSION,
  kind: 'VMwareCredsList',
  metadata: listMeta(),
  items: [MOCK_VMWARE_CRED_1],
}

// ─── OpenStack / PCD Credentials ─────────────────────────────────────────────

export const MOCK_OPENSTACK_CRED_1 = {
  apiVersion: API_VERSION,
  kind: 'OpenstackCreds',
  metadata: {
    ...baseMeta('pcd-cred-1'),
    labels: { 'vjailbreak.k8s.pf9.io/is-pcd': 'true' },
  },
  spec: {
    secretRef: { name: 'pcd-cred-1-secret' },
  },
  status: {
    openstackValidationStatus: 'Succeeded',
    openstackValidationMessage: 'Validated successfully',
    openstack: {
      networks: [
        { name: 'pcd-network-1', tags: [] },
        { name: 'pcd-network-2', tags: [] },
        { name: 'external-network', tags: [] },
      ],
      volumeTypes: ['standard', 'high-performance', 'ceph-ssd'],
      securityGroups: [],
      serverGroups: [],
    },
  },
}

export const MOCK_OPENSTACK_CREDS_LIST = {
  apiVersion: API_VERSION,
  kind: 'OpenstackCredsList',
  metadata: listMeta(),
  items: [MOCK_OPENSTACK_CRED_1],
}

// ─── Migration Templates ──────────────────────────────────────────────────────

// Template before controller populates status (polling state)
export const MOCK_MIGRATION_TEMPLATE_PENDING = {
  apiVersion: API_VERSION,
  kind: 'MigrationTemplate',
  metadata: {
    ...baseMeta('test-template-abc123'),
    labels: { refresh: 'false' },
  },
  spec: {
    source: { vmwareRef: 'vcenter-cred-1' },
    destination: { openstackRef: 'pcd-cred-1' },
    networkMapping: '',
    storageMapping: '',
  },
  status: {
    openstack: { networks: [], volumeTypes: [] },
    vmware: [],
  },
}

// Template after controller populates status (ready for VM selection)
export const MOCK_MIGRATION_TEMPLATE_READY = {
  ...MOCK_MIGRATION_TEMPLATE_PENDING,
  status: {
    openstack: {
      networks: ['pcd-network-1', 'pcd-network-2', 'external-network'],
      volumeTypes: ['standard', 'high-performance', 'ceph-ssd'],
    },
    vmware: [
      {
        id: 'vm-001',
        name: 'test-vm-1',
        vmid: 'vm-001',
        networks: ['VM Network', 'Management'],
        datastores: ['datastore1'],
        vmState: 'poweredOn',
        memory: 4096,
        cpuCount: 2,
        ipAddress: '192.168.1.101',
        networkInterfaces: [
          { mac: '00:50:56:aa:01:01', network: 'VM Network', ipAddress: ['192.168.1.101'] },
        ],
      },
      {
        id: 'vm-002',
        name: 'test-vm-2',
        vmid: 'vm-002',
        networks: ['VM Network'],
        datastores: ['datastore1'],
        vmState: 'poweredOn',
        memory: 8192,
        cpuCount: 4,
        ipAddress: '192.168.1.102',
        networkInterfaces: [
          { mac: '00:50:56:aa:01:02', network: 'VM Network', ipAddress: ['192.168.1.102'] },
        ],
      },
      {
        id: 'vm-003',
        name: 'test-vm-powered-off',
        vmid: 'vm-003',
        networks: ['VM Network'],
        datastores: ['datastore2'],
        vmState: 'poweredOff',
        memory: 2048,
        cpuCount: 1,
        ipAddress: '',
        networkInterfaces: [
          { mac: '00:50:56:aa:01:03', network: 'VM Network', ipAddress: [] },
        ],
      },
      {
        id: 'vm-004',
        name: 'test-vm-rdm',
        vmid: 'vm-004',
        networks: ['VM Network'],
        datastores: ['datastore1'],
        vmState: 'poweredOn',
        memory: 4096,
        cpuCount: 2,
        ipAddress: '192.168.1.104',
        rdmDisks: ['naa.600000000000001'],
        networkInterfaces: [
          { mac: '00:50:56:aa:01:04', network: 'VM Network', ipAddress: ['192.168.1.104'] },
        ],
      },
      {
        id: 'vm-005',
        name: 'test-vm-multi-network',
        vmid: 'vm-005',
        networks: ['VM Network', 'Management'],
        datastores: ['datastore1', 'datastore2'],
        vmState: 'poweredOn',
        memory: 16384,
        cpuCount: 8,
        ipAddress: '192.168.1.105',
        networkInterfaces: [
          { mac: '00:50:56:aa:01:05', network: 'VM Network', ipAddress: ['192.168.1.105'] },
          { mac: '00:50:56:aa:01:06', network: 'Management', ipAddress: ['10.0.0.105'] },
        ],
      },
    ],
  },
}

// ─── Large VM list for MIG-030 (stress test — 50+ VMs) ───────────────────────

export const MOCK_VM_LIST_LARGE = Array.from({ length: 55 }, (_, i) => ({
  id: `vm-large-${String(i + 1).padStart(3, '0')}`,
  name: `stress-test-vm-${String(i + 1).padStart(3, '0')}`,
  vmid: `vm-large-${String(i + 1).padStart(3, '0')}`,
  networks: ['VM Network'],
  datastores: ['datastore1'],
  vmState: i % 5 === 0 ? 'poweredOff' : 'poweredOn',
  memory: 4096,
  cpuCount: 2,
  ipAddress: i % 5 === 0 ? '' : `10.0.${Math.floor(i / 255)}.${(i % 255) + 1}`,
  networkInterfaces: [
    {
      mac: `00:50:56:bb:${String(Math.floor(i / 256)).padStart(2, '0')}:${String(i % 256).padStart(2, '0')}`,
      network: 'VM Network',
      ipAddress: i % 5 === 0 ? [] : [`10.0.${Math.floor(i / 255)}.${(i % 255) + 1}`],
    },
  ],
}))

export const MOCK_MIGRATION_TEMPLATE_LARGE = {
  ...MOCK_MIGRATION_TEMPLATE_PENDING,
  status: {
    openstack: {
      networks: ['pcd-network-1', 'pcd-network-2'],
      volumeTypes: ['standard'],
    },
    vmware: MOCK_VM_LIST_LARGE,
  },
}

// ─── Network & Storage mapping responses ─────────────────────────────────────

export const MOCK_NETWORK_MAPPING_CREATED = {
  apiVersion: API_VERSION,
  kind: 'NetworkMapping',
  metadata: baseMeta('network-mapping-abc123'),
  spec: {
    networks: [
      { source: 'VM Network', target: 'pcd-network-1' },
      { source: 'Management', target: 'pcd-network-2' },
    ],
  },
}

export const MOCK_STORAGE_MAPPING_CREATED = {
  apiVersion: API_VERSION,
  kind: 'StorageMapping',
  metadata: baseMeta('storage-mapping-abc123'),
  spec: {
    storages: [
      { source: 'datastore1', target: 'standard' },
      { source: 'datastore2', target: 'high-performance' },
    ],
  },
}

// ─── Migration plan create response ──────────────────────────────────────────

export const MOCK_MIGRATION_PLAN_CREATED = {
  apiVersion: API_VERSION,
  kind: 'MigrationPlan',
  metadata: baseMeta('new-migration-plan'),
  spec: {
    migrationStrategy: { type: 'cold' },
    migrationTemplate: 'test-template-abc123',
    retry: false,
    virtualMachines: [['test-vm-1', 'test-vm-2']],
  },
  status: {
    migrationStatus: 'Pending',
    migrationMessage: '',
  },
}

// ─── Three migrations for bulk-delete test (MIG-007) ─────────────────────────

export const MOCK_MIGRATIONS_FOR_BULK_DELETE = {
  apiVersion: API_VERSION,
  kind: 'MigrationList',
  metadata: listMeta(),
  items: [
    MOCK_MIGRATION_SUCCEEDED,
    MOCK_MIGRATION_FAILED,
    {
      ...MOCK_MIGRATION_PENDING,
      metadata: { ...baseMeta('test-vm-6-migration'), labels: { migrationplan: 'test-plan-5' } },
      spec: { ...MOCK_MIGRATION_PENDING.spec, vmName: 'test-vm-6', migrationPlan: 'test-plan-5' },
    },
  ],
}

// ─── IP validation responses ──────────────────────────────────────────────────

export const MOCK_IP_VALIDATION_VALID = {
  valid: true,
  conflicts: [],
}

export const MOCK_IP_VALIDATION_CONFLICT = {
  valid: false,
  conflicts: ['192.168.1.101'],
}

// ─── VMware clusters (for cluster selection dropdown) ─────────────────────────

export const MOCK_VMWARE_CLUSTER_1 = {
  apiVersion: API_VERSION,
  kind: 'VMwareCluster',
  metadata: {
    ...baseMeta('vcenter-cred-1-dc1-cluster'),
    annotations: { 'vjailbreak.k8s.pf9.io/datacenter': 'DC1' },
    labels: { 'vjailbreak.k8s.pf9.io/vmwarecreds': 'vcenter-cred-1' },
  },
  spec: {
    name: 'DC1-Cluster',
    hosts: ['esxi-host-1.example.com', 'esxi-host-2.example.com'],
  },
}

export const MOCK_VMWARE_CLUSTERS_LIST = {
  apiVersion: API_VERSION,
  kind: 'VMwareClusterList',
  metadata: listMeta(),
  items: [MOCK_VMWARE_CLUSTER_1],
}

// ─── PCD clusters (for cluster selection dropdown) ────────────────────────────

export const MOCK_PCD_CLUSTER_1 = {
  apiVersion: API_VERSION,
  kind: 'PCDCluster',
  metadata: {
    ...baseMeta('pcd-cluster-1'),
    labels: { 'vjailbreak.k8s.pf9.io/openstackcreds': 'pcd-cred-1' },
  },
  spec: {
    clusterName: 'pcd-cluster-1',
    hosts: ['pcd-host-1'],
  },
  status: {},
}

export const MOCK_PCD_CLUSTERS_LIST = {
  apiVersion: API_VERSION,
  kind: 'PCDClusterList',
  metadata: listMeta(),
  items: [MOCK_PCD_CLUSTER_1],
}

// ─── VMware hosts (for rolling migration ESXi host step) ─────────────────────

export const MOCK_VMWARE_HOST_1 = {
  apiVersion: API_VERSION,
  kind: 'VMwareHost',
  metadata: {
    ...baseMeta('esxi-host-1'),
    labels: { 'vjailbreak.k8s.pf9.io/vmwarecreds': 'vcenter-cred-1' },
  },
  spec: {
    name: 'esxi-host-1.example.com',
    hardwareUuid: 'aaaa-1111-bbbb-2222',
    hostConfigId: '',
    clusterName: 'vcenter-cred-1-dc1-cluster',
  },
  status: { esxiVersion: '7.0.3', vmCount: 2, state: 'Ready' },
}

export const MOCK_VMWARE_HOST_2 = {
  apiVersion: API_VERSION,
  kind: 'VMwareHost',
  metadata: {
    ...baseMeta('esxi-host-2'),
    labels: { 'vjailbreak.k8s.pf9.io/vmwarecreds': 'vcenter-cred-1' },
  },
  spec: {
    name: 'esxi-host-2.example.com',
    hardwareUuid: 'cccc-3333-dddd-4444',
    hostConfigId: '',
    clusterName: 'vcenter-cred-1-dc1-cluster',
  },
  status: { esxiVersion: '7.0.3', vmCount: 3, state: 'Ready' },
}

export const MOCK_VMWARE_HOSTS_LIST = {
  apiVersion: API_VERSION,
  kind: 'VMwareHostList',
  metadata: listMeta(),
  items: [MOCK_VMWARE_HOST_1, MOCK_VMWARE_HOST_2],
}

// ─── BMConfigs / MAAS configs (for rolling migration baremetal step) ──────────

export const MOCK_BM_CONFIG_1 = {
  apiVersion: API_VERSION,
  kind: 'BMConfig',
  metadata: { ...baseMeta('maas-config-1'), creationTimestamp: new Date('2026-05-20T10:00:00Z') },
  spec: {
    providerType: 'maas',
    apiUrl: 'http://maas.example.com/MAAS',
    apiKey: 'maas-api-key-redacted',
    userDataSecretRef: { name: 'maas-userdata-secret', namespace: 'migration-system' },
    insecure: false,
    os: 'ubuntu/jammy',
  },
  status: { validationStatus: 'Succeeded', validationMessage: 'MAAS config validated' },
}

export const MOCK_BM_CONFIGS_LIST = {
  apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
  kind: 'BMConfigList',
  metadata: listMeta(),
  items: [MOCK_BM_CONFIG_1],
}

// ─── OpenStack creds with PCD host config (for rolling migration) ─────────────

export const MOCK_OPENSTACK_CRED_WITH_HOST_CONFIG = {
  ...MOCK_OPENSTACK_CRED_1,
  spec: {
    ...MOCK_OPENSTACK_CRED_1.spec,
    projectName: 'admin',
    pcdHostConfig: [
      {
        id: 'host-config-1',
        name: 'PCD Host Config 1',
        clusterName: 'pcd-cluster-1',
        displayName: 'PCD Host Config 1',
      },
    ],
  },
}

// ─── VMware Machines (for VM selection step) ─────────────────────────────────

function baseMachineLabels() {
  return {
    'vjailbreak.k8s.pf9.io/vmwarecreds': 'vcenter-cred-1',
    'vjailbreak.k8s.pf9.io/vmware-cluster': 'vcenter-cred-1-dc1-cluster',
  }
}

export const MOCK_VMWARE_MACHINE_1 = {
  apiVersion: API_VERSION,
  kind: 'VMwareMachine',
  metadata: { name: 'vcenter-cred-1-test-vm-1', namespace: NS, creationTimestamp: '2026-05-20T10:00:00Z', labels: baseMachineLabels() },
  spec: {
    vms: {
      name: 'test-vm-1', vmid: 'vm-001', cpu: 2, memory: 4096,
      vmState: 'poweredOn', ipAddress: '192.168.1.101', osFamily: 'Linux',
      networks: ['VM Network'], datastores: ['datastore1'], disks: [],
      networkInterfaces: [{ mac: '00:50:56:aa:01:01', network: 'VM Network', ipAddress: ['192.168.1.101'] }],
    },
  },
  status: { migrated: false, powerState: 'running' },
}

export const MOCK_VMWARE_MACHINE_POWERED_OFF = {
  apiVersion: API_VERSION,
  kind: 'VMwareMachine',
  metadata: { name: 'vcenter-cred-1-test-vm-powered-off', namespace: NS, creationTimestamp: '2026-05-20T10:00:00Z', labels: baseMachineLabels() },
  spec: {
    vms: {
      name: 'test-vm-powered-off', vmid: 'vm-003', cpu: 1, memory: 2048,
      vmState: 'poweredOff', ipAddress: '',
      networks: ['VM Network'], datastores: ['datastore2'], disks: [],
      networkInterfaces: [{ mac: '00:50:56:aa:01:03', network: 'VM Network', ipAddress: [] }],
    },
  },
  status: { migrated: false, powerState: 'stopped' },
}

export const MOCK_VMWARE_MACHINE_MULTI_NETWORK = {
  apiVersion: API_VERSION,
  kind: 'VMwareMachine',
  metadata: { name: 'vcenter-cred-1-test-vm-multi-network', namespace: NS, creationTimestamp: '2026-05-20T10:00:00Z', labels: baseMachineLabels() },
  spec: {
    vms: {
      name: 'test-vm-multi-network', vmid: 'vm-005', cpu: 8, memory: 16384,
      vmState: 'poweredOn', ipAddress: '192.168.1.105', osFamily: 'Linux',
      networks: ['VM Network', 'Management'], datastores: ['datastore1', 'datastore2'], disks: [],
      networkInterfaces: [
        { mac: '00:50:56:aa:01:05', network: 'VM Network', ipAddress: ['192.168.1.105'] },
        { mac: '00:50:56:aa:01:06', network: 'Management', ipAddress: ['10.0.0.105'] },
      ],
    },
  },
  status: { migrated: false, powerState: 'running' },
}

export const MOCK_VMWARE_MACHINES_LIST = {
  apiVersion: API_VERSION,
  kind: 'VMwareMachineList',
  metadata: listMeta(),
  items: [MOCK_VMWARE_MACHINE_1, MOCK_VMWARE_MACHINE_POWERED_OFF, MOCK_VMWARE_MACHINE_MULTI_NETWORK],
}

export const MOCK_VMWARE_MACHINE_RDM = {
  apiVersion: API_VERSION,
  kind: 'VMwareMachine',
  metadata: { name: 'vcenter-cred-1-test-vm-rdm', namespace: NS, creationTimestamp: '2026-05-20T10:00:00Z', labels: baseMachineLabels() },
  spec: {
    vms: {
      name: 'test-vm-rdm', vmid: 'vm-004', cpu: 2, memory: 4096,
      vmState: 'poweredOn', ipAddress: '192.168.1.104', osFamily: 'Linux',
      networks: ['VM Network'], datastores: ['datastore1'],
      rdmDisks: ['naa.600000000000001'],
      disks: [],
      networkInterfaces: [{ mac: '00:50:56:aa:01:04', network: 'VM Network', ipAddress: ['192.168.1.104'] }],
    },
  },
  status: { migrated: false, powerState: 'running' },
}

export const MOCK_VMWARE_MACHINES_LIST_WITH_RDM = {
  apiVersion: API_VERSION,
  kind: 'VMwareMachineList',
  metadata: listMeta(),
  items: [...MOCK_VMWARE_MACHINES_LIST.items, MOCK_VMWARE_MACHINE_RDM],
}

export const MOCK_VMWARE_MACHINES_LIST_LARGE = {
  apiVersion: API_VERSION,
  kind: 'VMwareMachineList',
  metadata: listMeta(),
  items: Array.from({ length: 55 }, (_, i) => ({
    apiVersion: API_VERSION,
    kind: 'VMwareMachine',
    metadata: {
      name: `vcenter-cred-1-stress-test-vm-${String(i + 1).padStart(3, '0')}`,
      namespace: NS,
      creationTimestamp: '2026-05-20T10:00:00Z',
      labels: {
        'vjailbreak.k8s.pf9.io/vmwarecreds': 'vcenter-cred-1',
        'vjailbreak.k8s.pf9.io/vmware-cluster': 'vcenter-cred-1-dc1-cluster',
      },
    },
    spec: {
      vms: {
        name: `stress-test-vm-${String(i + 1).padStart(3, '0')}`,
        vmid: `vm-large-${String(i + 1).padStart(3, '0')}`,
        cpu: 2,
        memory: 4096,
        vmState: i % 5 === 0 ? 'poweredOff' : 'poweredOn',
        ipAddress: i % 5 === 0 ? '' : `10.0.${Math.floor(i / 255)}.${(i % 255) + 1}`,
        networks: ['VM Network'],
        datastores: ['datastore1'],
        disks: [],
        networkInterfaces: [{
          mac: `00:50:56:bb:${String(Math.floor(i / 256)).padStart(2, '0')}:${String(i % 256).padStart(2, '0')}`,
          network: 'VM Network',
          ipAddress: i % 5 === 0 ? [] : [`10.0.${Math.floor(i / 255)}.${(i % 255) + 1}`],
        }],
      },
    },
    status: { migrated: false, powerState: i % 5 === 0 ? 'stopped' : 'running' },
  })),
}

export const MOCK_RDM_DISK_1 = {
  apiVersion: API_VERSION,
  kind: 'RdmDisk',
  metadata: { ...baseMeta('rdm-disk-naa-600000000000001') },
  spec: {
    diskName: 'naa.600000000000001',
    displayName: 'RDM Disk 1',
    uuid: 'uuid-rdm-001',
    diskSize: 10737418240,
    ownerVMs: ['test-vm-rdm'],
    openstackVolumeRef: {},
  },
}

export const MOCK_RDM_DISKS_LIST = {
  apiVersion: API_VERSION,
  kind: 'RdmDiskList',
  metadata: listMeta(),
  items: [MOCK_RDM_DISK_1],
}

// ─── Volume Image Profiles (for MIG-018 conflict detection) ──────────────────

export const MOCK_VOLUME_IMAGE_PROFILE_A = {
  apiVersion: API_VERSION,
  kind: 'VolumeImageProfile',
  metadata: { ...baseMeta('profile-gpu-1'), labels: {} },
  spec: {
    osFamily: 'any',
    description: 'GPU profile 1',
    properties: { 'hw:gpu_count': '1', 'hw:cpu_policy': 'dedicated' },
  },
}

export const MOCK_VOLUME_IMAGE_PROFILE_B = {
  apiVersion: API_VERSION,
  kind: 'VolumeImageProfile',
  metadata: { ...baseMeta('profile-gpu-2'), labels: {} },
  spec: {
    osFamily: 'any',
    description: 'GPU profile 2 — conflicts with profile-gpu-1 on hw:gpu_count',
    properties: { 'hw:gpu_count': '2', 'hw:cpu_policy': 'shared' },
  },
}

export const MOCK_VOLUME_IMAGE_PROFILES_LIST = {
  apiVersion: API_VERSION,
  kind: 'VolumeImageProfileList',
  metadata: listMeta(),
  items: [MOCK_VOLUME_IMAGE_PROFILE_A, MOCK_VOLUME_IMAGE_PROFILE_B],
}

// ─── Rolling migration plan creation response ─────────────────────────────────

export const MOCK_ROLLING_MIGRATION_PLAN_CREATED = {
  apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
  kind: 'RollingMigrationPlan',
  metadata: baseMeta('rolling-plan-1'),
  spec: {
    migrationTemplate: 'test-template-abc123',
    virtualMachines: [['test-vm-1', 'test-vm-2']],
    esxiHosts: [
      { name: 'esxi-host-1.example.com', hostConfigId: 'host-config-1' },
      { name: 'esxi-host-2.example.com', hostConfigId: 'host-config-1' },
    ],
  },
  status: { migrationStatus: 'Pending', migrationMessage: '' },
}

// ─── Edit & Retry fixtures ─────────────────────────────────────────────────────
// Resource names follow the controller's conventions: the Migration is named
// "migration-<vm-k8s-name>" and the VMwareMachine is "<vm-k8s-name>".

export const MOCK_RETRY_VM_K8S_NAME = 'vcenter-cred-1-test-vm-retry'
export const MOCK_RETRY_MIGRATION_NAME = `migration-${MOCK_RETRY_VM_K8S_NAME}`
export const MOCK_RETRY_PLAN_NAME = 'retry-plan-1'
export const MOCK_RETRY_TEMPLATE_NAME = 'retry-template-1'
export const MOCK_RETRY_VM_KEY = 'test-vm-retry-2001'

export const MOCK_MIGRATION_FAILED_RETRYABLE = {
  apiVersion: API_VERSION,
  kind: 'Migration',
  metadata: {
    ...baseMeta(MOCK_RETRY_MIGRATION_NAME),
    labels: { migrationplan: MOCK_RETRY_PLAN_NAME },
    annotations: { 'vjailbreak.k8s.pf9.io/original-vm-name': MOCK_RETRY_VM_KEY },
  },
  spec: {
    migrationPlan: MOCK_RETRY_PLAN_NAME,
    podRef: 'v2v-helper-retry-1',
    vmName: 'test-vm-retry',
    migrationType: 'cold',
  },
  status: {
    phase: 'Failed',
    retryable: true,
    conditions: [
      {
        lastTransitionTime: '2026-06-10T10:30:00Z',
        message: 'No suitable flavor found',
        reason: 'Migration',
        status: 'False',
        type: 'Failed',
      },
    ],
  },
}

export const MOCK_RETRY_MIGRATION_PLAN = {
  apiVersion: API_VERSION,
  kind: 'MigrationPlan',
  metadata: baseMeta(MOCK_RETRY_PLAN_NAME),
  spec: {
    migrationStrategy: { type: 'cold', adminInitiatedCutOver: false },
    migrationTemplate: MOCK_RETRY_TEMPLATE_NAME,
    retry: false,
    virtualMachines: [[MOCK_RETRY_VM_KEY]],
    securityGroups: ['default-sg'],
    fallbackToDHCP: false,
  },
  status: { migrationStatus: 'Failed', migrationMessage: 'Migration failed for test-vm-retry' },
}

export const MOCK_RETRY_MIGRATION_TEMPLATE = {
  apiVersion: API_VERSION,
  kind: 'MigrationTemplate',
  metadata: { ...baseMeta(MOCK_RETRY_TEMPLATE_NAME), labels: { refresh: 'false' } },
  spec: {
    source: { vmwareRef: 'vcenter-cred-1', datacenter: 'DC1' },
    destination: { openstackRef: 'pcd-cred-1' },
    networkMapping: 'retry-netmap-1',
    storageMapping: 'retry-stormap-1',
    targetPCDClusterName: 'pcd-cluster-1',
    storageCopyMethod: 'normal',
  },
  status: {
    openstack: {
      networks: ['pcd-network-1', 'pcd-network-2', 'external-network'],
      volumeTypes: ['standard', 'high-performance', 'ceph-ssd'],
    },
    vmware: [],
  },
}

export const MOCK_RETRY_NETWORK_MAPPING = {
  apiVersion: API_VERSION,
  kind: 'NetworkMapping',
  metadata: baseMeta('retry-netmap-1'),
  spec: { networks: [{ source: 'VM Network', target: 'pcd-network-1' }] },
  status: { networkmappingValidationStatus: 'Succeeded' },
}

export const MOCK_RETRY_STORAGE_MAPPING = {
  apiVersion: API_VERSION,
  kind: 'StorageMapping',
  metadata: baseMeta('retry-stormap-1'),
  spec: { storages: [{ source: 'datastore1', target: 'standard' }] },
  status: { storagemappingValidationStatus: 'Succeeded' },
}

export const MOCK_RETRY_VMWARE_MACHINE = {
  apiVersion: API_VERSION,
  kind: 'VMwareMachine',
  metadata: {
    name: MOCK_RETRY_VM_K8S_NAME,
    namespace: NS,
    creationTimestamp: '2026-06-10T10:00:00Z',
    labels: {
      'vjailbreak.k8s.pf9.io/vmwarecreds': 'vcenter-cred-1',
    },
  },
  spec: {
    targetFlavorId: '',
    vms: {
      name: 'test-vm-retry',
      vmid: 'vm-2001',
      cpu: 2,
      memory: 4096,
      vmState: 'poweredOn',
      ipAddress: '192.168.1.150',
      osFamily: 'Linux',
      clusterName: 'cluster-1',
      networks: ['VM Network'],
      datastores: ['datastore1'],
      disks: [],
      networkInterfaces: [
        { mac: '00:50:56:aa:02:01', network: 'VM Network', ipAddress: ['192.168.1.150'] },
      ],
    },
  },
  status: { migrated: false, powerState: 'running' },
}

// OpenStack creds with flavors so the retry flavor dropdown has options.
export const MOCK_OPENSTACK_CRED_WITH_FLAVORS = {
  ...MOCK_OPENSTACK_CRED_1,
  spec: {
    ...MOCK_OPENSTACK_CRED_1.spec,
    flavors: [
      { id: 'flavor-small', name: 'm1.small', vcpus: 2, ram: 4096, disk: 20 },
      { id: 'flavor-large', name: 'm1.large', vcpus: 8, ram: 16384, disk: 80 },
    ],
  },
}

export const MOCK_MIGRATIONS_LIST_WITH_RETRYABLE = {
  apiVersion: API_VERSION,
  kind: 'MigrationList',
  metadata: listMeta(),
  items: [MOCK_MIGRATION_FAILED_RETRYABLE],
}

export const MOCK_RETRY_NETWORK_MAPPING_CREATED = {
  apiVersion: API_VERSION,
  kind: 'NetworkMapping',
  metadata: baseMeta('new-netmap-uuid-1'),
  spec: { networks: [{ source: 'VM Network', target: 'pcd-network-2' }] },
}

export const MOCK_RETRY_STORAGE_MAPPING_CREATED = {
  apiVersion: API_VERSION,
  kind: 'StorageMapping',
  metadata: baseMeta('new-stormap-uuid-1'),
  spec: { storages: [{ source: 'datastore1', target: 'standard' }] },
}

// Plan variant that carries existing per-VM IP override (preserveIP=false, UserAssignedIP).
// Used by RET-005 to verify that existing overrides are round-tripped through the retry form.
export const MOCK_RETRY_MIGRATION_PLAN_WITH_IP_OVERRIDE = {
  ...MOCK_RETRY_MIGRATION_PLAN,
  spec: {
    ...MOCK_RETRY_MIGRATION_PLAN.spec,
    networkOverridesPerVM: {
      [MOCK_RETRY_VM_KEY]: [
        {
          interfaceIndex: 0,
          preserveIP: false,
          preserveMAC: true,
          UserAssignedIP: '10.0.0.50',
        },
      ],
    },
  },
}

// Plan variant with two VMs (same batch). Used by RET-006 to verify the multi-VM warning
// banner appears when the plan's total VM count > 1.
export const MOCK_RETRY_MIGRATION_PLAN_MULTIVM = {
  ...MOCK_RETRY_MIGRATION_PLAN,
  spec: {
    ...MOCK_RETRY_MIGRATION_PLAN.spec,
    virtualMachines: [[MOCK_RETRY_VM_KEY, 'other-vm-9999']],
  },
}

// VMwareMachine with the datacenter annotation — mirrors how the controller stamps it.
// Used to verify the datacenter annotation path in useRetryPrefill.
export const MOCK_RETRY_VMWARE_MACHINE_WITH_DC_ANNOTATION = {
  ...MOCK_RETRY_VMWARE_MACHINE,
  metadata: {
    ...MOCK_RETRY_VMWARE_MACHINE.metadata,
    annotations: {
      'vjailbreak.k8s.pf9.io/datacenter': 'DC1',
    },
  },
}

// ─── New-plan fixtures (RET-003, RET-007) ─────────────────────────────────────
// API responses returned when the UI POSTs new resources during "Edit & Retry".

export const MOCK_RETRY_CLONE_TEMPLATE_SUFFIX = '-r-abc123'
export const MOCK_RETRY_CLONE_TEMPLATE_NAME = `${MOCK_RETRY_TEMPLATE_NAME}${MOCK_RETRY_CLONE_TEMPLATE_SUFFIX}`
export const MOCK_RETRY_CLONE_PLAN_NAME = `${MOCK_RETRY_PLAN_NAME}${MOCK_RETRY_CLONE_TEMPLATE_SUFFIX}`

export const MOCK_RETRY_CLONE_TEMPLATE_CREATED = {
  apiVersion: API_VERSION,
  kind: 'MigrationTemplate',
  metadata: { ...baseMeta(MOCK_RETRY_CLONE_TEMPLATE_NAME), labels: { refresh: 'false' } },
  spec: {
    source: { vmwareRef: 'vcenter-cred-1', datacenter: 'DC1' },
    destination: { openstackRef: 'pcd-cred-1' },
    networkMapping: 'new-netmap-uuid-1',
    storageMapping: 'new-stormap-uuid-1',
    targetPCDClusterName: 'pcd-cluster-1',
    storageCopyMethod: 'normal',
  },
  status: {
    openstack: { networks: [], volumeTypes: [] },
    vmware: [],
  },
}

export const MOCK_RETRY_CLONE_PLAN_CREATED = {
  apiVersion: API_VERSION,
  kind: 'MigrationPlan',
  metadata: baseMeta(MOCK_RETRY_CLONE_PLAN_NAME),
  spec: {
    migrationStrategy: { type: 'cold', adminInitiatedCutOver: false },
    migrationTemplate: MOCK_RETRY_CLONE_TEMPLATE_NAME,
    retry: false,
    virtualMachines: [[MOCK_RETRY_VM_KEY]],
  },
  status: { migrationStatus: 'Pending', migrationMessage: '' },
}
