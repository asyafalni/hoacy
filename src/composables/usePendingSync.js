import { ref, computed } from 'vue';

// submitPembayaran's response is opaque (no-cors) — we never actually know if a
// Form POST landed. Before this, a failed submission just vanished: the optimistic
// echo looked identical to a real success, so a network hiccup at the pos meant
// silently uncollected cash. Persisting the attempt here means a reload/crash
// doesn't lose track of it either — reconcile() clears an entry once the next
// Sheet refetch shows that house/month is no longer "Belum".
const KEY = 'iuran.pending.pos';

function load() {
  try { return JSON.parse(localStorage.getItem(KEY) || '[]'); } catch { return []; }
}

const pending = ref(load());
const persist = () => localStorage.setItem(KEY, JSON.stringify(pending.value));

export function usePendingSync() {
  function add(rec) {
    pending.value = [...pending.value, { ...rec, id: `${Date.now()}-${Math.random().toString(36).slice(2, 7)}` }];
    persist();
  }
  function remove(id) {
    pending.value = pending.value.filter((p) => p.id !== id);
    persist();
  }
  function reconcile(rumahList) {
    pending.value = pending.value.filter((p) => {
      const h = rumahList.find((r) => r.alamat === p.noRumah);
      return h?.status?.[p.bulan - 1] === 'Belum';   // still unpaid per the Sheet -> keep it flagged
    });
    persist();
  }
  return { pending: computed(() => pending.value), add, remove, reconcile };
}
