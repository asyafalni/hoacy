import { ref } from 'vue';
import { api } from '../lib/api.js';

// The public /sum page's data: cluster-wide aggregates computed by the server
// (backend/internal/tagihan RingkasanPublik). No sign-in, and by construction no
// house rows — nothing per-house ever reaches this page (PDP — see RingkasanPublik.vue).
const data = ref(null);
const loading = ref(false);

async function muat() {
  loading.value = true;
  try {
    data.value = (await api('/api/publik')).data;
  } catch { /* api() put it in apiError — App.vue shows the banner */
  } finally {
    loading.value = false;
  }
}

export function usePublik() {
  return { data, loading, muat };
}
