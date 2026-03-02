import { createContext, useContext } from 'react'

type MigrationFormType = 'standard' | 'rolling'

export interface MigrationFormContextValue {
  openMigrationForm: (type: MigrationFormType) => void
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
