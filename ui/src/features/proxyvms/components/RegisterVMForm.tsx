import { Box } from '@mui/material'
import { Section } from 'src/components'
import { DesignSystemForm, RHFSelect } from 'src/shared/components/forms'
import type { UseFormReturn } from 'react-hook-form'
import type { Option } from 'src/shared/components/forms/rhf/RHFSelect'
import VMAutocomplete from './VMAutocomplete'
import SSHAccessSection from './SSHAccessSection'
import type { SelectFormData, VMOption, SSHKeySource, GeneratedKey } from './types'

export const SELECT_FORM_ID = 'add-proxy-vm-form'

interface RegisterVMFormProps {
  form: UseFormReturn<SelectFormData>
  onSubmit: (data: SelectFormData) => void
  open: boolean
  onClose: () => void
  isSubmitDisabled: boolean
  credOptions: Option[]
  vmwareCredsRefSelect: string
  vmOptions: VMOption[]
  vmsLoading: boolean
  selectedVM: VMOption | null
  onVMChange: (vm: VMOption | null) => void
  isSubmitting: boolean
  sshKeySource: SSHKeySource
  onSshKeySourceChange: (v: SSHKeySource) => void
  generatedKey: GeneratedKey | null
  isGenerating: boolean
  generateError: string | null
  onClearGenerateError: () => void
  copied: boolean
  onGenerate: () => void
  onRegenerateKey: () => void
  onCopy: () => void
  onKeyFileUpload: (file: File | null) => void
  onSubmitErrorChange: (e: string | null) => void
}

export default function RegisterVMForm({
  form,
  onSubmit,
  open,
  onClose,
  isSubmitDisabled,
  credOptions,
  vmwareCredsRefSelect,
  vmOptions,
  vmsLoading,
  selectedVM,
  onVMChange,
  isSubmitting,
  sshKeySource,
  onSshKeySourceChange,
  generatedKey,
  isGenerating,
  generateError,
  onClearGenerateError,
  copied,
  onGenerate,
  onRegenerateKey,
  onCopy,
  onKeyFileUpload,
  onSubmitErrorChange
}: RegisterVMFormProps) {
  return (
    <DesignSystemForm
      id={SELECT_FORM_ID}
      form={form}
      onSubmit={onSubmit}
      keyboardSubmitProps={{ open, onClose, isSubmitDisabled }}
    >
      <Box sx={{ display: 'grid', gap: 2 }}>
        <Section>
          <RHFSelect
            name="vmwareCredsRef"
            label="VMware Credentials"
            options={credOptions}
            rules={{ required: 'VMware credentials are required' }}
            placeholder={
              credOptions.length === 0
                ? 'No validated VMware credentials found'
                : 'Select credentials'
            }
          />
        </Section>

        <Section>
          <VMAutocomplete
            options={vmOptions}
            loading={vmsLoading}
            disabled={!vmwareCredsRefSelect || isSubmitting}
            value={selectedVM}
            onChange={onVMChange}
            credSelected={Boolean(vmwareCredsRefSelect)}
          />
        </Section>

        <SSHAccessSection
          sshKeySource={sshKeySource}
          onSshKeySourceChange={onSshKeySourceChange}
          generatedKey={generatedKey}
          isGenerating={isGenerating}
          generateError={generateError}
          onClearGenerateError={onClearGenerateError}
          vmSelected={Boolean(selectedVM)}
          isSubmitting={isSubmitting}
          copied={copied}
          onGenerate={onGenerate}
          onRegenerateKey={onRegenerateKey}
          onCopy={onCopy}
          onKeyFileUpload={onKeyFileUpload}
          onSubmitErrorChange={onSubmitErrorChange}
        />
      </Box>
    </DesignSystemForm>
  )
}
