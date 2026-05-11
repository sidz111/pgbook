const API_BASE = "/v1";

async function apiFetch(
  path,
  { method = "GET", body = null, formData = false } = {},
) {
  const headers = formData ? {} : { "Content-Type": "application/json" };
  const options = {
    method,
    credentials: "include",
    headers,
  };
  if (body) {
    options.body = formData ? body : JSON.stringify(body);
  }

  const response = await fetch(API_BASE + path, options);
  const payload = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(payload.error || payload.message || "API request failed");
  }
  return payload;
}

function showMessage(id, message, type = "info") {
  const container = document.getElementById(id);
  if (!container) return;
  container.textContent = message;
  container.style.borderLeft =
    type === "error" ? "4px solid #d32f2f" : "4px solid #3751ff";
}

async function logout() {
  try {
    await apiFetch("/auth/logout", { method: "POST" });
  } catch (err) {
    console.warn(err.message);
  }
  window.location.href = "/login";
}

async function initLogin() {
  const form = document.getElementById("loginForm");
  if (!form) return;
  form.addEventListener("submit", async (event) => {
    event.preventDefault();
    const payload = {
      email: form.email.value,
      password: form.password.value,
    };
    try {
      await apiFetch("/auth/login", { method: "POST", body: payload });
      window.location.href = "/dashboard";
    } catch (err) {
      showMessage("loginMessage", err.message, "error");
    }
  });
}

async function initRegister() {
  const form = document.getElementById("registerForm");
  if (!form) return;
  form.addEventListener("submit", async (event) => {
    event.preventDefault();
    const payload = {
      name: form.name.value,
      email: form.email.value,
      password: form.password.value,
      role: form.role.value,
    };
    try {
      await apiFetch("/auth/register", { method: "POST", body: payload });
      showMessage(
        "registerMessage",
        "Registration successful. Redirecting to login...",
      );
      setTimeout(() => (window.location.href = "/login"), 1200);
    } catch (err) {
      showMessage("registerMessage", err.message, "error");
    }
  });
}

async function initDashboard() {
  const info = document.getElementById("userInfo");
  const actions = document.getElementById("dashboardActions");
  const details = document.getElementById("dashboardDetails");
  const message = document.getElementById("dashboardMessage");
  try {
    const user = await apiFetch("/auth/me");
    info.innerHTML = `<p><strong>User ID:</strong> ${user.user_id}</p><p><strong>Role:</strong> ${user.role}</p>`;
    const mapping = {
      admin: "/admin",
      pg_owner: "/owner",
      tenant: "/tenant",
    };
    const rolePath = mapping[user.role] || "/dashboard";
    actions.innerHTML = `<a class="btn btn-primary" href="${rolePath}">Open ${user.role.replace("_", " ")} Console</a>`;
    details.classList.remove("hidden");
    details.innerHTML = `<p>Use the buttons above to load your role-specific interface.</p>`;
  } catch (err) {
    showMessage(
      "dashboardMessage",
      "You must login first. Redirecting to login...",
      "error",
    );
    setTimeout(() => (window.location.href = "/login"), 1200);
  }
}

async function initOwner() {
  const message = document.getElementById("ownerMessage");
  let currentPGId = null;

  async function loadOwnerSummary() {
    try {
      const pg = await apiFetch("/pg/my-pg");
      currentPGId = pg.id;
      document.getElementById("pgInfo").innerHTML = `
        <p><strong>Name:</strong> ${pg.name}</p>
        <p><strong>Owner:</strong> ${pg.owner_name}</p>
        <p><strong>Phone:</strong> ${pg.phone}</p>
        <p><strong>Address:</strong> ${pg.address}</p>
      `;
      await Promise.all([
        loadRooms(),
        loadTenants(),
        loadTenantRequests(),
        loadPendingPayments(),
        loadStats(),
        loadSubscription(),
        loadQRCodes(currentPGId, "qrCodesInfo"),
      ]);
    } catch (err) {
      showMessage("ownerMessage", err.message, "error");
      document.getElementById("pgInfo").innerHTML =
        "<p>PG not found yet. Use the form below to create your PG.</p>";
      currentPGId = null;
    }
  }

  async function loadRooms() {
    if (!currentPGId) return;
    try {
      const data = await apiFetch(`/pg/${currentPGId}/rooms`);
      const rooms = data.rooms || data || [];
      document.getElementById("roomList").innerHTML = rooms.length
        ? rooms
            .map(
              (room) =>
                `<div class="item-card"><strong>${room.room_number}</strong> • ${room.sharing_type} • Rent ${room.rent_amount}</div>`,
            )
            .join("")
        : "<p>No rooms created yet.</p>";
    } catch (err) {
      document.getElementById("roomList").innerHTML = `<p>${err.message}</p>`;
    }
  }

  async function loadTenants() {
    if (!currentPGId) return;
    try {
      const data = await apiFetch(`/pg/${currentPGId}/tenants`);
      const tenants = data.tenants || data || [];
      document.getElementById("tenantList").innerHTML = tenants.length
        ? tenants
            .map(
              (t) =>
                `<div class="item-card"><strong>${t.first_name} ${t.last_name}</strong> • ${t.phone} • Room ${t.room_id}</div>`,
            )
            .join("")
        : "<p>No tenants registered yet.</p>";
    } catch (err) {
      document.getElementById("tenantList").innerHTML = `<p>${err.message}</p>`;
    }
  }

  async function loadTenantRequests() {
    if (!currentPGId) return;
    try {
      const data = await apiFetch(`/pg/${currentPGId}/tenants`);
      const tenants = data.tenants || data || [];
      const pendingRequests = tenants.filter(
        (t) => t.status === "pending_approval",
      );
      document.getElementById("tenantRequests").innerHTML =
        pendingRequests.length
          ? pendingRequests
              .map(
                (t) => `
                <div class="item-card">
                  <strong>${t.first_name} ${t.last_name}</strong> • ${t.phone}
                  <form class="approve-form" data-tenant-id="${t.id}">
                    <select name="room_id" required>
                      <option value="">Select Room</option>
                    </select>
                    <button type="submit" class="btn btn-primary">Approve</button>
                  </form>
                </div>
              `,
              )
              .join("")
          : "<p>No pending tenant requests.</p>";

      // Load available rooms for approval forms
      const roomsData = await apiFetch(`/pg/${currentPGId}/rooms`);
      const rooms = roomsData.rooms || roomsData || [];
      const availableRooms = rooms.filter(
        (r) => (r.occupied || 0) < (r.capacity || 0),
      );
      document
        .querySelectorAll('.approve-form select[name="room_id"]')
        .forEach((select) => {
          select.innerHTML =
            '<option value="">Select Room</option>' +
            availableRooms
              .map(
                (r) =>
                  `<option value="${r.id}">${r.room_number} (${r.sharing_type})</option>`,
              )
              .join("");
        });

      // Handle approval form submissions
      document.querySelectorAll(".approve-form").forEach((form) => {
        form.addEventListener("submit", async (event) => {
          event.preventDefault();
          const tenantId = form.dataset.tenantId;
          const roomId = form.room_id.value;
          try {
            await apiFetch(`/pg/${currentPGId}/tenants/${tenantId}/approve`, {
              method: "POST",
              body: { room_id: roomId },
            });
            showMessage("ownerMessage", "Tenant approved successfully");
            await loadTenantRequests();
            await loadTenants();
            await loadRooms();
          } catch (err) {
            showMessage("ownerMessage", err.message, "error");
          }
        });
      });

      window.rejectTenant = async (tenantId) => {
        try {
          await apiFetch(`/pg/${currentPGId}/tenants/${tenantId}/reject`, {
            method: "POST",
          });
          showMessage("ownerMessage", "Tenant request rejected.");
          await loadTenantRequests();
        } catch (err) {
          showMessage("ownerMessage", err.message, "error");
        }
      };
    } catch (err) {
      document.getElementById("tenantRequests").innerHTML =
        `<p>${err.message}</p>`;
    }
  }

  async function loadPendingPayments() {
    if (!currentPGId) return;
    try {
      const data = await apiFetch(`/pg/${currentPGId}/payments/pending`);
      const payments = data?.pending_payments || [];
      document.getElementById("pendingPayments").innerHTML = payments.length
        ? payments
            .map(
              (p) =>
                `<div class="item-card"><strong>${p.id}</strong> • ${p.amount} ${p.method} • ${p.status}</div>`,
            )
            .join("")
        : "<p>No pending payments.</p>";
    } catch (err) {
      document.getElementById("pendingPayments").innerHTML =
        `<p>${err.message}</p>`;
    }
  }

  async function loadStats() {
    if (!currentPGId) return;
    try {
      const stats = await apiFetch(`/pg/${currentPGId}/payments/stats`);
      document.getElementById("statsInfo").innerHTML = `
        <p><strong>Total Payments:</strong> ${stats.total_payments || 0}</p>
        <p><strong>Total Collected:</strong> ${stats.total_collected || 0}</p>
      `;
    } catch (err) {
      document.getElementById("statsInfo").innerHTML = `<p>${err.message}</p>`;
    }
  }

  async function loadSubscription() {
    if (!currentPGId) return;
    try {
      const subscription = await apiFetch(
        `/pg/${currentPGId}/subscription/active`,
      );
      if (subscription && subscription.status === "active") {
        document.getElementById("subscriptionInfo").innerHTML = `
          <p><strong>Plan:</strong> ${subscription.plan_name || "Monthly"}</p>
          <p><strong>Amount:</strong> ${subscription.amount || 0}</p>
          <p><strong>Expiry:</strong> ${subscription.expiry_date || "N/A"}</p>
        `;
      } else {
        document.getElementById("subscriptionInfo").innerHTML =
          "<p>No active subscription.</p>";
      }
    } catch (err) {
      document.getElementById("subscriptionInfo").innerHTML =
        `<p>${err.message}</p>`;
    }
  }

  async function loadQRCodes(pgId, containerId) {
    if (!pgId) return;
    const container = document.getElementById(containerId);
    if (!container) return;

    try {
      const qr = await apiFetch(`/pg/${pgId}/payments/qr-codes`);
      const ownerQRCode = qr.owner_qr_url
        ? `<img class="qr-image" src="${qr.owner_qr_url}" alt="Owner QR Code" />`
        : `<p>No owner QR code uploaded yet.</p>`;
      const adminQRCode = qr.admin_qr_url
        ? `<img class="qr-image" src="${qr.admin_qr_url}" alt="Admin QR Code" />`
        : `<p>No admin QR code uploaded yet.</p>`;

      container.innerHTML = `
        <div class="qr-section">
          <h3>Owner Payment QR</h3>
          ${ownerQRCode}
        </div>
        <div class="qr-section">
          <h3>Admin Subscription QR</h3>
          ${adminQRCode}
        </div>
      `;
    } catch (err) {
      container.innerHTML = `<p>${err.message}</p>`;
    }
  }

  const subscriptionForm = document.getElementById("createSubscriptionForm");
  subscriptionForm?.addEventListener("submit", async (event) => {
    event.preventDefault();
    if (!currentPGId) return;

    const proofFile = document.getElementById("proofFile")?.files?.[0];
    let body;
    let formData = false;

    if (proofFile) {
      body = new FormData();
      body.append("pg_id", currentPGId);
      body.append("amount", event.target.amount.value);
      body.append("plan_name", event.target.plan_name.value || "Monthly");
      body.append("proof", proofFile);
      if (event.target.proof_url.value) {
        body.append("proof_url", event.target.proof_url.value);
      }
      formData = true;
    } else {
      body = {
        pg_id: currentPGId,
        amount: Number(event.target.amount.value),
        proof_url: event.target.proof_url.value,
        plan_name: event.target.plan_name.value || "Monthly",
      };
    }

    try {
      await apiFetch(`/pg/${currentPGId}/subscription`, {
        method: "POST",
        body,
        formData,
      });
      showMessage("ownerMessage", "Subscription requested successfully.");
      subscriptionForm.reset();
      await loadSubscription();
    } catch (err) {
      showMessage("ownerMessage", err.message, "error");
    }
  });

  document.getElementById("logoutButton").addEventListener("click", logout);
  await loadOwnerSummary();
}

async function initPGManage() {
  const message = document.getElementById("pgMessage");
  const form = document.getElementById("createPGForm");
  let currentPG = null;

  async function loadPG() {
    try {
      const pg = await apiFetch("/pg/my-pg");
      currentPG = pg;
      form.name.value = pg.name || "";
      form.owner_name.value = pg.owner_name || "";
      form.phone.value = pg.phone || "";
      form.address.value = pg.address || "";
      form.querySelector("button[type=submit]").textContent = pg.id
        ? "Update PG"
        : "Save PG";
      message.textContent = "";
    } catch (err) {
      currentPG = null;
      if (form) {
        form.reset();
        form.querySelector("button[type=submit]").textContent = "Create PG";
      }
      showMessage(
        "pgMessage",
        "No existing PG found. Fill the form to create one.",
      );
    }
  }

  form?.addEventListener("submit", async (event) => {
    event.preventDefault();
    const payload = {
      name: form.name.value,
      owner_name: form.owner_name.value,
      phone: form.phone.value,
      address: form.address.value,
    };
    const path =
      currentPG && currentPG.id ? `/pg/${currentPG.id}` : "/pg/create";
    const method = currentPG && currentPG.id ? "PUT" : "POST";
    try {
      await apiFetch(path, { method, body: payload });
      showMessage("pgMessage", "PG saved successfully.");
      await loadPG();
    } catch (err) {
      showMessage("pgMessage", err.message, "error");
    }
  });

  document.getElementById("logoutButton").addEventListener("click", logout);
  await loadPG();
}

async function initRoomManage() {
  const message = document.getElementById("roomMessage");
  const stats = document.getElementById("roomStats");
  const roomList = document.getElementById("roomList");
  const form = document.getElementById("createRoomForm");
  let currentPGId = null;

  async function loadRoomStats() {
    if (!currentPGId) {
      stats.innerHTML = "<p>Create a PG first to manage rooms.</p>";
      roomList.innerHTML = "";
      return;
    }

    try {
      const data = await apiFetch(`/pg/${currentPGId}/rooms`);
      const rooms = data.rooms || data || [];
      const totalRooms = rooms.length;
      const totalBeds = rooms.reduce(
        (sum, room) => sum + Number(room.capacity || 0),
        0,
      );
      const occupiedBeds = rooms.reduce(
        (sum, room) => sum + Number(room.occupied || 0),
        0,
      );
      const vacancies = Math.max(totalBeds - occupiedBeds, 0);

      stats.innerHTML = `
        <p><strong>Total Rooms:</strong> ${totalRooms}</p>
        <p><strong>Total Bed Count:</strong> ${totalBeds}</p>
        <p><strong>Vacancies:</strong> ${vacancies}</p>
      `;
      roomList.innerHTML = rooms.length
        ? rooms
            .map(
              (room) =>
                `<div class="item-card"><strong>${room.room_number}</strong> • ${room.sharing_type} • Capacity ${room.capacity} • Occupied ${room.occupied || 0}</div>`,
            )
            .join("")
        : "<p>No rooms created yet.</p>";
    } catch (err) {
      stats.innerHTML = `<p>${err.message}</p>`;
      roomList.innerHTML = "";
    }
  }

  async function loadPG() {
    try {
      const pg = await apiFetch("/pg/my-pg");
      currentPGId = pg.id;
      message.textContent = "";
      await loadRoomStats();
    } catch (err) {
      currentPGId = null;
      showMessage("roomMessage", "No PG found. Create a PG first.", "error");
      await loadRoomStats();
    }
  }

  form?.addEventListener("submit", async (event) => {
    event.preventDefault();
    if (!currentPGId) {
      showMessage(
        "roomMessage",
        "Create a PG first before adding rooms.",
        "error",
      );
      return;
    }
    const payload = {
      room_number: form.room_number.value,
      capacity: Number(form.capacity.value),
      rent_amount: Number(form.rent_amount.value),
      sharing_type: form.sharing_type.value,
    };
    try {
      await apiFetch(`/pg/${currentPGId}/rooms/create`, {
        method: "POST",
        body: payload,
      });
      showMessage("roomMessage", "Room added successfully.");
      form.reset();
      await loadRoomStats();
    } catch (err) {
      showMessage("roomMessage", err.message, "error");
    }
  });

  document.getElementById("logoutButton").addEventListener("click", logout);
  await loadPG();
}

async function initTenantRequestsPage() {
  const requestMessage = document.getElementById("requestMessage");
  const tenantRequests = document.getElementById("tenantRequests");
  const allTenants = document.getElementById("allTenants");
  let currentPGId = null;

  async function loadRequests() {
    if (!currentPGId) return;
    try {
      const data = await apiFetch(`/pg/${currentPGId}/tenants`);
      const tenants = data.tenants || data || [];
      const pendingRequests = tenants.filter(
        (t) => t.status === "pending_approval",
      );
      tenantRequests.innerHTML = pendingRequests.length
        ? pendingRequests
            .map(
              (t) =>
                `<div class="item-card"><strong>${t.first_name} ${t.last_name}</strong> • ${t.phone} • Status: ${t.status || "pending"}</div>`,
            )
            .join("")
        : "<p>No pending tenant requests.</p>";
      allTenants.innerHTML = tenants.length
        ? tenants
            .filter((t) => t.status !== "pending_approval")
            .map(
              (t) =>
                `<div class="item-card"><strong>${t.first_name} ${t.last_name}</strong> • ${t.phone} • Status: ${t.status || "approved"}</div>`,
            )
            .join("")
        : "<p>No approved tenants yet.</p>";
    } catch (err) {
      requestMessage.textContent = err.message;
      tenantRequests.innerHTML = "";
      allTenants.innerHTML = "";
    }
  }

  async function loadPG() {
    try {
      const pg = await apiFetch("/pg/my-pg");
      currentPGId = pg.id;
      requestMessage.textContent = "";
      await loadRequests();
    } catch (err) {
      currentPGId = null;
      requestMessage.textContent = "No PG found. Please create a PG first.";
      tenantRequests.innerHTML = "";
      allTenants.innerHTML = "";
    }
  }

  document.getElementById("logoutButton").addEventListener("click", logout);
  await loadPG();
}

async function initTenant() {
  // Load available PGs for the tenant registration form
  async function loadAvailablePGs() {
    try {
      const data = await apiFetch("/tenant/pgs/available");
      const pgSelect = document.getElementById("pgId");
      if (pgSelect && data.pgs) {
        // Clear existing options except the first placeholder
        while (pgSelect.options.length > 1) {
          pgSelect.remove(1);
        }
        // Add available PGs as options
        data.pgs.forEach((pg) => {
          const option = document.createElement("option");
          option.value = pg.id;
          option.textContent = `${pg.name} - ${pg.address} (Owner: ${pg.owner_name})`;
          pgSelect.appendChild(option);
        });
      }
    } catch (err) {
      console.error("Failed to load available PGs:", err.message);
    }
  }

  // Handle tenant self-registration
  const registerForm = document.getElementById("tenantRegisterForm");
  const tenantInfo = document.getElementById("tenantInfo");
  const tenantPayments = document.getElementById("tenantPayments");
  const tenantUpdateSection = document.getElementById("tenantUpdateSection");

  async function loadCurrentTenant() {
    try {
      const tenant = await apiFetch("/tenant/me");
      if (registerForm) {
        registerForm.style.display = "none";
      }
      if (tenantUpdateSection) {
        tenantUpdateSection.style.display = "block";
      }
      tenantInfo.innerHTML = `
        <p><strong>${tenant.first_name} ${tenant.last_name}</strong></p>
        <p><strong>Phone:</strong> ${tenant.phone}</p>
        <p><strong>Status:</strong> ${tenant.status || "pending"}</p>
        <p><strong>PG:</strong> ${tenant.pg_id || "N/A"}</p>
        <p><strong>Room:</strong> ${tenant.room_id || "not assigned"}</p>
        <p><strong>Joining Date:</strong> ${tenant.joining_date || "N/A"}</p>
      `;
      await loadQRCodes(tenant.pg_id, "tenantQrCodes");
      const payments = await apiFetch(`/tenant/${tenant.id}/payments`);
      tenantPayments.innerHTML = payments.length
        ? payments
            .map(
              (p) =>
                `<div class="item-card"><strong>${p.month || p.method}</strong> • ${p.amount} • ${p.status}</div>`,
            )
            .join("")
        : "<p>No payments found.</p>";
    } catch (err) {
      if (registerForm) {
        registerForm.style.display = "block";
      }
      if (tenantUpdateSection) {
        tenantUpdateSection.style.display = "none";
      }
    }
  }

  if (registerForm) {
    loadAvailablePGs();

    registerForm.addEventListener("submit", async (event) => {
      event.preventDefault();
      const file = document.getElementById("idProof")?.files?.[0];
      let body;
      let formData = false;

      if (file) {
        body = new FormData();
        body.append("pg_id", registerForm.pg_id.value);
        body.append("first_name", registerForm.first_name.value);
        body.append("last_name", registerForm.last_name.value);
        body.append("phone", registerForm.phone.value);
        body.append("joining_date", registerForm.joining_date.value);
        body.append("id_proof_type", registerForm.id_proof_type.value);
        body.append("id_proof", file);
        formData = true;
      } else {
        body = {
          pg_id: registerForm.pg_id.value,
          first_name: registerForm.first_name.value,
          last_name: registerForm.last_name.value,
          phone: registerForm.phone.value,
          joining_date: registerForm.joining_date.value,
          id_proof_type: registerForm.id_proof_type.value,
        };
      }

      try {
        const result = await apiFetch("/tenant/self-register", {
          method: "POST",
          body,
          formData,
        });
        showMessage(
          "tenantMessage",
          result.message || "Registration submitted successfully",
        );
        registerForm.reset();
        await loadCurrentTenant();
      } catch (err) {
        showMessage("tenantMessage", err.message, "error");
      }
    });
  }

  await loadCurrentTenant();

  const tenantForm = document.getElementById("tenantLookupForm");
  const message = document.getElementById("tenantMessage");
  let currentTenantId = null;

  function setTenantContext(id) {
    currentTenantId = id;
    tenantInfo.innerHTML = "<p>Loading tenant details...</p>";
    tenantPayments.innerHTML = "";
    message.textContent = "";
  }

  async function loadTenantDetails() {
    if (!currentTenantId) return;
    try {
      const tenant = await apiFetch(`/tenant/${currentTenantId}`);
      tenantInfo.innerHTML = `
        <p><strong>${tenant.first_name} ${tenant.last_name}</strong></p>
        <p><strong>Phone:</strong> ${tenant.phone}</p>
        <p><strong>Room:</strong> ${tenant.room_id}</p>
        <p><strong>Status:</strong> ${tenant.status || "unknown"}</p>
      `;
      await loadQRCodes(tenant.pg_id, "tenantQrCodes");
      const payments = await apiFetch(`/tenant/${currentTenantId}/payments`);
      tenantPayments.innerHTML = payments.length
        ? payments
            .map(
              (p) =>
                `<div class="item-card"><strong>${p.month || p.method}</strong> • ${p.amount} • ${p.status}</div>`,
            )
            .join("")
        : "<p>No payments found.</p>";
    } catch (err) {
      showMessage("tenantMessage", err.message, "error");
    }
  }

  tenantForm?.addEventListener("submit", async (event) => {
    event.preventDefault();
    const tenantId = event.target.tenant_id.value.trim();
    if (!tenantId) {
      showMessage("tenantMessage", "Please provide a tenant ID.", "error");
      return;
    }
    setTenantContext(tenantId);
    await loadTenantDetails();
  });

  document
    .getElementById("sendNoticeBtn")
    ?.addEventListener("click", async () => {
      if (!currentTenantId)
        return showMessage("tenantMessage", "Load a tenant first.", "error");
      try {
        await apiFetch(`/tenant/${currentTenantId}/notice`, {
          method: "POST",
          body: { notice_period_days: 30 },
        });
        showMessage("tenantMessage", "Notice sent successfully.");
      } catch (err) {
        showMessage("tenantMessage", err.message, "error");
      }
    });

  document
    .getElementById("cancelNoticeBtn")
    ?.addEventListener("click", async () => {
      if (!currentTenantId)
        return showMessage("tenantMessage", "Load a tenant first.", "error");
      try {
        await apiFetch(`/tenant/${currentTenantId}/cancel-notice`, {
          method: "POST",
        });
        showMessage("tenantMessage", "Notice cancelled successfully.");
      } catch (err) {
        showMessage("tenantMessage", err.message, "error");
      }
    });

  const uploadPhotoForm = document.getElementById("uploadPhotoForm");
  const uploadIdForm = document.getElementById("uploadIdForm");

  uploadPhotoForm?.addEventListener("submit", async (event) => {
    event.preventDefault();
    if (!currentTenantId)
      return showMessage("tenantMessage", "Load a tenant first.", "error");
    const file = document.getElementById("photoFile").files[0];
    if (!file)
      return showMessage("tenantMessage", "Select a photo first.", "error");
    const formData = new FormData();
    formData.append("photo", file);
    try {
      await apiFetch(`/tenant/${currentTenantId}/upload-photo`, {
        method: "POST",
        body: formData,
        formData: true,
      });
      showMessage("tenantMessage", "Profile photo uploaded.");
    } catch (err) {
      showMessage("tenantMessage", err.message, "error");
    }
  });

  uploadIdForm?.addEventListener("submit", async (event) => {
    event.preventDefault();
    if (!currentTenantId)
      return showMessage("tenantMessage", "Load a tenant first.", "error");
    const file = document.getElementById("idFile").files[0];
    if (!file)
      return showMessage("tenantMessage", "Select a file first.", "error");
    const formData = new FormData();
    formData.append("document", file);
    try {
      await apiFetch(`/tenant/${currentTenantId}/upload-id-proof`, {
        method: "POST",
        body: formData,
        formData: true,
      });
      showMessage("tenantMessage", "ID proof uploaded successfully.");
    } catch (err) {
      showMessage("tenantMessage", err.message, "error");
    }
  });

  document.getElementById("logoutButton")?.addEventListener("click", logout);
}

async function initTenantProfile() {
  const tenantInfo = document.getElementById("tenantInfo");
  const tenantPayments = document.getElementById("tenantPayments");
  const message = document.getElementById("tenantMessage");
  const updateForm = document.getElementById("tenantProfileForm");

  async function loadProfile() {
    try {
      const tenant = await apiFetch("/tenant/me");
      tenantInfo.innerHTML = `
        <p><strong>${tenant.first_name} ${tenant.last_name}</strong></p>
        <p><strong>Phone:</strong> ${tenant.phone}</p>
        <p><strong>Status:</strong> ${tenant.status || "pending"}</p>
        <p><strong>PG ID:</strong> ${tenant.pg_id || "N/A"}</p>
        <p><strong>Room ID:</strong> ${tenant.room_id || "not assigned"}</p>
        <p><strong>Joining Date:</strong> ${tenant.joining_date || "N/A"}</p>
      `;
      await loadQRCodes(tenant.pg_id, "qrCodesInfo");
      const payments = await apiFetch(`/tenant/${tenant.id}/payments`);
      tenantPayments.innerHTML = payments.length
        ? payments
            .map(
              (p) =>
                `<div class="item-card"><strong>${p.month || p.method}</strong> • ${p.amount} • ${p.status}</div>`,
            )
            .join("")
        : "<p>No payments found.</p>";
    } catch (err) {
      message.textContent = err.message;
    }
  }

  updateForm?.addEventListener("submit", async (event) => {
    event.preventDefault();
    const payload = {
      first_name: updateForm.first_name.value,
      last_name: updateForm.last_name.value,
      phone: updateForm.phone.value,
      id_proof_type: updateForm.id_proof_type.value,
    };
    try {
      await apiFetch("/tenant/me", { method: "PUT", body: payload });
      showMessage("tenantMessage", "Profile updated successfully.");
      await loadProfile();
    } catch (err) {
      showMessage("tenantMessage", err.message, "error");
    }
  });

  document.getElementById("logoutButton")?.addEventListener("click", logout);
  await loadProfile();
}

async function initAdmin() {
  const message = document.getElementById("adminMessage");

  async function loadPGs() {
    try {
      const data = await apiFetch("/pg/list");
      const pgs = data.pgs || [];
      document.getElementById("pgList").innerHTML = pgs.length
        ? pgs
            .map(
              (pg) =>
                `<div class="item-card"><strong>${pg.name}</strong> • ${pg.address} • Owner ${pg.owner_name}</div>`,
            )
            .join("")
        : "<p>No PGs available.</p>";
    } catch (err) {
      document.getElementById("pgList").innerHTML = `<p>${err.message}</p>`;
    }
  }

  async function loadPendingSubscriptions() {
    try {
      const data = await apiFetch("/subscription/pending");
      const subscriptions = data.subscriptions || data || [];
      document.getElementById("pendingSubscriptions").innerHTML =
        subscriptions.length
          ? subscriptions
              .map(
                (sub) => `
          <div class="item-card">
            <p><strong>${sub.id}</strong> • PG ${sub.pg_id}</p>
            <p>Plan: ${sub.plan_name || "N/A"} • Amount: ${sub.amount || 0}</p>
            <p>Status: ${sub.status || "pending"}</p>
            <p>Start: ${sub.start_date || "unknown"} • Expiry: ${sub.expiry_date || "pending"}</p>
            <div class="button-row">
              <button class="btn btn-primary" onclick="approveSubscription('${sub.id}')">Approve</button>
              <button class="btn btn-secondary" onclick="rejectSubscription('${sub.id}')">Reject</button>
            </div>
          </div>
        `,
              )
              .join("")
          : "<p>No pending subscriptions.</p>";
    } catch (err) {
      document.getElementById("pendingSubscriptions").innerHTML =
        `<p>${err.message}</p>`;
    }
  }

  window.approveSubscription = async (subId) => {
    try {
      await apiFetch(`/subscription/${subId}/approve`, {
        method: "POST",
        body: { months: 1 },
      });
      showMessage("adminMessage", "Subscription approved.");
      await loadPendingSubscriptions();
    } catch (err) {
      showMessage("adminMessage", err.message, "error");
    }
  };

  window.rejectSubscription = async (subId) => {
    try {
      await apiFetch(`/subscription/${subId}/reject`, { method: "POST" });
      showMessage("adminMessage", "Subscription rejected.");
      await loadPendingSubscriptions();
    } catch (err) {
      showMessage("adminMessage", err.message, "error");
    }
  };

  document.getElementById("logoutButton")?.addEventListener("click", logout);
  await Promise.all([loadPGs(), loadPendingSubscriptions()]);
}

function initPage() {
  const path = window.location.pathname;
  if (path === "/login") return initLogin();
  if (path === "/register") return initRegister();
  if (path === "/dashboard") return initDashboard();
  if (path === "/owner") return initOwner();
  if (path === "/pg-manage") return initPGManage();
  if (path === "/room-manage") return initRoomManage();
  if (path === "/tenant-requests") return initTenantRequestsPage();
  if (path === "/tenant") return initTenant();
  if (path === "/admin") return initAdmin();
  document.querySelectorAll("#logoutButton").forEach((button) => {
    button.addEventListener("click", logout);
  });
}

initPage();
