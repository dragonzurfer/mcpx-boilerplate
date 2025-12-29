const state = {
  plans: [],
  selectedPlan: null,
  config: null
};

const planGrid = document.getElementById("plan-grid");
const checkoutBtn = document.getElementById("checkout-btn");
const checkoutStatus = document.getElementById("checkout-status");
const tokenInput = document.getElementById("token");
const usageOutput = document.getElementById("usage-output");
const loadUsageBtn = document.getElementById("load-usage");

const setStatus = (msg) => {
  if (checkoutStatus) {
    checkoutStatus.textContent = msg || "";
  }
};

const getToken = () => (tokenInput ? tokenInput.value.trim() : "");

const apiFetch = async (path, options = {}) => {
  const headers = options.headers || {};
  return fetch(path, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...headers
    }
  });
};

const renderPlans = () => {
  if (!planGrid) return;
  planGrid.innerHTML = "";
  state.plans.forEach((plan) => {
    const card = document.createElement("div");
    card.className = "plan-card" + (state.selectedPlan?.code === plan.code ? " selected" : "");
    card.innerHTML = `
      <h3>${plan.name}</h3>
      <div class="plan-price">₹${plan.priceInr}</div>
      <div class="plan-meta">${plan.interval} · ${plan.type.replace("_", " ")}</div>
      <div class="plan-meta">Quotas: ${Object.entries(plan.quotas || {})
        .map(([k, v]) => `${k}: ${v}`)
        .join(", ")}</div>
    `;
    card.addEventListener("click", () => {
      state.selectedPlan = plan;
      renderPlans();
    });
    planGrid.appendChild(card);
  });
};

const loadPlans = async () => {
  try {
    const res = await apiFetch("/api/billing/plans");
    const data = await res.json();
    state.plans = data.plans || [];
    state.selectedPlan = state.plans.find((p) => p.mostPopular) || state.plans[0];
    renderPlans();
  } catch (err) {
    setStatus("Failed to load plans.");
  }
};

const loadConfig = async () => {
  try {
    const res = await apiFetch("/config");
    state.config = await res.json();
  } catch (err) {
    state.config = null;
  }
};

const openRazorpay = (payload, token) => {
  if (!state.config?.razorpayKeyId) {
    setStatus("Missing Razorpay key. Set RAZORPAY_KEY_ID.");
    return;
  }
  const options = {
    key: state.config.razorpayKeyId,
    name: state.config.appName || "Mcpx App",
    description: payload.plan?.name || "Subscription",
    amount: payload.amount,
    currency: payload.currency || "INR",
    order_id: payload.type === "order" ? payload.orderId : undefined,
    subscription_id: payload.type === "subscription" ? payload.subscriptionId : undefined,
    handler: async (response) => {
      try {
        const verifyPayload = {
          razorpayOrderId: response.razorpay_order_id,
          razorpaySubscriptionId: response.razorpay_subscription_id,
          razorpayPaymentId: response.razorpay_payment_id,
          razorpaySignature: response.razorpay_signature
        };
        const verifyRes = await apiFetch("/api/billing/verify", {
          method: "POST",
          headers: { Authorization: `Bearer ${token}` },
          body: JSON.stringify(verifyPayload)
        });
        const verifyData = await verifyRes.json();
        if (!verifyRes.ok) {
          throw new Error(verifyData.error || "Verification failed");
        }
        setStatus("Payment verified. Access updated.");
      } catch (err) {
        setStatus(err.message || "Verification failed");
      }
    }
  };
  const rzp = new window.Razorpay(options);
  rzp.open();
};

const startCheckout = async () => {
  const token = getToken();
  if (!token) {
    setStatus("Paste a JWT from your app first.");
    return;
  }
  if (!state.selectedPlan) {
    setStatus("Select a plan first.");
    return;
  }
  setStatus("Preparing checkout...");
  try {
    const res = await apiFetch("/api/billing/checkout", {
      method: "POST",
      headers: { Authorization: `Bearer ${token}` },
      body: JSON.stringify({ plan: state.selectedPlan.code })
    });
    const data = await res.json();
    if (!res.ok) {
      throw new Error(data.error || "Checkout failed");
    }
    openRazorpay(data, token);
  } catch (err) {
    setStatus(err.message || "Checkout failed");
  }
};

const loadUsage = async () => {
  const token = getToken();
  if (!token) {
    usageOutput.textContent = "Paste a JWT first.";
    return;
  }
  usageOutput.textContent = "Loading...";
  try {
    const res = await apiFetch("/api/billing/me", {
      headers: { Authorization: `Bearer ${token}` }
    });
    const data = await res.json();
    if (!res.ok) {
      throw new Error(data.error || "Failed to load usage");
    }
    usageOutput.textContent = JSON.stringify(data, null, 2);
  } catch (err) {
    usageOutput.textContent = err.message || "Failed to load usage";
  }
};

const revealEls = Array.from(document.querySelectorAll("[data-reveal]"));
revealEls.forEach((el, idx) => {
  setTimeout(() => {
    el.classList.add("reveal-in");
  }, 120 * idx);
});

if (checkoutBtn) {
  checkoutBtn.addEventListener("click", startCheckout);
}
if (loadUsageBtn) {
  loadUsageBtn.addEventListener("click", loadUsage);
}

loadConfig();
loadPlans();
