import { describe, expect, it } from "vitest";
import {
  calculateCartTotal,
  filterCourierLocations,
  getValidNextStatuses,
  normalizeCourier,
  normalizeCourierLocation,
  normalizeCustomer,
  normalizeOrder,
  normalizeOrderItem,
  normalizeProduct,
  type CourierLocation,
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

describe("Courier and Status Transition helpers", () => {
  it("normalizes courier and location API fields", () => {
    const courier = normalizeCourier({ ID: "cour-1", Name: "Fast Delivery" });
    expect(courier).toEqual({
      id: "cour-1",
      name: "Fast Delivery",
      created_at: undefined,
    });

    const location = normalizeCourierLocation({
      ID: "loc-1",
      CourierID: "cour-1",
      LocationName: "Kathmandu Valley",
      DeliveryCharge: "100.00",
    });
    expect(location).toEqual({
      id: "loc-1",
      courier_id: "cour-1",
      location_name: "Kathmandu Valley",
      delivery_charge: "100.00",
      created_at: undefined,
    });
  });

  it("returns correct valid next statuses for all order statuses", () => {
    expect(getValidNextStatuses("confirmed")).toEqual([
      "pickup_complete",
      "follow_up",
      "hold",
      "cancelled",
    ]);
    expect(getValidNextStatuses("pickup_complete")).toEqual([
      "dispatched",
      "follow_up",
      "hold",
      "redirected",
      "cancelled",
    ]);
    expect(getValidNextStatuses("dispatched")).toEqual([
      "arrived",
      "follow_up",
      "hold",
      "redirected",
      "cancelled",
      "returned",
    ]);
    expect(getValidNextStatuses("delivered")).toEqual([]);
    expect(getValidNextStatuses("cancelled")).toEqual([]);
    expect(getValidNextStatuses("returned")).toEqual([]);
  });

  it("filters courier locations by search query", () => {
    const locations: CourierLocation[] = [
      { id: "1", courier_id: "c1", location_name: "Kathmandu Valley", delivery_charge: "100" },
      { id: "2", courier_id: "c1", location_name: "Pokhara Lakeside", delivery_charge: "150" },
      { id: "3", courier_id: "c1", location_name: "Butwal Traffic Chowk", delivery_charge: "150" },
      { id: "4", courier_id: "c1", location_name: "Biratnagar Main", delivery_charge: "180" },
    ];

    expect(filterCourierLocations(locations, "")).toHaveLength(4);
    expect(filterCourierLocations(locations, "pokhara")).toEqual([
      { id: "2", courier_id: "c1", location_name: "Pokhara Lakeside", delivery_charge: "150" },
    ]);
    expect(filterCourierLocations(locations, "TRAFFIC")).toEqual([
      { id: "3", courier_id: "c1", location_name: "Butwal Traffic Chowk", delivery_charge: "150" },
    ]);
    expect(filterCourierLocations(locations, "nonexistent")).toHaveLength(0);
  });
});