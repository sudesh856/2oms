import { useEffect, useState } from "react";
import { jwtDecode } from "jwt-decode";
import {
  BrowserRouter,
  Link,
  Navigate,
  Route,
  Routes,
} from "react-router-dom";

type TokenPayload = {
  role: "superadmin" | "admin" | "staff";
};

type Order = {
  ID: string;
  CustomerID: string;
  Source: string;
  Status: string;
  CourierID: string | null;
  LocationID: string | null;
  Address: string;
  CodAmount: unknown;
  IsStoreVisit: boolean;
  CreatedBy: string;
  CreatedAt: string;
  UpdatedAt: string;
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

function Layout({ children }: { children: React.ReactNode }) {
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
          <Link to="/">Dashboard</Link>

          <Link to="/orders">Orders</Link>
          <Link to="/orders/new">Create Order</Link>

          {(role === "admin" || role === "superadmin") && (
            <span>Admin</span>
          )}

          <span>{role ?? "Unknown role"}</span>

          <button type="button" onClick={logout}>
            Logout
          </button>
        </nav>
      </header>

      {children}
    </div>
  );
}

function Dashboard() {
  return (
    <Layout>
      <main>
        <h2>Dashboard</h2>
        <p>Welcome to Nephot OMS.</p>
      </main>
    </Layout>
  );
}

function CreateOrder() {
  const [customerId, setCustomerId] = useState("");
  const [source, setSource] = useState("phone");
  const [address, setAddress] = useState("");
  const [codAmount, setCodAmount] = useState("");
  const [isStoreVisit, setIsStoreVisit] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault();

    setMessage("");
    setError("");

    const token = localStorage.getItem("token");

    if (!token) {
      setError("You are not authenticated.");
      return;
    }

    try {
      const response = await fetch(
        "http://localhost:8080/api/orders",
        {
          method: "POST",
          headers: {
            Authorization: `Bearer ${token}`,
            "Content-Type": "application/json",
          },
          body: JSON.stringify({
            customer_id: customerId,
            source,
            address,
            cod_amount: codAmount,
            is_store_visit: isStoreVisit,
          }),
        }
      );

      if (!response.ok) {
        const text = await response.text();
        throw new Error(text || `Failed to create order (${response.status})`);
      }

      const order = await response.json();

      
      setMessage(`Order created successfully: ${order.ID}`);

      setCustomerId("");
      setSource("phone");
      setAddress("");
      setCodAmount("");
      setIsStoreVisit(false);
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Failed to create order"
      );
    }
  }

  return (
    <main>
      <h2>Create Order</h2>

      <form onSubmit={handleSubmit}>
        <div>
          <label htmlFor="customerId">Customer ID</label>
          <input
            id="customerId"
            type="text"
            value={customerId}
            onChange={(event) => setCustomerId(event.target.value)}
            required
          />
        </div>

        <div>
          <label htmlFor="source">Source</label>
          <select
            id="source"
            value={source}
            onChange={(event) => setSource(event.target.value)}
          >
            <option value="website">Website</option>
            <option value="daraz">Daraz</option>
            <option value="phone">Phone</option>
            <option value="facebook">Facebook</option>
            <option value="instagram">Instagram</option>
            <option value="store">Store</option>
          </select>
        </div>

        <div>
          <label htmlFor="address">Address</label>
          <input
            id="address"
            type="text"
            value={address}
            onChange={(event) => setAddress(event.target.value)}
            required
          />
        </div>

        <div>
          <label htmlFor="codAmount">COD Amount</label>
          <input
            id="codAmount"
            type="number"
            step="0.01"
            min="0"
            value={codAmount}
            onChange={(event) => setCodAmount(event.target.value)}
            required
          />
        </div>

        <div>
          <label>
            <input
              type="checkbox"
              checked={isStoreVisit}
              onChange={(event) =>
                setIsStoreVisit(event.target.checked)
              }
            />
            Store visit
          </label>
        </div>

        {error && <p>{error}</p>}
        {message && <p>{message}</p>}

        <button type="submit">Create Order</button>
      </form>
    </main>
  );
}

function Orders() {
  const [orders, setOrders] = useState<Order[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    async function loadOrders() {
      const token = localStorage.getItem("token");

      if (!token) {
        setError("You are not authenticated.");
        setLoading(false);
        return;
      }

      try {
        const response = await fetch(
          "http://localhost:8080/api/orders",
          {
            method: "GET",
            headers: {
              Authorization: `Bearer ${token}`,
            },
          }
        );

        if (!response.ok) {
          throw new Error(`Failed to load orders (${response.status})`);
        }

        const data = await response.json();

        setOrders(data);
      } catch (err) {
        setError(
          err instanceof Error
            ? err.message
            : "Failed to load orders"
        );
      } finally {
        setLoading(false);
      }
    }

    loadOrders();
  }, []);

  return (
    <Layout>
      <main>
        <h2>Orders</h2>

        {loading && <p>Loading orders...</p>}

        {error && <p>{error}</p>}

        {!loading && !error && orders.length === 0 && (
          <p>No orders found.</p>
        )}

        {!loading && !error && orders.length > 0 && (
          <div>
            {orders.map((order) => (
              <article key={order.ID}>
  <h3>Order {order.ID}</h3>

  <p>
    <strong>Status:</strong> {order.Status}
  </p>

  <p>
    <strong>Source:</strong> {order.Source}
  </p>

  <p>
    <strong>Address:</strong> {order.Address}
  </p>

  <p>
    <strong>Store visit:</strong>{" "}
    {order.IsStoreVisit ? "Yes" : "No"}
  </p>
</article>
            ))}
          </div>
        )}
      </main>
    </Layout>
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

        <Route
          path="/orders"
          element={
            <ProtectedRoute>
              <Orders />
            </ProtectedRoute>
          }
        />

        <Route
          path="/orders/new"
          element={
            <ProtectedRoute>
              <Layout>
                <CreateOrder />
              </Layout>
            </ProtectedRoute>
          }
        />
      </Routes>
    </BrowserRouter>
  );
}