import argparse
from validatevcenter import *
from vmops import *

def main():
    # Create the argument parser
    parser = argparse.ArgumentParser()

    # Add the required arguments
    parser.add_argument("-u", "--username", help="vCenter username", required=True)
    parser.add_argument("-p", "--password", help="vCenter password", required=True)
    parser.add_argument("-H", "--host", help="vCenter host", required=True)
    parser.add_argument('-nossl', '--disable-ssl-verification',
                        help="Disable SSL verification", required=False, action='store_true')
    # VM name
    parser.add_argument("-n", "--vm-name", help="VM name", required=False)
    # VM uuid
    parser.add_argument("-id", "--vm-uuid", help="VM uuid", required=False)

    # Parse the command line arguments
    args = parser.parse_args()

    # Call the validate_vcenter function with the provided credentials
    try:
        service_instance = validate_vcenter(args.username, args.password, args.host, args.disable_ssl_verification)
    except Exception as e:
        # Connection failed
        print("Connection to vCenter failed:", str(e))
        return None

    if args.vm_name:
        # Get the VM by name
        vm = get_vm_by_name(service_instance, args.vm_name)
        if vm:
            print("VM found by name:", vm.name)
            # get cpu and memory
            print("CPU:", vm.config.hardware.numCPU)
            print("Memory:", vm.config.hardware.memoryMB)
            
        else:
            print("VM not found by name:", args.vm_name)    
    elif args.vm_uuid:
        # Get the VM by uuid
        vm = get_vm_by_uuid(service_instance, args.vm_uuid)
        if vm:
            print("VM found by uuid:", vm.name)
            print(vm)
        else:
            print("VM not found by uuid:", args.vm_uuid)

if __name__ == "__main__":
    main()