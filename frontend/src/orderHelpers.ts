export type Customer = {
  id: string;
  phone: string;
  phone2?: string | null;
  name: string;
  address?: string | null;
  created_at?: string;
};

export function normalizeCustomer(value?: (Partial<Customer> & {
  ID?: string;
  Phone?: string;
  Phone2?: string | null;
  Name?: string;
  Address?: string | null;
  CreatedAt?: string;
}) | null): Customer {
  const v = value ?? {};
  return {
    id: v.id ?? v.ID ?? "",
    phone: v.phone ?? v.Phone ?? "",
    phone2: v.phone2 ?? v.Phone2,
    name: v.name ?? v.Name ?? "",
    address: v.address ?? v.Address,
    created_at: v.created_at ?? v.CreatedAt,
  };
}

export type Product = {
  id: string;
  name: string;
  price: unknown;
  available_qty: number;
  warehouse_qty: number;
  created_at?: string;
};

export function normalizeProduct(value?: (Partial<Product> & {
  ID?: string;
  Name?: string;
  Price?: unknown;
  AvailableQty?: number;
  WarehouseQty?: number;
  CreatedAt?: string;
}) | null): Product {
  const v = value ?? {};
  return {
    id: v.id ?? v.ID ?? "",
    name: v.name ?? v.Name ?? "",
    price: v.price ?? v.Price,
    available_qty: v.available_qty ?? v.AvailableQty ?? 0,
    warehouse_qty: v.warehouse_qty ?? v.WarehouseQty ?? 0,
    created_at: v.created_at ?? v.CreatedAt,
  };
}

export type CartItem = {
  product: Product;
  quantity: number;
};

export function calculateCartTotal(cart: CartItem[]) {
  return cart.reduce(
    (sum, item) => sum + Number(item.product.price) * item.quantity,
    0
  );
}

export type Order = {
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
  is_legacy?: boolean;
};

export function normalizeOrder(value?: (Partial<Order> & {
  ID?: string;
  CustomerID?: string;
  Source?: string;
  Status?: string;
  CourierID?: string | null;
  LocationID?: string | null;
  Address?: string;
  CodAmount?: unknown;
  CODAmount?: unknown;
  IsStoreVisit?: boolean;
  CreatedBy?: string;
  CreatedAt?: string;
  UpdatedAt?: string;
  IsLegacy?: boolean;
}) | null): Order {
  const v = value ?? {};
  return {
    id: v.id ?? v.ID ?? "",
    customer_id: v.customer_id ?? v.CustomerID ?? "",
    source: v.source ?? v.Source ?? "",
    status: v.status ?? v.Status ?? "",
    courier_id: v.courier_id ?? v.CourierID,
    location_id: v.location_id ?? v.LocationID,
    address: v.address ?? v.Address ?? "",
    cod_amount: v.cod_amount ?? v.CodAmount ?? v.CODAmount,
    is_store_visit: v.is_store_visit ?? v.IsStoreVisit ?? false,
    created_by: v.created_by ?? v.CreatedBy ?? "",
    created_at: v.created_at ?? v.CreatedAt ?? "",
    updated_at: v.updated_at ?? v.UpdatedAt ?? "",
    is_legacy: v.is_legacy ?? v.IsLegacy,
  };
}

export type OrderItem = {
  id: string;
  order_id: string;
  product_id: string;
  quantity: number;
  price: unknown;
};

export function normalizeOrderItem(value?: (Partial<OrderItem> & {
  ID?: string;
  OrderID?: string;
  ProductID?: string;
  Quantity?: number;
  Price?: unknown;
}) | null): OrderItem {
  const v = value ?? {};
  return {
    id: String(v.id ?? v.ID ?? ""),
    order_id: v.order_id ?? v.OrderID ?? "",
    product_id: v.product_id ?? v.ProductID ?? "",
    quantity: Number(v.quantity ?? v.Quantity ?? 0),
    price: v.price ?? v.Price,
  };
}

export type Courier = {
  id: string;
  name: string;
  created_at?: string;
};

export function normalizeCourier(value?: (Partial<Courier> & {
  ID?: string;
  Name?: string;
  CreatedAt?: string;
}) | null): Courier {
  const v = value ?? {};
  return {
    id: v.id ?? v.ID ?? "",
    name: v.name ?? v.Name ?? "",
    created_at: v.created_at ?? v.CreatedAt,
  };
}

export type CourierLocation = {
  id: string;
  courier_id: string;
  location_name: string;
  delivery_charge: unknown;
  created_at?: string;
};

export function normalizeCourierLocation(value?: (Partial<CourierLocation> & {
  ID?: string;
  CourierID?: string;
  LocationName?: string;
  DeliveryCharge?: unknown;
  CreatedAt?: string;
}) | null): CourierLocation {
  const v = value ?? {};
  return {
    id: v.id ?? v.ID ?? "",
    courier_id: v.courier_id ?? v.CourierID ?? "",
    location_name: v.location_name ?? v.LocationName ?? "",
    delivery_charge: v.delivery_charge ?? v.DeliveryCharge,
    created_at: v.created_at ?? v.CreatedAt,
  };
}

export type FollowUp = {
  id: string;
  order_id: string;
  attempt_no: number;
  next_action: string;
  preferred_day?: string | null;
  next_action_date?: string | null;
  note?: string | null;
  assigned_to?: string | null;
  created_at?: string;
  status?: string;
  customer_id?: string;
  customer_name?: string;
  customer_phone?: string;
  assigned_to_name?: string | null;
  assigned_to_phone?: string | null;
};

export function normalizeFollowUp(value?: (Partial<FollowUp> & {
  ID?: string;
  OrderID?: string;
  AttemptNo?: number;
  NextAction?: string | { String?: string; Valid?: boolean };
  PreferredDay?: string | { String?: string; Valid?: boolean } | null;
  NextActionDate?: string | { Time?: string; Valid?: boolean } | null;
  Note?: string | { String?: string; Valid?: boolean } | null;
  AssignedTo?: string;
  CreatedAt?: string | { Time?: string; Valid?: boolean };
  Status?: string;
  CustomerID?: string;
  CustomerName?: string;
  CustomerPhone?: string;
  AssignedToName?: string | { String?: string; Valid?: boolean } | null;
  AssignedToPhone?: string | { String?: string; Valid?: boolean } | null;
}) | null): FollowUp {
  const v = value ?? {};
  const extractText = (val: unknown): string => {
    if (typeof val === "string") return val;
    if (val && typeof val === "object" && "String" in val) {
      return String((val as { String?: unknown }).String ?? "");
    }
    return "";
  };
  const extractDate = (val: unknown): string | null => {
    if (typeof val === "string") return val;
    if (val && typeof val === "object" && "Time" in val) {
      const t = val as { Time?: unknown; Valid?: boolean };
      return t.Valid && t.Time ? String(t.Time).slice(0, 10) : null;
    }
    return null;
  };

  return {
    id: v.id ?? v.ID ?? "",
    order_id: v.order_id ?? v.OrderID ?? "",
    attempt_no: Number(v.attempt_no ?? v.AttemptNo ?? 0),
    next_action: v.next_action ?? extractText(v.NextAction),
    preferred_day: (v.preferred_day ?? extractText(v.PreferredDay)) || null,
    next_action_date: v.next_action_date ?? extractDate(v.NextActionDate),
    note: (v.note ?? extractText(v.Note)) || null,
    assigned_to: v.assigned_to ?? v.AssignedTo,
    created_at: typeof v.created_at === "string" ? v.created_at : (extractDate(v.CreatedAt) || ""),
    status: v.status ?? v.Status,
    customer_id: v.customer_id ?? v.CustomerID,
    customer_name: v.customer_name ?? v.CustomerName ?? "",
    customer_phone: v.customer_phone ?? v.CustomerPhone ?? "",
    assigned_to_name: (v.assigned_to_name ?? extractText(v.AssignedToName)) || null,
    assigned_to_phone: (v.assigned_to_phone ?? extractText(v.AssignedToPhone)) || null,
  };
}

export const VALID_NEXT_STATUSES: Record<string, string[]> = {
  confirmed: ["pickup_complete", "follow_up", "hold", "cancelled"],
  pickup_complete: ["dispatched", "follow_up", "hold", "redirected", "cancelled"],
  dispatched: ["arrived", "follow_up", "hold", "redirected", "cancelled", "returned"],
  arrived: ["delivered", "follow_up", "hold", "redirected", "cancelled", "returned"],
  follow_up: ["confirmed", "pickup_complete", "hold", "cancelled"],
  hold: ["confirmed", "pickup_complete", "cancelled"],
  redirected: ["dispatched", "arrived", "cancelled"],
  delivered: [],
  cancelled: [],
  returned: [],
};

export function getValidNextStatuses(currentStatus?: string): string[] {
  if (!currentStatus) return [];
  return VALID_NEXT_STATUSES[currentStatus] || [];
}

export function filterCourierLocations(
  locations: CourierLocation[],
  query: string
): CourierLocation[] {
  const trimmed = query.trim().toLowerCase();
  if (!trimmed) {
    return locations;
  }
  return locations.filter((loc) =>
    loc.location_name.toLowerCase().includes(trimmed)
  );
}
