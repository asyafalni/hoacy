import { ref, computed } from 'vue';

const SHEET = import.meta.env.VITE_SHEET_ID;
const gviz = (tab) =>
  `https://docs.google.com/spreadsheets/d/${SHEET}/gviz/tq?tqx=out:json&sheet=${tab}`;

/** gviz wraps its JSON in a JS callback — strip it. */
function parse(text) {
  const json = JSON.parse(text.slice(text.indexOf('{'), text.lastIndexOf('}') + 1));
  return json.table.rows.map((r) => (r.c || []).map((c) => (c ? c.v : null)));
}

const rows = ref([]);
const loading = ref(true);
const error = ref(null);
const optimistic = ref([]);   // rows submitted this session, not yet in the sheet

export function useSheet() {
  async function load() {
    loading.value = true;
    try {
      const res = await fetch(gviz('API'));
      rows.value = parse(await res.text());
      error.value = null;
    } catch (e) {
      error.value = e;
    } finally {
      loading.value = false;
    }
  }

  // key/value block (cols A,B) -> { kas_tunai, rekening, tunggakan_total, ... }
  const meta = computed(() =>
    Object.fromEntries(rows.value.filter((r) => r[0]).map((r) => [r[0], r[1]])));

  // per-house block (cols D..Z)
  const rumah = computed(() =>
    rows.value.filter((r) => r[3]).map((r) => ({
      // alamat = cluster + blok + '-' + rumah, e.g. N7-09
      alamat: r[3], nama: r[4], telp: r[5], luas: r[6], tipe: r[7],
      tarif: r[8], tunggakan: r[9],
      status: r.slice(10, 22),   // 12 months of "Lunas|Sebagian|Pending|Belum|-"
      cluster: r[22], blok: r[23], rumah: r[24],
      // months (1-12) of next year already submitted — see docs/sheets-schema.md §6
      mukaTahunDepan: r[25] ? String(r[25]).split(',').map(Number).filter(Boolean) : [],
    })));

  /** Local echo so the satpam sees the row immediately after submitting. */
  function echo(rec) { optimistic.value.push({ ...rec, at: Date.now() }); }

  return { load, rows, loading, error, meta, rumah, optimistic, echo };
}
