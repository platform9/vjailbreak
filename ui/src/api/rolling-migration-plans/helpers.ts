import { v4 as uuidv4 } from "uuid"
import {
  NetworkMapping,
  ResourceRef,
  StorageMapping,
  VMSequence,
  MigrationStrategy,
} from "./model"

interface CreateRollingMigrationPlanParams {
  name?: string
  clusterName: string
  vms: VMSequence[]
  vmwareCredsRef: ResourceRef
  openstackCredsRef: ResourceRef
  bmConfigRef: ResourceRef
  networkMappings?: NetworkMapping[]
  storageMappings?: StorageMapping[]
  advancedOptions?: Record<string, unknown>
  firstBootScript?: string
  migrationStrategy?: MigrationStrategy
  migrationTemplate?: string
  cloudInitConfigRef?: ResourceRef
  namespace?: string
}

export const createRollingMigrationPlanJson = (
  params: CreateRollingMigrationPlanParams
) => {
  const {
    name,
    clusterName,
    vms,
    vmwareCredsRef,
    openstackCredsRef,
    bmConfigRef,
    networkMappings,
    storageMappings,
    advancedOptions,
    firstBootScript,
    migrationStrategy,
    migrationTemplate,
    cloudInitConfigRef,
    namespace,
  } = params || {}

  return {
    apiVersion: "vjailbreak.k8s.pf9.io/v1alpha1",
    kind: "RollingMigrationPlan",
    metadata: {
      name: name || `rolling-migration-plan-${uuidv4().substring(0, 8)}`,
      namespace: namespace,
      labels: {
        "app.kubernetes.io/name": "migration",
        "app.kubernetes.io/part-of": "vjailbreak",
      },
    },
    spec: {
      clusterSequence: [
        {
          clusterName,
          vmSequence: vms,
        },
      ],
      vmwareCredsRef,
      openstackCredsRef,
      bmConfigRef,
      ...(networkMappings && networkMappings.length > 0 && { networkMappings }),
      ...(storageMappings && storageMappings.length > 0 && { storageMappings }),
      ...(advancedOptions && { advancedOptions }),
      ...(firstBootScript && { firstBootScript }),
      ...(migrationStrategy && { migrationStrategy }),
      ...(migrationTemplate && { migrationTemplate }),
      ...(cloudInitConfigRef && { cloudInitConfigRef }),
    },
  }
}
