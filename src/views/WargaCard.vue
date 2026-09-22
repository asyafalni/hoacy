<script setup vapor>
import { ref, computed, watch, onMounted } from 'vue';
import { useSheet } from '../composables/useSheet';
import { usePembayaranLedger } from '../composables/usePembayaranLedger';
import { useScrollLock } from '../composables/useScrollLock';
import { BULAN, rupiah, rupiahPendek, alamat, REKENING, BLOK_LIST, BLOK_WARNA_DEFAULT } from '../lib/tariff';
import { tarifPada } from '../lib/tarifHistoris';
import { urlPembayaran } from '../lib/forms';
import Card from '../components/ui/Card.vue';
import Tag from '../components/ui/Tag.vue';
import Button from '../components/ui/Button.vue';
import RekeningCard from '../components/RekeningCard.vue';

const { rumah, meta, blokWarna, rumahRiwayatRows, tarifVersiRows, rateCardAktif } = useSheet();
// Own house only, but gviz has no server-side row filter — this fetches the
// whole ledger same as Bendahara.vue does, filtered down client-side below.
// Fine here for the same reason it's fine there: this screen is PIN-gated
// (deterrent, not real access control — see the note further down), unlike
// RingkasanPublik.vue which deliberately never touches this composable.
const { rows: pembayaranRowsAll, load: loadPembayaran } = usePembayaranLedger();
onMounted(loadPembayaran);

// No login, but blok+rumah alone is guessable by any resident — so opening a card
// also needs that house's PIN (Rumah!L in the Sheet, defaults to the last 3 digits
// of the phone number, admin can overwrite it per row). This is a deterrent, same
// as the Pos/Kas PinGate: the Sheet is public-by-design (docs/deploy.md), so the PIN
// itself travels in the same gviz response the app already reads — it stops a
// neighbour from browsing someone else's dues in the app UI, not a determined actor
// reading the raw feed directly.
// Alamat has three parts: cluster code + block number + house number -> N7-09.
// ?alamat=N7-09 lets each house have its own QR link (still gated by PIN); otherwise
// the resident picks blok + types rumah. Cluster is always "N" here, so it's baked
// into BLOK_LIST's labels instead of asking for it as a separate field.
const CLUSTER = 'N';
const chipStyle = (active) => active ? 'min-height:42px'
  : 'min-height:42px;background:var(--color-surface);box-shadow:var(--shadow-sm)';

const pinOkKey = (id) => `iuran.pinok.${id}`;
const qsAlamat = new URLSearchParams(location.search).get('alamat') || '';
const initialAlamat = qsAlamat || localStorage.getItem('iuran.alamat') || '';

const key = ref('');
const inBlok = ref('');
const inRumah = ref('');
const notFound = ref(false);
const pendingHouse = ref(null);
const pinInput = ref('');
const pinError = ref(false);

const me = computed(() => rumah.value.find((h) => h.alamat === key.value));

// Each block gets its own color (admin-set in the Sheet, Blok tab — see
// docs/sheets-schema.md §2), applied to the card header here and to the house
// badge in PosSatpam.vue.
const warnaKartu = computed(() =>
  (me.value && (blokWarna.value[String(me.value.blok)] || BLOK_WARNA_DEFAULT[String(me.value.blok)]))
  || '#c67139');

function tryUnlock(id) {
  const found = rumah.value.find((h) => h.alamat === id);
  if (!found) { notFound.value = true; return; }
  notFound.value = false;
  if (localStorage.getItem(pinOkKey(id)) === '1') {
    key.value = id;
    localStorage.setItem('iuran.alamat', id);
  } else {
    pendingHouse.value = found;
    pinInput.value = '';
    pinError.value = false;
  }
}

function open() {
  const rumahNum = inRumah.value.trim();
  if (!inBlok.value || !rumahNum) { notFound.value = true; return; }
  tryUnlock(alamat({ cluster: CLUSTER, blok: inBlok.value, rumah: rumahNum }));
}

function submitPin() {
  if (!pendingHouse.value) return;
  const entered = pinInput.value.trim();
  if (entered && entered === String(pendingHouse.value.pin ?? '')) {
    localStorage.setItem(pinOkKey(pendingHouse.value.alamat), '1');
    key.value = pendingHouse.value.alamat;
    localStorage.setItem('iuran.alamat', pendingHouse.value.alamat);
    pendingHouse.value = null;
  } else {
    pinError.value = true;
  }
  pinInput.value = '';
}
function batalPin() { pendingHouse.value = null; pinInput.value = ''; pinError.value = false; }
function ganti() { key.value = ''; localStorage.removeItem('iuran.alamat'); }

// ?alamat= / remembered address: try it once the Sheet has loaded — still goes
// through tryUnlock, so a QR-code link alone can't skip the PIN on a new device.
watch(rumah, (list) => {
  if (list.length && initialAlamat && !key.value && !pendingHouse.value) tryUnlock(initialAlamat);
}, { immediate: true });

const cls = (s) => ({ Lunas: 'lunas', Sebagian: 'sebagian', Pending: 'pending', Belum: 'belum' }[s] || 'kosong');

// ── Konfirmasi transfer ───────────────────────────────────────────────────────
// Bukti transfer is a file, and a Sheet cannot hold one. So the sheet below
// collects months + a local preview, then hands off to the prefilled Google Form
// whose LAST question is a file-upload (Drive) — that question requires a Google
// sign-in, which is exactly the step that makes the proof auditable.
const trOpen = ref(false);
useScrollLock(trOpen);
const trFile = ref(null);
const camInput = ref(null);
const fileInput = ref(null);
const trMuka = ref(false);       // "bayar di muka": reveals not-yet-due months, off by default
const trMukaDepan = ref(false);  // nested further: reveals next year's months

// tahun_aktif datang dari Sheet (API!B8, = YEAR(TODAY())) supaya app tidak pernah
// hardcode tahun — fallback ke tahun device kalau Sheet belum dimigrasi.
const tahunIni = computed(() => Number(meta.value.tahun_aktif) || new Date().getFullYear());

// ── Kartu tahun-tahun sebelumnya ──────────────────────────────────────────────
// `Status` (dan karenanya `me.status`/`me.tunggakan`) cuma tahun berjalan — grid-nya
// reset tiap 1 Januari (sheets-schema.md §5). Buat tahun lalu, hitung ulang di sini
// dari `Pembayaran` mentah (append-only, nggak pernah direset), rumus yang sama
// persis dengan Status!P2/AC2 di Sheet, cuma tahunnya diparameterkan. Tarif per
// bulan diambil dari `RumahRiwayat`/`TarifVersi` (sheets-schema.md §12/§13) —
// luas/tipe dan rate-card yang BERLAKU di bulan itu, bukan yang berlaku sekarang.
const pembayaranMeSemua = computed(() => !me.value ? []
  : pembayaranRowsAll.value.filter((r) => r[1] === me.value.alamat));

// Tahun mana saja yang punya data — bukan cuma "tahunIni - 3" hardcoded, biar
// nggak nampilin tahun kosong buat rumah yang baru gabung cluster.
const historyYears = computed(() => {
  const tahunList = pembayaranMeSemua.value.map((r) => Number(r[3])).filter(Boolean);
  if (!tahunList.length) return [];
  const minTahun = Math.min(...tahunList);
  const years = [];
  for (let y = tahunIni.value - 1; y >= minTahun; y -= 1) years.push(y);
  return years;
});

// Tarif per bulan buat tahun manapun (termasuk tahun berjalan) — dipakai buat
// nampilin angkanya langsung di grid, bukan cuma di kartu "Tagihan berjalan"
// yang SELALU tahun berjalan sendiri (itu yang bikin bingung: ganti tab tahun
// nggak mengubah kartu itu sama sekali, jadi kelihatannya "nggak ada beda"
// padahal bedanya ada di grid, cuma nggak pernah ditulis angkanya).
function hitungTarifBulan(tahun) {
  return Array.from({ length: 12 }, (_, i) =>
    tarifPada(rumahRiwayatRows.value, tarifVersiRows.value, me.value.alamat, tahun, i + 1));
}

function hitungTahun(tahun) {
  const rowsTahun = pembayaranMeSemua.value.filter((r) => Number(r[3]) === tahun);
  const tarifBulan = hitungTarifBulan(tahun);
  let tunggakan = 0;
  let dataLengkap = true;
  const status = Array.from({ length: 12 }, (_, i) => {
    const bulan = i + 1;
    const tarif = tarifBulan[i];
    // Belum ada baris RumahRiwayat/TarifVersi yang berlaku sejauh itu — jangan
    // diam-diam anggap tarif 0 (nanti kelihatan "Lunas" padahal cuma nggak ada
    // datanya). Tandai sebagai tidak diketahui, sama seperti "-" yang dipakai
    // buat bulan yang belum jatuh tempo.
    if (tarif == null) { dataLengkap = false; return '-'; }
    const sah = rowsTahun.filter((r) => Number(r[2]) === bulan && r[10] === 'sah')
      .reduce((sum, r) => sum + (Number(r[4]) || 0), 0);
    tunggakan += Math.max(0, tarif - sah);
    if (sah >= tarif) return 'Lunas';
    if (sah > 0) return 'Sebagian';
    const pending = rowsTahun.filter((r) => Number(r[2]) === bulan && r[10] === 'pending')
      .reduce((sum, r) => sum + (Number(r[4]) || 0), 0);
    return pending > 0 ? 'Pending' : 'Belum';
  });
  return { status, tunggakan, dataLengkap, tarifBulan };
}

const tahunDilihat = ref(0);   // 0 = tahun berjalan; diinisialisasi di watch(rumah) bawah
watch(me, (h) => { if (h) tahunDilihat.value = tahunIni.value; });
const tahunData = computed(() => (!me.value || tahunDilihat.value === tahunIni.value)
  ? { status: me.value?.status || [], tunggakan: me.value?.tunggakan || 0, dataLengkap: true,
      tarifBulan: me.value ? hitungTarifBulan(tahunIni.value) : [] }
  : hitungTahun(tahunDilihat.value));

// tunggakan tahun-tahun sebelumnya, dipakai buat bayar (bukan cuma dilihat) —
// digabung lintas tahun biar bisa dilunasi sekaligus dalam satu konfirmasi.
const owedPastYears = computed(() => {
  const out = [];
  for (const tahun of historyYears.value) {
    hitungTahun(tahun).status.forEach((s, i) => {
      if (s === 'Belum' || s === 'Sebagian') out.push({ bulan: i + 1, tahun });
    });
  }
  return out.sort((a, b) => a.tahun - b.tahun || a.bulan - b.bulan);
});
// Jumlah persis (bukan "jumlah bulan × tarif sekarang") — tiap tahun bisa punya
// tarif berbeda (RumahRiwayat/TarifVersi), dan "Sebagian" cuma nyisa selisihnya,
// bukan tarif penuh.
const owedPastYearsTotal = computed(() =>
  historyYears.value.reduce((sum, tahun) => sum + hitungTahun(tahun).tunggakan, 0));

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

// bulan tahun depan yang belum ada baris Pembayaran-nya (API!Z, lihat sheets-schema.md §10)
const mukaDepan = computed(() => {
  if (!me.value) return [];
  const sudah = new Set(me.value.mukaTahunDepan || []);
  return Array.from({ length: 12 }, (_, i) => i + 1)
    .filter((b) => !sudah.has(b))
    .map((b) => ({ bulan: b, tahun: tahunIni.value + 1 }));
});

function openTransfer() {
  if (!owed.value.length && !owedPastYears.value.length && !muka.value.length && !mukaDepan.value.length) return;
  trMonths.value = owed.value.length ? [owed.value[0]]
    : owedPastYears.value.length ? [owedPastYears.value[0]] : [];
  trMuka.value = !owed.value.length && !owedPastYears.value.length;  // semua kewajiban lunas -> langsung buka bagian muka
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
  <section v-if="!me && !pendingHouse" class="scr col" style="gap:var(--space-3)">
    <h3 style="margin:0">Buka kartu iuran</h3>
    <p class="text-muted" style="font-size:13px;margin:0">
      Tidak perlu akun — masukkan blok dan nomor rumah Anda.
    </p>

    <div class="field">
      <label>Blok</label>
      <div style="display:grid;grid-template-columns:repeat(5,1fr);gap:var(--space-2)">
        <button v-for="b in BLOK_LIST" :key="b" type="button" class="btn"
                :class="inBlok === b ? 'btn-primary' : 'btn-secondary'"
                :style="chipStyle(inBlok === b)" @click="inBlok = b">
          N{{ b }}
        </button>
      </div>
    </div>
    <div class="field" style="max-width:160px">
      <label>No. Rumah</label>
      <input class="input" v-model="inRumah" inputmode="numeric" maxlength="2"
             placeholder="09" @keyup.enter="open">
    </div>
    <p class="text-muted num" style="font-size:11px;margin:0">
      Nomor rumah 2 digit — No. 1 ditulis <b>01</b>.
    </p>
    <Button block @click="open">Lihat kartu saya</Button>
    <p class="text-muted num" style="font-size:11px;margin:0">Contoh: Blok N7, No. 09.</p>
    <p v-if="notFound" class="text-muted" style="font-size:11.5px">Alamat tidak ditemukan.</p>
    <a href="#/ringkasan" class="text-muted" style="font-size:11.5px;text-align:center">
      Lihat ringkasan kas cluster →
    </a>
  </section>

  <!-- verifikasi PIN rumah -->
  <section v-else-if="pendingHouse" class="scr col"
           style="gap:var(--space-3);align-items:center;text-align:center;padding-top:var(--space-8)">
    <div style="width:56px;height:56px;border-radius:50%;background:var(--color-accent-2-200);
                color:var(--color-accent-2-800);display:flex;align-items:center;
                justify-content:center;font-size:22px">🔒</div>
    <h3 style="margin:0">Verifikasi PIN</h3>
    <p class="text-muted" style="font-size:13px;margin:0">
      <b>{{ pendingHouse.alamat }}</b><br>
      Masukkan 3 digit PIN rumah Anda untuk membuka kartu.
    </p>
    <input class="input num" v-model="pinInput" type="password" inputmode="numeric" maxlength="3"
           placeholder="•••" style="max-width:120px;text-align:center;font-size:20px;letter-spacing:.3em"
           autofocus @keyup.enter="submitPin">
    <Button style="max-width:220px" @click="submitPin">Buka kartu</Button>
    <p v-if="pinError" style="font-size:11.5px;color:var(--color-accent-700);margin:0">PIN salah, coba lagi.</p>
    <p class="text-muted" style="font-size:11px;margin:var(--space-2) 0 0;max-width:280px">
      Lupa PIN? Default-nya 3 digit terakhir no. HP yang terdaftar — kalau sudah diganti, hubungi bendahara.
    </p>
    <button class="btn btn-ghost" style="font-size:12px" @click="batalPin">← Ganti alamat</button>
  </section>

  <!-- kartu -->
  <section v-else class="scr col" style="gap:var(--space-3)">
    <div class="spread">
      <span class="kick">Kartu saya</span>
      <button class="btn btn-ghost" style="font-size:12px" @click="ganti">Ganti rumah</button>
    </div>

    <div style="border-radius:calc(var(--radius-lg)*1.15);overflow:hidden;
                background:var(--color-surface);box-shadow:var(--shadow-md)">
      <div :style="{ background: warnaKartu }" style="padding:var(--space-4);position:relative;overflow:hidden">
        <div style="position:absolute;right:-36px;top:-46px;width:150px;height:150px;border-radius:50%;
                    background:rgba(255,255,255,.18)"></div>
        <div style="position:relative;font-family:var(--font-heading);font-size:21px;color:#fff">
          {{ me.nama }}
        </div>
        <div class="row" style="position:relative;gap:var(--space-6);margin-top:var(--space-2)">
          <div>
            <div style="font-size:9.5px;letter-spacing:.1em;text-transform:uppercase;color:rgba(255,255,255,.75)">Alamat</div>
            <div class="num" style="font-size:13.5px;font-weight:700;color:#fff">
              Blok {{ me.cluster }}{{ me.blok }} · No. {{ me.rumah }}
            </div>
          </div>
          <div>
            <div style="font-size:9.5px;letter-spacing:.1em;text-transform:uppercase;color:rgba(255,255,255,.75)">Luas / Tarif</div>
            <div class="num" style="font-size:13.5px;font-weight:700;color:#fff">
              {{ me.luas }} m² · {{ rupiahPendek(me.tarif) }}/bln
            </div>
          </div>
        </div>
      </div>

      <div style="padding:var(--space-4)">
        <div class="spread" style="margin-bottom:var(--space-2)">
          <span class="kick">Tahun {{ tahunDilihat }}</span>
          <Tag :status="tahunData.tunggakan > 0 ? 'Sebagian' : 'Lunas'">
            {{ tahunData.tunggakan > 0 ? rupiahPendek(tahunData.tunggakan) + ' belum dibayar' : 'Lunas' }}
          </Tag>
        </div>

        <!-- lihat kartu tahun-tahun sebelumnya — cuma muncul kalau ada datanya -->
        <div v-if="historyYears.length" class="row" style="gap:6px;flex-wrap:wrap;margin-bottom:var(--space-3)">
          <button type="button" class="btn" :class="tahunDilihat === tahunIni ? 'btn-primary' : 'btn-secondary'"
                  style="min-height:32px;padding:5px 12px;font-size:12px" @click="tahunDilihat = tahunIni">
            {{ tahunIni }}
          </button>
          <button v-for="y in historyYears" :key="y" type="button" class="btn"
                  :class="tahunDilihat === y ? 'btn-primary' : 'btn-secondary'"
                  style="min-height:32px;padding:5px 12px;font-size:12px" @click="tahunDilihat = y">
            {{ y }}
          </button>
        </div>

        <div class="months">
          <div v-for="(s, i) in tahunData.status" :key="i" class="month" :class="cls(s)">
            <div style="font-size:12.5px;font-weight:700">{{ BULAN[i] }}</div>
            <div class="num" style="font-size:10px;opacity:.8">{{ s === '-' ? '—' : s }}</div>
            <div v-if="tahunData.tarifBulan[i] != null" class="num"
                 style="font-size:9px;opacity:.65">{{ rupiahPendek(tahunData.tarifBulan[i]) }}</div>
          </div>
        </div>
        <p v-if="tahunDilihat !== tahunIni && !tahunData.dataLengkap" class="text-muted"
           style="font-size:10px;margin:var(--space-2) 0 0">
          Sebagian bulan belum ada catatan riwayat luas/tarif sejauh itu di Sheet — ditampilkan "—".
        </p>
      </div>
    </div>

    <Card>
      <span style="font-size:13.5px;font-weight:700">Tagihan berjalan</span>
      <div class="col" style="gap:5px">
        <div class="spread" style="font-size:12.5px">
          <span class="text-muted">ISLK ({{ me.luas }} m²)</span>
          <span class="num">{{ rupiah(me.tarif - (rateCardAktif?.iuranRt || 0)) }}</span>
        </div>
        <div class="spread" style="font-size:12.5px">
          <span class="text-muted">Iuran RT</span>
          <span class="num">{{ rupiah(rateCardAktif?.iuranRt || 0) }}</span>
        </div>
        <div class="hr" style="margin:2px 0"></div>
        <div class="spread">
          <span style="font-size:13px;font-weight:700">Total tagihan {{ tahunIni }}</span>
          <span class="num" style="font-family:var(--font-heading);font-size:24px;color:var(--color-accent-700)">
            {{ rupiah(me.tunggakan) }}
          </span>
        </div>
        <div v-if="owedPastYears.length" class="spread" style="font-size:12px">
          <span class="text-muted">+ tunggakan tahun sebelumnya ({{ owedPastYears.length }} bln)</span>
          <span class="num" style="font-weight:700;color:var(--color-accent-700)">
            {{ rupiah(owedPastYearsTotal) }}
          </span>
        </div>
      </div>
      <Button block :disabled="!owed.length && !owedPastYears.length && !muka.length && !mukaDepan.length"
              @click="openTransfer">
        {{ owed.length || owedPastYears.length ? 'Bayar transfer' : 'Bayar di muka' }}
      </Button>
      <div class="text-muted num" style="text-align:center;font-size:10.5px">
        {{ REKENING.bank }} {{ REKENING.nomor }} a.n. {{ REKENING.nama }} · jatuh tempo tgl 20
      </div>
    </Card>

    <!-- side sheet: konfirmasi transfer + bukti -->
    <div v-if="trOpen" class="dialog-backdrop sheet-backdrop" @click.self="trOpen = false">
      <div class="dialog sheet" style="width:100%;max-width:480px;
           border-radius:var(--radius-lg) var(--radius-lg) 0 0;
           max-height:92dvh;overflow-y:auto">
        <div class="spread">
          <div>
            <div class="dialog-title">Konfirmasi Transfer</div>
            <div class="text-muted num" style="font-size:12px">{{ me.alamat }} · {{ me.nama }}</div>
          </div>
          <button class="btn btn-ghost" @click="trOpen = false">×</button>
        </div>

        <RekeningCard />

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

        <!-- tunggakan lintas tahun — selalu tampil kalau ada, bukan disembunyikan
             di balik toggle "+", karena ini kewajiban lama yang mudah terlewat -->
        <template v-if="owedPastYears.length">
          <div class="kick" style="color:var(--color-accent-700)">
            Tunggakan tahun sebelumnya ({{ owedPastYears.length }} bulan)
          </div>
          <div class="row" style="flex-wrap:wrap;gap:var(--space-2)">
            <button v-for="m in owedPastYears" :key="keyOf(m)" class="btn"
                    :class="trKeys.has(keyOf(m)) ? 'btn-primary' : 'btn-secondary'"
                    @click="toggleTr(m)">
              {{ BULAN[m.bulan - 1].slice(0, 3) }} '{{ String(m.tahun).slice(2) }}
            </button>
          </div>
        </template>

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
