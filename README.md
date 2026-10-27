# IntentOS — Deterministic AI Planning & Scheduling System

IntentOS is a personal planning and execution system that converts messy human intentions into clear, achievable next steps and schedules them with a deterministic topological planner.

---

## Architecture Overview

- **Backend**: Go 1.22+ (`net/http`, `go-chi/chi/v5`, `jackc/pgx/v5/pgxpool`, `golang-jwt/jwt/v5`, `rs/zerolog`).
  - **Deterministic Planner**: Kahn's topological sorting algorithm with dependency resolution, cycle detection, completed prerequisite tolerance, and deadline evaluation.
  - **Safe Database Migrations**: Transactional migration runner with PostgreSQL advisory locking (`pg_advisory_lock`), `schema_migrations` tracking, and automatic legacy baseline reconciliation.
- **Frontend**: React 18, TypeScript, Vite, Tailwind CSS / custom design system with dark mode glassmorphic interface, interactive KPI overview, Intention management, Task tracking, and Daily Execution Timeline.
- **Database**: PostgreSQL 14+ on port 5432.

---

## Local Development & Setup

### 1. Database Configuration
Ensure PostgreSQL is running locally and credentials match `backend/.env`:
```powershell
# Copy example environment configuration
Copy-Item backend/.env.example backend/.env
```
Default connection parameters in `backend/.env`:
```env
PORT=8080
DB_HOST=localhost
DB_PORT=5432
DB_USER=intentos
DB_PASSWORD=secret
DB_NAME=intentos
JWT_SECRET=local-dev-secret-change-this-before-deployment
```

### 2. Run the Backend
```powershell
cd backend
go run ./cmd/server
```
The server will automatically run migrations (or reconcile existing schemas), record them in `schema_migrations`, and listen on `http://localhost:8080`.

### 3. Run the Frontend
```powershell
cd frontend
npm install
npm run dev
```
Open your browser at **`http://localhost:5173`**.
Vite automatically proxies API requests (`/api/*`) to `http://localhost:8080/*`.

---

## Running Tests

### Backend Tests
```powershell
cd backend
go test -v ./...
```
Runs unit tests for password hashing, JWT token generation/validation, topological sort ordering, dependency resolution, completed prerequisite handling, and cycle detection.

### Frontend Tests
```powershell
cd frontend
npm test
```
Runs Jest unit tests for state validation, localStorage auth persistence, task filtering logic, priority mapping, duration calculations, and natural language heuristic parsing.

### Production Builds
```powershell
# Backend binary
cd backend
go build -o server.exe ./cmd/server

# Frontend bundle
cd frontend
npm run build
```

---

## API Endpoints

### Authentication
- `POST /auth/register` — Register a new account (returns JWT token and user info).
- `POST /auth/login` — Sign in with email and password (returns JWT token and user info).

### Intentions (Protected)
- `GET /intentions` — List authenticated user's intentions.
- `POST /intentions` — Create an intention (`{ title, description }`).
- `GET /intentions/{id}` — Get single intention details.
- `PUT /intentions/{id}` — Update intention.
- `DELETE /intentions/{id}` — Delete intention and cascade linked tasks.

### Tasks (Protected)
- `GET /tasks` — List tasks (supports query filters `?intention_id=...&status=...`).
- `POST /tasks` — Create task (`{ intention_id, title, description, estimated_duration, priority, deadline }`).
- `GET /tasks/{id}` — Retrieve task.
- `PUT /tasks/{id}` — Update task (supports partial updates and toggling `status`).
- `DELETE /tasks/{id}` — Delete task.
- `POST /tasks/{id}/dependencies` — Add a task dependency (`{ depends_on_task_id }`).
- `DELETE /tasks/{id}/dependencies/{depId}` — Remove a task dependency.

### Planner (Protected)
- `POST /plan/generate` — Generate chronological schedule blocks for pending tasks.
- `POST /plan/replan` — Clear future blocks and generate fresh schedule.
- `GET /plan/blocks` — List scheduled time slots.
- `GET /plan/history` — List plan revision history.
