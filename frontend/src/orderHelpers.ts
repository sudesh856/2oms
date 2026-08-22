export type Customer = {
  id: string;
  phone: string;
  phone2?: string | null;
  name: string;
  address?: string | null;
  created_at?: string;
};

export function normalizeCustomer(value: Partial<Customer> & {
  ID?: string;
  Phone?: string;
  Phone2?: string | null;
  Name?: string;
  Address?: string | null;
  CreatedAt?: string;
}): Customer {
  return {
    id: value.id ?? value.ID ?? "",
    phone: value.phone ?? value.Phone ?? "",
    phone2: value.phone2 ?? value.Phone2,
    name: value.name ?? value.Name ?? "",
    address: value.address ?? value.Address,
    created_at: value.created_at ?? value.CreatedAt,
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

export function normalizeProduct(value: Partial<Product> & {
  ID?: string;
  Name?: string;
  Price?: unknown;
  AvailableQty?: number;
  WarehouseQty?: number;
  CreatedAt?: string;
}): Product {
  return {
    id: value.id ?? value.ID ?? "",
    name: value.name ?? value.Name ?? "",
    price: value.price ?? value.Price,
    available_qty: value.available_qty ?? value.AvailableQty ?? 0,
    warehouse_qty: value.warehouse_qty ?? value.WarehouseQty ?? 0,
    created_at: value.created_at ?? value.CreatedAt,
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

export function normalizeOrder(value: Partial<Order> & {
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
}): Order {
  return {
    id: value.id ?? value.ID ?? "",
    customer_id: value.customer_id ?? value.CustomerID ?? "",
    source: value.source ?? value.Source ?? "",
    status: value.status ?? value.Status ?? "",
    courier_id: value.courier_id ?? value.CourierID,
    location_id: value.location_id ?? value.LocationID,
    address: value.address ?? value.Address ?? "",
    cod_amount: value.cod_amount ?? value.CodAmount ?? value.CODAmount,
    is_store_visit: value.is_store_visit ?? value.IsStoreVisit ?? false,
    created_by: value.created_by ?? value.CreatedBy ?? "",
    created_at: value.created_at ?? value.CreatedAt ?? "",
    updated_at: value.updated_at ?? value.UpdatedAt ?? "",
    is_legacy: value.is_legacy ?? value.IsLegacy,
  };
}

export type OrderItem = {
  id: string;
  order_id: string;
  product_id: string;
  quantity: number;
  price: unknown;
};

export function normalizeOrderItem(value: Partial<OrderItem> & {
  ID?: string;
  OrderID?: string;
  ProductID?: string;
  Quantity?: number;
  Price?: unknown;
}): OrderItem {
  return {
    id: String(value.id ?? value.ID ?? ""),
    order_id: value.order_id ?? value.OrderID ?? "",
    product_id: value.product_id ?? value.ProductID ?? "",
    quantity: Number(value.quantity ?? value.Quantity ?? 0),
    price: value.price ?? value.Price,
  };
}

export type Courier = {
  id: string;
  name: string;
  created_at?: string;
};

export function normalizeCourier(value: Partial<Courier> & {
  ID?: string;
  Name?: string;
  CreatedAt?: string;
}): Courier {
  return {
    id: value.id ?? value.ID ?? "",
    name: value.name ?? value.Name ?? "",
    created_at: value.created_at ?? value.CreatedAt,
  };
}

export type CourierLocation = {
  id: string;
  courier_id: string;
  location_name: string;
  delivery_charge: unknown;
  created_at?: string;
};

export function normalizeCourierLocation(value: Partial<CourierLocation> & {
  ID?: string;
  CourierID?: string;
  LocationName?: string;
  DeliveryCharge?: unknown;
  CreatedAt?: string;
}): CourierLocation {
  return {
    id: value.id ?? value.ID ?? "",
    courier_id: value.courier_id ?? value.CourierID ?? "",
    location_name: value.location_name ?? value.LocationName ?? "",
    delivery_charge: value.delivery_charge ?? value.DeliveryCharge,
    created_at: value.created_at ?? value.CreatedAt,
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
