import React, { useEffect, useMemo, useRef, useState } from "react";
import { Platform, StatusBar } from "react-native";
import { GoogleSignin } from "@react-native-google-signin/google-signin";
import * as RNIap from "react-native-iap";
import { WebView } from "react-native-webview";
import { SafeAreaProvider, SafeAreaView } from "react-native-safe-area-context";

import { exchangeGoogleTokenForAppSession, verifyGooglePlaySubscriptionPurchase } from "./src/api";
import { CONFIG } from "./src/config";
import { clearSession, loadSession, saveSession, type Session } from "./src/storage";

type WebMessage =
  | { type: "google_signin" }
  | { type: "iap_subscribe"; planCode?: string }
  | { type: "auth"; token: string; user?: any }
  | { type: "logout" }
  | { type: string; [k: string]: any };

function extractString(v: unknown): string {
  return typeof v === "string" ? v : "";
}

async function getAndroidOfferTokenForSku(sku: string): Promise<string> {
  const tryGetSubscriptions = async (): Promise<any[]> => {
    const getSubscriptions: any = (RNIap as any).getSubscriptions;
    if (typeof getSubscriptions !== "function") return [];
    try {
      return (await getSubscriptions([sku])) || [];
    } catch {
      // some versions accept { skus: [...] }
    }
    try {
      return (await getSubscriptions({ skus: [sku] })) || [];
    } catch {
      return [];
    }
  };

  const subs = await tryGetSubscriptions();
  const sub = Array.isArray(subs) ? subs.find((s) => extractString(s?.productId) === sku) : null;

  const offerDetails =
    (sub as any)?.subscriptionOfferDetails ||
    (sub as any)?.subscriptionOfferDetailsAndroid ||
    (sub as any)?.offers ||
    [];

  const offerToken =
    extractString(offerDetails?.[0]?.offerToken) ||
    extractString(offerDetails?.[0]?.offerTokenAndroid) ||
    extractString(offerDetails?.[0]?.token);

  if (!offerToken) {
    throw new Error(
      "Subscription offerToken not found. Ensure the product has an active base plan/offer and install the app from Play internal testing (not USB install)."
    );
  }

  return offerToken;
}

async function signInWithGoogle(): Promise<string> {
  const webClientId = CONFIG.GOOGLE_SIGNIN.WEB_CLIENT_ID;
  if (!webClientId) {
    throw new Error("Missing Google web client id (CONFIG.GOOGLE_SIGNIN.WEB_CLIENT_ID)");
  }
  GoogleSignin.configure({ webClientId });
  await GoogleSignin.hasPlayServices({ showPlayServicesUpdateDialog: true });
  const res = await GoogleSignin.signIn();

  // v13 returns { type: 'success', data: User }, older versions returned User directly.
  if ((res as any)?.type && (res as any).type !== "success") {
    if ((res as any).type === "cancelled") throw new Error("Sign-in cancelled");
    throw new Error("Sign-in failed");
  }
  const maybeUser = (res as any)?.type === "success" ? (res as any).data : res;
  const idToken = extractString((maybeUser as any)?.idToken);
  if (!idToken) throw new Error("Missing idToken from Google Sign-In");
  return idToken;
}

export default function App() {
  const webViewRef = useRef<WebView>(null);
  const [session, setSession] = useState<Session | null>(null);

  const sessionRef = useRef<Session | null>(null);
  useEffect(() => {
    sessionRef.current = session;
  }, [session]);

  const allowedProducts = useMemo(() => {
    return new Set(Object.values(CONFIG.PLAY_SUBSCRIPTIONS.PLAN_TO_PRODUCT_ID));
  }, []);

  const postToWeb = (payload: any) => {
    try {
      webViewRef.current?.postMessage(JSON.stringify(payload || {}));
    } catch {
      // ignore
    }
  };

  useEffect(() => {
    loadSession()
      .then((s) => setSession(s))
      .catch(() => setSession(null));
  }, []);

  useEffect(() => {
    const init = async () => {
      try {
        await (RNIap as any).initConnection?.();
        if (Platform.OS === "android") {
          await (RNIap as any).flushFailedPurchasesCachedAsPendingAndroid?.();
        }
      } catch {
        // ignore: app can still run without billing in dev builds
      }
      return () => {
        try {
          (RNIap as any).endConnection?.();
        } catch {
          // ignore
        }
      };
    };
    const cleanupPromise = init();
    return () => {
      void cleanupPromise.then((cleanup) => cleanup?.());
    };
  }, []);

  useEffect(() => {
    const purchaseSub = (RNIap as any).purchaseUpdatedListener?.(async (purchase: any) => {
      const productId = extractString(purchase?.productId || purchase?.productIdAndroid);
      const purchaseToken = extractString(purchase?.purchaseToken || purchase?.purchaseTokenAndroid);
      const appToken = extractString(sessionRef.current?.token);

      if (!productId || !purchaseToken || !allowedProducts.has(productId)) {
        return;
      }

      try {
        if (!appToken) throw new Error("Not signed in");
        await verifyGooglePlaySubscriptionPurchase({ appToken, productId, purchaseToken });
        postToWeb({ type: "iap_result", ok: true });
        await (RNIap as any).finishTransaction?.({ purchase, isConsumable: false });
      } catch (err: any) {
        postToWeb({ type: "iap_result", ok: false, error: err?.message || "Purchase verification failed" });
      }
    });

    const errorSub = (RNIap as any).purchaseErrorListener?.((err: any) => {
      postToWeb({ type: "iap_result", ok: false, error: err?.message || "Purchase failed" });
    });

    return () => {
      try {
        purchaseSub?.remove?.();
      } catch {
        // ignore
      }
      try {
        errorSub?.remove?.();
      } catch {
        // ignore
      }
    };
  }, [allowedProducts]);

  useEffect(() => {
    const appToken = extractString(session?.token);
    if (!appToken) return;

    const restore = async () => {
      try {
        const purchases: any[] = (await (RNIap as any).getAvailablePurchases?.()) || [];
        for (const p of purchases) {
          const productId = extractString(p?.productId || p?.productIdAndroid);
          const purchaseToken = extractString(p?.purchaseToken || p?.purchaseTokenAndroid);
          if (!productId || !purchaseToken || !allowedProducts.has(productId)) continue;
          await verifyGooglePlaySubscriptionPurchase({ appToken, productId, purchaseToken });
        }
      } catch {
        // ignore
      }
    };

    void restore();
  }, [allowedProducts, session?.token]);

  const handleWebMessage = async (raw: string) => {
    let msg: WebMessage | null = null;
    try {
      msg = JSON.parse(raw) as WebMessage;
    } catch {
      return;
    }
    if (!msg || typeof msg !== "object" || typeof msg.type !== "string") return;

    if (msg.type === "auth") {
      const token = extractString((msg as any).token);
      if (!token) return;
      const next: Session = { token, user: (msg as any).user || undefined };
      setSession(next);
      await saveSession(next);
      return;
    }

    if (msg.type === "logout") {
      setSession(null);
      await clearSession();
      try {
        await GoogleSignin.signOut();
      } catch {
        // ignore
      }
      return;
    }

    if (msg.type === "google_signin") {
      try {
        const googleIdToken = await signInWithGoogle();
        const next = await exchangeGoogleTokenForAppSession(googleIdToken);
        setSession(next);
        await saveSession(next);
        postToWeb({ type: "auth", token: next.token, user: next.user || null });
      } catch (err: any) {
        postToWeb({ type: "auth_error", error: err?.message || "Sign-in failed" });
      }
      return;
    }

    if (msg.type === "iap_subscribe") {
      const planCode = extractString((msg as any).planCode) || "paid";
      const productId = CONFIG.PLAY_SUBSCRIPTIONS.PLAN_TO_PRODUCT_ID[planCode];
      if (!productId) {
        postToWeb({ type: "iap_result", ok: false, error: `Unknown plan: ${planCode}` });
        return;
      }
      try {
        const requestSubscription: any = (RNIap as any).requestSubscription;
        if (typeof requestSubscription !== "function") throw new Error("Billing not available in this build");
        if (Platform.OS === "android") {
          const offerToken = await getAndroidOfferTokenForSku(productId);
          await requestSubscription({ subscriptionOffers: [{ sku: productId, offerToken }] });
        } else {
          await requestSubscription({ sku: productId });
        }
      } catch (err: any) {
        postToWeb({ type: "iap_result", ok: false, error: err?.message || "Failed to start purchase" });
      }
    }
  };

  return (
    <SafeAreaProvider>
      <SafeAreaView style={{ flex: 1, backgroundColor: "#0b6b6f" }} edges={["top", "bottom"]}>
        <StatusBar translucent={false} backgroundColor="#0b6b6f" barStyle="light-content" />
        <WebView
          ref={webViewRef}
          style={{ flex: 1 }}
          source={{ uri: CONFIG.WEBAPP_URL }}
          javaScriptEnabled
          domStorageEnabled
          originWhitelist={["*"]}
          thirdPartyCookiesEnabled
          sharedCookiesEnabled
          onLoadEnd={() => {
            if (session?.token) {
              postToWeb({ type: "auth", token: session.token, user: session.user || null });
            }
          }}
          onMessage={(e) => {
            const raw = extractString(e.nativeEvent?.data);
            void handleWebMessage(raw);
          }}
        />
      </SafeAreaView>
    </SafeAreaProvider>
  );
}
