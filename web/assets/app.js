const state = {
  config: null,
  token: null,
  user: null,
  plans: [],
  anonId: null,
  post: null,
  entitlement: null
};

const selectors = {
  page: () => document.querySelector("[data-page]"),
  postsGrid: () => document.getElementById("posts-grid"),
  coursesGrid: () => document.getElementById("courses-grid"),
  pricingCards: () => document.getElementById("pricing-cards"),
  postBody: () => document.getElementById("post-body"),
  courseBody: () => document.getElementById("course-body"),
  topicChips: () => document.getElementById("topic-chips"),
  navLogin: () => document.getElementById("nav-login"),
  ctaLogin: () => document.getElementById("cta-login")
};

const API = {
  config: "/config",
  authLogin: "/api/auth/login",
  posts: "/api/posts",
  courses: "/api/courses",
  promos: "/api/promos",
  events: "/api/events/batch",
  plans: "/api/plans",
  createOrder: "/api/payments/create-order",
  confirm: "/api/payments/confirm",
  me: "/api/me"
};

const init = async () => {
  state.token = getToken();
  state.user = getStoredUser();
  state.entitlement = getStoredEntitlement();
  state.anonId = getOrCreateAnonId();
  state.config = await fetchJSON(API.config);

  updateNavState();
  await loadUser();
  updateNavState();
  initNavActions();
  highlightNav();
  initLoginButtons();
  initSearchOverlay();
  initShareModal();

  const page = selectors.page()?.dataset.page;
  if (page === "home") {
    await renderHome();
  }
  if (page === "post") {
    await renderPost();
  }
  if (page === "pricing") {
    await renderPricing();
  }
  if (page === "courses") {
    await renderCourses();
  }
  if (page === "course") {
    await renderCourse();
  }
  if (page === "admin-analytics") {
    await renderAdminAnalytics();
  }
  if (page === "account") {
    await renderAccount();
  }
  if (page === "admin-dashboard") {
    await renderAdminDashboard();
  }
  if (page === "admin-posts") {
    await renderAdminPosts();
  }
  if (page === "admin-funnel") {
    await renderAdminFunnel();
  }
  if (page === "admin-promos") {
    await renderAdminPromos();
  }
  if (page === "admin-post-analytics") {
    await renderAdminPostAnalytics();
  }
  if (page === "admin-users") {
    await renderAdminUsers();
  }
  if (page === "admin-settings") {
    await renderAdminSettings();
  }
};

const fetchJSON = async (url, options = {}) => {
  const headers = options.headers || {};
  if (state.token) {
    headers.Authorization = `Bearer ${state.token}`;
  }
  const res = await fetch(url, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...headers
    }
  });
  if (!res.ok) {
    let message = `Request failed: ${res.status}`;
    try {
      const data = await res.json();
      if (data?.error) {
        message = data.error;
      }
    } catch (err) {
      // ignore parse errors
    }
    const error = new Error(message);
    error.status = res.status;
    throw error;
  }
  return res.json();
};

const loadUser = async () => {
  if (!state.token) return;
  try {
    const data = await fetchJSON(API.me);
    state.user = data.user || null;
    state.entitlement = data.entitlement || null;
    setStoredUser(state.user);
    setStoredEntitlement(state.entitlement);
  } catch (err) {
    clearToken();
  }
};

const listingAccessLevels = () => {
  if (state.user) {
    return "PUBLIC,TRIAL,PAID";
  }
  return "PUBLIC";
};

const authHeader = () => {
  if (!state.token) return {};
  return { Authorization: `Bearer ${state.token}` };
};

const renderHome = async () => {
  const grid = selectors.postsGrid();
  if (!grid) return;

  const accessLevels = listingAccessLevels();
  const data = await fetchJSON(`${API.posts}?access_level=${encodeURIComponent(accessLevels)}`, {
    headers: authHeader()
  });
  const items = data.items || [];

  grid.innerHTML = items.map(renderPostCard).join("");
  animateIn(grid.children);
};

const renderCourses = async () => {
  const grid = selectors.coursesGrid();
  if (!grid) return;

  const accessLevels = listingAccessLevels();
  const data = await fetchJSON(`${API.courses}?access_level=${encodeURIComponent(accessLevels)}`, {
    headers: authHeader()
  });
  const items = data.items || [];

  grid.innerHTML = items.map(renderCourseCard).join("");
  animateIn(grid.children);
};

const renderPost = async () => {
  const page = selectors.page();
  if (!page) return;

  const slug = page.dataset.postSlug;
  const data = await fetchJSON(`${API.posts}/${slug}`, {
    headers: state.token ? { Authorization: `Bearer ${state.token}` } : {}
  });
  state.post = data.post;

  const body = selectors.postBody();
  if (body) {
    body.innerHTML = data.html || "";
  }

  if (data.is_locked && data.gate?.type === "LOGIN_REQUIRED") {
    showLoginGate();
  }
  if (data.is_locked && data.gate?.type === "PAYWALL") {
    showPaywallGate();
  }

  const promoEligible = !data.is_locked && data.access_level === "TRIAL" && !data.entitlement_active;
  if (promoEligible) {
    initPromoSlots("post", data.post.id);
  }

  trackPostEngagement(data.post.id, { promoEligible });
};

const renderCourse = async () => {
  const page = selectors.page();
  if (!page) return;

  const slug = page.dataset.courseSlug;
  const data = await fetchJSON(`${API.courses}/${slug}`, {
    headers: state.token ? { Authorization: `Bearer ${state.token}` } : {}
  });

  const body = selectors.courseBody();
  if (body) {
    body.innerHTML = data.html || "";
  }

  if (data.is_locked && data.gate?.type === "LOGIN_REQUIRED") {
    showLoginGate();
  }
  if (data.is_locked && data.gate?.type === "PAYWALL") {
    showPaywallGate();
  }
};

const renderAdminAnalytics = async () => {
  const errorEl = document.getElementById("admin-error");
  const totalEl = document.getElementById("kpi-total");
  const paidActiveEl = document.getElementById("kpi-paid-active");
  const promoCTREl = document.getElementById("kpi-promo-ctr");
  const stageTotalEl = document.getElementById("stage-total");
  const stageBarsEl = document.getElementById("stage-bars");
  const stageDonutEl = document.getElementById("stage-donut");
  const stageLegendEl = document.getElementById("stage-legend");
  const funnelRowsEl = document.getElementById("funnel-rows");
  const promoTableBody = document.getElementById("promo-table-body");
  const promoSummaryEl = document.getElementById("promo-summary");

  const fail = (message) => {
    if (errorEl) {
      errorEl.textContent = message;
      errorEl.classList.remove("hidden");
    }
  };

  if (!requireAdmin(errorEl)) {
    return;
  }

  try {
    const [funnelData, promoStats, promoDefs] = await Promise.all([
      fetchJSON("/api/admin/analytics/funnel"),
      fetchJSON("/api/admin/analytics/promos"),
      fetchJSON("/api/admin/promos")
    ]);

    const stages = normalizeStages(funnelData?.stages || []);
    const totalUsers = stages.reduce((sum, stage) => sum + stage.count, 0);
    const paidActive = stages.find((stage) => stage.key === "PAID_ACTIVE")?.count || 0;

    if (totalEl) totalEl.textContent = totalUsers.toLocaleString();
    if (paidActiveEl) paidActiveEl.textContent = paidActive.toLocaleString();
    if (stageTotalEl) stageTotalEl.textContent = totalUsers.toLocaleString();

    renderStageBars(stageBarsEl, stages, totalUsers);
    renderStageDonut(stageDonutEl, stageLegendEl, stages, totalUsers);
    renderFunnelRows(funnelRowsEl, stages);

    const promoRows = buildPromoRows(promoStats, promoDefs?.promos || []);
    renderPromoTable(promoTableBody, promoSummaryEl, promoRows);
    if (promoCTREl) {
      promoCTREl.textContent = formatPercent(overallPromoCTR(promoRows));
    }
  } catch (err) {
    console.error(err);
    fail("Unable to load analytics. Ensure you are signed in as an admin.");
  }
};

const renderAdminPostAnalytics = async () => {
  const page = selectors.page();
  const errorEl = document.getElementById("post-analytics-error");
  if (!page) return;
  clearError(errorEl);
  if (!requireAdmin(errorEl)) return;

  const postID = page.dataset.postId;
  if (!postID) {
    setError(errorEl, "Post ID missing.");
    return;
  }

  const fromInput = document.getElementById("post-analytics-from");
  const toInput = document.getElementById("post-analytics-to");
  const applyBtn = document.getElementById("post-analytics-apply");
  const uniqueEl = document.getElementById("post-kpi-unique");
  const viewsEl = document.getElementById("post-kpi-views");
  const completionEl = document.getElementById("post-kpi-completion");
  const promoCTREl = document.getElementById("post-kpi-promo-ctr");
  const promoSummaryEl = document.getElementById("post-kpi-promo-summary");
  const funnelEl = document.getElementById("post-funnel-steps");
  const seriesBody = document.getElementById("post-timeseries-body");
  const promoTable = document.getElementById("post-promo-table");

  const today = new Date();
  const toDefault = today.toISOString().slice(0, 10);
  const fromDefault = new Date(today.getTime() - 13 * 24 * 60 * 60 * 1000).toISOString().slice(0, 10);
  if (fromInput && !fromInput.value) fromInput.value = fromDefault;
  if (toInput && !toInput.value) toInput.value = toDefault;

  const loadAnalytics = async () => {
    clearError(errorEl);
    const from = fromInput?.value || fromDefault;
    const to = toInput?.value || toDefault;
    const query = `from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`;
    try {
      const [summary, series, funnel, promos, promoDefs] = await Promise.all([
        fetchJSON(`/api/admin/analytics/posts/${postID}/summary?${query}`),
        fetchJSON(`/api/admin/analytics/posts/${postID}/timeseries?${query}`),
        fetchJSON(`/api/admin/analytics/posts/${postID}/funnel?${query}`),
        fetchJSON(`/api/admin/analytics/posts/${postID}/promos?${query}`),
        fetchJSON("/api/admin/promos")
      ]);

      const totals = summary?.totals || {};
      if (uniqueEl) uniqueEl.textContent = (totals.unique_impressions || 0).toLocaleString();
      if (viewsEl) viewsEl.textContent = (totals.total_views || 0).toLocaleString();
      if (completionEl) completionEl.textContent = formatPercent(totals.completion_rate || 0);
      if (promoCTREl) promoCTREl.textContent = formatPercent(totals.promo_ctr || 0);
      if (promoSummaryEl) {
        promoSummaryEl.textContent = `${totals.promo_impressions || 0} impressions · ${totals.promo_clicks || 0} clicks`;
      }

      const steps = funnel?.steps || [];
      renderPostFunnel(funnelEl, steps);

      const days = series?.days || [];
      renderPostTimeseries(seriesBody, days);

      const promoRows = promos?.items || [];
      renderPostPromoTable(promoTable, promoRows, promoDefs?.promos || []);
    } catch (err) {
      setError(errorEl, err.message || "Failed to load post analytics.");
    }
  };

  if (applyBtn) {
    applyBtn.addEventListener("click", (event) => {
      event.preventDefault();
      loadAnalytics();
    });
  }

  await loadAnalytics();
};

const renderAccount = async () => {
  const errorEl = document.getElementById("account-error");
  if (!state.token) {
    if (errorEl) {
      errorEl.textContent = "Sign in to view your account.";
      errorEl.classList.remove("hidden");
    }
    return;
  }

  try {
    const data = await fetchJSON(API.me);
    state.user = data.user || state.user;
    state.entitlement = data.entitlement || state.entitlement;
    setStoredUser(state.user);
    setStoredEntitlement(state.entitlement);
    updateNavState();

    const nameEl = document.getElementById("account-name");
    const emailEl = document.getElementById("account-email");
    const planEl = document.getElementById("account-plan");
    const statusEl = document.getElementById("account-status");
    const expiryEl = document.getElementById("account-expiry");
    const expiryRow = document.getElementById("account-expiry-row");
    const renewBtn = document.getElementById("account-renew");

    if (nameEl) nameEl.textContent = state.user?.name || "Member";
    if (emailEl) emailEl.textContent = state.user?.email || "—";

    if (state.entitlement) {
      if (planEl) planEl.textContent = state.entitlement.plan_code?.toUpperCase() || "Plan";
      if (statusEl) statusEl.textContent = state.entitlement.status || "—";
      const expiryValue = formatDate(state.entitlement.end_at);
      if (expiryEl) expiryEl.textContent = expiryValue;
      if (expiryRow) {
        expiryRow.classList.toggle("hidden", expiryValue === "—");
      }
      if (renewBtn) {
        renewBtn.textContent = state.entitlement.status === "ACTIVE" ? "Manage plan" : "Renew membership";
      }
    } else {
      if (planEl) planEl.textContent = "No active plan";
      if (statusEl) statusEl.textContent = "Free";
      if (expiryEl) expiryEl.textContent = "—";
      if (expiryRow) {
        expiryRow.classList.add("hidden");
      }
    }

    if (renewBtn) {
      renewBtn.addEventListener("click", () => {
        window.location.href = "/pricing";
      });
    }
  } catch (err) {
    console.error(err);
    if (errorEl) {
      errorEl.textContent = "Unable to load account details.";
      errorEl.classList.remove("hidden");
    }
  }
};

const renderAdminDashboard = async () => {
  const errorEl = document.getElementById("admin-dashboard-error");
  clearError(errorEl);
  if (!requireAdmin(errorEl)) return;
};

const renderAdminPosts = async () => {
  const errorEl = document.getElementById("admin-posts-error");
  clearError(errorEl);
  if (!requireAdmin(errorEl)) return;

  const queryInput = document.getElementById("posts-query");
  const statusSelect = document.getElementById("posts-status");
  const refreshBtn = document.getElementById("posts-refresh");
  const table = document.getElementById("posts-table");

  const editorTitle = document.getElementById("post-editor-title");
  const editorForm = document.getElementById("post-editor");

  const resetEditor = () => {
    if (!editorForm) return;
    editorForm.reset();
    document.getElementById("post-id").value = "";
    if (editorTitle) editorTitle.textContent = "New post";
  };

  const loadPosts = async () => {
    if (!table) return;
    try {
      clearError(errorEl);
      const params = new URLSearchParams();
      if (queryInput?.value) params.set("q", queryInput.value.trim());
      if (statusSelect?.value) params.set("status", statusSelect.value);
      const data = await fetchJSON(`/api/admin/posts?${params.toString()}`);
      const items = data.items || [];
      table.innerHTML = items
        .map(
          (post) => `
        <tr>
          <td class="py-4 text-slate-800">${post.title}</td>
          <td class="py-4 text-slate-600">${post.access_level}</td>
          <td class="py-4 text-slate-600">${post.status}</td>
          <td class="py-4 text-slate-600">
            <a class="text-primary" href="/admin/posts/${post.id}/analytics">View</a>
          </td>
          <td class="py-4 text-right">
            <button class="text-primary" data-edit-post="${post.id}">Edit</button>
          </td>
        </tr>
      `
        )
        .join("");

      table.querySelectorAll("[data-edit-post]").forEach((btn) => {
        btn.addEventListener("click", () => loadPostDetail(btn.dataset.editPost));
      });
    } catch (err) {
      setError(errorEl, err.message || "Failed to load posts.");
    }
  };

  const loadPostDetail = async (id) => {
    if (!id) return;
    try {
      clearError(errorEl);
      const data = await fetchJSON(`/api/admin/posts/${id}`);
      const post = data.post;
      if (!post) return;
      document.getElementById("post-id").value = post.id;
      document.getElementById("post-title").value = post.title || "";
      document.getElementById("post-slug").value = post.slug || "";
      document.getElementById("post-excerpt").value = post.excerpt || "";
      document.getElementById("post-body").value = post.body_markdown || "";
      document.getElementById("post-access").value = post.access_level || "PUBLIC";
      document.getElementById("post-status").value = post.status || "DRAFT";
      document.getElementById("post-published").value = post.published_at ? post.published_at.slice(0, 10) : "";
      document.getElementById("post-topic-tags").value = (post.topic_tags || []).join(", ");
      document.getElementById("post-interest-tags").value = (post.interest_tags || []).join(", ");
      document.getElementById("post-meta-title").value = post.meta_title || "";
      document.getElementById("post-meta-description").value = post.meta_description || "";
      document.getElementById("post-meta-image").value = post.meta_image_url || "";
      document.getElementById("post-canonical").value = post.canonical_url || "";
      document.getElementById("post-noindex").checked = Boolean(post.noindex);
      if (editorTitle) editorTitle.textContent = `Editing: ${post.title}`;
    } catch (err) {
      setError(errorEl, err.message || "Failed to load post.");
    }
  };

  if (refreshBtn) refreshBtn.addEventListener("click", loadPosts);
  if (queryInput) queryInput.addEventListener("change", loadPosts);
  if (statusSelect) statusSelect.addEventListener("change", loadPosts);

  if (editorForm) {
    editorForm.addEventListener("submit", async (event) => {
      event.preventDefault();
      clearError(errorEl);
      const payload = buildPostPayload();
      const validationError = validatePostPayload(payload);
      if (validationError) {
        setError(errorEl, validationError);
        return;
      }
      try {
        const postID = document.getElementById("post-id").value;
        const method = postID ? "PUT" : "POST";
        const url = postID ? `/api/admin/posts/${postID}` : "/api/admin/posts";
        await fetchJSON(url, {
          method,
          body: JSON.stringify(payload)
        });
        showToast("Post saved");
        resetEditor();
        await loadPosts();
      } catch (err) {
        setError(errorEl, err.message || "Failed to save post.");
      }
    });
  }

  const resetBtn = document.getElementById("post-reset");
  if (resetBtn) resetBtn.addEventListener("click", resetEditor);

  const buildPostPayload = () => {
    const publishedRaw = document.getElementById("post-published").value;
    return {
      slug: document.getElementById("post-slug").value.trim(),
      title: document.getElementById("post-title").value.trim(),
      excerpt: document.getElementById("post-excerpt").value.trim(),
      body_markdown: document.getElementById("post-body").value.trim(),
      access_level: document.getElementById("post-access").value,
      status: document.getElementById("post-status").value,
      published_at: publishedRaw ? `${publishedRaw}T00:00:00Z` : null,
      topic_tags: splitTags(document.getElementById("post-topic-tags").value),
      interest_tags: splitTags(document.getElementById("post-interest-tags").value),
      meta_title: document.getElementById("post-meta-title").value.trim(),
      meta_description: document.getElementById("post-meta-description").value.trim(),
      meta_image_url: document.getElementById("post-meta-image").value.trim(),
      canonical_url: document.getElementById("post-canonical").value.trim(),
      noindex: document.getElementById("post-noindex").checked
    };
  };

  await loadPosts();
};

const renderAdminFunnel = async () => {
  const errorEl = document.getElementById("admin-funnel-error");
  clearError(errorEl);
  if (!requireAdmin(errorEl)) return;

  const configForm = document.getElementById("funnel-config");
  const weightsContainer = document.getElementById("funnel-weights");
  const stagesContainer = document.getElementById("funnel-stages");

  try {
    const [configData, weightsData, stagesData] = await Promise.all([
      fetchJSON("/api/admin/funnel/config"),
      fetchJSON("/api/admin/funnel/weights"),
      fetchJSON("/api/admin/funnel/stages")
    ]);

    const cfg = configData.config || {};
    document.getElementById("funnel-window").value = cfg.scoring_window_days || 14;
    document.getElementById("funnel-decay").checked = Boolean(cfg.decay_enabled);
    document.getElementById("funnel-decay-factor").value = cfg.daily_decay_factor || 0.9;
    document.getElementById("funnel-dormant").value = cfg.dormant_days_threshold || 21;

    const weights = (weightsData.weights || []).map((weight) => ({
      eventType: weight.event_type || weight.EventType || "",
      weight: Number(weight.weight ?? weight.Weight ?? 0),
      enabled: Boolean(weight.enabled ?? weight.Enabled)
    }));
    if (weightsContainer) {
      if (weights.length === 0) {
        weightsContainer.innerHTML = "<p class=\"text-sm text-slate-500\">No weights found yet. Refresh to load defaults.</p>";
      } else {
        weightsContainer.innerHTML = weights
          .map(
            (weight) => `
          <div class="flex items-center gap-3" data-weight-row>
            <div class="flex-1 text-sm text-slate-600">
              <p class="font-medium text-slate-700">${weight.eventType || "—"}</p>
              <p class="text-xs text-slate-500">${describeEventWeight(weight.eventType)}</p>
            </div>
            <input type="number" placeholder="0" class="w-20 rounded-xl bg-white border border-slate-200 px-3 py-2 text-slate-700" value="${weight.weight}" data-weight-input />
            <label class="text-xs text-slate-500 flex items-center gap-2">
              <input type="checkbox" ${weight.enabled ? "checked" : ""} data-weight-enabled />
              Enabled
            </label>
          </div>
        `
          )
          .join("");
      }
    }

    const stages = (stagesData.stages || []).map((stage) => ({
      stage: stage.stage || stage.Stage || "",
      minScore: Number(stage.min_score ?? stage.MinScore ?? 0),
      maxScore: stage.max_score ?? stage.MaxScore ?? null,
      enabled: Boolean(stage.enabled ?? stage.Enabled)
    }));
    if (stagesContainer) {
      if (stages.length === 0) {
        stagesContainer.innerHTML = "<p class=\"text-sm text-slate-500\">No stages found yet. Refresh to load defaults.</p>";
      } else {
        stagesContainer.innerHTML = stages
          .map(
            (stage) => `
          <div class="rounded-2xl border border-slate-200 p-4" data-stage-row>
            <div class="text-sm text-slate-600">${stage.stage || "—"}</div>
            <div class="mt-3 grid grid-cols-2 gap-3">
              <input type="number" placeholder="Min" class="rounded-xl bg-white border border-slate-200 px-3 py-2 text-slate-700" value="${stage.minScore}" data-stage-min />
              <input type="number" placeholder="Max (optional)" class="rounded-xl bg-white border border-slate-200 px-3 py-2 text-slate-700" value="${stage.maxScore ?? ""}" data-stage-max />
            </div>
            <label class="mt-3 text-xs text-slate-500 flex items-center gap-2">
              <input type="checkbox" ${stage.enabled ? "checked" : ""} data-stage-enabled />
              Enabled
            </label>
          </div>
        `
          )
          .join("");
      }
    }

    if (configForm) {
      configForm.addEventListener("submit", async (event) => {
        event.preventDefault();
        clearError(errorEl);
        const payload = {
          scoring_window_days: Number(document.getElementById("funnel-window").value),
          decay_enabled: document.getElementById("funnel-decay").checked,
          daily_decay_factor: Number(document.getElementById("funnel-decay-factor").value),
          dormant_days_threshold: Number(document.getElementById("funnel-dormant").value)
        };
        const validationError = validateFunnelConfig(payload);
        if (validationError) {
          setError(errorEl, validationError);
          return;
        }
        try {
          await fetchJSON("/api/admin/funnel/config", {
            method: "PUT",
            body: JSON.stringify(payload)
          });
          showToast("Config saved");
        } catch (err) {
          setError(errorEl, err.message || "Failed to save config.");
        }
      });
    }

    const weightsSave = document.getElementById("funnel-weights-save");
    if (weightsSave) {
      weightsSave.addEventListener("click", async () => {
        clearError(errorEl);
        const rows = Array.from(document.querySelectorAll("[data-weight-row]"));
        const payload = rows.map((row, index) => ({
          event_type: weights[index]?.eventType,
          weight: Number(row.querySelector("[data-weight-input]").value),
          enabled: row.querySelector("[data-weight-enabled]").checked
        }));
        const validationError = validateWeightsPayload(payload);
        if (validationError) {
          setError(errorEl, validationError);
          return;
        }
        try {
          await fetchJSON("/api/admin/funnel/weights", { method: "PUT", body: JSON.stringify(payload) });
          showToast("Weights saved");
        } catch (err) {
          setError(errorEl, err.message || "Failed to save weights.");
        }
      });
    }

    const stagesSave = document.getElementById("funnel-stages-save");
    if (stagesSave) {
      stagesSave.addEventListener("click", async () => {
        clearError(errorEl);
        const rows = Array.from(document.querySelectorAll("[data-stage-row]"));
        const payload = rows.map((row, index) => ({
          stage: stages[index]?.stage,
          min_score: Number(row.querySelector("[data-stage-min]").value),
          max_score: parseNullableNumber(row.querySelector("[data-stage-max]").value),
          enabled: row.querySelector("[data-stage-enabled]").checked
        }));
        const validationError = validateStagesPayload(payload);
        if (validationError) {
          setError(errorEl, validationError);
          return;
        }
        try {
          await fetchJSON("/api/admin/funnel/stages", { method: "PUT", body: JSON.stringify(payload) });
          showToast("Stages saved");
        } catch (err) {
          setError(errorEl, err.message || "Failed to save stages.");
        }
      });
    }
  } catch (err) {
    console.error(err);
    if (errorEl) {
      setError(errorEl, "Unable to load funnel settings.");
    }
  }
};

const renderAdminPromos = async () => {
  const errorEl = document.getElementById("admin-promos-error");
  clearError(errorEl);
  if (!requireAdmin(errorEl)) return;

  const table = document.getElementById("promos-table");
  const refreshBtn = document.getElementById("promos-refresh");
  const form = document.getElementById("promo-editor");
  const editorTitle = document.getElementById("promo-editor-title");
  const variantsContainer = document.getElementById("promo-variants");
  const addVariantBtn = document.getElementById("promo-add-variant");

  const formatVariantPayload = (payload) => {
    if (!payload) return "";
    if (typeof payload === "string") return payload;
    try {
      return JSON.stringify(payload);
    } catch (err) {
      console.error(err);
      return "";
    }
  };

  const buildVariantCard = (variant = {}, index = 0) => {
    const card = document.createElement("div");
    card.className = "rounded-2xl border border-slate-200 bg-white/70 p-4 space-y-3";
    card.dataset.variantCard = "true";
    card.innerHTML = `
      <div class="flex items-center justify-between">
        <p class="text-xs uppercase tracking-wide text-slate-500" data-variant-label>Variant ${index + 1}</p>
        <button type="button" data-remove-variant class="text-xs uppercase tracking-wide text-slate-500">Remove</button>
      </div>
      <input data-variant-field="headline" placeholder="Headline" class="w-full rounded-2xl bg-white border border-slate-200 px-4 py-3 text-slate-700" />
      <textarea data-variant-field="body" rows="3" placeholder="Short benefit statement" class="w-full rounded-2xl bg-white border border-slate-200 px-4 py-3 text-slate-700"></textarea>
      <div class="grid gap-3 md:grid-cols-2">
        <div class="space-y-2">
          <label class="text-sm text-slate-600">CTA text</label>
          <input data-variant-field="cta_text" placeholder="Subscribe now" class="w-full rounded-2xl bg-white border border-slate-200 px-4 py-3 text-slate-700" />
        </div>
        <div class="space-y-2">
          <label class="text-sm text-slate-600">CTA action</label>
          <select data-variant-field="cta_action" class="w-full rounded-2xl bg-white border border-slate-200 px-4 py-3 text-slate-700">
            <option value="OPEN_PRICING">OPEN_PRICING</option>
            <option value="START_CHECKOUT">START_CHECKOUT</option>
            <option value="OPEN_SAMPLE_POST">OPEN_SAMPLE_POST</option>
            <option value="OPEN_ACCOUNT_RENEW">OPEN_ACCOUNT_RENEW</option>
          </select>
        </div>
      </div>
      <div class="space-y-2">
        <label class="text-sm text-slate-600">CTA payload JSON (optional)</label>
        <input data-variant-field="cta_payload" placeholder='{"plan_default":"monthly"}' class="w-full rounded-2xl bg-white border border-slate-200 px-4 py-3 text-slate-700" />
        <p class="text-xs text-slate-500">Only required for START_CHECKOUT to preselect a plan.</p>
      </div>
      <div class="space-y-2">
        <label class="text-sm text-slate-600">Variant weight</label>
        <input data-variant-field="weight" type="number" placeholder="1" class="w-full rounded-2xl bg-white border border-slate-200 px-4 py-3 text-slate-700" />
      </div>
    `;

    const fieldValues = {
      headline: variant.headline || variant.Headline || "",
      body: variant.body || variant.Body || "",
      cta_text: variant.cta_text || variant.CTAText || "",
      cta_action: variant.cta_action || variant.CTAAction || "OPEN_PRICING",
      cta_payload: formatVariantPayload(variant.cta_payload || variant.CTAPayload),
      weight: Number.isFinite(Number(variant.weight || variant.Weight)) ? String(variant.weight || variant.Weight) : "1"
    };

    Object.entries(fieldValues).forEach(([field, value]) => {
      const input = card.querySelector(`[data-variant-field="${field}"]`);
      if (!input) return;
      if (field === "cta_action") {
        input.value = value || "OPEN_PRICING";
        return;
      }
      input.value = value;
    });

    const removeBtn = card.querySelector("[data-remove-variant]");
    if (removeBtn) {
      removeBtn.addEventListener("click", () => {
        card.remove();
        syncVariantCards();
      });
    }
    return card;
  };

  const syncVariantCards = () => {
    if (!variantsContainer) return;
    const cards = Array.from(variantsContainer.querySelectorAll("[data-variant-card]"));
    cards.forEach((card, idx) => {
      const label = card.querySelector("[data-variant-label]");
      if (label) {
        label.textContent = `Variant ${idx + 1}`;
      }
      const removeBtn = card.querySelector("[data-remove-variant]");
      if (removeBtn) {
        const disabled = cards.length === 1;
        removeBtn.disabled = disabled;
        removeBtn.classList.toggle("opacity-40", disabled);
        removeBtn.classList.toggle("cursor-not-allowed", disabled);
      }
    });
  };

  const renderVariants = (variants) => {
    if (!variantsContainer) return;
    variantsContainer.innerHTML = "";
    const rows = Array.isArray(variants) && variants.length > 0 ? variants : [{}];
    rows.forEach((variant, idx) => {
      variantsContainer.appendChild(buildVariantCard(variant, idx));
    });
    syncVariantCards();
  };

  const collectVariants = () => {
    if (!variantsContainer) return { variants: [], errors: [] };
    const cards = Array.from(variantsContainer.querySelectorAll("[data-variant-card]"));
    const variants = [];
    const errors = [];
    cards.forEach((card, idx) => {
      const readField = (name) => {
        const input = card.querySelector(`[data-variant-field="${name}"]`);
        return input ? input.value.trim() : "";
      };
      const payloadRaw = readField("cta_payload");
      let payloadValue = {};
      if (payloadRaw) {
        try {
          payloadValue = JSON.parse(payloadRaw);
        } catch (err) {
          console.error(err);
          errors.push(`Variant ${idx + 1} CTA payload JSON is invalid.`);
        }
      }
      const weightValue = Number(readField("weight"));
      variants.push({
        headline: readField("headline"),
        body: readField("body"),
        cta_text: readField("cta_text"),
        cta_action: readField("cta_action"),
        cta_payload: payloadValue || {},
        weight: Number.isFinite(weightValue) ? weightValue : 1
      });
    });
    return { variants, errors };
  };

  const resetForm = () => {
    if (form) form.reset();
    document.getElementById("promo-id").value = "";
    if (editorTitle) editorTitle.textContent = "New promo";
    renderVariants([]);
  };

  const loadPromos = async () => {
    if (!table) return;
    try {
      clearError(errorEl);
      const data = await fetchJSON("/api/admin/promos");
      const promos = data.promos || [];
      table.innerHTML = promos
        .map(
          (promo) => `
        <tr>
          <td class="py-4 text-slate-800">${promo.code || promo.Code}</td>
          <td class="py-4 text-slate-600">${promo.slot || promo.Slot}</td>
          <td class="py-4 text-slate-600">${promo.status || promo.Status}</td>
          <td class="py-4 text-right"><button class="text-primary" data-edit-promo="${promo.id || promo.ID}">Edit</button></td>
        </tr>
      `
        )
        .join("");

      table.querySelectorAll("[data-edit-promo]").forEach((btn) => {
        btn.addEventListener("click", () => loadPromoDetail(btn.dataset.editPromo));
      });

      state._adminPromos = promos;
    } catch (err) {
      setError(errorEl, err.message || "Failed to load promos.");
    }
  };

  const loadPromoDetail = async (id) => {
    if (!id) return;
    try {
      clearError(errorEl);
      const data = await fetchJSON(`/api/admin/promos/${id}`);
      const promo = data.promo || {};
      const variants = data.variants || [];
      document.getElementById("promo-id").value = promo.id || promo.ID;
      document.getElementById("promo-code").value = promo.code || promo.Code || "";
      document.getElementById("promo-name").value = promo.name || promo.Name || "";
      document.getElementById("promo-slot").value = promo.slot || promo.Slot || "INLINE";
      document.getElementById("promo-status").value = promo.status || promo.Status || "ACTIVE";
      document.getElementById("promo-priority").value = promo.priority || promo.Priority || 0;
      document.getElementById("promo-cooldown").value = promo.cooldown_hours || promo.CooldownHours || 0;
      document.getElementById("promo-impressions").value = promo.max_impressions_per_day || promo.MaxImpressionsPerDay || 0;
      document.getElementById("promo-clicks").value = promo.max_clicks_per_day || promo.MaxClicksPerDay || 0;
      document.getElementById("promo-stages").value = parseEligibleStages(promo.eligible_stages_json || promo.EligibleStagesJSON).join(", ");
      document.getElementById("promo-start").value = promo.start_at ? promo.start_at.slice(0, 10) : "";
      document.getElementById("promo-end").value = promo.end_at ? promo.end_at.slice(0, 10) : "";
      if (editorTitle) editorTitle.textContent = `Editing: ${promo.name || promo.code || promo.Code}`;
      renderVariants(variants);
    } catch (err) {
      setError(errorEl, err.message || "Failed to load promo details.");
    }
  };

  if (refreshBtn) refreshBtn.addEventListener("click", loadPromos);
  if (addVariantBtn) {
    addVariantBtn.addEventListener("click", () => {
      if (!variantsContainer) return;
      variantsContainer.appendChild(buildVariantCard({}, variantsContainer.children.length));
      syncVariantCards();
    });
  }

  const buildPromoPayload = () => {
    const { variants, errors } = collectVariants();
    const payload = {
      code: document.getElementById("promo-code").value.trim(),
      name: document.getElementById("promo-name").value.trim(),
      slot: document.getElementById("promo-slot").value,
      status: document.getElementById("promo-status").value,
      priority: Number(document.getElementById("promo-priority").value),
      eligible_stages: splitTags(document.getElementById("promo-stages").value),
      cooldown_hours: Number(document.getElementById("promo-cooldown").value),
      max_impressions_per_day: Number(document.getElementById("promo-impressions").value),
      max_clicks_per_day: Number(document.getElementById("promo-clicks").value),
      start_at: toISODate(document.getElementById("promo-start").value),
      end_at: toISODate(document.getElementById("promo-end").value),
      variants
    };
    return { payload, errors };
  };

  if (form) {
    form.addEventListener("submit", async (event) => {
      event.preventDefault();
      clearError(errorEl);
      const { payload, errors: variantErrors } = buildPromoPayload();
      const validationError = validatePromoPayload(payload, variantErrors);
      if (validationError) {
        setError(errorEl, validationError);
        return;
      }
      try {
        const promoID = document.getElementById("promo-id").value;
        const method = promoID ? "PUT" : "POST";
        const url = promoID ? `/api/admin/promos/${promoID}` : "/api/admin/promos";
        await fetchJSON(url, { method, body: JSON.stringify(payload) });
        showToast("Promo saved");
        resetForm();
        await loadPromos();
      } catch (err) {
        setError(errorEl, err.message || "Failed to save promo.");
      }
    });
  }

  const resetBtn = document.getElementById("promo-reset");
  if (resetBtn) resetBtn.addEventListener("click", resetForm);
  renderVariants([]);
  await loadPromos();
};

const renderAdminUsers = async () => {
  const errorEl = document.getElementById("admin-users-error");
  clearError(errorEl);
  if (!requireAdmin(errorEl)) return;

  const table = document.getElementById("users-table");
  const refreshBtn = document.getElementById("users-refresh");
  const queryInput = document.getElementById("users-query");
  const detail = document.getElementById("user-detail");

  const loadUsers = async () => {
    try {
      clearError(errorEl);
      const params = new URLSearchParams();
      if (queryInput?.value) params.set("q", queryInput.value.trim());
      const data = await fetchJSON(`/api/admin/users?${params.toString()}`);
      const items = data.items || [];
      if (table) {
        table.innerHTML = items
          .map(
            (user) => `
          <tr>
            <td class="py-4 text-slate-800">${user.name || "—"}</td>
            <td class="py-4 text-slate-600">${user.email}</td>
            <td class="py-4 text-slate-600">${user.role}</td>
            <td class="py-4 text-right"><button class="text-primary" data-user-id="${user.id}">View</button></td>
          </tr>
        `
          )
          .join("");
        table.querySelectorAll("[data-user-id]").forEach((btn) => {
          btn.addEventListener("click", () => loadUserDetail(btn.dataset.userId));
        });
      }
    } catch (err) {
      setError(errorEl, err.message || "Failed to load users.");
    }
  };

  const loadUserDetail = async (id) => {
    if (!id || !detail) return;
    try {
      clearError(errorEl);
      const data = await fetchJSON(`/api/admin/users/${id}`);
      const user = data.user || {};
      const metrics = data.metrics || {};
      const entitlement = data.entitlement || {};
      detail.innerHTML = `
        <p class="text-ink text-lg">${user.name || "—"}</p>
        <p class="text-slate-500">${user.email || "—"}</p>
        <div class="mt-4 text-sm text-slate-600 space-y-1">
          <div>Role: ${user.role || "—"}</div>
          <div>Status: ${user.status || "—"}</div>
          <div>Stage: ${metrics.stage || "—"}</div>
          <div>Score: ${metrics.score ?? "—"}</div>
          <div>Plan: ${entitlement.plan_code || "—"}</div>
          <div>Entitlement: ${entitlement.status || "—"}</div>
        </div>
      `;
    } catch (err) {
      setError(errorEl, err.message || "Failed to load user.");
    }
  };

  if (refreshBtn) refreshBtn.addEventListener("click", loadUsers);
  if (queryInput) queryInput.addEventListener("change", loadUsers);

  await loadUsers();
};

const renderAdminSettings = async () => {
  const errorEl = document.getElementById("admin-settings-error");
  clearError(errorEl);
  if (!requireAdmin(errorEl)) return;

  const form = document.getElementById("settings-form");
  try {
    const data = await fetchJSON("/api/admin/settings/site");
    const settings = data.settings || {};
    document.getElementById("settings-site-name").value = settings.site_name || settings.SiteName || "";
    document.getElementById("settings-site-url").value = settings.site_url || settings.SiteURL || "";
    document.getElementById("settings-primary-color").value = settings.primary_color || settings.PrimaryColor || "";
    document.getElementById("settings-og-image").value = settings.default_og_image_url || settings.DefaultOgImageURL || "";
    document.getElementById("settings-twitter-site").value = settings.twitter_site || settings.TwitterSite || "";
    document.getElementById("settings-twitter-creator").value = settings.twitter_creator || settings.TwitterCreator || "";
    document.getElementById("settings-google").value = settings.google_verification || settings.GoogleVerification || "";
    document.getElementById("settings-bing").value = settings.bing_verification || settings.BingVerification || "";
    document.getElementById("settings-pinterest").value = settings.pinterest_verification || settings.PinterestVerification || "";
  } catch (err) {
    console.error(err);
    setError(errorEl, "Unable to load settings.");
  }

  if (form) {
    form.addEventListener("submit", async (event) => {
      event.preventDefault();
      clearError(errorEl);
      const payload = {
        site_name: document.getElementById("settings-site-name").value.trim(),
        site_url: document.getElementById("settings-site-url").value.trim(),
        primary_color: document.getElementById("settings-primary-color").value.trim(),
        default_og_image_url: document.getElementById("settings-og-image").value.trim(),
        twitter_site: document.getElementById("settings-twitter-site").value.trim(),
        twitter_creator: document.getElementById("settings-twitter-creator").value.trim(),
        google_verification: document.getElementById("settings-google").value.trim(),
        bing_verification: document.getElementById("settings-bing").value.trim(),
        pinterest_verification: document.getElementById("settings-pinterest").value.trim()
      };
      const validationError = validateSettingsPayload(payload);
      if (validationError) {
        setError(errorEl, validationError);
        return;
      }
      try {
        await fetchJSON("/api/admin/settings/site", { method: "PUT", body: JSON.stringify(payload) });
        showToast("Settings saved");
        setTimeout(() => window.location.reload(), 500);
      } catch (err) {
        setError(errorEl, err.message || "Failed to save settings.");
      }
    });
  }
};

const renderPricing = async () => {
  const container = selectors.pricingCards();
  if (!container) return;

  const data = await fetchJSON(API.plans);
  state.plans = data.plans || [];

  container.innerHTML = state.plans.map(renderPlanCard).join("");

  container.querySelectorAll("[data-plan]").forEach((btn) => {
    btn.addEventListener("click", () => {
      const code = btn.dataset.plan;
      if (!state.token) {
        showLoginGate();
        return;
      }
      openCheckout(code);
    });
  });
};

const renderPostCard = (post) => {
  const access = String(post.access_level || "").toUpperCase();
  const badge = access === "PAID"
    ? `<div class="text-xs uppercase tracking-wide text-ember">PAID</div>`
    : "";
  return `
    <a href="/post/${post.slug}" class="rounded-3xl border border-slate-200/60 bg-white/90 p-6 hover:border-skyline/60 transition">
      ${badge}
      <h3 class="mt-4 font-display text-xl text-ink">${post.title}</h3>
      <p class="mt-2 text-slate-600 text-sm">${post.excerpt || ""}</p>
      <div class="mt-4 text-skyline text-sm">Explore →</div>
    </a>
  `;
};

const renderCourseCard = (course) => {
  return `
    <a href="/course/${course.slug}" class="rounded-3xl border border-slate-200/60 bg-white/90 p-6 hover:border-ember/60 transition">
      <div class="text-xs uppercase tracking-wide text-slate-500">${course.access_level}</div>
      <h3 class="mt-4 font-display text-xl text-ink">${course.title}</h3>
      <p class="mt-2 text-slate-600 text-sm">${course.excerpt || ""}</p>
      <div class="mt-4 text-ember text-sm">Start course →</div>
    </a>
  `;
};

const renderPlanCard = (plan) => {
  const highlight = plan.mostPopular ? "border-skyline/80" : "border-slate-200/60";
  return `
    <div class="rounded-3xl border ${highlight} bg-white/90 p-6">
      <p class="text-xs uppercase tracking-wide text-slate-500">${plan.name}</p>
      <h3 class="mt-4 font-display text-3xl text-ink">₹${plan.priceInr}</h3>
      <p class="text-slate-600">${plan.interval === "yearly" ? "per year" : "per month"}</p>
      <button data-plan="${plan.code}" class="mt-6 w-full rounded-full bg-skyline text-white py-3 font-semibold">Subscribe</button>
    </div>
  `;
};

const updateNavState = () => {
  const isSignedIn = Boolean(state.user);
  document.querySelectorAll("#nav-login, #cta-login").forEach((btn) => {
    if (!btn) return;
    btn.classList.toggle("hidden", isSignedIn);
  });

  const logoutBtn = document.getElementById("nav-logout");
  if (logoutBtn) {
    logoutBtn.classList.toggle("hidden", !isSignedIn);
  }

  const accountLink = document.getElementById("nav-account");
  if (accountLink) {
    accountLink.classList.toggle("hidden", !isSignedIn);
  }

  const ctaSection = document.getElementById("cta-login-section");
  if (ctaSection) {
    ctaSection.classList.toggle("hidden", isSignedIn);
  }

  const role = String(state.user?.role || "").toUpperCase();
  const isAdmin = role === "ADMIN" || role === "SUPER_ADMIN";
  document.querySelectorAll("[data-admin-link]").forEach((link) => {
    if (!link) return;
    link.classList.toggle("hidden", !isAdmin);
  });
};

const initNavActions = () => {
  const logoutBtn = document.getElementById("nav-logout");
  if (logoutBtn) {
    logoutBtn.addEventListener("click", () => {
      clearToken();
      window.location.reload();
    });
  }
};

const highlightNav = () => {
  const path = window.location.pathname;
  document.querySelectorAll("[data-nav-link]").forEach((link) => {
    if (!link) return;
    const target = link.getAttribute("data-nav-link");
    const isActive = target === "/" ? path === "/" : path.startsWith(target);
    link.classList.toggle("text-ink", isActive);
  });

  document.querySelectorAll("[data-admin-nav]").forEach((link) => {
    if (!link) return;
    const target = link.getAttribute("data-admin-nav");
    const isActive = target === "/admin/posts" ? path.startsWith("/admin/posts") : path === target;
    link.classList.toggle("text-ink", isActive);
    link.classList.toggle("border-primary/60", isActive);
  });
};

const requireAdmin = (errorEl) => {
  if (!state.token || !state.user) {
    if (errorEl) {
      errorEl.textContent = "Sign in with an admin account to view this page.";
      errorEl.classList.remove("hidden");
    }
    return false;
  }
  const role = String(state.user?.role || "").toUpperCase();
  const isAdmin = role === "ADMIN" || role === "SUPER_ADMIN";
  if (!isAdmin && errorEl) {
    errorEl.textContent = "You do not have admin access.";
    errorEl.classList.remove("hidden");
  }
  return isAdmin;
};

const splitTags = (value) => {
  return value
    .split(",")
    .map((tag) => tag.trim())
    .filter(Boolean);
};

const setError = (el, message) => {
  if (!el) return;
  el.textContent = message;
  el.classList.remove("hidden");
  const rect = el.getBoundingClientRect();
  const viewportHeight = window.innerHeight || document.documentElement.clientHeight;
  if (rect.top < 0 || rect.bottom > viewportHeight) {
    el.scrollIntoView({ behavior: "smooth", block: "center" });
  }
};

const clearError = (el) => {
  if (!el) return;
  el.textContent = "";
  el.classList.add("hidden");
};

const validatePostPayload = (payload) => {
  if (!payload.slug) return "Post slug is required.";
  if (!payload.title) return "Post title is required.";
  if (!payload.body_markdown) return "Post body is required.";
  if (!payload.access_level) return "Access level is required.";
  if (!payload.status) return "Status is required.";
  return null;
};

const validateFunnelConfig = (payload) => {
  if (!Number.isFinite(payload.scoring_window_days) || payload.scoring_window_days <= 0) {
    return "Scoring window must be greater than 0.";
  }
  if (payload.decay_enabled) {
    if (!Number.isFinite(payload.daily_decay_factor) || payload.daily_decay_factor <= 0 || payload.daily_decay_factor > 1) {
      return "Daily decay factor must be between 0 and 1.";
    }
  }
  if (!Number.isFinite(payload.dormant_days_threshold) || payload.dormant_days_threshold < 0) {
    return "Dormant threshold must be 0 or greater.";
  }
  return null;
};

const validateWeightsPayload = (payload) => {
  if (!payload || payload.length === 0) return "No weights to save.";
  for (const row of payload) {
    if (!row.event_type) return "Event type is required for all weights.";
    if (!Number.isFinite(row.weight)) return `Weight missing for ${row.event_type}.`;
  }
  return null;
};

const validateStagesPayload = (payload) => {
  if (!payload || payload.length === 0) return "No stages to save.";
  for (const row of payload) {
    if (!row.stage) return "Stage name is required for all thresholds.";
    if (!Number.isFinite(row.min_score)) return `Min score missing for ${row.stage}.`;
    if (row.max_score !== null && row.max_score !== undefined) {
      if (!Number.isFinite(row.max_score)) return `Max score invalid for ${row.stage}.`;
      if (row.max_score < row.min_score) return `Max score must be >= min score for ${row.stage}.`;
    }
  }
  return null;
};

const validatePromoPayload = (payload, variantErrors = []) => {
  if (!payload.code) return "Promo code is required.";
  if (!payload.name) return "Promo name is required.";
  if (!payload.slot) return "Promo slot is required.";
  if (!payload.status) return "Promo status is required.";
  if (!payload.variants || payload.variants.length === 0) return "At least one variant is required.";
  if (variantErrors.length > 0) return variantErrors[0];
  for (let index = 0; index < payload.variants.length; index += 1) {
    const variant = payload.variants[index];
    const label = `Variant ${index + 1}`;
    if (!variant.headline) return `${label} headline is required.`;
    if (!variant.body) return `${label} body is required.`;
    if (!variant.cta_text) return `${label} CTA text is required.`;
    if (!variant.cta_action) return `${label} CTA action is required.`;
  }
  return null;
};

const validateSettingsPayload = (payload) => {
  if (payload.primary_color) {
    const valid = /^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/.test(payload.primary_color);
    if (!valid) {
      return "Primary color must be a hex value like #38bdf8.";
    }
  }
  if (payload.site_url && !payload.site_url.startsWith("http")) {
    return "Site URL must start with http:// or https://";
  }
  return null;
};

const parseNullableNumber = (value) => {
  const trimmed = String(value || "").trim();
  if (trimmed === "") return null;
  const parsed = Number(trimmed);
  return Number.isNaN(parsed) ? null : parsed;
};

const safeJSONParse = (value) => {
  const trimmed = String(value || "").trim();
  if (!trimmed) return null;
  try {
    return JSON.parse(trimmed);
  } catch (err) {
    console.error(err);
    return null;
  }
};

const parseEligibleStages = (raw) => {
  if (!raw) return [];
  try {
    return JSON.parse(raw);
  } catch (err) {
    return [];
  }
};

function describeEventWeight(eventType) {
  const key = String(eventType || "").toLowerCase();
  if (key === "post_open") {
    return "One time when a post is opened.";
  }
  if (key === "scroll_depth") {
    return "Fires at 25/50/75/90% scroll milestones.";
  }
  if (key === "time_on_page") {
    return "Fires at 15/45/90 seconds.";
  }
  if (key === "post_complete") {
    return "Triggered after scroll + time completion.";
  }
  if (key === "promo_click") {
    return "CTA clicks on promos.";
  }
  if (key === "paywall_hit") {
    return "Locked content attempts.";
  }
  return "Event contribution to funnel score.";
}

const toISODate = (value) => {
  const trimmed = String(value || "").trim();
  if (!trimmed) return null;
  return `${trimmed}T00:00:00Z`;
};

const formatDate = (value) => {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toISOString().slice(0, 10);
};

const stagePalette = {
  NEW: { label: "New", color: "#38bdf8" },
  CASUAL: { label: "Casual", color: "#22d3ee" },
  ENGAGED: { label: "Engaged", color: "#f59e0b" },
  HOT: { label: "Hot", color: "#f97316" },
  PAID_ACTIVE: { label: "Paid active", color: "#22c55e" },
  PAID_EXPIRED: { label: "Paid expired", color: "#f43f5e" },
  DORMANT: { label: "Dormant", color: "#94a3b8" }
};

const stageOrder = ["NEW", "CASUAL", "ENGAGED", "HOT", "PAID_ACTIVE", "PAID_EXPIRED", "DORMANT"];

const normalizeStages = (rows) => {
  const counts = new Map();
  rows.forEach((row) => {
    if (!row?.stage) return;
    counts.set(String(row.stage).toUpperCase(), Number(row.count) || 0);
  });

  const stages = [];
  stageOrder.forEach((key) => {
    stages.push(buildStageDatum(key, counts.get(key) || 0));
    counts.delete(key);
  });

  counts.forEach((count, key) => {
    stages.push(buildStageDatum(key, count));
  });

  return stages;
};

const buildStageDatum = (key, count) => {
  const palette = stagePalette[key] || { label: key, color: "#64748b" };
  return { key, label: palette.label, color: palette.color, count };
};

const renderStageBars = (container, stages, total) => {
  if (!container) return;
  container.innerHTML = stages
    .map((stage) => {
      const pct = total > 0 ? (stage.count / total) * 100 : 0;
      return `
        <div class="flex items-center gap-3">
          <div class="w-24 text-xs uppercase tracking-wide text-slate-500">${stage.label}</div>
          <div class="flex-1 h-3 rounded-full bg-slate-200 overflow-hidden">
            <div class="h-full rounded-full" style="width:${pct}%; background:${stage.color};"></div>
          </div>
          <div class="w-12 text-sm text-slate-700 text-right">${stage.count}</div>
        </div>
      `;
    })
    .join("");
};

const renderStageDonut = (donutEl, legendEl, stages, total) => {
  if (!donutEl || !legendEl) return;
  if (total === 0) {
    donutEl.style.background = "#e2e8f0";
    donutEl.innerHTML = "";
    legendEl.innerHTML = "<p class=\"text-slate-500\">No data yet.</p>";
    return;
  }

  let cursor = 0;
  const segments = stages.map((stage) => {
    const pct = total > 0 ? (stage.count / total) * 100 : 0;
    const start = cursor;
    const end = cursor + pct;
    cursor = end;
    return `${stage.color} ${start}% ${end}%`;
  });

  donutEl.style.background = `conic-gradient(${segments.join(", ")})`;
  donutEl.innerHTML = `
    <div class="absolute inset-6 rounded-full bg-white border border-slate-200 flex items-center justify-center">
      <div class="text-center">
        <div class="font-display text-2xl text-ink">${total}</div>
        <div class="text-xs uppercase text-slate-500">Users</div>
      </div>
    </div>
  `;

  legendEl.innerHTML = stages
    .map(
      (stage) => `
        <div class="flex items-center gap-2">
          <span class="h-2 w-2 rounded-full" style="background:${stage.color};"></span>
          <span>${stage.label} (${formatPercent(total > 0 ? stage.count / total : 0)})</span>
        </div>
      `
    )
    .join("");
};

const renderFunnelRows = (container, stages) => {
  if (!container) return;
  const maxCount = stages.reduce((max, stage) => Math.max(max, stage.count), 0) || 1;
  container.innerHTML = stages
    .map((stage) => {
      const pct = (stage.count / maxCount) * 100;
      return `
        <div class="flex items-center justify-center">
          <div class="h-10 rounded-2xl flex items-center justify-between px-4 text-slate-800" style="width:${pct}%; background:${stage.color};">
            <span class="text-xs uppercase tracking-wide">${stage.label}</span>
            <span class="text-sm font-semibold">${stage.count}</span>
          </div>
        </div>
      `;
    })
    .join("");
};

const buildPromoRows = (promoStats, promos) => {
  const impressionsMap = new Map();
  (promoStats?.impressions || []).forEach((row) => {
    impressionsMap.set(row.promo_id, Number(row.count) || 0);
  });

  const clicksMap = new Map();
  (promoStats?.clicks || []).forEach((row) => {
    clicksMap.set(row.promo_id, Number(row.count) || 0);
  });

  const promoMap = new Map();
  promos.forEach((promo) => {
    const promoID = promo.id || promo.ID;
    if (!promoID) return;
    promoMap.set(promoID, promo);
  });

  const ids = new Set([...impressionsMap.keys(), ...clicksMap.keys(), ...promoMap.keys()]);
  const rows = [];
  ids.forEach((id) => {
    const promo = promoMap.get(id);
    const name = promo?.name || promo?.Name || promo?.code || promo?.Code;
    const slot = promo?.slot || promo?.Slot;
    const impressions = impressionsMap.get(id) || 0;
    const clicks = clicksMap.get(id) || 0;
    rows.push({
      id,
      name: name || `Promo #${id}`,
      slot: slot || "—",
      impressions,
      clicks,
      ctr: impressions > 0 ? clicks / impressions : 0
    });
  });

  return rows.sort((a, b) => b.impressions - a.impressions);
};

const renderPromoTable = (tbody, summaryEl, rows) => {
  if (!tbody) return;
  if (rows.length === 0) {
    tbody.innerHTML = "<tr><td class=\"py-4 text-slate-500\" colspan=\"5\">No promo activity yet.</td></tr>";
    if (summaryEl) summaryEl.textContent = "No promo events recorded.";
    return;
  }

  tbody.innerHTML = rows
    .map(
      (row) => `
        <tr>
          <td class="py-4 font-semibold text-slate-800">${row.name}</td>
          <td class="py-4 text-slate-600">${row.slot}</td>
          <td class="py-4 text-slate-600">${row.impressions}</td>
          <td class="py-4 text-slate-600">${row.clicks}</td>
          <td class="py-4 text-slate-700">${formatPercent(row.ctr)}</td>
        </tr>
      `
    )
    .join("");

  const totalImpressions = rows.reduce((sum, row) => sum + row.impressions, 0);
  const totalClicks = rows.reduce((sum, row) => sum + row.clicks, 0);
  if (summaryEl) {
    summaryEl.textContent = `${totalImpressions} impressions · ${totalClicks} clicks`;
  }
};

const overallPromoCTR = (rows) => {
  const totalImpressions = rows.reduce((sum, row) => sum + row.impressions, 0);
  const totalClicks = rows.reduce((sum, row) => sum + row.clicks, 0);
  if (totalImpressions === 0) return 0;
  return totalClicks / totalImpressions;
};

const renderPostFunnel = (container, steps) => {
  if (!container) return;
  if (!Array.isArray(steps) || steps.length === 0) {
    container.innerHTML = "<p class=\"text-slate-500 text-sm\">No funnel data yet.</p>";
    return;
  }
  const maxValue = Math.max(...steps.map((step) => step.value || 0), 0);
  container.innerHTML = steps
    .map((step) => {
      const value = step.value || 0;
      const pct = maxValue > 0 ? Math.round((value / maxValue) * 100) : 0;
      return `
        <div>
          <div class="flex items-center justify-between text-sm text-slate-600">
            <span>${step.label}</span>
            <span>${value.toLocaleString()}</span>
          </div>
          <div class="mt-2 h-2 rounded-full bg-slate-100">
            <div class="h-2 rounded-full bg-skyline" style="width:${pct}%"></div>
          </div>
        </div>
      `;
    })
    .join("");
};

const renderPostTimeseries = (tbody, days) => {
  if (!tbody) return;
  if (!Array.isArray(days) || days.length === 0) {
    tbody.innerHTML = "<tr><td class=\"py-4 text-slate-500\" colspan=\"3\">No data for this range.</td></tr>";
    return;
  }
  tbody.innerHTML = days
    .map(
      (day) => `
        <tr>
          <td class="py-3 text-slate-700">${formatDate(day.day_date)}</td>
          <td class="py-3 text-slate-600">${Number(day.unique_impressions || 0).toLocaleString()}</td>
          <td class="py-3 text-slate-600">${Number(day.completes || 0).toLocaleString()}</td>
        </tr>
      `
    )
    .join("");
};

const renderPostPromoTable = (tbody, rows, promos) => {
  if (!tbody) return;
  if (!Array.isArray(rows) || rows.length === 0) {
    tbody.innerHTML = "<tr><td class=\"py-4 text-slate-500\" colspan=\"5\">No promo activity yet.</td></tr>";
    return;
  }
  const promoMap = new Map();
  promos.forEach((promo) => {
    const id = promo.id || promo.ID;
    if (id) promoMap.set(String(id), promo);
  });

  tbody.innerHTML = rows
    .map((row) => {
      const promo = promoMap.get(String(row.promo_id));
      const name = promo?.name || promo?.code || `Promo #${row.promo_id}`;
      const variantLabel = row.variant_id ? `Variant #${row.variant_id}` : "—";
      const impressions = Number(row.impressions || 0);
      const clicks = Number(row.clicks || 0);
      const ctr = impressions > 0 ? clicks / impressions : 0;
      return `
        <tr>
          <td class="py-3 font-semibold text-slate-800">${name}</td>
          <td class="py-3 text-slate-600">${variantLabel}</td>
          <td class="py-3 text-slate-600">${impressions.toLocaleString()}</td>
          <td class="py-3 text-slate-600">${clicks.toLocaleString()}</td>
          <td class="py-3 text-slate-700">${formatPercent(ctr)}</td>
        </tr>
      `;
    })
    .join("");
};

const formatPercent = (value) => {
  if (!Number.isFinite(value)) return "0%";
  return `${(value * 100).toFixed(1)}%`;
};

const initLoginButtons = () => {
  const loginButtons = [selectors.navLogin(), selectors.ctaLogin()].filter(Boolean);
  if (loginButtons.length === 0) return;

  loginButtons.forEach((btn) => {
    btn.addEventListener("click", () => {
      if (!window.google?.accounts?.id) {
        loadGoogleScript();
        setTimeout(() => promptGoogleLogin(), 800);
        return;
      }
      promptGoogleLogin();
    });
  });

  if (state.config?.googleClientId) {
    loadGoogleScript();
  }
};

const initSearchOverlay = () => {
  const trigger = document.getElementById("nav-search");
  if (!trigger) return;

  trigger.addEventListener("click", () => {
    const overlay = document.createElement("div");
    overlay.className = "fixed inset-0 bg-black/70 z-50 flex items-start justify-center pt-24";
    overlay.innerHTML = `
      <div class="w-full max-w-2xl bg-white rounded-3xl border border-slate-200 p-6">
        <div class="flex items-center gap-3">
          <input id="search-input" placeholder="Search posts..." class="flex-1 bg-transparent border border-slate-200 rounded-full px-4 py-2 text-ink" />
          <button id="search-close" class="text-slate-500">Close</button>
        </div>
        <div class="mt-4 space-y-3" id="search-results"></div>
      </div>
    `;
    document.body.appendChild(overlay);

    const input = overlay.querySelector("#search-input");
    const results = overlay.querySelector("#search-results");
    const closeBtn = overlay.querySelector("#search-close");

    closeBtn.addEventListener("click", () => overlay.remove());
    input.addEventListener("input", async () => {
      const query = input.value.trim();
      if (!query) {
        results.innerHTML = "";
        return;
      }
      try {
        const accessLevels = listingAccessLevels();
        const data = await fetchJSON(
          `${API.posts}?q=${encodeURIComponent(query)}&access_level=${encodeURIComponent(accessLevels)}`,
          { headers: authHeader() }
        );
        const items = data.items || [];
        results.innerHTML = items
          .map((item) => `<a href="/post/${item.slug}" class="block p-3 rounded-xl border border-slate-200 hover:border-skyline/60">${item.title}</a>`)
          .join("");
      } catch (err) {
        results.innerHTML = "<p class=\"text-slate-500\">No results.</p>";
      }
    });
  });
};

const initShareModal = () => {
  const trigger = document.getElementById("share-button");
  if (!trigger) return;

  trigger.addEventListener("click", () => {
    const overlay = document.createElement("div");
    overlay.className = "fixed inset-0 bg-black/70 z-50 flex items-center justify-center";
    overlay.innerHTML = `
      <div class="bg-white rounded-3xl border border-slate-200 p-6 w-full max-w-md">
        <h3 class="font-display text-xl text-ink">Share this post</h3>
        <div class="mt-4 grid gap-3">
          <button id="share-copy" class="w-full rounded-full border border-slate-200 py-2 text-slate-700">Copy link</button>
          <a href="https://twitter.com/intent/tweet?url=${encodeURIComponent(window.location.href)}" target="_blank" class="w-full rounded-full border border-slate-200 py-2 text-center text-slate-700">Share on Twitter</a>
          <a href="https://www.linkedin.com/sharing/share-offsite/?url=${encodeURIComponent(window.location.href)}" target="_blank" class="w-full rounded-full border border-slate-200 py-2 text-center text-slate-700">Share on LinkedIn</a>
        </div>
        <button id="share-close" class="mt-4 text-slate-500">Close</button>
      </div>
    `;
    document.body.appendChild(overlay);
    overlay.querySelector("#share-close").addEventListener("click", () => overlay.remove());
    overlay.querySelector("#share-copy").addEventListener("click", async () => {
      await navigator.clipboard.writeText(window.location.href);
      showToast("Link copied");
    });
  });
};

const loadGoogleScript = () => {
  if (document.getElementById("google-identity")) return;
  const script = document.createElement("script");
  script.id = "google-identity";
  script.src = "https://accounts.google.com/gsi/client";
  script.async = true;
  script.defer = true;
  script.onload = () => initGoogleIdentity();
  document.head.appendChild(script);
};

const initGoogleIdentity = () => {
  if (!state.config?.googleClientId || !window.google?.accounts?.id) return;
  window.google.accounts.id.initialize({
    client_id: state.config.googleClientId,
    callback: handleGoogleCredential,
    use_fedcm_for_prompt: !isLocalhost(),
    ux_mode: "popup"
  });
};

const promptGoogleLogin = () => {
  if (!window.google?.accounts?.id) return;
  window.google.accounts.id.prompt();
};

const isLocalhost = () => {
  const host = window.location.hostname;
  return host === "localhost" || host === "127.0.0.1";
};

const handleGoogleCredential = async (response) => {
  if (!response?.credential) return;
  try {
    const payload = await fetchJSON(API.authLogin, {
      method: "POST",
      body: JSON.stringify({ googleToken: response.credential, anon_id: state.anonId })
    });
    state.token = payload.token;
    localStorage.setItem("jwt", state.token);
    state.user = payload.user || null;
    setStoredUser(state.user);
    window.location.reload();
  } catch (err) {
    console.error(err);
  }
};

const getToken = () => localStorage.getItem("jwt");
const clearToken = () => {
  localStorage.removeItem("jwt");
  clearStoredUser();
  clearStoredEntitlement();
  state.user = null;
  state.entitlement = null;
};

const getStoredUser = () => {
  const raw = localStorage.getItem("user");
  if (!raw) return null;
  try {
    return JSON.parse(raw);
  } catch (err) {
    console.error(err);
    return null;
  }
};

const setStoredUser = (user) => {
  if (!user) {
    clearStoredUser();
    return;
  }
  localStorage.setItem("user", JSON.stringify(user));
};

const clearStoredUser = () => localStorage.removeItem("user");

const getStoredEntitlement = () => {
  const raw = localStorage.getItem("entitlement");
  if (!raw) return null;
  try {
    return JSON.parse(raw);
  } catch (err) {
    console.error(err);
    return null;
  }
};

const setStoredEntitlement = (entitlement) => {
  if (!entitlement) {
    clearStoredEntitlement();
    return;
  }
  localStorage.setItem("entitlement", JSON.stringify(entitlement));
};

const clearStoredEntitlement = () => localStorage.removeItem("entitlement");

const getOrCreateAnonId = () => {
  const key = "anon_id";
  let id = localStorage.getItem(key);
  if (!id) {
    id = crypto.randomUUID();
    localStorage.setItem(key, id);
  }
  return id;
};

const openCheckout = async (planCode) => {
  await loadRazorpay();
  const data = await fetchJSON(API.createOrder, {
    method: "POST",
    body: JSON.stringify({ plan_code: planCode })
  });

  const options = {
    key: data.razorpay_key_id,
    amount: data.amount,
    currency: data.currency,
    name: state.config?.siteName || "Explore",
    order_id: data.order_id,
    handler: async (response) => {
      await fetchJSON(API.confirm, {
        method: "POST",
        body: JSON.stringify({
          razorpay_order_id: response.razorpay_order_id,
          razorpay_payment_id: response.razorpay_payment_id,
          razorpay_signature: response.razorpay_signature
        })
      });
      await loadUser();
      updateNavState();
      showToast("Payment successful. Welcome back!");
      setTimeout(() => window.location.reload(), 1200);
    }
  };

  const rzp = new window.Razorpay(options);
  rzp.open();
};

const loadRazorpay = () => {
  return new Promise((resolve) => {
    if (window.Razorpay) {
      resolve();
      return;
    }
    const script = document.createElement("script");
    script.src = "https://checkout.razorpay.com/v1/checkout.js";
    script.onload = resolve;
    document.body.appendChild(script);
  });
};

const showLoginGate = () => {
  showOverlay({
    title: "Continue reading",
    message: "Sign in with Google to unlock the full post.",
    primary: { text: "Continue with Google", action: promptGoogleLogin }
  });
};

const showPaywallGate = () => {
  showOverlay({
    title: "Members only",
    message: "Subscribe to unlock full access and continue learning.",
    primary: { text: "View pricing", action: () => (window.location.href = "/pricing") }
  });
};

const showOverlay = ({ title, message, primary }) => {
  const overlay = document.createElement("div");
  overlay.className = "fixed inset-0 bg-black/70 flex items-center justify-center z-50";
  overlay.innerHTML = `
    <div class="max-w-md w-full bg-white text-ink rounded-3xl p-8 border border-slate-200">
      <h3 class="font-display text-2xl">${title}</h3>
      <p class="mt-3 text-slate-600">${message}</p>
      <button class="mt-6 w-full rounded-full bg-skyline text-white py-3 font-semibold" id="overlay-primary">${primary.text}</button>
    </div>
  `;
  document.body.appendChild(overlay);
  overlay.querySelector("#overlay-primary").addEventListener("click", () => {
    primary.action();
    overlay.remove();
  });
};

const showToast = (message) => {
  const toast = document.createElement("div");
  toast.className = "fixed bottom-6 right-6 bg-white text-ink px-4 py-3 rounded-full border border-slate-200 shadow-lg";
  toast.textContent = message;
  document.body.appendChild(toast);
  setTimeout(() => toast.remove(), 2600);
};

const initPromoSlots = (pageType, postId) => {
  if (!postId) return;
  const slots = ["INLINE", "BOTTOM_CARD"];

  slots.forEach((slot) => {
    decidePromo(slot, postId).then((promo) => {
      if (!promo) return;
      renderPromo(slot, promo, postId);
    });
  });
};

const decidePromo = async (slot, postId) => {
  const url = `${API.promos}/decide?slot=${slot}&post_id=${postId}&anon_id=${state.anonId}`;
  const res = await fetchJSON(url);
  if (res.promo) {
    res.promo.decision_id = res.decision_id;
  }
  return res.promo;
};

const renderPromo = (slot, promo, postId) => {
  const body = selectors.postBody();
  if (!body) return;

  const card = document.createElement("div");
  card.className = "rounded-2xl border border-slate-200 bg-white/90 p-6 my-8";
  card.innerHTML = `
    <p class="text-xs uppercase tracking-wide text-slate-500">${slot.replace("_", " ")}</p>
    <h3 class="mt-2 font-display text-xl text-ink">${promo.headline}</h3>
    <p class="mt-2 text-slate-600">${promo.body}</p>
    <button class="mt-4 px-4 py-2 rounded-full bg-ember text-white font-semibold">${promo.cta_text}</button>
  `;

  const button = card.querySelector("button");
  button.addEventListener("click", () => {
    logPromo("click", promo, postId);
    if (promo.cta_action === "OPEN_PRICING") {
      window.location.href = "/pricing";
      return;
    }
    if (promo.cta_action === "START_CHECKOUT") {
      const plan = promo.cta_payload?.plan_default || "monthly";
      openCheckout(plan);
    }
  });

  if (slot === "INLINE") {
    const heading = body.querySelector("h2");
    if (heading) {
      heading.after(card);
    } else {
      const paragraphs = body.querySelectorAll("p");
      if (paragraphs.length >= 3) {
        paragraphs[2].after(card);
      } else {
        body.appendChild(card);
      }
    }
  } else {
    body.appendChild(card);
  }

  logPromo("impression", promo, postId);
};

const logPromo = async (type, promo, postId) => {
  const payload = {
    decision_id: promo.decision_id,
    promo_id: promo.promo_id,
    variant_id: promo.variant_id,
    post_id: postId,
    anon_id: state.anonId
  };
  await fetchJSON(`${API.promos}/${type}`, {
    method: "POST",
    body: JSON.stringify(payload)
  });
};

const trackPostEngagement = (postId, options = {}) => {
  if (!postId) return;
  const promoEligible = options.promoEligible === true;
  const milestones = [25, 50, 75, 90];
  const seen = new Set();
  const timeMilestones = [15, 45, 90];
  let scrollComplete = false;
  let timeComplete = false;
  let completionSent = false;

  sendEvents([{ type: "post_open", entity_type: "POST", entity_id: postId, meta: {} }]);

  const maybeComplete = async () => {
    if (completionSent || !scrollComplete || !timeComplete) return;
    completionSent = true;
    try {
      await sendEvents([{ type: "post_complete", entity_type: "POST", entity_id: postId, meta: {} }]);
    } catch (err) {
      console.error(err);
    }

    if (!promoEligible) return;
    try {
      const promo = await decidePromo("MODAL_ON_COMPLETE", postId);
      if (promo) {
        showCompletionModal(promo, postId);
      }
    } catch (err) {
      console.error(err);
    }
  };

  const onScroll = () => {
    const scrolled = window.scrollY + window.innerHeight;
    const height = document.documentElement.scrollHeight;
    const pct = Math.round((scrolled / height) * 100);
    if (pct >= 75 && !scrollComplete) {
      scrollComplete = true;
      maybeComplete();
    }
    milestones.forEach((m) => {
      if (pct >= m && !seen.has(`scroll_${m}`)) {
        seen.add(`scroll_${m}`);
        sendEvents([{ type: "scroll_depth", entity_type: "POST", entity_id: postId, meta: { pct: m } }]);
      }
    });
  };
  window.addEventListener("scroll", onScroll);

  timeMilestones.forEach((sec) => {
    setTimeout(() => {
      sendEvents([{ type: "time_on_page", entity_type: "POST", entity_id: postId, meta: { sec } }]);
      if (sec === 45 && !timeComplete) {
        timeComplete = true;
        maybeComplete();
      }
    }, sec * 1000);
  });
};

const showCompletionModal = (promo, postId) => {
  const overlay = document.createElement("div");
  overlay.className = "fixed inset-0 bg-black/70 flex items-center justify-center z-50";
  overlay.innerHTML = `
    <div class="max-w-md w-full bg-white text-ink rounded-3xl p-8 border border-slate-200">
      <p class="text-xs uppercase tracking-wide text-slate-500">Completion</p>
      <h3 class="mt-3 font-display text-2xl">${promo.headline}</h3>
      <p class="mt-3 text-slate-600">${promo.body}</p>
      <button class="mt-6 w-full rounded-full bg-skyline text-white py-3 font-semibold" data-primary>${promo.cta_text}</button>
      <button class="mt-3 w-full text-slate-500" data-close>Maybe later</button>
    </div>
  `;
  document.body.appendChild(overlay);

  logPromo("impression", promo, postId);

  overlay.querySelector("[data-primary]").addEventListener("click", () => {
    logPromo("click", promo, postId);
    if (promo.cta_action === "OPEN_PRICING") {
      window.location.href = "/pricing";
    }
    if (promo.cta_action === "START_CHECKOUT") {
      const plan = promo.cta_payload?.plan_default || "monthly";
      openCheckout(plan);
    }
    overlay.remove();
  });

  overlay.querySelector("[data-close]").addEventListener("click", () => overlay.remove());
};

const sendEvents = async (events) => {
  if (!events || events.length === 0) return;
  try {
    await fetchJSON(API.events, {
      method: "POST",
      body: JSON.stringify({ anon_id: state.anonId, events })
    });
  } catch (err) {
    console.error(err);
  }
};

const animateIn = (elements) => {
  if (!window.motion) return;
  [...elements].forEach((el, index) => {
    window.motion.animate(el, { opacity: [0, 1], transform: ["translateY(12px)", "translateY(0)"] }, { duration: 0.4, delay: index * 0.05 });
  });
};

document.addEventListener("DOMContentLoaded", init);
