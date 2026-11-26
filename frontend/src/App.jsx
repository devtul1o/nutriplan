import { useEffect, useState, useCallback } from 'react'
import { api } from './api.js'
import Auth from './Auth.jsx'
import Builder from './Builder.jsx'
import MyPlans from './MyPlans.jsx'
import { MAX_PLANS } from './calc.js'

export default function App() {
  const [user, setUser] = useState(undefined) // undefined = carregando, null = deslogado
  const [plans, setPlans] = useState([])
  const [tab, setTab] = useState('plans')
  const [editing, setEditing] = useState(null)

  const loadPlans = useCallback(async () => {
    try { setPlans(await api.plans()) } catch { /* sessão pode ter expirado */ }
  }, [])

  useEffect(() => {
    api.me().then(setUser).catch(() => setUser(null))
  }, [])
  useEffect(() => { if (user) loadPlans() }, [user, loadPlans])

  if (user === undefined) return <div className="center muted">Carregando…</div>
  if (!user) return <Auth onLogin={setUser} />

  const full = plans.length >= MAX_PLANS
  const openBuilder = (plan = null) => { setEditing(plan); setTab('builder') }
  const logout = async () => { await api.logout(); setUser(null); setPlans([]); setTab('plans') }

  return (
    <>
      <header className="top">
        <div className="top-in">
          <b className="brand">🥗 NutriPlan</b>
          <nav>
            <button className={tab === 'plans' ? 'on' : ''} onClick={() => setTab('plans')}>Minhas dietas <span className="badge">{plans.length}/{MAX_PLANS}</span></button>
            <button className={tab === 'builder' && !editing ? 'on' : ''} onClick={() => openBuilder(null)} disabled={full}
              title={full ? 'Limite de 3 planos. Exclua um para criar outro.' : ''}>+ Nova dieta</button>
          </nav>
          <div className="who">Olá, {user.name.split(' ')[0]} <button className="link" onClick={logout}>Sair</button></div>
        </div>
      </header>
      <main className="wrap">
        {tab === 'plans' ? (
          <MyPlans plans={plans} onEdit={openBuilder} onChanged={loadPlans} onNew={() => openBuilder(null)} />
        ) : (
          <Builder key={editing?.id ?? 'new'} plan={editing}
            onSaved={async () => { await loadPlans(); setEditing(null); setTab('plans') }}
            onCancel={() => { setEditing(null); setTab('plans') }} />
        )}
      </main>
    </>
  )
}
