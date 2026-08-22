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