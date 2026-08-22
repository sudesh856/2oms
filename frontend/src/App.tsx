import { useEffect, useMemo, useRef, useState } from "react";
import { jwtDecode } from "jwt-decode";
import {
  BrowserRouter,
  Link,
  Navigate,
  Route,
  Routes,
  useNavigate,
  useParams,
} from "react-router-dom";
import { Cell, Pie, PieChart, ResponsiveContainer, Tooltip } from "recharts";
import {
  calculateCartTotal,
  filterCourierLocations,
  getValidNextStatuses,
  normalizeCustomer,
  normalizeCourier,
  normalizeCourierLocation,
  normalizeOrder,
  normalizeOrderItem,
  normalizeProduct,
  type CartItem,
  type Courier,
  type CourierLocation,
  type Customer,
  type Order,
  type OrderItem,
  type Product,
} from "./orderHelpers";
const API = import.meta.env.VITE_API_BASE_URL || "http://localhost:8080/api";

type Role = "superadmin" | "admin" | "staff";

type TokenPayload = {
  role: Role;
  user_id?: string;
  exp?: number;
};

type FollowUp = {
  id: string;
  order_id: string;
  attempt_no: number;
  next_action: string;
  next_action_date?: string | null;
  note?: string | null;
  customer_name: string;
  customer_phone: string;
  assigned_to_name?: string | null;
};


type DashboardSummary = {
  today_orders: number;
  pending_confirmations: number;
  problem_orders: number;
  total_orders: number;
  confirmed_orders: number;
  cancelled_orders: number;
  delivered_orders: number;
  status_counts: Array<{ status: string; count: number }>;
  courier_counts: Array<{ CourierName?: string | null; Count: number }>;
  follow_ups_due: Array<{
    ID: string;
    OrderID: string;
    NextAction: string;
    NextActionDate: string;
    Note: string;
    CustomerName: string;
    CustomerPhone: string;
  }>;
};

type StaffPerformance = {
  UserID: string;
  Name: string;
  Role: Role;
  CallsMade: number;
  OrdersConfirmed: number;
  OrdersCancelled: number;
  FollowUpsLogged: number;
};

type ConfirmedCourierCount = {
  CourierName: string;
  LocationName?: string | null;
  OrderCount: number;
  ProductCount: number;
};

type User = {
  id: string;
  name: string;
  phone: string;
  role: Role;
  status?: string;
  is_active: boolean;
};

const retryListeners = new Set<(retrying: boolean) => void>();
let networkRetrying = false;

function setNetworkRetrying(value: boolean) {
  networkRetrying = value;
  retryListeners.forEach((listener) => listener(value));
}

function useNetworkRetrying() {
  const [retrying, setRetrying] = useState(networkRetrying);

  useEffect(() => {
    retryListeners.add(setRetrying);
    return () => {
      retryListeners.delete(setRetrying);
    };
  }, []);

  return retrying;
}

function NetworkRetryNotice() {
  const retrying = useNetworkRetrying();

  if (!retrying) return null;

  return <div className="alert network-retry">Can't reach the server — it may be waking up, retrying...</div>;
}

function wait(milliseconds: number) {
  return new Promise((resolve) => window.setTimeout(resolve, milliseconds));
}

async function fetchWithRetry(
  input: RequestInfo | URL,
  options: RequestInit,
) {
  for (let attempt = 0; attempt < 3; attempt += 1) {
    try {
      const response = await fetch(input, options);
      setNetworkRetrying(false);
      return response;
    } catch (error) {
      if (attempt === 2) {
        setNetworkRetrying(false);
        throw new Error("Can't reach the server after retrying. Please try again.");
      }

      setNetworkRetrying(true);
      await wait(500);

      if (!(error instanceof TypeError)) {
        throw error;
      }
    }
  }

  throw new Error("Can't reach the server after retrying. Please try again.");
}

function token() {
  return localStorage.getItem("token");
}

function getRole(): Role | null {
  const value = token();

  if (!value) return null;

  try {
    return jwtDecode<TokenPayload>(value).role ?? null;
  } catch {
    return null;
  }
}

async function apiFetch(path: string, options: RequestInit = {}) {
  const currentToken = token();

  const headers = new Headers(options.headers);

  if (currentToken) {
    headers.set("Authorization", `Bearer ${currentToken}`);
  }

  if (options.body && !(options.body instanceof FormData) && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }

  const response = await fetchWithRetry(`${API}${path}`, {
    ...options,
    headers,
  });

  if (response.status === 401) {
    localStorage.removeItem("token");
    window.location.href = "/login";
    throw new Error("Session expired");
  }

  return response;
}

async function readError(response: Response) {
  const text = await response.text();

  if (!text) {
    return `Request failed (${response.status})`;
  }

  try {
    const data = JSON.parse(text);

    return (
      data.message ||
      data.error ||
      data.detail ||
      text
    );
  } catch {
    return text;
  }
}

function money(value: unknown) {
  if (value === null || value === undefined || value === "") {
    return "—";
  }

  return String(value);
}

function formatDate(value: string) {
  if (!value) return "—";

  return new Date(value).toLocaleString();
}

function frontendInvitationURL(value: unknown) {
  if (typeof value !== "string" || !value) return "";

  try {
    const invitation = new URL(value, window.location.origin);
    return `${window.location.origin}${invitation.pathname}${invitation.search}${invitation.hash}`;
  } catch {
    return "";
  }
}

function statusLabel(status?: string) {
  return (status ?? "unknown").replaceAll("_", " ");
}

function Login() {
  const navigate = useNavigate();

  const [setupMode, setSetupMode] = useState(false);
  const [setupName, setSetupName] = useState("");
  const [companyName, setCompanyName] = useState("");
  const [phone, setPhone] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");

  const [loading, setLoading] = useState(false);

  async function handleLogin(event: React.FormEvent) {
    event.preventDefault();

    setLoading(true);
    setError("");

    try {
      const response = await fetchWithRetry(`${API}/auth/login`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          phone,
          password,
        }),
      });

      if (!response.ok) {
        throw new Error(await readError(response));
      }

      const data = await response.json();

      localStorage.setItem("token", data.token);

      navigate("/");
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Login failed"
      );
    } finally {
      setLoading(false);
    }
  }

  async function handleSetup(event: React.FormEvent) {
    event.preventDefault();
    setLoading(true);
    setError("");

    try {
      const response = await fetchWithRetry(`${API}/auth/setup`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          name: setupName,
          company_name: companyName,
          phone,
          password,
        }),
      });

      if (!response.ok) {
        throw new Error(await readError(response));
      }

      const data = await response.json();
      localStorage.setItem("token", data.token);
      navigate("/");
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Failed to create organization account"
      );
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="login-page">
      <div className="login-card">
        <NetworkRetryNotice />
        <div className="brand-mark">N</div>

        <p className="eyebrow">NEPHOT</p>
        <h1>{setupMode ? "Create your organization account" : "Order Management"}</h1>
        <p className="muted">
          {setupMode
            ? "Set up the first owner account for your OMS."
            : "Sign in to manage customers, products and orders."}
        </p>

        <form onSubmit={setupMode ? handleSetup : handleLogin} className="stack">
          {setupMode && (
            <label>
              Company name
              <input
                value={companyName}
                onChange={(event) => setCompanyName(event.target.value)}
                required
              />
            </label>
          )}

          {setupMode && (
            <label>
              Name
              <input
                value={setupName}
                onChange={(event) => setSetupName(event.target.value)}
                required
              />
            </label>
          )}

          <label>
            Phone
            <input
              type="tel"
              value={phone}
              onChange={(event) => setPhone(event.target.value)}
              placeholder="98XXXXXXXX"
              required
            />
          </label>

          <label>
            Password
            <input
              type="password"
              value={password}
              onChange={(event) =>
                setPassword(event.target.value)
              }
              placeholder="Password"
              required
            />
          </label>

          {error && <div className="alert error">{error}</div>}

          <button
            className="button primary full"
            type="submit"
            disabled={loading}
          >
            {loading
              ? setupMode ? "Creating account..." : "Signing in..."
              : setupMode ? "Create organization account" : "Sign in"}
          </button>
        </form>

        {setupMode ? (
          <button
            className="button ghost full"
            type="button"
            onClick={() => {
              setSetupMode(false);
              setError("");
            }}
          >
            Back to sign in
          </button>
        ) : (
          <button
            className="button ghost full"
            type="button"
            onClick={() => setSetupMode(true)}
          >
            Create organization account
          </button>
        )}
      </div>
    </div>
  );
}

function Invitation() {
  const { token: invitationToken } = useParams();
  const navigate = useNavigate();
  const [name, setName] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [activated, setActivated] = useState(false);

  useEffect(() => {
    if (!invitationToken) {
      setError("Invitation is invalid or expired.");
      setLoading(false);
      return;
    }

    fetchWithRetry(`${API}/auth/invitation/${invitationToken}`, {})
      .then(async (response) => {
        if (!response.ok) throw new Error(await readError(response));
        const data = await response.json();
        setName(data.name || "");
      })
      .catch((err) => setError(err instanceof Error ? err.message : "Invitation is invalid or expired."))
      .finally(() => setLoading(false));
  }, [invitationToken]);

  async function activate(event: React.FormEvent) {
    event.preventDefault();
    if (password !== confirmPassword) {
      setError("Passwords do not match.");
      return;
    }
    if (!invitationToken) return;

    setSaving(true);
    setError("");
    try {
      const response = await fetchWithRetry(`${API}/auth/accept-invitation/${invitationToken}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ password }),
      });
      if (!response.ok) throw new Error(await readError(response));
      setActivated(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to activate account");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="login-page">
      <div className="login-card">
        <div className="brand-mark">N</div>
        {activated ? (
          <>
            <p className="eyebrow">Nephot OMS</p>
            <h1>Account activated</h1>
            <p className="muted">Your account is ready. Sign in with the password you created.</p>
            <button className="button primary full" type="button" onClick={() => navigate("/login")}>Go to sign in</button>
          </>
        ) : loading ? (
          <p className="muted">Checking invitation...</p>
        ) : error && !name ? (
          <div className="alert error">{error}</div>
        ) : (
          <>
            <p className="eyebrow">Nephot OMS</p>
            <h1>You've been invited</h1>
            <p className="muted">Create your own password to activate this account.</p>
            <form className="stack" onSubmit={activate}>
              <label>
                Name
                <input value={name} readOnly />
              </label>
              <label>
                Create password
                <input type="password" minLength={8} value={password} onChange={(event) => setPassword(event.target.value)} required />
              </label>
              <label>
                Confirm password
                <input type="password" minLength={8} value={confirmPassword} onChange={(event) => setConfirmPassword(event.target.value)} required />
              </label>
              {error && <div className="alert error">{error}</div>}
              <button className="button primary full" type="submit" disabled={saving}>{saving ? "Activating..." : "Activate account"}</button>
            </form>
          </>
        )}
      </div>
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
    <div className="app-shell">
      <aside className="sidebar">
        <div className="sidebar-brand">
          <div className="brand-mark small">N</div>
          <div>
            <strong>Nephot</strong>
            <span>OMS</span>
          </div>
        </div>

        <nav className="sidebar-nav">
          <Link to="/">Dashboard</Link>
          <Link to="/customers">Customers</Link>
          <Link to="/products">Products</Link>
          <Link to="/orders">Orders</Link>
          <Link to="/orders/new">Create Order</Link>
          <Link to="/followups">Follow-ups</Link>
          {(role === "admin" || role === "superadmin") && (
            <Link to="/couriers">Couriers</Link>
          )}
          {(role === "admin" || role === "superadmin") && <Link to="/users">Users</Link>}
          {role === "superadmin" && <Link to="/imports">Import history</Link>}
        </nav>

        <div className="sidebar-bottom">
          <div className="role-box">
            <span>Signed in as</span>
            <strong>{role ?? "Unknown"}</strong>
          </div>

          <button
            type="button"
            className="button ghost full"
            onClick={logout}
          >
            Logout
          </button>
        </div>
      </aside>

      <div className="main-shell">
        <header className="topbar">
          <div>
            <span className="eyebrow">OPERATIONS</span>
          </div>

          <div className="topbar-role">
            {role}
          </div>
        </header>

        <main className="content">
          <NetworkRetryNotice />
          {children}
        </main>
      </div>
    </div>
  );
}

function Dashboard() {
  const [orders, setOrders] = useState<Order[]>([]);
  const [products, setProducts] = useState<Product[]>([]);
  const [summary, setSummary] = useState<DashboardSummary | null>(null);
  const [staffPerformance, setStaffPerformance] = useState<StaffPerformance[]>([]);
  const [confirmedCourierCounts, setConfirmedCourierCounts] = useState<ConfirmedCourierCount[]>([]);
  const [dashboardTab, setDashboardTab] = useState<"overview" | "reports">("overview");
  const [loading, setLoading] = useState(true);
  const canViewReports = ["admin", "superadmin"].includes(getRole() ?? "");

  useEffect(() => {
    async function load() {
      try {
        const [ordersResponse, productsResponse, summaryResponse] =
          await Promise.all([
            apiFetch("/orders"),
            apiFetch("/products"),
            apiFetch("/dashboard/summary"),
          ]);

        if (
          !ordersResponse.ok ||
          !productsResponse.ok
        ) {
          throw new Error("Failed to load dashboard");
        }

        setOrders(await ordersResponse.json());
        setProducts(await productsResponse.json());
        if (summaryResponse.ok) setSummary(await summaryResponse.json());
        if (canViewReports) {
          const [staffResponse, courierResponse] = await Promise.all([
            apiFetch("/reports/staff-performance"),
            apiFetch("/reports/confirmed-courier-wise"),
          ]);
          if (staffResponse.ok) setStaffPerformance(await staffResponse.json());
          if (courierResponse.ok) setConfirmedCourierCounts(await courierResponse.json());
        }
      } catch {
        // Individual pages expose detailed errors.
      } finally {
        setLoading(false);
      }
    }

    load();
  }, [canViewReports]);

  const activeOrders = orders.filter(
    (order) =>
      !["delivered", "cancelled", "returned"].includes(order.status)
  ).length;

  return (
    <Layout>
      <PageHeader
        eyebrow="Dashboard"
        title="Operations overview"
        description="A quick view of your current order management activity."
      />

      {loading ? (
        <div className="card">Loading dashboard...</div>
      ) : (
        <>
          <section className="stats-grid">
            <StatCard
              label="Today's orders"
              value={summary?.today_orders ?? orders.length}
              href="/orders"
            />
            <StatCard
              label="Pending confirmations"
              value={summary?.pending_confirmations ?? activeOrders}
              href="/orders"
            />
            <StatCard
              label="Follow-up / problem"
              value={summary?.problem_orders ?? 0}
              href="/orders/problems"
            />
            <StatCard
              label="Total orders"
              value={summary?.total_orders ?? orders.length}
              href="/orders"
            />
            <StatCard
              label="Confirmed orders"
              value={summary?.confirmed_orders ?? 0}
              href="/orders?status=confirmed"
            />
            <StatCard
              label="Cancelled orders"
              value={summary?.cancelled_orders ?? 0}
              href="/orders?status=cancelled"
            />
            <StatCard
              label="Delivered orders"
              value={summary?.delivered_orders ?? 0}
              href="/orders?status=delivered"
            />
          </section>

          {canViewReports && (
            <section className="card dashboard-tabs">
              <div className="tab-list" role="tablist" aria-label="Dashboard views">
                <button className={`tab-button ${dashboardTab === "overview" ? "active" : ""}`} onClick={() => setDashboardTab("overview")} role="tab" aria-selected={dashboardTab === "overview"}>Overview</button>
                <button className={`tab-button ${dashboardTab === "reports" ? "active" : ""}`} onClick={() => setDashboardTab("reports")} role="tab" aria-selected={dashboardTab === "reports"}>Staff & courier reports</button>
              </div>
            </section>
          )}

          {dashboardTab === "overview" ? <>
          <section className="card">
            <div className="section-heading">
              <div><span className="eyebrow">Pipeline</span><h2>Current status counts</h2></div>
              <Link to="/orders/problems" className="text-link">Problem orders</Link>
            </div>
            <div className="compact-list">
              {(summary?.status_counts ?? []).map((item) => <div className="compact-row" key={item.status}><StatusBadge status={item.status} /><strong>{item.count}</strong></div>)}
            </div>
          </section>

          <section className="dashboard-grid">
            <div className="card">
              <div className="section-heading">
                <div><span className="eyebrow">Calls</span><h2>Follow-ups due today</h2></div>
                <Link to="/followups" className="text-link">View all</Link>
              </div>
              {(summary?.follow_ups_due ?? []).length === 0 ? (
                <p className="muted">No follow-ups due today.</p>
              ) : (
                <div className="compact-list">
                  {(summary?.follow_ups_due ?? []).slice(0, 5).map((followUp) => (
                    <Link className="compact-row" to={`/orders/${followUp.OrderID}`} key={followUp.ID}>
                      <div><strong>{followUp.CustomerName}</strong><span>{followUp.CustomerPhone} · {followUp.NextAction}</span></div>
                      <span>{followUp.Note || "-"}</span>
                    </Link>
                  ))}
                </div>
              )}
            </div>

            <div className="card">
              <div className="section-heading">
                <div><span className="eyebrow">Channels</span><h2>By courier</h2></div>
                <Link to="/orders" className="text-link">Orders</Link>
              </div>
              {(summary?.courier_counts ?? []).length === 0 ? (
                <p className="muted">No courier counts today.</p>
              ) : (
                <div className="compact-list">
                  {(summary?.courier_counts ?? []).map((courier) => (
                    <div className="compact-row" key={courier.CourierName ?? "unassigned"}>
                      <strong>{courier.CourierName || "Unassigned"}</strong>
                      <span>{courier.Count}</span>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </section>

          <section className="dashboard-grid">
            <div className="card">
              <div className="section-heading">
                <div>
                  <span className="eyebrow">Recent</span>
                  <h2>Latest orders</h2>
                </div>
                <Link to="/orders" className="text-link">
                  View all
                </Link>
              </div>

              {orders.length === 0 ? (
                <EmptyState message="No orders yet." />
              ) : (
                <div className="compact-list">
                  {orders.slice(0, 5).map((order) => (
                    <Link
                      className="compact-row"
                      to={`/orders/${order.id}`}
                      key={order.id}
                    >
                      <div>
                        <strong>
                          {order.id.slice(0, 8)}
                        </strong>
                        <span>
                          {order.source} · {formatDate(order.created_at)}
                        </span>
                      </div>

                      <StatusBadge status={order.status} />
                    </Link>
                  ))}
                </div>
              )}
            </div>

            <div className="card">
              <div className="section-heading">
                <div>
                  <span className="eyebrow">Inventory</span>
                  <h2>Stock watch</h2>
                </div>
                <Link to="/products" className="text-link">
                  Products
                </Link>
              </div>

              {products.length === 0 ? (
                <EmptyState message="No products yet." />
              ) : (
                <div className="compact-list">
                  {products.slice(0, 5).map((product) => (
                    <div className="compact-row" key={product.id}>
                      <div>
                        <strong>{product.name}</strong>
                        <span>
                          Rs. {money(product.price)}
                        </span>
                      </div>

                      <span
                        className={
                          product.available_qty <= 5
                            ? "stock low"
                            : "stock"
                        }
                      >
                        {product.available_qty} available
                      </span>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </section>
          </> : (
            <DashboardReports staffPerformance={staffPerformance} confirmedCourierCounts={confirmedCourierCounts} />
          )}
        </>
      )}
    </Layout>
  );
}

function StatCard({
  label,
  value,
  href,
}: {
  label: string;
  value: number;
  href: string;
}) {
  return (
    <Link to={href} className="stat-card">
      <span>{label}</span>
      <strong>{value}</strong>
    </Link>
  );
}

function DashboardReports({
  staffPerformance,
  confirmedCourierCounts,
}: {
  staffPerformance: StaffPerformance[];
  confirmedCourierCounts: ConfirmedCourierCount[];
}) {
  const chartData = staffPerformance
    .filter((staff) => staff.CallsMade > 0)
    .map((staff) => ({ name: staff.Name, value: staff.CallsMade }));
  const colors = ["#2563eb", "#0f766e", "#d97706", "#b42318", "#7c3aed"];

  return (
    <section className="dashboard-grid dashboard-reports">
      <div className="card">
        <div className="section-heading">
          <div><span className="eyebrow">Staff performance</span><h2>Calls by staff member</h2></div>
        </div>
        {chartData.length === 0 ? <p className="muted">No follow-up attempts recorded yet.</p> : (
          <div className="report-chart">
            <ResponsiveContainer width="100%" height={240}>
              <PieChart>
                <Pie data={chartData} dataKey="value" nameKey="name" innerRadius={55} outerRadius={85} paddingAngle={2}>
                  {chartData.map((item, index) => <Cell key={item.name} fill={colors[index % colors.length]} />)}
                </Pie>
                <Tooltip />
              </PieChart>
            </ResponsiveContainer>
          </div>
        )}
        <div className="compact-list">
          {staffPerformance.length === 0 ? <p className="muted">No staff members yet.</p> : staffPerformance.map((staff) => (
            <div className="compact-row" key={staff.UserID}>
              <div><strong>{staff.Name}</strong><span>{staff.Role} · {staff.CallsMade} calls</span></div>
              <span>{staff.OrdersConfirmed} confirmed · {staff.OrdersCancelled} cancelled</span>
            </div>
          ))}
        </div>
      </div>

      <div className="card">
        <div className="section-heading">
          <div><span className="eyebrow">Confirmed orders</span><h2>Courier and location</h2></div>
        </div>
        {confirmedCourierCounts.length === 0 ? <p className="muted">No confirmed courier orders yet.</p> : (
          <div className="compact-list">
            {confirmedCourierCounts.map((item) => (
              <div className="compact-row" key={`${item.CourierName}-${item.LocationName ?? "unassigned"}`}>
                <div><strong>{item.CourierName}</strong><span>{item.LocationName || "Unassigned location"}</span></div>
                <span>{item.OrderCount} orders · {item.ProductCount} products</span>
              </div>
            ))}
          </div>
        )}
      </div>
    </section>
  );
}

function PageHeader({
  eyebrow,
  title,
  description,
  action,
}: {
  eyebrow: string;
  title: string;
  description?: string;
  action?: React.ReactNode;
}) {
  return (
    <div className="page-header">
      <div>
        <span className="eyebrow">{eyebrow}</span>
        <h1>{title}</h1>
        {description && <p className="muted">{description}</p>}
      </div>

      {action}
    </div>
  );
}

function Customers() {
  const [customers, setCustomers] = useState<Customer[]>([]);
  const [search, setSearch] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  async function loadCustomers(value = search) {
    setLoading(true);
    setError("");

    try {
      const response = await apiFetch(
        `/customers?search=${encodeURIComponent(value)}`
      );

      if (!response.ok) {
        throw new Error(await readError(response));
      }

      const data = await response.json();
      setCustomers(data.map(normalizeCustomer));
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Failed to load customers"
      );
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    loadCustomers("");
  }, []);

  return (
    <Layout>
      <PageHeader
        eyebrow="Customers"
        title="Customer directory"
        description="Search and manage your customer records."
        action={
          <Link to="/customers/new" className="button primary">
            New customer
          </Link>
        }
      />

      <div className="card">
        <form
          className="search-row"
          onSubmit={(event) => {
            event.preventDefault();
            loadCustomers();
          }}
        >
          <input
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder="Search by phone or name..."
          />
          <button className="button secondary" type="submit">
            Search
          </button>
        </form>
      </div>

      {error && <div className="alert error">{error}</div>}

      <div className="card table-card">
        {loading ? (
          <p className="muted">Loading customers...</p>
        ) : customers.length === 0 ? (
          <EmptyState message="No customers found." />
        ) : (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Phone</th>
                  <th>Address</th>
                  <th />
                </tr>
              </thead>

              <tbody>
                {customers.map((customer) => (
                  <tr key={customer.id}>
                    <td>
                      <strong>{customer.name}</strong>
                    </td>
                    <td>{customer.phone}</td>
                    <td>{customer.address || "—"}</td>
                    <td>
                      <Link
                        className="text-link"
                        to={`/customers/${customer.id}`}
                      >
                        View
                      </Link>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </Layout>
  );
}

function CustomerForm() {
  const navigate = useNavigate();

  const [phone, setPhone] = useState("");
  const [phone2, setPhone2] = useState("");
  const [name, setName] = useState("");
  const [address, setAddress] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function submit(event: React.FormEvent) {
    event.preventDefault();

    setLoading(true);
    setError("");

    try {
      const response = await apiFetch("/customers", {
        method: "POST",
        body: JSON.stringify({
          phone,
          phone2: phone2 || null,
          name,
          address: address || null,
        }),
      });

      if (!response.ok) {
        throw new Error(await readError(response));
      }

      const customer = normalizeCustomer(await response.json());

      if (!customer.id) {
        throw new Error("Customer was created without an ID");
      }

      navigate(`/customers/${customer.id}`);
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Failed to create customer"
      );
    } finally {
      setLoading(false);
    }
  }

  return (
    <Layout>
      <PageHeader
        eyebrow="Customers"
        title="New customer"
        description="Create a customer record before placing an order."
      />

      <div className="card form-card">
        <form onSubmit={submit} className="stack">
          <div className="form-grid">
            <label>
              Name
              <input
                value={name}
                onChange={(event) => setName(event.target.value)}
                required
              />
            </label>

            <label>
              Phone
              <input
                value={phone}
                onChange={(event) => setPhone(event.target.value)}
                required
              />
            </label>

            <label>
              Secondary phone
              <input
                value={phone2}
                onChange={(event) => setPhone2(event.target.value)}
              />
            </label>

            <label className="wide">
              Address
              <textarea
                value={address}
                onChange={(event) =>
                  setAddress(event.target.value)
                }
                rows={3}
              />
            </label>
          </div>

          {error && <div className="alert error">{error}</div>}

          <div className="form-actions">
            <Link className="button ghost" to="/customers">
              Cancel
            </Link>

            <button
              className="button primary"
              type="submit"
              disabled={loading}
            >
              {loading ? "Creating..." : "Create customer"}
            </button>
          </div>
        </form>
      </div>
    </Layout>
  );
}

function CustomerDetail() {
  const { id } = useParams();
  const [customer, setCustomer] = useState<Customer | null>(null);
  const [orders, setOrders] = useState<Order[]>([]);
  const [error, setError] = useState("");

  useEffect(() => {
    async function load() {
      if (!id || id === "undefined") return;

      try {
        const response = await apiFetch(`/customers/${id}`);

        if (!response.ok) {
          throw new Error(await readError(response));
        }

        setCustomer(normalizeCustomer(await response.json()));

        const historyResponse = await apiFetch(`/customers/${id}/history`);
        if (historyResponse.ok) setOrders(await historyResponse.json());
      } catch (err) {
        setError(
          err instanceof Error
            ? err.message
            : "Failed to load customer"
        );
      }
    }

    load();
  }, [id]);

  return (
    <Layout>
      <PageHeader
        eyebrow="Customer"
        title={customer?.name ?? "Customer"}
        description="Customer information."
      />

      {error && <div className="alert error">{error}</div>}

      {!customer && !error ? (
        <div className="card">Loading...</div>
      ) : customer ? (
        <div className="detail-grid">
          <div className="card">
            <span className="eyebrow">Contact</span>
            <div className="detail-list">
              <div>
                <span>Phone</span>
                <strong>{customer.phone}</strong>
              </div>
              <div>
                <span>Phone 2</span>
                <strong>{customer.phone2 || "—"}</strong>
              </div>
              <div>
                <span>Address</span>
                <strong>{customer.address || "—"}</strong>
              </div>
            </div>
          </div>

          <div className="card table-card">
            <span className="eyebrow">History</span>
            <h2>Orders for this customer</h2>
            {orders.length === 0 ? <p className="muted">No orders found.</p> : <div className="compact-list">{orders.map((order) => <Link className="compact-row" to={`/orders/${order.id}`} key={order.id}><strong>{order.id.slice(0, 8)}</strong><StatusBadge status={order.status} /><span>{formatDate(order.created_at)}</span></Link>)}</div>}
          </div>

          <div className="card">
            <span className="eyebrow">Actions</span>

            <div className="action-stack">
              <Link
                to={`/orders/new?customer=${customer.id}`}
                className="button primary"
              >
                Create order for customer
              </Link>

              <Link
                to="/customers"
                className="button ghost"
              >
                Back to customers
              </Link>
            </div>
          </div>
        </div>
      ) : null}
    </Layout>
  );
}

function Products() {
  const role = getRole();

  const [products, setProducts] = useState<Product[]>([]);
  const [search, setSearch] = useState("");
  const [showSuggestions, setShowSuggestions] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const searchRef = useRef<HTMLDivElement>(null);

  const suggestions = search.trim().length >= 2
    ? products
        .filter((product) => product.name.toLowerCase().includes(search.trim().toLowerCase()))
        .slice(0, 8)
    : [];

  useEffect(() => {
    function closeSuggestions(event: MouseEvent) {
      if (searchRef.current && !searchRef.current.contains(event.target as Node)) {
        setShowSuggestions(false);
      }
    }

    document.addEventListener("mousedown", closeSuggestions);
    return () => document.removeEventListener("mousedown", closeSuggestions);
  }, []);

  async function load(value = search) {
    setLoading(true);
    setError("");

    try {
      const response = await apiFetch(
        `/products?search=${encodeURIComponent(value)}`
      );

      if (!response.ok) {
        throw new Error(await readError(response));
      }

      const data = await response.json();
      setProducts(data.map(normalizeProduct));
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Failed to load products"
      );
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load("");
  }, []);

  return (
    <Layout>
      <PageHeader
        eyebrow="Inventory"
        title="Products"
        description="Manage your available and warehouse inventory."
        action={
          role === "admin" || role === "superadmin" ? (
            <Link to="/products/new" className="button primary">
              Create Product
            </Link>
          ) : undefined
        }
      />

      <div className="card">
        <form
          className="search-row"
          onSubmit={(event) => {
            event.preventDefault();
            load();
          }}
        >
          <div className="product-search" ref={searchRef}>
            <input
              value={search}
              onFocus={() => setShowSuggestions(true)}
              onChange={(event) => {
                setSearch(event.target.value);
                setShowSuggestions(true);
              }}
              placeholder="Search products..."
            />
            {showSuggestions && suggestions.length > 0 && (
              <div className="product-suggestions">
                {suggestions.map((product) => (
                  <button
                    className="product-suggestion"
                    key={product.id}
                    type="button"
                    onClick={() => {
                      setSearch(product.name);
                      setShowSuggestions(false);
                      load(product.name);
                    }}
                  >
                    <strong>{product.name}</strong>
                    <span>Rs. {money(product.price)}</span>
                  </button>
                ))}
              </div>
            )}
          </div>
          <button className="button secondary" type="submit">
            Search
          </button>
        </form>
      </div>

      {error && <div className="alert error">{error}</div>}

      <div className="card table-card">
        {loading ? (
          <p className="muted">Loading products...</p>
        ) : products.length === 0 ? (
          <EmptyState message="No products found." />
        ) : (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Product</th>
                  <th>Price</th>
                  <th>Available</th>
                  <th>Warehouse</th>
                  {role !== "staff" && <th />}
                </tr>
              </thead>

              <tbody>
                {products.map((product) => (
                  <tr key={product.id}>
                    <td>
                      <strong>{product.name}</strong>
                    </td>
                    <td>Rs. {money(product.price)}</td>
                    <td>
                      <span
                        className={
                          product.available_qty <= 5
                            ? "stock low"
                            : "stock"
                        }
                      >
                        {product.available_qty}
                      </span>
                    </td>
                    <td>{product.warehouse_qty}</td>
                    {role !== "staff" && (
                      <td>
                        <Link
                          className="text-link"
                          to={`/products/${product.id}/edit`}
                        >
                          Edit
                        </Link>
                      </td>
                    )}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </Layout>
  );
}

function ProductForm() {
  const navigate = useNavigate();
  const { id } = useParams();
  const editing = Boolean(id);

  const [name, setName] = useState("");
  const [price, setPrice] = useState("");
  const [availableQty, setAvailableQty] = useState("0");
  const [warehouseQty, setWarehouseQty] = useState("0");
  const [loading, setLoading] = useState(editing);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    async function load() {
      if (!id) {
        setLoading(false);
        return;
      }

      try {
        const response = await apiFetch(`/products/${id}`);

        if (!response.ok) {
          throw new Error(await readError(response));
        }

        const product: Product = await response.json();

        setName(product.name);
        setPrice(String(product.price));
        setAvailableQty(String(product.available_qty));
        setWarehouseQty(String(product.warehouse_qty));
      } catch (err) {
        setError(
          err instanceof Error
            ? err.message
            : "Failed to load product"
        );
      } finally {
        setLoading(false);
      }
    }

    load();
  }, [id]);

  async function submit(event: React.FormEvent) {
    event.preventDefault();

    setSaving(true);
    setError("");

    try {
      const body = JSON.stringify({
        name,
        price,
        available_qty: Number(availableQty),
        warehouse_qty: Number(warehouseQty),
      });

      const response = await apiFetch(
        editing ? `/products/${id}` : "/products",
        {
          method: editing ? "PUT" : "POST",
          body,
        }
      );

      if (!response.ok) {
        throw new Error(await readError(response));
      }

      navigate("/products");
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Failed to save product"
      );
    } finally {
      setSaving(false);
    }
  }

  if (loading) {
    return (
      <Layout>
        <div className="card">Loading product...</div>
      </Layout>
    );
  }

  return (
    <Layout>
      <PageHeader
        eyebrow="Inventory"
        title={editing ? "Edit product" : "New product"}
        description={
          editing
            ? "Update product inventory and pricing."
            : "Add a product to the inventory."
        }
      />

      <div className="card form-card">
        <form onSubmit={submit} className="stack">
          <div className="form-grid">
            <label className="wide">
              Product name
              <input
                value={name}
                onChange={(event) => setName(event.target.value)}
                required
              />
            </label>

            <label>
              Price
              <input
                type="number"
                min="0"
                step="0.01"
                value={price}
                onChange={(event) =>
                  setPrice(event.target.value)
                }
                required
              />
            </label>

            <label>
              Available quantity
              <input
                type="number"
                min="0"
                value={availableQty}
                onChange={(event) =>
                  setAvailableQty(event.target.value)
                }
                required
              />
            </label>

            <label>
              Warehouse quantity
              <input
                type="number"
                min="0"
                value={warehouseQty}
                onChange={(event) =>
                  setWarehouseQty(event.target.value)
                }
                required
              />
            </label>
          </div>

          {error && <div className="alert error">{error}</div>}

          <div className="form-actions">
            <Link to="/products" className="button ghost">
              Cancel
            </Link>

            <button
              type="submit"
              className="button primary"
              disabled={saving}
            >
              {saving
                ? "Saving..."
                : editing
                  ? "Save changes"
                  : "Create product"}
            </button>
          </div>
        </form>
      </div>
    </Layout>
  );
}

function LocationAutocomplete({
  locations,
  selectedLocationID,
  onSelectLocation,
  disabled,
}: {
  locations: CourierLocation[];
  selectedLocationID: string;
  onSelectLocation: (id: string) => void;
  disabled?: boolean;
}) {
  const [search, setSearch] = useState("");
  const selected = useMemo(
    () => locations.find((l) => l.id === selectedLocationID),
    [locations, selectedLocationID]
  );

  const filtered = useMemo(() => {
    if (!search.trim()) return locations.slice(0, 10);
    return filterCourierLocations(locations, search).slice(0, 20);
  }, [locations, search]);

  if (disabled) {
    return (
      <div className="search-row">
        <input
          disabled
          placeholder="Select a courier first..."
        />
      </div>
    );
  }

  if (selected) {
    return (
      <div className="selected-customer">
        <div>
          <strong>{selected.location_name}</strong>
          {selected.delivery_charge ? (
            <span>Delivery charge: Rs. {money(selected.delivery_charge)}</span>
          ) : null}
        </div>
        <button
          type="button"
          className="button ghost"
          onClick={() => {
            onSelectLocation("");
            setSearch("");
          }}
        >
          Change
        </button>
      </div>
    );
  }

  return (
    <div>
      <div className="search-row">
        <input
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Search location (e.g. Kathmandu, Pokhara)..."
        />
        {search && (
          <button
            type="button"
            className="button ghost"
            onClick={() => setSearch("")}
          >
            Clear
          </button>
        )}
      </div>

      {locations.length > 0 && (
        <div
          className="search-results"
          style={{ maxHeight: "220px", overflowY: "auto" }}
        >
          {filtered.length === 0 ? (
            <p className="muted small" style={{ padding: "8px 0" }}>
              No matching locations found for "{search}".
            </p>
          ) : (
            filtered.map((loc) => (
              <button
                type="button"
                key={loc.id}
                className="search-result"
                onClick={() => {
                  onSelectLocation(loc.id);
                  setSearch("");
                }}
              >
                <strong>{loc.location_name}</strong>
                {loc.delivery_charge ? (
                  <span>Delivery charge: Rs. {money(loc.delivery_charge)}</span>
                ) : null}
              </button>
            ))
          )}
        </div>
      )}
    </div>
  );
}

function CreateOrder() {
  const navigate = useNavigate();
  const role = getRole();

  const [customerSearch, setCustomerSearch] = useState("");
  const [customer, setCustomer] = useState<Customer | null>(null);
  const [customerResults, setCustomerResults] = useState<Customer[]>([]);

  const [products, setProducts] = useState<Product[]>([]);
  const [productSearch, setProductSearch] = useState("");
  const [cart, setCart] = useState<CartItem[]>([]);

  const [source, setSource] = useState("phone");
  const [address, setAddress] = useState("");
  const [codAmount, setCodAmount] = useState("");
  const [isStoreVisit, setIsStoreVisit] = useState(false);

  const [couriers, setCouriers] = useState<Courier[]>([]);
  const [selectedCourierID, setSelectedCourierID] = useState("");
  const [courierLocations, setCourierLocations] = useState<CourierLocation[]>([]);
  const [selectedLocationID, setSelectedLocationID] = useState("");

  const [loadingProducts, setLoadingProducts] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");

  useEffect(() => {
    async function loadCouriers() {
      try {
        const response = await apiFetch("/couriers");
        if (response.ok) {
          const data = await response.json();
          setCouriers(Array.isArray(data) ? data.map(normalizeCourier) : []);
        }
      } catch {
        // Couriers are optional.
      }
    }
    loadCouriers();
  }, []);

  useEffect(() => {
    if (!selectedCourierID) {
      setCourierLocations([]);
      setSelectedLocationID("");
      return;
    }
    async function loadLocations() {
      try {
        const response = await apiFetch(`/couriers/${selectedCourierID}/locations`);
        if (response.ok) {
          const data = await response.json();
          setCourierLocations(Array.isArray(data) ? data.map(normalizeCourierLocation) : []);
        }
      } catch {
        setCourierLocations([]);
      }
    }
    loadLocations();
    setSelectedLocationID("");
  }, [selectedCourierID]);

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const customerId = params.get("customer");

    if (!customerId) return;

    async function loadCustomer() {
      try {
        const response = await apiFetch(`/customers/${customerId}`);

        if (response.ok) {
          const customer = normalizeCustomer(await response.json());
          setCustomer(customer);
          setAddress(customer.address || "");
        }
      } catch {
        // Customer can still be selected manually.
      }
    }

    loadCustomer();
  }, []);

  async function searchCustomers() {
    if (!customerSearch.trim()) {
      setCustomerResults([]);
      return;
    }

    try {
      const response = await apiFetch(
        `/customers?search=${encodeURIComponent(
          customerSearch.trim()
        )}`
      );

      if (!response.ok) {
        throw new Error(await readError(response));
      }

      const data = await response.json();
      setCustomerResults(data.map(normalizeCustomer));
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Failed to search customers"
      );
    }
  }

  async function loadProducts(value = "") {
    setLoadingProducts(true);

    try {
      const response = await apiFetch(
        `/products?search=${encodeURIComponent(value)}`
      );

      if (!response.ok) {
        throw new Error(await readError(response));
      }

      const data = await response.json();
      setProducts(data.map(normalizeProduct));
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Failed to load products"
      );
    } finally {
      setLoadingProducts(false);
    }
  }

  useEffect(() => {
    loadProducts("");
  }, []);

  function addProduct(product: Product) {
    setError("");

    if (product.available_qty <= 0) {
      setError(`${product.name} is out of stock.`);
      return;
    }

    setCart((current) => {
      const existing = current.find(
        (item) => item.product.id === product.id
      );

      if (existing) {
        if (existing.quantity >= product.available_qty) {
          return current;
        }

        return current.map((item) =>
          item.product.id === product.id
            ? {
                ...item,
                quantity: item.quantity + 1,
              }
            : item
        );
      }

      return [
        ...current,
        {
          product,
          quantity: 1,
        },
      ];
    });
  }

  function changeQuantity(productId: string, quantity: number) {
    if (!Number.isFinite(quantity) || quantity < 1) {
      return;
    }

    setCart((current) =>
      current
        .map((item) => {
          if (item.product.id !== productId) {
            return item;
          }

          const next = Math.max(
            0,
            Math.min(quantity, item.product.available_qty)
          );

          return {
            ...item,
            quantity: next,
          };
        })
    );
  }

  const total = useMemo(() => calculateCartTotal(cart), [cart]);

  async function submit(event: React.FormEvent) {
    event.preventDefault();

    setError("");
    setMessage("");

    if (!customer) {
      setError("Select a customer first.");
      return;
    }

    if (cart.length === 0) {
      setError("Add at least one product.");
      return;
    }

    if (!address.trim()) {
      setError("Address is required.");
      return;
    }

    setSaving(true);

    try {
      const body: Record<string, unknown> = {
        customer_id: customer.id,
        source,
        address,
        is_store_visit: isStoreVisit,
        items: cart.map((item) => ({
          product_id: item.product.id,
          quantity: item.quantity,
        })),
      };

      if (role === "admin" || role === "superadmin") {
        body.cod_amount = codAmount;
      }

      if (selectedCourierID) {
        body.courier_id = selectedCourierID;
      }
      if (selectedLocationID) {
        body.location_id = selectedLocationID;
      }

      const response = await apiFetch("/orders", {
        method: "POST",
        body: JSON.stringify(body),
      });

      if (!response.ok) {
        throw new Error(await readError(response));
      }

      const order: Order = await response.json();

      setMessage(
        `Order ${order.id.slice(0, 8)} created successfully.`
      );

      setCart([]);
      setCustomer(null);
      setCustomerSearch("");
      setCustomerResults([]);
      setAddress("");
      setCodAmount("");
      setIsStoreVisit(false);
      setSelectedCourierID("");
      setSelectedLocationID("");

      setTimeout(() => {
        navigate(`/orders/${order.id}`);
      }, 700);
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Failed to create order"
      );
    } finally {
      setSaving(false);
    }
  }

  return (
    <Layout>
      <PageHeader
        eyebrow="Orders"
        title="Create order"
        description="Build a new order from an existing customer and available products."
      />

      <form onSubmit={submit}>
        <div className="order-layout">
          <div className="order-main">
            <section className="card">
              <div className="section-heading">
                <div>
                  <span className="eyebrow">01</span>
                  <h2>Customer</h2>
                </div>
              </div>

              {customer ? (
                <div className="selected-customer">
                  <div>
                    <strong>{customer.name}</strong>
                    <span>
                      {customer.phone} ·{" "}
                      {customer.address || "No address"}
                    </span>
                  </div>

                  <button
                    type="button"
                    className="button ghost"
                    onClick={() => setCustomer(null)}
                  >
                    Change
                  </button>
                </div>
              ) : (
                <>
                  <div className="search-row">
                    <input
                      value={customerSearch}
                      onChange={(event) =>
                        setCustomerSearch(event.target.value)
                      }
                      onKeyDown={(event) => {
                        if (event.key === "Enter") {
                          event.preventDefault();
                          void searchCustomers();
                        }
                      }}
                      placeholder="Search customer by name or phone..."
                    />

                    <button
                      type="button"
                      className="button secondary"
                      onClick={searchCustomers}
                    >
                      Find
                    </button>
                  </div>

                  {customerResults.length > 0 && (
                    <div className="search-results">
                      {customerResults.map((item) => (
                        <button
                          type="button"
                          key={item.id}
                          className="search-result"
                          onClick={() => {
                            setCustomer(item);
                            setAddress(item.address || "");
                            setCustomerResults([]);
                          }}
                        >
                          <strong>{item.name}</strong>
                          <span>{item.phone}</span>
                        </button>
                      ))}
                    </div>
                  )}

                  <Link
                    className="text-link create-inline"
                    to="/customers/new"
                  >
                    + Create new customer
                  </Link>
                </>
              )}
            </section>

            <section className="card">
              <div className="section-heading">
                <div>
                  <span className="eyebrow">02</span>
                  <h2>Products</h2>
                </div>
              </div>

              <div className="search-row">
                <input
                  value={productSearch}
                  onChange={(event) =>
                    setProductSearch(event.target.value)
                  }
                  placeholder="Search products..."
                />

                <button
                  type="button"
                  className="button secondary"
                  onClick={() => loadProducts(productSearch)}
                >
                  Search
                </button>
              </div>

              <div className="product-picker">
                {loadingProducts ? (
                  <p className="muted">Loading products...</p>
                ) : products.length === 0 ? (
                  <p className="muted">No products found.</p>
                ) : (
                  products.map((product) => (
                    <button
                      type="button"
                      key={product.id}
                      className="product-option"
                      disabled={product.available_qty <= 0}
                      onClick={() => addProduct(product)}
                    >
                      <span>
                        <strong>{product.name}</strong>
                        <small>
                          Rs. {money(product.price)}
                        </small>
                      </span>

                      <span className="stock">
                        {product.available_qty} left
                      </span>
                    </button>
                  ))
                )}
              </div>

              {cart.length > 0 && (
                <div className="cart">
                  <h3>Order items</h3>

                  {cart.map((item) => (
                    <div className="cart-row" key={item.product.id}>
                      <div>
                        <strong>{item.product.name}</strong>
                        <span>
                          Rs. {money(item.product.price)} each
                        </span>
                      </div>

                      <input
                        className="quantity-input"
                        type="number"
                        min="1"
                        max={item.product.available_qty}
                        value={item.quantity}
                        onChange={(event) =>
                          changeQuantity(
                            item.product.id,
                            Number(event.target.value)
                          )
                        }
                      />

                      <strong>
                        Rs.{" "}
                        {(
                          Number(item.product.price) *
                          item.quantity
                        ).toFixed(2)}
                      </strong>
                    </div>
                  ))}
                </div>
              )}
            </section>

            <section className="card">
              <div className="section-heading">
                <div>
                  <span className="eyebrow">03</span>
                  <h2>Delivery details</h2>
                </div>
              </div>

              <div className="form-grid">
                <label>
                  Source
                  <select
                    value={source}
                    onChange={(event) =>
                      setSource(event.target.value)
                    }
                  >
                    <option value="website">Website</option>
                    <option value="daraz">Daraz</option>
                    <option value="phone">Phone</option>
                    <option value="facebook">Facebook</option>
                    <option value="instagram">Instagram</option>
                    <option value="store">Store</option>
                  </select>
                </label>

                <label>
                  Courier
                  <select
                    value={selectedCourierID}
                    onChange={(event) =>
                      setSelectedCourierID(event.target.value)
                    }
                  >
                    <option value="">Select courier (optional)</option>
                    {couriers.map((c) => (
                      <option key={c.id} value={c.id}>
                        {c.name}
                      </option>
                    ))}
                  </select>
                </label>

                <div className="wide">
                  <label style={{ display: "block", marginBottom: "6px" }}>
                    Courier location
                  </label>
                  <LocationAutocomplete
                    locations={courierLocations}
                    selectedLocationID={selectedLocationID}
                    onSelectLocation={setSelectedLocationID}
                    disabled={!selectedCourierID || courierLocations.length === 0}
                  />
                </div>

                {(role === "admin" ||
                  role === "superadmin") && (
                  <label>
                    COD amount
                    <input
                      type="number"
                      min="0"
                      step="0.01"
                      value={codAmount}
                      onChange={(event) =>
                        setCodAmount(event.target.value)
                      }
                      required
                    />
                  </label>
                )}

                <label className="wide">
                  Delivery address
                  <textarea
                    value={address}
                    onChange={(event) =>
                      setAddress(event.target.value)
                    }
                    rows={4}
                    required
                  />
                </label>

                <label className="checkbox-label wide">
                  <input
                    type="checkbox"
                    checked={isStoreVisit}
                    onChange={(event) =>
                      setIsStoreVisit(event.target.checked)
                    }
                  />
                  Customer will visit the store
                </label>
              </div>
            </section>
          </div>

          <aside className="order-summary card">
            <span className="eyebrow">Order summary</span>
            <h2>Review</h2>

            <div className="summary-row">
              <span>Customer</span>
              <strong>
                {customer?.name || "Not selected"}
              </strong>
            </div>

            <div className="summary-row">
              <span>Items</span>
              <strong>
                {cart.reduce(
                  (sum, item) => sum + item.quantity,
                  0
                )}
              </strong>
            </div>

            <div className="summary-total">
              <span>Products total</span>
              <strong>Rs. {total.toFixed(2)}</strong>
            </div>

            {error && (
              <div className="alert error">{error}</div>
            )}

            {message && (
              <div className="alert success">{message}</div>
            )}

            <button
              type="submit"
              className="button primary full"
              disabled={saving}
            >
              {saving ? "Creating order..." : "Create order"}
            </button>
          </aside>
        </div>
      </form>
    </Layout>
  );
}

function Orders() {
  const role = getRole();
  const [orders, setOrders] = useState<Order[]>([]);
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("");
  const [source, setSource] = useState("");
  const [courierID, setCourierID] = useState("");
  const [customerID, setCustomerID] = useState("");
  const [fromDate, setFromDate] = useState("");
  const [toDate, setToDate] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  async function exportOrders() {
    const params = new URLSearchParams({ status, search, source, courier_id: courierID, customer_id: customerID, from_date: fromDate, to_date: toDate });
    const response = await apiFetch(`/reports/orders.csv?${params.toString()}`);
    if (!response.ok) { setError(await readError(response)); return; }
    const blob = await response.blob();
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = "orders.csv";
    link.click();
    URL.revokeObjectURL(url);
  }

  useEffect(() => {
    async function load() {
      try {
        const params = new URLSearchParams();
        if (search) params.set("search", search);
        if (status) params.set("status", status);
        if (source) params.set("source", source);
        if (courierID) params.set("courier_id", courierID);
        if (customerID) params.set("customer_id", customerID);
        if (fromDate) params.set("from_date", fromDate);
        if (toDate) params.set("to_date", toDate);
        const response = await apiFetch(`/orders?${params.toString()}`);

        if (!response.ok) {
          throw new Error(await readError(response));
        }

        const data = await response.json();
        setOrders(Array.isArray(data) ? data.map(normalizeOrder) : []);
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

    load();
  }, [search, status, source, courierID, customerID, fromDate, toDate]);

  return (
    <Layout>
      <PageHeader
        eyebrow="Orders"
        title="Order management"
        description={`${orders.length} total orders`}
        action={
          <span>
            {(role === "admin" || role === "superadmin") && <button className="button ghost" type="button" onClick={exportOrders}>Export CSV</button>}
            <Link to="/orders/new" className="button primary">Create order</Link>
          </span>
        }
      />

      <div className="card">
        <div className="filters">
          <input
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder="Search orders..."
          />

          <select
            value={status}
            onChange={(event) => setStatus(event.target.value)}
          >
            <option value="">All statuses</option>
            <option value="confirmed">Confirmed</option>
            <option value="pickup_complete">
              Pickup complete
            </option>
            <option value="dispatched">Dispatched</option>
            <option value="arrived">Arrived</option>
            <option value="delivered">Delivered</option>
            <option value="follow_up">Follow up</option>
            <option value="hold">Hold</option>
            <option value="redirected">Redirected</option>
            <option value="cancelled">Cancelled</option>
            <option value="returned">Returned</option>
          </select>
          <select value={source} onChange={(event) => setSource(event.target.value)}><option value="">All sources</option><option value="website">Website</option><option value="daraz">Daraz</option><option value="phone">Phone</option><option value="facebook">Facebook</option><option value="instagram">Instagram</option><option value="store">Store</option></select>
          <input value={courierID} onChange={(event) => setCourierID(event.target.value)} placeholder="Courier ID" />
          <input value={customerID} onChange={(event) => setCustomerID(event.target.value)} placeholder="Customer ID" />
          <label>From <input type="date" value={fromDate} onChange={(event) => setFromDate(event.target.value)} /></label>
          <label>To <input type="date" value={toDate} onChange={(event) => setToDate(event.target.value)} /></label>
        </div>
      </div>

      {error && <div className="alert error">{error}</div>}

      <div className="card table-card">
        {loading ? (
          <p className="muted">Loading orders...</p>
        ) : orders.length === 0 ? (
          <EmptyState message="No orders found." />
        ) : (
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Order</th>
                  <th>Source</th>
                  <th>Status</th>
                  <th>COD</th>
                  <th>Created</th>
                  <th />
                </tr>
              </thead>

              <tbody>
                {orders.map((order) => (
                  <tr key={order.id}>
                    <td>
                      <strong>{order.id.slice(0, 8)}</strong>
                      {order.is_legacy && <LegacyBadge />}
                    </td>
                    <td>{order.source}</td>
                    <td>
                      <StatusBadge status={order.status} />
                    </td>
                    <td>Rs. {money(order.cod_amount)}</td>
                    <td>{formatDate(order.created_at)}</td>
                    <td>
                      <Link
                        className="text-link"
                        to={`/orders/${order.id}`}
                      >
                        View
                      </Link>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </Layout>
  );
}

function OrderDetail() {
  const { id } = useParams();
  const role = getRole();

  const [order, setOrder] = useState<Order | null>(null);
  const [items, setItems] = useState<OrderItem[]>([]);
  const [products, setProducts] = useState<Product[]>([]);
  const [customer, setCustomer] = useState<Customer | null>(null);
  const [couriers, setCouriers] = useState<Courier[]>([]);
  const [courierLocations, setCourierLocations] = useState<CourierLocation[]>([]);
  const [selectedCourierID, setSelectedCourierID] = useState("");
  const [selectedLocationID, setSelectedLocationID] = useState("");
  const [savingCourier, setSavingCourier] = useState(false);
  const [newStatus, setNewStatus] = useState("");
  const [followUpAction, setFollowUpAction] = useState("no_answer");
  const [followUpDate, setFollowUpDate] = useState("");
  const [followUpNote, setFollowUpNote] = useState("");
  const [followUpSaving, setFollowUpSaving] = useState(false);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");

  useEffect(() => {
    async function loadCouriers() {
      try {
        const response = await apiFetch("/couriers");
        if (response.ok) {
          const data = await response.json();
          setCouriers(Array.isArray(data) ? data.map(normalizeCourier) : []);
        }
      } catch {
        // Couriers optional
      }
    }
    loadCouriers();
  }, []);

  useEffect(() => {
    if (!selectedCourierID) {
      setCourierLocations([]);
      return;
    }
    async function loadLocations() {
      try {
        const response = await apiFetch(`/couriers/${selectedCourierID}/locations`);
        if (response.ok) {
          const data = await response.json();
          setCourierLocations(Array.isArray(data) ? data.map(normalizeCourierLocation) : []);
        }
      } catch {
        setCourierLocations([]);
      }
    }
    loadLocations();
  }, [selectedCourierID]);

  async function load() {
    if (!id) return;

    setLoading(true);
    setError("");

    try {
      const orderResponse = await apiFetch(`/orders/${id}`);

      if (!orderResponse.ok) {
        throw new Error(await readError(orderResponse));
      }

      const orderData = normalizeOrder(await orderResponse.json());

      setOrder(orderData);
      setNewStatus(orderData.status);
      setSelectedCourierID(orderData.courier_id || "");
      setSelectedLocationID(orderData.location_id || "");

      const customerResponse = await apiFetch(
        `/customers/${orderData.customer_id}`
      );

      if (customerResponse.ok) {
        setCustomer(normalizeCustomer(await customerResponse.json()));
      }

      const itemsResponse = await apiFetch(
        `/orders/${id}/items`
      );

      if (itemsResponse.ok) {
        const data = await itemsResponse.json();
        setItems(Array.isArray(data) ? data.map(normalizeOrderItem) : []);
      }

      const productsResponse = await apiFetch("/products");

      if (productsResponse.ok) {
        const data = await productsResponse.json();
        setProducts(Array.isArray(data) ? data.map(normalizeProduct) : []);
      }
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Failed to load order"
      );
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
  }, [id]);

  async function updateStatus() {
    if (!id || !order || newStatus === order.status) {
      return;
    }

    setSaving(true);
    setError("");
    setMessage("");

    try {
      const response = await apiFetch(
        `/orders/${id}/status`,
        {
          method: "PATCH",
          body: JSON.stringify({
            status: newStatus,
          }),
        }
      );

      if (!response.ok) {
        throw new Error(await readError(response));
      }

      const updated = normalizeOrder(await response.json());
      setOrder(updated);
      setNewStatus(updated.status);
      setMessage("Order status updated.");
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Failed to update status"
      );
    } finally {
      setSaving(false);
    }
  }

  async function updateCourierLocation() {
    if (!id || !order) return;
    setSavingCourier(true);
    setError("");
    setMessage("");

    try {
      const response = await apiFetch(`/orders/${id}`, {
        method: "PATCH",
        body: JSON.stringify({
          courier_id: selectedCourierID || null,
          location_id: selectedLocationID || null,
        }),
      });

      if (!response.ok) {
        throw new Error(await readError(response));
      }

      const updated = normalizeOrder(await response.json());
      setOrder(updated);
      setSelectedCourierID(updated.courier_id || "");
      setSelectedLocationID(updated.location_id || "");
      setMessage("Courier and location updated.");
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Failed to update courier/location"
      );
    } finally {
      setSavingCourier(false);
    }
  }

  async function clearCourierLocation() {
    if (!id || !order) return;
    setSavingCourier(true);
    setError("");
    setMessage("");

    try {
      const response = await apiFetch(`/orders/${id}`, {
        method: "PATCH",
        body: JSON.stringify({
          courier_id: null,
          location_id: null,
        }),
      });

      if (!response.ok) {
        throw new Error(await readError(response));
      }

      const updated = normalizeOrder(await response.json());
      setOrder(updated);
      setSelectedCourierID("");
      setSelectedLocationID("");
      setMessage("Courier and location unassigned.");
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Failed to clear courier/location"
      );
    } finally {
      setSavingCourier(false);
    }
  }

  async function createFollowUp() {
    if (!id) return;
    setFollowUpSaving(true);
    setError("");
    try {
      const response = await apiFetch(`/orders/${id}/followup`, {
        method: "POST",
        body: JSON.stringify({
          next_action: followUpAction,
          next_action_date: followUpDate,
          note: followUpNote,
        }),
      });
      if (!response.ok) throw new Error(await readError(response));
      setFollowUpNote("");
      setMessage("Follow-up recorded.");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to record follow-up");
    } finally {
      setFollowUpSaving(false);
    }
  }

  const productMap = useMemo(
    () => new Map(products.map((product) => [product.id, product])),
    [products]
  );
  const courierMap = useMemo(
    () => new Map(couriers.map((c) => [c.id, c])),
    [couriers]
  );
  const courierLocationMap = useMemo(
    () => new Map(courierLocations.map((l) => [l.id, l])),
    [courierLocations]
  );
  const validNextStatuses = useMemo(
    () => (order ? getValidNextStatuses(order.status) : []),
    [order?.status]
  );
  const isTerminal = validNextStatuses.length === 0;

  const STATUS_LABELS: Record<string, string> = {
    confirmed: "Confirmed",
    pickup_complete: "Pickup complete",
    dispatched: "Dispatched",
    arrived: "Arrived",
    delivered: "Delivered",
    follow_up: "Follow up",
    hold: "Hold",
    redirected: "Redirected",
    cancelled: "Cancelled",
    returned: "Returned",
  };

  return (
    <Layout>
      <PageHeader
        eyebrow="Order"
        title={order ? order.id.slice(0, 8) : "Order"}
        description={
          order
            ? `Created ${formatDate(order.created_at)}`
            : undefined
        }
      />

      {error && <div className="alert error">{error}</div>}
      {message && <div className="alert success">{message}</div>}

      {loading ? (
        <div className="card">Loading order...</div>
      ) : order ? (
        <div className="detail-grid">
          <div className="stack">
            <section className="card">
              <div className="section-heading">
                <div>
                  <span className="eyebrow">Order</span>
                  <h2>Details</h2>
                  {order.is_legacy && <LegacyBadge />}
                </div>

                <StatusBadge status={order.status} />
              </div>

              <div className="detail-list">
                <div>
                  <span>Source</span>
                  <strong>{order.source}</strong>
                </div>

                <div>
                  <span>COD</span>
                  <strong>
                    Rs. {money(order.cod_amount)}
                  </strong>
                </div>

                <div>
                  <span>Store visit</span>
                  <strong>
                    {order.is_store_visit ? "Yes" : "No"}
                  </strong>
                </div>

                <div>
                  <span>Address</span>
                  <strong>{order.address}</strong>
                </div>

                <div>
                  <span>Courier</span>
                  <strong>
                    {courierMap.get(order.courier_id || "")?.name || (order.courier_id ? order.courier_id.slice(0, 8) : "Not assigned")}
                  </strong>
                </div>

                <div>
                  <span>Location</span>
                  <strong>
                    {courierLocationMap.get(order.location_id || "")?.location_name || (order.location_id ? order.location_id.slice(0, 8) : "—")}
                  </strong>
                </div>
              </div>
            </section>

            <section className="card">
              <div className="section-heading">
                <div>
                  <span className="eyebrow">Logistics</span>
                  <h2>Courier & Destination</h2>
                </div>
              </div>

              <div className="form-grid">
                <label>
                  Courier
                  <select
                    value={selectedCourierID}
                    onChange={(event) => {
                      setSelectedCourierID(event.target.value);
                      setSelectedLocationID("");
                    }}
                  >
                    <option value="">No courier assigned</option>
                    {couriers.map((c) => (
                      <option key={c.id} value={c.id}>
                        {c.name}
                      </option>
                    ))}
                  </select>
                </label>

                <div className="wide">
                  <label style={{ display: "block", marginBottom: "6px" }}>
                    Courier location
                  </label>
                  <LocationAutocomplete
                    locations={courierLocations}
                    selectedLocationID={selectedLocationID}
                    onSelectLocation={setSelectedLocationID}
                    disabled={!selectedCourierID || courierLocations.length === 0}
                  />
                </div>
              </div>

              <div style={{ marginTop: "1rem", display: "flex", gap: "8px", flexWrap: "wrap" }}>
                <button
                  type="button"
                  className="button secondary"
                  disabled={
                    savingCourier ||
                    (selectedCourierID === (order.courier_id || "") &&
                      selectedLocationID === (order.location_id || ""))
                  }
                  onClick={updateCourierLocation}
                >
                  {savingCourier ? "Saving..." : "Save courier & location"}
                </button>

                {(order.courier_id || order.location_id || selectedCourierID || selectedLocationID) && (
                  <button
                    type="button"
                    className="button ghost"
                    disabled={savingCourier}
                    onClick={clearCourierLocation}
                  >
                    Clear / Unassign
                  </button>
                )}
              </div>
            </section>

            <section className="card">
              <div className="section-heading">
                <div>
                  <span className="eyebrow">Customer</span>
                  <h2>
                    {customer?.name || order.customer_id.slice(0, 8)}
                  </h2>
                </div>
              </div>

              {customer && (
                <div className="detail-list">
                  <div>
                    <span>Phone</span>
                    <strong>{customer.phone}</strong>
                  </div>
                  <div>
                    <span>Address</span>
                    <strong>
                      {customer.address || "—"}
                    </strong>
                  </div>
                </div>
              )}
            </section>

            <section className="card">
              <div className="section-heading">
                <div>
                  <span className="eyebrow">Items</span>
                  <h2>Products</h2>
                </div>
              </div>

              {items.length === 0 ? (
                <p className="muted">
                  No order items returned by the API.
                </p>
              ) : (
                <div className="compact-list">
                  {items.map((item) => {
                    const product = productMap.get(item.product_id);

                    return (
                      <div className="compact-row" key={item.id}>
                        <div>
                          <strong>
                            {product?.name ||
                              item.product_id.slice(0, 8)}
                          </strong>
                          <span>
                            {item.quantity} × Rs.{" "}
                            {money(item.price)}
                          </span>
                        </div>

                        <strong>
                          Rs.{" "}
                          {(
                            Number(item.price) *
                            item.quantity
                          ).toFixed(2)}
                        </strong>
                      </div>
                    );
                  })}
                </div>
              )}
            </section>
          </div>

          <aside className="card">
            <span className="eyebrow">Workflow</span>
            <h2>Update status</h2>

            <div className="stack">
              <label>
                Status
                <select
                  value={newStatus}
                  onChange={(event) =>
                    setNewStatus(event.target.value)
                  }
                  disabled={isTerminal || saving}
                >
                  <option value={order.status}>
                    {STATUS_LABELS[order.status] || order.status} (current)
                  </option>
                  {validNextStatuses.map((st) => (
                    <option key={st} value={st}>
                      {STATUS_LABELS[st] || st}
                    </option>
                  ))}
                </select>
              </label>

              <button
                className="button primary full"
                type="button"
                disabled={
                  saving || isTerminal || newStatus === order.status
                }
                onClick={updateStatus}
              >
                {saving ? "Updating..." : "Update status"}
              </button>

              {isTerminal ? (
                <p className="muted small">
                  This order is in a terminal state ({STATUS_LABELS[order.status] || order.status}). No further status transitions are allowed.
                </p>
              ) : (
                <p className="muted small">
                  Only valid next status transitions are shown.
                </p>
              )}

              {role === "staff" && !isTerminal && (
                <p className="muted small">
                  Staff status changes are subject to the same
                  backend workflow rules.
                </p>
              )}

              <hr />
              <h2>Record follow-up</h2>
              <label>
                Action
                <select value={followUpAction} onChange={(event) => setFollowUpAction(event.target.value)}>
                  <option value="no_answer">No answer</option>
                  <option value="call_again">Call again</option>
                </select>
              </label>
              <label>
                Call again date
                <input type="date" value={followUpDate} onChange={(event) => setFollowUpDate(event.target.value)} />
              </label>
              <label>
                Note
                <textarea value={followUpNote} onChange={(event) => setFollowUpNote(event.target.value)} maxLength={500} />
              </label>
              <button className="button full" type="button" disabled={followUpSaving} onClick={createFollowUp}>
                {followUpSaving ? "Saving..." : "Record follow-up"}
              </button>
            </div>
          </aside>
        </div>
      ) : null}
    </Layout>
  );
}

function FollowUps() {
  const [items, setItems] = useState<FollowUp[]>([]);
  const [dueToday, setDueToday] = useState(true);
  const [unanswered, setUnanswered] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    async function load() {
      try {
        const params = new URLSearchParams({
          due_today: String(dueToday),
          unanswered: String(unanswered),
        });
        const response = await apiFetch(`/followups?${params.toString()}`);
        if (!response.ok) throw new Error(await readError(response));
        setItems(await response.json());
      } catch (err) {
        setError(err instanceof Error ? err.message : "Failed to load follow-ups");
      }
    }
    load();
  }, [dueToday, unanswered]);

  return (
    <Layout>
      <PageHeader eyebrow="Follow-ups" title="Follow-up queue" description="Due today and unanswered customers" />
      <div className="card filters">
        <label><input type="checkbox" checked={dueToday} onChange={(event) => setDueToday(event.target.checked)} /> Due today</label>
        <label><input type="checkbox" checked={unanswered} onChange={(event) => setUnanswered(event.target.checked)} /> Unanswered</label>
      </div>
      {error && <div className="alert error">{error}</div>}
      <div className="card table-card">
        {items.length === 0 ? <EmptyState message="No follow-ups found." /> : (
          <div className="table-wrap">
            <table>
              <thead><tr><th>Customer</th><th>Action</th><th>Due</th><th>Assigned to</th><th>Note</th></tr></thead>
              <tbody>{items.map((item) => <tr key={item.id}><td><strong>{item.customer_name}</strong><br />{item.customer_phone}</td><td>{item.next_action} #{item.attempt_no}</td><td>{item.next_action_date || "-"}</td><td>{item.assigned_to_name || "Unassigned"}</td><td>{item.note || "-"}</td></tr>)}</tbody>
            </table>
          </div>
        )}
      </div>
    </Layout>
  );
}

function Users() {
  const currentRole = getRole();
  const [users, setUsers] = useState<User[]>([]);
  const [name, setName] = useState("");
  const [phone, setPhone] = useState("");
  const [role, setRole] = useState<Role>("staff");
  const [invitationURL, setInvitationURL] = useState("");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");

  async function loadUsers() {
    setLoading(true);
    setError("");
    try {
      const response = await apiFetch("/users");
      if (!response.ok) throw new Error(await readError(response));
      setUsers(await response.json());
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load users");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    loadUsers();
  }, []);

  async function createUser(event: React.FormEvent) {
    event.preventDefault();
    setSaving(true);
    setError("");
    setMessage("");
    try {
      const response = await apiFetch("/users", {
        method: "POST",
        body: JSON.stringify({ name, phone, role }),
      });
      if (!response.ok) throw new Error(await readError(response));
      const data = await response.json();
      setName("");
      setPhone("");
      setInvitationURL(frontendInvitationURL(data.invitation_url));
      setMessage("Invitation created. Share the invitation URL securely.");
      await loadUsers();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create user");
    } finally {
      setSaving(false);
    }
  }

  async function resendInvitation(id: string) {
    setSaving(true);
    setError("");
    setMessage("");
    try {
      const response = await apiFetch(`/users/${id}/resend-invitation`, { method: "POST" });
      if (!response.ok) throw new Error(await readError(response));
      const data = await response.json();
      setInvitationURL(frontendInvitationURL(data.invitation_url));
      setMessage("Invitation renewed. Share the new invitation URL securely.");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to resend invitation");
    } finally {
      setSaving(false);
    }
  }

  async function revokeInvitation(id: string) {
    setSaving(true);
    setError("");
    setMessage("");
    try {
      const response = await apiFetch(`/users/${id}/revoke-invitation`, { method: "POST" });
      if (!response.ok) throw new Error(await readError(response));
      setMessage("Invitation revoked.");
      await loadUsers();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to revoke invitation");
    } finally {
      setSaving(false);
    }
  }

  async function updateUser(id: string, body: Record<string, unknown>, successMessage: string) {
    setSaving(true);
    setError("");
    setMessage("");
    try {
      const response = await apiFetch(`/users/${id}`, {
        method: "PATCH",
        body: JSON.stringify(body),
      });
      if (!response.ok) throw new Error(await readError(response));
      setMessage(successMessage);
      await loadUsers();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to update user");
    } finally {
      setSaving(false);
    }
  }

  return (
    <Layout>
      <PageHeader
        eyebrow="Administration"
        title="Users"
        description="Invite and manage internal Admin and Staff accounts."
      />

      {error && <div className="alert error">{error}</div>}
      {message && <div className="alert success">{message}</div>}

      <div className="detail-grid">
        <section className="card form-card">
          <div className="section-heading">
            <div>
              <span className="eyebrow">New account</span>
              <h2>Create user</h2>
            </div>
          </div>
          <form className="stack" onSubmit={createUser}>
            <div className="form-grid">
              <label>
                Name
                <input value={name} onChange={(event) => setName(event.target.value)} required />
              </label>
              <label>
                Phone
                <input value={phone} onChange={(event) => setPhone(event.target.value)} required />
              </label>
              <label>
                Role
                <select value={role} onChange={(event) => setRole(event.target.value as Role)}>
                  <option value="staff">Staff</option>
                  {currentRole === "superadmin" && <option value="admin">Admin</option>}
                </select>
              </label>
            </div>
            <p className="muted small">The invited user will create their own password from the invitation.</p>
            <button className="button primary" type="submit" disabled={saving}>
              {saving ? "Sending..." : "Send Invitation"}
            </button>
          </form>
          {invitationURL && (
            <div className="alert success invitation-url">
              Invitation URL: <a href={invitationURL}>{invitationURL}</a>
            </div>
          )}
        </section>

        <section className="card table-card">
          <div className="section-heading">
            <div>
              <span className="eyebrow">Accounts</span>
              <h2>Internal users</h2>
            </div>
          </div>
          {loading ? (
            <p className="muted">Loading users...</p>
          ) : users.length === 0 ? (
            <EmptyState message="No users found." />
          ) : (
            <div className="table-wrap">
              <table>
                <thead>
                  <tr><th>Name</th><th>Phone</th><th>Role</th><th>Status</th><th /></tr>
                </thead>
                <tbody>
                  {users.map((user) => (
                    <tr key={user.id}>
                      <td><strong>{user.name}</strong></td>
                      <td>{user.phone}</td>
                      <td>{user.role}</td>
                      <td>{user.status || (user.is_active ? "Active" : "Inactive")}</td>
                      <td>
                        {user.role !== "superadmin" && (
                          <div className="user-actions">
                            {user.status === "invited" ? (
                              <>
                                <button className="button ghost" type="button" disabled={saving} onClick={() => resendInvitation(user.id)}>Resend invitation</button>
                                <button className="button ghost" type="button" disabled={saving} onClick={() => revokeInvitation(user.id)}>Revoke invitation</button>
                              </>
                            ) : (
                              <button className="button ghost" type="button" disabled={saving} onClick={() => updateUser(user.id, { is_active: !user.is_active }, user.is_active ? "User deactivated." : "User activated.")}>
                                {user.is_active ? "Deactivate" : "Activate"}
                              </button>
                            )}
                          </div>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </section>
      </div>

    </Layout>
  );
}

type LegacyImportRun = {
  ID?: string;
  Status?: string;
  RowsRead?: number;
  RowsInserted?: number;
  RowsSkipped?: number;
  ErrorMessage?: { String?: string; Valid?: boolean } | string;
};

function LegacyImport() {
  const [year, setYear] = useState(String(new Date().getFullYear()));
  const [source, setSource] = useState("phone");
  const [run, setRun] = useState<LegacyImportRun | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [mappedMode, setMappedMode] = useState(false);
  const status = run?.Status ?? "";

  async function loadStatus() {
    try {
      const response = await apiFetch("/imports/legacy");
      if (response.status === 404) return;
      if (!response.ok) throw new Error(await readError(response));
      setRun(await response.json());
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load import status");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    loadStatus();
  }, []);

  useEffect(() => {
    if (status !== "queued" && status !== "running") return;
    const timer = window.setInterval(loadStatus, 2000);
    return () => window.clearInterval(timer);
  }, [status]);

  async function startImport() {
    if (!window.confirm("Start the historical import for this company? It can only be started once.")) return;
    setSaving(true);
    setError("");
    try {
      const response = await apiFetch("/imports/legacy", {
        method: "POST",
        body: JSON.stringify({ year: Number(year), source }),
      });
      if (!response.ok) throw new Error(await readError(response));
      setRun(await response.json());
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to start historical import");
    } finally {
      setSaving(false);
    }
  }

  const errorMessage = typeof run?.ErrorMessage === "string"
    ? run.ErrorMessage
    : run?.ErrorMessage?.String;

  return (
    <Layout>
      <PageHeader eyebrow="Administration" title="Historical import" description="Import the existing company records into the normal OMS screens." />
      {error && <div className="alert error">{error}</div>}
      <section className="card form-card">
        <div className="form-actions">
          <button className="button ghost" type="button" onClick={() => setMappedMode(false)}>Fixed Nephot import</button>
          <button className="button ghost" type="button" onClick={() => setMappedMode(true)}>Upload mapped files</button>
        </div>
        {mappedMode ? <MappedImport /> : loading ? <p className="muted">Loading import status...</p> : run ? (
          <div className="stack">
            <div className="section-heading"><div><span className="eyebrow">Import status</span><h2>{status}</h2></div></div>
            <div className="compact-list">
              <div className="compact-row"><span>Rows read</span><strong>{run.RowsRead ?? 0}</strong></div>
              <div className="compact-row"><span>Rows inserted</span><strong>{run.RowsInserted ?? 0}</strong></div>
              <div className="compact-row"><span>Rows skipped</span><strong>{run.RowsSkipped ?? 0}</strong></div>
            </div>
            {status === "failed" && <>
              <div className="alert error">{errorMessage || "The import failed."}</div>
              <button className="button primary" type="button" onClick={() => void startImport()} disabled={saving}>Retry import</button>
            </>}
            {status === "completed" && <div className="alert success">Import completed. Imported records are available in Orders, Customers, Products, and Couriers.</div>}
            {(status === "queued" || status === "running") && <p className="muted">The import is running. This page refreshes automatically.</p>}
          </div>
        ) : (
          <form className="stack" onSubmit={(event) => { event.preventDefault(); void startImport(); }}>
            <div className="section-heading"><div><span className="eyebrow">One-time setup</span><h2>Import historical records</h2></div></div>
            <p className="muted">This imports the existing CSV history for this company and ignores duplicate historical rows.</p>
            <div className="form-grid">
              <label>Historical year<input type="number" min="2000" max="2100" value={year} onChange={(event) => setYear(event.target.value)} required /></label>
              <label>Default order source<select value={source} onChange={(event) => setSource(event.target.value)}><option value="website">Website</option><option value="daraz">Daraz</option><option value="phone">Phone</option><option value="facebook">Facebook</option><option value="instagram">Instagram</option><option value="store">Store</option></select></label>
            </div>
            <button className="button primary" type="submit" disabled={saving}>{saving ? "Starting..." : "Start historical import"}</button>
          </form>
        )}
      </section>
    </Layout>
  );
}

type UploadedTable = { id: string; status: string; headers: string[]; preview: Array<Record<string, string>> };

function MappedImport() {
  const fields = ["name", "phone", "phone2", "address", "product", "quantity", "cod_amount", "status", "courier", "location", "remarks", "date"];
  const [ordersFile, setOrdersFile] = useState<File | null>(null);
  const [optionalFiles, setOptionalFiles] = useState<Record<string, File | null>>({ products: null, couriers: null, locations: null });
  const [upload, setUpload] = useState<UploadedTable | null>(null);
  const [mapping, setMapping] = useState<Record<string, string>>({});
  const [year, setYear] = useState(String(new Date().getFullYear()));
  const [source, setSource] = useState("phone");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");

  async function uploadFiles(event: React.FormEvent) {
    event.preventDefault();
    if (!ordersFile) return;
    setSaving(true);
    setError("");
    const form = new FormData();
    form.append("orders", ordersFile);
    Object.entries(optionalFiles).forEach(([name, file]) => { if (file) form.append(name, file); });
    try {
      const response = await apiFetch("/imports/mapped/upload", { method: "POST", body: form });
      if (!response.ok) throw new Error(await readError(response));
      const data: UploadedTable = await response.json();
      setUpload(data);
      const defaults: Record<string, string> = {};
      fields.forEach((field) => { const match = data.headers.find((header) => header.toLowerCase().replaceAll(" ", "_") === field); if (match) defaults[field] = match; });
      setMapping(defaults);
      setMessage("File uploaded. Review the mapping before importing.");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to upload files");
    } finally {
      setSaving(false);
    }
  }

  async function startMappedImport(event: React.FormEvent) {
    event.preventDefault();
    if (!upload) return;
    if (!window.confirm("Start importing these mapped records into this company?")) return;
    setSaving(true);
    setError("");
    try {
      const mappingResponse = await apiFetch(`/imports/mapped/${upload.id}/mapping`, { method: "PUT", body: JSON.stringify({ mapping }) });
      if (!mappingResponse.ok) throw new Error(await readError(mappingResponse));
      const response = await apiFetch(`/imports/mapped/${upload.id}/start`, { method: "POST", body: JSON.stringify({ year: Number(year), source, mapping }) });
      if (!response.ok) throw new Error(await readError(response));
      setMessage("Mapped import started. Check the fixed import status for progress.");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to start mapped import");
    } finally {
      setSaving(false);
    }
  }

  if (!upload) return (
    <form className="stack" onSubmit={uploadFiles}>
      <div className="section-heading"><div><span className="eyebrow">Flexible import</span><h2>Upload company files</h2></div></div>
      <label>Orders file <input type="file" accept=".csv,.xlsx" onChange={(event) => setOrdersFile(event.target.files?.[0] ?? null)} required /></label>
      <label>Products file (optional) <input type="file" accept=".csv,.xlsx" onChange={(event) => setOptionalFiles((current) => ({ ...current, products: event.target.files?.[0] ?? null }))} /></label>
      <label>Couriers file (optional) <input type="file" accept=".csv,.xlsx" onChange={(event) => setOptionalFiles((current) => ({ ...current, couriers: event.target.files?.[0] ?? null }))} /></label>
      <label>Locations file (optional) <input type="file" accept=".csv,.xlsx" onChange={(event) => setOptionalFiles((current) => ({ ...current, locations: event.target.files?.[0] ?? null }))} /></label>
      {error && <div className="alert error">{error}</div>}
      <button className="button primary" type="submit" disabled={saving}>{saving ? "Uploading..." : "Upload files"}</button>
    </form>
  );

  return (
    <form className="stack" onSubmit={startMappedImport}>
      <div className="section-heading"><div><span className="eyebrow">Column mapping</span><h2>Map your columns</h2></div></div>
      {message && <div className="alert success">{message}</div>}
      <div className="form-grid">{fields.map((field) => <label key={field}>{field.replaceAll("_", " ")}<select value={mapping[field] ?? ""} onChange={(event) => setMapping((current) => ({ ...current, [field]: event.target.value }))}><option value="">No mapping</option>{upload.headers.map((header) => <option key={header} value={header}>{header}</option>)}</select></label>)}</div>
      <label>Historical year<input type="number" min="2000" max="2100" value={year} onChange={(event) => setYear(event.target.value)} required /></label>
      <label>Default order source<select value={source} onChange={(event) => setSource(event.target.value)}><option value="website">Website</option><option value="daraz">Daraz</option><option value="phone">Phone</option><option value="facebook">Facebook</option><option value="instagram">Instagram</option><option value="store">Store</option></select></label>
      <div className="table-wrap"><table><thead><tr>{upload.headers.map((header) => <th key={header}>{header}</th>)}</tr></thead><tbody>{upload.preview.map((row, index) => <tr key={index}>{upload.headers.map((header) => <td key={header}>{row[header]}</td>)}</tr>)}</tbody></table></div>
      {error && <div className="alert error">{error}</div>}
      <button className="button primary" type="submit" disabled={saving}>{saving ? "Starting..." : "Confirm and import"}</button>
    </form>
  );
}

function Couriers() {
  const [couriers, setCouriers] = useState<Courier[]>([]);
  const [locations, setLocations] = useState<CourierLocation[]>([]);
  const [selectedCourier, setSelectedCourier] = useState("");
  const [courierName, setCourierName] = useState("");
  const [locationName, setLocationName] = useState("");
  const [deliveryCharge, setDeliveryCharge] = useState("");
  const [editingCourier, setEditingCourier] = useState<string | null>(null);
  const [editingLocation, setEditingLocation] = useState<string | null>(null);
  const [error, setError] = useState("");

  async function loadCouriers() {
    const response = await apiFetch("/couriers");
    if (!response.ok) throw new Error(await readError(response));
    const data = (await response.json()).map(normalizeCourier);
    setCouriers(data);
    if (!selectedCourier && data[0]?.id) setSelectedCourier(data[0].id);
  }

  async function loadLocations(courierID = selectedCourier) {
    if (!courierID) return;
    const response = await apiFetch(`/couriers/${courierID}/locations`);
    if (!response.ok) throw new Error(await readError(response));
    setLocations((await response.json()).map(normalizeCourierLocation));
  }

  useEffect(() => {
    const timer = window.setTimeout(() => {
      loadCouriers().catch((err) => setError(err instanceof Error ? err.message : "Failed to load couriers"));
    }, 0);
    return () => window.clearTimeout(timer);
  }, []);

  useEffect(() => {
    const timer = window.setTimeout(() => {
      loadLocations().catch((err) => setError(err instanceof Error ? err.message : "Failed to load locations"));
    }, 0);
    return () => window.clearTimeout(timer);
  }, [selectedCourier]);

  async function saveCourier() {
    const path = editingCourier ? `/couriers/${editingCourier}` : "/couriers";
    const response = await apiFetch(path, { method: editingCourier ? "PATCH" : "POST", body: JSON.stringify({ name: courierName }) });
    if (!response.ok) { setError(await readError(response)); return; }
    setCourierName(""); setEditingCourier(null); await loadCouriers();
  }

  async function removeCourier(id: string) {
    const response = await apiFetch(`/couriers/${id}`, { method: "DELETE" });
    if (!response.ok) { setError(await readError(response)); return; }
    setSelectedCourier(""); await loadCouriers();
  }

  async function saveLocation() {
    if (!selectedCourier) return;
    const path = editingLocation ? `/couriers/${selectedCourier}/locations/${editingLocation}` : `/couriers/${selectedCourier}/locations`;
    const response = await apiFetch(path, { method: editingLocation ? "PATCH" : "POST", body: JSON.stringify({ location_name: locationName, delivery_charge: deliveryCharge }) });
    if (!response.ok) { setError(await readError(response)); return; }
    setLocationName(""); setDeliveryCharge(""); setEditingLocation(null); await loadLocations();
  }

  async function removeLocation(id: string) {
    if (!selectedCourier) return;
    const response = await apiFetch(`/couriers/${selectedCourier}/locations/${id}`, { method: "DELETE" });
    if (!response.ok) { setError(await readError(response)); return; }
    await loadLocations();
  }

  return (
    <Layout>
      <PageHeader eyebrow="Admin" title="Couriers and locations" description="Manage courier-owned delivery locations" />
      {error && <div className="alert error">{error}</div>}
      <div className="detail-grid">
        <section className="card">
          <h2>Couriers</h2>
          <div className="stack"><input value={courierName} onChange={(event) => setCourierName(event.target.value)} placeholder="Courier name" /><button className="button primary" type="button" onClick={saveCourier}>{editingCourier ? "Save courier" : "Add courier"}</button></div>
          <div className="compact-list">{couriers.map((courier) => <div className="compact-row" key={courier.id}><button className="text-link" type="button" onClick={() => setSelectedCourier(courier.id)}>{courier.name}</button><div className="courier-actions"><button className="button ghost" type="button" onClick={() => { setEditingCourier(courier.id); setCourierName(courier.name); }}>Edit</button><button className="button ghost" type="button" onClick={() => removeCourier(courier.id)}>Delete</button></div></div>)}</div>
        </section>
        <section className="card">
          <h2>Locations</h2>
          <select value={selectedCourier} onChange={(event) => setSelectedCourier(event.target.value)}><option value="">Select courier</option>{couriers.map((courier) => <option key={courier.id} value={courier.id}>{courier.name}</option>)}</select>
          <div className="stack"><input value={locationName} onChange={(event) => setLocationName(event.target.value)} placeholder="Location name" /><input value={deliveryCharge} onChange={(event) => setDeliveryCharge(event.target.value)} placeholder="Delivery charge" /><button className="button primary" type="button" disabled={!selectedCourier} onClick={saveLocation}>{editingLocation ? "Save location" : "Add location"}</button></div>
          <div className="compact-list">{locations.map((location) => <div className="compact-row" key={location.id}><span>{location.location_name} ({money(location.delivery_charge)})</span><div className="courier-actions"><button className="button ghost" type="button" onClick={() => { setEditingLocation(location.id); setLocationName(location.location_name); setDeliveryCharge(String(location.delivery_charge ?? "")); }}>Edit</button><button className="button ghost" type="button" onClick={() => removeLocation(location.id)}>Delete</button></div></div>)}</div>
        </section>
      </div>
    </Layout>
  );
}

function StatusBadge({ status }: { status: string }) {
  const color = ["cancelled", "returned"].includes(status)
    ? "status-red"
    : ["follow_up", "hold", "redirected"].includes(status)
      ? "status-amber"
      : "status-green";

  return (
    <span className={`status ${color}`}>
      {statusLabel(status)}
    </span>
  );
}

function EmptyState({ message }: { message: string }) {
  return (
    <div className="empty-state">
      <strong>{message}</strong>
      <span>Nothing to show here yet.</span>
    </div>
  );
}

function ProtectedRoute({
  children,
}: {
  children: React.ReactNode;
}) {
  if (!token()) {
    return <Navigate to="/login" replace />;
  }

  return children;
}

function RoleProtectedRoute({
  roles,
  children,
}: {
  roles: Role[];
  children: React.ReactNode;
}) {
  const role = getRole();

  if (!role || !roles.includes(role)) {
    return <Navigate to="/" replace />;
  }

  return children;
}

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route path="/invite/:token" element={<Invitation />} />

        <Route
          path="/"
          element={
            <ProtectedRoute>
              <Dashboard />
            </ProtectedRoute>
          }
        />

        <Route
          path="/customers"
          element={
            <ProtectedRoute>
              <Customers />
            </ProtectedRoute>
          }
        />

        <Route
          path="/customers/new"
          element={
            <ProtectedRoute>
              <CustomerForm />
            </ProtectedRoute>
          }
        />

        <Route
          path="/customers/:id"
          element={
            <ProtectedRoute>
              <CustomerDetail />
            </ProtectedRoute>
          }
        />

        <Route
          path="/products"
          element={
            <ProtectedRoute>
              <Products />
            </ProtectedRoute>
          }
        />

        <Route
          path="/products/new"
          element={
            <RoleProtectedRoute roles={["admin", "superadmin"]}>
              <ProductForm />
            </RoleProtectedRoute>
          }
        />

        <Route
          path="/products/:id/edit"
          element={
            <RoleProtectedRoute roles={["admin", "superadmin"]}>
              <ProductForm />
            </RoleProtectedRoute>
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
          path="/orders/problems"
          element={
            <ProtectedRoute>
              <ProblemOrders />
            </ProtectedRoute>
          }
        />

        <Route
          path="/orders/new"
          element={
            <ProtectedRoute>
              <CreateOrder />
            </ProtectedRoute>
          }
        />

        <Route
          path="/orders/:id"
          element={
            <ProtectedRoute>
              <OrderDetail />
            </ProtectedRoute>
          }
        />

        <Route
          path="/followups"
          element={
            <ProtectedRoute>
              <FollowUps />
            </ProtectedRoute>
          }
        />

        <Route
          path="/couriers"
          element={
            <RoleProtectedRoute roles={["admin", "superadmin"]}>
              <Couriers />
            </RoleProtectedRoute>
          }
        />

        <Route
          path="/users"
          element={
            <RoleProtectedRoute roles={["admin", "superadmin"]}>
              <Users />
            </RoleProtectedRoute>
          }
        />

        <Route
          path="/imports"
          element={
            <RoleProtectedRoute roles={["superadmin"]}>
              <LegacyImport />
            </RoleProtectedRoute>
          }
        />

        <Route
          path="*"
          element={<Navigate to="/" replace />}
        />
      </Routes>
    </BrowserRouter>
  );
}

function LegacyBadge() {
  return <span className="status legacy-badge">Legacy</span>;
}

function ProblemOrders() {
  const [orders, setOrders] = useState<Order[]>([]);
  const [error, setError] = useState("");

  useEffect(() => {
    async function load() {
      try {
        const response = await apiFetch("/orders/problems");
        if (!response.ok) throw new Error(await readError(response));
        setOrders(await response.json());
      } catch (err) {
        setError(err instanceof Error ? err.message : "Failed to load problem orders");
      }
    }
    load();
  }, []);

  return (
    <Layout>
      <PageHeader eyebrow="Orders" title="Problem orders" description="Follow-up, hold, redirect, cancelled, and returned orders" />
      {error && <div className="alert error">{error}</div>}
      <div className="card table-card"><div className="table-wrap"><table><thead><tr><th>Order</th><th>Status</th><th>Source</th><th>Created</th></tr></thead><tbody>{orders.map((order) => <tr key={order.id}><td><Link className="text-link" to={`/orders/${order.id}`}>{order.id.slice(0, 8)}</Link></td><td><StatusBadge status={order.status} /></td><td>{order.source}</td><td>{formatDate(order.created_at)}</td></tr>)}</tbody></table></div></div>
    </Layout>
  );
}