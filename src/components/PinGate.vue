<script setup vapor>
// Deterrent-only gate for petugas-only routes (Pos, Kas). This is a static site
// with no server — the "pin" prop is a Vite env var baked into the public JS
// bundle, so it is NOT a secret from anyone who opens devtools. It exists to
// keep warga from wandering into /pos or /kas by accident, paired with those
// routes being left out of the public nav (see App.vue). Do not rely on this
// for anything that actually needs to stay private.
import { ref } from 'vue';
import Button from './ui/Button.vue';

const props = defineProps({
  pin: { type: String, default: '' },
  storageKey: { type: String, required: true },
  title: { type: String, required: true },
  envVar: { type: String, required: true },
});

const key = `iuran.pin.${props.storageKey}`;
const unlocked = ref(localStorage.getItem(key) === '1');
const input = ref('');
const salah = ref(false);

function coba() {
  if (props.pin && input.value === props.pin) {
    localStorage.setItem(key, '1');
    unlocked.value = true;
    salah.value = false;
  } else {
    salah.value = true;
  }
  input.value = '';
}
</script>

<template>
  <section v-if="!unlocked" class="scr col" style="gap:var(--space-3);align-items:center;
           justify-content:center;text-align:center">
    <div style="width:56px;height:56px;border-radius:50%;background:var(--color-neutral-200);
                display:flex;align-items:center;justify-content:center;font-size:22px">🔒</div>
    <h4 style="margin:0">{{ title }}</h4>
    <p class="text-muted" style="font-size:12.5px;margin:0">
      Khusus petugas (satpam / bendahara / admin / komite) — masukkan PIN akses.
    </p>
    <template v-if="pin">
      <input class="input" v-model="input" type="password" inputmode="numeric"
             placeholder="PIN" style="max-width:200px;text-align:center" @keyup.enter="coba">
      <Button @click="coba">Masuk</Button>
      <p v-if="salah" class="text-muted" style="font-size:11.5px;color:var(--color-accent-700)">
        PIN salah, coba lagi.
      </p>
    </template>
    <p v-else class="text-muted" style="font-size:11.5px">
      PIN belum dikonfigurasi — isi <code>{{ envVar }}</code> di <code>.env</code>.
    </p>
  </section>
  <slot v-else />
</template>
