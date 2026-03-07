export const GROWTH_DISPLAY_MULTIPLIER = 100

export function scaleGrowthValue(value) {
  const numeric = Number(value || 0)
  if (!Number.isFinite(numeric)) return 0
  return numeric * GROWTH_DISPLAY_MULTIPLIER
}

export function formatScaledGrowth(value, options = {}) {
  const scaled = scaleGrowthValue(value)
  const {
    minimumFractionDigits = 0,
    maximumFractionDigits = scaled >= 1000 ? 0 : 2
  } = options

  return new Intl.NumberFormat('zh-CN', {
    minimumFractionDigits,
    maximumFractionDigits
  }).format(scaled)
}
