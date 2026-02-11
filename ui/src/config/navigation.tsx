import { NavigationItem } from '../types/navigation'
import MigrationIcon from '@mui/icons-material/SwapHoriz'
import AgentsIcon from '@mui/icons-material/Computer'
import CredentialsIcon from '@mui/icons-material/VpnKey'
import ConfigIcon from '@mui/icons-material/Settings'
import MonitoringIcon from '@mui/icons-material/Insights'
import DescriptionIcon from '@mui/icons-material/Description'
import { Storage } from '@mui/icons-material'

export const navigationItems: NavigationItem[] = [
  {
    id: 'migration',
    label: 'Migration',
    path: '/dashboard/migrations',
    icon: <MigrationIcon />,
    children: [
      {
        id: 'migrations',
        label: 'Migration',
        path: '/dashboard/migrations'
      },
      {
        id: 'cluster-conversions',
        label: 'Cluster Conversion',
        path: '/dashboard/cluster-conversions'
      }
    ]
  },
  {
    id: 'credentials-group',
    label: 'Credentials',
    path: '/dashboard/credentials',
    icon: <CredentialsIcon />,
    children: [
      {
        id: 'vm-credentials',
        label: 'VMware Credentials',
        path: '/dashboard/credentials/vm'
      },
      {
        id: 'pcd-credentials',
        label: 'PCD Credentials',
        path: '/dashboard/credentials/pcd'
      },
      {
        id: 'array-credentials',
        label: 'Array Credentials',
        path: '/dashboard/storage-management'
      }
    ]
  },
  {
    id: 'agents',
    label: 'Agents',
    path: '/dashboard/agents',
    icon: <AgentsIcon />
  },
  {
    id: 'settings',
    label: 'Settings',
    path: '/dashboard/global-settings',
    icon: <ConfigIcon />,
    children: [
      {
        id: 'baremetal-config',
        label: 'Bare Metal Config',
        path: '/dashboard/baremetal-config',
        icon: <Storage />
      },
      {
        id: 'global-settings',
        label: 'Global Settings',
        path: '/dashboard/global-settings'
      }
    ]
  },
  {
    id: 'monitoring',
    label: 'Monitoring',
    path: '/grafana',
    icon: <MonitoringIcon />,
    external: true
  },
  {
    id: 'docs',
    label: 'Documentation',
    path: '/docs',
    icon: <DescriptionIcon />,
    external: true,
    externalUrl: 'https://platform9.github.io/vjailbreak/introduction/getting_started/'
  }
]

const findNavigationItem = (
  items: NavigationItem[],
  predicate: (item: NavigationItem) => boolean
): NavigationItem | undefined => {
  for (const item of items) {
    if (predicate(item)) return item
    if (item.children?.length) {
      const found = findNavigationItem(item.children, predicate)
      if (found) return found
    }
  }
  return undefined
}

export const getNavigationItemById = (id: string): NavigationItem | undefined => {
  return findNavigationItem(navigationItems, (item) => item.id === id)
}

export const getNavigationItemByPath = (path: string): NavigationItem | undefined => {
  return findNavigationItem(navigationItems, (item) => item.path === path)
}
