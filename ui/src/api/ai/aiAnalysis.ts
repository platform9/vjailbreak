import api from 'src/api/axios'
import type { AIAnalyzeRequest, AIAnalyzeResponse } from './model'

export async function analyzeMigration(req: AIAnalyzeRequest): Promise<AIAnalyzeResponse> {
  return api.post<AIAnalyzeResponse>({ endpoint: '/vpw/v1/ai/analyze', data: req })
}

export async function getAIKeyStatus(): Promise<{ configured: boolean }> {
  return api.get<{ configured: boolean }>({ endpoint: '/vpw/v1/ai/key' })
}

export async function saveAIKey(apiKey: string, adminKey: string): Promise<void> {
  await api.post<void>({ endpoint: '/vpw/v1/ai/key', data: { api_key: apiKey, admin_key: adminKey } })
}
