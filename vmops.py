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

