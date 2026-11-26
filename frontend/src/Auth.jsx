import { useState } from 'react'
import { api } from './api.js'

export default function Auth({ onLogin }) {
  const [mode, setMode] = useState('login')
  const [form, setForm] = useState({ name: '', email: '', password: '' })
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const set = (k) => (e) => setForm({ ...form, [k]: e.target.value })

  async function submit(e) {
    e.preventDefault()
    setError('')
    setBusy(true)
    try {
      const user = mode === 'login'
        ? await api.login(form.email, form.password)
        : await api.register(form.name, form.email, form.password)
      onLogin(user)
    } catch (err) {
      setError(err.message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="auth-wrap">
      <form className="card auth" onSubmit={submit}>
        <h1>🥗 NutriPlan</h1>
        <p className="muted">Calcule calorias e monte suas dietas com a tabela TACO e a base USDA.</p>
        {mode === 'register' && (
          <label>Nome<input value={form.name} onChange={set('name')} required minLength={2} maxLength={80} autoComplete="name" /></label>
        )}
        <label>Email<input type="email" value={form.email} onChange={set('email')} required autoComplete="email" /></label>
        <label>Senha<input type="password" value={form.password} onChange={set('password')} required minLength={8} maxLength={72}
          autoComplete={mode === 'login' ? 'current-password' : 'new-password'} />
          {mode === 'register' && <small className="muted">Mínimo de 8 caracteres.</small>}
        </label>
        {error && <div className="error" role="alert">{error}</div>}
        <button className="btn primary" disabled={busy}>{busy ? 'Aguarde…' : mode === 'login' ? 'Entrar' : 'Criar conta'}</button>
        <button type="button" className="link" onClick={() => { setMode(mode === 'login' ? 'register' : 'login'); setError('') }}>
          {mode === 'login' ? 'Não tem conta? Cadastre-se' : 'Já tem conta? Entrar'}
        </button>
      </form>
    </div>
  )
}
