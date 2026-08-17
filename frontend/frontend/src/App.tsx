import { useState } from "react";
import { jwtDecode } from "jwt-decode";
import {
  BrowserRouter,
  Navigate,
  Route,
  Routes,
} from "react-router-dom";

function Login() {
  const [phone, setPhone] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");

  async function handleLogin(event: React.FormEvent) {
    event.preventDefault();
    setError("");

    try {
      const response = await fetch(
        "http://localhost:8080/api/auth/login",
        {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          body: JSON.stringify({
            phone,
            password,
          }),
        }
      );

      if (!response.ok) {
        throw new Error("Invalid phone number or password");
      }

      const data = await response.json();

      localStorage.setItem("token", data.token);

      window.location.href = "/";
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Login failed"
      );
    }
  }

  return (
    <div>
      <h1>Nephot OMS</h1>

      <form onSubmit={handleLogin}>
        <div>
          <label htmlFor="phone">Phone</label>
          <input
            id="phone"
            type="tel"
            value={phone}
            onChange={(event) => setPhone(event.target.value)}
            required
          />
        </div>

        <div>
          <label htmlFor="password">Password</label>
          <input
            id="password"
            type="password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            required
          />
        </div>

        {error && <p>{error}</p>}

        <button type="submit">Login</button>
      </form>
    </div>
  );
}


type TokenPayload = {
  role: "superadmin" | "admin" | "staff";
};

function getRole(): TokenPayload["role"] | null {
  const token = localStorage.getItem("token");

  if (!token) {
    return null;
  }

  try {
    const decoded = jwtDecode<TokenPayload>(token);
    return decoded.role ?? null;
  } catch {
    return null;
  }
}

function Dashboard() {
  const role = getRole();

  function logout() {
    localStorage.removeItem("token");
    window.location.href = "/login";
  }

  return (
    <div>
      <header>
        <h1>Nephot OMS</h1>

        <nav>
         <a href="/">Dashboard</a>

{(role === "admin" || role === "superadmin") && (
  <span>Admin</span>
)}

<span>{role ?? "Unknown role"}</span>

<button type="button" onClick={logout}>
            Logout
          </button>
        </nav>
      </header>

      <main>
        <h2>Dashboard</h2>
        <p>Welcome to Nephot OMS.</p>
      </main>
    </div>
  );
}

function ProtectedRoute({
  children,
}: {
  children: React.ReactNode;
}) {
  const token = localStorage.getItem("token");

  if (!token) {
    return <Navigate to="/login" replace />;
  }

  return children;
}

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<Login />} />

        <Route
          path="/"
          element={
            <ProtectedRoute>
              <Dashboard />
            </ProtectedRoute>
          }
        />
      </Routes>
    </BrowserRouter>
  );
}