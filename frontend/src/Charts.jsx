import { PieChart, Pie, Cell, Tooltip, ResponsiveContainer, BarChart, Bar, XAxis, YAxis } from 'recharts'
import { macroData, fmt, sumItems } from './calc.js'

export function MacroChart({ totals, size = 220 }) {
  const data = macroData(totals)
  const empty = data.every((d) => d.kcal === 0)
  return (
    <div className="macro">
      <div className="donut" style={{ height: size }}>
        <ResponsiveContainer>
          <PieChart>
            <Pie
              data={empty ? [{ name: 'vazio', kcal: 1 }] : data}
              dataKey="kcal" innerRadius="62%" outerRadius="92%" paddingAngle={empty ? 0 : 2}
              stroke="none" isAnimationActive animationDuration={350}
            >
              {empty ? <Cell fill="#e5e7eb" /> : data.map((d) => <Cell key={d.name} fill={d.color} />)}
            </Pie>
          </PieChart>
        </ResponsiveContainer>
        <div className="donut-center">
          <b>{fmt(totals.kcal)}</b>
          <span>kcal</span>
        </div>
      </div>
      <ul className="legend">
        {data.map((d) => (
          <li key={d.name}>
            <i style={{ background: d.color }} />
            <span>{d.name}</span>
            <b>{fmt(d.grams, 1)} g</b>
            <em>{fmt(d.pct)}%</em>
          </li>
        ))}
        <li className="fiber"><i style={{ background: '#16a34a' }} /><span>Fibras</span><b>{fmt(totals.fiber, 1)} g</b><em /></li>
      </ul>
    </div>
  )
}

export function MealBars({ meals }) {
  const data = meals.map((m) => ({ name: m.name, kcal: Math.round(sumItems(m.items).kcal) }))
  if (!data.some((d) => d.kcal > 0)) return null
  return (
    <div style={{ height: 30 + data.length * 34 }}>
      <ResponsiveContainer>
        <BarChart data={data} layout="vertical" margin={{ left: 0, right: 12 }}>
          <XAxis type="number" hide />
          <YAxis type="category" dataKey="name" width={110} tick={{ fontSize: 12 }} axisLine={false} tickLine={false} />
          <Tooltip formatter={(v) => [`${v} kcal`, 'Calorias']} cursor={{ fill: 'rgba(0,0,0,.05)' }} />
          <Bar dataKey="kcal" fill="#16a34a" radius={4} />
        </BarChart>
      </ResponsiveContainer>
    </div>
  )
}
