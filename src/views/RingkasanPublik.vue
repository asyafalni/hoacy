<script setup vapor>
import { computed, onMounted } from 'vue';
import { useSheet } from '../composables/useSheet';
import { rupiah, rupiahPendek } from '../lib/tariff';
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
const { meta, rumah, load } = useSheet();
onMounted(load);

const kas = computed(() => Number(meta.value.kas_tunai || 0));
const bank = computed(() => Number(meta.value.rekening || 0));
const tunggakan = computed(() => Number(meta.value.tunggakan_total || 0));
const jumlahRumah = computed(() => Number(meta.value.jumlah_rumah || rumah.value.length || 0));

const terkumpul = computed(() => Number(meta.value.terkumpul_bulan_ini || 0));
const target = computed(() => Number(meta.value.target_bulan_ini || 0));
const persenTarget = computed(() => (target.value ? Math.min(100, Math.round((terkumpul.value / target.value) * 100)) : 0));

const opex = computed(() => Number(meta.value.opex_bulanan || 0));
const proyeksi = computed(() => terkumpul.value - opex.value);
const opexBelumDiisi = computed(() => !opex.value);
</script>

<template>
  <section class="scr col" style="gap:var(--space-3)">
    <div>
      <h4 style="margin:0">Ringkasan Kas Cluster</h4>
      <div class="text-muted" style="font-size:11.5px">
        {{ jumlahRumah }} rumah · terbuka untuk siapa saja, tanpa data per-rumah
      </div>
    </div>

    <Card elev="md" style="background:var(--color-neutral-900);color:var(--color-neutral-100)">
      <span class="kick" style="color:var(--color-accent-300)">Terkumpul bulan ini</span>
      <div class="num" style="font-family:var(--font-heading);font-size:32px;line-height:1.1">
        {{ rupiah(terkumpul) }}
      </div>
      <div style="height:10px;border-radius:999px;background:var(--color-neutral-700);overflow:hidden">
        <div :style="{ width: persenTarget + '%', height: '100%', background: 'var(--color-accent-2-500)' }"></div>
      </div>
      <div class="spread" style="font-size:11.5px;color:var(--color-neutral-400)">
        <span>{{ persenTarget }}% dari target</span>
        <span>Target: {{ rupiah(target) }}</span>
      </div>
    </Card>

    <Card v-if="!opexBelumDiisi">
      <span class="kick">Proyeksi vs OPEX minimum</span>
      <div class="spread" style="align-items:flex-end">
        <span class="num" style="font-family:var(--font-heading);font-size:26px"
              :style="{ color: proyeksi >= 0 ? 'var(--color-accent-2-700)' : 'var(--color-accent-700)' }">
          {{ proyeksi >= 0 ? 'Surplus' : 'Defisit' }} {{ rupiah(Math.abs(proyeksi)) }}
        </span>
      </div>
      <p class="text-muted" style="font-size:11.5px;margin:0">
        Terkumpul {{ rupiahPendek(terkumpul) }} − OPEX minimum {{ rupiahPendek(opex) }}/bulan.
        Ini sinyal buat diskusi apakah tarif perlu disesuaikan — bukan keputusan otomatis.
      </p>
    </Card>
    <Card v-else>
      <span class="kick">Proyeksi vs OPEX minimum</span>
      <p class="text-muted" style="font-size:12px;margin:0">
        Bendahara belum mengisi OPEX bulanan minimum di Sheet — proyeksi surplus/defisit
        belum bisa dihitung.
      </p>
    </Card>

    <Card>
      <div class="spread" style="font-size:12.5px">
        <span class="text-muted">Kas Tunai di pos</span>
        <span class="num" style="font-weight:700">{{ rupiah(kas) }}</span>
      </div>
      <div class="spread" style="font-size:12.5px">
        <span class="text-muted">Rekening bank</span>
        <span class="num" style="font-weight:700">{{ rupiah(bank) }}</span>
      </div>
      <div class="spread" style="font-size:12.5px">
        <span class="text-muted">Tunggakan seluruh cluster</span>
        <span class="num" style="font-weight:700;color:var(--color-accent-700)">{{ rupiah(tunggakan) }}</span>
      </div>
    </Card>

    <p class="text-muted" style="font-size:10.5px;text-align:center">
      Halaman ini cuma nampilin angka gabungan seluruh cluster — tidak ada nama,
      alamat, atau status bayar rumah tertentu.
    </p>
  </section>
</template>
