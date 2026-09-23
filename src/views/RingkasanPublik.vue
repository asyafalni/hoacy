<script setup vapor>
import { ref, computed } from 'vue';
import { useSheet } from '../composables/useSheet';
import { useChartTooltip } from '../composables/useChartTooltip';
import { useScrollLock } from '../composables/useScrollLock';
import { rupiah, rupiahPendek, BULAN } from '../lib/tariff';
import { tahunOf, selisihBulan } from '../lib/tagihan';
import Card from '../components/ui/Card.vue';

// Public, no PIN, on purpose — this is the one screen in the app anyone can open
// unauthenticated. That constrains what it's allowed to show: cluster-wide totals
// only, nothing that names or narrows down to a specific house. No per-block
// breakdown (a block is only ~10-20 houses — small enough that "this block is
// behind" starts to be identifying once combined with what neighbours already
// know informally), no per-payment activity feed (timing + amount is basically
// a fingerprint for who paid), no resident names or addresses. Under UU PDP,
// pure aggregate figures that can't be traced to an individual sit outside the
// law's definition of "data pribadi" — that's the line every number on this page
// is checked against. See the Q&A in project history for the full reasoning.
// This is also why this page uses `useSheet()`'s `riwayat` (Riwayat tab, `D-Riwayat` —
// a pre-aggregated SUMIFS per month) instead of `usePembayaranLedger`: that
// composable fetches the raw Pembayaran ledger (alamat, petugas, bukti_url —
// a photo of someone's transfer proof), fine for the PIN-gated Kas screen but
// not for a zero-barrier public page. See docs/sheets-schema.md `D-API/Riwayat`.
// Data is loaded once by App.vue — no load() here (it used to fetch twice).
const { meta, rumah, totals, opexList, riwayat, targetPada, TAHUN, sekarang } = useSheet();

const kas = computed(() => Number(meta.value.kas_tunai || 0));
const bank = computed(() => Number(meta.value.rekening || 0));
const tunggakan = computed(() => totals.value.tunggakan);
const jumlahRumah = computed(() => totals.value.jumlahRumah);

const terkumpul = computed(() => totals.value.terkumpulBulanIni);
const target = computed(() => totals.value.target);
const persenTarget = computed(() => (target.value ? Math.min(100, Math.round((terkumpul.value / target.value) * 100)) : 0));

const opex = computed(() => totals.value.opex);
const opexDiperbarui = computed(() => totals.value.opexDiperbarui);
const opexBelumDiisi = computed(() => !opex.value);

// Riwayat terkumpul vs target, per bulan — dari Riwayat tab (`D-Riwayat`), pre-agregat
// di Sheet lewat SUMIFS, bukan dihitung di sini dari Pembayaran mentah (lihat
// catatan PDP di atas). Target tiap bulan = tarif yang BERLAKU bulan itu
// (RumahRiwayat/TarifVersi) dijumlah untuk semua rumah aktif — jadi kenaikan
// tarif atau rumah yang baru mulai ditagih kelihatan di garisnya.
const riwayatBulanan = computed(() => riwayat.value.map((b) => ({
  ...b, key: `${b.tahun}-${b.bulan}`, target: targetPada(b.tahun, b.bulan),
})));

const DURASI_OPT = [
  { value: '3', label: '3 bulan' },
  { value: '6', label: '6 bulan' },
  { value: '12', label: '1 tahun' },
  { value: 'semua', label: 'Semua' },
];
const durasi = ref('6');
const riwayatTampil = computed(() => (durasi.value === 'semua'
  ? riwayatBulanan.value
  : riwayatBulanan.value.slice(-Number(durasi.value))));
const riwayatMax = computed(() =>
  Math.max(1, ...riwayatTampil.value.map((b) => Math.max(b.terkumpul, b.target))));

const { wrapEl: riwayatWrapEl, tip: riwayatTip, show: showRiwayatTip, hide: hideRiwayatTip } = useChartTooltip();

// Warga yang bayar setahun sekaligus bikin sebagian kas "sudah dititipkan" buat
// bulan-bulan depan — itu kewajiban (jasa yang masih harus RT berikan), bukan
// surplus bebas pakai: setiap bulan yang sudah sah ('D-API'!L) tapi belum jatuh
// tempo, dinilai dengan tarif yang berlaku di bulan itu. Cuma agregat yang
// tampil di sini — hitungan per-rumah tetap tidak pernah dirender.
const dibayarDimukaDetail = computed(() => {
  let rumahCount = 0, bulanTahunIni = 0, nominalTahunIni = 0, bulanTahunDepan = 0, nominalTahunDepan = 0;
  for (const h of rumah.value) {
    const muka = [...h.lunas].filter((p) => p > sekarang.value);
    if (muka.length) rumahCount += 1;
    for (const p of muka) {
      const nominal = h.tarifPer(p) || 0;
      if (tahunOf(p) === TAHUN.value) { bulanTahunIni += 1; nominalTahunIni += nominal; }
      else { bulanTahunDepan += 1; nominalTahunDepan += nominal; }
    }
  }
  return { rumahCount, bulanTahunIni, nominalTahunIni, bulanTahunDepan, nominalTahunDepan,
           total: nominalTahunIni + nominalTahunDepan };
});
const dibayarDimuka = computed(() => dibayarDimukaDetail.value.total);

// Bukan surplus/defisit bulan ini — itu bikin kesan untung-rugi, padahal iuran
// itu penagihannya nggak pasti (telat, nunggak, dst). Yang lebih relevan buat RT
// adalah runway: berapa lama kas bersih (kas+rekening, dikurangi yang sudah
// dititipkan di muka) masih nutup OPEX (termasuk gaji karyawan) kalau iuran
// berhenti masuk sama sekali bulan ini.
const kasBersih = computed(() => kas.value + bank.value - dibayarDimuka.value);
const runwayBulan = computed(() => (opex.value ? kasBersih.value / opex.value : 0));
const runwayAman = computed(() => runwayBulan.value >= 3);

// Aging piutang: umur dihitung dari bulan tertunggak paling lama sampai bulan
// berjalan. Tunggakan di bawah 3 bulan itu wajar (telat bayar biasa, belum
// perlu ditindaklanjuti) — baru masuk hitungan "aging" begitu sudah 3 bulan
// atau lebih tidak dibayar, itu sinyal buat bendahara mulai follow up personal.
// Cuma agregat (jumlah rumah, rata-rata umur, total nominal) yang tampil, sama
// seperti angka lain di halaman ini — tidak ada rumah mana yang disebut.
const tunggakanAging = computed(() => {
  // Lintas tahun: umur dihitung dari bulan tertunggak paling lama (h.tertua,
  // src/lib/tagihan.js) — bukan cuma tahun berjalan.
  const rumahNunggak = rumah.value.filter((h) => h.tertua).map((h) => ({
    umur: selisihBulan(h.tertua, sekarang.value) + 1,
    nominal: h.tunggakan,
  }));

  const aging = rumahNunggak.filter((h) => h.umur >= 3);

  // Distribusi umur lebih kepake buat bendahara daripada satu angka rata-rata
  // (rata-rata gampang ketutup satu rumah nunggak ekstrem lama).
  // warna makin tua/gelap makin lama umurnya — sinyal visual sekilas, tanpa
  // perlu baca angka dulu
  const BUCKET = [
    { label: '3–6 bulan', test: (u) => u < 6, warna: 'var(--color-accent-300)' },
    { label: '6–12 bulan', test: (u) => u >= 6 && u < 12, warna: 'var(--color-accent-500)' },
    { label: '1–3 tahun', test: (u) => u >= 12 && u < 36, warna: 'var(--color-accent-700)' },
    { label: '> 3 tahun', test: (u) => u >= 36, warna: 'var(--color-accent-900)' },
  ];
  const nominalAgingTotal = aging.reduce((sum, h) => sum + h.nominal, 0);
  const distribusi = BUCKET.map((b) => {
    const di = aging.filter((h) => b.test(h.umur));
    const nominal = di.reduce((sum, h) => sum + h.nominal, 0);
    return {
      label: b.label,
      jumlah: di.length,
      nominal,
      persenRumah: aging.length ? Math.round((di.length / aging.length) * 100) : 0,
      persenNominal: nominalAgingTotal ? Math.round((nominal / nominalAgingTotal) * 100) : 0,
      warna: b.warna,
    };
  });

  return {
    jumlahRumah: rumahNunggak.length,
    belumDianggap: rumahNunggak.length - aging.length,
    jumlahAging: aging.length,
    nominalAging: nominalAgingTotal,
    distribusi,
  };
});

// Dua pie chart per permintaan bendahara: kiri = distribusi jumlah rumah,
// kanan = distribusi nominal — bisa beda bentuk (satu rumah nunggak besar
// mendominasi pie nominal walau cuma 1 dari banyak rumah di pie jumlah), jadi
// dua chart terpisah, bukan satu angka gabungan. SVG stroke-dasharray donut
// (bukan conic-gradient div) supaya tiap slice punya elemen sendiri buat
// hover/tap — itu yang bikin tooltip per-slice bisa jalan.
const PIE_R = 50;
const PIE_C = 2 * Math.PI * PIE_R;
function pieSegments(parts, key) {
  const total = parts.reduce((sum, p) => sum + p[key], 0);
  let offset = 0;
  return parts.filter((p) => p[key] > 0).map((p) => {
    const len = total ? (p[key] / total) * PIE_C : 0;
    const seg = { ...p, len, offset };
    offset += len;
    return seg;
  });
}
const segRumah = computed(() => pieSegments(tunggakanAging.value.distribusi, 'jumlah'));
const segNominal = computed(() => pieSegments(tunggakanAging.value.distribusi, 'nominal'));

// Tooltip: satu state dipakai kedua pie (rumah & nominal), posisinya relatif
// ke pieWrapEl (bukan halaman) supaya nggak perlu urus scroll offset.
const { wrapEl: pieWrapEl, tip, show: showTip, hide: hideTip } = useChartTooltip();

const showOpex = ref(false);
const showRiwayat = ref(false);
const showDibayarDimuka = ref(false);
useScrollLock(showOpex);
useScrollLock(showRiwayat);
useScrollLock(showDibayarDimuka);
</script>

<template>
  <section class="scr col" style="gap:var(--space-3)">
    <div class="spread">
      <div>
        <h4 style="margin:0">Ringkasan Kas Cluster</h4>
        <div class="text-muted" style="font-size:11.5px">
          {{ jumlahRumah }} rumah · terbuka untuk siapa saja, tanpa data per-rumah
        </div>
      </div>
      <a href="#/" class="btn btn-ghost" style="font-size:12px;flex:none">← Beranda</a>
    </div>

    <!-- Hero: runway, bukan angka bulan-ini — penagihan iuran itu kerjaan yang
         tidak pasti, jadi yang paling penting ditampilkan duluan adalah berapa
         lama kas+rekening masih nutup OPEX (termasuk gaji karyawan) kalau iuran
         berhenti masuk sama sekali, bukan untung-rugi bulan ini. -->
    <Card v-if="!opexBelumDiisi" elev="md" style="background:var(--color-neutral-900);color:var(--color-neutral-100)">
      <span class="kick" :style="{ color: runwayAman ? 'var(--color-accent-2-300)' : 'var(--color-accent-300)' }">
        Runway kas cluster
      </span>
      <div class="num" style="font-family:var(--font-heading);font-size:32px;line-height:1.1"
           :style="{ color: runwayAman ? 'var(--color-accent-2-300)' : 'var(--color-accent-300)' }">
        {{ runwayBulan.toFixed(1) }} bulan
      </div>
      <p style="font-size:11.5px;margin:0;color:var(--color-neutral-400)">
        Kalau iuran berhenti masuk sama sekali mulai sekarang, kas bersih yang ada masih
        cukup menutup semua pengeluaran cluster (termasuk gaji karyawan) selama kurang
        lebih segitu — "kas bersih" karena titipan warga yang sudah bayar di muka sudah
        dikeluarkan dulu, itu bukan kas bebas pakai. Penagihan iuran itu kerjaan yang
        tidak pasti, jadi runway inilah yang perlu dijaga, bukan untung-rugi bulan ini.
      </p>

      <!-- rincian hitungan, buat yang mau cross-check angkanya sendiri -->
      <div class="col" style="gap:6px;background:rgba(255,255,255,.06);border-radius:var(--radius-md);
           padding:var(--space-3) var(--space-4);font-size:12px">
        <div class="spread" style="color:var(--color-neutral-300)">
          <span>Kas tunai di pos</span>
          <span class="num" style="font-weight:700;color:var(--color-neutral-100)">{{ rupiah(kas) }}</span>
        </div>
        <div class="spread" style="color:var(--color-neutral-300)">
          <span>Saldo rekening bank</span>
          <span class="num" style="font-weight:700;color:var(--color-neutral-100)">{{ rupiah(bank) }}</span>
        </div>
        <div class="spread" style="color:var(--color-neutral-300);cursor:pointer"
             @click="showDibayarDimuka = true">
          <span style="text-decoration:underline dotted;text-underline-offset:2px">
            − Dibayar di muka (blm jatuh tempo)
          </span>
          <span class="num" style="font-weight:700;color:var(--color-neutral-100)">{{ rupiah(dibayarDimuka) }}</span>
        </div>
        <div class="spread" style="padding-top:6px;border-top:1px solid rgba(255,255,255,.14)">
          <span style="font-weight:700">= Kas bersih</span>
          <span class="num" style="font-weight:700;color:var(--color-neutral-100)">{{ rupiah(kasBersih) }}</span>
        </div>
        <div class="spread" style="color:var(--color-neutral-300)">
          <span>÷ OPEX minimum / bulan</span>
          <span class="num" style="font-weight:700;color:var(--color-neutral-100)">{{ rupiah(opex) }}</span>
        </div>
        <div class="spread" style="padding-top:6px;border-top:1px solid rgba(255,255,255,.14)">
          <span style="font-weight:700">= Runway</span>
          <span class="num" style="font-weight:700"
                :style="{ color: runwayAman ? 'var(--color-accent-2-300)' : 'var(--color-accent-300)' }">
            {{ runwayBulan.toFixed(1) }} bulan
          </span>
        </div>
      </div>

      <button type="button" class="btn" style="width:100%;justify-content:center;font-size:12px;
              background:var(--color-accent-400);color:var(--color-neutral-900)"
              @click="showOpex = true">
        Lihat rincian OPEX →
      </button>
    </Card>
    <Card v-else elev="md" style="background:var(--color-neutral-900);color:var(--color-neutral-100)">
      <span class="kick" style="color:var(--color-accent-300)">Runway kas cluster</span>
      <p style="font-size:12px;margin:0;color:var(--color-neutral-400)">
        Bendahara belum mengisi OPEX bulanan minimum di Sheet — runway belum bisa dihitung.
      </p>
    </Card>

    <!-- Tap buat buka riwayat (bottom sheet) — kartu ini sendiri tetap nunjukin
         angka bulan berjalan, riwayatnya nggak perlu makan tempat di halaman utama. -->
    <Card style="cursor:pointer" @click="showRiwayat = true">
      <div class="spread">
        <span class="kick">Terkumpul bulan ini</span>
        <span class="text-muted" style="font-size:11px">Riwayat →</span>
      </div>
      <div class="num" style="font-family:var(--font-heading);font-size:22px;line-height:1.1">
        {{ rupiah(terkumpul) }}
      </div>
      <div style="height:8px;border-radius:999px;background:var(--color-neutral-200);overflow:hidden">
        <div :style="{ width: persenTarget + '%', height: '100%', background: 'var(--color-accent-2-500)' }"></div>
      </div>
      <div class="spread" style="font-size:11.5px">
        <span class="text-muted">{{ persenTarget }}% dari target</span>
        <span class="text-muted">Target: {{ rupiah(target) }}</span>
      </div>
    </Card>

    <!-- Tunggakan, kartu sendiri per permintaan bendahara — bukan cuma total nominal,
         tapi juga umurnya. Iuran ini kewajiban bersama, bukan sesuatu yang boleh
         santai soal keterlambatan — ambang 3 bulan di sini murni operasional
         (kapan mulai follow up personal), bukan bilang keterlambatan itu wajar. -->
    <Card>
      <span class="kick">Tunggakan cluster</span>
      <div class="num" style="font-family:var(--font-heading);font-size:26px;line-height:1.1;
           color:var(--color-accent-700)">
        {{ rupiah(tunggakan) }}
      </div>
      <p class="text-muted" style="font-size:11.5px;margin:0">
        {{ tunggakanAging.jumlahRumah }} rumah punya tunggakan, semua tahun.
      </p>

      <div class="col" style="gap:6px;background:var(--color-bg);border-radius:var(--radius-md);
           padding:var(--space-3) var(--space-4);font-size:12.5px">
        <div class="spread">
          <span class="text-muted">Belum dianggap aging (&lt; 3 bulan)</span>
          <span class="num" style="font-weight:700">{{ tunggakanAging.belumDianggap }} rumah</span>
        </div>
        <div class="spread" style="padding-top:6px;border-top:1px solid var(--color-divider)">
          <span class="text-muted">Sudah aging (≥ 3 bulan nunggak)</span>
          <span class="num" style="font-weight:700;color:var(--color-accent-700)">
            {{ tunggakanAging.jumlahAging }} rumah
          </span>
        </div>
        <div class="spread" v-if="tunggakanAging.jumlahAging">
          <span class="text-muted">Nominal yang sudah aging</span>
          <span class="num" style="font-weight:700;color:var(--color-accent-700)">
            {{ rupiah(tunggakanAging.nominalAging) }}
          </span>
        </div>
      </div>

      <!-- distribusi umur, dua pie chart — lebih kepake daripada satu angka
           rata-rata (rata-rata gampang ketutup satu rumah nunggak ekstrem
           lama). Kiri & kanan bisa beda bentuk: satu rumah nunggak gede bisa
           mendominasi pie nominal walau cuma 1 dari banyak rumah di pie jumlah. -->
      <div v-if="tunggakanAging.jumlahAging" class="col" style="gap:var(--space-3)">
        <div ref="pieWrapEl" class="row" style="position:relative;gap:var(--space-4);justify-content:center"
             @click.self="hideTip">
          <div class="col" style="align-items:center;gap:6px">
            <div style="position:relative;width:112px;height:112px">
              <svg viewBox="0 0 120 120" width="112" height="112" style="transform:rotate(-90deg)">
                <circle cx="60" cy="60" :r="PIE_R" fill="none" stroke="var(--color-neutral-200)" stroke-width="20" />
                <circle v-for="seg in segRumah" :key="seg.label" cx="60" cy="60" :r="PIE_R" fill="none"
                        :stroke="seg.warna" stroke-width="20"
                        :stroke-dasharray="`${seg.len} ${PIE_C - seg.len}`" :stroke-dashoffset="-seg.offset"
                        style="cursor:pointer" @mouseenter="showTip($event, seg)" @mousemove="showTip($event, seg)"
                        @mouseleave="hideTip" @click.stop="showTip($event, seg)" />
              </svg>
              <div style="position:absolute;inset:16px;border-radius:50%;background:var(--color-surface);
                   display:flex;flex-direction:column;align-items:center;justify-content:center;pointer-events:none">
                <span class="num" style="font-weight:700;font-size:16px">{{ tunggakanAging.jumlahAging }}</span>
                <span class="text-muted" style="font-size:9px">rumah</span>
              </div>
            </div>
            <span class="text-muted" style="font-size:10.5px;text-align:center">Jumlah rumah</span>
          </div>
          <div class="col" style="align-items:center;gap:6px">
            <div style="position:relative;width:112px;height:112px">
              <svg viewBox="0 0 120 120" width="112" height="112" style="transform:rotate(-90deg)">
                <circle cx="60" cy="60" :r="PIE_R" fill="none" stroke="var(--color-neutral-200)" stroke-width="20" />
                <circle v-for="seg in segNominal" :key="seg.label" cx="60" cy="60" :r="PIE_R" fill="none"
                        :stroke="seg.warna" stroke-width="20"
                        :stroke-dasharray="`${seg.len} ${PIE_C - seg.len}`" :stroke-dashoffset="-seg.offset"
                        style="cursor:pointer" @mouseenter="showTip($event, seg)" @mousemove="showTip($event, seg)"
                        @mouseleave="hideTip" @click.stop="showTip($event, seg)" />
              </svg>
              <div style="position:absolute;inset:16px;border-radius:50%;background:var(--color-surface);
                   display:flex;flex-direction:column;align-items:center;justify-content:center;pointer-events:none">
                <span class="num" style="font-weight:700;font-size:13px">{{ rupiahPendek(tunggakanAging.nominalAging) }}</span>
                <span class="text-muted" style="font-size:9px">nominal</span>
              </div>
            </div>
            <span class="text-muted" style="font-size:10.5px;text-align:center">Nominal</span>
          </div>

          <!-- tooltip: satu state buat kedua chart, posisi relatif ke pieWrapEl -->
          <div v-if="tip" style="position:absolute;pointer-events:none;z-index:1;
               background:var(--color-neutral-900);color:var(--color-neutral-100);
               border-radius:var(--radius-sm);padding:6px 10px;font-size:11px;line-height:1.5;
               white-space:nowrap;box-shadow:var(--shadow-md)"
               :style="{ left: tip.x + 'px', top: tip.y + 'px', transform: 'translate(-50%, -115%)' }">
            <div style="font-weight:700">{{ tip.label }}</div>
            <div>{{ tip.jumlah }} rumah · {{ tip.persenRumah }}% rumah</div>
            <div>{{ rupiah(tip.nominal) }} · {{ tip.persenNominal }}% nominal</div>
          </div>
        </div>

        <!-- satu legend buat kedua chart, biar labelnya nggak dobel -->
        <div class="col" style="gap:5px">
          <div v-for="b in tunggakanAging.distribusi" :key="b.label" class="row"
               style="gap:8px;align-items:center;font-size:11.5px">
            <span :style="{ background: b.warna }" style="width:10px;height:10px;border-radius:3px;flex:none"></span>
            <span class="text-muted grow">{{ b.label }}</span>
            <span class="num" style="font-weight:700;width:78px;text-align:right">{{ b.persenRumah }}% rumah</span>
            <span class="num" style="font-weight:700;width:88px;text-align:right">{{ b.persenNominal }}% nominal</span>
          </div>
        </div>
      </div>

      <p class="text-muted" style="font-size:10.5px;margin:0">
        Umur dihitung dari bulan tertunggak paling lama (termasuk tahun-tahun sebelumnya)
        sampai bulan berjalan. Tunggakan baru mulai "diumurkan" setelah 3 bulan tidak
        dibayar — sinyal buat bendahara mulai follow up personal, bukan sekadar telat
        bayar biasa. Transfer yang masih menunggu verifikasi tidak dihitung tunggakan.
      </p>
    </Card>

    <p class="text-muted" style="font-size:10.5px;text-align:center">
      Halaman ini cuma nampilin angka gabungan seluruh cluster — tidak ada nama,
      alamat, atau status bayar rumah tertentu.
    </p>

    <!-- rincian OPEX — bottom sheet, bukan halaman terpisah -->
    <div v-if="showOpex" class="dialog-backdrop sheet-backdrop" @click.self="showOpex = false">
      <div class="dialog sheet" style="border-radius:var(--radius-lg) var(--radius-lg) 0 0;
           max-height:85dvh">
       <div class="sheet-scroll">
        <div class="spread">
          <div>
            <div class="dialog-title">Rincian OPEX Bulanan</div>
            <div class="text-muted" style="font-size:12px">
              Per kategori, bukan per orang<span v-if="opexDiperbarui"> · diperbarui {{ opexDiperbarui }}</span>
            </div>
          </div>
          <button class="btn btn-ghost" @click="showOpex = false">×</button>
        </div>

        <div v-for="o in opexList" :key="o.kategori" class="row" style="gap:var(--space-3)">
          <div style="width:40px;height:40px;flex:none;border-radius:50%;background:var(--color-accent-2-100);
                      display:flex;align-items:center;justify-content:center;font-size:18px">
            {{ o.ikon }}
          </div>
          <span class="grow" style="font-size:13.5px;font-weight:600">{{ o.kategori }}</span>
          <span class="num" style="font-weight:700">{{ rupiah(o.nominal) }}</span>
        </div>

        <p v-if="!opexList.length" class="text-muted" style="text-align:center;font-size:12.5px">
          Bendahara belum mengisi rincian OPEX di Sheet (tab M-Opex).
        </p>

        <div v-else class="spread" style="background:var(--color-neutral-900);color:var(--color-neutral-100);
             border-radius:var(--radius-md);padding:var(--space-3) var(--space-4)">
          <span style="font-size:13.5px;font-weight:700">Total OPEX / bulan</span>
          <span class="num" style="font-family:var(--font-heading);font-size:21px;color:var(--color-accent-300)">
            {{ rupiah(opex) }}
          </span>
        </div>

        <p class="text-muted" style="font-size:10.5px;text-align:center">
          Diedit bendahara langsung di Google Sheet — begitu diubah, angka ini otomatis ikut.
        </p>
       </div>
      </div>
    </div>

    <!-- riwayat terkumpul vs target — bottom sheet, dibuka dari kartu "Terkumpul
         bulan ini". Garis putus-putus di tiap batang = target bulan itu
         (lihat catatan di script, riwayatBulanan). -->
    <div v-if="showRiwayat" class="dialog-backdrop sheet-backdrop" @click.self="showRiwayat = false">
      <div class="dialog sheet" style="border-radius:var(--radius-lg) var(--radius-lg) 0 0;
           max-height:85dvh">
       <div class="sheet-scroll">
        <div class="spread">
          <div>
            <div class="dialog-title">Riwayat Terkumpul vs Target</div>
            <div class="text-muted" style="font-size:12px">Agregat seluruh cluster, per bulan</div>
          </div>
          <button class="btn btn-ghost" @click="showRiwayat = false">×</button>
        </div>

        <div class="seg" style="align-self:flex-start">
          <label v-for="opt in DURASI_OPT" :key="opt.value" class="seg-opt">
            <input type="radio" name="durasi-ringkasan" :value="opt.value" v-model="durasi">
            {{ opt.label }}
          </label>
        </div>

        <div v-if="riwayatTampil.length" ref="riwayatWrapEl" style="position:relative">
          <div style="position:relative;height:140px">
            <div style="display:flex;align-items:flex-end;gap:4px;height:100%">
              <div v-for="b in riwayatTampil" :key="b.key" style="flex:1;height:100%;
                   display:flex;align-items:flex-end;min-width:0;position:relative">
                <div style="position:absolute;left:-2px;right:-2px;border-top:1.5px dashed var(--color-accent-700);
                     opacity:.65;pointer-events:none" :style="{ bottom: (b.target / riwayatMax) * 100 + '%' }"></div>
                <div style="width:100%;border-radius:3px 3px 0 0;cursor:pointer;transition:opacity .12s;
                     background:var(--color-accent-2-500)"
                     :style="{ height: Math.max(2, (b.terkumpul / riwayatMax) * 100) + '%' }"
                     @mouseenter="showRiwayatTip($event, b)" @mousemove="showRiwayatTip($event, b)"
                     @mouseleave="hideRiwayatTip" @click="showRiwayatTip($event, b)"></div>
              </div>
            </div>
          </div>

          <div style="display:flex;gap:4px;margin-top:4px">
            <span v-for="(b, i) in riwayatTampil" :key="b.key" style="flex:1;text-align:center;
                  font-size:8.5px;min-width:0" class="text-muted">
              {{ i % Math.max(1, Math.ceil(riwayatTampil.length / 12)) === 0 ? BULAN[b.bulan - 1].slice(0, 3) : '' }}
            </span>
          </div>

          <div v-if="riwayatTip" style="position:absolute;pointer-events:none;z-index:1;
               background:var(--color-neutral-900);color:var(--color-neutral-100);
               border-radius:var(--radius-sm);padding:6px 10px;font-size:11px;line-height:1.5;
               white-space:nowrap;box-shadow:var(--shadow-md)"
               :style="{ left: riwayatTip.x + 'px', top: riwayatTip.y + 'px', transform: 'translate(-50%, -115%)' }">
            <div style="font-weight:700">{{ BULAN[riwayatTip.bulan - 1] }} {{ riwayatTip.tahun }}</div>
            <div>Terkumpul: {{ rupiah(riwayatTip.terkumpul) }}</div>
            <div>Target: {{ rupiah(riwayatTip.target) }}</div>
          </div>
        </div>
        <p v-else class="text-muted" style="font-size:12px;margin:0">Belum ada riwayat pembayaran.</p>

        <p class="text-muted" style="font-size:10.5px;text-align:center">
          Garis putus-putus = target bulan itu: tarif yang berlaku saat itu untuk semua
          rumah yang sudah ditagih. Batang = iuran yang masuk untuk bulan tersebut.
        </p>
       </div>
      </div>
    </div>

    <!-- rincian dibayar di muka — bottom sheet, dibuka dari baris "Dibayar di
         muka" pada breakdown kartu Runway di atas. -->
    <div v-if="showDibayarDimuka" class="dialog-backdrop sheet-backdrop" @click.self="showDibayarDimuka = false">
      <div class="dialog sheet" style="border-radius:var(--radius-lg) var(--radius-lg) 0 0;
           max-height:85dvh">
       <div class="sheet-scroll">
        <div class="spread">
          <div>
            <div class="dialog-title">Dibayar di Muka</div>
            <div class="text-muted" style="font-size:12px">Kewajiban, bukan kas bebas pakai</div>
          </div>
          <button class="btn btn-ghost" @click="showDibayarDimuka = false">×</button>
        </div>

        <div class="num" style="font-family:var(--font-heading);font-size:26px;line-height:1.1">
          {{ rupiah(dibayarDimuka) }}
        </div>
        <p class="text-muted" style="font-size:11.5px;margin:0">
          {{ dibayarDimukaDetail.rumahCount }} rumah sudah titip iuran di muka — kewajiban RT
          buat bulan-bulan itu, sudah dikurangkan dari kas bersih &amp; runway.
        </p>

        <div class="col" style="gap:10px;background:var(--color-bg);border-radius:var(--radius-md);
             padding:var(--space-3) var(--space-4);font-size:12.5px">
          <div class="col" style="gap:2px">
            <span class="text-muted" style="font-size:11.5px">Bulan tahun ini (lunas sebelum jatuh tempo)</span>
            <span class="num" style="font-weight:700">
              {{ dibayarDimukaDetail.bulanTahunIni }} bulan · {{ rupiah(dibayarDimukaDetail.nominalTahunIni) }}
            </span>
          </div>
          <div class="col" style="gap:2px;padding-top:8px;border-top:1px solid var(--color-divider)">
            <span class="text-muted" style="font-size:11.5px">Bulan tahun depan</span>
            <span class="num" style="font-weight:700">
              {{ dibayarDimukaDetail.bulanTahunDepan }} bulan · {{ rupiah(dibayarDimukaDetail.nominalTahunDepan) }}
            </span>
          </div>
        </div>

        <p class="text-muted" style="font-size:10.5px;text-align:center">
          Titipan warga yang bayar iuran di muka (mis. setahun sekaligus). RT masih "berutang"
          jasa buat bulan-bulan itu, jadi ini bukan surplus — beda dari tunggakan (piutang), ini
          justru kebalikannya.
        </p>
       </div>
      </div>
    </div>
  </section>
</template>
