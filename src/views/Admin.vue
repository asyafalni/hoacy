<script setup vapor>
// Admin: the master data that used to be typed into the Sheet (houses, their
// luas/tipe history, rate cards, block colors, pengurus), the manual Sheet
// sync, and the security log. Every change is checked by the server and lands
// in the audit log (tab Log).
import { ref, computed, watch } from 'vue';
import { useSaya } from '../composables/useKas';
import { api, ApiError } from '../lib/api';
import { rupiah, BLOK_LIST, BLOK_WARNA_DEFAULT, BULAN } from '../lib/tariff';
import { waktuTeks } from '../lib/tagihan';
import Card from '../components/ui/Card.vue';
import Button from '../components/ui/Button.vue';
import PengurusGate from '../components/PengurusGate.vue';

const { saya, status, boleh } = useSaya();
const TAB = ['Rumah', 'Tarif', 'Pengurus', 'Sinkron', 'Keamanan', 'Log'];
const tab = ref('Rumah');
const pesan = ref('');
const ok = ref('');
const sibuk = ref(false);

async function jalan(fn, berhasil = '') {
  sibuk.value = true; pesan.value = ''; ok.value = '';
  try { await fn(); if (berhasil) ok.value = berhasil; }
  catch (e) { pesan.value = e instanceof ApiError ? e.message : 'Gagal — coba lagi.'; }
  finally { sibuk.value = false; }
}
const A = (path, opt = {}) => api(path, { pakai: 'pengurus', ...opt });
const periodeDariBulan = (v) => (v ? Number(v.slice(0, 4)) * 100 + Number(v.slice(5, 7)) : 0);   // "2026-09" → 202609
const bulanDariPeriode = (p) => `${Math.floor(p / 100)}-${String(p % 100).padStart(2, '0')}`;
const labelPeriode = (p) => `${BULAN[(p % 100) - 1].slice(0, 3)} ${Math.floor(p / 100)}`;

// ── Rumah ──
const rumah = ref([]);
const cari = ref('');
const pilihAlamat = ref('');
const pilih = computed(() => rumah.value.find((r) => r.alamat === pilihAlamat.value));
const daftarRumah = computed(() => rumah.value.filter((r) => !cari.value
  || r.alamat.toLowerCase().includes(cari.value.toLowerCase()) || r.nama.toLowerCase().includes(cari.value.toLowerCase())));
const edit = ref({ nama: '', telp: '' });
const riwayatBaru = ref({ bulan: '', luas: '', tipe: 'rumah' });
const konfirmasi = ref('');
const pinAcak = ref('');
const rumahBaru = ref({ blok: '', rumah: '', nama: '', telp: '' });
async function muatRumah() { rumah.value = (await A('/api/admin/rumah')).rumah; }
watch(pilih, (r) => {
  if (!r) return;
  edit.value = { nama: r.nama, telp: r.telp || '' };
  riwayatBaru.value = { bulan: '', luas: '', tipe: 'rumah' };
  konfirmasi.value = ''; pinAcak.value = '';
});
const simpanRumah = () => jalan(async () => {
  await A(`/api/admin/rumah/${pilih.value.alamat}`, { method: 'PUT', body: edit.value });
  await muatRumah();
}, 'Data rumah disimpan.');
const tambahRiwayat = () => jalan(async () => {
  await A(`/api/admin/rumah/${pilih.value.alamat}/riwayat`, { method: 'POST',
    body: { berlaku: periodeDariBulan(riwayatBaru.value.bulan), luas: Number(riwayatBaru.value.luas), tipe: riwayatBaru.value.tipe } });
  await muatRumah();
}, 'Riwayat ditambahkan.');
const hapusRiwayat = (b) => confirm(`Hapus baris riwayat ${labelPeriode(b)}? Tagihan bulan-bulan itu dihitung ulang.`)
  && jalan(async () => { await A(`/api/admin/rumah/${pilih.value.alamat}/riwayat/${b}`, { method: 'DELETE' }); await muatRumah(); }, 'Riwayat dihapus.');
const resetPin = () => confirm(`Reset PIN ${pilih.value.alamat} ke bawaan (6 digit terakhir no. HP)? Semua HP yang sedang membuka kartu ini akan keluar.`)
  && jalan(async () => { pinAcak.value = (await A(`/api/admin/rumah/${pilih.value.alamat}/reset-pin`, { method: 'POST' })).pinAcak || ''; await muatRumah(); },
    'PIN direset.');
const nonaktifkan = () => jalan(async () => {
  await A(`/api/admin/rumah/${pilih.value.alamat}/nonaktif`, { method: 'POST', body: { konfirmasi: konfirmasi.value } });
  await muatRumah();
}, 'Rumah dinonaktifkan.');
const aktifkan = () => jalan(async () => { await A(`/api/admin/rumah/${pilih.value.alamat}/aktif`, { method: 'POST' }); await muatRumah(); }, 'Rumah diaktifkan lagi.');
const tambahRumah = () => jalan(async () => {
  const r = await A('/api/admin/rumah', { method: 'POST', body: { cluster: 'N', ...rumahBaru.value } });
  await muatRumah();
  pilihAlamat.value = r.alamat;
  pinAcak.value = r.pinAcak || '';
  rumahBaru.value = { blok: '', rumah: '', nama: '', telp: '' };
}, 'Rumah ditambahkan — isi riwayat luas/tipe supaya tagihannya mulai.');

// ── Blok ──
const warnaBlok = ref({});
const simpanWarna = (b) => jalan(() => A(`/api/admin/blok/${b}`, { method: 'PUT', body: { warna: warnaBlok.value[b] } }), `Warna blok ${b} disimpan.`);

// ── Tarif ──
const tarif = ref([]);
const kartuEdit = ref(null);   // { bulan, baris: [...] }
async function muatTarif() { tarif.value = (await A('/api/admin/tarif')).tarif; }
function kartuBaru() {
  const dasar = tarif.value[0];
  kartuEdit.value = { bulan: '', baris: dasar ? dasar.baris.map((b) => ({ ...b })) : [
    { komponen: 'islk_rumah', luasMin: 0, nominal: 0 }, { komponen: 'iuran_rt', luasMin: 0, nominal: 50000 }] };
}
const ubahKartu = (k) => { kartuEdit.value = { bulan: bulanDariPeriode(k.berlaku), baris: k.baris.map((b) => ({ ...b })), ada: true }; };
const simpanKartu = () => jalan(async () => {
  const baris = kartuEdit.value.baris.map((b) => ({ komponen: b.komponen, luasMin: Number(b.luasMin) || 0, nominal: Math.round(Number(b.nominal) || 0) }));
  await A(`/api/admin/tarif/${periodeDariBulan(kartuEdit.value.bulan)}`, { method: 'PUT', body: { baris } });
  kartuEdit.value = null;
  await muatTarif();
}, 'Rate card disimpan.');
const hapusKartu = (k) => confirm(`Hapus seluruh rate card ${labelPeriode(k.berlaku)}?`)
  && jalan(async () => { await A(`/api/admin/tarif/${k.berlaku}`, { method: 'DELETE' }); await muatTarif(); }, 'Rate card dihapus.');
const KOMPONEN = { islk_rumah: 'ISLK rumah (≥ luas)', islk_kavling: 'ISLK kavling (per m²)', iuran_rt: 'Iuran RT' };

// ── Pengurus ──
const pengurus = ref([]);
const formPengurus = ref({ email: '', nama: '', peran: 'bendahara', aktif: true, undang: true });
async function muatPengurus() { pengurus.value = (await A('/api/admin/pengurus')).pengurus; }
const simpanPengurus = () => jalan(async () => {
  const f = formPengurus.value;
  const r = await A(`/api/admin/pengurus/${encodeURIComponent(f.email.trim().toLowerCase())}`, { method: 'PUT',
    body: { nama: f.nama, peran: f.peran, aktif: f.aktif, undang: f.undang } });
  await muatPengurus();
  ok.value = r.undanganTerkirim ? 'Disimpan, undangan email terkirim.' : 'Disimpan.';
  formPengurus.value = { email: '', nama: '', peran: 'bendahara', aktif: true, undang: true };
});
const ubahPengurus = (p) => { formPengurus.value = { ...p, undang: false }; };

// ── Sinkron ──
const tabSheet = ref([]);
const tabPilih = ref('');
const pratinjau = ref(null);
const centang = ref([]);
async function muatSinkron() { tabSheet.value = (await A('/api/admin/sinkron')).tab; }
const lihatPratinjau = () => jalan(async () => {
  centang.value = [];
  pratinjau.value = (await A('/api/admin/sinkron/pratinjau', { method: 'POST', body: { tab: tabPilih.value } })).pratinjau;
});
const terapkan = () => confirm(`Terapkan ${tabPilih.value}: ${pratinjau.value.tambah.length} tambah, ${pratinjau.value.ubah.length} ubah, ${centang.value.length} hapus?`)
  && jalan(async () => {
    await A('/api/admin/sinkron/terapkan', { method: 'POST', body: { tab: tabPilih.value, hapus: centang.value } });
    pratinjau.value = null;
  }, 'Sinkron diterapkan dan dicatat di Log.');
const muatOpex = () => jalan(() => A('/api/admin/opex/muat-ulang', { method: 'POST' }), 'Opex & Saldo Awal dimuat ulang dari Sheet.');

// ── Keamanan & Log ──
const keamanan = ref(null);
const log = ref(null);

async function bukaTab(t) {
  tab.value = t; pesan.value = ''; ok.value = '';
  await jalan(async () => {
    if (t === 'Rumah') await muatRumah();
    if (t === 'Tarif') await muatTarif();
    if (t === 'Pengurus') await muatPengurus();
    if (t === 'Sinkron') await muatSinkron();
    if (t === 'Keamanan') keamanan.value = (await A('/api/admin/keamanan')).keamanan;
    if (t === 'Log') log.value = await A('/api/admin/log');
  });
}
watch(status, (s) => { if (s === 'masuk' && boleh()) bukaTab(tab.value); }, { immediate: true });
BLOK_LIST.forEach((b) => { warnaBlok.value[b] = BLOK_WARNA_DEFAULT[b]; });
const baris = 'padding:7px 0;border-top:1px solid var(--color-divider);font-size:12.5px';
</script>

<template>
 <PengurusGate>
  <section class="scr col" style="gap:var(--space-3)">
    <div class="spread">
      <div>
        <h4 style="margin:0">Admin</h4>
        <div class="text-muted" style="font-size:11.5px">{{ saya.nama }} · setiap perubahan tercatat di Log</div>
      </div>
      <a href="#/kas" class="btn btn-ghost" style="font-size:12px">← Kas</a>
    </div>

    <p v-if="!boleh()" class="text-muted" style="font-size:12.5px">Halaman ini khusus peran admin.</p>
    <template v-else>
    <div class="row" style="gap:6px;flex-wrap:wrap">
      <button v-for="t in TAB" :key="t" type="button" class="btn" :class="tab === t ? 'btn-primary' : 'btn-secondary'"
              style="min-height:34px;padding:5px 12px;font-size:12.5px" @click="bukaTab(t)">{{ t }}</button>
    </div>
    <p v-if="pesan" style="font-size:12px;color:var(--color-accent-700);margin:0">{{ pesan }}</p>
    <p v-if="ok" style="font-size:12px;color:var(--color-accent-2-800);margin:0">✓ {{ ok }}</p>

    <!-- ── Rumah ── -->
    <template v-if="tab === 'Rumah'">
      <input class="input" v-model="cari" placeholder="Cari alamat atau nama">
      <select class="input" v-model="pilihAlamat" style="width:100%">
        <option value="">— pilih rumah ({{ daftarRumah.length }}) —</option>
        <option v-for="r in daftarRumah" :key="r.alamat" :value="r.alamat">
          {{ r.alamat }} · {{ r.nama }}{{ r.nonaktifAt ? ' (nonaktif)' : '' }}{{ r.riwayat.length ? '' : ' · tanpa tarif' }}
        </option>
      </select>

      <Card v-if="pilih">
        <span class="kick">{{ pilih.alamat }}</span>
        <div class="field"><label>Nama kepala keluarga</label><input class="input" v-model="edit.nama"></div>
        <div class="field"><label>No. HP (62…)</label><input class="input num" v-model="edit.telp" inputmode="numeric"></div>
        <Button :disabled="sibuk" @click="simpanRumah">Simpan</Button>

        <div class="kick" style="margin-top:var(--space-2)">Riwayat luas / tipe</div>
        <p class="text-muted" style="font-size:11px;margin:0">
          Baris pertama = bulan mulai ditagih. Rumah berubah luas/tipe? Tambah baris baru mulai bulan itu —
          bulan sebelumnya tetap dengan tarif lamanya.
        </p>
        <div v-for="x in pilih.riwayat" :key="x.berlaku" class="spread" :style="baris">
          <span>Mulai {{ labelPeriode(x.berlaku) }} · {{ x.luas }} m² · {{ x.tipe }}</span>
          <button class="btn btn-ghost" style="font-size:11px" @click="hapusRiwayat(x.berlaku)">Hapus</button>
        </div>
        <div class="row" style="gap:6px;flex-wrap:wrap;align-items:flex-end">
          <div class="field"><label>Mulai bulan</label><input class="input" type="month" v-model="riwayatBaru.bulan"></div>
          <div class="field" style="max-width:90px"><label>Luas m²</label><input class="input num" v-model="riwayatBaru.luas" inputmode="decimal"></div>
          <div class="field"><label>Tipe</label>
            <select class="input" v-model="riwayatBaru.tipe"><option>rumah</option><option>kavling</option></select></div>
          <Button :disabled="sibuk || !riwayatBaru.bulan || !(Number(riwayatBaru.luas) > 0)" @click="tambahRiwayat">Tambah</Button>
        </div>

        <div class="kick" style="margin-top:var(--space-2)">PIN</div>
        <div class="spread" style="font-size:12.5px">
          <span class="text-muted">{{ pilih.pinDiganti ? 'Sudah diganti warga ' + waktuTeks(pilih.pinDiganti) : 'Masih PIN bawaan' }}</span>
          <button class="btn btn-secondary" style="font-size:12px" :disabled="sibuk" @click="resetPin">Reset PIN</button>
        </div>
        <p v-if="pinAcak" style="font-size:12.5px;margin:0">
          Rumah ini tanpa no. HP — PIN sementaranya <b class="num" style="font-size:16px">{{ pinAcak }}</b>.
          Sampaikan ke warga; hanya tampil sekali.
        </p>

        <div class="kick" style="margin-top:var(--space-2)">Status</div>
        <template v-if="pilih.nonaktifAt">
          <p style="font-size:12.5px;margin:0">Nonaktif sejak {{ pilih.nonaktifAt.slice(0, 10) }} oleh {{ pilih.nonaktifOleh }}.</p>
          <Button variant="secondary" :disabled="sibuk" @click="aktifkan">Aktifkan lagi</Button>
        </template>
        <template v-else>
          <p class="text-muted" style="font-size:11px;margin:0">
            Hanya untuk rumah yang digabung (merger) atau yang sudah dipastikan tidak bisa ditagih lagi. Rumah
            hilang dari semua daftar dan total; catatannya tetap tersimpan. Alasannya dicatat di luar sistem.
            Ketik <b>{{ pilih.alamat }}</b> untuk konfirmasi.
          </p>
          <div class="row" style="gap:6px">
            <input class="input grow" v-model="konfirmasi" :placeholder="pilih.alamat">
            <Button variant="secondary" :disabled="sibuk || konfirmasi !== pilih.alamat" @click="nonaktifkan">Nonaktifkan</Button>
          </div>
        </template>
      </Card>

      <Card>
        <span class="kick">Tambah rumah</span>
        <div class="row" style="gap:6px">
          <div class="field"><label>Blok</label>
            <select class="input" v-model="rumahBaru.blok"><option value="">—</option>
              <option v-for="b in BLOK_LIST" :key="b" :value="b">N{{ b }}</option></select></div>
          <div class="field" style="max-width:90px"><label>No.</label><input class="input num" v-model="rumahBaru.rumah" maxlength="3" inputmode="numeric"></div>
        </div>
        <div class="field"><label>Nama</label><input class="input" v-model="rumahBaru.nama"></div>
        <div class="field"><label>No. HP (62…, opsional)</label><input class="input num" v-model="rumahBaru.telp" inputmode="numeric"></div>
        <Button :disabled="sibuk || !rumahBaru.blok || !rumahBaru.rumah || !rumahBaru.nama" @click="tambahRumah">Tambah rumah</Button>
      </Card>

      <Card>
        <span class="kick">Warna blok</span>
        <div v-for="b in BLOK_LIST" :key="b" class="spread" :style="baris">
          <span>Blok N{{ b }}</span>
          <div class="row" style="gap:6px">
            <input type="color" v-model="warnaBlok[b]" style="width:44px;height:30px;border:0;background:none">
            <button class="btn btn-ghost" style="font-size:11px" @click="simpanWarna(b)">Simpan</button>
          </div>
        </div>
      </Card>
    </template>

    <!-- ── Tarif ── -->
    <template v-if="tab === 'Tarif'">
      <p class="text-muted" style="font-size:11.5px;margin:0">
        Satu rate card = semua aturan yang berlaku mulai satu bulan. Harga berubah? Buat rate card baru yang
        lengkap (termasuk yang tidak berubah) mulai bulan itu. ISLK rumah = harga daftar <b>dikurangi</b> Iuran RT.
      </p>
      <Button v-if="!kartuEdit" variant="secondary" @click="kartuBaru">+ Rate card baru</Button>
      <Card v-if="kartuEdit">
        <div class="field"><label>Berlaku mulai</label>
          <input class="input" type="month" v-model="kartuEdit.bulan" :disabled="kartuEdit.ada"></div>
        <div v-for="(b, i) in kartuEdit.baris" :key="i" class="row" style="gap:6px;align-items:flex-end;flex-wrap:wrap">
          <div class="field"><label v-if="!i">Komponen</label>
            <select class="input" v-model="b.komponen">
              <option v-for="(l, k) in KOMPONEN" :key="k" :value="k">{{ l }}</option></select></div>
          <div v-if="b.komponen === 'islk_rumah'" class="field" style="max-width:80px"><label v-if="!i">≥ m²</label>
            <input class="input num" v-model="b.luasMin" inputmode="decimal"></div>
          <div class="field grow"><label v-if="!i">Nominal</label><input class="input num" v-model="b.nominal" inputmode="numeric"></div>
          <button class="btn btn-ghost" style="font-size:12px" @click="kartuEdit.baris.splice(i, 1)">×</button>
        </div>
        <button class="btn btn-ghost" style="font-size:12px;justify-content:flex-start"
                @click="kartuEdit.baris.push({ komponen: 'islk_rumah', luasMin: 0, nominal: 0 })">+ baris</button>
        <div class="row" style="gap:6px">
          <Button :disabled="sibuk || !kartuEdit.bulan" @click="simpanKartu">Simpan rate card</Button>
          <Button variant="secondary" @click="kartuEdit = null">Batal</Button>
        </div>
      </Card>
      <Card v-for="k in tarif" :key="k.berlaku">
        <div class="spread">
          <span class="kick">Mulai {{ labelPeriode(k.berlaku) }}</span>
          <div class="row" style="gap:2px">
            <button class="btn btn-ghost" style="font-size:11px" @click="ubahKartu(k)">Ubah</button>
            <button class="btn btn-ghost" style="font-size:11px" @click="hapusKartu(k)">Hapus</button>
          </div>
        </div>
        <div v-for="(b, i) in k.baris" :key="i" class="spread" style="font-size:12.5px">
          <span class="text-muted">{{ KOMPONEN[b.komponen] }}{{ b.komponen === 'islk_rumah' ? ` ${b.luasMin} m²` : '' }}</span>
          <span class="num">{{ rupiah(b.nominal) }}</span>
        </div>
      </Card>
    </template>

    <!-- ── Pengurus ── -->
    <template v-if="tab === 'Pengurus'">
      <Card>
        <span class="kick">{{ pengurus.some(p => p.email === formPengurus.email) ? 'Ubah pengurus' : 'Tambah pengurus' }}</span>
        <div class="field"><label>Email (akun yang dipakai masuk)</label>
          <input class="input" type="email" v-model="formPengurus.email"></div>
        <div class="field"><label>Nama (tercatat di setiap verifikasi)</label><input class="input" v-model="formPengurus.nama"></div>
        <div class="row" style="gap:var(--space-3);flex-wrap:wrap;align-items:center">
          <select class="input" v-model="formPengurus.peran">
            <option value="bendahara">Bendahara</option><option value="komite">Komite (lihat saja)</option><option value="admin">Admin</option>
          </select>
          <label class="radio"><input type="checkbox" v-model="formPengurus.aktif"> Aktif</label>
          <label class="radio"><input type="checkbox" v-model="formPengurus.undang"> Kirim undangan email</label>
        </div>
        <Button :disabled="sibuk || !formPengurus.email || !formPengurus.nama" @click="simpanPengurus">Simpan</Button>
      </Card>
      <Card>
        <div v-for="p in pengurus" :key="p.email" class="spread" :style="baris" style="cursor:pointer" @click="ubahPengurus(p)">
          <div :style="p.aktif ? '' : 'opacity:.5'">
            <div style="font-weight:600">{{ p.nama }}</div>
            <div class="text-muted" style="font-size:11px">{{ p.email }} · {{ p.pernahMasuk ? 'sudah pernah masuk' : 'belum pernah masuk' }}</div>
          </div>
          <span class="tag tag-neutral">{{ p.aktif ? p.peran : 'nonaktif' }}</span>
        </div>
      </Card>
    </template>

    <!-- ── Sinkron ── -->
    <template v-if="tab === 'Sinkron'">
      <p class="text-muted" style="font-size:11.5px;margin:0">
        Bawa data dari Google Sheet ke database — selalu Sheet → database. Pilih tab, lihat pratinjau, lalu
        terapkan. Baris yang hilang dari Sheet <b>tidak</b> dihapus kecuali Anda centang. Impor pertama: ikuti
        urutan M-Blok → M-Rumah → M-RumahRiwayat → M-TarifVersi → L-… → M-Impor.
      </p>
      <div class="row" style="gap:6px">
        <select class="input grow" v-model="tabPilih">
          <option value="">— pilih tab —</option>
          <option v-for="t in tabSheet" :key="t" :value="t">{{ t }}</option>
        </select>
        <Button :disabled="sibuk || !tabPilih" @click="lihatPratinjau">Pratinjau</Button>
      </div>
      <Card v-if="pratinjau">
        <span class="kick">{{ pratinjau.tab }}</span>
        <div class="text-muted" style="font-size:12.5px">
          {{ pratinjau.tambah.length }} tambah · {{ pratinjau.ubah.length }} ubah · {{ pratinjau.sama }} sama ·
          {{ pratinjau.hilang.length }} hilang · {{ pratinjau.galat.length }} galat
        </div>
        <template v-if="pratinjau.galat.length">
          <div class="kick">Galat — baris ini dilewati</div>
          <div v-for="g in pratinjau.galat" :key="g.baris + g.pesan" :style="baris" style="color:var(--color-accent-700)">
            Baris {{ g.baris }}: {{ g.pesan }}</div>
        </template>
        <template v-if="pratinjau.ubah.length">
          <div class="kick">Berubah</div>
          <div v-for="u in pratinjau.ubah" :key="u.kunci" :style="baris">
            <div class="text-muted" style="text-decoration:line-through">{{ u.lama }}</div><div>{{ u.baru }}</div></div>
        </template>
        <template v-if="pratinjau.hilang.length">
          <div class="kick">Ada di database, hilang dari Sheet — centang untuk menghapus</div>
          <label v-for="h in pratinjau.hilang" :key="h.kunci" class="radio" :style="baris">
            <input type="checkbox" :value="h.kunci" v-model="centang"> {{ h.ringkas }}</label>
        </template>
        <template v-if="pratinjau.tambah.length">
          <div class="kick">Baru ({{ pratinjau.tambah.length }})</div>
          <div v-for="t in pratinjau.tambah.slice(0, 50)" :key="t.kunci" :style="baris">{{ t.ringkas }}</div>
          <div v-if="pratinjau.tambah.length > 50" class="text-muted" style="font-size:11px">… dan {{ pratinjau.tambah.length - 50 }} lagi</div>
        </template>
        <Button :disabled="sibuk || !(pratinjau.tambah.length + pratinjau.ubah.length + centang.length)" @click="terapkan">
          Terapkan
        </Button>
      </Card>
      <Card>
        <span class="kick">Opex &amp; Saldo Awal</span>
        <p class="text-muted" style="font-size:11.5px;margin:0">
          Tetap diedit di Sheet (M-Opex, M-SaldoAwal); server memuatnya otomatis tiap hari. Baru mengubahnya?
        </p>
        <Button variant="secondary" :disabled="sibuk" @click="muatOpex">Muat ulang sekarang</Button>
      </Card>
    </template>

    <!-- ── Keamanan ── -->
    <template v-if="tab === 'Keamanan' && keamanan">
      <Card>
        <span class="kick">PIN warga salah</span>
        <div class="spread"><span>24 jam terakhir</span><b class="num">{{ keamanan.gagal24j }}</b></div>
        <div class="spread"><span>7 hari terakhir</span><b class="num">{{ keamanan.gagal7h }}</b></div>
        <p class="text-muted" style="font-size:11px;margin:0">
          Satu rumah terkunci 15 menit setelah 5 salah, 1 jam setelah 10, 24 jam setelah 20 dalam sehari.
          Satu IP terkunci 1 jam setelah 30 salah di rumah mana pun.
        </p>
      </Card>
      <Card v-if="keamanan.perIP && keamanan.perIP.length">
        <span class="kick">Per IP (7 hari)</span>
        <div v-for="x in keamanan.perIP" :key="x.ip" class="spread" :style="baris">
          <span :style="x.mencurigakan ? 'color:var(--color-accent-700);font-weight:700' : ''">
            {{ x.mencurigakan ? '⚠ ' : '' }}{{ x.ip }}</span>
          <span class="text-muted">{{ x.gagal }}× di {{ x.rumah }} rumah · {{ waktuTeks(x.terakhir) }}</span>
        </div>
      </Card>
      <Card v-if="keamanan.perRumah && keamanan.perRumah.length">
        <span class="kick">Per rumah (7 hari)</span>
        <div v-for="x in keamanan.perRumah" :key="x.alamat" class="spread" :style="baris">
          <span>{{ x.alamat }}{{ x.ada ? '' : ' (bukan alamat terdaftar)' }}</span>
          <span class="text-muted">{{ x.gagal }}× ({{ x.gagal24j }} hari ini) · {{ x.ip }} IP</span>
        </div>
      </Card>
      <Card>
        <span class="kick">100 percobaan terakhir</span>
        <div v-for="(x, i) in keamanan.terbaru || []" :key="i" :style="baris">
          <span :style="x.berhasil ? '' : 'color:var(--color-accent-700)'">{{ x.berhasil ? '✓' : '✗' }} {{ x.alamat }}</span>
          <span class="text-muted" style="font-size:11px"> · {{ waktuTeks(x.waktu) }} · {{ x.ip }}</span>
          <div class="text-muted truncate" style="font-size:10.5px">{{ x.perangkat }}</div>
        </div>
      </Card>
    </template>

    <!-- ── Log ── -->
    <template v-if="tab === 'Log' && log">
      <Card>
        <span class="kick">Sinkron</span>
        <p v-if="!(log.sinkron || []).length" class="text-muted" style="font-size:12px;margin:0">Belum ada.</p>
        <div v-for="x in log.sinkron || []" :key="x.id" class="spread" :style="baris">
          <span>{{ x.tab }} · {{ x.oleh }}</span>
          <span class="text-muted" style="font-size:11px">{{ waktuTeks(x.waktu) }} · +{{ x.ringkasan.tambah }} ~{{ x.ringkasan.ubah }} −{{ x.ringkasan.hapus }}</span>
        </div>
      </Card>
      <Card>
        <span class="kick">Perubahan data master</span>
        <div v-for="x in log.audit || []" :key="x.id" :style="baris">
          <div class="spread"><span>{{ x.tabel }} · {{ x.kunci }}</span><span class="text-muted" style="font-size:11px">{{ waktuTeks(x.waktu) }}</span></div>
          <div class="text-muted" style="font-size:11px">oleh {{ x.oleh }}</div>
        </div>
      </Card>
    </template>
    </template>
  </section>
 </PengurusGate>
</template>
