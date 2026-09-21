<script setup vapor>
import { ref, computed } from 'vue';
import { useSheet } from '../composables/useSheet';
import { BULAN, rupiah, rupiahPendek } from '../lib/tariff';
import { urlPembayaran } from '../lib/forms';
import Card from '../components/ui/Card.vue';
import Tag from '../components/ui/Tag.vue';
import Button from '../components/ui/Button.vue';

const { rumah } = useSheet();

// No login: blok is fixed, resident types the house number. Persisted so the card
// opens straight away next time (and a per-house QR link can set it via ?rumah=).
const saved = new URLSearchParams(location.search).get('rumah')
           || localStorage.getItem('iuran.rumah') || '';
const noRumah = ref(saved);
const input = ref('');

const me = computed(() => rumah.value.find((h) => h.no === noRumah.value));

function open() {
  const n = input.value.replace(/\D/g, '');
  if (!n) return;
  const id = 'N-' + n.padStart(2, '0');
  if (!rumah.value.some((h) => h.no === id)) { notFound.value = true; return; }
  noRumah.value = id;
  localStorage.setItem('iuran.rumah', id);
}
const notFound = ref(false);
function ganti() { noRumah.value = ''; localStorage.removeItem('iuran.rumah'); }

const cls = (s) => ({ Lunas: 'lunas', Sebagian: 'sebagian', Pending: 'pending', Belum: 'belum' }[s] || 'kosong');

// first unpaid month -> prefilled transfer form
const bayarUrl = computed(() => {
  if (!me.value) return '#';
  const i = me.value.status.findIndex((s) => s === 'Belum' || s === 'Sebagian');
  return urlPembayaran({
    noRumah: me.value.no, bulan: (i < 0 ? 0 : i) + 1,
    nominal: me.value.tarif, metode: 'transfer', petugas: 'Warga',
  });
});
</script>

<template>
  <!-- masuk: blok + nomor rumah, tanpa akun -->
  <section v-if="!me" class="scr col" style="gap:var(--space-3)">
    <div style="width:74px;height:74px;border-radius:50%;background:var(--color-accent-2-200);
                color:var(--color-accent-2-800);display:flex;align-items:center;
                justify-content:center;font-family:var(--font-heading);font-size:28px">N</div>
    <h3 style="margin:0">Buka kartu iuran</h3>
    <p class="text-muted" style="font-size:13px;margin:0">
      Tidak perlu akun — masukkan blok dan nomor rumah Anda.<br>
      <span style="opacity:.75">No login — just block and house number.</span>
    </p>
    <div class="row" style="align-items:flex-end">
      <div class="field" style="width:96px">
        <label>Blok</label>
        <input class="input" value="N" readonly style="text-align:center">
      </div>
      <div class="field grow">
        <label>Nomor rumah</label>
        <input class="input" v-model="input" inputmode="numeric" placeholder="mis. 03" @keyup.enter="open">
      </div>
    </div>
    <Button block @click="open">Lihat kartu saya</Button>
    <p v-if="notFound" class="text-muted" style="font-size:11.5px">Nomor rumah tidak ditemukan di Blok N.</p>
  </section>

  <!-- kartu -->
  <section v-else class="scr col" style="gap:var(--space-3)">
    <div class="spread">
      <span class="kick">Kartu saya</span>
      <button class="btn btn-ghost" style="font-size:12px" @click="ganti">Ganti rumah</button>
    </div>

    <div style="border-radius:calc(var(--radius-lg)*1.15);overflow:hidden;
                background:var(--color-surface);box-shadow:var(--shadow-md)">
      <div style="background:var(--color-accent);padding:var(--space-4);position:relative;overflow:hidden">
        <div style="position:absolute;right:-36px;top:-46px;width:150px;height:150px;border-radius:50%;
                    background:var(--color-accent-2-400);opacity:.55"></div>
        <div style="position:relative;font-family:var(--font-heading);font-size:21px;color:var(--color-bg)">
          {{ me.nama }}
        </div>
        <div class="row" style="position:relative;gap:var(--space-6);margin-top:var(--space-2)">
          <div>
            <div style="font-size:9.5px;letter-spacing:.1em;text-transform:uppercase;color:var(--color-accent-200)">Blok</div>
            <div class="num" style="font-size:13.5px;font-weight:700;color:var(--color-bg)">{{ me.no }} / RT 03</div>
          </div>
          <div>
            <div style="font-size:9.5px;letter-spacing:.1em;text-transform:uppercase;color:var(--color-accent-200)">Luas / Tarif</div>
            <div class="num" style="font-size:13.5px;font-weight:700;color:var(--color-bg)">
              {{ me.luas }} m² · {{ rupiahPendek(me.tarif) }}/bln
            </div>
          </div>
        </div>
      </div>

      <div style="padding:var(--space-4)">
        <div class="spread" style="margin-bottom:var(--space-2)">
          <span class="kick">Tahun 2026</span>
          <Tag :status="me.tunggakan > 0 ? 'Sebagian' : 'Lunas'">
            {{ me.tunggakan > 0 ? rupiahPendek(me.tunggakan) + ' belum dibayar' : 'Lunas' }}
          </Tag>
        </div>
        <div class="months">
          <div v-for="(s, i) in me.status" :key="i" class="month" :class="cls(s)">
            <div style="font-size:12.5px;font-weight:700">{{ BULAN[i] }}</div>
            <div class="num" style="font-size:10px;opacity:.8">{{ s === '-' ? '—' : s }}</div>
          </div>
        </div>
      </div>
    </div>

    <Card>
      <div class="spread">
        <span style="font-size:13.5px;font-weight:700">Tagihan berjalan</span>
        <span class="num" style="font-family:var(--font-heading);font-size:22px;color:var(--color-accent-700)">
          {{ rupiah(me.tunggakan) }}
        </span>
      </div>
      <p class="text-muted" style="font-size:11.5px;margin:0">
        Tarif {{ rupiah(me.tarif) }}/bulan (ISLK per luas tanah + Iuran RT Rp 50.000).
      </p>
      <Button as="a" :href="bayarUrl" target="_blank" block>Bayar transfer</Button>
      <div class="text-muted num" style="text-align:center;font-size:10.5px">
        BCA 7290-xxx a.n. Kas RT 03/14 · jatuh tempo tgl 20
      </div>
    </Card>
  </section>
</template>
