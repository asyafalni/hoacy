<script setup vapor>
import { ref, computed, watch } from 'vue';
import { useSheet } from '../composables/useSheet';
import { useScrollLock } from '../composables/useScrollLock';
import { BULAN, rupiah, rupiahPendek, alamat, REKENING, BLOK_LIST, BLOK_WARNA_DEFAULT } from '../lib/tariff';
import { urlTransfer } from '../lib/forms';
import { labelBulan, tahunOf, geserPeriode } from '../lib/tagihan';
import Card from '../components/ui/Card.vue';
import Tag from '../components/ui/Tag.vue';
import Button from '../components/ui/Button.vue';
import RekeningCard from '../components/RekeningCard.vue';

const { rumah, blokWarna, rateCardAktif, TAHUN, sekarang } = useSheet();

// No login, but blok+rumah alone is guessable by any resident — so opening a card
// also needs that house's PIN (Rumah!G in the Sheet, defaults to the last 3 digits
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
const storedAlamat = localStorage.getItem('iuran.alamat') || '';
const initialAlamat = qsAlamat || storedAlamat;

const key = ref('');
const inBlok = ref('');
const inRumah = ref('');
const notFound = ref(false);
const pendingHouse = ref(null);
const pinInput = ref('');
const pinError = ref(false);

const me = computed(() => rumah.value.find((h) => h.alamat === key.value));

// Each block gets its own color (admin-set in the Sheet, Blok tab — see
// docs/sheets-schema.md `Blok`), applied to the card header here and to the house
// badge in PosSatpam.vue.
const warnaKartu = computed(() =>
  (me.value && (blokWarna.value[String(me.value.blok)] || BLOK_WARNA_DEFAULT[String(me.value.blok)]))
  || '#c67139');

function tryUnlock(id, { dariSimpanan = false } = {}) {
  const found = rumah.value.find((h) => h.alamat === id);
  if (!found) {
    // A remembered address that's since been deactivated: forget it quietly
    // instead of greeting the resident with "Alamat tidak ditemukan".
    if (dariSimpanan) { localStorage.removeItem('iuran.alamat'); return; }
    notFound.value = true;
    return;
  }
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
// "Ganti rumah" also forgets this house's PIN on this device — on a shared
// phone, the next person must enter it again to reopen the card.
function ganti() {
  localStorage.removeItem(pinOkKey(key.value));
  localStorage.removeItem('iuran.alamat');
  key.value = '';
}

// ?alamat= / remembered address: try it once the Sheet has loaded — still goes
// through tryUnlock, so a QR-code link alone can't skip the PIN on a new device.
watch(rumah, (list) => {
  if (list.length && initialAlamat && !key.value && !pendingHouse.value) {
    tryUnlock(initialAlamat, { dariSimpanan: !qsAlamat });
  }
}, { immediate: true });

const cls = (s) => ({ Lunas: 'lunas', Pending: 'pending', Belum: 'belum' }[s] || 'kosong');
const label = (p) => labelBulan(p, TAHUN.value, BULAN);

// ── Kartu per tahun ───────────────────────────────────────────────────────────
// Status + tarif per month for any year come from src/lib/tagihan.js — the same
// lookup Pos, Kas and /sum use; a past month is billed at the rate in force then.
// Year tabs start at the house's first RumahRiwayat row (when billing began).
const tahunIni = computed(() => TAHUN.value);
const historyYears = computed(() => {
  if (!me.value?.mulai) return [];
  const years = [];
  for (let y = tahunIni.value - 1; y >= tahunOf(me.value.mulai); y -= 1) years.push(y);
  return years;
});
const tahunDilihat = ref(0);
watch(me, (h) => { if (h) tahunDilihat.value = tahunIni.value; });
const tahunData = computed(() => {
  if (!me.value) return { status: [], tarifBulan: [], tunggakan: 0 };
  const y = tahunDilihat.value || tahunIni.value;
  return {
    ...me.value.kartuTahun(y),
    tunggakan: me.value.tunggakanList.filter((t) => tahunOf(t.periode) === y)
      .reduce((sum, t) => sum + t.tarif, 0),
  };
});
const tunggakanLalu = computed(() => !me.value ? [] :
  me.value.tunggakanList.filter((t) => tahunOf(t.periode) < tahunIni.value));

// ── Konfirmasi transfer ───────────────────────────────────────────────────────
// Bukti transfer is a file, and a Sheet cannot hold one. So the sheet below
// collects months + a local preview, then opens ONE prefilled "Konfirmasi
// Transfer" Form covering every chosen month (its "rincian" answer) whose last
// question is the bukti upload — one transfer, one bukti, one submission.
const trOpen = ref(false);
useScrollLock(trOpen);
const trFile = ref(null);
const camInput = ref(null);
const fileInput = ref(null);
const trMuka = ref(false);       // "bayar di muka": reveals not-yet-due months, off by default
const trMukaDepan = ref(false);  // nested further: reveals next year's months

// Owed = every unpaid due month since billing started (all years, oldest first).
const owed = computed(() => me.value?.tunggakanList.map((t) => t.periode) || []);

// Not yet due, not already paid/pending, with a known tarif — rest of this
// year, then next year (hidden behind the "bayar di muka" toggles).
const bisaMuka = computed(() => {
  if (!me.value) return [];
  const out = [];
  for (let p = geserPeriode(sekarang.value, 1); tahunOf(p) <= tahunIni.value + 1; p = geserPeriode(p, 1)) {
    if (me.value.statusPada(p) === '-' && me.value.tarifPer(p) != null) out.push(p);
  }
  return out;
});
const muka = computed(() => bisaMuka.value.filter((p) => tahunOf(p) === tahunIni.value));
const mukaDepan = computed(() => bisaMuka.value.filter((p) => tahunOf(p) > tahunIni.value));
const bisaBayar = computed(() => me.value?.tarifDiatur && (owed.value.length || bisaMuka.value.length));

const trMonths = ref([]);   // periodes
function openTransfer() {
  if (!bisaBayar.value) return;
  trMonths.value = owed.value.slice(0, 1);
  trMuka.value = !owed.value.length;                    // semua kewajiban lunas -> langsung buka bagian muka
  trMukaDepan.value = trMuka.value && !muka.value.length;  // tahun ini juga tuntas -> langsung ke tahun depan
  trFile.value = null;
  trOpen.value = true;
}
function toggleTr(p) {
  trMonths.value = trMonths.value.includes(p)
    ? trMonths.value.filter((x) => x !== p)
    : [...trMonths.value, p].sort((a, b) => a - b);
}
function pickFile(e) { trFile.value = e.target.files?.[0] || null; e.target.value = ''; }
function openCamera() { camInput.value?.click(); }
function openGallery() { fileInput.value?.click(); }

// Each month at its own tarif — an old or pre-upgrade month is billed at the
// rate in force then (RumahRiwayat/TarifVersi), always the full amount.
const trItems = computed(() => !me.value ? [] :
  trMonths.value.map((p) => ({ periode: p, nominal: me.value.tarifPer(p) })));
const trTotal = computed(() => trItems.value.reduce((sum, i) => sum + i.nominal, 0));
const trReady = computed(() => !!trFile.value && trItems.value.length > 0);

// One window.open straight from the tap — a user gesture, so popup blockers
// let it through (several delayed opens used to lose every month but the first).
function kirimKonfirmasi() {
  if (!trReady.value) return;
  window.open(urlTransfer({ alamat: me.value.alamat, items: trItems.value }), '_blank');
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
    <a href="#/sum" class="text-muted" style="font-size:11.5px;text-align:center">
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
              {{ me.tarifDiatur ? `${me.luas} m² · ${rupiahPendek(me.tarif)}/bln` : 'Belum diatur' }}
            </div>
          </div>
        </div>
      </div>

      <div style="padding:var(--space-4)">
        <div class="spread" style="margin-bottom:var(--space-2)">
          <span class="kick">Tahun {{ tahunDilihat }}</span>
          <Tag :status="tahunData.tunggakan > 0 ? 'Tunggak' : 'Lunas'">
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
      </div>
    </div>

    <Card v-if="!me.tarifDiatur">
      <span style="font-size:13.5px;font-weight:700">Tarif belum diatur</span>
      <p class="text-muted" style="font-size:12px;margin:0">
        Luas/tipe rumah ini belum dicatat pengurus, jadi tagihannya belum bisa dihitung.
        Hubungi bendahara — pembayaran lewat aplikasi dibuka setelah tarifnya diatur.
      </p>
    </Card>
    <Card v-else>
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
          <span style="font-size:13px;font-weight:700">Total tunggakan</span>
          <span class="num" style="font-family:var(--font-heading);font-size:24px;color:var(--color-accent-700)">
            {{ rupiah(me.tunggakan) }}
          </span>
        </div>
        <div v-if="tunggakanLalu.length" class="spread" style="font-size:12px">
          <span class="text-muted">termasuk tahun sebelumnya ({{ tunggakanLalu.length }} bln)</span>
          <span class="num" style="font-weight:700;color:var(--color-accent-700)">
            {{ rupiah(tunggakanLalu.reduce((sum, t) => sum + t.tarif, 0)) }}
          </span>
        </div>
      </div>
      <Button block :disabled="!bisaBayar" @click="openTransfer">
        {{ owed.length ? 'Bayar transfer' : 'Bayar di muka' }}
      </Button>
      <div class="text-muted num" style="text-align:center;font-size:10.5px">
        {{ REKENING.bank }} {{ REKENING.nomor }} a.n. {{ REKENING.nama }} · jatuh tempo tgl 20
      </div>
    </Card>

    <!-- side sheet: konfirmasi transfer + bukti -->
    <div v-if="trOpen" class="dialog-backdrop sheet-backdrop" @click.self="trOpen = false">
      <div class="dialog sheet" style="width:100%;max-width:480px;
           border-radius:var(--radius-lg) var(--radius-lg) 0 0;
           max-height:92dvh">
       <div class="sheet-scroll">
        <div class="spread">
          <div>
            <div class="dialog-title">Konfirmasi Transfer</div>
            <div class="text-muted num" style="font-size:12px">{{ me.alamat }} · {{ me.nama }}</div>
          </div>
          <button class="btn btn-ghost" @click="trOpen = false">×</button>
        </div>

        <RekeningCard />

        <!-- tunggakan semua tahun, paling lama duluan — selalu tampil, bukan
             disembunyikan di balik toggle, karena kewajiban lama mudah terlewat -->
        <template v-if="owed.length">
          <div class="kick">Tunggakan ({{ owed.length }} bulan)</div>
          <div class="row" style="flex-wrap:wrap;gap:var(--space-2)">
            <button v-for="p in owed" :key="p" class="btn"
                    :class="trMonths.includes(p) ? 'btn-primary' : 'btn-secondary'"
                    @click="toggleTr(p)">
              {{ label(p) }}
            </button>
          </div>
        </template>
        <p v-else class="text-muted" style="font-size:12px;margin:0">
          Tidak ada tunggakan — lanjut bayar di muka di bawah.
        </p>

        <button v-if="!trMuka && muka.length" type="button" class="btn btn-ghost"
                style="justify-content:flex-start;font-size:12px;padding-left:0" @click="trMuka = true">
          + Bayar di muka untuk bulan berikutnya
        </button>
        <template v-if="trMuka && muka.length">
          <div class="kick">Bayar di muka ({{ tahunIni }}, belum jatuh tempo)</div>
          <div class="row" style="flex-wrap:wrap;gap:var(--space-2)">
            <button v-for="p in muka" :key="p" class="btn"
                    :class="trMonths.includes(p) ? 'btn-primary' : 'btn-secondary'"
                    @click="toggleTr(p)">
              {{ label(p) }}
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
            <button v-for="p in mukaDepan" :key="p" class="btn"
                    :class="trMonths.includes(p) ? 'btn-primary' : 'btn-secondary'"
                    @click="toggleTr(p)">
              {{ label(p) }}
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
    </div>
  </section>
</template>
