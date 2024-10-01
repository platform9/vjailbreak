export const getVmwareCredsList = () => ({
  apiVersion: "vjailbreak.k8s.pf9.io/v1alpha1",
  items: [
    {
      apiVersion: "vjailbreak.k8s.pf9.io/v1alpha1",
      kind: "VMwareCreds",
      metadata: {
        annotations: {
          "kubectl.kubernetes.io/last-applied-configuration":
            '{"apiVersion":"vjailbreak.k8s.pf9.io/v1alpha1","kind":"VMwareCreds","metadata":{"name":"badvmwarecreds","namespace":"migration-system"},"spec":{"VCENTER_HOST":"vcenter.phx.pnap.platform9.horse","VCENTER_INSECURE":true,"VCENTER_PASSWORD":"badcreds","VCENTER_USERNAME":"tanay@jumpcloud.com"},"status":{"vmwareValidationMessage":"Error validating VMwareCreds \'pnapbmc1\': failed to login: ServerFaultCode: Cannot complete login due to an incorrect user name or password.","vmwareValidationStatus":"Failed"}}\n',
        },
        creationTimestamp: "2024-09-23T18:06:37Z",
        generation: 1,
        managedFields: [
          {
            apiVersion: "vjailbreak.k8s.pf9.io/v1alpha1",
            fieldsType: "FieldsV1",
            fieldsV1: {
              "f:metadata": {
                "f:annotations": {
                  "f:kubectl.kubernetes.io/last-applied-configuration": {},
                },
              },
            },
            manager: "kubectl-last-applied",
            operation: "Apply",
          },
          {
            apiVersion: "vjailbreak.k8s.pf9.io/v1alpha1",
            fieldsType: "FieldsV1",
            fieldsV1: {
              "f:spec": {
                "f:VCENTER_HOST": {},
                "f:VCENTER_INSECURE": {},
                "f:VCENTER_PASSWORD": {},
                "f:VCENTER_USERNAME": {},
              },
            },
            manager: "kubectl",
            operation: "Apply",
            time: "2024-09-23T18:13:05Z",
          },
          {
            apiVersion: "vjailbreak.k8s.pf9.io/v1alpha1",
            fieldsType: "FieldsV1",
            fieldsV1: {
              "f:status": {
                ".": {},
                "f:vmwareValidationMessage": {},
                "f:vmwareValidationStatus": {},
              },
            },
            manager: "kubectl-edit",
            operation: "Update",
            subresource: "status",
            time: "2024-09-23T18:31:34Z",
          },
        ],
        name: "badvmwarecreds",
        namespace: "migration-system",
        resourceVersion: "1002136",
        uid: "707206a2-b8d4-4391-b259-51a2ea0bad9d",
      },
      spec: {
        VCENTER_HOST: "vcenter.phx.pnap.platform9.horse",
        VCENTER_INSECURE: true,
        VCENTER_PASSWORD: "badcreds",
        VCENTER_USERNAME: "tanay@jumpcloud.com",
      },
      status: {
        vmwareValidationMessage:
          "Error validating VMwareCreds 'pnapbmc1': failed to login: ServerFaultCode: Cannot complete login due to an incorrect user name or password.",
        vmwareValidationStatus: "Failed",
      },
    },
    {
      apiVersion: "vjailbreak.k8s.pf9.io/v1alpha1",
      kind: "VMwareCreds",
      metadata: {
        annotations: {
          "kubectl.kubernetes.io/last-applied-configuration":
            '{"apiVersion":"vjailbreak.k8s.pf9.io/v1alpha1","kind":"VMwareCreds","metadata":{"name":"goodvmwarecreds","namespace":"migration-system"},"spec":{"VCENTER_HOST":"vcenter.phx.pnap.platform9.horse","VCENTER_INSECURE":true,"VCENTER_PASSWORD":"Password456","VCENTER_USERNAME":"tanay@jumpcloud.com"},"status":{"vmwareValidationMessage":"Successfully authenticated to VMware","vmwareValidationStatus":"Succeeded"}}\n',
        },
        creationTimestamp: "2024-09-23T18:06:37Z",
        generation: 1,
        managedFields: [
          {
            apiVersion: "vjailbreak.k8s.pf9.io/v1alpha1",
            fieldsType: "FieldsV1",
            fieldsV1: {
              "f:metadata": {
                "f:annotations": {
                  "f:kubectl.kubernetes.io/last-applied-configuration": {},
                },
              },
            },
            manager: "kubectl-last-applied",
            operation: "Apply",
          },
          {
            apiVersion: "vjailbreak.k8s.pf9.io/v1alpha1",
            fieldsType: "FieldsV1",
            fieldsV1: {
              "f:spec": {
                "f:VCENTER_HOST": {},
                "f:VCENTER_INSECURE": {},
                "f:VCENTER_PASSWORD": {},
                "f:VCENTER_USERNAME": {},
              },
            },
            manager: "kubectl",
            operation: "Apply",
            time: "2024-09-23T18:13:05Z",
          },
          {
            apiVersion: "vjailbreak.k8s.pf9.io/v1alpha1",
            fieldsType: "FieldsV1",
            fieldsV1: {
              "f:status": {
                ".": {},
                "f:vmwareValidationMessage": {},
                "f:vmwareValidationStatus": {},
              },
            },
            manager: "kubectl-edit",
            operation: "Update",
            subresource: "status",
            time: "2024-09-23T18:31:14Z",
          },
        ],
        name: "goodvmwarecreds",
        namespace: "migration-system",
        resourceVersion: "1002077",
        uid: "e1424b7e-13a2-45e5-b76b-79c662d65e4c",
      },
      spec: {
        VCENTER_HOST: "vcenter.phx.pnap.platform9.horse",
        VCENTER_INSECURE: true,
        VCENTER_PASSWORD: "Password456",
        VCENTER_USERNAME: "tanay@jumpcloud.com",
      },
      status: {
        vmwareValidationMessage: "Successfully authenticated to VMware",
        vmwareValidationStatus: "Succeeded",
      },
    },
  ],
  kind: "VMwareCredsList",
  metadata: {
    continue: "",
    resourceVersion: "1071359",
  },
})

export const getVmwareCred = (good = true) => {
  const allCreds = getVmwareCredsList()?.items
  return good ? allCreds[1] : allCreds[0]
}

export const getOpenstackCredsList = () => ({
  apiVersion: "vjailbreak.k8s.pf9.io/v1alpha1",
  items: [
    {
      apiVersion: "vjailbreak.k8s.pf9.io/v1alpha1",
      kind: "OpenstackCreds",
      metadata: {
        annotations: {
          "kubectl.kubernetes.io/last-applied-configuration":
            '{"apiVersion":"vjailbreak.k8s.pf9.io/v1alpha1","kind":"OpenstackCreds","metadata":{"name":"badopenstackcreds","namespace":"migration-system"},"spec":{"OS_AUTH_URL":"https://sa-pmo-cspmo.platform9.horse/keystone/v3","OS_DOMAIN_NAME":"Default","OS_PASSWORD":"badcreds","OS_REGION_NAME":"cspmo","OS_TENANT_NAME":"service","OS_USERNAME":"tanay@platform9.com"},"status":{"openstackValidationMessage":"Error validating OpenstackCreds \'sapmo1\'","openstackValidationStatus":"Failed"}}\n',
        },
        creationTimestamp: "2024-09-23T18:06:37Z",
        generation: 1,
        managedFields: [
          {
            apiVersion: "vjailbreak.k8s.pf9.io/v1alpha1",
            fieldsType: "FieldsV1",
            fieldsV1: {
              "f:metadata": {
                "f:annotations": {
                  "f:kubectl.kubernetes.io/last-applied-configuration": {},
                },
              },
            },
            manager: "kubectl-last-applied",
            operation: "Apply",
          },
          {
            apiVersion: "vjailbreak.k8s.pf9.io/v1alpha1",
            fieldsType: "FieldsV1",
            fieldsV1: {
              "f:spec": {
                "f:OS_AUTH_URL": {},
                "f:OS_DOMAIN_NAME": {},
                "f:OS_PASSWORD": {},
                "f:OS_REGION_NAME": {},
                "f:OS_TENANT_NAME": {},
                "f:OS_USERNAME": {},
              },
            },
            manager: "kubectl",
            operation: "Apply",
            time: "2024-09-23T18:13:05Z",
          },
          {
            apiVersion: "vjailbreak.k8s.pf9.io/v1alpha1",
            fieldsType: "FieldsV1",
            fieldsV1: {
              "f:status": {
                ".": {},
                "f:openstackValidationMessage": {},
                "f:openstackValidationStatus": {},
              },
            },
            manager: "kubectl-edit",
            operation: "Update",
            subresource: "status",
            time: "2024-09-23T18:31:02Z",
          },
        ],
        name: "badopenstackcreds",
        namespace: "migration-system",
        resourceVersion: "1002039",
        uid: "8781cd70-6bf8-46c6-922e-bcace532fb6d",
      },
      spec: {
        OS_AUTH_URL: "https://sa-pmo-cspmo.platform9.horse/keystone/v3",
        OS_DOMAIN_NAME: "Default",
        OS_PASSWORD: "badcreds",
        OS_REGION_NAME: "cspmo",
        OS_TENANT_NAME: "service",
        OS_USERNAME: "tanay@platform9.com",
      },
      status: {
        openstackValidationMessage: "Error validating OpenstackCreds 'sapmo1'",
        openstackValidationStatus: "Failed",
      },
    },
    {
      apiVersion: "vjailbreak.k8s.pf9.io/v1alpha1",
      kind: "OpenstackCreds",
      metadata: {
        annotations: {
          "kubectl.kubernetes.io/last-applied-configuration":
            '{"apiVersion":"vjailbreak.k8s.pf9.io/v1alpha1","kind":"OpenstackCreds","metadata":{"name":"goodopenstackcreds","namespace":"migration-system"},"spec":{"OS_AUTH_URL":"https://sa-pmo-cspmo.platform9.horse/keystone/v3","OS_DOMAIN_NAME":"Default","OS_PASSWORD":"Password123","OS_REGION_NAME":"cspmo","OS_TENANT_NAME":"service","OS_USERNAME":"tanay@platform9.com"},"status":{"openstackValidationMessage":"Successfully authenticated to Openstack","openstackValidationStatus":"Succeeded"}}\n',
        },
        creationTimestamp: "2024-09-23T18:06:37Z",
        generation: 1,
        managedFields: [
          {
            apiVersion: "vjailbreak.k8s.pf9.io/v1alpha1",
            fieldsType: "FieldsV1",
            fieldsV1: {
              "f:metadata": {
                "f:annotations": {
                  "f:kubectl.kubernetes.io/last-applied-configuration": {},
                },
              },
            },
            manager: "kubectl-last-applied",
            operation: "Apply",
          },
          {
            apiVersion: "vjailbreak.k8s.pf9.io/v1alpha1",
            fieldsType: "FieldsV1",
            fieldsV1: {
              "f:spec": {
                "f:OS_AUTH_URL": {},
                "f:OS_DOMAIN_NAME": {},
                "f:OS_PASSWORD": {},
                "f:OS_REGION_NAME": {},
                "f:OS_TENANT_NAME": {},
                "f:OS_USERNAME": {},
              },
            },
            manager: "kubectl",
            operation: "Apply",
            time: "2024-09-23T18:13:04Z",
          },
          {
            apiVersion: "vjailbreak.k8s.pf9.io/v1alpha1",
            fieldsType: "FieldsV1",
            fieldsV1: {
              "f:status": {
                ".": {},
                "f:openstackValidationMessage": {},
                "f:openstackValidationStatus": {},
              },
            },
            manager: "kubectl-edit",
            operation: "Update",
            subresource: "status",
            time: "2024-09-23T18:30:46Z",
          },
        ],
        name: "goodopenstackcreds",
        namespace: "migration-system",
        resourceVersion: "1001987",
        uid: "83fabcfb-8664-4154-8ad8-9ce6405b9fc9",
      },
      spec: {
        OS_AUTH_URL: "https://sa-pmo-cspmo.platform9.horse/keystone/v3",
        OS_DOMAIN_NAME: "Default",
        OS_PASSWORD: "Password123",
        OS_REGION_NAME: "cspmo",
        OS_TENANT_NAME: "service",
        OS_USERNAME: "tanay@platform9.com",
      },
      status: {
        openstackValidationMessage: "Successfully authenticated to Openstack",
        openstackValidationStatus: "Succeeded",
      },
    },
  ],
  kind: "OpenstackCredsList",
  metadata: {
    continue: "",
    resourceVersion: "1084297",
  },
})

export const getOpenstackCred = (good = true) => {
  const allCreds = getOpenstackCredsList()?.items
  return good ? allCreds[1] : allCreds[0]
}

export const getMigrationTemplatesList = () => ({
  apiVersion: "vjailbreak.k8s.pf9.io/v1alpha1",
  items: [
    {
      apiVersion: "vjailbreak.k8s.pf9.io/v1alpha1",
      kind: "MigrationTemplate",
      metadata: {
        annotations: {
          "kubectl.kubernetes.io/last-applied-configuration":
            '{"apiVersion":"vjailbreak.k8s.pf9.io/v1alpha1","kind":"MigrationTemplate","metadata":{"name":"migrationtemplate","namespace":"migration-system"},"spec":{"destination":{"openstackRef":"goodopenstackcreds"},"networkMapping":"goodnetworkmapping","source":{"datacenter":"PNAP BMC","vmwareRef":"goodvmwarecreds"},"storageMapping":"goodstoragemapping"},"status":{"openstack":{"networks":["tenant-1","vlan3003","tenant-2","tenant-3","vlan3002"],"volumeTypes":["lvm"]},"vmware":[{"datastores":["vcenter-datastore-1"],"name":"rocky9.1-01","networks":["VM Network"]},{"datastores":["vcenter-datastore-1"],"name":"w198","networks":["VM Network"]},{"datastores":["vcenter-datastore-1","vcenter-datastore-1"],"name":"mig_test_cbt_bak-clone-24","networks":["VM Network"]},{"datastores":["vcenter-datastore-1","vcenter-datastore-1"],"name":"mig_test_cbt_bak-clone-23","networks":["VM Network"]},{"datastores":["vcenter-datastore-1","vcenter-datastore-1"],"name":"mig_test_cbt_bak-clone-22","networks":["VM Network"]},{"datastores":["vcenter-datastore-1","vcenter-datastore-1"],"name":"mig_test_cbt_bak-clone-20","networks":["VM Network"]},{"datastores":["vcenter-datastore-1","vcenter-datastore-1"],"name":"mig_test_cbt_bak-clone-27","networks":["VM Network"]},{"datastores":["vcenter-datastore-1","vcenter-datastore-1"],"name":"mig_test_cbt_bak-clone-26","networks":["VM Network"]},{"datastores":["vcenter-datastore-1","vcenter-datastore-1"],"name":"mig_test_cbt_bak-clone-18","networks":["VM Network"]},{"datastores":["vcenter-datastore-1","vcenter-datastore-1"],"name":"mig_test_cbt_bak-clone-15","networks":["VM Network"]},{"datastores":["vcenter-datastore-1","vcenter-datastore-1"],"name":"mig_test_cbt_bak-clone-13","networks":["VM Network"]},{"datastores":["vcenter-datastore-1","vcenter-datastore-1"],"name":"mig_test_cbt_bak-clone-12","networks":["VM Network"]},{"datastores":["vcenter-datastore-1"],"name":"mig_test_cbt_bak-clone-30","networks":["VM Network"]},{"datastores":["vcenter-datastore-1","vcenter-datastore-1"],"name":"mig_test_cbt_bak-clone-29","networks":["VM Network"]},{"datastores":["vcenter-datastore-1","vcenter-datastore-1"],"name":"mig_test_cbt_bak-clone-11"},{"datastores":["vcenter-datastore-1","vcenter-datastore-1"],"name":"mig_test_cbt_bak-clone-10","networks":["VM Network"]},{"datastores":["vcenter-datastore-1","vcenter-datastore-1"],"name":"mig_test_cbt_bak-clone-9","networks":["VM Network"]},{"datastores":["vcenter-datastore-1","vcenter-datastore-1"],"name":"mig_test_cbt_bak-clone-8","networks":["VM Network"]},{"datastores":["vcenter-datastore-1","vcenter-datastore-1"],"name":"esxi-host-02","networks":["VM Network"]},{"datastores":["vcenter-datastore-1"],"name":"Appliance","networks":["VM Network"]},{"datastores":["vcenter-datastore-1"],"name":"w2019-4","networks":["VM Network"]},{"datastores":["vcenter-datastore-1","vcenter-datastore-1"],"name":"mig_test_cbt_bak-clone-25","networks":["VM Network"]},{"datastores":["vcenter-datastore-1"],"name":"migration-demo","networks":["VM Network"]},{"datastores":["vcenter-datastore-1","vcenter-datastore-1"],"name":"mig_test_cbt_bak-clone-19","networks":["VM Network"]},{"datastores":["vcenter-datastore-1"],"name":"test-ubuntu-vmdk","networks":["VM Network"]},{"datastores":["vcenter-datastore-1","vcenter-datastore-1"],"name":"mig_test_cbt","networks":["VM Network"]},{"datastores":["vcenter-datastore-1"],"name":"w2019-5","networks":["VM Network"]},{"datastores":["vcenter-datastore-1","vcenter-datastore-1"],"name":"mig_test_cbt_bak-clone-16","networks":["VM Network"]},{"datastores":["vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1"],"name":"esxi-host-01","networks":["VM Network"]},{"datastores":["vcenter-datastore-1","vcenter-datastore-1"],"name":"mig_test_cbt_uefi","networks":["VM Network"]},{"datastores":["vcenter-datastore-1"],"name":"Vcenter-test-host02","networks":["VM Network","VM Network"]},{"datastores":["vcenter-datastore-1","vcenter-datastore-1"],"name":"esxi-host-03","networks":["VM Network"]},{"datastores":["vcenter-datastore-1"],"name":"thick-test","networks":["VM Network"]},{"datastores":["vcenter-datastore-1"],"name":"tapas-ubuntu-2204-kube-v1.27","networks":["VM Network"]},{"datastores":["vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1"],"name":"VMware vCenter Server opencloud","networks":["VM Network"]},{"datastores":["vcenter-datastore-1"],"name":"mig_test","networks":["VM Network"]},{"datastores":["vcenter-datastore-1","vcenter-datastore-1"],"name":"mig_test_cbt_bak-clone-6","networks":["VM Network"]},{"datastores":["vcenter-datastore-1"],"name":"New Virtual Machine","networks":["VM Network"]},{"datastores":["vcenter-datastore-1","vcenter-datastore-1"],"name":"mig_test_cbt_bak-clone-2","networks":["VM Network"]},{"datastores":["vcenter-datastore-1","vcenter-datastore-1"],"name":"mig_test_cbt_bak-clone-5","networks":["VM Network"]},{"datastores":["vcenter-datastore-1"],"name":"platform9-vmware-appliance-test","networks":["VM Network"]},{"datastores":["vcenter-datastore-1"],"name":"dev test","networks":["VM Network"]},{"datastores":["vcenter-datastore-1","vcenter-datastore-1"],"name":"winserver2k22","networks":["VM Network","VM Network 2"]},{"datastores":["vcenter-datastore-1"],"name":"AIO","networks":["VM Network"]},{"datastores":["vcenter-datastore-1","vcenter-datastore-1"],"name":"mig_test_cbt_bak-clone-28","networks":["VM Network"]},{"datastores":["vcenter-datastore-1","vcenter-datastore-1"],"name":"wintst_new","networks":["VM Network"]},{"datastores":["vcenter-datastore-1","vcenter-datastore-1"],"name":"wintest_uefi_new","networks":["VM Network"]},{"datastores":["vcenter-datastore-1","vcenter-datastore-1"],"name":"mig_test_cbt_bak","networks":["VM Network"]},{"datastores":["vcenter-datastore-1","vcenter-datastore-1"],"name":"mig_test_cbt_bak-clone-14","networks":["VM Network"]},{"datastores":["vcenter-datastore-1"],"name":"win10test","networks":["VM Network"]},{"datastores":["local01","local01"],"name":"cloned-nka-poc"},{"datastores":["vcenter-datastore-1"],"name":"openstack-lab","networks":["VM Network"]},{"datastores":["vcenter-datastore-1"],"name":"centos7-01","networks":["VM Network"]},{"datastores":["vcenter-datastore-1","vcenter-datastore-1"],"name":"mig_test_cbt_bak-clone-21","networks":["VM Network"]},{"datastores":["vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1"],"name":"esx-01-h-agent-vm","networks":["VM Network"]},{"datastores":["vcenter-datastore-1"],"name":"test-vmdk-2","networks":["VM Network"]},{"datastores":["vcenter-datastore-1","vcenter-datastore-1"],"name":"mig_test_cbt_bak-clone-4","networks":["VM Network"]},{"datastores":["vcenter-datastore-1","vcenter-datastore-1"],"name":"winserver2k19","networks":["VM Network","VM Network 2"]},{"datastores":["vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1"],"name":"vcenter.phx.pnap","networks":["VM Network"]},{"datastores":["vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1"],"name":"VMware vCenter Server pub ip","networks":["Public-Network"]},{"datastores":["local01"],"name":"testvm","networks":["VM Network"]},{"datastores":["vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1"],"name":"vCenter Server Appliance test","networks":["VM Network"]},{"datastores":["vcenter-datastore-1","vcenter-datastore-1"],"name":"mig_test_cbt_bak-clone-17","networks":["VM Network"]},{"datastores":["vcenter-datastore-1"],"name":"Vcenter-test-host01","networks":["VM Network","VM Network"]},{"datastores":["vcenter-datastore-1"],"name":"vcenter-pub-host1","networks":["Public-Network","VM Network"]},{"datastores":["vcenter-datastore-1","vcenter-datastore-1"],"name":"mig_test_cbt_bak-clone-0","networks":["VM Network"]},{"datastores":["vcenter-datastore-1"],"name":"vcenter-nfs-server","networks":["Public-Network","VM Network"]},{"datastores":["local01"],"name":"thick-test-2","networks":["VM Network"]},{"datastores":["vcenter-datastore-1"],"name":"win2022-test","networks":["VM Network"]},{"datastores":["vcenter-datastore-1","local01"],"name":"winserver2k16","networks":["VM Network"]},{"datastores":["vcenter-datastore-1"],"name":"w2019-3","networks":["VM Network"]},{"datastores":["vcenter-datastore-1","vcenter-datastore-1"],"name":"mig_test_cbt_bak-clone-1","networks":["VM Network"]},{"datastores":["vcenter-datastore-1","vcenter-datastore-1"],"name":"mig_test_cbt_bak-clone-3","networks":["VM Network"]},{"datastores":["vcenter-datastore-1"],"name":"nfs-server","networks":["VM Network"]},{"datastores":["vcenter-datastore-1","vcenter-datastore-1"],"name":"mig_test_cbt_bak-clone-7","networks":["VM Network"]},{"datastores":["vcenter-datastore-1"],"name":"win2k16","networks":["bcm-network"]},{"datastores":["vcenter-datastore-1"],"name":"niket-vm","networks":["VM Network"]},{"datastores":["vcenter-datastore-1"],"name":"niket-test","networks":["VM Network"]},{"datastores":["local01","local01","local01"],"name":"Nested_ESXi7","networks":["VM Network","VM Network"]},{"datastores":["vcenter-datastore-1"],"name":"w197","networks":["VM Network"]},{"datastores":["vcenter-datastore-1"],"name":"w196","networks":["VM Network"]},{"datastores":["local01","local01"],"name":"ufo1","networks":["VM Network"]},{"datastores":["vcenter-datastore-1"],"name":"node004","networks":["bcm-network"]},{"datastores":["vcenter-datastore-1"],"name":"node005","networks":["bcm-network"]},{"datastores":["vcenter-datastore-1"],"name":"node003","networks":["bcm-network"]},{"datastores":["vcenter-datastore-1"],"name":"node006","networks":["bcm-network"]},{"datastores":["vcenter-datastore-1"],"name":"test-multi-nic","networks":["VM Network","bcm-network"]},{"datastores":["local01"],"name":"w221","networks":["VM Network"]},{"datastores":["vcenter-datastore-1"],"name":"bcm-manager","networks":["bcm-network"]},{"datastores":["vcenter-datastore-1"],"name":"w195","networks":["VM Network"]},{"datastores":["vcenter-datastore-1"],"name":"nvidia-bcm-router","networks":["VM Network","VM Network","VM Network","VM Network","bcm-network","VM Network","VM Network","VM Network","VM Network","VM Network"]},{"datastores":["vcenter-datastore-1"],"name":"node001","networks":["bcm-network"]},{"datastores":["vcenter-datastore-1"],"name":"w194","networks":["VM Network"]},{"datastores":["local01","local01","local01"],"name":"hstax-agent-vm","networks":["VM Network"]},{"datastores":["vcenter-datastore-1"],"name":"node002","networks":["bcm-network"]},{"datastores":["local01"],"name":"coriolis-u2204","networks":["VM Network"]},{"datastores":["local01","local01"],"name":"ufo2","networks":["VM Network"]},{"datastores":["vcenter-datastore-1"],"name":"u2204-01","networks":["VM Network"]},{"datastores":["vcenter-datastore-1"],"name":"w2019-2","networks":["VM Network"]},{"datastores":["local01"],"name":"w2019-1","networks":["VM Network"]},{"datastores":["vcenter-datastore-1"],"name":"w191","networks":["VM Network"]},{"datastores":["local01"],"name":"testvm1","networks":["VM Network"]},{"datastores":["vcenter-datastore-1"],"name":"forklift-w2019-1","networks":["VM Network"]},{"datastores":["local01"],"name":"w192","networks":["VM Network"]},{"datastores":["vcenter-datastore-1"],"name":"w193","networks":["VM Network"]},{"datastores":["vcenter-datastore-1"],"name":"rparikh-test","networks":["VM Network"]},{"datastores":["vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1","vcenter-datastore-1"],"name":"VMware vCenter Server Appliance","networks":["VM Network"]},{"datastores":["vcenter-datastore-1"],"name":"suse11"},{"datastores":["vcenter-datastore-1"],"name":"Suse"},{"datastores":["local01","local01"],"name":"Nokia-Poc"}]}}\n',
        },
        creationTimestamp: "2024-09-23T18:06:38Z",
        generation: 1,
        managedFields: [
          {
            apiVersion: "vjailbreak.k8s.pf9.io/v1alpha1",
            fieldsType: "FieldsV1",
            fieldsV1: {
              "f:metadata": {
                "f:annotations": {
                  "f:kubectl.kubernetes.io/last-applied-configuration": {},
                },
              },
            },
            manager: "kubectl-last-applied",
            operation: "Apply",
          },
          {
            apiVersion: "vjailbreak.k8s.pf9.io/v1alpha1",
            fieldsType: "FieldsV1",
            fieldsV1: {
              "f:spec": {
                "f:destination": {
                  "f:openstackRef": {},
                },
                "f:networkMapping": {},
                "f:source": {
                  "f:datacenter": {},
                  "f:vmwareRef": {},
                },
                "f:storageMapping": {},
              },
            },
            manager: "kubectl",
            operation: "Apply",
            time: "2024-09-23T18:13:07Z",
          },
          {
            apiVersion: "vjailbreak.k8s.pf9.io/v1alpha1",
            fieldsType: "FieldsV1",
            fieldsV1: {
              "f:status": {
                ".": {},
                "f:openstack": {
                  ".": {},
                  "f:networks": {},
                  "f:volumeTypes": {},
                },
                "f:vmware": {},
              },
            },
            manager: "kubectl-edit",
            operation: "Update",
            subresource: "status",
            time: "2024-09-23T18:29:25Z",
          },
        ],
        name: "migrationtemplate",
        namespace: "migration-system",
        resourceVersion: "1001743",
        uid: "cd889b8b-4261-4ad4-9c92-7c1cb7655e8a",
      },
      spec: {
        destination: {
          openstackRef: "goodopenstackcreds",
        },
        networkMapping: "goodnetworkmapping",
        source: {
          datacenter: "PNAP BMC",
          vmwareRef: "goodvmwarecreds",
        },
        storageMapping: "goodstoragemapping",
      },
      status: {
        openstack: {
          networks: [
            "tenant-1",
            "vlan3003",
            "tenant-2",
            "tenant-3",
            "vlan3002",
          ],
          volumeTypes: ["lvm"],
        },
        vmware: [
          {
            datastores: ["vcenter-datastore-1"],
            name: "rocky9.1-01",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "w198",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1", "vcenter-datastore-1"],
            name: "mig_test_cbt_bak-clone-24",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1", "vcenter-datastore-1"],
            name: "mig_test_cbt_bak-clone-23",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1", "vcenter-datastore-1"],
            name: "mig_test_cbt_bak-clone-22",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1", "vcenter-datastore-1"],
            name: "mig_test_cbt_bak-clone-20",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1", "vcenter-datastore-1"],
            name: "mig_test_cbt_bak-clone-27",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1", "vcenter-datastore-1"],
            name: "mig_test_cbt_bak-clone-26",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1", "vcenter-datastore-1"],
            name: "mig_test_cbt_bak-clone-18",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1", "vcenter-datastore-1"],
            name: "mig_test_cbt_bak-clone-15",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1", "vcenter-datastore-1"],
            name: "mig_test_cbt_bak-clone-13",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1", "vcenter-datastore-1"],
            name: "mig_test_cbt_bak-clone-12",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "mig_test_cbt_bak-clone-30",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1", "vcenter-datastore-1"],
            name: "mig_test_cbt_bak-clone-29",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1", "vcenter-datastore-1"],
            name: "mig_test_cbt_bak-clone-11",
          },
          {
            datastores: ["vcenter-datastore-1", "vcenter-datastore-1"],
            name: "mig_test_cbt_bak-clone-10",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1", "vcenter-datastore-1"],
            name: "mig_test_cbt_bak-clone-9",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1", "vcenter-datastore-1"],
            name: "mig_test_cbt_bak-clone-8",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1", "vcenter-datastore-1"],
            name: "esxi-host-02",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "Appliance",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "w2019-4",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1", "vcenter-datastore-1"],
            name: "mig_test_cbt_bak-clone-25",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "migration-demo",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1", "vcenter-datastore-1"],
            name: "mig_test_cbt_bak-clone-19",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "test-ubuntu-vmdk",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1", "vcenter-datastore-1"],
            name: "mig_test_cbt",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "w2019-5",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1", "vcenter-datastore-1"],
            name: "mig_test_cbt_bak-clone-16",
            networks: ["VM Network"],
          },
          {
            datastores: [
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
            ],
            name: "esxi-host-01",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1", "vcenter-datastore-1"],
            name: "mig_test_cbt_uefi",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "Vcenter-test-host02",
            networks: ["VM Network", "VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1", "vcenter-datastore-1"],
            name: "esxi-host-03",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "thick-test",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "tapas-ubuntu-2204-kube-v1.27",
            networks: ["VM Network"],
          },
          {
            datastores: [
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
            ],
            name: "VMware vCenter Server opencloud",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "mig_test",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1", "vcenter-datastore-1"],
            name: "mig_test_cbt_bak-clone-6",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "New Virtual Machine",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1", "vcenter-datastore-1"],
            name: "mig_test_cbt_bak-clone-2",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1", "vcenter-datastore-1"],
            name: "mig_test_cbt_bak-clone-5",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "platform9-vmware-appliance-test",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "dev test",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1", "vcenter-datastore-1"],
            name: "winserver2k22",
            networks: ["VM Network", "VM Network 2"],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "AIO",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1", "vcenter-datastore-1"],
            name: "mig_test_cbt_bak-clone-28",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1", "vcenter-datastore-1"],
            name: "wintst_new",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1", "vcenter-datastore-1"],
            name: "wintest_uefi_new",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1", "vcenter-datastore-1"],
            name: "mig_test_cbt_bak",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1", "vcenter-datastore-1"],
            name: "mig_test_cbt_bak-clone-14",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "win10test",
            networks: ["VM Network"],
          },
          {
            datastores: ["local01", "local01"],
            name: "cloned-nka-poc",
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "openstack-lab",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "centos7-01",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1", "vcenter-datastore-1"],
            name: "mig_test_cbt_bak-clone-21",
            networks: ["VM Network"],
          },
          {
            datastores: [
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
            ],
            name: "esx-01-h-agent-vm",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "test-vmdk-2",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1", "vcenter-datastore-1"],
            name: "mig_test_cbt_bak-clone-4",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1", "vcenter-datastore-1"],
            name: "winserver2k19",
            networks: ["VM Network", "VM Network 2"],
          },
          {
            datastores: [
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
            ],
            name: "vcenter.phx.pnap",
            networks: ["VM Network"],
          },
          {
            datastores: [
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
            ],
            name: "VMware vCenter Server pub ip",
            networks: ["Public-Network"],
          },
          {
            datastores: ["local01"],
            name: "testvm",
            networks: ["VM Network"],
          },
          {
            datastores: [
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
            ],
            name: "vCenter Server Appliance test",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1", "vcenter-datastore-1"],
            name: "mig_test_cbt_bak-clone-17",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "Vcenter-test-host01",
            networks: ["VM Network", "VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "vcenter-pub-host1",
            networks: ["Public-Network", "VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1", "vcenter-datastore-1"],
            name: "mig_test_cbt_bak-clone-0",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "vcenter-nfs-server",
            networks: ["Public-Network", "VM Network"],
          },
          {
            datastores: ["local01"],
            name: "thick-test-2",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "win2022-test",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1", "local01"],
            name: "winserver2k16",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "w2019-3",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1", "vcenter-datastore-1"],
            name: "mig_test_cbt_bak-clone-1",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1", "vcenter-datastore-1"],
            name: "mig_test_cbt_bak-clone-3",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "nfs-server",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1", "vcenter-datastore-1"],
            name: "mig_test_cbt_bak-clone-7",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "win2k16",
            networks: ["bcm-network"],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "niket-vm",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "niket-test",
            networks: ["VM Network"],
          },
          {
            datastores: ["local01", "local01", "local01"],
            name: "Nested_ESXi7",
            networks: ["VM Network", "VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "w197",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "w196",
            networks: ["VM Network"],
          },
          {
            datastores: ["local01", "local01"],
            name: "ufo1",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "node004",
            networks: ["bcm-network"],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "node005",
            networks: ["bcm-network"],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "node003",
            networks: ["bcm-network"],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "node006",
            networks: ["bcm-network"],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "test-multi-nic",
            networks: ["VM Network", "bcm-network"],
          },
          {
            datastores: ["local01"],
            name: "w221",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "bcm-manager",
            networks: ["bcm-network"],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "w195",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "nvidia-bcm-router",
            networks: [
              "VM Network",
              "VM Network",
              "VM Network",
              "VM Network",
              "bcm-network",
              "VM Network",
              "VM Network",
              "VM Network",
              "VM Network",
              "VM Network",
            ],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "node001",
            networks: ["bcm-network"],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "w194",
            networks: ["VM Network"],
          },
          {
            datastores: ["local01", "local01", "local01"],
            name: "hstax-agent-vm",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "node002",
            networks: ["bcm-network"],
          },
          {
            datastores: ["local01"],
            name: "coriolis-u2204",
            networks: ["VM Network"],
          },
          {
            datastores: ["local01", "local01"],
            name: "ufo2",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "u2204-01",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "w2019-2",
            networks: ["VM Network"],
          },
          {
            datastores: ["local01"],
            name: "w2019-1",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "w191",
            networks: ["VM Network"],
          },
          {
            datastores: ["local01"],
            name: "testvm1",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "forklift-w2019-1",
            networks: ["VM Network"],
          },
          {
            datastores: ["local01"],
            name: "w192",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "w193",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "rparikh-test",
            networks: ["VM Network"],
          },
          {
            datastores: [
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
              "vcenter-datastore-1",
            ],
            name: "VMware vCenter Server Appliance",
            networks: ["VM Network"],
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "suse11",
          },
          {
            datastores: ["vcenter-datastore-1"],
            name: "Suse",
          },
          {
            datastores: ["local01", "local01"],
            name: "Nokia-Poc",
          },
        ],
      },
    },
  ],
  kind: "MigrationTemplateList",
  metadata: {
    continue: "",
    resourceVersion: "1098633",
  },
})

export const getMigrationTemplate = () => {
  return getMigrationTemplatesList().items[0]
}
