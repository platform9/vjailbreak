// Agent-count recommendation — DESIGN.md §9.1 (CPU-resource model).
//
//   CR = t * C
//   A  = clamp( ceil( max(0, CR - (m + ΣΔ)) / F ), 0, A_max )
//
// Pure + explainable: returns the suggested value, the unclamped raw value (to detect
// "exceeds one-shot capacity"), and a human-readable derivation for the UI.

export interface AgentRecommendationInput {
  /** Total VMs to migrate across the selected buckets — `t`. */
  totalVms: number
  /** CPU request (cores) per migration pod — `C`. */
  cpuPerMigration: number
  /** Free cores on the master — `m`. */
  masterFreeCores: number
  /** Total free cores across existing agents — `ΣΔ`. */
  agentFreeCores: number
  /** Schedulable cores a fresh agent adds — `F`. */
  freshAgentCores: number
  /** Ceiling on new agents — `A_max`. */
  maxAgents: number
}

export interface AgentRecommendation {
  /** Suggested number of new agents (clamped to [0, maxAgents]). */
  value: number
  /** Unclamped agent count — if > maxAgents, the run will proceed in waves. */
  rawValue: number
  /** True when rawValue exceeds maxAgents (one-shot capacity exceeded). */
  exceedsCapacity: boolean
  maxAgents: number
  /** Plain-language explanation of how `value` was derived. */
  derivation: string
}

const clamp = (x: number, lo: number, hi: number) => Math.max(lo, Math.min(x, hi))

export function recommendAgents(input: AgentRecommendationInput): AgentRecommendation {
  const { totalVms, cpuPerMigration, masterFreeCores, agentFreeCores, freshAgentCores, maxAgents } =
    input

  const coresNeeded = totalVms * cpuPerMigration // CR
  const freeNow = masterFreeCores + agentFreeCores // m + ΣΔ
  const deficit = Math.max(0, coresNeeded - freeNow)
  const rawValue = freshAgentCores > 0 ? Math.ceil(deficit / freshAgentCores) : 0
  const value = clamp(rawValue, 0, maxAgents)

  const derivation =
    `${totalVms} VMs × ${cpuPerMigration} cores = ${coresNeeded} needed; ` +
    `${freeNow} free now (master ${masterFreeCores} + agents ${agentFreeCores}); ` +
    `each agent adds ${freshAgentCores} → suggest ${value} new agent${value === 1 ? '' : 's'}.`

  return {
    value,
    rawValue,
    exceedsCapacity: rawValue > maxAgents,
    maxAgents,
    derivation
  }
}
