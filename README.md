markdown
# KerionLabs Monitoring

A full observability stack deployed to Azure with Terraform. A Go service
scrapes public job boards (Greenhouse, Lever, Ashby), exposes Prometheus
metrics, and surfaces matches in Grafana. Everything runs in Docker
Compose on a single Linux VM provisioned from code.

## Architecture

```
Internet
   │
   ▼
┌─────────────────────────────────────┐
│  Azure VM (southafricanorth)        │
│                                     │
│   Nginx :443 ──► Grafana :3000      │
│      │                              │
│      └────────► Prometheus :9090    │
│                     │               │
│                     └─► internships │
│                         :8080       │
│                                     │
│   node-exporter :9100 ──► (scraped  │
│                           by Prom)  │
└─────────────────────────────────────┘
```

Nginx terminates TLS and routes traffic. Grafana queries Prometheus,
which scrapes the Go service and node-exporter. All five run as
containers on one VM, provisioned with Terraform.

## Stack

| Component | Purpose |
|---|---|
| **Nginx** | Reverse proxy, TLS termination, basic auth for Prometheus |
| **Grafana** | Dashboards and visualization |
| **Prometheus** | Metrics storage and query engine |
| **node-exporter** | Host-level metrics (CPU, memory, disk, network) |
| **internships** | Go service scraping job boards, exposing Prometheus metrics |

## Local development

Requires Docker and Docker Compose.

```bash
git clone https://github.com/<you>/kerionlabs-monitoring.git
cd kerionlabs-monitoring
cp .env.example .env
# edit .env and set GRAFANA_ADMIN_PASSWORD
docker compose up -d
Then point grafana.local and prometheus.local at 127.0.0.1:

bash
echo "127.0.0.1 grafana.local prometheus.local" | sudo tee -a /etc/hosts
Open https://grafana.local (accept the self-signed cert warning).

The Go service
cmd/internships/ scrapes public job boards and exposes:

Metric	Type	Description
radar_board_jobs{board}	gauge	Total jobs on each board at last scan
radar_board_matches{board}	gauge	Jobs passing filters at last scan
radar_open_matches{category,eligibility}	gauge	Open matches by category and location
radar_new_matches_total	counter	Newly discovered matches
radar_scan_errors_total{board}	counter	Failed board scans
radar_scan_duration_seconds{board}	histogram	Time per board
radar_last_scan_timestamp_seconds	gauge	Unix time of last completed scan
Sources supported: Greenhouse, Lever, Ashby. Board list is in
cmd/internships/boards.txt.

Run tests:

bash
go test ./cmd/internships/...
Azure deployment
Provisioned with Terraform in terraform/.

Prerequisites
Terraform >= 1.5

Azure CLI, logged in (az login)

SSH keypair at ~/.ssh/id_rsa.pub

An Azure for Students subscription

Configuration
Create terraform/terraform.tfvars (gitignored):

hcl
subscription_id     = "<your-subscription-id>"
allowed_ssh_cidr    = "<your-public-ip>/32"
ssh_public_key_path = "~/.ssh/id_rsa.pub"
Get your IP with curl ifconfig.me.

Deploy
bash
cd terraform
terraform init
terraform apply
After ~3 minutes, Terraform prints:

text
public_ip   = "<ip>"
ssh_command = "ssh azureuser@<ip>"
Point DNS at the VM:

bash
sudo sed -i "/grafana.local/d" /etc/hosts
echo "<ip> grafana.local prometheus.local" | sudo tee -a /etc/hosts
SSH in and create .env:

bash
ssh azureuser@<ip>
cd ~/kerionlabs-monitoring
echo 'GRAFANA_ADMIN_PASSWORD=<your-password>' > .env
sudo docker compose up -d
Destroy
bash
cd terraform
terraform destroy
Always destroy when you're done. See "Costs" below.

Costs
Free-tier eligible but not free forever:

Resource	Monthly cost
Standard_B2ats_v2 VM	$0 (within 750 free hours)
Public IP (Standard SKU, static)	~$3.65
OS disk (64 GiB P6)	~$10
Outbound bandwidth	First 15 GB free, then ~$0.12/GB
For a dev deployment, destroy it between sessions. The full cost of
leaving it up for a month is ~$14 — sustainable for a while, but
destroying is cheap and correct.

Region restrictions
Azure for Students subscriptions are restricted to a small list of
regions by policy. southafricanorth is used here because it's closest
to Kenya (~80 ms). Other allowed regions for this subscription:
switzerlandnorth, italynorth, polandcentral, indiasouthcentral.

Verify with:

bash
az policy assignment list \
  --query "[?parameters.listOfAllowedLocations.value!=null].parameters.listOfAllowedLocations.value[]" \
  -o tsv
Grafana provisioning
Three files are required for a fresh Grafana instance to load dashboards
and datasources:

grafana/provisioning/datasources/prometheus.yaml — Prometheus
datasource with a stable uid: prometheus

grafana/provisioning/dashboards/default.yaml — provider config
telling Grafana where to find dashboard JSON

grafana/provisioning/dashboards/*.json — dashboard definitions

Verify the datasource loaded:

bash
curl -sk -u admin:<password> https://grafana.local/api/datasources
An empty array [] means it didn't load.

CI
GitHub Actions runs on every push to main:

go test ./... with race detector

go vet

gofmt format check

Multi-stage Docker build

Trivy CVE scan

Push to ghcr.io

Lessons learned (operational notes)
These are traps that cost real debugging time; recording them here so
future-me doesn't repeat them.

Grafana admin password. GF_SECURITY_ADMIN_PASSWORD in .env
is authoritative on container boot. The CLI command
grafana cli admin reset-admin-password changes it temporarily, but
a restart reverts it to the .env value. To change permanently: edit
.env, run docker compose down -v, then up -d.

Dashboard datasource references. Use the datasource's uid
directly in provisioned dashboards ("uid": "prometheus"), not
${DS_PROMETHEUS}. The placeholder only resolves if the dashboard
defines a templating variable for it — otherwise Grafana passes the
literal string and panels show "No data".

Grafana 13 datasource schema. Include uid and jsonData
(httpMethod, timeInterval) in the datasource YAML. The old schema
triggers a deprecation warning and may fail to load.

Secrets never leave the laptop. .env is gitignored. Every fresh
VM needs it created manually before docker compose up. In
production, use Azure Key Vault + managed identity.

Go 1.22 pin. The Prometheus client libraries are pinned to
versions compatible with Go 1.22 (client_golang v1.20.5,
common v0.55.0, etc.). Newer versions require Go 1.25+, which isn't
available as a stable Docker image.

What's next
CD pipeline — deploy from GitHub Actions after CI succeeds

Cloud-init secrets — pass GRAFANA_ADMIN_PASSWORD as a Terraform
variable so no manual SSH is needed

Kubernetes — the same containers on AKS once horizontal scaling
matters

Multi-cloud — parallel Terraform config for AWS

License
MIT

text

## How to create it

From `~/kerionlabs-monitoring`, run:

```bash
cat > README.md <<'READMEEOF'
<paste the entire block above here>
READMEEOF
Then validate it renders:

bash
head -30 README.md
The READMEEOF marker at the end must be on its own line, no leading spaces. That's the same heredoc rule as before — if it fails, the file will hang waiting for the closing marker. Press Ctrl+C to abort, and try again with the closing READMEEOF on a fresh line.

Then commit
bash
git add README.md
git commit -m "Add project README

Documents architecture, local setup, Azure deployment, costs, and
operational lessons learned from the initial deployment."
git push
Also add .env.example
The README references it. Create it too:

bash
cat > .env.example <<'EOF'
# Copy to .env and fill in real values.
# .env is gitignored — this file is the template that ships in the repo.
GRAFANA_ADMIN_PASSWORD=changeme
EOF

git add .env.example
git commit -m "Add .env.example template"
git push
Check GitHub renders it
Open 
