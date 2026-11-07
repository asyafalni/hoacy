<script setup vapor>
import { ref, computed, watch, nextTick } from 'vue';
import { useKas, useSaya } from '../composables/useKas';
import { useScrollLock } from '../composables/useScrollLock';
import { api, ApiError, urlBerkas } from '../lib/api';
import { keluarPengurus } from '../lib/pengurus';
import { rupiah, rupiahPendek, BULAN, REKENING } from '../lib/tariff';
import { labelBulan, waktuTeks } from '../lib/tagihan';
import Card from '../components/ui/Card.vue';
import Button from '../components/ui/Button.vue';
import PengurusGate from '../components/PengurusGate.vue';

const { rumah, saldo, totals, pending, cek, tunai, TAHUN, muat, pastikan, loading } = useKas();
const { saya, status, boleh } = useSaya();
// Kas data only exists once the server has accepted the sign-in — load it then.
watch(status, (s) => { if (s === 'masuk') pastikan(); }, { immediate: true });

const total = computed(() => totals.value.jumlahRumah);
const persen = computed(() => (total.value ? Math.round((totals.value.lunasBulanIni / total.value) * 100) : 0));
const label = (p) => labelBulan(p, TAHUN.value, BULAN);
const bulanList = (g) => g.items.map((i) => label(i.periode)).join(', ');
const kurang = (g) => g.items.filter((i) => i.tarif != null && i.nominal < i.tarif);
const pesan = ref('');
const sibuk = ref(new Set());   // kirimanId while its verdict is being sent

async function keluar() {
  await keluarPengurus();
  status.value = 'perlu-masuk';
}

// One verdict per submission: `sah` verifies a transfer, `tolak` rejects it
// or voids a mistaken cash entry — its months go back to "Belum". The server
// answers once it's recorded, so the list simply reloads.
async function putuskan(g, keputusan) {
  if (sibuk.value.has(g.kirimanId)) return;
  if (keputusan === 'tolak' && !confirm(g.metode === 'tunai'
    ? `Batalkan catatan tunai ${g.alamat} · ${bulanList(g)} (${rupiah(g.total)})?\nBulan-bulan ini kembali "Belum". Uangnya tidak dihitung di Kas Tunai.`
    : `Tolak transfer ${g.alamat} · ${bulanList(g)} (${rupiah(g.total)})?\nBulan-bulan ini kembali "Belum" dan warga perlu mengirim ulang.`)) return;
  sibuk.value = new Set([...sibuk.value, g.kirimanId]);
  pesan.value = '';
  try {
    await api('/api/kas/keputusan', { method: 'POST', pakai: 'pengurus', body: { kirimanId: g.kirimanId, keputusan } });
    await muat();
  } catch (e) {
    pesan.value = e instanceof ApiError ? e.message : 'Gagal menyimpan keputusan.';
  } finally {
    const s = new Set(sibuk.value); s.delete(g.kirimanId); sibuk.value = s;
  }
}

// ── Lihat bukti: the photo comes through the server, only for pengurus ──
const buktiTampil = ref(null);
const buktiSrc = ref('');
const buktiGagal = ref(false);
useScrollLock(buktiTampil);
async function lihatBukti(g) {
  buktiTampil.value = g;
  buktiGagal.value = false;
  if (buktiSrc.value) URL.revokeObjectURL(buktiSrc.value);
  buktiSrc.value = '';
  try { buktiSrc.value = await urlBerkas(`/api/kas/bukti/${g.kirimanId}`); } catch { buktiGagal.value = true; }
}

// ── Setor ke bank ──
const showSetor = ref(false);
useScrollLock(showSetor);
const setorNominal = ref(0);
function bukaSetor() { setorNominal.value = saldo.value.kas; pesan.value = ''; showSetor.value = true; }
async function setor() {
  const n = Math.round(Number(setorNominal.value));
  if (!n) return;
  try {
    await api('/api/kas/setoran', { method: 'POST', pakai: 'pengurus', body: { nominal: n } });
    showSetor.value = false;
    await muat();
  } catch (e) { pesan.value = e.message; }
}

// ── Pengeluaran ──
const showKeluar = ref(false);
useScrollLock(showKeluar);
const daftarKeluar = ref(null);
const keluarForm = ref({ keterangan: '', nominal: '', sumber: 'bank' });
const nota = ref(null);
const notaInput = ref(null);
const mengirimKeluar = ref(false);
async function bukaPengeluaran() {
  showKeluar.value = true;
  pesan.value = '';
  try { daftarKeluar.value = (await api('/api/kas/pengeluaran', { pakai: 'pengurus' })).pengeluaran; } catch { daftarKeluar.value = []; }
}
async function catatPengeluaran() {
  const f = keluarForm.value;
  const fd = new FormData();
  fd.append('keterangan', f.keterangan.trim());
  fd.append('nominal', String(Math.round(Number(f.nominal) || 0)));
  fd.append('sumber', f.sumber);
  if (nota.value) fd.append('nota', nota.value);
  mengirimKeluar.value = true;
  pesan.value = '';
  try {
    await api('/api/kas/pengeluaran', { method: 'POST', pakai: 'pengurus', form: fd });
    keluarForm.value = { keterangan: '', nominal: '', sumber: f.sumber };
    nota.value = null;
    await Promise.all([bukaPengeluaran(), muat()]);
  } catch (e) { pesan.value = e.message; } finally { mengirimKeluar.value = false; }
}
async function lihatNota(p) {
  try { window.open(await urlBerkas(`/api/kas/nota/${p.id}`), '_blank'); } catch { pesan.value = 'Nota tidak bisa dibuka.'; }
}

// ── Riwayat kas masuk tunai — lazy-rendered 10 rows at a time ──
const showRiwayatKas = ref(false);
useScrollLock(showRiwayatKas);
const riwayatVisibleN = ref(10);
const riwayatTampil = computed(() => tunai.value.slice(0, riwayatVisibleN.value));
const riwayatScrollEl = ref(null);   // the observer's root must be the sheet, not the viewport
const riwayatSentinel = ref(null);
let riwayatObserver = null;
watch(showRiwayatKas, async (open) => {
  if (!open) { riwayatObserver?.disconnect(); return; }
  riwayatVisibleN.value = 10;
  await nextTick();
  riwayatObserver = new IntersectionObserver((entries) => {
    if (entries[0].isIntersecting && riwayatVisibleN.value < tunai.value.length) riwayatVisibleN.value += 10;
  }, { root: riwayatScrollEl.value, rootMargin: '200px' });
  if (riwayatSentinel.value) riwayatObserver.observe(riwayatSentinel.value);
});
const batal = (g) => g.keabsahan === 'tolak' || g.keabsahan === 'dobel';

// ── Iuran per tahun ──
const showTahunan = ref(false);
useScrollLock(showTahunan);
const tahunList = computed(() => {
  const mulai = rumah.value.map((h) => h.mulai).filter(Boolean);
  const dari = mulai.length ? Math.floor(Math.min(...mulai) / 100) : TAHUN.value;
  return Array.from({ length: TAHUN.value - dari + 1 }, (_, i) => TAHUN.value - i);
});
const tahunPilih = ref(0);
const ringkasan = ref(null);
async function pilihTahun(y) {
  tahunPilih.value = y;
  ringkasan.value = null;
  try { ringkasan.value = (await api(`/api/kas/tahun/${y}`, { pakai: 'pengurus' })).ringkasan; } catch { ringkasan.value = { error: true }; }
}
function bukaTahunan() { showTahunan.value = true; pilihTahun(tahunPilih.value || TAHUN.value); }
const METODE_LABEL = { tunai: 'Tunai', transfer: 'Transfer', impor: 'Data lama (impor)' };
const tombol = 'justify-content:center;background:var(--color-surface);box-shadow:var(--shadow-sm)';
</script>

<template>
 <PengurusGate>
  <section class="scr col" style="gap:var(--space-3)">
    <div class="spread">
      <div>
        <h4 style="margin:0">Kas RT 03/14</h4>
        <div class="text-muted" style="font-size:11.5px">
          {{ loading && !total ? 'Memuat…' : `${total} rumah` }} · {{ saya.nama }} ({{ saya.peran }})
        </div>
      </div>
      <div class="row" style="gap:4px;flex:none">
        <button v-if="tunai.length" class="btn btn-ghost" style="font-size:12px;padding:6px 8px"
                title="Riwayat kas masuk tunai" @click="showRiwayatKas = true">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor"
               stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="9"></circle>
            <path d="M12 7v5l3 3"></path>
          </svg>
        </button>
        <button class="btn btn-ghost" style="font-size:12px" @click="keluar">Keluar</button>
      </div>
    </div>

    <p v-if="pesan" style="font-size:12px;color:var(--color-accent-700);margin:0">{{ pesan }}</p>

    <!-- tunai is the exception since Jan 2027 (bendahara records it on /kas/tunai);
         the card only shows while cash is held, i.e. until it's deposited -->
    <Card v-if="saldo.kas !== 0" elev="md" style="background:var(--color-neutral-900);color:var(--color-neutral-100)">
      <span class="kick" style="color:var(--color-accent-300)">Kas Tunai</span>
      <div class="num" style="font-family:var(--font-heading);font-size:32px;line-height:1.1">
        {{ rupiah(saldo.kas) }}
      </div>
      <Button v-if="boleh('bendahara')" block style="background:var(--color-accent-400);color:var(--color-neutral-900)"
              @click="bukaSetor">
        Setor ke Bank
      </Button>
      <div style="text-align:center;font-size:10.5px;color:var(--color-neutral-400)">
        atau biarkan sebagai petty cash operasional
      </div>
    </Card>

    <Card>
      <span class="kick">Rekening {{ REKENING.bank }} {{ REKENING.nomor.slice(0, 4) }}-xxxx</span>
      <div class="num" style="font-family:var(--font-heading);font-size:28px;line-height:1.1">
        {{ rupiah(saldo.rekening) }}
      </div>
      <p class="text-muted" style="font-size:11.5px;margin:0">
        Transfer terverifikasi + setoran tunai − pengeluaran
      </p>
    </Card>

    <Card>
      <div class="spread">
        <span style="font-size:13px;font-weight:700">Terkumpul bulan ini</span>
        <span class="num" style="font-size:13px;font-weight:700">{{ persen }}% lunas</span>
      </div>
      <div style="height:10px;border-radius:999px;background:var(--color-neutral-200);overflow:hidden">
        <div :style="{ width: persen + '%', height: '100%', background: 'var(--color-accent-2-500)' }"></div>
      </div>
      <div class="spread" style="font-size:12.5px">
        <span class="text-muted">Tunggakan seluruh cluster</span>
        <span class="num" style="font-weight:700;color:var(--color-accent-700)">
          {{ rupiahPendek(totals.tunggakan) }}
        </span>
      </div>
    </Card>

    <!-- a house without a riwayat row is billed nothing until the admin adds
         one — never silently "Rp0" -->
    <Card v-if="totals.tarifBelumDiatur.length" style="background:var(--color-accent-100);gap:4px">
      <span style="font-size:12.5px;font-weight:700;color:var(--color-accent-800)">
        ⚠ {{ totals.tarifBelumDiatur.length }} rumah belum punya tarif
      </span>
      <span style="font-size:11.5px">
        {{ totals.tarifBelumDiatur.map(h => h.alamat).join(', ') }} — isi riwayat luas/tipe di
        Admin → Rumah. Sampai itu, rumah ini tidak bisa bayar dan tidak masuk target.
      </span>
    </Card>

    <!-- transfers waiting — one card per submission (one bukti, any months) -->
    <Card v-if="pending.length" style="gap:2px">
      <span class="kick">Perlu diverifikasi ({{ pending.length }})</span>
      <div v-for="g in pending" :key="g.kirimanId" class="col" style="gap:6px;padding:8px 0;
           border-top:1px solid var(--color-divider)">
        <div class="spread">
          <span style="font-weight:700;font-size:13px">{{ g.alamat }} · {{ bulanList(g) }}</span>
          <span class="num" style="font-weight:700">{{ rupiah(g.total) }}</span>
        </div>
        <div class="text-muted" style="font-size:11px">{{ waktuTeks(g.waktu) }}</div>
        <div v-if="kurang(g).length" style="font-size:11px;color:var(--color-accent-700)">
          ⚠ Nominal di bawah tarif untuk {{ kurang(g).map(i => label(i.periode)).join(', ') }} — cocokkan dengan bukti.
        </div>
        <div class="row" style="gap:6px">
          <button v-if="g.adaBukti" type="button" class="btn btn-secondary" @click="lihatBukti(g)"
             :style="tombol + ';flex:1;font-size:12px;padding:6px 10px'">
            Lihat bukti
          </button>
          <span v-else class="text-muted" style="flex:1;font-size:11px;align-self:center">(tanpa bukti)</span>
          <template v-if="boleh('bendahara')">
            <button class="btn btn-ghost" style="flex:none;font-size:12px;padding:6px 10px"
                    :disabled="sibuk.has(g.kirimanId)" @click="putuskan(g, 'tolak')">Tolak</button>
            <button class="btn btn-primary" style="flex:1;font-size:12px;padding:6px 10px"
                    :disabled="sibuk.has(g.kirimanId)" @click="putuskan(g, 'sah')">
              {{ sibuk.has(g.kirimanId) ? 'Menyimpan…' : 'Verifikasi' }}
            </button>
          </template>
        </div>
      </div>
    </Card>

    <!-- months that don't add up to the total — only from old Form data now -->
    <Card v-if="cek.length" style="gap:2px">
      <span class="kick">Rincian tidak cocok ({{ cek.length }})</span>
      <p class="text-muted" style="font-size:11px;margin:0">
        Tidak dihitung. Minta warga kirim ulang konfirmasi transfer dari aplikasi.
      </p>
      <div v-for="g in cek" :key="g.kirimanId" class="spread" style="padding:8px 0;
           border-top:1px solid var(--color-divider);font-size:12.5px">
        <span><b>{{ g.alamat }}</b> · {{ bulanList(g) }}</span>
        <button v-if="g.adaBukti" type="button" class="btn btn-ghost" style="font-size:11.5px;padding:4px 6px"
                @click="lihatBukti(g)">Lihat bukti</button>
      </div>
    </Card>

    <a v-if="boleh('bendahara')" class="btn btn-secondary" href="#/kas/tunai" :style="tombol">
      Catat pembayaran tunai
    </a>
    <button type="button" class="btn btn-secondary" :style="tombol" @click="bukaPengeluaran">
      Pengeluaran
    </button>
    <button type="button" class="btn btn-secondary" :style="tombol" @click="bukaTahunan">
      Iuran per tahun
    </button>
    <a class="btn btn-secondary" href="#/kas/rumah" :style="tombol">Lihat semua kartu rumah</a>
    <a class="btn btn-secondary" href="#/kas/qr" :style="tombol">Cetak QR per rumah</a>
    <a v-if="boleh()" class="btn btn-secondary" href="#/kas/admin" :style="tombol">
      Admin — rumah, tarif, pengurus, sinkron, keamanan
    </a>

    <!-- setor ke bank -->
    <div v-if="showSetor" class="dialog-backdrop sheet-backdrop" @click.self="showSetor = false">
      <div class="dialog sheet" style="border-radius:var(--radius-lg) var(--radius-lg) 0 0;max-height:85dvh">
       <div class="sheet-scroll">
        <div class="spread">
          <div class="dialog-title">Setor ke Bank</div>
          <button class="btn btn-ghost" @click="showSetor = false">×</button>
        </div>
        <p class="text-muted" style="font-size:12px;margin:0">
          Uang tunai yang disetor pindah dari Kas Tunai ke Rekening. Isi negatif untuk tarik tunai dari bank.
        </p>
        <div class="field">
          <label>Nominal</label>
          <input class="input num" v-model="setorNominal" type="number" inputmode="numeric">
        </div>
        <p v-if="pesan" style="font-size:12px;color:var(--color-accent-700);margin:0">{{ pesan }}</p>
        <Button block @click="setor">Simpan setoran {{ rupiah(Number(setorNominal) || 0) }}</Button>
       </div>
      </div>
    </div>

    <!-- pengeluaran -->
    <div v-if="showKeluar" class="dialog-backdrop sheet-backdrop" @click.self="showKeluar = false">
      <div class="dialog sheet" style="border-radius:var(--radius-lg) var(--radius-lg) 0 0;max-height:90dvh">
       <div class="sheet-scroll">
        <div class="spread">
          <div class="dialog-title">Pengeluaran</div>
          <button class="btn btn-ghost" @click="showKeluar = false">×</button>
        </div>
        <template v-if="boleh('bendahara')">
          <div class="field">
            <label>Keterangan</label>
            <input class="input" v-model="keluarForm.keterangan" maxlength="200" placeholder="mis. Ganti lampu jalan Blok 7">
          </div>
          <div class="row" style="gap:var(--space-2)">
            <div class="field grow">
              <label>Nominal</label>
              <input class="input num" v-model="keluarForm.nominal" type="number" inputmode="numeric">
            </div>
            <div class="field">
              <label>Dari</label>
              <select class="input" v-model="keluarForm.sumber">
                <option value="bank">Rekening</option>
                <option value="kas">Kas tunai</option>
              </select>
            </div>
          </div>
          <input ref="notaInput" type="file" accept="image/*,application/pdf" style="display:none"
                 @change="nota = $event.target.files?.[0] || null">
          <button type="button" class="btn btn-secondary" :style="tombol" @click="notaInput?.click()">
            {{ nota ? '✓ ' + nota.name : '🧾 Lampirkan nota (opsional)' }}
          </button>
          <p v-if="pesan" style="font-size:12px;color:var(--color-accent-700);margin:0">{{ pesan }}</p>
          <Button block :disabled="mengirimKeluar || !keluarForm.keterangan.trim() || !(Number(keluarForm.nominal) > 0)"
                  @click="catatPengeluaran">
            {{ mengirimKeluar ? 'Menyimpan…' : 'Catat pengeluaran' }}
          </Button>
        </template>
        <div class="kick" style="margin-top:var(--space-2)">Terakhir</div>
        <p v-if="!daftarKeluar" class="text-muted" style="font-size:12px;margin:0">Memuat…</p>
        <p v-else-if="!daftarKeluar.length" class="text-muted" style="font-size:12px;margin:0">Belum ada pengeluaran.</p>
        <div v-for="p in daftarKeluar || []" :key="p.id" class="spread"
             style="padding:7px 0;border-top:1px solid var(--color-divider);font-size:12.5px;gap:var(--space-2)">
          <div>
            <div style="font-weight:600">{{ p.keterangan }}</div>
            <div class="text-muted" style="font-size:11px">
              {{ waktuTeks(p.waktu) }} · {{ p.sumber === 'kas' ? 'kas tunai' : 'rekening' }} · {{ p.oleh }}
              <a v-if="p.adaNota" href="#" @click.prevent="lihatNota(p)">· nota</a>
            </div>
          </div>
          <span class="num" style="font-weight:700;flex:none">{{ rupiahPendek(p.nominal) }}</span>
        </div>
       </div>
      </div>
    </div>

    <!-- riwayat kas masuk tunai -->
    <div v-if="showRiwayatKas" class="dialog-backdrop sheet-backdrop" @click.self="showRiwayatKas = false">
      <div class="dialog sheet" style="border-radius:var(--radius-lg) var(--radius-lg) 0 0;max-height:85dvh">
       <div ref="riwayatScrollEl" class="sheet-scroll">
        <div class="spread">
          <div>
            <div class="dialog-title">Riwayat Kas Masuk</div>
            <div class="text-muted" style="font-size:12px">
              Tunai · salah input? "Batalkan", lalu catat ulang di Catat pembayaran tunai.
            </div>
          </div>
          <button class="btn btn-ghost" @click="showRiwayatKas = false">×</button>
        </div>
        <div class="col" style="gap:2px">
          <div v-for="g in riwayatTampil" :key="g.kirimanId" class="spread"
               style="padding:8px 0;border-top:1px solid var(--color-divider);gap:var(--space-2)">
            <div :style="batal(g) ? 'opacity:.55' : ''">
              <div style="font-size:12.5px;font-weight:600">{{ g.alamat }} · {{ bulanList(g) }}</div>
              <div class="text-muted" style="font-size:11px">
                {{ g.petugas }} · {{ waktuTeks(g.waktu) }}{{ g.keabsahan === 'tolak' ? ' · dibatalkan' : g.keabsahan === 'dobel' ? ' · dobel, tidak dihitung' : '' }}
              </div>
            </div>
            <div class="col" style="align-items:flex-end;gap:2px;flex:none">
              <span class="num" style="font-weight:700;font-size:12.5px"
                    :style="batal(g) ? 'text-decoration:line-through;opacity:.5' : ''">
                {{ rupiahPendek(g.total) }}
              </span>
              <button v-if="!batal(g) && boleh('bendahara')" class="btn btn-ghost"
                      style="font-size:11px;padding:2px 4px;min-height:0"
                      :disabled="sibuk.has(g.kirimanId)" @click="putuskan(g, 'tolak')">
                Batalkan
              </button>
            </div>
          </div>
          <div ref="riwayatSentinel" style="height:1px"></div>
          <p class="text-muted" style="text-align:center;font-size:11px">
            {{ riwayatVisibleN < tunai.length ? 'Memuat lagi…' : `— ${tunai.length} dari ${tunai.length} —` }}
          </p>
        </div>
       </div>
      </div>
    </div>

    <!-- iuran per tahun -->
    <div v-if="showTahunan" class="dialog-backdrop sheet-backdrop" @click.self="showTahunan = false">
      <div class="dialog sheet" style="border-radius:var(--radius-lg) var(--radius-lg) 0 0;max-height:85dvh">
       <div class="sheet-scroll">
        <div class="spread">
          <div>
            <div class="dialog-title">Iuran per Tahun</div>
            <div class="text-muted" style="font-size:12px">Menurut bulan tagihan · hanya yang sah</div>
          </div>
          <button class="btn btn-ghost" @click="showTahunan = false">×</button>
        </div>
        <label class="row" style="gap:8px;align-items:center">
          <span class="kick">Tahun</span>
          <select :value="tahunPilih" class="input" style="min-height:32px;font-size:13px"
                  @change="pilihTahun(Number($event.target.value))">
            <option v-for="y in tahunList" :key="y" :value="y">{{ y }}</option>
          </select>
        </label>
        <p v-if="!ringkasan" class="text-muted" style="font-size:12px;margin:0">Memuat {{ tahunPilih }}…</p>
        <p v-else-if="ringkasan.error" class="text-muted" style="font-size:12px;margin:0">Gagal memuat.</p>
        <template v-else>
          <div class="spread" style="background:var(--color-bg);border-radius:var(--radius-md);
               padding:var(--space-3) var(--space-4)">
            <span style="font-size:13.5px;font-weight:700">Terkumpul {{ tahunPilih }}</span>
            <span class="num" style="font-family:var(--font-heading);font-size:21px;color:var(--color-accent-700)">
              {{ rupiah(ringkasan.total) }}
            </span>
          </div>
          <div class="col" style="gap:2px">
            <div v-for="(n, m) in ringkasan.perMetode" :key="m" class="spread" style="font-size:12px">
              <span class="text-muted">{{ METODE_LABEL[m] || m }}</span>
              <span class="num">{{ rupiah(n) }}</span>
            </div>
          </div>
          <div class="col" style="gap:0">
            <div v-for="b in ringkasan.bulan" :key="b.bulan" class="spread"
                 style="padding:7px 0;border-top:1px solid var(--color-divider);font-size:12.5px">
              <span>{{ BULAN[b.bulan - 1] }}</span>
              <span class="text-muted" style="font-size:11.5px">{{ b.rumah }} rumah</span>
              <span class="num" style="font-weight:700;min-width:84px;text-align:right">{{ rupiahPendek(b.total) }}</span>
            </div>
          </div>
        </template>
       </div>
      </div>
    </div>

    <!-- bukti — fetched through the server, shown inline -->
    <div v-if="buktiTampil" class="dialog-backdrop sheet-backdrop" @click.self="buktiTampil = null">
      <div class="dialog sheet" style="border-radius:var(--radius-lg) var(--radius-lg) 0 0;max-height:85dvh">
       <div class="sheet-scroll">
        <div class="spread">
          <div>
            <div class="dialog-title">Bukti Transfer</div>
            <div class="text-muted" style="font-size:12px">
              {{ buktiTampil.alamat }} · {{ bulanList(buktiTampil) }} · {{ rupiah(buktiTampil.total) }}
            </div>
          </div>
          <button class="btn btn-ghost" @click="buktiTampil = null">×</button>
        </div>
        <p v-if="buktiGagal" class="text-muted" style="text-align:center;font-size:12.5px">
          Bukti tidak bisa dimuat. Untuk bukti lama dari Google Form, pastikan folder unggahan Form
          dibagikan ke akun Google RT.
        </p>
        <p v-else-if="!buktiSrc" class="text-muted" style="text-align:center;font-size:12.5px">Memuat…</p>
        <img v-else :src="buktiSrc" alt="Bukti transfer" style="width:100%;border-radius:var(--radius-md);display:block"
             @error="buktiGagal = true">
        <a v-if="buktiSrc" :href="buktiSrc" target="_blank" class="text-muted" style="font-size:11.5px;text-align:center">
          Buka di tab baru (untuk PDF)
        </a>
       </div>
      </div>
    </div>
  </section>
 </PengurusGate>
</template>
