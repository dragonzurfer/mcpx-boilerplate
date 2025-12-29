import AsyncStorage from "@react-native-async-storage/async-storage";

export type AppUser = {
  id: number;
  name?: string;
  email?: string;
};

export type Session = {
  token: string;
  user?: AppUser;
};

const SESSION_KEY = "mcpx.session.v1";

export async function loadSession(): Promise<Session | null> {
  const raw = await AsyncStorage.getItem(SESSION_KEY);
  if (!raw) return null;
  try {
    const parsed = JSON.parse(raw) as Session;
    if (!parsed?.token) return null;
    return parsed;
  } catch {
    return null;
  }
}

export async function saveSession(session: Session): Promise<void> {
  await AsyncStorage.setItem(SESSION_KEY, JSON.stringify(session));
}

export async function clearSession(): Promise<void> {
  await AsyncStorage.removeItem(SESSION_KEY);
}
