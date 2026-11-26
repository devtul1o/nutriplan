# NutriPlan

Calculadora de calorias + montagem de dietas. **Go** (API + SQLite) e **React** (Vite + Recharts).
Fontes de alimentos: tabela **TACO 4ª ed.** (597 alimentos, embutida no backend) e **USDA FoodData Central** (consultada pelo servidor, então a chave nunca vai para o navegador).

```
backend/   API em Go
frontend/  React (Vite)
deploy/    Caddyfile, serviço systemd, exemplo de variáveis
```

## 1. Rodar localmente

Pré-requisitos: Go 1.22+, Node 18+.

**Chave da USDA (grátis):** https://fdc.nal.usda.gov/api-key-signup (chega por email).

```bash
# Terminal 1 – backend
cd backend
cp .env.example .env        # edite JWT_SECRET e USDA_API_KEY
go mod tidy                 # baixa dependências e gera go.sum
set -a; source .env; set +a # (Linux/Mac) carrega as variáveis
go run .

# Terminal 2 – frontend
cd frontend
npm install
npm run dev                 # abre http://localhost:5173
```
Gere o `JWT_SECRET` com: `openssl rand -hex 32`.

## 2. Arquitetura de deploy (e por quê)

```
Navegador ──► Vercel (React) ──/api/* (rewrite)──► Oracle VM: Caddy (HTTPS) ──► API Go ──► SQLite
```
O Vercel encaminha `/api/*` para o seu servidor. Assim o navegador só fala com **um domínio**: o cookie de sessão é *first-party* (`HttpOnly`, `Secure`, `SameSite=Lax`), sem CORS e sem o bloqueio de cookies de terceiros.

Sua escolha (Vercel + Oracle Free) é boa e gratuita. Duas observações:
- A Oracle às vezes está sem capacidade de VM gratuita na região; se acontecer, tente outro horário ou use a VM `VM.Standard.E2.1.Micro` (AMD), que basta para este app.
- Alternativa mais simples: **tudo na VM Oracle** (Caddy serve o `frontend/dist` e faz proxy de `/api`), sem Vercel. Se quiser, peço que eu monte essa variante.

## 3. Deploy passo a passo

### 3.1 Suba o código para o GitHub
```bash
cd nutri
git init && git add . && git commit -m "NutriPlan"
# crie um repositório PRIVADO em github.com e siga as instruções de "push an existing repository"
```
Os arquivos `.env` já estão no `.gitignore`. **Nunca** publique chaves.

### 3.2 Crie a VM na Oracle Cloud
1. Crie conta em https://www.oracle.com/cloud/free/ (pede cartão para verificação; não cobra no Always Free).
2. Menu → *Compute* → *Instances* → **Create instance**.
3. Imagem: **Ubuntu 22.04**. Shape: *Ampere (VM.Standard.A1.Flex, 1 OCPU/6 GB)* ou *VM.Standard.E2.1.Micro*. Anote qual escolheu (ARM = `arm64`, AMD = `amd64`).
4. Em *SSH keys*, escolha **Generate a key pair** e baixe a chave privada (guarde bem).
5. Crie a instância e anote o **IP público**.
6. **Libere as portas 80 e 443:** na instância → *Subnet* → *Security List* → *Add Ingress Rules*: origem `0.0.0.0/0`, TCP, portas `80` e `443` (uma regra para cada).
7. Conecte: `ssh -i caminho/da/chave ubuntu@IP_PUBLICO`.
8. Já dentro da VM, libere também o firewall interno do Ubuntu:
```bash
sudo iptables -I INPUT 6 -m state --state NEW -p tcp --dport 80 -j ACCEPT
sudo iptables -I INPUT 6 -m state --state NEW -p tcp --dport 443 -j ACCEPT
sudo apt update && sudo apt install -y iptables-persistent && sudo netfilter-persistent save
```

### 3.3 Domínio gratuito (necessário para HTTPS)
1. Entre em https://www.duckdns.org, crie um subdomínio (ex.: `nutriplan-api`) e aponte para o IP público da VM.
2. Seu endereço será `nutriplan-api.duckdns.org`.

### 3.4 Compile e envie o backend
No **seu computador** (o SQLite usado é Go puro, então a compilação cruzada é simples):
```bash
cd backend
go mod tidy
# VM ARM:
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o nutriplan .
# VM AMD: troque arm64 por amd64
scp -i chave nutriplan ubuntu@IP_PUBLICO:/tmp/
```

### 3.5 Configure o servidor (na VM)
```bash
sudo useradd --system --home /opt/nutriplan --shell /usr/sbin/nologin nutriplan
sudo mkdir -p /opt/nutriplan /var/lib/nutriplan
sudo mv /tmp/nutriplan /opt/nutriplan/ && sudo chmod 755 /opt/nutriplan/nutriplan
sudo chown -R nutriplan: /var/lib/nutriplan

# variáveis de ambiente (copie deploy/nutriplan.env.example e preencha)
sudo nano /etc/nutriplan.env      # gere o segredo com: openssl rand -hex 32
sudo chmod 600 /etc/nutriplan.env

# serviço (copie o conteúdo de deploy/nutriplan.service)
sudo nano /etc/systemd/system/nutriplan.service
sudo systemctl daemon-reload && sudo systemctl enable --now nutriplan
sudo systemctl status nutriplan   # deve mostrar "active (running)"
```
Em `FRONTEND_ORIGIN` coloque a URL do Vercel (passo 3.7). Pode deixar um valor provisório e corrigir depois.

### 3.6 HTTPS com Caddy (certificado automático)
```bash
sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https curl
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
sudo apt update && sudo apt install -y caddy
sudo nano /etc/caddy/Caddyfile     # use deploy/Caddyfile, com o seu subdomínio
sudo systemctl reload caddy
```
Teste no navegador: `https://nutriplan-api.duckdns.org/api/health` → `{"status":"ok"}`.

### 3.7 Frontend no Vercel
1. Edite `frontend/vercel.json` e troque `SEU-SUBDOMINIO.duckdns.org` pelo seu subdomínio. Faça commit e push.
2. Em https://vercel.com entre com o GitHub → **Add New → Project** → importe o repositório.
3. **Root Directory:** `frontend` · Framework: *Vite* (detectado) · **Deploy**.
4. Copie a URL gerada (ex.: `https://nutriplan.vercel.app`).
5. Na VM, ajuste `FRONTEND_ORIGIN=https://nutriplan.vercel.app` em `/etc/nutriplan.env` e rode `sudo systemctl restart nutriplan`.
6. Abra a URL do Vercel, crie uma conta e teste. 🎉

> Se usar domínio próprio no Vercel, atualize `FRONTEND_ORIGIN` para ele.

### 3.8 Backup do banco (recomendado)
Os dados ficam em `/var/lib/nutriplan/nutri.db`. Faça cópias periódicas:
```bash
sudo apt install -y sqlite3
sudo crontab -e
# todo dia às 3h:
0 3 * * * sqlite3 /var/lib/nutriplan/nutri.db ".backup '/var/lib/nutriplan/backup-$(date +\%F).db'"
```
Baixe os backups para fora da VM de vez em quando (`scp`).

## 4. O que já está protegido
- Senhas com **bcrypt**; mensagens de erro de login genéricas e comparação em tempo constante.
- Sessão em cookie **HttpOnly + Secure + SameSite=Lax** (JavaScript não consegue ler o token); JWT HS256 com validade de 7 dias.
- Proteção **CSRF**: SameSite, exigência de `application/json` e checagem do header `Origin`.
- **Rate limit** em login (por IP e por email), cadastro e buscas.
- Consultas SQL parametrizadas; todo acesso a planos filtra por `user_id` (um usuário nunca acessa o plano de outro).
- Validação e limites no servidor (3 planos, 6 refeições, gramas, tamanhos de texto); corpo de requisição limitado a 1 MB.
- Chave da USDA e segredo do JWT apenas no servidor; HTTPS obrigatório via Caddy; cabeçalhos de segurança; serviço systemd sem privilégios.

**Fica por sua conta:** manter a VM atualizada (`sudo apt upgrade`), não expor a porta 8080 (só 80/443), guardar backups, e se for coletar dados de verdade, ter uma política de privacidade (LGPD).

## 5. Como funciona
- **Calorias/macros:** os valores por 100 g do alimento são gravados no plano e escalados pela gramatura (alterável a qualquer momento). Calorias dos macros: 4 kcal/g (proteína, carboidrato) e 9 kcal/g (gordura).
- **TACO:** busca sem acento/maiúsculas. **USDA:** busca em inglês (Foundation + SR Legacy), com cache de 30 min para poupar o limite de 1.000 req/h.
- Créditos: TACO – NEPA/UNICAMP; USDA FoodData Central (domínio público, CC0).
