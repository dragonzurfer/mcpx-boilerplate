export const CONFIG = {
  WEBAPP_URL: "https://app.mcpx.in",
  API_BASE_URL: "https://app.mcpx.in",

  GOOGLE_SIGNIN: {
    // OAuth client ID (type: Web application). This is the same value used by the web UI.
    WEB_CLIENT_ID: "",
  },

  PLAY_SUBSCRIPTIONS: {
    // Web plan code -> Play subscription product ID
    PLAN_TO_PRODUCT_ID: {
      pro: "app_pro_monthly",
    } as Record<string, string>,
  },
} as const;
