import atexit
from pyVim.connect import SmartConnect, Disconnect

def validate_vcenter(username, password, host, disable_ssl_verification=False):
    # Connect to vCenter
    service_instance = SmartConnect(host=host, user=username, pwd=password, disableSslCertValidation=disable_ssl_verification)

    # Connection successful
    print("Connection to vCenter successful!")

    # doing this means you don't need to remember to disconnect your script/objects
    atexit.register(Disconnect, service_instance)

    return service_instance