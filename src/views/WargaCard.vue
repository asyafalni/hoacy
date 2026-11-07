<script setup vapor>
import { ref, computed, watch, onMounted } from 'vue';
import { useScrollLock } from '../composables/useScrollLock';
import { api, ApiError, sesiWarga, simpanSesiWarga } from '../lib/api';
import { BULAN, rupiah, rupiahPendek, alamat, REKENING, BLOK_LIST, BLOK_WARNA_DEFAULT } from '../lib/tariff';
import { urlWaOperasional } from '../lib/tautan';
import { labelBulan, tahunOf } from '../lib/tagihan';
import Card from '../components/ui/Card.vue';
import Tag from '../components/ui/Tag.vue';
import Button from '../components/ui/Button.vue';
import RekeningCard from '../components/RekeningCard.vue';

// No account, but blok+rumah alone is guessable — so a card needs that house's
// 6-digit PIN (default: the last 6 digits of the registered phone). The server
// checks it, logs every attempt, locks a house (and an IP) after repeated
// misses, and answers an unknown address, an inactive one and a wrong PIN the
// same way. After that the phone keeps a session token, never the PIN; changing
// the PIN signs every other phone of that house out.
// ?alamat=N7-09 opens a house's own QR link (the PIN is still asked).
const CLUSTER = 'N';
const chipStyle = (active) => active ? 'min-height:42px'
  : 'min-height:42px;background:var(--color-surface);box-shadow:var(--shadow-sm)';
const qsAlamat = new URLSearchParams(location.search).get('alamat') || '';

const data = ref(null);          // /api/warga — this house only
const me = computed(() => (data.value ? data.value.rumah : null));
const inBlok = ref('');
const inRumah = ref('');
const pendingAlamat = ref('');   // alamat waiting for its PIN
const pinInput = ref('');
const pesan = ref('');
const memeriksa = ref(false);

const warnaKartu = computed(() =>
  (me.value && (data.value.warna || BLOK_WARNA_DEFAULT[String(me.value.blok)])) || '#c67139');

async function muatKartu() {
  memeriksa.value = true;
  try {
    data.value = await api('/api/warga', { pakai: 'warga' });
    return true;
  } catch (e) {
    if (e instanceof ApiError && e.status === 401) {
      simpanSesiWarga(null);   // PIN changed or house deactivated: sign in again
      pesan.value = sesiTadi ? 'Sesi di perangkat ini sudah berakhir — silakan masuk lagi.' : '';
    } else {
      pesan.value = e.message;
    }
    return false;
  } finally {
    memeriksa.value = false;
  }
}
let sesiTadi = false;

async function masuk(id, pin) {
  memeriksa.value = true;
  pesan.value = '';
  try {
    const r = await api('/api/warga/masuk', { method: 'POST', body: { alamat: id, pin } });
    simpanSesiWarga({ alamat: id, token: r.token });
    pendingAlamat.value = '';
    await muatKartu();
  } catch (e) {
    pesan.value = e.message;
    memeriksa.value = false;
  }
}

function mulai(id) {
  pesan.value = '';
  pendingAlamat.value = id;
  pinInput.value = '';
}
function open() {
  const rumahNum = inRumah.value.trim();
  if (!inBlok.value || !rumahNum) { pesan.value = 'Pilih blok dan isi nomor rumah.'; return; }
  mulai(alamat({ cluster: CLUSTER, blok: inBlok.value, rumah: rumahNum }));
}
function submitPin() {
  const pin = pinInput.value.trim();
  if (pin.length !== 6 || memeriksa.value) { pesan.value = pin ? 'PIN terdiri dari 6 angka.' : ''; return; }
  pinInput.value = '';
  masuk(pendingAlamat.value, pin);
}
function batalPin() { pendingAlamat.value = ''; pinInput.value = ''; pesan.value = ''; }
// "Ganti rumah" signs this phone out of the house — on a shared phone the next
// person must enter the PIN again.
function ganti() {
  simpanSesiWarga(null);
  data.value = null;
  pesan.value = '';
}

onMounted(async () => {
  const s = sesiWarga();
  if (s && (!qsAlamat || qsAlamat === s.alamat)) {
    sesiTadi = true;
    if (await muatKartu()) return;
  }
  if (qsAlamat) mulai(qsAlamat);
});
// back in the foreground after a while → fresh status
document.addEventListener('visibilitychange', () => {
  if (document.visibilityState === 'visible' && data.value) muatKartu();
});

const cls = (s) => ({ Lunas: 'lunas', Pending: 'pending', Belum: 'belum' }[s] || 'kosong');
const tahunIni = computed(() => (data.value ? data.value.hari.tahun : new Date().getFullYear()));
const label = (p) => labelBulan(p, tahunIni.value, BULAN);

// ── Kartu per tahun — computed by the server for every year since billing began ──
const kartuPer = computed(() => Object.fromEntries((me.value?.kartu || []).map((k) => [k.tahun, k])));
const historyYears = computed(() => (me.value?.kartu || [])
  .filter((k) => k.tahun < tahunIni.value && k.status.some((s) => s !== '-'))
  .map((k) => k.tahun).sort((a, b) => b - a));
const tahunDilihat = ref(0);
watch(me, (h) => { if (h && !tahunDilihat.value) tahunDilihat.value = tahunIni.value; });
const tahunData = computed(() => {
  const y = tahunDilihat.value || tahunIni.value;
  const k = kartuPer.value[y] || { status: Array(12).fill('-'), tarifBulan: Array(12).fill(null) };
  return {
    ...k,
    tunggakan: (me.value?.tunggakanList || []).filter((t) => tahunOf(t.periode) === y).reduce((s, t) => s + t.tarif, 0),
  };
});
const tunggakanLalu = computed(() => !me.value ? [] :
  me.value.tunggakanList.filter((t) => tahunOf(t.periode) < tahunIni.value));

// ── Konfirmasi transfer: months + one proof photo, straight to the server ──
const trOpen = ref(false);
useScrollLock(trOpen);
const trFile = ref(null);
const camInput = ref(null);
const fileInput = ref(null);
const trMuka = ref(false);       // "bayar di muka": reveals not-yet-due months, off by default
const trMukaDepan = ref(false);  // nested further: reveals next year's months
const trPesan = ref('');
const mengirim = ref(false);
const terkirim = ref(null);      // { total, bulan } after a successful upload

const owed = computed(() => me.value?.tunggakanList.map((t) => t.periode) || []);
const tarifOf = computed(() => {
  const m = {};
  for (const t of me.value?.tunggakanList || []) m[t.periode] = t.tarif;
  for (const t of me.value?.muka || []) m[t.periode] = t.tarif;
  return m;
});
const muka = computed(() => (me.value?.muka || []).filter((t) => tahunOf(t.periode) === tahunIni.value).map((t) => t.periode));
const mukaDepan = computed(() => (me.value?.muka || []).filter((t) => tahunOf(t.periode) > tahunIni.value).map((t) => t.periode));
const bisaBayar = computed(() => me.value?.tarifDiatur && (owed.value.length || me.value.muka.length));

const trMonths = ref([]);   // periodes
function openTransfer() {
  if (!bisaBayar.value) return;
  trMonths.value = owed.value.slice(0, 1);
  trMuka.value = !owed.value.length;
  trMukaDepan.value = trMuka.value && !muka.value.length;
  trFile.value = null;
  trPesan.value = '';
  terkirim.value = null;
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

// Each month at its own tarif, always the full amount — the server checks it again.
const trTotal = computed(() => trMonths.value.reduce((s, p) => s + (tarifOf.value[p] || 0), 0));
const trReady = computed(() => !!trFile.value && trMonths.value.length > 0 && !mengirim.value);

async function kirimKonfirmasi() {
  if (!trReady.value) return;
  if (trFile.value.size > 10 * 1024 * 1024) { trPesan.value = 'Ukuran file maksimal 10 MB.'; return; }
  const fd = new FormData();
  trMonths.value.forEach((p) => fd.append('periode', String(p)));
  fd.append('bukti', trFile.value);
  mengirim.value = true;
  trPesan.value = '';
  try {
    const r = await api('/api/warga/transfer', { method: 'POST', pakai: 'warga', form: fd });
    terkirim.value = { total: r.total, bulan: trMonths.value.map(label).join(', ') };
    await muatKartu();
  } catch (e) {
    trPesan.value = e.message;
    if (e.status === 401) { trOpen.value = false; data.value = null; }
  } finally {
    mengirim.value = false;
  }
}

// ── Ganti PIN ──
const pinOpen = ref(false);
useScrollLock(pinOpen);
const pinLama = ref('');
const pinBaru = ref('');
const pinUlang = ref('');
const pinPesan = ref('');
function bukaGantiPin() { pinLama.value = pinBaru.value = pinUlang.value = pinPesan.value = ''; pinOpen.value = true; }
async function gantiPin() {
  if (!/^\d{6}$/.test(pinBaru.value)) { pinPesan.value = 'PIN baru harus 6 angka.'; return; }
  if (pinBaru.value !== pinUlang.value) { pinPesan.value = 'Ulangi PIN baru dengan sama persis.'; return; }
  try {
    const r = await api('/api/warga/pin', { method: 'POST', pakai: 'warga', body: { lama: pinLama.value, baru: pinBaru.value } });
    simpanSesiWarga({ alamat: me.value.alamat, token: r.token });
    data.value = { ...data.value, pinBawaan: false };
    pinOpen.value = false;
  } catch (e) { pinPesan.value = e.message; }
}
</script>

<template>
  <section v-if="memeriksa && !pendingAlamat && !me" class="scr col"
           style="align-items:center;padding-top:var(--space-8)">
    <p class="text-muted" style="font-size:13px">Memuat kartu…</p>
  </section>

  <!-- masuk: blok + nomor rumah, tanpa akun -->
  <section v-else-if="!me && !pendingAlamat" class="scr col" style="gap:var(--space-3)">
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
    <p v-if="pesan" class="text-muted" style="font-size:11.5px;color:var(--color-accent-700)">{{ pesan }}</p>
    <a href="#/sum" class="text-muted" style="font-size:11.5px;text-align:center">
      Lihat ringkasan kas cluster →
    </a>
  </section>

  <!-- verifikasi PIN rumah -->
  <section v-else-if="pendingAlamat" class="scr col"
           style="gap:var(--space-3);align-items:center;text-align:center;padding-top:var(--space-8)">
    <div style="width:56px;height:56px;border-radius:50%;background:var(--color-accent-2-200);
                color:var(--color-accent-2-800);display:flex;align-items:center;
                justify-content:center;font-size:22px">🔒</div>
    <h3 style="margin:0">Verifikasi PIN</h3>
    <p class="text-muted" style="font-size:13px;margin:0">
      <b>{{ pendingAlamat }}</b><br>
      Masukkan 6 digit PIN rumah Anda untuk membuka kartu.
    </p>
    <input class="input num" v-model="pinInput" type="password" inputmode="numeric" maxlength="6"
           placeholder="••••••" style="max-width:120px;text-align:center;font-size:20px;letter-spacing:.3em"
           autofocus @keyup.enter="submitPin">
    <Button style="max-width:220px" :disabled="memeriksa" @click="submitPin">
      {{ memeriksa ? 'Memeriksa…' : 'Buka kartu' }}
    </Button>
    <p v-if="pesan" style="font-size:11.5px;color:var(--color-accent-700);margin:0">{{ pesan }}</p>
    <p class="text-muted" style="font-size:11px;margin:var(--space-2) 0 0;max-width:280px">
      Lupa PIN? Default-nya 6 digit terakhir no. HP yang terdaftar — kalau sudah diganti, minta pengurus meresetnya.
    </p>
    <button class="btn btn-ghost" style="font-size:12px" @click="batalPin">← Ganti alamat</button>
  </section>

  <!-- kartu -->
  <section v-else class="scr col" style="gap:var(--space-3)">
    <div class="spread">
      <span class="kick">Kartu saya</span>
      <div class="row" style="gap:2px">
        <button class="btn btn-ghost" style="font-size:12px" @click="bukaGantiPin">Ganti PIN</button>
        <button class="btn btn-ghost" style="font-size:12px" @click="ganti">Ganti rumah</button>
      </div>
    </div>

    <Card v-if="data.pinBawaan" style="background:var(--color-accent-100);gap:4px">
      <span style="font-size:12.5px;font-weight:700;color:var(--color-accent-800)">PIN Anda masih bawaan</span>
      <span style="font-size:11.5px">
        PIN bawaan = 6 digit terakhir no. HP — mudah ditebak orang yang tahu nomor Anda.
        <a href="#" @click.prevent="bukaGantiPin">Ganti sekarang</a>.
      </span>
    </Card>

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
        <div class="spread" style="margin-bottom:var(--space-3)">
          <!-- tahun-tahun sebelumnya — dropdown, karena daftarnya terus bertambah -->
          <label v-if="historyYears.length" class="row" style="gap:8px;align-items:center">
            <span class="kick">Tahun</span>
            <select v-model.number="tahunDilihat" class="input" style="min-height:32px;font-size:13px">
              <option :value="tahunIni">{{ tahunIni }}</option>
              <option v-for="y in historyYears" :key="y" :value="y">{{ y }}</option>
            </select>
          </label>
          <span v-else class="kick">Tahun {{ tahunDilihat }}</span>
          <Tag :status="tahunData.tunggakan > 0 ? 'Tunggak' : 'Lunas'">
            {{ tahunData.tunggakan > 0 ? rupiahPendek(tahunData.tunggakan) + ' belum dibayar' : 'Lunas' }}
          </Tag>
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
          <span class="num">{{ rupiah(me.rateCard ? me.rateCard.islk : me.tarif) }}</span>
        </div>
        <div class="spread" style="font-size:12.5px">
          <span class="text-muted">Iuran RT</span>
          <span class="num">{{ rupiah(me.rateCard ? me.rateCard.iuranRt : 0) }}</span>
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

        <template v-if="terkirim">
          <div style="text-align:center;padding:var(--space-4) 0" class="col">
            <div style="font-size:34px">✓</div>
            <div class="dialog-title">Konfirmasi terkirim</div>
            <p class="text-muted" style="font-size:12.5px;margin:0">
              {{ terkirim.bulan }} · {{ rupiah(terkirim.total) }} — menunggu verifikasi pengurus.
            </p>
          </div>
          <Button block @click="trOpen = false">Tutup</Button>
        </template>
        <template v-else>
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

        <p v-if="trPesan" style="font-size:12px;color:var(--color-accent-700);margin:0">{{ trPesan }}</p>
        <Button block :variant="trReady ? 'primary' : 'secondary'" :disabled="mengirim" @click="kirimKonfirmasi">
          {{ mengirim ? 'Mengirim bukti…' : 'Kirim konfirmasi' }}
        </Button>
        <div class="text-muted" style="text-align:center;font-size:10.5px">
          {{ trReady || mengirim
             ? 'Setelah terkirim, status menjadi Pending sampai diverifikasi pengurus'
             : 'Lampirkan bukti transfer untuk melanjutkan' }}
        </div>
        <a class="text-muted" target="_blank" style="text-align:center;font-size:11.5px"
           :href="urlWaOperasional(`Halo pengurus, saya ${me.nama} (${me.alamat}). Ada kendala saat konfirmasi transfer: `)">
          Ada kendala kirim bukti? <b style="color:var(--color-accent-700)">Hubungi pengurus via WhatsApp</b>
        </a>
        </template>
       </div>
      </div>
    </div>
    <!-- ganti PIN -->
    <div v-if="pinOpen" class="dialog-backdrop sheet-backdrop" @click.self="pinOpen = false">
      <div class="dialog sheet" style="width:100%;max-width:480px;border-radius:var(--radius-lg) var(--radius-lg) 0 0;max-height:92dvh">
       <div class="sheet-scroll">
        <div class="spread">
          <div class="dialog-title">Ganti PIN</div>
          <button class="btn btn-ghost" @click="pinOpen = false">×</button>
        </div>
        <p class="text-muted" style="font-size:12px;margin:0">
          6 angka. Setelah diganti, HP lain yang membuka kartu ini perlu masuk lagi dengan PIN baru.
        </p>
        <div class="field"><label>PIN sekarang</label>
          <input class="input num" v-model="pinLama" type="password" inputmode="numeric" maxlength="6"></div>
        <div class="field"><label>PIN baru</label>
          <input class="input num" v-model="pinBaru" type="password" inputmode="numeric" maxlength="6"></div>
        <div class="field"><label>Ulangi PIN baru</label>
          <input class="input num" v-model="pinUlang" type="password" inputmode="numeric" maxlength="6" @keyup.enter="gantiPin"></div>
        <p v-if="pinPesan" style="font-size:12px;color:var(--color-accent-700);margin:0">{{ pinPesan }}</p>
        <Button block @click="gantiPin">Simpan PIN baru</Button>
       </div>
      </div>
    </div>
  </section>
</template>
