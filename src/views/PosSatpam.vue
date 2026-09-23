<script setup vapor>
import { ref, computed, watch } from 'vue';
import { useSheet } from '../composables/useSheet';
import { usePendingSync } from '../composables/usePendingSync';
import { useScrollLock } from '../composables/useScrollLock';
import { BULAN, rupiah, rupiahPendek, BLOK_LIST, BLOK_WARNA_DEFAULT } from '../lib/tariff';
import { submitTunai } from '../lib/forms';
import { labelBulan } from '../lib/tagihan';
import Card from '../components/ui/Card.vue';
import Tag from '../components/ui/Tag.vue';
import Button from '../components/ui/Button.vue';
import PinGate from '../components/PinGate.vue';

const { rumah, blokWarna, satpamList, TAHUN, load } = useSheet();
const { pending, add: addPending, reconcile } = usePendingSync();
watch(rumah, (list) => { if (list.length) reconcile(list); }, { immediate: true });
// light tint of the block's color behind the house number — dark text stays legible
const badgeStyle = (h) => {
  const hex = blokWarna.value[String(h.blok)] || BLOK_WARNA_DEFAULT[String(h.blok)] || '#c0b6a5';
  return `background:color-mix(in srgb, ${hex} 22%, white)`;
};
const PIN = import.meta.env.VITE_PIN_POS || '';
const PER_PAGE = 10;

// The Pos PIN is shared by every satpam — it only gates the screen. Each payment
// still needs to say which one of them actually took the cash (Petugas tab, peran
// "satpam" — docs/sheets-schema.md `Petugas`), so the app makes them pick their name once
// per device and remembers it, instead of hardcoding a single name for everyone.
const petugasAktif = ref(localStorage.getItem('iuran.petugas.pos') || '');
function pilihPetugas(nama) {
  petugasAktif.value = nama;
  localStorage.setItem('iuran.petugas.pos', nama);
}
function gantiPetugas() {
  petugasAktif.value = '';
  localStorage.removeItem('iuran.petugas.pos');
}
const STATUS_LIST = [
  { value: 'belum', label: 'Belum bayar', test: (h) => h.tunggakan > 0 },
  { value: 'lunas', label: 'Lunas', test: (h) => h.tarifDiatur && h.tunggakan <= 0 },
];

const q = ref('');
const blokFilter = ref('');    // '' = semua blok
const statusFilter = ref(''); // '' = semua status
const showFilter = ref(false);
const chipStyle = (active) => active ? 'min-height:40px'
  : 'min-height:40px;background:var(--color-surface);box-shadow:var(--shadow-sm)';
const page = ref(1);
const sel = ref(null);        // selected house
const bulan = ref([]);        // periodes being paid
useScrollLock(showFilter);
useScrollLock(sel);
const toast = ref('');

// arrears first — the whole point of the post screen is speed
const daftar = computed(() => rumah.value
  .filter((h) => !blokFilter.value || String(h.blok) === blokFilter.value)
  .filter((h) => !statusFilter.value
                 || STATUS_LIST.find((s) => s.value === statusFilter.value).test(h))
  .filter((h) => !q.value || h.nama.toLowerCase().includes(q.value.toLowerCase())
                 || h.alamat.toLowerCase().includes(q.value.toLowerCase()))
  .slice().sort((a, b) => b.tunggakan - a.tunggakan));

watch([q, blokFilter, statusFilter], () => { page.value = 1; });

const totalPages = computed(() => Math.max(1, Math.ceil(daftar.value.length / PER_PAGE)));
const halaman = computed(() => daftar.value.slice((page.value - 1) * PER_PAGE, page.value * PER_PAGE));

const label = (p) => labelBulan(p, TAHUN.value, BULAN);
const ringkasBelum = (h) => h.tunggakanList.map((t) => label(t.periode)).join(', ');

// Every owed month across years, oldest first; the two oldest are preselected.
function pilih(h) {
  if (!h.tarifDiatur) return;
  sel.value = h;
  bulan.value = h.tunggakanList.slice(0, 2).map((t) => t.periode);
}
function toggle(p) {
  const a = bulan.value;
  bulan.value = a.includes(p) ? a.filter((x) => x !== p) : [...a, p].sort((x, y) => x - y);
}

// A month is always paid in full at its own tarif (RumahRiwayat/TarifVersi) —
// a house upgraded mid-year owes its earlier months at the old rate. Partial
// payments don't exist in the app (docs/sheets-schema.md `Pembayaran`).
const items = computed(() => !sel.value ? [] : bulan.value.map((p) => ({ periode: p, nominal: sel.value.tarifPer(p) })));
const total = computed(() => items.value.reduce((sum, i) => sum + i.nominal, 0));

const submitting = ref(false);   // disables the button — stops a double-tap from firing two rows

// One silent POST per transaction (all its months in one "rincian" answer).
// Deliberately no "open the Google Form directly" fallback anywhere on this
// screen: only Kas is meant to touch Sheet/Form URLs. If a submission is stuck,
// "Coba lagi" retries the same POST — see usePendingSync.js.
async function catat() {
  if (!sel.value || !items.value.length || submitting.value) return;
  submitting.value = true;
  try {
    const rec = { alamat: sel.value.alamat, items: items.value, petugas: petugasAktif.value };
    addPending(rec);           // survives a reload/crash — see usePendingSync
    await submitTunai(rec);    // opaque; the pending entry is what the satpam sees
    toast.value = `${sel.value.nama} · ${rec.items.length} bulan tunai tercatat. Masuk Kas Tunai pos.`;
    sel.value = null; bulan.value = [];
    setTimeout(load, 4000);    // sheet cache settles, then reconcile (see watch(rumah))
    setTimeout(() => (toast.value = ''), 4000);
  } finally {
    submitting.value = false;
  }
}

async function retryPending(p) {
  await submitTunai(p);
  await load();
}
</script>

<template>
 <PinGate :pin="PIN" storage-key="pos" title="Pos Satpam" env-var="VITE_PIN_POS">

  <!-- PIN dipakai bersama semua satpam; nama dipilih sekali per device supaya
       tiap pembayaran tercatat atas nama yang benar-benar menerima uangnya -->
  <section v-if="!petugasAktif" class="scr col"
           style="gap:var(--space-3);align-items:center;text-align:center;padding-top:var(--space-8)">
    <h3 style="margin:0">Siapa Anda?</h3>
    <p class="text-muted" style="font-size:13px;margin:0">
      Pilih nama Anda — tercatat di setiap pembayaran yang Anda terima.
    </p>
    <div class="col" style="gap:var(--space-2);width:100%;max-width:280px">
      <button v-for="nama in satpamList" :key="nama" type="button" class="btn btn-secondary"
              style="width:100%;min-height:44px;background:var(--color-surface);box-shadow:var(--shadow-sm)"
              @click="pilihPetugas(nama)">
        {{ nama }}
      </button>
    </div>
    <p v-if="!satpamList.length" class="text-muted" style="font-size:12px">
      Daftar nama satpam belum diisi admin di Sheet (tab Petugas, peran "satpam").
    </p>
  </section>

  <section v-else class="scr col" style="gap:var(--space-3)">
    <div class="spread">
      <div>
        <h4 style="margin:0">Terima Iuran</h4>
        <div class="text-muted" style="font-size:11.5px">Pos Cypress</div>
      </div>
      <button class="btn btn-ghost" style="font-size:12px" @click="gantiPetugas">
        {{ petugasAktif }} · Ganti
      </button>
    </div>

    <div class="row" style="gap:var(--space-2)">
      <input class="input grow" v-model="q" placeholder="Cari alamat (mis. N7-09) atau nama">
      <button type="button" class="btn btn-secondary" style="flex:none;min-height:36px;padding:6px 14px;
              font-size:12.5px;background:var(--color-surface);box-shadow:var(--shadow-sm)"
              @click="showFilter = true">
        <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor"
             stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M4 5h16M7 12h10M10 19h4"></path>
        </svg>
        Filter
      </button>
    </div>

    <div v-if="blokFilter || statusFilter" class="row" style="flex-wrap:wrap;gap:6px">
      <span v-if="blokFilter" class="tag tag-accent" style="cursor:pointer" @click="blokFilter = ''">
        Blok N{{ blokFilter }} ✕
      </span>
      <span v-if="statusFilter" class="tag tag-accent" style="cursor:pointer" @click="statusFilter = ''">
        {{ STATUS_LIST.find(s => s.value === statusFilter).label }} ✕
      </span>
    </div>

    <!-- submissions the sheet hasn't confirmed yet — no-cors POST means we never
         actually know if these landed, so keep them visible until reconcile()
         (watch(rumah) above) sees every month in it as lunas -->
    <Card v-if="pending.length" style="background:var(--color-accent-100);gap:6px">
      <div class="spread">
        <span style="font-size:12.5px;font-weight:700;color:var(--color-accent-800)">
          ⚠ {{ pending.length }} pembayaran belum tersinkron
        </span>
      </div>
      <div v-for="p in pending" :key="p.id" class="spread" style="font-size:12px">
        <span>{{ p.alamat }} · {{ p.items.map(i => label(i.periode)).join(', ') }} ·
          {{ rupiahPendek(p.items.reduce((s, i) => s + i.nominal, 0)) }}</span>
        <button class="btn btn-ghost" style="font-size:11px;padding-inline:6px" @click="retryPending(p)">
          Coba lagi
        </button>
      </div>
    </Card>

    <div class="spread text-muted" style="font-size:11.5px">
      <span>{{ daftar.length }} rumah</span>
      <span v-if="totalPages > 1">Hal. {{ page }}/{{ totalPages }}</span>
    </div>

    <div class="col">
      <Card v-for="h in halaman" :key="h.alamat" style="flex-direction:row;align-items:center;
            gap:var(--space-3)" :style="{ cursor: h.tarifDiatur ? 'pointer' : 'default',
            opacity: h.tarifDiatur ? 1 : .6 }" @click="pilih(h)">
        <div class="num" :style="badgeStyle(h)" style="width:46px;height:46px;flex:none;border-radius:50%;
             display:flex;align-items:center;justify-content:center;font-family:var(--font-heading)">
          {{ h.rumah }}
        </div>
        <div class="grow">
          <div class="num" style="font-size:10.5px;letter-spacing:.06em;color:var(--color-accent-700)">{{ h.alamat }}</div>
          <div class="truncate" style="font-size:14px;font-weight:700">{{ h.nama }}</div>
          <div class="text-muted" style="font-size:11.5px">
            {{ !h.tarifDiatur ? 'Tarif belum diatur — hubungi bendahara'
               : h.tunggakan ? 'Belum: ' + ringkasBelum(h) : 'Lunas' }}
          </div>
        </div>
        <div class="col" style="align-items:flex-end;gap:5px">
          <Tag v-if="!h.tarifDiatur" status="neutral">Tarif?</Tag>
          <Tag v-else :status="h.tunggakan ? 'Belum' : 'Lunas'">
            {{ h.tunggakan ? h.tunggakanList.length + ' bln' : 'Lunas' }}
          </Tag>
          <span class="num text-muted" style="font-size:11.5px">
            {{ h.tunggakan ? rupiahPendek(h.tunggakan) : '—' }}
          </span>
        </div>
      </Card>
      <p v-if="!daftar.length" class="text-muted" style="text-align:center;font-size:12.5px">
        Tidak ada rumah yang cocok.
      </p>
    </div>

    <div v-if="totalPages > 1" class="row" style="justify-content:center;gap:var(--space-2)">
      <button class="btn btn-secondary" :disabled="page === 1" @click="page--">‹ Sebelumnya</button>
      <button class="btn btn-secondary" :disabled="page === totalPages" @click="page++">Berikutnya ›</button>
    </div>

    <!-- sheet: panel filter blok + status -->
    <div v-if="showFilter" class="dialog-backdrop sheet-backdrop" @click.self="showFilter = false">
      <div class="dialog sheet" style="border-radius:var(--radius-lg) var(--radius-lg) 0 0;
           max-height:85dvh">
       <div class="sheet-scroll">
        <div class="spread">
          <div class="dialog-title">Filter</div>
          <button class="btn btn-ghost" @click="showFilter = false">×</button>
        </div>

        <div class="kick">Blok</div>
        <button type="button" class="btn" :class="!blokFilter ? 'btn-primary' : 'btn-secondary'"
                :style="chipStyle(!blokFilter) + ';width:100%'" @click="blokFilter = ''">
          Semua blok
        </button>
        <div style="display:grid;grid-template-columns:repeat(5,1fr);gap:var(--space-2)">
          <button v-for="b in BLOK_LIST" :key="b" type="button" class="btn"
                  :class="blokFilter === b ? 'btn-primary' : 'btn-secondary'"
                  :style="chipStyle(blokFilter === b)" @click="blokFilter = b">
            N{{ b }}
          </button>
        </div>

        <div class="kick">Status</div>
        <div class="row" style="gap:var(--space-2)">
          <button type="button" class="btn grow" :class="!statusFilter ? 'btn-primary' : 'btn-secondary'"
                  :style="chipStyle(!statusFilter)" @click="statusFilter = ''">
            Semua
          </button>
          <button v-for="s in STATUS_LIST" :key="s.value" type="button" class="btn grow"
                  :class="statusFilter === s.value ? 'btn-primary' : 'btn-secondary'"
                  :style="chipStyle(statusFilter === s.value)" @click="statusFilter = s.value">
            {{ s.label }}
          </button>
        </div>

        <Button block @click="showFilter = false">Terapkan</Button>
       </div>
      </div>
    </div>

    <!-- sheet: pilih bulan (selalu penuh per bulan), catat -->
    <div v-if="sel" class="dialog-backdrop" @click.self="sel = null">
      <div class="dialog">
        <div class="spread">
          <div>
            <div class="dialog-title">{{ sel.alamat }} · {{ sel.nama }}</div>
            <div class="text-muted" style="font-size:12px">
              {{ sel.luas }} m² · tarif {{ rupiah(sel.tarif) }}/bulan
            </div>
          </div>
        </div>

        <div class="kick">Bulan yang dibayar</div>
        <div v-if="sel.tunggakanList.length" class="row" style="flex-wrap:wrap;gap:var(--space-2)">
          <button v-for="t in sel.tunggakanList" :key="t.periode" class="btn"
                  :class="bulan.includes(t.periode) ? 'btn-primary' : 'btn-secondary'"
                  @click="toggle(t.periode)">{{ label(t.periode) }}</button>
        </div>
        <p v-else class="text-muted" style="font-size:12px;margin:0">Tidak ada tunggakan.</p>
        <p class="text-muted" style="font-size:11px;margin:0">
          Iuran dibayar penuh per bulan. Pembayaran sebagian tidak dicatat di sini — arahkan ke bendahara.
        </p>

        <div class="spread" style="background:var(--color-bg);border-radius:var(--radius-md);
             padding:var(--space-3) var(--space-4)">
          <span style="font-size:13.5px;font-weight:700">Diterima tunai</span>
          <span class="num" style="font-family:var(--font-heading);font-size:21px;color:var(--color-accent-700)">
            {{ rupiah(total) }}
          </span>
        </div>

        <Button block :disabled="submitting || !items.length" @click="catat">
          {{ submitting ? 'Mengirim…' : 'Catat & Kirim ke Sheet' }}
        </Button>
      </div>
    </div>

    <div v-if="toast" class="card elev-lg"
         style="position:fixed;left:50%;transform:translateX(-50%);bottom:96px;
                width:min(432px,calc(100% - 32px));background:var(--color-neutral-900);
                color:var(--color-neutral-100);font-size:12.5px">{{ toast }}</div>
  </section>
 </PinGate>
</template>
