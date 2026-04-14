import { useEffect, useState } from 'react'
import axios from 'axios'
import type { OpenstackCreds } from 'src/api/openstack-creds/model'
import { getOpenstackCredentials } from 'src/api/openstack-creds/openstackCreds'
import type { VMwareCreds } from 'src/api/vmware-creds/model'
import { getVmwareCredentials } from 'src/api/vmware-creds/vmwareCreds'
import type { FormValues } from 'src/features/migration/types'
import { isNilOrEmpty } from 'src/utils'

export function useExistingVmwareCredentials({
  params,
  getFieldErrorsUpdater
}: {
  params: FormValues
  getFieldErrorsUpdater: (key: string | number) => (value: string) => void
}) {
  const [vmwareCredentials, setVmwareCredentials] = useState<VMwareCreds | undefined>(undefined)

  useEffect(() => {
    const fetchCredentials = async () => {
      if (!params.vmwareCreds || !params.vmwareCreds.existingCredName) return

      try {
        const existingCredName = params.vmwareCreds.existingCredName
        const response = await getVmwareCredentials(existingCredName)
        setVmwareCredentials(response)
      } catch (error) {
        console.error('Error fetching existing VMware credentials:', error)
        getFieldErrorsUpdater('vmwareCreds')(
          'Error fetching VMware credentials: ' +
            (axios.isAxiosError(error) ? error?.response?.data?.message : error)
        )
      }
    }

    if (isNilOrEmpty(params.vmwareCreds)) return

    setVmwareCredentials(undefined)
    getFieldErrorsUpdater('vmwareCreds')('')
    fetchCredentials()
  }, [params.vmwareCreds, getFieldErrorsUpdater])

  return { vmwareCredentials, setVmwareCredentials }
}

export function useExistingOpenstackCredentials({
  params,
  getFieldErrorsUpdater
}: {
  params: FormValues
  getFieldErrorsUpdater: (key: string | number) => (value: string) => void
}) {
  const [openstackCredentials, setOpenstackCredentials] = useState<OpenstackCreds | undefined>(
    undefined
  )

  useEffect(() => {
    const fetchCredentials = async () => {
      if (!params.openstackCreds || !params.openstackCreds.existingCredName) return

      try {
        const existingCredName = params.openstackCreds.existingCredName
        const response = await getOpenstackCredentials(existingCredName)
        setOpenstackCredentials(response)
      } catch (error) {
        console.error('Error fetching existing OpenStack credentials:', error)
        getFieldErrorsUpdater('openstackCreds')(
          'Error fetching PCD credentials: ' +
            (axios.isAxiosError(error) ? error?.response?.data?.message : error)
        )
      }
    }

    if (isNilOrEmpty(params.openstackCreds)) return

    setOpenstackCredentials(undefined)
    getFieldErrorsUpdater('openstackCreds')('')
    fetchCredentials()
  }, [params.openstackCreds, getFieldErrorsUpdater])

  return { openstackCredentials, setOpenstackCredentials }
}
