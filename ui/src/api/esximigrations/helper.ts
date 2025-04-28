import { ESXIMigration } from "./model"

export const getESXiName = (esxiMigration: ESXIMigration): string => {
  return esxiMigration?.spec?.esxiName || ""
}

export const getOpenstackCredsRefName = (
  esxiMigration: ESXIMigration
): string => {
  return esxiMigration?.spec?.openstackCredsRef?.name || ""
}

export const getVMwareCredsRefName = (esxiMigration: ESXIMigration): string => {
  return esxiMigration?.spec?.vmwareCredsRef?.name || ""
}

export const getRollingMigrationPlanRefName = (
  esxiMigration: ESXIMigration
): string => {
  return esxiMigration?.spec?.rollingMigrationPlanRef?.name || ""
}
