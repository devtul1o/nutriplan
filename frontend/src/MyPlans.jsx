import { useState } from 'react'
import { api } from './api.js'
import { MacroChart, MealBars } from './Charts.jsx'
import { sumItems, fmt, MAX_PLANS, WEEK_DAYS, normalizeDays, weekSummary } from './calc.js'

export default function MyPlans({ plans, onEdit, onChanged, onNew }) {
    const [viewing, setViewing] = useState(null)
    const [dayIdx, setDayIdx] = useState(0)
    const [error, setError] = useState('')

    async function remove(p) {
        if (!window.confirm(`Excluir o plano "${p.name}"? Esta ação não pode ser desfeita.`)) return
        try { await api.deletePlan(p.id); onChanged() } catch (e) { setError(e.message) }
    }

    if (plans.length === 0) {
        return (
            <div className="card empty">
                <h2>Você ainda não tem dietas</h2>
                <p className="muted">Monte até {MAX_PLANS} planos alimentares semanais com até 6 refeições por dia.</p>
                <button className="btn primary" onClick={onNew}>Montar minha primeira dieta</button>
            </div>
        )
    }

    // Modo expandido ao clicar em "Ver"
    if (viewing) {
        const days = normalizeDays(viewing)
        const { perDay } = weekSummary(days)
        const currentDay = days[dayIdx]
        const dayTotals = perDay[dayIdx]

        return (
            <div className="plan-expanded">
                <div className="expanded-head">
                    <button className="btn" onClick={() => setViewing(null)}>← Voltar</button>
                    <div>
                        <h2>{viewing.name}</h2>
                        {viewing.goal && <span className="goal">🎯 {viewing.goal}</span>}
                    </div>
                </div>

                <div className="day-tabs">
                    {WEEK_DAYS.map((d, idx) => {
                        const hasItems = days[idx].meals.some((m) => m.items.length > 0)
                        return (
                            <button
                                key={d.key}
                                className={`day-tab ${idx === dayIdx ? 'on' : ''} ${hasItems ? 'filled' : ''}`}
                                onClick={() => setDayIdx(idx)}
                            >
                                {d.label}
                            </button>
                        )
                    })}
                </div>

                <div className="expanded-grid">
                    <section className="card meals-view">
                        <h3>Refeições — {WEEK_DAYS[dayIdx].label}</h3>
                        {currentDay.meals.every((m) => m.items.length === 0) ? (
                            <p className="muted">Nenhum alimento cadastrado para este dia.</p>
                        ) : (
                            currentDay.meals.map((m, i) => (
                                <section key={i}>
                                    <h4>{m.name} <span>{fmt(sumItems(m.items).kcal)} kcal</span></h4>
                                    {m.items.length === 0 ? <p className="muted small">Sem alimentos.</p> : (
                                        <ul>
                                            {m.items.map((it, j) => (
                                                <li key={j}>
                                                    <span>{it.name}</span>
                                                    <b>{fmt(it.grams)} g</b>
                                                    <em>{fmt(sumItems([it]).kcal)} kcal</em>
                                                </li>
                                            ))}
                                        </ul>
                                    )}
                                </section>
                            ))
                        )}
                    </section>

                    <aside className="card">
                        <h3>Calorias do dia ({WEEK_DAYS[dayIdx].label})</h3>
                        <MacroChart totals={dayTotals} size={200} />
                        <MealBars meals={currentDay.meals} />
                    </aside>
                </div>
            </div>
        )
    }

    // Cards da Home
    return (
        <>
            <h2>Minhas dietas</h2>
            {error && <div className="error">{error}</div>}
            <div className="plans">
                {plans.map((p) => {
                    const days = normalizeDays(p)
                    const { total, avgKcal } = weekSummary(days)
                    return (
                        <article className="card plan" key={p.id}>
                            <div className="plan-head">
                                <div>
                                    <h3>{p.name}</h3>
                                    {p.goal && <span className="goal">🎯 {p.goal}</span>}
                                </div>
                                <div className="kcal-big">
                                    <b>{fmt(avgKcal)}</b> kcal/dia (média)
                                </div>
                            </div>
                            <MacroChart totals={total} size={170} />
                            <div className="actions center-actions">
                                <button className="btn primary" onClick={() => { setDayIdx(0); setViewing(p) }}>Ver</button>
                                <button className="btn" onClick={() => onEdit(p)}>Editar</button>
                                <button className="btn danger" onClick={() => remove(p)}>Excluir</button>
                            </div>
                        </article>
                    )
                })}
            </div>
        </>
    )
}