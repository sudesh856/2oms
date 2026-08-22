import { describe, expect, it } from "vitest";
import {
  calculateCartTotal,
  normalizeCustomer,
  normalizeOrder,
  normalizeOrderItem,
  normalizeProduct,
} from "./orderHelpers";

describe("Create Order data helpers", () => {
  it("normalizes customer API fields for display", () => {
    const customer = normalizeCustomer({
      ID: "customer-1",
      Name: "Hari",
      Phone: "9800000000",
      Address: "Kathmandu",
    });

    expect(customer).toMatchObject({
      id: "customer-1",
      name: "Hari",
      phone: "9800000000",
      address: "Kathmandu",
    });
  });

  it("normalizes product API fields for display", () => {
    const product = normalizeProduct({
      ID: "product-1",
      Name: "Razor",
      Price: "125.50",
      AvailableQty: 12,
      WarehouseQty: 20,
    });

    expect(product).toMatchObject({
      id: "product-1",
      name: "Razor",
      price: "125.50",
      available_qty: 12,
      warehouse_qty: 20,
    });
  });

  it("calculates the order total from a real product price", () => {
    const product = normalizeProduct({
      ID: "product-1",
      Name: "Razor",
      Price: "125.50",
      AvailableQty: 12,
      WarehouseQty: 20,
    });

    expect(calculateCartTotal([{ product, quantity: 2 }])).toBe(251);
    expect(Number.isNaN(calculateCartTotal([{ product, quantity: 2 }]))).toBe(false);
  });
});

describe("Order Detail data helpers", () => {
  it("normalizes order API fields including COD amount", () => {
    const order = normalizeOrder({
      ID: "order-123",
      CustomerID: "cust-456",
      Source: "website",
      Status: "confirmed",
      Address: "Kathmandu, Nepal",
      CodAmount: "150.00",
      IsStoreVisit: false,
      CreatedBy: "user-789",
      CreatedAt: "2026-08-22T10:00:00Z",
    });

    expect(order).toMatchObject({
      id: "order-123",
      customer_id: "cust-456",
      source: "website",
      status: "confirmed",
      address: "Kathmandu, Nepal",
      cod_amount: "150.00",
      is_store_visit: false,
      created_by: "user-789",
      created_at: "2026-08-22T10:00:00Z",
    });
  });

  it("normalizes order when COD amount is omitted for staff", () => {
    const order = normalizeOrder({
      id: "order-123",
      customer_id: "cust-456",
      source: "phone",
      status: "confirmed",
      address: "Lalitpur",
    });

    expect(order.cod_amount).toBeUndefined();
    expect(order.id).toBe("order-123");
    expect(order.customer_id).toBe("cust-456");
  });

  it("normalizes order item API fields for display", () => {
    const item = normalizeOrderItem({
      ID: "item-1",
      OrderID: "order-123",
      ProductID: "prod-razor",
      Quantity: 1,
      Price: "150.00",
    });

    expect(item).toMatchObject({
      id: "item-1",
      order_id: "order-123",
      product_id: "prod-razor",
      quantity: 1,
      price: "150.00",
    });
  });

  it("matches normalized order item with normalized product in productMap", () => {
    const rawProducts = [
      {
        ID: "prod-razor",
        Name: "Razor",
        Price: "150.00",
        AvailableQty: 10,
        WarehouseQty: 20,
      },
    ];
    const rawItems = [
      {
        ID: "item-1",
        OrderID: "order-123",
        ProductID: "prod-razor",
        Quantity: 2,
        Price: "150.00",
      },
    ];

    const products = rawProducts.map(normalizeProduct);
    const items = rawItems.map(normalizeOrderItem);
    const productMap = new Map(products.map((p) => [p.id, p]));

    const item = items[0];
    const product = productMap.get(item.product_id);

    expect(product).toBeDefined();
    expect(product?.name).toBe("Razor");
    expect(Number(item.price) * item.quantity).toBe(300);
  });
});