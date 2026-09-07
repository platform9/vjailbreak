export interface DebugPromptResponse {
  version: string
  last_updated: string
  prompt: string
}

export async function fetchDebugPrompt(): Promise<DebugPromptResponse> {
  const resp = await fetch('/dev-api/sdk/vpw/v1/ai/debug-prompt')
  if (!resp.ok) {
    throw new Error(`debug-prompt endpoint returned ${resp.status}`)
  }
  return resp.json() as Promise<DebugPromptResponse>
}
