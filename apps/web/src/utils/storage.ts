export function stored(key: string, fallback: string): string {
  try {
    return localStorage.getItem(key) ?? fallback;
  } catch {
    return fallback;
  }
}

export function persist(key: string, value: string): void {
  try {
    localStorage.setItem(key, value);
  } catch {
    /* storage unavailable: preferences stay in memory */
  }
}

export function removeStored(key: string): void {
  try {
    localStorage.removeItem(key);
  } catch {
    /* ignore */
  }
}

// One-time migration from legacy openportal.* keys to openscanner.* keys.
export function migrateStorage(): void {
  for (const name of ['recent_opt_in', 'recent_searches', 'safe_search', 'locale', 'theme']) {
    try {
      const next = `openscanner.${name}`;
      if (localStorage.getItem(next) === null) {
        const legacy = localStorage.getItem(`openportal.${name}`);
        if (legacy !== null) localStorage.setItem(next, legacy);
      }
      localStorage.removeItem(`openportal.${name}`);
    } catch {
      /* ignore */
    }
  }
}
