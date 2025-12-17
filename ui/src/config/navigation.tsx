import { NavigationItem } from '../types/navigation'
import MigrationIcon from '@mui/icons-material/SwapHoriz'
import AgentsIcon from '@mui/icons-material/Computer'
import CredentialsIcon from '@mui/icons-material/VpnKey'
import ClusterIcon from '@mui/icons-material/Hub'
import ConfigIcon from '@mui/icons-material/Settings'
import MonitoringIcon from '@mui/icons-material/Analytics'
import StorageIcon from '@mui/icons-material/Storage'

export const navigationItems: NavigationItem[] = [
  {
    id: 'migrations',
    label: 'Migrations',
    path: '/dashboard/migrations',
    icon: <MigrationIcon />
  },
  {
    id: 'cluster-conversions',
    label: 'Cluster Conversions',
    path: '/dashboard/cluster-conversions',
    icon: <ClusterIcon />
  },
  {
    id: 'credentials',
    label: 'Credentials',
    path: '/dashboard/credentials',
    icon: <CredentialsIcon />
  },
  {
    id: 'storage-management',
    label: 'Storage Management',
    path: '/dashboard/storage-management',
    icon: <StorageIcon />
  },
  {
    id: 'agents',
    label: 'Agents',
    path: '/dashboard/agents',
    icon: <AgentsIcon />
  },
  {
    id: 'baremetal-config',
    label: 'Bare Metal Config',
    path: '/dashboard/baremetal-config',
    icon: <ConfigIcon />
  },
  {
    id: 'monitoring',
    label: 'Monitoring',
    path: '/grafana',
    icon: <MonitoringIcon />,
    external: true
  }
]

export const getNavigationItemById = (id: string): NavigationItem | undefined => {
  return navigationItems.find(item => item.id === id)
}

export const getNavigationItemByPath = (path: string): NavigationItem | undefined => {
  return navigationItems.find(item => item.path === path)
}