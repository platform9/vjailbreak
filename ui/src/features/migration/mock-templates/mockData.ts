import { CUTOVER_TYPES } from '../constants'
import type { SavedTemplate } from './types'

// Seed data standing in for the future MigrationTemplate list API (see plan.md).
// Timestamps are relative to a fixed reference point rather than `Date.now()` so the
// "last used X days ago" copy in TemplateCard stays stable across renders/tests.
const NOW = new Date('2026-07-16T00:00:00Z').getTime()
const daysAgo = (n: number) => new Date(NOW - n * 24 * 60 * 60 * 1000).toISOString()

export const mockSavedTemplates: SavedTemplate[] = [
  {
    name: 'production-rhel-east',
    displayName: 'Production RHEL · East',
    description:
      'Standard hot migration for east-region RHEL web & app tiers. Admin-gated cutover, Ceph NVMe storage.',
    createdAt: daysAgo(126),
    timesUsed: 28,
    lastUsedAt: daysAgo(2),
    sourceVCenter: 'vcenter-east.example.com',
    destination: 'pcd-east-1',
    tenantProject: 'platform-ops',
    targetCluster: 'cluster-prod-a',
    networkMappings: [
      { source: 'vmnet-prod', target: 'net-prod-east-a' },
      { source: 'vmnet-data', target: 'net-data-east' }
    ],
    storageMappings: [{ source: 'east-nvme-ds01', target: 'ceph-nvme-east' }],
    dataCopyMethod: 'hot',
    cutoverOption: CUTOVER_TYPES.ADMIN_INITIATED,
    vmwareCluster: 'vcenter-east.example.com:DC-East:cluster-prod-a',
    pcdCluster: 'cluster-prod-a',
    osFamily: 'linuxGuest'
  },
  {
    name: 'web-tier-west-region',
    displayName: 'Web tier · West region',
    description:
      'West-region front-end fleet onto pcd-west-1. Hot copy with admin cutover and post-migration smoke test.',
    createdAt: daysAgo(90),
    timesUsed: 17,
    lastUsedAt: daysAgo(3),
    sourceVCenter: 'vcenter-west.example.com',
    destination: 'pcd-west-1',
    tenantProject: 'web-services',
    targetCluster: 'cluster-prod-a',
    networkMappings: [{ source: 'vmnet-web', target: 'net-web-west-a' }],
    storageMappings: [{ source: 'west-ssd-ds02', target: 'ceph-ssd-west' }],
    dataCopyMethod: 'hot',
    cutoverOption: CUTOVER_TYPES.ADMIN_INITIATED,
    vmwareCluster: 'vcenter-west.example.com:DC-West:cluster-prod-a',
    pcdCluster: 'cluster-prod-a',
    osFamily: 'linuxGuest'
  },
  {
    name: 'cold-bulk-archive',
    displayName: 'Cold bulk archive',
    description:
      'Power-off cold copy for decommissioned and low-priority VMs. Cuts over automatically once data lands.',
    createdAt: daysAgo(60),
    timesUsed: 9,
    lastUsedAt: daysAgo(10),
    sourceVCenter: 'vcenter-east.example.com',
    destination: 'pcd-east-1',
    tenantProject: 'data-platform',
    targetCluster: 'cluster-prod-b',
    networkMappings: [{ source: 'vmnet-batch', target: 'net-batch-east' }],
    storageMappings: [{ source: 'east-bulk-ds03', target: 'ceph-bulk-east' }],
    dataCopyMethod: 'cold',
    cutoverOption: CUTOVER_TYPES.IMMEDIATE,
    vmwareCluster: 'vcenter-east.example.com:DC-East:cluster-prod-b',
    pcdCluster: 'cluster-prod-b',
    osFamily: 'linuxGuest'
  },
  {
    name: 'dev-sandbox-dry-run',
    displayName: 'Dev sandbox dry-run',
    description:
      'Mock migration that never touches the source VM. Used to validate mappings before a real cutover.',
    createdAt: daysAgo(45),
    timesUsed: 12,
    lastUsedAt: daysAgo(6),
    sourceVCenter: 'vcenter-east.example.com',
    destination: 'pcd-east-1',
    tenantProject: 'dev-sandbox',
    targetCluster: 'cluster-prod-a',
    networkMappings: [{ source: 'vmnet-dev', target: 'net-dev-east' }],
    storageMappings: [{ source: 'east-dev-ds04', target: 'ceph-dev-east' }],
    dataCopyMethod: 'mock',
    cutoverOption: CUTOVER_TYPES.ADMIN_INITIATED,
    vmwareCluster: 'vcenter-east.example.com:DC-East:cluster-prod-a',
    pcdCluster: 'cluster-prod-a',
    osFamily: 'linuxGuest'
  },
  {
    name: 'my-draft-config',
    displayName: 'My draft config',
    description: 'Work-in-progress mapping set for the upcoming finance DB migration wave.',
    createdAt: daysAgo(4),
    timesUsed: 0,
    sourceVCenter: 'vcenter-east.example.com',
    destination: 'pcd-east-1',
    tenantProject: 'finance',
    targetCluster: 'cluster-prod-b',
    networkMappings: [{ source: 'vmnet-fin', target: 'net-fin-east' }],
    storageMappings: [{ source: 'east-fin-ds05', target: 'ceph-fin-east' }],
    dataCopyMethod: 'cold',
    cutoverOption: CUTOVER_TYPES.TIME_WINDOW,
    vmwareCluster: 'vcenter-east.example.com:DC-East:cluster-prod-b',
    pcdCluster: 'cluster-prod-b',
    osFamily: 'windowsGuest'
  },
  {
    name: 'gpu-batch-cluster',
    displayName: 'GPU batch cluster',
    description:
      'Personal template for GPU-flavored batch worker migrations, not yet shared with the team.',
    createdAt: daysAgo(1),
    timesUsed: 1,
    lastUsedAt: daysAgo(1),
    sourceVCenter: 'vcenter-west.example.com',
    destination: 'pcd-west-1',
    tenantProject: 'ml-platform',
    targetCluster: 'cluster-gpu-a',
    networkMappings: [{ source: 'vmnet-gpu', target: 'net-gpu-west' }],
    storageMappings: [{ source: 'west-gpu-ds06', target: 'ceph-gpu-west' }],
    dataCopyMethod: 'hot',
    cutoverOption: CUTOVER_TYPES.ADMIN_INITIATED,
    vmwareCluster: 'vcenter-west.example.com:DC-West:cluster-gpu-a',
    pcdCluster: 'cluster-gpu-a',
    osFamily: 'linuxGuest',
    useGPU: true
  }
]
