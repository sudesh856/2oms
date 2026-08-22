import { describe, expect, it } from "vitest";
import {
  calculateCartTotal,
  normalizeCustomer,
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