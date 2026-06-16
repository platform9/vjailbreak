import { Box, MenuItem, Select, FormControl, InputLabel, Typography } from '@mui/material'
import { KeyValueGrid } from 'src/components'

interface RetrySourceDestinationSummaryProps {
  vmwareCredName?: string
  datacenter?: string
  sourceCluster?: string
  openstackCredName?: string
  pcdClusters: Array<{ id: string; name?: string }>
  selectedPcdClusterId: string
  onPcdClusterChange: (id: string) => void
  disabled?: boolean
}

// Read-only source environment + editable target cluster for retry mode.
// Credentials, datacenter, and source cluster are locked; only target cluster
// can change (which cascades a mapping reset in the parent form).
export function RetrySourceDestinationSummary({
  vmwareCredName,
  datacenter,
  sourceCluster,
  openstackCredName,
  pcdClusters,
  selectedPcdClusterId,
  onPcdClusterChange,
  disabled
}: RetrySourceDestinationSummaryProps) {
  // pcdClusters may not have loaded yet when prefill stored the cluster name
  // instead of the id. Resolve by id first, fall back to name match.
  const resolvedClusterId =
    pcdClusters.find((c) => c.id === selectedPcdClusterId)?.id ||
    pcdClusters.find((c) => c.name === selectedPcdClusterId)?.id ||
    selectedPcdClusterId

  return (
    <Box data-testid="retry-source-destination-summary" sx={{ display: 'grid', gap: 2 }}>
      <Box sx={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 2 }}>
        <Box sx={{ display: 'grid', gap: 1.5 }}>
          <Typography variant="subtitle2" color="text.secondary">
            VMware source
          </Typography>
          <KeyValueGrid
            items={[
              { label: 'VMware credentials', value: vmwareCredName || '—' },
              { label: 'Datacenter', value: datacenter || '—' },
              { label: 'Source cluster', value: sourceCluster || '—' }
            ]}
          />
        </Box>

        <Box sx={{ display: 'grid', gap: 1.5 }}>
          <Typography variant="subtitle2" color="text.secondary">
            PCD destination
          </Typography>
          <KeyValueGrid
            items={[{ label: 'PCD credentials', value: openstackCredName || '—' }]}
          />
          <FormControl size="small" disabled={disabled} sx={{ maxWidth: 360 }}>
            <InputLabel id="retry-target-cluster-label">Target cluster</InputLabel>
            <Select
              labelId="retry-target-cluster-label"
              label="Target cluster"
              value={resolvedClusterId}
              onChange={(e) => onPcdClusterChange(e.target.value)}
              inputProps={{ 'data-testid': 'retry-target-cluster-select' }}
            >
              {pcdClusters.map((c) => (
                <MenuItem key={c.id} value={c.id}>
                  {c.name || c.id}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
        </Box>
      </Box>

      <Typography variant="caption" color="text.secondary">
        Credentials, datacenter, and source cluster are locked while retrying a migration.
        Changing the target cluster will reset network and storage mappings.
      </Typography>
    </Box>
  )
}
