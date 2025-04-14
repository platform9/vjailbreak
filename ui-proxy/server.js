const express = require('express');
const axios = require('axios');
const k8s = require('@kubernetes/client-node');

const app = express();
app.use(express.json());

const kc = new k8s.KubeConfig();
kc.loadFromDefault();

const k8sCustomApi = kc.makeApiClient(k8s.CustomObjectsApi);
const coreV1Api = kc.makeApiClient(k8s.CoreV1Api);

// Define your CRDs
const CRDs = {
  openstack: {
    group: 'vjailbreak.io',
    version: 'v1',
    plural: 'openstackcreds'
  },
  vcenter: {
    group: 'vjailbreak.io',
    version: 'v1',
    plural: 'vmwarecreds'
  }
};

async function getVmwareCredsFromSecret(secretRef, namespace) {
  const secretNs = secretRef.namespace || namespace;
  const secretName = secretRef.name;
  const secret = await coreV1Api.readNamespacedSecret(secretName, secretNs);
  const data = secret.body.data;

  const decode = (field) => Buffer.from(data[field], 'base64').toString();
  return {
    host: decode('VCENTER_HOST'),
    username: data.username ? decode('VCENTER_USERNAME') : undefined,
    password: data.password ? decode('VCENTER_PASSWORD') : undefined,
    datacenter: decode('VCENTER_DATACENTER')
  };
}

app.post('/proxy/:type', async (req, res) => {
  const { type } = req.params; // "vcenter" or "openstack"
  const { target, endpoint, method = 'GET', body = {}, headers = {}, namespace = 'default' } = req.body;

  try {
    const crd = CRDs[type];
    if (!crd) throw new Error(`Unsupported type: ${type}`);

    // 1. Get CR
    const cr = await k8sCustomApi.getNamespacedCustomObject(
      crd.group,
      crd.version,
      namespace,
      crd.plural,
      target
    );
    const spec = cr.body.spec;
    const secretRef = spec.secretRef;
    if (!secretRef) throw new Error(`Missing secretRef in ${type} CR`);

    // 2. Get credentials from the referenced secret
    const creds = await getCredsFromSecret(secretRef, namespace);

    // 3. Prepare request
    let requestConfig = {
      method,
      url: creds.endpoint + endpoint,
      headers,
      data: body,
      validateStatus: () => true
    };

    if (type === 'vcenter') {
      requestConfig.headers['Authorization'] = 'Basic ' + Buffer.from(`${creds.username}:${creds.password}`).toString('base64');
    } else if (type === 'openstack' && endpoint === '/v3/auth/tokens') {
      requestConfig.data = { auth: creds.auth };
    }

    const resp = await axios(requestConfig);
    res.status(resp.status).send(resp.data);
  } catch (err) {
    console.error('Proxy error:', err);
    res.status(500).json({ error: err.message });
  }
});

const port = process.env.PORT || 8080;
app.listen(port, () => {
  console.log(`Proxy server running on port ${port}`);
});
