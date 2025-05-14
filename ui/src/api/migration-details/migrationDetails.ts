import axios from "../axios";

const VJAILBREAK_DEFAULT_NAMESPACE = "migration-system"; // or wherever your helper pods live

export async function getMigrationLogs(migrationName: string, namespace = VJAILBREAK_DEFAULT_NAMESPACE) {

  // 
  const podName = `v2v-helper-${migrationName}`;
  const endpoint = `/api/v1/namespaces/${namespace}/pods/${podName}/log`;

  const response = await axios.get({
    endpoint,
  })

  return response
}
