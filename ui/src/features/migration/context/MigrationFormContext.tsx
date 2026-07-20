import { createContext, useContext } from 'react'
import type { SavedTemplate } from '../api/migration-blueprints/types'

type MigrationFormType = 'standard' | 'rolling'

export interface RetryMigrationConfig {
  migrationName: string
  namespace: string
  planName: string
  vmName: string
}

export interface MigrationFormContextValue {
  openMigrationForm: (
    type: MigrationFormType,
    retryConfig?: RetryMigrationConfig,
    templatePrefill?: SavedTemplate
  ) => void
}

export const MigrationFormContext = createContext<MigrationFormContextValue | undefined>(undefined)

export const useMigrationFormActions = () => {
  const context = useContext(MigrationFormContext)

  if (!context) {
    return {
      openMigrationForm: () => {
        if (process.env.NODE_ENV !== 'production') {
          // eslint-disable-next-line no-console
          console.warn('MigrationFormContext provider missing; openMigrationForm call ignored.')
        }
      }
    }
  }

  return context
}
