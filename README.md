# NutriPlan

Calculadora de calorias e planejamento semanal de dietas. Desenvolvido com **Go** (API + SQLite) e **React** (Vite + Recharts).

Fontes de alimentos: tabela **TACO 4ª ed.** (597 alimentos, embutida no binário via `//go:embed`) e **USDA FoodData Central** (consultada automaticamente como fallback pelo servidor com tradução PT ↔ EN via MyMemory API, mantendo a chave protegida no backend).

```text
backend/   API em Go (autenticação, planos semanais, TACO + fallback USDA)
frontend/  Interface React (Vite + Recharts)
deploy/    Caddyfile, serviço systemd, exemplo de variáveis de ambiente
```

## 1. Rodar localmente

**Pré-requisitos:** Go 1.22+, Node 18+.

**Chave da USDA (grátis):** https://fdc.nal.usda.gov/api-key-signup (enviada por e-mail).

### Backend

```bash
cd backend
cp .env.example .env        # preencha JWT_SECRET e USDA_API_KEY
go mod tidy                 # baixa dependências e gera go.sum
```

Gere um `JWT_SECRET` seguro com:

```bash
openssl rand -hex 32
```

Carregue as variáveis e inicie o servidor:

**Linux / macOS:**
```bash
set -a; source .env; set +a
go run .
```

**Windows (PowerShell):**
```powershell
Get-Content .env | ForEach-Object {
  if ($_ -match '^\s*([^#][^=]+)=(.*)$') {
    [Environment]::SetEnvironmentVariable($matches[1].Trim(), $matches[2].Trim(), 'Process')
  }
}
go run .
```

### Frontend

```bash
cd frontend
npm install
npm run dev                 # abre http://localhost:5173
```

## 2. Arquitetura de deploy

```text
Navegador ──► Vercel (React) ──/api/* (rewrite)──► GCP VM + Caddy (HTTPS) ──► API Go ──► SQLite
```

O Vercel faz proxy das rotas `/api/*` para a VM na Google Cloud. Assim, o navegador se comunica sempre com o **mesmo domínio**, permitindo cookies de sessão *first-party* (`HttpOnly`, `Secure`, `SameSite=Lax`), sem problemas de CORS ou bloqueio de cookies de terceiros.

## 3. Deploy passo a passo (GCP Always Free + Vercel)

### 3.1 Suba o código para o GitHub

```bash
git init && git add . && git commit -m "NutriPlan"
# Crie um repositório PRIVADO em github.com e faça o push
```

Os arquivos `.env` já estão listados no `.gitignore`. **Nunca** publique chaves nem o binário compilado no repositório.

### 3.2 Crie a VM gratuita na Google Cloud (GCP)

1. Acesse o Console da GCP (https://console.cloud.google.com) com uma conta de faturamento ativa (cartão de crédito apenas para verificação; a máquina abaixo entra no nível *Always Free*).
2. Vá em **Compute Engine → Instâncias de VM → Criar instância**.
3. Configure exatamente dentro da cota gratuita:
   - **Nome:** `nutriplan-api`
   - **Região:** `us-east1`, `us-central1` ou `us-west1` (obrigatório para zerar o custo da VM)
   - **Série / Tipo de máquina:** `E2` → **`e2-micro`** (2 vCPU compartilhadas, 1 GB de memória)
   - **Sistema operacional e armazenamento:** clique em **Alterar**, selecione **Ubuntu 24.04 LTS** (x86/64), **Disco permanente padrão** (10 GB a 30 GB) e desative a **Programação de snapshots** (deixe em *Sem programação* para evitar cobranças extras).
   - **Firewall:** marque **Permitir o tráfego HTTP** e **Permitir tráfegos HTTPS**.
4. Clique em **Criar** e anote o **IP externo** gerado.

### 3.3 Domínio gratuito (DuckDNS)

1. Acesse https://www.duckdns.org, faça login e crie um subdomínio (ex.: `nutriplan-api`).
2. Aponte o subdomínio para o **IP externo** da VM criada na GCP.
3. Seu endereço ficará como `nutriplan-api.duckdns.org`.

### 3.4 Compile e envie o backend

No **seu computador**, dentro da pasta `backend/` (o SQLite utilizado é Go puro, sem necessidade de CGO):

**Windows (PowerShell):**
```powershell
cd backend
go mod tidy
$env:GOOS="linux"
$env:GOARCH="amd64"
$env:CGO_ENABLED="0"
go build -o nutriplan .
```

**Linux / macOS:**
```bash
cd backend
go mod tidy
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o nutriplan .
```

Envie o arquivo `nutriplan` para a VM usando o botão **Fazer upload de arquivo** no terminal SSH do navegador da GCP (ele envia para `/home/SEU_USUARIO/nutriplan`) ou via `scp`/`gcloud compute scp`.

> Após o envio, apague o binário `nutriplan` local da pasta `backend/` (ou adicione-o ao `.gitignore`) antes de dar `git push`.

### 3.5 Configure o servidor na VM

Abra o terminal **SSH** da VM no Console da GCP e execute:

```bash
sudo useradd --system --home /opt/nutriplan --shell /usr/sbin/nologin nutriplan
sudo mkdir -p /opt/nutriplan /var/lib/nutriplan

# Ajuste o caminho de origem caso tenha enviado para ~ em vez de /tmp
sudo mv ~/nutriplan /opt/nutriplan/
sudo chmod 755 /opt/nutriplan/nutriplan
sudo chown -R nutriplan: /var/lib/nutriplan

# Gere o segredo JWT
openssl rand -hex 32

# Crie o arquivo de variáveis de ambiente (baseado em deploy/nutriplan.env.example)
sudo nano /etc/nutriplan.env
sudo chmod 600 /etc/nutriplan.env
```

Exemplo de conteúdo para `/etc/nutriplan.env`:

```env
JWT_SECRET=seu-hex-de-64-caracteres
USDA_API_KEY=sua-chave-da-usda
COOKIE_SECURE=true
FRONTEND_ORIGIN=https://seu-app.vercel.app
PORT=8080
DB_PATH=/var/lib/nutriplan/nutri.db
```

Configure e inicie o serviço `systemd` (usando o conteúdo de `deploy/nutriplan.service`):

```bash
sudo nano /etc/systemd/system/nutriplan.service
sudo systemctl daemon-reload
sudo systemctl enable --now nutriplan
sudo systemctl status nutriplan   # deve exibir "active (running)"
```

### 3.6 HTTPS automático com Caddy

Ainda na VM, instale o Caddy para gerenciar os certificados SSL automaticamente:

```bash
sudo apt update
sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https curl
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
sudo apt update && sudo apt install -y caddy
```

Edite `/etc/caddy/Caddyfile` com o seu subdomínio do DuckDNS:

```caddy
seu-subdominio.duckdns.org {
    encode gzip
    reverse_proxy 127.0.0.1:8080
}
```

Recarregue o Caddy e teste o endpoint de saúde:

```bash
sudo systemctl reload caddy
curl https://seu-subdominio.duckdns.org/api/health
```

O retorno esperado é `{"status":"ok"}`.

### 3.7 Frontend no Vercel

1. Edite `frontend/vercel.json` e substitua `SEU-SUBDOMINIO.duckdns.org` pelo seu domínio do DuckDNS. Faça commit e push.
2. Em https://vercel.com, entre com o GitHub → **Add New → Project** → importe o repositório.
3. Defina **Root Directory** como `frontend` (o framework *Vite* é detectado automaticamente) e clique em **Deploy**.
4. Copie a URL publicada (ex.: `https://nutriplan.vercel.app`).
5. Na VM, atualize `FRONTEND_ORIGIN` em `/etc/nutriplan.env` com a URL do Vercel e reinicie a API:
   ```bash
   sudo systemctl restart nutriplan
   ```
6. Acesse a URL do Vercel, crie sua conta e comece a usar.

> Se configurar um domínio próprio no Vercel, atualize `FRONTEND_ORIGIN` para o novo endereço.

### 3.8 Backup do banco SQLite (recomendado)

Os dados ficam armazenados em `/var/lib/nutriplan/nutri.db`. Para agendar backups diários às 3h:

```bash
sudo apt install -y sqlite3
sudo crontab -e
```

Adicione a linha abaixo:

```cron
0 3 * * * sqlite3 /var/lib/nutriplan/nutri.db ".backup '/var/lib/nutriplan/backup-$(date +\%F).db'"
```

## 4. Segurança já implementada

- Senhas armazenadas com **bcrypt**, mensagens de erro de login genéricas e comparação em tempo constante.
- Sessão via cookie **HttpOnly + Secure + SameSite=Lax** (invisível ao JavaScript); JWT HS256 com validade de 7 dias.
- Proteção **CSRF**: `SameSite`, exigência de `Content-Type: application/json` e validação do header `Origin`.
- **Rate limit** para login (por IP e por e-mail), cadastro e buscas.
- Consultas SQL parametrizadas com isolamento por `user_id` em todas as rotas de planos.
- Validação de limites no servidor (planos, refeições, gramaturas e textos) e limite de 1 MB por requisição.
- Chave da USDA e segredo JWT mantidos apenas no servidor; HTTPS obrigatório via Caddy; serviço `systemd` rodando com usuário sem privilégios.

**Manutenção recomendada:** manter o sistema atualizado (`sudo apt update && sudo apt upgrade`), manter a porta `8080` fechada para acesso externo (apenas `80`/`443` abertas via Caddy) e copiar periodicamente os backups do SQLite para fora da VM.

## 5. Como funciona

- **Planejamento semanal:** cada plano nutricional é organizado nos **7 dias da semana** (com abas por dia no editor). Os cartões da página inicial exibem a média diária de calorias e a distribuição de macronutrientes.
- **Calorias e macronutrientes:** os valores por 100 g de cada alimento são salvos no plano e recalculados proporcionalmente conforme a gramatura definida (4 kcal/g para proteínas e carboidratos; 9 kcal/g para gorduras).
- **Busca inteligente de alimentos:**
  - **TACO:** busca local instantânea (sem distinção de acentos ou maiúsculas).
  - **USDA (fallback automático):** caso a busca local não encontre resultados suficientes, o servidor traduz automaticamente o termo de português para inglês, consulta a API da USDA (Foundation + SR Legacy) com cache de 30 minutos e traduz os nomes de volta para português (com cache em memória via MyMemory API).
- **Créditos:** Tabela TACO – NEPA/UNICAMP; USDA FoodData Central (domínio público, CC0).
