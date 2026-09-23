<script setup vapor>
import { ref, computed, onMounted, watch, nextTick } from 'vue';
import { useSheet } from '../composables/useSheet';
import { usePembayaranLedger } from '../composables/usePembayaranLedger';
import { useScrollLock } from '../composables/useScrollLock';
import { rupiah, rupiahPendek, BULAN, REKENING } from '../lib/tariff';
import { urlSetoran, submitKeputusan, driveImageUrl } from '../lib/forms';
import { labelBulan } from '../lib/tagihan';
import Card from '../components/ui/Card.vue';
import Button from '../components/ui/Button.vue';
import PinGate from '../components/PinGate.vue';

const { meta, diperbarui, rumah, totals, bendaharaList, tarifRumah, TAHUN } = useSheet();
const { pending, cek, tunai, load: loadLedger, loadTahun, ringkasanTahun } = usePembayaranLedger();
onMounted(loadLedger);
const PIN = import.meta.env.VITE_PIN_KAS || '';

const kas   = computed(() => Number(meta.value.kas_tunai || 0));
const bank  = computed(() => Number(meta.value.rekening || 0));
const total = computed(() => totals.value.jumlahRumah);
const persen = computed(() => (total.value ? Math.round((totals.value.lunasBulanIni / total.value) * 100) : 0));

// Setor ke Bank = one Setoran row; API!kas_tunai/rekening move the amount from
// kas to bank by formula, nothing to mark on Pembayaran (docs/sheets-schema.md `Setoran`).
const setorUrl = computed(() =>
  urlSetoran({ nominal: kas.value, oleh: bendaharaNama.value || 'Bendahara' }));

// Who's verifying — same pattern as Pos's petugasAktif: the Kas PIN is shared by
// everyone on the roster (Petugas tab, peran "bendahara" — docs/sheets-schema.md
// `Petugas`), so each person picks their own name once per device before anything shows,
// and it's what gets recorded on Keputusan!oleh.
const bendaharaNama = ref(localStorage.getItem('iuran.bendahara.nama') || '');
function pilihBendahara(nama) {
  bendaharaNama.value = nama;
  localStorage.setItem('iuran.bendahara.nama', nama);
}
function gantiBendahara() {
  bendaharaNama.value = '';
  localStorage.removeItem('iuran.bendahara.nama');
}

// Riwayat kas masuk — bottom sheet, bukan dilempar semua ke halaman utama.
// Lazy-render 10 baris per langkah (bukan lazy-fetch — seluruh tab Pembayaran
// sudah sekali fetch lewat gviz, ini cuma ngerem berapa banyak yang di-render
// sekaligus) — IntersectionObserver di sentinel bawah list nambah 10 lagi
// begitu keliatan, sampai habis.
const showRiwayatKas = ref(false);
useScrollLock(showRiwayatKas);

// Lihat bukti — dulu buka tab baru (link Drive mentah), sekarang tetap di
// halaman ini, gambarnya ditampilkan langsung (driveImageUrl rewrite ke
// endpoint thumbnail Drive yang beneran ngeluarin byte gambar).
const buktiTampil = ref(null);   // { alamat, bulan, buktiUrl } atau null
useScrollLock(buktiTampil);
const buktiGagal = ref(false);
function lihatBukti(p) { buktiGagal.value = false; buktiTampil.value = p; }
// Iuran per tahun — one `Iuran<tahun>` tab per dues year (pre-created for ten
// years in the Sheet, docs/sheets-schema.md), fetched only when picked here.
// Years run from the earliest billing start (RumahRiwayat) to this year.
const showTahunan = ref(false);
useScrollLock(showTahunan);
const tahunList = computed(() => {
  const mulai = rumah.value.map((h) => h.mulai).filter(Boolean);
  const dari = mulai.length ? Math.floor(Math.min(...mulai) / 100) : TAHUN.value;
  return Array.from({ length: TAHUN.value - dari + 1 }, (_, i) => TAHUN.value - i);
});
const tahunPilih = ref(0);
const ringkasan = computed(() => ringkasanTahun(tahunPilih.value));
function pilihTahun(y) { tahunPilih.value = y; loadTahun(y); }
function bukaTahunan() { showTahunan.value = true; pilihTahun(tahunPilih.value || TAHUN.value); }
const METODE_LABEL = { tunai: 'Tunai', transfer: 'Transfer', impor: 'Data lama (impor)' };

const riwayatVisibleN = ref(10);
const riwayatTampil = computed(() => tunai.value.slice(0, riwayatVisibleN.value));
const riwayatScrollEl = ref(null);   // the sheet's own scrolling element — must be
                                      // the observer's `root`, not the viewport, or
                                      // the sentinel counts as "visible" immediately
                                      // and the whole list loads in one burst.
const riwayatSentinel = ref(null);
let riwayatObserver = null;
watch(showRiwayatKas, async (open) => {
  if (!open) { riwayatObserver?.disconnect(); return; }
  riwayatVisibleN.value = 10;
  await nextTick();
  riwayatObserver = new IntersectionObserver((entries) => {
    if (entries[0].isIntersecting && riwayatVisibleN.value < tunai.value.length) {
      riwayatVisibleN.value += 10;
    }
  }, { root: riwayatScrollEl.value, rootMargin: '200px' });
  if (riwayatSentinel.value) riwayatObserver.observe(riwayatSentinel.value);
});

const label = (e) => labelBulan(e.tahun * 100 + e.bulan, TAHUN.value, BULAN);
const bulanList = (g) => g.items.map(label).join(', ');
// Warga fills the Form's rincian themselves (prefilled, but editable) — flag
// any month claimed at less than that month's tarif before it gets verified.
const kurang = (g) => g.items.filter((e) => e.nominal < (tarifRumah(e.alamat, e.tahun, e.bulan) || 0));

// One Keputusan row per submission (Form E, docs/sheets-schema.md
// `Keputusan`): `sah` verifies a transfer, `tolak` rejects it or voids a
// mistaken cash entry — its months go back to "Belum" and can be paid again.
// A submission stays "terkirim" (buttons disabled) until a reload shows its
// new status — gviz caches for minutes, so re-enabling on a timer used to
// invite a second, duplicate verdict.
const terkirim = ref(new Map());   // group key -> 'sah' | 'tolak'
watch([pending, tunai], ([p, t]) => {
  const masih = new Set([...p.map((g) => g.key), ...t.filter((g) => !g.tolak).map((g) => g.key)]);
  terkirim.value = new Map([...terkirim.value].filter(([k]) => masih.has(k)));
});
async function putuskan(g, keputusan) {
  if (terkirim.value.has(g.key)) return;
  if (keputusan === 'tolak' && !confirm(g.items[0].metode === 'tunai'
    ? `Batalkan catatan tunai ${g.alamat} · ${bulanList(g)} (${rupiah(g.total)})?\nBulan-bulan ini kembali "Belum". Uangnya tidak dihitung di Kas Tunai.`
    : `Tolak transfer ${g.alamat} · ${bulanList(g)} (${rupiah(g.total)})?\nBulan-bulan ini kembali "Belum" dan warga perlu mengirim ulang.`)) return;
  terkirim.value = new Map([...terkirim.value, [g.key, keputusan]]);
  await submitKeputusan({ alamat: g.alamat, waktu: g.waktu, keputusan,
                          oleh: bendaharaNama.value || 'Bendahara' });
  setTimeout(loadLedger, 4000);   // sheet cache settles, then the group's status updates
}
</script>

<template>
 <PinGate :pin="PIN" storage-key="kas" title="Kas Bendahara" env-var="VITE_PIN_KAS">

  <!-- PIN dipakai bersama seluruh bendahara/admin/komite; nama dipilih sekali per
       device supaya tiap verifikasi tercatat atas nama yang benar -->
  <section v-if="!bendaharaNama" class="scr col"
           style="gap:var(--space-3);align-items:center;text-align:center;padding-top:var(--space-8)">
    <h3 style="margin:0">Siapa Anda?</h3>
    <p class="text-muted" style="font-size:13px;margin:0">
      Pilih nama Anda — tercatat di setiap verifikasi transfer.
    </p>
    <div class="col" style="gap:var(--space-2);width:100%;max-width:280px">
      <button v-for="nama in bendaharaList" :key="nama" type="button" class="btn btn-secondary"
              style="width:100%;min-height:44px;background:var(--color-surface);box-shadow:var(--shadow-sm)"
              @click="pilihBendahara(nama)">
        {{ nama }}
      </button>
    </div>
    <p v-if="!bendaharaList.length" class="text-muted" style="font-size:12px">
      Daftar nama belum diisi admin di Sheet (tab Petugas, peran "bendahara").
    </p>
  </section>

  <section v-else class="scr col" style="gap:var(--space-3)">
    <div class="spread">
      <div>
        <h4 style="margin:0">Kas RT 03/14</h4>
        <div class="text-muted" style="font-size:11.5px">
          {{ total }} rumah · sinkron {{ diperbarui || '—' }}
        </div>
      </div>
      <div class="row" style="gap:4px;flex:none">
        <button class="btn btn-ghost" style="font-size:12px;padding:6px 8px" title="Riwayat kas masuk"
                @click="showRiwayatKas = true">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor"
               stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="9"></circle>
            <path d="M12 7v5l3 3"></path>
          </svg>
        </button>
        <button class="btn btn-ghost" style="font-size:12px" @click="gantiBendahara">
          {{ bendaharaNama }} · Ganti
        </button>
      </div>
    </div>

    <Card elev="md" style="background:var(--color-neutral-900);color:var(--color-neutral-100)">
      <span class="kick" style="color:var(--color-accent-300)">Kas Tunai di pos</span>
      <div class="num" style="font-family:var(--font-heading);font-size:32px;line-height:1.1">
        {{ rupiah(kas) }}
      </div>
      <Button as="a" :href="setorUrl" target="_blank" block
              style="background:var(--color-accent-400);color:var(--color-neutral-900)">
        Setor ke Bank
      </Button>
      <div style="text-align:center;font-size:10.5px;color:var(--color-neutral-400)">
        atau biarkan sebagai petty cash operasional
      </div>
    </Card>

    <Card>
      <span class="kick">Rekening {{ REKENING.bank }} {{ REKENING.nomor.slice(0, 4) }}-xxxx</span>
      <div class="num" style="font-family:var(--font-heading);font-size:28px;line-height:1.1">
        {{ rupiah(bank) }}
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

    <!-- rumah tanpa baris RumahRiwayat: tidak ditagih sama sekali sampai admin
         mengisinya — jangan sampai diam-diam jadi "Rp0" -->
    <Card v-if="totals.tarifBelumDiatur.length" style="background:var(--color-accent-100);gap:4px">
      <span style="font-size:12.5px;font-weight:700;color:var(--color-accent-800)">
        ⚠ {{ totals.tarifBelumDiatur.length }} rumah belum punya tarif
      </span>
      <span style="font-size:11.5px">
        {{ totals.tarifBelumDiatur.map(h => h.alamat).join(', ') }} — isi baris baseline di tab
        RumahRiwayat. Sampai itu, rumah ini tidak bisa bayar dan tidak masuk target.
      </span>
    </Card>

    <!-- transfer menunggu verifikasi — satu kartu per transfer (satu bukti, bisa
         beberapa bulan). Verifikasi/Tolak menulis satu baris ke tab Keputusan (append-only, lihat
         docs/sheets-schema.md `Keputusan`), bukan mengedit baris asalnya -->
    <Card v-if="pending.length" style="gap:2px">
      <span class="kick">Perlu diverifikasi ({{ pending.length }})</span>
      <div v-for="g in pending" :key="g.key" class="col" style="gap:6px;padding:8px 0;
           border-top:1px solid var(--color-divider)">
        <div class="spread">
          <span style="font-weight:700;font-size:13px">{{ g.alamat }} · {{ bulanList(g) }}</span>
          <span class="num" style="font-weight:700">{{ rupiah(g.total) }}</span>
        </div>
        <div class="text-muted" style="font-size:11px">{{ g.waktu }}</div>
        <div v-if="kurang(g).length" style="font-size:11px;color:var(--color-accent-700)">
          ⚠ Nominal di bawah tarif untuk {{ kurang(g).map(label).join(', ') }} — cocokkan dengan bukti.
          Bulan itu tetap "Belum" walau diverifikasi.
        </div>
        <div class="row" style="gap:6px">
          <button v-if="g.buktiUrl" type="button" class="btn btn-secondary" @click="lihatBukti(g)"
             style="flex:1;justify-content:center;font-size:12px;padding:6px 10px;
                    background:var(--color-surface);box-shadow:var(--shadow-sm)">
            Lihat bukti
          </button>
          <span v-else class="text-muted" style="flex:1;font-size:11px;align-self:center">
            (tanpa bukti — cek Sheet)
          </span>
          <button class="btn btn-ghost" style="flex:none;font-size:12px;padding:6px 10px"
                  :disabled="terkirim.has(g.key)" @click="putuskan(g, 'tolak')">
            {{ terkirim.get(g.key) === 'tolak' ? 'Ditolak ✓' : 'Tolak' }}
          </button>
          <button class="btn btn-primary" style="flex:1;font-size:12px;padding:6px 10px"
                  :disabled="terkirim.has(g.key)" @click="putuskan(g, 'sah')">
            {{ terkirim.get(g.key) === 'sah' ? 'Terkirim ✓' : 'Verifikasi' }}
          </button>
        </div>
      </div>
    </Card>

    <!-- rincian Form transfer tidak cocok dengan totalnya: tidak dihitung sama
         sekali (Pembayaran!I = "cek") — warga perlu kirim ulang konfirmasi -->
    <Card v-if="cek.length" style="gap:2px">
      <span class="kick">Rincian tidak cocok ({{ cek.length }})</span>
      <p class="text-muted" style="font-size:11px;margin:0">
        Tidak dihitung. Minta warga kirim ulang konfirmasi transfer dari aplikasi.
      </p>
      <div v-for="g in cek" :key="g.key" class="spread" style="padding:8px 0;
           border-top:1px solid var(--color-divider);font-size:12.5px">
        <span><b>{{ g.alamat }}</b> · {{ bulanList(g) }}</span>
        <button v-if="g.buktiUrl" type="button" class="btn btn-ghost" style="font-size:11.5px;padding:4px 6px"
                @click="lihatBukti(g)">Lihat bukti</button>
      </div>
    </Card>

    <button type="button" class="btn btn-secondary" @click="bukaTahunan"
            style="justify-content:center;background:var(--color-surface);box-shadow:var(--shadow-sm)">
      Iuran per tahun
    </button>
    <a class="btn btn-secondary" href="#/kas/rumah"
       style="justify-content:center;background:var(--color-surface);box-shadow:var(--shadow-sm)">
      Lihat semua kartu rumah
    </a>
    <a class="btn btn-secondary" href="#/kas/qr"
       style="justify-content:center;background:var(--color-surface);box-shadow:var(--shadow-sm)">
      Cetak QR per rumah
    </a>

    <!-- riwayat kas masuk (tunai) — bottom sheet, lazy-render 10 baris per
         langkah lewat IntersectionObserver di sentinel, bukan nge-dump semua
         audit trail sekaligus ke layar -->
    <div v-if="showRiwayatKas" class="dialog-backdrop sheet-backdrop" @click.self="showRiwayatKas = false">
      <div class="dialog sheet" style="border-radius:var(--radius-lg) var(--radius-lg) 0 0;
           max-height:85dvh">
       <div ref="riwayatScrollEl" class="sheet-scroll">
        <div class="spread">
          <div>
            <div class="dialog-title">Riwayat Kas Masuk</div>
            <div class="text-muted" style="font-size:12px">
              Tunai · cross-check dengan laporan satpam. Salah input? "Batalkan", lalu catat ulang di Pos.
            </div>
          </div>
          <button class="btn btn-ghost" @click="showRiwayatKas = false">×</button>
        </div>

        <div v-if="!tunai.length" class="text-muted" style="text-align:center;font-size:12.5px">
          Belum ada kas masuk tunai.
        </div>
        <div v-else class="col" style="gap:2px">
          <div v-for="g in riwayatTampil" :key="g.key" class="spread"
               style="padding:8px 0;border-top:1px solid var(--color-divider);gap:var(--space-2)">
            <div :style="g.tolak || g.dobel ? 'opacity:.55' : ''">
              <div style="font-size:12.5px;font-weight:600">{{ g.alamat }} · {{ bulanList(g) }}</div>
              <div class="text-muted" style="font-size:11px">
                {{ g.petugas }} · {{ g.waktu }}{{ g.tolak ? ' · dibatalkan' : g.dobel ? ' · dobel, tidak dihitung' : '' }}
              </div>
            </div>
            <div class="col" style="align-items:flex-end;gap:2px;flex:none">
              <span class="num" style="font-weight:700;font-size:12.5px"
                    :style="g.tolak || g.dobel ? 'text-decoration:line-through;opacity:.5' : ''">
                {{ rupiahPendek(g.total) }}
              </span>
              <button v-if="!g.tolak && !g.dobel" class="btn btn-ghost"
                      style="font-size:11px;padding:2px 4px;min-height:0"
                      :disabled="terkirim.has(g.key)" @click="putuskan(g, 'tolak')">
                {{ terkirim.has(g.key) ? 'Dibatalkan ✓' : 'Batalkan' }}
              </button>
            </div>
          </div>
          <div ref="riwayatSentinel" style="height:1px"></div>
          <p v-if="riwayatVisibleN < tunai.length" class="text-muted" style="text-align:center;font-size:11px">
            Memuat lagi…
          </p>
          <p v-else class="text-muted" style="text-align:center;font-size:11px">
            — {{ tunai.length }} dari {{ tunai.length }} —
          </p>
        </div>
       </div>
      </div>
    </div>

    <!-- iuran per tahun — satu tab Sheet `Iuran<tahun>` per tahun iuran,
         diambil hanya saat tahunnya dipilih -->
    <div v-if="showTahunan" class="dialog-backdrop sheet-backdrop" @click.self="showTahunan = false">
      <div class="dialog sheet" style="border-radius:var(--radius-lg) var(--radius-lg) 0 0;
           max-height:85dvh">
       <div class="sheet-scroll">
        <div class="spread">
          <div>
            <div class="dialog-title">Iuran per Tahun</div>
            <div class="text-muted" style="font-size:12px">Menurut bulan tagihan · hanya yang sah</div>
          </div>
          <button class="btn btn-ghost" @click="showTahunan = false">×</button>
        </div>

        <div class="row" style="gap:6px;flex-wrap:wrap">
          <button v-for="y in tahunList" :key="y" type="button" class="btn"
                  :class="tahunPilih === y ? 'btn-primary' : 'btn-secondary'"
                  style="min-height:32px;padding:5px 12px;font-size:12px" @click="pilihTahun(y)">
            {{ y }}
          </button>
        </div>

        <p v-if="!ringkasan" class="text-muted" style="font-size:12px;margin:0">Memuat tab Iuran{{ tahunPilih }}…</p>
        <p v-else-if="ringkasan.error" class="text-muted" style="font-size:12px;margin:0">
          Tab <b>Iuran{{ tahunPilih }}</b> belum ada di Sheet — buat dengan rumus yang sama seperti tab
          tahun lainnya (docs/sheets-schema.md).
        </p>
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

    <!-- bukti transfer — dulu link keluar ke Drive di tab baru, sekarang
         gambarnya langsung ditampilkan di sini -->
    <div v-if="buktiTampil" class="dialog-backdrop sheet-backdrop" @click.self="buktiTampil = null">
      <div class="dialog sheet" style="border-radius:var(--radius-lg) var(--radius-lg) 0 0;
           max-height:85dvh">
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

        <img v-if="!buktiGagal" :src="driveImageUrl(buktiTampil.buktiUrl)" alt="Bukti transfer"
             style="width:100%;border-radius:var(--radius-md);display:block"
             @error="buktiGagal = true">
        <p v-else class="text-muted" style="text-align:center;font-size:12.5px">
          Gambar tidak bisa dimuat di sini — mungkin izin file Drive-nya belum "siapa saja
          dengan link".
          <a :href="buktiTampil.buktiUrl" target="_blank">Buka langsung di Drive →</a>
        </p>
       </div>
      </div>
    </div>
  </section>
 </PinGate>
</template>
