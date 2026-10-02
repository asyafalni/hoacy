<script setup vapor>
// PIN screen for the petugas-only routes (Pos, Kas). The PIN is checked by the
// server (Apps Script — src/server/core.js), which also locks out repeated wrong
// guesses; nothing about it ships in this bundle. The accepted PIN is kept on
// this device (useAuth) and sent with every request, so what the server returns
// is what a wrong-PIN visitor can never see — unlike the old client-side gate.
import { ref } from 'vue';
import { api, ApiError } from '../lib/api';
import { useAuth } from '../composables/useAuth';
import Button from './ui/Button.vue';

const props = defineProps({
  role: { type: String, required: true },   // 'pos' | 'kas'
  title: { type: String, required: true },
});

const { pins, ditolak, setPin } = useAuth();
const input = ref('');
const pesan = ref('');
const memeriksa = ref(false);

const PESAN = {
  pin: 'PIN salah, coba lagi.',
  terkunci: 'Terlalu banyak percobaan salah — coba lagi dalam 15 menit.',
};

async function coba() {
  const pin = input.value.trim();
  if (!pin || memeriksa.value) return;
  memeriksa.value = true;
  pesan.value = '';
  try {
    await api('login', { role: props.role, pin });
    setPin(props.role, pin);   // unlocks the slot — the view loads its data from here
  } catch (e) {
    pesan.value = e instanceof ApiError
      ? (PESAN[e.kode] || e.message || 'Gagal memeriksa PIN.')
      : 'Tidak bisa menghubungi server — periksa koneksi.';
  } finally {
    memeriksa.value = false;
    input.value = '';
  }
}
</script>

<template>
  <section v-if="!pins[role]" class="scr col" style="gap:var(--space-3);align-items:center;
           justify-content:center;text-align:center">
    <div style="width:56px;height:56px;border-radius:50%;background:var(--color-neutral-200);
                display:flex;align-items:center;justify-content:center;font-size:22px">🔒</div>
    <h4 style="margin:0">{{ title }}</h4>
    <p class="text-muted" style="font-size:12.5px;margin:0">
      Khusus petugas (satpam / bendahara / admin / komite) — masukkan PIN akses.
    </p>
    <p v-if="ditolak === role && !pesan" class="text-muted" style="font-size:11.5px;margin:0">
      PIN yang tersimpan tidak lagi berlaku — masukkan PIN yang baru.
    </p>
    <input class="input" v-model="input" type="password" inputmode="numeric"
           placeholder="PIN" style="max-width:200px;text-align:center" @keyup.enter="coba">
    <Button :disabled="memeriksa" @click="coba">{{ memeriksa ? 'Memeriksa…' : 'Masuk' }}</Button>
    <p v-if="pesan" class="text-muted" style="font-size:11.5px;color:var(--color-accent-700)">{{ pesan }}</p>
  </section>
  <slot v-else />
</template>
