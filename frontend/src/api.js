// Todas as chamadas usam /api (mesma origem): o Vite faz proxy em dev e o Vercel em produção.
async function request(path, options = {}) {
  const res = await fetch('/api' + path, {
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json' },
    ...options,
    body: options.body ? JSON.stringify(options.body) : undefined,
  })
  let data = null
  try { data = await res.json() } catch { /* sem corpo */ }
  if (!res.ok) {
    const err = new Error(data?.error || 'Erro inesperado. Tente novamente.')
    err.status = res.status
    throw err
  }
  return data
}

export const api = {
  me: () => request('/auth/me'),
  login: (email, password) => request('/auth/login', { method: 'POST', body: { email, password } }),
  register: (name, email, password) => request('/auth/register', { method: 'POST', body: { name, email, password } }),
  logout: () => request('/auth/logout', { method: 'POST', body: {} }),
  plans: () => request('/plans'),
  createPlan: (p) => request('/plans', { method: 'POST', body: p }),
  updatePlan: (id, p) => request('/plans/' + id, { method: 'PUT', body: p }),
  deletePlan: (id) => request('/plans/' + id, { method: 'DELETE' }),
  searchFoods: (q) => request(`/foods/search?q=${encodeURIComponent(q)}`),
}
