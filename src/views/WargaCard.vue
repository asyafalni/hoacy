<script setup vapor>
import { ref, computed } from 'vue';
import { useSheet } from '../composables/useSheet';
import { BULAN, rupiah, rupiahPendek, alamat, parseAlamat, REKENING } from '../lib/tariff';
import { urlPembayaran } from '../lib/forms';
import Card from '../components/ui/Card.vue';
import Tag from '../components/ui/Tag.vue';
import Button from '../components/ui/Button.vue';

const { rumah, meta } = useSheet();

// No login: blok is fixed, resident types the house number. Persisted so the card
// opens straight away next time (and a per-house QR link can set it via ?rumah=).
// Alamat has three parts: cluster code + block number + house number -> N7-09.
// ?alamat=N7-09 lets each house have its own QR link; otherwise the resident types
// blok + rumah once and it is remembered.
const CLUSTER = 'N';
const saved = new URLSearchParams(location.search).get('alamat')
           || localStorage.getItem('iuran.alamat') || '';
const key = ref(saved);
const inBlok = ref('');
const inRumah = ref('');
const notFound = ref(false);

const me = computed(() => rumah.value.find((h) => h.alamat === key.value));

function open() {
  const parsed = parseAlamat(`${CLUSTER}${inBlok.value}-${inRumah.value}`);
  if (!parsed) { notFound.value = true; return; }
  const id = alamat(parsed);
  if (!rumah.value.some((h) => h.alamat === id)) { notFound.value = true; return; }
  notFound.value = false;
  key.value = id;
  localStorage.setItem('iuran.alamat', id);
}
function ganti() { key.value = ''; localStorage.removeItem('iuran.alamat'); }

const cls = (s) => ({ Lunas: 'lunas', Sebagian: 'sebagian', Pending: 'pending', Belum: 'belum' }[s] || 'kosong');

// ── Konfirmasi transfer ───────────────────────────────────────────────────────
// Bukti transfer is a file, and a Sheet cannot hold one. So the sheet below
// collects months + a local preview, then hands off to the prefilled Google Form
// whose LAST question is a file-upload (Drive) — that question requires a Google
// sign-in, which is exactly the step that makes the proof auditable.
const trOpen = ref(false);
const trFile = ref(null);
const camInput = ref(null);
const fileInput = ref(null);
const trMuka = ref(false);       // "bayar di muka": reveals not-yet-due months, off by default
const trMukaDepan = ref(false);  // nested further: reveals next year's months

// tahun_aktif datang dari Sheet (API!B8, = YEAR(TODAY())) supaya app tidak pernah
// hardcode tahun — fallback ke tahun device kalau Sheet belum dimigrasi.
const tahunIni = computed(() => Number(meta.value.tahun_aktif) || new Date().getFullYear());

// trMonths menyimpan { bulan (1-12), tahun } — bukan cuma index bulan — supaya bisa
// mencampur bulan tahun berjalan dan bulan tahun depan dalam satu konfirmasi.
const trMonths = ref([]);
const keyOf = (m) => `${m.tahun}-${m.bulan}`;
const trKeys = computed(() => new Set(trMonths.value.map(keyOf)));

// tunggakan + kewajiban berjalan — apa yang muncul otomatis saat dialog dibuka
const owed = computed(() => !me.value ? [] : me.value.status
  .map((s, i) => ({ s, i }))
  .filter((x) => x.s === 'Belum' || x.s === 'Sebagian')
  .map((x) => ({ bulan: x.i + 1, tahun: tahunIni.value })));

// belum jatuh tempo tahun ini (status "-") — hanya muncul kalau klik "bayar di muka"
const muka = computed(() => !me.value ? [] : me.value.status
  .map((s, i) => ({ s, i }))
  .filter((x) => x.s === '-')
  .map((x) => ({ bulan: x.i + 1, tahun: tahunIni.value })));

// bulan tahun depan yang belum ada baris Pembayaran-nya (API!Z, lihat sheets-schema.md §6)
const mukaDepan = computed(() => {
  if (!me.value) return [];
  const sudah = new Set(me.value.mukaTahunDepan || []);
  return Array.from({ length: 12 }, (_, i) => i + 1)
    .filter((b) => !sudah.has(b))
    .map((b) => ({ bulan: b, tahun: tahunIni.value + 1 }));
});

function openTransfer() {
  if (!owed.value.length && !muka.value.length && !mukaDepan.value.length) return;
  trMonths.value = owed.value.length ? [owed.value[0]] : [];
  trMuka.value = !owed.value.length;               // sudah lunas kewajiban -> langsung buka bagian muka
  trMukaDepan.value = trMuka.value && !muka.value.length;  // tahun ini juga tuntas -> langsung ke tahun depan
  trFile.value = null;
  trOpen.value = true;
}
function toggleTr(m) {
  const k = keyOf(m);
  trMonths.value = trKeys.value.has(k)
    ? trMonths.value.filter((x) => keyOf(x) !== k)
    : [...trMonths.value, m].sort((a, b) => a.tahun - b.tahun || a.bulan - b.bulan);
}
function pickFile(e) { trFile.value = e.target.files?.[0] || null; e.target.value = ''; }
function openCamera() { camInput.value?.click(); }
function openGallery() { fileInput.value?.click(); }

const trTotal = computed(() => trMonths.value.length * (me.value?.tarif || 0));
const trReady = computed(() => !!trFile.value && trMonths.value.length > 0);

// one Form per month — separate rows keep partial history auditable
const trUrls = computed(() => !me.value ? [] : trMonths.value.map((m) =>
  urlPembayaran({
    noRumah: me.value.alamat, bulan: m.bulan, tahun: m.tahun, nominal: me.value.tarif,
    metode: 'transfer', petugas: 'Warga',
    catatan: trFile.value ? `bukti: ${trFile.value.name}` : '',
  })));

function kirimKonfirmasi() {
  if (!trReady.value) return;
  trUrls.value.forEach((u, k) => setTimeout(() => window.open(u, '_blank'), k * 250));
  trOpen.value = false;
}
</script>

<template>
  <!-- masuk: blok + nomor rumah, tanpa akun -->
  <section v-if="!me" class="scr col" style="gap:var(--space-3)">
    <div style="width:74px;height:74px;border-radius:50%;background:var(--color-accent-2-200);
                color:var(--color-accent-2-800);display:flex;align-items:center;
                justify-content:center;font-family:var(--font-heading);font-size:28px">N</div>
    <h3 style="margin:0">Buka kartu iuran</h3>
    <p class="text-muted" style="font-size:13px;margin:0">
      Tidak perlu akun — masukkan blok dan nomor rumah Anda.<br>
      <span style="opacity:.75">No login — cluster code, block number, house number.</span>
    </p>
    <div class="row" style="align-items:flex-end">
      <div class="field" style="width:64px">
        <label>Cluster</label>
        <input class="input" :value="CLUSTER" readonly style="text-align:center">
      </div>
      <div class="field" style="width:64px">
        <label>Blok</label>
        <input class="input" v-model="inBlok" inputmode="numeric" placeholder="7"
               style="text-align:center" @keyup.enter="open">
      </div>
      <div class="field grow">
        <label>Rumah</label>
        <input class="input" v-model="inRumah" inputmode="numeric" placeholder="09" @keyup.enter="open">
      </div>
    </div>
    <Button block @click="open">Lihat kartu saya</Button>
    <p class="text-muted num" style="font-size:11px;margin:0">Contoh: N7-09 — Blok N7, No. 09.</p>
    <p v-if="notFound" class="text-muted" style="font-size:11.5px">Alamat tidak ditemukan.</p>
  </section>

  <!-- kartu -->
  <section v-else class="scr col" style="gap:var(--space-3)">
    <div class="spread">
      <span class="kick">Kartu saya</span>
      <button class="btn btn-ghost" style="font-size:12px" @click="ganti">Ganti rumah</button>
    </div>

    <div style="border-radius:calc(var(--radius-lg)*1.15);overflow:hidden;
                background:var(--color-surface);box-shadow:var(--shadow-md)">
      <div style="background:var(--color-accent);padding:var(--space-4);position:relative;overflow:hidden">
        <div style="position:absolute;right:-36px;top:-46px;width:150px;height:150px;border-radius:50%;
                    background:var(--color-accent-2-400);opacity:.55"></div>
        <div style="position:relative;font-family:var(--font-heading);font-size:21px;color:var(--color-bg)">
          {{ me.nama }}
        </div>
        <div class="row" style="position:relative;gap:var(--space-6);margin-top:var(--space-2)">
          <div>
            <div style="font-size:9.5px;letter-spacing:.1em;text-transform:uppercase;color:var(--color-accent-200)">Alamat</div>
            <div class="num" style="font-size:13.5px;font-weight:700;color:var(--color-bg)">{{ me.alamat }}</div>
            <div style="font-size:11px;color:var(--color-bg)">
              Blok {{ me.cluster }}{{ me.blok }} - No. {{ me.rumah }}
            </div>
          </div>
          <div>
            <div style="font-size:9.5px;letter-spacing:.1em;text-transform:uppercase;color:var(--color-accent-200)">Luas / Tarif</div>
            <div class="num" style="font-size:13.5px;font-weight:700;color:var(--color-bg)">
              {{ me.luas }} m² · {{ rupiahPendek(me.tarif) }}/bln
            </div>
          </div>
        </div>
      </div>

      <div style="padding:var(--space-4)">
        <div class="spread" style="margin-bottom:var(--space-2)">
          <span class="kick">Tahun {{ tahunIni }}</span>
          <Tag :status="me.tunggakan > 0 ? 'Sebagian' : 'Lunas'">
            {{ me.tunggakan > 0 ? rupiahPendek(me.tunggakan) + ' belum dibayar' : 'Lunas' }}
          </Tag>
        </div>
        <div class="months">
          <div v-for="(s, i) in me.status" :key="i" class="month" :class="cls(s)">
            <div style="font-size:12.5px;font-weight:700">{{ BULAN[i] }}</div>
            <div class="num" style="font-size:10px;opacity:.8">{{ s === '-' ? '—' : s }}</div>
          </div>
        </div>
      </div>
    </div>

    <Card>
      <div class="spread">
        <span style="font-size:13.5px;font-weight:700">Tagihan berjalan</span>
        <span class="num" style="font-family:var(--font-heading);font-size:22px;color:var(--color-accent-700)">
          {{ rupiah(me.tunggakan) }}
        </span>
      </div>
      <p class="text-muted" style="font-size:11.5px;margin:0">
        Tarif {{ rupiah(me.tarif) }}/bulan (ISLK per luas tanah + Iuran RT Rp 50.000).
      </p>
      <Button block :disabled="!owed.length && !muka.length && !mukaDepan.length" @click="openTransfer">
        {{ owed.length ? 'Bayar transfer' : 'Bayar di muka' }}
      </Button>
      <div class="text-muted num" style="text-align:center;font-size:10.5px">
        {{ REKENING.bank }} {{ REKENING.nomor }} a.n. {{ REKENING.nama }} · jatuh tempo tgl 20
      </div>
    </Card>

    <!-- side sheet: konfirmasi transfer + bukti -->
    <div v-if="trOpen" class="dialog-backdrop" @click.self="trOpen = false">
      <div class="dialog" style="align-self:flex-end;width:100%;max-width:480px;
           border-radius:var(--radius-lg) var(--radius-lg) 0 0">
        <div class="spread">
          <div>
            <div class="dialog-title">Konfirmasi Transfer</div>
            <div class="text-muted num" style="font-size:12px">{{ me.alamat }} · {{ me.nama }}</div>
          </div>
          <button class="btn btn-ghost" @click="trOpen = false">×</button>
        </div>

        <div class="col" style="background:var(--color-bg);border-radius:var(--radius-md);
             padding:var(--space-3) var(--space-4);gap:6px">
          <div class="spread" style="font-size:12.5px">
            <span class="text-muted">Rekening tujuan</span>
            <span class="num" style="font-weight:700">{{ REKENING.bank }} {{ REKENING.nomor }}</span>
          </div>
          <div class="spread" style="font-size:12.5px">
            <span class="text-muted">Atas nama</span><span style="font-weight:600">{{ REKENING.nama }}</span>
          </div>
        </div>

        <template v-if="owed.length">
          <div class="kick">Bulan yang dibayar</div>
          <div class="row" style="flex-wrap:wrap;gap:var(--space-2)">
            <button v-for="m in owed" :key="keyOf(m)" class="btn"
                    :class="trKeys.has(keyOf(m)) ? 'btn-primary' : 'btn-secondary'"
                    @click="toggleTr(m)">
              {{ BULAN[m.bulan - 1] }}
            </button>
          </div>
        </template>
        <p v-else class="text-muted" style="font-size:12px;margin:0">
          Tidak ada tunggakan tahun ini — lanjut bayar di muka di bawah.
        </p>

        <button v-if="!trMuka && muka.length" type="button" class="btn btn-ghost"
                style="justify-content:flex-start;font-size:12px;padding-left:0" @click="trMuka = true">
          + Bayar di muka untuk bulan berikutnya
        </button>
        <template v-if="trMuka && muka.length">
          <div class="kick">Bayar di muka ({{ tahunIni }}, belum jatuh tempo)</div>
          <div class="row" style="flex-wrap:wrap;gap:var(--space-2)">
            <button v-for="m in muka" :key="keyOf(m)" class="btn"
                    :class="trKeys.has(keyOf(m)) ? 'btn-primary' : 'btn-secondary'"
                    @click="toggleTr(m)">
              {{ BULAN[m.bulan - 1] }}
            </button>
          </div>
        </template>

        <button v-if="trMuka && !trMukaDepan && mukaDepan.length" type="button" class="btn btn-ghost"
                style="justify-content:flex-start;font-size:12px;padding-left:0" @click="trMukaDepan = true">
          + Bahkan bayar untuk tahun {{ tahunIni + 1 }}
        </button>
        <template v-if="trMukaDepan && mukaDepan.length">
          <div class="kick">Tahun {{ tahunIni + 1 }}</div>
          <div class="row" style="flex-wrap:wrap;gap:var(--space-2)">
            <button v-for="m in mukaDepan" :key="keyOf(m)" class="btn"
                    :class="trKeys.has(keyOf(m)) ? 'btn-primary' : 'btn-secondary'"
                    @click="toggleTr(m)">
              {{ BULAN[m.bulan - 1] }} '{{ String(m.tahun).slice(2) }}
            </button>
          </div>
        </template>

        <div class="kick">Bukti transfer</div>
        <input ref="camInput" type="file" accept="image/*" capture="environment"
               @change="pickFile" style="display:none">
        <input ref="fileInput" type="file" accept="image/*,application/pdf"
               @change="pickFile" style="display:none">

        <div v-if="!trFile" class="row" style="gap:var(--space-2)">
          <button type="button" class="btn btn-secondary grow" @click="openCamera">📷 Ambil foto</button>
          <button type="button" class="btn btn-secondary grow" @click="openGallery">🖼 Pilih file</button>
        </div>
        <div v-else style="border-radius:var(--radius-lg);padding:var(--space-4);
                    border:1px dashed var(--color-accent-2-500);background:var(--color-accent-2-100)">
          <div class="row" style="text-align:left">
            <div style="width:40px;height:40px;flex:none;border-radius:50%;
                        background:var(--color-accent-2-500);color:var(--color-bg);
                        display:flex;align-items:center;justify-content:center;font-size:16px">✓</div>
            <div class="grow">
              <div class="truncate" style="font-size:12.5px;font-weight:700">{{ trFile.name }}</div>
              <div class="text-muted" style="font-size:11px">
                <a href="#" @click.prevent="openCamera">Ambil ulang</a> ·
                <a href="#" @click.prevent="openGallery">Ganti file</a>
              </div>
            </div>
          </div>
        </div>
        <div class="text-muted" style="font-size:11px">Foto struk / screenshot m-banking · JPG, PNG, PDF</div>

        <div class="spread" style="background:var(--color-bg);border-radius:var(--radius-md);
             padding:var(--space-3) var(--space-4)">
          <span style="font-size:13.5px;font-weight:700">Total dikonfirmasi</span>
          <span class="num" style="font-family:var(--font-heading);font-size:21px;color:var(--color-accent-700)">
            {{ rupiah(trTotal) }}
          </span>
        </div>

        <Button block :variant="trReady ? 'primary' : 'secondary'" @click="kirimKonfirmasi">
          Kirim konfirmasi
        </Button>
        <div class="text-muted" style="text-align:center;font-size:10.5px">
          {{ trReady
             ? 'Form terbuka untuk melampirkan bukti — status menjadi Pending verifikasi'
             : 'Lampirkan bukti transfer untuk melanjutkan' }}
        </div>
      </div>
    </div>
  </section>
</template>
