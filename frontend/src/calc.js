export const MAX_PLANS = 3
export const MAX_MEALS = 6

const g = (v) => Number(v) || 0

export function itemTotals(it) {
  const f = g(it.grams) / 100
  return { kcal: it.kcal * f, protein: it.protein * f, carbs: it.carbs * f, fat: it.fat * f, fiber: it.fiber * f }
}

export function sumItems(items) {
  return items.reduce(
    (a, it) => {
      const t = itemTotals(it)
      return { kcal: a.kcal + t.kcal, protein: a.protein + t.protein, carbs: a.carbs + t.carbs, fat: a.fat + t.fat, fiber: a.fiber + t.fiber }
    },
    { kcal: 0, protein: 0, carbs: 0, fat: 0, fiber: 0 },
  )
}

export const planTotals = (meals) => sumItems(meals.flatMap((m) => m.items))

export const fmt = (n, d = 0) => Number(n).toLocaleString('pt-BR', { minimumFractionDigits: d, maximumFractionDigits: d })

// Distribuição calórica dos macros (4 kcal/g proteína e carboidrato, 9 kcal/g gordura)
export function macroData(t) {
  const rows = [
    { name: 'Proteínas', grams: t.protein, kcal: t.protein * 4, color: '#2563eb' },
    { name: 'Carboidratos', grams: t.carbs, kcal: t.carbs * 4, color: '#f59e0b' },
    { name: 'Gorduras', grams: t.fat, kcal: t.fat * 9, color: '#ef4444' },
  ]
  const total = rows.reduce((a, r) => a + r.kcal, 0)
  return rows.map((r) => ({ ...r, pct: total ? (r.kcal / total) * 100 : 0 }))
}

export const WEEK_DAYS = [
  { key: 'seg', label: 'Seg' },
  { key: 'ter', label: 'Ter' },
  { key: 'qua', label: 'Qua' },
  { key: 'qui', label: 'Qui' },
  { key: 'sex', label: 'Sex' },
  { key: 'sab', label: 'Sáb' },
  { key: 'dom', label: 'Dom' },
]

export const DEFAULT_MEALS = ['Café da manhã', 'Almoço', 'Jantar']

export function createEmptyWeek() {
  return WEEK_DAYS.map((d) => ({
    day: d.key,
    meals: DEFAULT_MEALS.map((name) => ({ name, items: [] })),
  }))
}

export function normalizeDays(plan) {
  const byKey = new Map((plan?.days || []).map((d) => [d.day, d.meals || []]))
  return WEEK_DAYS.map((d) => ({
    day: d.key,
    meals: byKey.has(d.key) ? byKey.get(d.key) : DEFAULT_MEALS.map((name) => ({ name, items: [] })),
  }))
}

export function weekSummary(days = []) {
  const perDay = days.map((d) => planTotals(d.meals || []))
  const filled = perDay.filter((t) => t.kcal > 0)
  const total = perDay.reduce(
      (a, t) => ({
        kcal: a.kcal + t.kcal,
        protein: a.protein + t.protein,
        carbs: a.carbs + t.carbs,
        fat: a.fat + t.fat,
        fiber: a.fiber + t.fiber,
      }),
      { kcal: 0, protein: 0, carbs: 0, fat: 0, fiber: 0 },
  )
  return {
    total,
    avgKcal: filled.length ? total.kcal / filled.length : 0,
    perDay,
  }
}