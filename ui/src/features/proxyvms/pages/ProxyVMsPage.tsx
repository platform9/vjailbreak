import { useState } from 'react'
import { Box, Button } from '@mui/material'
import AddIcon from '@mui/icons-material/Add'
import DnsIcon from '@mui/icons-material/Dns'
import { ListingToolbar } from 'src/components/grid'
import { useProxyVMsQuery } from 'src/hooks/api/useProxyVMsQuery'
import AddProxyVMDrawer from '../components/AddProxyVMDrawer'
import ProxyVMsTable from '../components/ProxyVMsTable'

export default function ProxyVMsPage() {
  const [addDrawerOpen, setAddDrawerOpen] = useState(false)
  const { data: proxyVMs = [], isLoading } = useProxyVMsQuery()

  const toolbar = (
    <ListingToolbar
      title="Proxy VMs"
      icon={<DnsIcon />}
      actions={
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
          <Button
            variant="contained"
            startIcon={<AddIcon />}
            onClick={() => setAddDrawerOpen(true)}
            sx={{ height: 40 }}
          >
            Add Proxy VM
          </Button>
        </Box>
      }
    />
  )

  return (
    <Box sx={{ height: '100%', width: '100%', overflow: 'hidden' }}>
      <ProxyVMsTable proxyVMs={proxyVMs} loading={isLoading} toolbar={toolbar} />

      <AddProxyVMDrawer
        open={addDrawerOpen}
        onClose={() => setAddDrawerOpen(false)}
      />
    </Box>
  )
}
