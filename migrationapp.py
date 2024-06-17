import argparse
from validatevcenter import *

def main():
    # Create the argument parser
    parser = argparse.ArgumentParser()

    # Add the required arguments
    parser.add_argument("-u", "--username", help="vCenter username", required=True)
    parser.add_argument("-p", "--password", help="vCenter password", required=True)
    parser.add_argument("-H", "--host", help="vCenter host", required=True)
    parser.add_argument('-nossl', '--disable-ssl-verification',
                        help="Disable SSL verification", required=False, action='store_true')

    # Parse the command line arguments
    args = parser.parse_args()

    # Call the validate_vcenter function with the provided credentials
    try:
        service_instance = validate_vcenter(args.username, args.password, args.host, args.disable_ssl_verification)
    except Exception as e:
        # Connection failed
        print("Connection to vCenter failed:", str(e))
        return None

if __name__ == "__main__":
    main()