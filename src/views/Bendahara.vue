<script setup vapor>
import { ref, computed, onMounted } from 'vue';
import { useSheet } from '../composables/useSheet';
import { usePembayaranLedger } from '../composables/usePembayaranLedger';
import { rupiah, rupiahPendek, BULAN, REKENING } from '../lib/tariff';
import { urlSetoran, batchId, submitVerifikasi } from '../lib/forms';
import Card from '../components/ui/Card.vue';
import Button from '../components/ui/Button.vue';
import PinGate from '../components/PinGate.vue';

const { meta, rumah, bendaharaList } = useSheet();
const { pending, tunai, load: loadLedger } = usePembayaranLedger();
onMounted(loadLedger);
const PIN = import.meta.env.VITE_PIN_KAS || '';

const kas   = computed(() => Number(meta.value.kas_tunai || 0));
const bank  = computed(() => Number(meta.value.rekening || 0));
const lunas = computed(() => Number(meta.value.lunas_bulan_ini || 0));
const total = computed(() => Number(meta.value.jumlah_rumah || rumah.value.length || 0));
const persen = computed(() => (total.value ? Math.round((lunas.value / total.value) * 100) : 0));

// Setor ke Bank = one Setoran row; then paste the batch id into Pembayaran!L
// for the rows currently marked "kas" (docs/sheets-schema.md §4).
const setorUrl = computed(() =>
  urlSetoran({ batchId: batchId(), nominal: kas.value, oleh: 'Bendahara' }));

// Who's verifying — same pattern as Pos's petugasAktif: the Kas PIN is shared by
// everyone on the roster (API!AF), so each person picks their own name once per
// device before anything shows, and it's what gets recorded on Verifikasi!oleh.
const bendaharaNama = ref(localStorage.getItem('iuran.bendahara.nama') || '');
function pilihBendahara(nama) {
  bendaharaNama.value = nama;
  localStorage.setItem('iuran.bendahara.nama', nama);
}
function gantiBendahara() {
  bendaharaNama.value = '';
  localStorage.removeItem('iuran.bendahara.nama');
}

const keyOf = (p) => `${p.alamat}-${p.bulan}-${p.tahun}`;
const verifying = ref([]);
async function verifikasi(p) {
  const k = keyOf(p);
  if (verifying.value.includes(k)) return;
  verifying.value = [...verifying.value, k];
  try {
    await submitVerifikasi({ alamat: p.alamat, bulan: p.bulan, tahun: p.tahun,
                              oleh: bendaharaNama.value || 'Bendahara' });
    setTimeout(loadLedger, 3000);   // sheet cache settles, then the row drops off `pending`
  } finally {
    setTimeout(() => { verifying.value = verifying.value.filter((x) => x !== k); }, 3500);
  }
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
      Daftar nama belum diisi admin di Sheet (tab API, kolom AF).
    </p>
  </section>

  <section v-else class="scr col" style="gap:var(--space-3)">
    <div class="spread">
      <div>
        <h4 style="margin:0">Kas RT 03/14</h4>
        <div class="text-muted" style="font-size:11.5px">
          {{ total }} rumah · sinkron {{ meta.updated || '—' }}
        </div>
      </div>
      <button class="btn btn-ghost" style="font-size:12px" @click="gantiBendahara">
        {{ bendaharaNama }} · Ganti
      </button>
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
          {{ rupiahPendek(meta.tunggakan_total || 0) }}
        </span>
      </div>
    </Card>

    <!-- transfer menunggu verifikasi — bukti (Pembayaran!I, Drive) bisa dibuka
         sebelum tap Verifikasi, yang menulis ke tab Verifikasi (append-only, lihat
         docs/sheets-schema.md §5), bukan mengedit baris asalnya -->
    <Card v-if="pending.length" style="gap:2px">
      <span class="kick">Perlu diverifikasi ({{ pending.length }})</span>
      <div v-for="p in pending" :key="keyOf(p)" class="col" style="gap:6px;padding:8px 0;
           border-top:1px solid var(--color-divider)">
        <div class="spread">
          <span style="font-weight:700;font-size:13px">{{ p.alamat }} · {{ BULAN[p.bulan - 1] }}</span>
          <span class="num" style="font-weight:700">{{ rupiah(p.nominal) }}</span>
        </div>
        <div class="row" style="gap:6px">
          <a v-if="p.buktiUrl" class="btn btn-secondary" :href="p.buktiUrl" target="_blank"
             style="flex:1;justify-content:center;font-size:12px;padding:6px 10px;
                    background:var(--color-surface);box-shadow:var(--shadow-sm)">
            Lihat bukti
          </a>
          <span v-else class="text-muted" style="flex:1;font-size:11px;align-self:center">
            (tanpa bukti — cek Sheet)
          </span>
          <button class="btn btn-primary" style="flex:1;font-size:12px;padding:6px 10px"
                  :disabled="verifying.includes(keyOf(p))" @click="verifikasi(p)">
            {{ verifying.includes(keyOf(p)) ? 'Mengirim…' : 'Verifikasi' }}
          </button>
        </div>
      </div>
    </Card>

    <!-- audit trail buat cross-check laporan satpam — siapa lapor, berapa, kapan -->
    <Card v-if="tunai.length" style="gap:2px">
      <span class="kick">Riwayat kas masuk (tunai)</span>
      <div v-for="t in tunai" :key="t.timestamp + t.alamat + t.bulan" class="spread"
           style="padding:6px 0;border-top:1px solid var(--color-divider)">
        <div>
          <div style="font-size:12.5px;font-weight:600">{{ t.alamat }} · {{ BULAN[t.bulan - 1] }}</div>
          <div class="text-muted" style="font-size:11px">{{ t.petugas }} · {{ t.timestamp }}</div>
        </div>
        <span class="num" style="font-weight:700;font-size:12.5px">{{ rupiahPendek(t.nominal) }}</span>
      </div>
    </Card>

    <p class="text-muted" style="font-size:11.5px">
      Verifikasi bisa lewat tombol di atas, atau langsung centang kolom
      <code>terverifikasi</code> pada tab <code>Pembayaran</code> di Sheet — dua-duanya
      berujung sama. Status <em>Pending → Lunas</em> dihitung formula.
    </p>

    <a class="btn btn-secondary" href="#/kas/qr"
       style="justify-content:center;background:var(--color-surface);box-shadow:var(--shadow-sm)">
      Cetak QR per rumah
    </a>
  </section>
 </PinGate>
</template>
