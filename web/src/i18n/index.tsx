import {
  createContext,
  useContext,
  createSignal,
  createMemo,
  createEffect,
  type Accessor,
  type ParentComponent,
} from "solid-js";
	import * as i18n from "@solid-primitives/i18n";
	import { LOCALE, type Locale } from "~/domain";
	import { dict as en_dict, type RawDictionary, type Dictionary } from "./en";
	import { dict as id_dict } from "./id";
	import { useAuth, type AuthContextValue } from "~/auth";

	const dicts: Record<Locale, Dictionary> = {
	[LOCALE.ENGLISH]: i18n.flatten(en_dict),
	[LOCALE.INDONESIAN]: i18n.flatten(id_dict),
};

function detectLocale(email?: string | null): Locale {
	if (typeof window === "undefined") return LOCALE.INDONESIAN;
	const key = email ? `warehouse-ui-locale:${email}` : "warehouse-ui-locale";
	const stored = localStorage.getItem(key);
	if (stored === LOCALE.ENGLISH || stored === LOCALE.INDONESIAN) return stored;
	const globalStored = localStorage.getItem("warehouse-ui-locale");
	if (globalStored === LOCALE.ENGLISH || globalStored === LOCALE.INDONESIAN) return globalStored;
	if (typeof navigator !== "undefined" && navigator.language?.startsWith(LOCALE.INDONESIAN)) return LOCALE.INDONESIAN;
	return LOCALE.INDONESIAN;
}

export interface I18nContextValue {
  locale: Accessor<Locale>;
  setLocale: (l: Locale) => void;
  t: i18n.Translator<Dictionary>;
  intlLocale: Accessor<string>;
  formatDate: (date: Date | string, options?: Intl.DateTimeFormatOptions) => string;
  formatRelativeTime: (date: Date, now: number) => string;
}

const I18nContext = createContext<I18nContextValue>();

export const I18nProvider: ParentComponent = (props) => {
  let auth: AuthContextValue | undefined;
  try {
    auth = useAuth();
  } catch {
    // Outside AuthProvider fallback
  }
  const email = () => auth?.user()?.email;
  const [locale, setLocaleSignal] = createSignal<Locale>(detectLocale(email()));

  let lastUserEmail: string | null | undefined = undefined;

  // Sync initial locale when user logs in / loads session
  createEffect(() => {
    const user = auth?.user();
    if (user === undefined) return; // Still loading session

    const currentEmail = user?.email ?? null;
    // Only synchronize from backend user if the user session changed (e.g. initial load or account switch)
    if (currentEmail !== lastUserEmail) {
      lastUserEmail = currentEmail;
		if (user && (user.locale === LOCALE.ENGLISH || user.locale === LOCALE.INDONESIAN)) {
        setLocaleSignal(user.locale);
        localStorage.setItem(user.email ? `warehouse-ui-locale:${user.email}` : "warehouse-ui-locale", user.locale);
        if (typeof document !== "undefined") {
          document.documentElement.lang = user.locale;
        }
      } else {
        const stored = detectLocale(currentEmail);
        setLocaleSignal(stored);
        if (typeof document !== "undefined") {
          document.documentElement.lang = stored;
        }
      }
    }
  });

  const setLocale = (l: Locale) => {
    setLocaleSignal(l);
    auth?.setUserLocale?.(l);
    const userEmail = email();
    localStorage.setItem(userEmail ? `warehouse-ui-locale:${userEmail}` : "warehouse-ui-locale", l);
    localStorage.setItem("warehouse-ui-locale", l);
    if (typeof document !== "undefined") {
      document.documentElement.lang = l;
    }

    // Persist to backend if authenticated
    if (userEmail) {
      fetch("/api/v1/user/locale", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ locale: l }),
      }).catch(() => {});
    }
  };

  const dict = createMemo(() => dicts[locale()]);
  const t = i18n.translator(dict, i18n.resolveTemplate);
	const intlLocale = createMemo(() => (locale() === LOCALE.INDONESIAN ? "id-ID" : "en-US"));

  const formatDate = (date: Date | string, options?: Intl.DateTimeFormatOptions): string => {
    const d = typeof date === "string" ? new Date(date) : date;
    if (isNaN(d.getTime())) return "-";
    return d.toLocaleDateString(
      intlLocale(),
      options ?? { year: "numeric", month: "short", day: "numeric" }
    );
  };

  const formatRelativeTime = (date: Date, now: number): string => {
    const diffSec = Math.floor((now - date.getTime()) / 1000);
    if (diffSec < 60) return t("sync.justNow");
    const diffMin = Math.floor(diffSec / 60);
    if (diffMin < 60) return t("sync.minAgo", { count: diffMin });
    const diffHour = Math.floor(diffMin / 60);
    if (diffHour < 24) return t("sync.hAgo", { count: diffHour });
    const diffDay = Math.floor(diffHour / 24);
    return t("sync.dAgo", { count: diffDay });
  };

  if (typeof document !== "undefined") {
    document.documentElement.lang = locale();
  }

  const value: I18nContextValue = {
    locale,
    setLocale,
    t,
    intlLocale,
    formatDate,
    formatRelativeTime,
  };

  return <I18nContext.Provider value={value}>{props.children}</I18nContext.Provider>;
};

export function useI18n(): I18nContextValue {
  const ctx = useContext(I18nContext);
  if (!ctx) {
    throw new Error("useI18n must be used within I18nProvider");
  }
  return ctx;
}

export type { Locale, Dictionary, RawDictionary };
