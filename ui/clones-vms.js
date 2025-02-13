import axios from 'axios';
import https from 'https';

/**
 * Clones an existing VM to a new VM in vCenter using the vSphere Automation API
 * and dynamically fetches the source VM's folder, resource pool, datastore, etc.
 *
 * Usage:
 *   node cloneVM_dynamic.js
 *
 * Note: Update vCenter credentials, host, and SOURCE_VM_ID below.
 */

// -------------------
// 1. CONFIGURATION
// -------------------

// vCenter connection details:
const VCENTER_HOST = 'https://vcenter.phx.pnap.platform9.horse';
const VCENTER_USERNAME = 'ppatil@jumpcloud.com';
const VCENTER_PASSWORD = 'Hanumaan.1011';
const DATACENTER_ID_OR_NAME = 'PNAP BMC';

// The source VM name you want to clone (e.g. "winserver2k22-vj-test")
const SOURCE_VM_NAME = 'winserver2k22-vj-test';
const DATASTORE_ID_OR_NAME = 'vcenter-datastore-1';
const HOST_ID_OR_NAME = 'esx-01.phx.pnap.platform9.horse';

// Provide a name for your cloned VM
const CLONE_VM_NAME = `pratik-vj-test`;


const NUMBER_OF_VMS = 15;

// -------------------
// 2. HELPER: CREATE VCENTER SESSION
// -------------------

async function createVCenterSession() {
    // First, create an axios instance for authentication
    const authAxios = axios.create({
        baseURL: `${VCENTER_HOST}`,  // Remove /rest from base URL
        headers: {
            'Content-Type': 'application/json'
        },
        httpsAgent: new https.Agent({
            rejectUnauthorized: false
        })
    });

    try {
        // Get session token using the correct endpoint
        const sessionResponse = await authAxios.post('/api/session', {}, {
            auth: {
                username: VCENTER_USERNAME,
                password: VCENTER_PASSWORD
            }
        });

        // Create new axios instance with the session token
        return axios.create({
            baseURL: `${VCENTER_HOST}`,  // Remove /rest from base URL
            headers: {
                'Content-Type': 'application/json',
                'vmware-api-session-id': sessionResponse.data,  // The token is directly in data
            },
            httpsAgent: new https.Agent({
                rejectUnauthorized: false
            })
        });
    } catch (error) {
        console.error('Authentication failed:', error.message);
        if (error.response) {
            console.error('Error details:', error.response.data);
        }
        throw error;
    }
}

// -------------------
// 3. MAIN: CLONE VM
// -------------------

async function cloneVm() {
    try {
        // Create an authenticated session
        const vcenter = await createVCenterSession();

        // First, get the VM ID from the name
        console.log(`Looking up VM ID for: ${SOURCE_VM_NAME}...`);
        const listVmsResponse = await vcenter.get('/api/vcenter/vm');

        if (listVmsResponse.status !== 200) {
            throw new Error('Failed to retrieve VM list');
        }

        const sourceVm = listVmsResponse?.data?.find(vm => vm.name === SOURCE_VM_NAME);
        if (!sourceVm) {
            throw new Error(`Could not find VM with name: ${SOURCE_VM_NAME}`);
        }

        const sourceVmId = sourceVm?.vm;
        console.log(`Found VM ID: ${sourceVmId}`);

        // Get the source VM details
        console.log(`Fetching details for source VM...`);
        const getVmResp = await vcenter.get(`/api/vcenter/vm/${sourceVmId}`);

        if (getVmResp.status !== 200 || !getVmResp.data) {
            throw new Error(`Could not retrieve VM details for: ${SOURCE_VM_NAME}`);
        }

        const sourceVmData = getVmResp.data;
        console.log(`Source VM details retrieved successfully`);


        // Create multiple VMs
        for (let i = 1; i <= NUMBER_OF_VMS; i++) {
            const vmName = `${CLONE_VM_NAME}-${i}`;

            const cloneSpec = {
                source: sourceVmId,
                name: vmName,
                power_on: true,
                // placement: {
                //     host: HOST_ID_OR_NAME,
                //     datastore: DATASTORE_ID_OR_NAME,
                // }
            };
            console.log(`Cloning source VM "${SOURCE_VM_NAME}" into a new VM: "${vmName}" (${i}/${NUMBER_OF_VMS})`);
            const cloneResponse = await vcenter.post(`/api/vcenter/vm?action=clone&vmw-task=true`, cloneSpec);
            if (cloneResponse.status === 200 || cloneResponse.status === 202) {
                const newVmId = cloneResponse.data;
                console.log(`Clone ${i} successful! New VM ID is: ${newVmId}`);
            } else {
                console.error(`Clone ${i} failed with status code: ${cloneResponse.status}`);
            }
        }

    } catch (error) {
        console.error(`Clone operation failed: ${error.message}`);
    }
}

// -------------------
// 4. RUN THE SCRIPT
// -------------------
cloneVm();