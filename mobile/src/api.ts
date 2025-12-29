import { CONFIG } from "./config";
import type { Session } from "./storage";

export async function exchangeGoogleTokenForAppSession(googleIdToken: string): Promise<Session> {
  const res = await fetch(`${CONFIG.API_BASE_URL}/api/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ googleToken: googleIdToken }),
  });
  const data = (await res.json().catch(() => ({}))) as any;
  if (!res.ok) {
    throw new Error(data?.error || "Login failed");
  }
  return { token: String(data.token || ""), user: data.user || undefined };
}

export async function verifyGooglePlaySubscriptionPurchase(params: {
  appToken: string;
  productId: string;
  purchaseToken: string;
}): Promise<void> {
  const res = await fetch(`${CONFIG.API_BASE_URL}/api/billing/googleplay/verify`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${params.appToken}`,
    },
    body: JSON.stringify({
      productId: params.productId,
      purchaseToken: params.purchaseToken,
    }),
  });
  const data = (await res.json().catch(() => ({}))) as any;
  if (!res.ok) {
    throw new Error(data?.error || "Purchase verification failed");
  }
}

