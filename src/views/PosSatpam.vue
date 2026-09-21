<script setup vapor>
import { ref, computed } from 'vue';
import { useSheet } from '../composables/useSheet';
import { BULAN, rupiah, rupiahPendek } from '../lib/tariff';
import { urlPembayaran, submitPembayaran } from '../lib/forms';
import Card from '../components/ui/Card.vue';
import Tag from '../components/ui/Tag.vue';
import Button from '../components/ui/Button.vue';

const { rumah, echo, load } = useSheet();
const PETUGAS = 'Pos Satpam - Ujang';

const q = ref('');
const sel = ref(null);        // selected house
const bulan = ref([]);        // month indices being paid
const separuh = ref(false);   // partial: 50%
const toast = ref('');

// arrears first — the whole point of the post screen is speed
const daftar = computed(() => rumah.value
  .filter((h) => !q.value || h.nama.toLowerCase().includes(q.value.toLowerCase()) || h.no.includes(q.value))
  .slice().sort((a, b) => b.tunggakan - a.tunggakan));

const belum = (h) => h.status
  .map((s, i) => ({ s, i }))
  .filter((x) => x.s === 'Belum' || x.s === 'Sebagian')
  .map((x) => x.i);

function pilih(h) { sel.value = h; bulan.value = belum(h).slice(0, 2); separuh.value = false; }
function toggle(i) {
  const a = bulan.value;
  bulan.value = a.includes(i) ? a.filter((x) => x !== i) : [...a, i].sort((x, y) => x - y);
}

const total = computed(() => {
  if (!sel.value) return 0;
  const raw = bulan.value.length * sel.value.tarif * (separuh.value ? 0.5 : 1);
  return Math.round(raw / 1000) * 1000;
});

async function catat() {
  if (!sel.value || !bulan.value.length) return;
  const per = Math.round(total.value / bulan.value.length / 1000) * 1000;
  for (const i of bulan.value) {
    const rec = { noRumah: sel.value.no, bulan: i + 1, nominal: per,
                  metode: 'tunai', petugas: PETUGAS };
    echo(rec);
    await submitPembayaran(rec);       // opaque; the echo is what the satpam sees
  }
  toast.value = `${sel.value.nama} · ${bulan.value.length} bulan tunai tercatat. Masuk Kas Tunai pos.`;
  sel.value = null; bulan.value = [];
  setTimeout(load, 4000);              // sheet cache settles, then reconcile
  setTimeout(() => (toast.value = ''), 4000);
}

// fallback when the silent POST is unreliable: open the prefilled form
const formUrl = computed(() => sel.value && bulan.value.length
  ? urlPembayaran({ noRumah: sel.value.no, bulan: bulan.value[0] + 1,
                    nominal: Math.round(total.value / bulan.value.length),
                    metode: 'tunai', petugas: PETUGAS })
  : '#');
</script>

<template>
  <section class="scr col" style="gap:var(--space-3)">
    <div class="spread">
      <div>
        <h4 style="margin:0">Terima Iuran</h4>
        <div class="text-muted" style="font-size:11.5px">Pos Cypress · {{ PETUGAS }}</div>
      </div>
    </div>

    <input class="input" v-model="q" placeholder="Cari nomor rumah atau nama">

    <div class="col">
      <Card v-for="h in daftar" :key="h.no" style="flex-direction:row;align-items:center;
            gap:var(--space-3);cursor:pointer" @click="pilih(h)">
        <div class="num" style="width:46px;height:46px;flex:none;border-radius:50%;
             display:flex;align-items:center;justify-content:center;font-family:var(--font-heading);
             background:var(--color-neutral-200)">{{ h.no.replace('N-','') }}</div>
        <div class="grow">
          <div class="truncate" style="font-size:14px;font-weight:700">{{ h.nama }}</div>
          <div class="text-muted" style="font-size:11.5px">
            {{ h.tunggakan ? 'Belum: ' + belum(h).map(i => BULAN[i].slice(0,3)).join(', ') : 'Lunas' }}
          </div>
        </div>
        <div class="col" style="align-items:flex-end;gap:5px">
          <Tag :status="h.tunggakan ? 'Belum' : 'Lunas'">
            {{ h.tunggakan ? belum(h).length + ' bln' : 'Lunas' }}
          </Tag>
          <span class="num text-muted" style="font-size:11.5px">
            {{ h.tunggakan ? rupiahPendek(h.tunggakan) : '—' }}
          </span>
        </div>
      </Card>
    </div>

    <!-- sheet: pilih bulan, penuh / sebagian, catat -->
    <div v-if="sel" class="dialog-backdrop" @click.self="sel = null">
      <div class="dialog">
        <div class="spread">
          <div>
            <div class="dialog-title">{{ sel.no }} · {{ sel.nama }}</div>
            <div class="text-muted" style="font-size:12px">
              {{ sel.luas }} m² · tarif {{ rupiah(sel.tarif) }}/bulan
            </div>
          </div>
        </div>

        <div class="kick">Bulan yang dibayar</div>
        <div class="row" style="flex-wrap:wrap;gap:var(--space-2)">
          <button v-for="i in belum(sel)" :key="i" class="btn"
                  :class="bulan.includes(i) ? 'btn-primary' : 'btn-secondary'"
                  @click="toggle(i)">{{ BULAN[i] }}</button>
        </div>

        <div class="kick">Nominal diterima</div>
        <div class="row">
          <Button class="grow" :variant="separuh ? 'secondary' : 'primary'" @click="separuh = false">Penuh</Button>
          <Button class="grow" :variant="separuh ? 'primary' : 'secondary'" @click="separuh = true">Sebagian (50%)</Button>
        </div>

        <div class="spread" style="background:var(--color-bg);border-radius:var(--radius-md);
             padding:var(--space-3) var(--space-4)">
          <span style="font-size:13.5px;font-weight:700">Diterima tunai</span>
          <span class="num" style="font-family:var(--font-heading);font-size:21px;color:var(--color-accent-700)">
            {{ rupiah(total) }}
          </span>
        </div>

        <Button block @click="catat">Catat &amp; Kirim ke Sheet</Button>
        <a class="btn btn-ghost" :href="formUrl" target="_blank"
           style="justify-content:center;font-size:11.5px">buka Google Form (jika jaringan buruk)</a>
      </div>
    </div>

    <div v-if="toast" class="card elev-lg"
         style="position:fixed;left:50%;transform:translateX(-50%);bottom:96px;
                width:min(432px,calc(100% - 32px));background:var(--color-neutral-900);
                color:var(--color-neutral-100);font-size:12.5px">{{ toast }}</div>
  </section>
</template>
