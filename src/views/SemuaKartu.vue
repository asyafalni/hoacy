<script setup vapor>
import { ref, computed, watch } from 'vue';
import { useSheet } from '../composables/useSheet';
import { useScrollLock } from '../composables/useScrollLock';
import { BULAN, rupiah, rupiahPendek, BLOK_LIST, BLOK_WARNA_DEFAULT } from '../lib/tariff';
import { urlWhatsapp } from '../lib/forms';
import PinGate from '../components/PinGate.vue';
import Card from '../components/ui/Card.vue';
import Tag from '../components/ui/Tag.vue';

// Read-only version of the Pos house list — same search/filter/pagination, but
// bendahara is here to LOOK, not collect cash, so tapping a house opens its
// 12-month card (same grid WargaCard.vue shows the resident) instead of a
// payment dialog. Shares the Kas PIN/storage-key with Bendahara.vue and CetakQR.vue.
const { rumah, blokWarna } = useSheet();
const PIN = import.meta.env.VITE_PIN_KAS || '';
const PER_PAGE = 10;

const badgeStyle = (h) => {
  const hex = blokWarna.value[String(h.blok)] || BLOK_WARNA_DEFAULT[String(h.blok)] || '#c0b6a5';
  return `background:color-mix(in srgb, ${hex} 22%, white)`;
};

const STATUS_LIST = [
  { value: 'belum', label: 'Belum bayar', test: (h) => h.tunggakan > 0 },
  { value: 'lunas', label: 'Lunas', test: (h) => h.tunggakan <= 0 },
];

const q = ref('');
const blokFilter = ref('');
const statusFilter = ref('');
const showFilter = ref(false);
const chipStyle = (active) => active ? 'min-height:40px'
  : 'min-height:40px;background:var(--color-surface);box-shadow:var(--shadow-sm)';
const page = ref(1);
const sel = ref(null);   // house whose card is open
useScrollLock(showFilter);
useScrollLock(sel);

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

const belum = (h) => h.status
  .map((s, i) => ({ s, i }))
  .filter((x) => x.s === 'Belum' || x.s === 'Sebagian')
  .map((x) => x.i);

const cls = (s) => ({ Lunas: 'lunas', Sebagian: 'sebagian', Pending: 'pending', Belum: 'belum' }[s] || 'kosong');
</script>

<template>
 <PinGate :pin="PIN" storage-key="kas" title="Kas Bendahara" env-var="VITE_PIN_KAS">
  <section class="scr col" style="gap:var(--space-3)">
    <div class="spread">
      <div>
        <h4 style="margin:0">Semua Kartu Rumah</h4>
        <div class="text-muted" style="font-size:11.5px">Lihat status per rumah — bukan buat catat bayar</div>
      </div>
      <a href="#/kas" class="btn btn-ghost" style="font-size:12px">← Kas</a>
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

    <div class="spread text-muted" style="font-size:11.5px">
      <span>{{ daftar.length }} rumah</span>
      <span v-if="totalPages > 1">Hal. {{ page }}/{{ totalPages }}</span>
    </div>

    <div class="col">
      <Card v-for="h in halaman" :key="h.alamat" style="flex-direction:row;align-items:center;
            gap:var(--space-3);cursor:pointer" @click="sel = h">
        <div class="num" :style="badgeStyle(h)" style="width:46px;height:46px;flex:none;border-radius:50%;
             display:flex;align-items:center;justify-content:center;font-family:var(--font-heading)">
          {{ h.rumah }}
        </div>
        <div class="grow">
          <div class="num" style="font-size:10.5px;letter-spacing:.06em;color:var(--color-accent-700)">{{ h.alamat }}</div>
          <div class="truncate" style="font-size:14px;font-weight:700">{{ h.nama }}</div>
          <div class="text-muted" style="font-size:11.5px">
            {{ h.tunggakan ? 'Belum: ' + belum(h).map(i => BULAN[i].slice(0,3)).join(', ') : 'Lunas' }}
          </div>
        </div>
        <div class="col" style="align-items:flex-end;gap:5px">
          <Tag :status="h.tunggakan ? 'Belum' : 'Lunas'">
            {{ h.tunggakan ? belum(h).length + ' bln' : 'Lunas' }}
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

    <!-- sheet: panel filter blok + status (sama seperti Pos) -->
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

        <button class="btn btn-primary" style="width:100%" @click="showFilter = false">Terapkan</button>
       </div>
      </div>
    </div>

    <!-- sheet: kartu 12 bulan rumah terpilih — lihat saja, tidak ada aksi bayar -->
    <div v-if="sel" class="dialog-backdrop sheet-backdrop" @click.self="sel = null">
      <div class="dialog sheet" style="border-radius:var(--radius-lg) var(--radius-lg) 0 0;
           max-height:85dvh">
       <div class="sheet-scroll">
        <div class="spread">
          <div>
            <div class="dialog-title">{{ sel.alamat }} · {{ sel.nama }}</div>
            <div class="text-muted" style="font-size:12px">
              {{ sel.luas }} m² · tarif {{ rupiah(sel.tarif) }}/bulan
            </div>
          </div>
          <button class="btn btn-ghost" @click="sel = null">×</button>
        </div>

        <div class="spread" style="background:var(--color-bg);border-radius:var(--radius-md);
             padding:var(--space-3) var(--space-4)">
          <span style="font-size:13.5px;font-weight:700">Tunggakan</span>
          <span class="num" style="font-family:var(--font-heading);font-size:21px;color:var(--color-accent-700)">
            {{ rupiah(sel.tunggakan) }}
          </span>
        </div>

        <a v-if="sel.telp" class="btn btn-primary" :href="urlWhatsapp(sel)" target="_blank"
           style="justify-content:center;background:#25D366">
          Kirim link kartu via WhatsApp
        </a>
        <p v-else class="text-muted" style="font-size:11.5px;margin:0">
          Tidak ada no. HP terdaftar untuk rumah ini (Rumah!F kosong).
        </p>

        <div class="months">
          <div v-for="(s, i) in sel.status" :key="i" class="month" :class="cls(s)">
            <div style="font-size:12.5px;font-weight:700">{{ BULAN[i] }}</div>
            <div class="num" style="font-size:10px;opacity:.8">{{ s === '-' ? '—' : s }}</div>
          </div>
        </div>
       </div>
      </div>
    </div>
  </section>
 </PinGate>
</template>
