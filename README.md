# Deployment Pipeline

A self-hosted mini-deployment platform that demonstrates automated builds, zero-downtime reloads, and real-time log streaming.

Push a button in the browser, and:
1. Railpack will auto-detects the `sample-app` and builds a production-ready Docker image.
2. BuildKit handles the heavy lifting of the container build process.
3. Go API orchestrates the container lifecycle (build → run → health check).
4. Caddy** reloads dynamically via its Admin API for zero-downtime routing.
5. Vite + React** frontend provides a live SSE (Server-Sent Events) log stream.


## 🚀 Quick Start
- Docker** (Desktop or Engine) and Docker Compose

### 2. Launch the Stack
Clone the repository and run:

docker compose up --build -d

This starts four services:
- `api`: The Go backend orchestrator.
- `frontend`: Vite-powered React dashboard.
- `buildkit`: Dedicated build daemon for Railpack.
- `caddy`: Reverse proxy and dynamic router.


## 🧪 How to Test the App

### Step 1: Access the Dashboard
Open your browser and navigate to:
👉 http://localhost:8080


### Step 2: Trigger a Deployment
1. Click the Deploy button.
2. The UI will immediately show a new deployment in a `pending` state.
3. It will transition to `building`. Click on the deployment ID to see the **Live Build Logs**.

### Step 3: Monitor Live Logs
In the log view, you'll see Railpack:
- Detecting the project type (go/typescript).
- Installing dependencies (`npm install`).
- Preparing the start command (`node index.js`).
- Pushing the final image to the local Docker daemon.

### Step 4: Verify the Deployed App
Once the status changes to `running`, the `sample-app` is live!
The pipeline maps the app to an internal port and updates Caddy.

Access the deployed sample app at:
👉 http://localhost/app/

*Note: The trailing slash is important for routing.*

### Step 5: Test Rollback
1. Trigger another deployment (changes the state/image).
2. Once the new deployment is running, go to the previous deployment.
3. Click the **"Rollback"** button.
4. The system will stop the current container and restart the previous image, updating Caddy back to the old version.

---

## 🛠 Architecture & Tech Stack

- Backend**: Go (Gin)
- Frontend**: React (Vite, TanStack Query/Router)
- Build Engine**: [Railpack](https://github.com/railwayapp/railpack) + BuildKit
- Routing**: Caddy (Dynamic configuration via HTTP API)
- Log Streaming**: Server-Sent Events (SSE) with an in-memory ring buffer.

### Project Structure(tree)
.
├── api/             # Go Backend
│   ├── src/api/     # Handlers, Controllers, Repo
│   └── src/services/# Railpack, Docker, Caddy integrations
├── frontend/        # React Frontend
├── sample-app/      # Simple Node.js app to be deployed
├── Caddyfile        # Initial Caddy configuration
└── docker-compose.yml


## 🔍 Troubleshooting

**Q: Build fails with "BUILDKIT_HOST" errors?**
A: Ensure the `buildkit` container is running (`docker compose ps`). The `api` service is configured to talk to it over the internal Docker network.

**Q: Cannot access http://localhost/app/?**
A: Check the API logs (`docker compose logs -f api`). Ensure the health check passed. If the health check fails, Caddy won't be reloaded.

Logs are empty or disconnected?**
A: The backend uses an in-memory ring buffer. If you restart the `api` container, previous logs are cleared. Ensure your browser supports Server-Sent Events.

