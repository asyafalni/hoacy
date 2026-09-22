<script setup vapor>
import { computed } from 'vue';
import { useSheet } from '../composables/useSheet';
import { rupiah, rupiahPendek, REKENING } from '../lib/tariff';
import { urlSetoran, batchId } from '../lib/forms';
import Card from '../components/ui/Card.vue';
import Button from '../components/ui/Button.vue';
import PinGate from '../components/PinGate.vue';

const { meta, rumah } = useSheet();
const PIN = import.meta.env.VITE_PIN_KAS || '';

const kas   = computed(() => Number(meta.value.kas_tunai || 0));
const bank  = computed(() => Number(meta.value.rekening || 0));
const lunas = computed(() => Number(meta.value.lunas_bulan_ini || 0));
const total = computed(() => Number(meta.value.jumlah_rumah || rumah.value.length || 0));
const persen = computed(() => (total.value ? Math.round((lunas.value / total.value) * 100) : 0));

// Setor ke Bank = one Setoran row; then paste the batch id into Pembayaran!K
// for the rows currently marked "kas" (docs/sheets-schema.md §4).
const setorUrl = computed(() =>
  urlSetoran({ batchId: batchId(), nominal: kas.value, oleh: 'Bendahara' }));
</script>

<template>
 <PinGate :pin="PIN" storage-key="kas" title="Kas Bendahara" env-var="VITE_PIN_KAS">
  <section class="scr col" style="gap:var(--space-3)">
    <div>
      <h4 style="margin:0">Kas RT 03/14</h4>
      <div class="text-muted" style="font-size:11.5px">
        {{ total }} rumah · sinkron {{ meta.updated || '—' }}
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
          {{ rupiahPendek(meta.tunggakan_total || 0) }}
        </span>
      </div>
    </Card>

    <p class="text-muted" style="font-size:11.5px">
      Verifikasi transfer dilakukan di Sheet: centang kolom <code>terverifikasi</code>
      pada tab <code>Pembayaran</code>. Status <em>Pending → Lunas</em> dihitung formula.
    </p>
  </section>
 </PinGate>
</template>
