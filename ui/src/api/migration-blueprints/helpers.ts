import { MigrationBlueprintSpec } from './model'

export const createMigrationBlueprintJson = (name: string, spec: MigrationBlueprintSpec) => ({
  apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
  kind: 'MigrationBlueprint',
  metadata: {
    name
  },
  spec
})
