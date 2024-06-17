from pyVim.connect import SmartConnect, Disconnect
from pyVmomi import vim
from helper import *

# get vm by name
def get_vm_by_name(service_instance, vm_name):
    # Get the content
    content = service_instance.RetrieveContent()

    vms = get_all_obj(content, [vim.VirtualMachine])

    for i in vms:
        if i.name == vm_name:
            return i

    return None

# get vm by uuid
def get_vm_by_uuid(service_instance, vm_uuid):
    # Get the content
    content = service_instance.RetrieveContent()

    vms = get_all_obj(content, [vim.VirtualMachine])

    for i in vms:
        if i.summary.config.uuid == vm_uuid:
            return i

    return None

def get_all_info(service_instance):
    """
    Get all the information about the VM
    """
    data = {}  # Create a dict to store the data
    content = service_instance.RetrieveContent()
    children = content.rootFolder.childEntity
    for child in children:  # Iterate though DataCenters
        datacenter = child
        data[datacenter.name] = {}  # Add data Centers to data dict
        clusters = datacenter.hostFolder.childEntity
        for cluster in clusters:  # Iterate through the clusters in the DC
            # Add Clusters to data dict
            data[datacenter.name][cluster.name] = {}
            hosts = cluster.host  # Variable to make pep8 compliance
            for host in hosts:  # Iterate through Hosts in the Cluster
                hostname = host.summary.config.name
                # Add VMs to data dict by config name
                data[datacenter.name][cluster.name][hostname] = {}
                vms = host.vm
                for vm in vms:  # Iterate through each VM on the host
                    vmname = vm.summary.config.name
                    mac = []
                    for d in vm.config.hardware.device:
                        if hasattr(d, 'macAddress'):
                            mac.append(d.macAddress)
                    data[datacenter.name][cluster.name][hostname][vmname] = {'cpu': vm.config.hardware.numCPU, 
                                                                            'memory': vm.config.hardware.memoryMB,
                                                                            'state': vm.runtime.powerState,
                                                                            'mac': mac,
                                                                            'uuid': vm.summary.config.uuid,
                                                                            }
    return data
