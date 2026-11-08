<script setup vapor>
// Sign-in for the pengurus-only routes (/kas…). Production signs in with Clerk
// (invitation only — the admin adds people in Kas → Admin → Pengurus); the
// server then checks the session and the pengurus table, so what renders in
// the slot is only what the server agreed to send. Local development (no
// Clerk key) asks for an email instead, which only a development backend accepts.
import { ref, onMounted, nextTick, watch } from 'vue';
import { modeDev, muatClerk, emailDev, setEmailDev, keluarPengurus } from '../lib/pengurus';
import { useSaya } from '../composables/useKas';
import Button from './ui/Button.vue';

const { saya, status, cekSaya } = useSaya();
const kotakMasuk = ref(null);
const inputEmail = ref(emailDev.value);
let clerk = null;

async function tampilkanMasuk() {
  if (modeDev) return;
  await nextTick();
  if (clerk && kotakMasuk.value) clerk.mountSignIn(kotakMasuk.value, { signUpUrl: undefined });
}

onMounted(async () => {
  if (modeDev) { cekSaya(); return; }
  clerk = await muatClerk();
  clerk.addListener(({ session }) => {
    if (session && status.value !== 'masuk') cekSaya();
    if (!session) { saya.value = null; status.value = 'perlu-masuk'; tampilkanMasuk(); }
  });
  if (clerk.session) cekSaya();
  else { status.value = 'perlu-masuk'; tampilkanMasuk(); }
});

watch(status, (s) => { if (s === 'perlu-masuk') tampilkanMasuk(); });

function masukDev() {
  setEmailDev(inputEmail.value.trim().toLowerCase());
  cekSaya();
}
async function keluar() {
  await keluarPengurus();
  saya.value = null;
  status.value = 'perlu-masuk';
}
</script>

<template>
  <slot v-if="status === 'masuk' && saya" />

  <section v-else class="scr col" style="gap:var(--space-3);align-items:center;text-align:center;
           padding-top:var(--space-6)">
    <div style="width:56px;height:56px;border-radius:50%;background:var(--color-neutral-200);
                display:flex;align-items:center;justify-content:center;font-size:22px">🔒</div>
    <h4 style="margin:0">Kas Pengurus</h4>

    <p v-if="status === 'memuat'" class="text-muted" style="font-size:12.5px;margin:0">Memeriksa akun…</p>

    <template v-else-if="status === 'bukan-pengurus'">
      <p style="font-size:12.5px;margin:0;color:var(--color-accent-700);max-width:300px">
        Akun ini belum terdaftar sebagai pengurus. Minta admin menambahkan email Anda di
        Kas → Admin → Pengurus.
      </p>
      <Button variant="secondary" @click="keluar">Keluar &amp; ganti akun</Button>
    </template>

    <template v-else>
      <p class="text-muted" style="font-size:12.5px;margin:0;max-width:300px">
        Khusus pengurus (bendahara / admin / komite). Masuk dengan akun Google yang sudah didaftarkan admin.
      </p>
      <!-- Clerk renders its sign-in form here -->
      <div v-if="!modeDev" ref="kotakMasuk"></div>
      <template v-else>
        <p class="text-muted" style="font-size:11px;margin:0">Mode pengembangan — tanpa Clerk.</p>
        <input class="input" v-model="inputEmail" type="email" placeholder="email pengurus"
               style="max-width:260px" @keyup.enter="masukDev">
        <Button @click="masukDev">Masuk</Button>
      </template>
    </template>
  </section>
</template>
