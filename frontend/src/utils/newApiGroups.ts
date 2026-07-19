export interface NewApiGroup {
  name: string
  ratio: number
}

export const DEFAULT_NEWAPI_MAX_GROUP_MULTIPLIER = 1

export function isValidNewApiGroupMultiplier(value: number): boolean {
  return Number.isFinite(value) && value >= 0
}

export function eligibleNewApiGroups(groups: Record<string, number>, maxMultiplier: number): NewApiGroup[] {
  if (!isValidNewApiGroupMultiplier(maxMultiplier)) return []

  return Object.entries(groups)
    .map(([name, ratio]) => ({ name: name.trim(), ratio }))
    .filter(group => group.name !== '' && Number.isFinite(group.ratio) && group.ratio >= 0 && group.ratio <= maxMultiplier)
    .sort((left, right) => left.ratio - right.ratio || left.name.localeCompare(right.name))
}
