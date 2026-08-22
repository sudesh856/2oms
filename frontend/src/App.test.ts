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
  normalizeFollowUp,
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

describe("Problem Orders and Unsafe Truncation resilience", () => {
  it("normalizes raw SQLC PascalCase problem orders payload from /orders/problems", () => {
    const rawProblemOrders = [
      {
        ID: "e8574e44-1234-4567-89ab-cdef01234567",
        CustomerID: "cust-9999-uuid",
        Source: "facebook",
        Status: "follow_up",
        Address: "Baneshwor, Kathmandu",
        CodAmount: "2500.00",
        IsStoreVisit: false,
        CreatedBy: "admin-uuid",
        CreatedAt: "2026-08-20T14:30:00Z",
        UpdatedAt: "2026-08-20T14:30:00Z",
        IsLegacy: true,
      },
    ];

    const normalized = rawProblemOrders.map(normalizeOrder);
    expect(normalized).toHaveLength(1);
    expect(normalized[0].id).toBe("e8574e44-1234-4567-89ab-cdef01234567");
    expect(normalized[0].customer_id).toBe("cust-9999-uuid");
    expect(normalized[0].status).toBe("follow_up");
    expect(normalized[0].source).toBe("facebook");
    expect(normalized[0].is_legacy).toBe(true);

    const displayId = normalized[0].id ? normalized[0].id.slice(0, 8) : "—";
    expect(displayId).toBe("e8574e44");
  });

  it("handles incomplete / legacy order with missing or undefined fields without crashing", () => {
    const incompleteOrders = [
      {},
      { ID: undefined, CustomerID: undefined },
      { id: null, customer_id: null },
      { id: "", customer_id: "" },
    ];

    const normalized = incompleteOrders.map((raw) => normalizeOrder(raw as any));

    normalized.forEach((order) => {
      expect(() => {
        const orderIdDisplay = order.id ? order.id.slice(0, 8) : "—";
        const customerIdDisplay = order.customer_id ? order.customer_id.slice(0, 8) : "—";
        expect(orderIdDisplay).toBe("—");
        expect(customerIdDisplay).toBe("—");
      }).not.toThrow();
    });
  });

  it("renders fully populated normal orders identically to before", () => {
    const normalOrder = normalizeOrder({
      id: "a1b2c3d4-5678-90ab-cdef-112233445566",
      customer_id: "c9d8e7f6-5432-10fe-dcba-665544332211",
      source: "website",
      status: "confirmed",
      address: "Patan Durbar Square",
      cod_amount: "1200.00",
      is_store_visit: false,
      created_by: "staff-1",
      created_at: "2026-08-22T08:00:00Z",
      updated_at: "2026-08-22T08:00:00Z",
      is_legacy: false,
    });

    expect(normalOrder.id ? normalOrder.id.slice(0, 8) : "—").toBe("a1b2c3d4");
    expect(normalOrder.customer_id ? normalOrder.customer_id.slice(0, 8) : "—").toBe("c9d8e7f6");
    expect(normalOrder.source).toBe("website");
    expect(normalOrder.status).toBe("confirmed");
  });
});

describe("Follow-Up Queue and Order Detail Follow-Up History", () => {
  it("normalizes raw SQLC follow-up payload without broken '#' placeholder", () => {
    const rawFollowUp = {
      ID: "b6f5b544-1b64-4b86-bbb9-0bdfc14b9a68",
      OrderID: "5830cf5d-7562-40a4-ac9e-d4084e69600e",
      AttemptNo: 1,
      NextAction: "call_again",
      PreferredDay: "Monday",
      NextActionDate: "2026-08-22",
      Note: "Customer asked to call back in the afternoon",
      AssignedTo: "user-123",
      CreatedAt: "2026-08-22T09:00:00Z",
      Status: "follow_up",
      CustomerID: "cust-456",
      CustomerName: "Hari Bahadur",
      CustomerPhone: "9841234567",
      AssignedToName: "Staff User",
    };

    const fu = normalizeFollowUp(rawFollowUp);
    expect(fu.id).toBe("b6f5b544-1b64-4b86-bbb9-0bdfc14b9a68");
    expect(fu.order_id).toBe("5830cf5d-7562-40a4-ac9e-d4084e69600e");
    expect(fu.customer_name).toBe("Hari Bahadur");
    expect(fu.customer_phone).toBe("9841234567");
    expect(fu.next_action).toBe("call_again");
    expect(fu.attempt_no).toBe(1);
    expect(fu.next_action_date).toBe("2026-08-22");
    expect(fu.assigned_to_name).toBe("Staff User");
    expect(fu.note).toBe("Customer asked to call back in the afternoon");

    const actionDisplay = `${fu.next_action === "call_again" ? "Call again" : fu.next_action || "—"} #${fu.attempt_no}`;
    expect(actionDisplay).toBe("Call again #1");
  });

  it("handles follow-up with missing optional fields safely", () => {
    const incompleteFollowUp = {
      id: "fu-1",
      order_id: "ord-1",
      attempt_no: 2,
      next_action: "no_answer",
    };

    const fu = normalizeFollowUp(incompleteFollowUp);
    expect(fu.id).toBe("fu-1");
    expect(fu.order_id).toBe("ord-1");
    expect(fu.attempt_no).toBe(2);
    expect(fu.next_action).toBe("no_answer");
    expect(fu.customer_name).toBe("");
    expect(fu.customer_phone).toBe("");
    expect(fu.assigned_to_name).toBeNull();
    expect(fu.note).toBeNull();
    expect(fu.next_action_date).toBeNull();

    const actionDisplay = `${fu.next_action === "call_again" ? "Call again" : fu.next_action === "no_answer" ? "No answer" : fu.next_action || "—"} #${fu.attempt_no}`;
    expect(actionDisplay).toBe("No answer #2");
  });

  it("normalizes order detail follow-up history list", () => {
    const rawHistory = [
      {
        id: "fu-2",
        order_id: "ord-100",
        attempt_no: 2,
        next_action: "call_again",
        next_action_date: "2026-08-23",
        note: "Will pick up tomorrow",
        created_at: "2026-08-22T12:00:00Z",
        assigned_to_name: "Admin User",
      },
      {
        id: "fu-1",
        order_id: "ord-100",
        attempt_no: 1,
        next_action: "no_answer",
        next_action_date: null,
        note: "Phone unreachable",
        created_at: "2026-08-21T10:00:00Z",
        assigned_to_name: "Staff Member",
      },
    ];

    const history = rawHistory.map(normalizeFollowUp);
    expect(history).toHaveLength(2);
    expect(history[0].attempt_no).toBe(2);
    expect(history[0].next_action).toBe("call_again");
    expect(history[0].assigned_to_name).toBe("Admin User");
    expect(history[1].attempt_no).toBe(1);
    expect(history[1].next_action).toBe("no_answer");
    expect(history[1].assigned_to_name).toBe("Staff Member");
  });
});

describe("Users management status and action states", () => {
  it("determines correct action labels and availability for mixed user statuses", () => {
    const mockUsers = [
      { id: "u-1", name: "Super User", phone: "9800000001", role: "superadmin", is_active: true, status: "active" },
      { id: "u-2", name: "Active Admin", phone: "9800000002", role: "admin", is_active: true, status: "active" },
      { id: "u-3", name: "Inactive Staff", phone: "9800000003", role: "staff", is_active: false, status: "inactive" },
      { id: "u-4", name: "Invited Staff", phone: "9800000004", role: "staff", is_active: false, status: "invited" },
    ];

    function getUserActions(user: typeof mockUsers[0]): string[] {
      if (user.role === "superadmin") return [];
      if (user.status === "invited") return ["Resend", "Revoke"];
      return [user.is_active ? "Deactivate" : "Activate"];
    }

    expect(getUserActions(mockUsers[0])).toEqual([]);
    expect(getUserActions(mockUsers[1])).toEqual(["Deactivate"]);
    expect(getUserActions(mockUsers[2])).toEqual(["Activate"]);
    expect(getUserActions(mockUsers[3])).toEqual(["Resend", "Revoke"]);
  });
});