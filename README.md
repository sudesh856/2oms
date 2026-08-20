# Nephot OMS

## Local backend

Copy `.env.example` to `.env` and set the database and JWT values. Run the migrations, then start the API:

```text
migrate -path database/migrations -database "$DATABASE_URL" up
cd backend
go run ./cmd/server
```

`ALLOWED_ORIGINS` is a comma-separated list of exact frontend origins. Include `http://localhost:5173` for local development. When omitted, the backend allows the deployed frontend origin `https://frontend398745lkajsgd.onrender.com`.

## Frontend

Set `VITE_API_BASE_URL` before building the frontend. The deployed API value is:

```text
VITE_API_BASE_URL=https://twooms.onrender.com/api
```

For local development, use `VITE_API_BASE_URL=http://localhost:8080/api` and run:

```text
cd frontend
npm install
npm run dev
```

## Internal users

On a fresh database, the Login page offers first-time organization setup. The server creates that first account as the Superadmin and logs it in immediately; the request cannot choose a role. After setup, the Superadmin creates Admin and Staff accounts from the Users screen. There is no public signup after the first account exists.

The CLI seed commands remain available for development or recovery, but they require explicit `BOOTSTRAP_*` environment variables and have no default credentials.