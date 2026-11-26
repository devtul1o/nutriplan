import { useEffect, useMemo, useRef, useState } from 'react'
import { api } from './api.js'
import { MacroChart, MealBars } from './Charts.jsx'
import { planTotals, sumItems, fmt, MAX_MEALS, WEEK_DAYS, createEmptyWeek, normalizeDays } from './calc.js'

const MEAL_SUGGESTIONS = ['Café da manhã', 'Lanche da manhã', 'Almoço', 'Lanche da tarde', 'Jantar', 'Ceia']
const GOALS = ['Emagrecimento', 'Ganho de massa muscular', 'Manutenção do peso', 'Definição', 'Reeducação alimentar']

function FoodSearch({ activeMeal, onPick }) {
  const [q, setQ] = useState('')
  const [results, setResults] = useState([])
  const [source, setSource] = useState('')
  const [state, setState] = useState('idle')
  const [error, setError] = useState('')
  const seq = useRef(0)

  useEffect(() => {
    if (q.trim().length < 2) { setResults([]); setSource(''); setState('idle'); return }
    const id = ++seq.current
    setState('loading')
    const t = setTimeout(async () => {
      try {
        const r = await api.searchFoods(q.trim())
        if (id !== seq.current) return
        setResults(r.foods || [])
        setSource(r.source || '')
        setError('')
        setState('done')
      } catch (e) {
        if (id !== seq.current) return
        setError(e.message)
        setState('error')
      }
    }, 350)
    return () => clearTimeout(t)
  }, [q])

  return (
      <div className="card search">
        <h3>Adicionar alimento</h3>
        <p className="muted small">Adicionando em: <b>{activeMeal || '—'}</b></p>
        <input
            className="search-input"
            placeholder="Digite o alimento. Ex.: arroz, frango grelhado, pasta de amendoim…"
            value={q}
            onChange={(e) => setQ(e.target.value)}
        />
        {state === 'done' && source === 'usda' && results.length > 0 && (
            <p className="muted small">Não encontrado na TACO — exibindo resultados traduzidos da USDA.</p>
        )}
        <div className="results">
          {state === 'loading' && <p className="muted small">Buscando…</p>}
          {state === 'error' && <p className="error">{error}</p>}
          {state === 'done' && results.length === 0 && <p className="muted small">Nada encontrado.</p>}
          {results.map((f) => (
              <button key={f.source + f.id} className="result" onClick={() => onPick(f)} disabled={!activeMeal}>
                <span className="r-name">{f.name}</span>
                <span className="r-info">{fmt(f.kcal)} kcal · P {fmt(f.protein, 1)} · C {fmt(f.carbs, 1)} · G {fmt(f.fat, 1)} <small>/100 g</small></span>
                <span className="r-add">+</span>
              </button>
          ))}
        </div>
      </div>
  )
}

export default function Builder({ plan, onSaved, onCancel }) {
  const [name, setName] = useState(plan?.name || '')
  const [goal, setGoal] = useState(plan?.goal || '')
  const [days, setDays] = useState(() => (plan ? normalizeDays(plan) : createEmptyWeek()))
  const [dayIdx, setDayIdx] = useState(0)
  const [active, setActive] = useState(0)
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)

  const meals = days[dayIdx].meals
  const totals = useMemo(() => planTotals(meals), [meals])

  const setMeals = (updater) =>
      setDays((ds) =>
          ds.map((d, idx) =>
              idx === dayIdx ? { ...d, meals: typeof updater === 'function' ? updater(d.meals) : updater } : d,
          ),
      )

  const changeDay = (idx) => {
    setDayIdx(idx)
    setActive(0)
  }

  const updateMeal = (i, fn) => setMeals((ms) => ms.map((m, k) => (k === i ? fn(m) : m)))

  function addFood(f) {
    updateMeal(active, (m) => ({
      ...m,
      items: [...m.items, { source: f.source, foodId: f.id, name: f.name, grams: 100, kcal: f.kcal, protein: f.protein, carbs: f.carbs, fat: f.fat, fiber: f.fiber }],
    }))
  }

  const setGrams = (mi, ii, v) =>
      updateMeal(mi, (m) => ({
        ...m,
        items: m.items.map((it, k) => (k === ii ? { ...it, grams: v === '' ? '' : Math.min(5000, Math.max(0, Number(v))) } : it)),
      }))

  const removeItem = (mi, ii) => updateMeal(mi, (m) => ({ ...m, items: m.items.filter((_, k) => k !== ii) }))

  const addMeal = () => {
    if (meals.length >= MAX_MEALS) return
    const used = new Set(meals.map((m) => m.name))
    setMeals([...meals, { name: MEAL_SUGGESTIONS.find((s) => !used.has(s)) || `Refeição ${meals.length + 1}`, items: [] }])
    setActive(meals.length)
  }

  const removeMeal = (i) => {
    setMeals(meals.filter((_, k) => k !== i))
    setActive((a) => Math.max(0, Math.min(a, meals.length - 2)))
  }

  async function save() {
    setError('')
    if (!name.trim()) return setError('Dê um nome ao plano.')
    for (const d of days) {
      if (d.meals.some((m) => m.items.some((it) => !(Number(it.grams) > 0)))) {
        return setError('Informe a gramatura de todos os alimentos (maior que 0).')
      }
    }
    const payload = {
      name: name.trim(),
      goal: goal.trim(),
      days: days.map((d) => ({
        day: d.day,
        meals: d.meals.map((m) => ({
          ...m,
          items: m.items.map((it) => ({ ...it, grams: Number(it.grams) })),
        })),
      })),
    }
    setSaving(true)
    try {
      plan ? await api.updatePlan(plan.id, payload) : await api.createPlan(payload)
      onSaved()
    } catch (e) {
      setError(e.message)
    } finally {
      setSaving(false)
    }
  }

  return (
      <div className="builder">
        <section className="col-main">
          <div className="card">
            <h2>{plan ? 'Editar dieta' : 'Nova dieta'}</h2>
            <div className="day-tabs">
              {WEEK_DAYS.map((d, idx) => {
                const hasItems = days[idx].meals.some((m) => m.items.length > 0)
                return (
                    <button
                        key={d.key}
                        type="button"
                        className={`day-tab ${idx === dayIdx ? 'on' : ''} ${hasItems ? 'filled' : ''}`}
                        onClick={() => changeDay(idx)}
                    >
                      {d.label}
                    </button>
                )
              })}
            </div>
            <div className="grid2">
              <label>Nome do plano<input value={name} onChange={(e) => setName(e.target.value)} maxLength={80} placeholder="Ex.: Plano de verão" /></label>
              <label>Objetivo<input list="goals" value={goal} onChange={(e) => setGoal(e.target.value)} maxLength={200} placeholder="Ex.: Emagrecimento" />
                <datalist id="goals">{GOALS.map((g) => <option key={g} value={g} />)}</datalist>
              </label>
            </div>
          </div>

          {meals.map((m, mi) => {
            const t = sumItems(m.items)
            return (
                <div key={mi} className={'card meal' + (active === mi ? ' active' : '')} onClick={() => setActive(mi)}>
                  <div className="meal-head">
                    <input className="meal-name" value={m.name} maxLength={40} onChange={(e) => updateMeal(mi, (x) => ({ ...x, name: e.target.value }))} aria-label="Nome da refeição" />
                    <span className="meal-kcal">{fmt(t.kcal)} kcal</span>
                    <button className="icon" title="Remover refeição" onClick={(e) => { e.stopPropagation(); removeMeal(mi) }}>🗑</button>
                  </div>
                  {m.items.length === 0 && <p className="muted small">{active === mi ? 'Busque um alimento ao lado e clique para adicionar aqui.' : 'Clique para selecionar esta refeição.'}</p>}
                  {m.items.map((it, ii) => (
                      <div className="item" key={ii}>
                        <div className="item-name">{it.name}<small>{it.source === 'taco' ? 'TACO' : 'USDA'} · P {fmt(it.protein * (Number(it.grams) || 0) / 100, 1)} g · C {fmt(it.carbs * (Number(it.grams) || 0) / 100, 1)} g · G {fmt(it.fat * (Number(it.grams) || 0) / 100, 1)} g</small></div>
                        <div className="grams">
                          <input type="number" min="1" max="5000" step="5" value={it.grams} onChange={(e) => setGrams(mi, ii, e.target.value)} onClick={(e) => e.stopPropagation()} aria-label="Gramas" /> g
                        </div>
                        <b className="item-kcal">{fmt(it.kcal * (Number(it.grams) || 0) / 100)} kcal</b>
                        <button className="icon" title="Remover" onClick={(e) => { e.stopPropagation(); removeItem(mi, ii) }}>✕</button>
                      </div>
                  ))}
                </div>
            )
          })}

          <div className="day-nav-row">
            <button className="btn" onClick={addMeal} disabled={meals.length >= MAX_MEALS}>
              + Adicionar refeição ({meals.length}/{MAX_MEALS})
            </button>
            <div className="day-nav-btns">
              <button className="btn" disabled={dayIdx === 0} onClick={() => changeDay(dayIdx - 1)}>← Dia anterior</button>
              {dayIdx < WEEK_DAYS.length - 1 ? (
                  <button className="btn primary" onClick={() => changeDay(dayIdx + 1)}>Próximo dia →</button>
              ) : (
                  <button className="btn primary" onClick={save} disabled={saving}>{saving ? 'Salvando…' : 'Salvar dieta'}</button>
              )}
            </div>
          </div>
        </section>

        <aside className="col-side">
          <FoodSearch activeMeal={meals[active]?.name} onPick={addFood} />
          <div className="card summary">
            <h3>Resumo ({WEEK_DAYS[dayIdx].label})</h3>
            <MacroChart totals={totals} />
            <MealBars meals={meals} />
            {error && <div className="error" role="alert">{error}</div>}
            <div className="actions">
              <button className="btn primary" onClick={save} disabled={saving}>{saving ? 'Salvando…' : 'Salvar plano'}</button>
              <button className="btn" onClick={onCancel}>Cancelar</button>
            </div>
          </div>
        </aside>
      </div>
  )
}