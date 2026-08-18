import { useEffect, useMemo, useState } from "react";
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

const API = "http://localhost:8080/api";

type Role = "superadmin" | "admin" | "staff";

type TokenPayload = {
  role: Role;
  user_id?: string;
  exp?: number;
};

type Customer = {
  id: string;
  phone: string;
  phone2?: string | null;
  name: string;
  address?: string | null;
  created_at?: string;
};

type Product = {
  id: string;
  name: string;
  price: unknown;
  available_qty: number;
  warehouse_qty: number;
  created_at?: string;
};

type Order = {
  id: string;
  customer_id: string;
  source: string;
  status: string;
  courier_id?: string | null;
  location_id?: string | null;
  address: string;
  cod_amount?: unknown;
  is_store_visit: boolean;
  created_by: string;
  created_at: string;
  updated_at: string;
};

type OrderItem = {
  id: number;
  order_id: string;
  product_id: string;
  quantity: number;
  price: unknown;
};

type CartItem = {
  product: Product;
  quantity: number;
};

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

  if (options.body && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }

  const response = await fetch(`${API}${path}`, {
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

function statusLabel(status: string) {
  return status.replaceAll("_", " ");
}

function Login() {
  const navigate = useNavigate();

  const [phone, setPhone] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function handleLogin(event: React.FormEvent) {
    event.preventDefault();

    setLoading(true);
    setError("");

    try {
      const response = await fetch(`${API}/auth/login`, {
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

  return (
    <div className="login-page">
      <div className="login-card">
        <div className="brand-mark">N</div>

        <p className="eyebrow">NEPHOT</p>
        <h1>Order Management</h1>
        <p className="muted">
          Sign in to manage customers, products and orders.
        </p>

        <form onSubmit={handleLogin} className="stack">
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
            {loading ? "Signing in..." : "Sign in"}
          </button>
        </form>
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

        <main className="content">{children}</main>
      </div>
    </div>
  );
}

function Dashboard() {
  const [orders, setOrders] = useState<Order[]>([]);
  const [customers, setCustomers] = useState<Customer[]>([]);
  const [products, setProducts] = useState<Product[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    async function load() {
      try {
        const [ordersResponse, customersResponse, productsResponse] =
          await Promise.all([
            apiFetch("/orders"),
            apiFetch("/customers"),
            apiFetch("/products"),
          ]);

        if (
          !ordersResponse.ok ||
          !customersResponse.ok ||
          !productsResponse.ok
        ) {
          throw new Error("Failed to load dashboard");
        }

        setOrders(await ordersResponse.json());
        setCustomers(await customersResponse.json());
        setProducts(await productsResponse.json());
      } catch {
        // Individual pages expose detailed errors.
      } finally {
        setLoading(false);
      }
    }

    load();
  }, []);

  const activeOrders = orders.filter(
    (order) =>
      !["delivered", "cancelled", "returned"].includes(order.status)
  ).length;

  const lowStock = products.filter(
    (product) => product.available_qty <= 5
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
              label="Total orders"
              value={orders.length}
              href="/orders"
            />
            <StatCard
              label="Active orders"
              value={activeOrders}
              href="/orders"
            />
            <StatCard
              label="Customers"
              value={customers.length}
              href="/customers"
            />
            <StatCard
              label="Low stock"
              value={lowStock}
              href="/products"
            />
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

      setCustomers(await response.json());
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

      const customer = await response.json();

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
  const [error, setError] = useState("");

  useEffect(() => {
    async function load() {
      if (!id) return;

      try {
        const response = await apiFetch(`/customers/${id}`);

        if (!response.ok) {
          throw new Error(await readError(response));
        }

        setCustomer(await response.json());
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
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

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

      setProducts(await response.json());
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
              New product
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
          <input
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder="Search products..."
          />
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

  const [loadingProducts, setLoadingProducts] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const customerId = params.get("customer");

    if (!customerId) return;

    async function loadCustomer() {
      try {
        const response = await apiFetch(`/customers/${customerId}`);

        if (response.ok) {
          const data = await response.json();
          setCustomer(data);
          setAddress(data.address || "");
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

      setCustomerResults(await response.json());
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

      setProducts(await response.json());
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
        .filter((item) => item.quantity > 0)
    );
  }

  const total = useMemo(() => {
    return cart.reduce((sum, item) => {
      return (
        sum +
        Number(item.product.price) * item.quantity
      );
    }, 0);
  }, [cart]);

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
  const [orders, setOrders] = useState<Order[]>([]);
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    async function load() {
      try {
        const response = await apiFetch("/orders");

        if (!response.ok) {
          throw new Error(await readError(response));
        }

        setOrders(await response.json());
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
  }, []);

  const filtered = orders.filter((order) => {
    const matchesSearch =
      !search ||
      order.id.toLowerCase().includes(search.toLowerCase()) ||
      order.customer_id
        .toLowerCase()
        .includes(search.toLowerCase()) ||
      order.source.toLowerCase().includes(search.toLowerCase());

    const matchesStatus =
      !status || order.status === status;

    return matchesSearch && matchesStatus;
  });

  return (
    <Layout>
      <PageHeader
        eyebrow="Orders"
        title="Order management"
        description={`${orders.length} total orders`}
        action={
          <Link to="/orders/new" className="button primary">
            Create order
          </Link>
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
        </div>
      </div>

      {error && <div className="alert error">{error}</div>}

      <div className="card table-card">
        {loading ? (
          <p className="muted">Loading orders...</p>
        ) : filtered.length === 0 ? (
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
                {filtered.map((order) => (
                  <tr key={order.id}>
                    <td>
                      <strong>{order.id.slice(0, 8)}</strong>
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
  const [newStatus, setNewStatus] = useState("");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");

  async function load() {
    if (!id) return;

    setLoading(true);
    setError("");

    try {
      const orderResponse = await apiFetch(`/orders/${id}`);

      if (!orderResponse.ok) {
        throw new Error(await readError(orderResponse));
      }

      const orderData: Order = await orderResponse.json();

      setOrder(orderData);
      setNewStatus(orderData.status);

      const customerResponse = await apiFetch(
        `/customers/${orderData.customer_id}`
      );

      if (customerResponse.ok) {
        setCustomer(await customerResponse.json());
      }

      const itemsResponse = await apiFetch(
        `/orders/${id}/items`
      );

      if (itemsResponse.ok) {
        setItems(await itemsResponse.json());
      }

      const productsResponse = await apiFetch("/products");

      if (productsResponse.ok) {
        setProducts(await productsResponse.json());
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

      setOrder(await response.json());
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

  const productMap = new Map(
    products.map((product) => [product.id, product])
  );

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
                >
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
              </label>

              <button
                className="button primary full"
                type="button"
                disabled={
                  saving || newStatus === order.status
                }
                onClick={updateStatus}
              >
                {saving ? "Updating..." : "Update status"}
              </button>

              <p className="muted small">
                The backend validates whether the requested
                transition is allowed.
              </p>

              {role === "staff" && (
                <p className="muted small">
                  Staff status changes are subject to the same
                  backend workflow rules.
                </p>
              )}
            </div>
          </aside>
        </div>
      ) : null}
    </Layout>
  );
}

function StatusBadge({ status }: { status: string }) {
  return (
    <span className={`status status-${status}`}>
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
          path="*"
          element={<Navigate to="/" replace />}
        />
      </Routes>
    </BrowserRouter>
  );
}