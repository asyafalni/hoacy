import { ref, computed } from 'vue';

// submitTunai's response is opaque (no-cors) — we never actually know if a Form
// POST landed. Persisting each attempt here means a network hiccup at the pos,
// or a reload/crash, doesn't silently lose uncollected-looking cash:
// reconcile() clears a transaction only once the next Sheet refetch shows
// every month in it as sah for that house. A retry that duplicates a POST
// which actually did land is harmless — the Sheet marks the second copy
// `dobel` and counts it once (docs/sheets-schema.md `D-Pembayaran`).
const KEY = 'iuran.pending.pos.v2';

function read() {
  try { return JSON.parse(localStorage.getItem(KEY) || '[]'); } catch { return []; }
}

const pending = ref(read());
const persist = () => {
  try { localStorage.setItem(KEY, JSON.stringify(pending.value)); } catch { /* private mode */ }
};

export function usePendingSync() {
  /** rec: { alamat, items: [{ periode, nominal }], petugas } */
  function add(rec) {
    pending.value = [...pending.value, { ...rec, id: `${Date.now()}-${Math.random().toString(36).slice(2, 7)}` }];
    persist();
  }
  function reconcile(rumahList) {
    pending.value = pending.value.filter((p) => {
      const h = rumahList.find((r) => r.alamat === p.alamat);
      // house gone (deactivated) → nothing left to confirm against
      return h && !p.items.every((i) => h.lunas.has(i.periode));
    });
    persist();
  }
  return { pending: computed(() => pending.value), add, reconcile };
}
