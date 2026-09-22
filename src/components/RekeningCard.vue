<script setup vapor>
// BRI's own brand blue (#0857c3, pulled straight from the official 2025 logo file at
// src/assets/logos/bri.svg) — deliberately not the app's Organic accent color, since
// this card represents the destination bank, not the app itself.
import { ref } from 'vue';
import { REKENING } from '../lib/tariff';
import briLogo from '../assets/logos/bri.svg';

const nomorRapi = REKENING.nomor.replace(/(\d{4})(?=\d)/g, '$1 ');
const copied = ref(false);

async function salin() {
  try {
    await navigator.clipboard.writeText(REKENING.nomor);
  } catch {
    // clipboard API needs a secure context — plain-textarea fallback for anything else
    const el = document.createElement('textarea');
    el.value = REKENING.nomor;
    el.style.position = 'fixed';
    el.style.opacity = '0';
    document.body.appendChild(el);
    el.select();
    document.execCommand('copy');
    el.remove();
  }
  copied.value = true;
  setTimeout(() => { copied.value = false; }, 1800);
}
</script>

<template>
  <div style="position:relative;overflow:hidden;border-radius:var(--radius-lg);
              padding:var(--space-4);background:linear-gradient(135deg,#0d6dea,#04358a);
              box-shadow:var(--shadow-md);color:#fff">
    <div style="position:absolute;right:-34px;top:-44px;width:140px;height:140px;
                border-radius:50%;background:rgba(255,255,255,.09)"></div>
    <div style="position:absolute;right:40px;bottom:-58px;width:92px;height:92px;
                border-radius:50%;background:rgba(255,255,255,.07)"></div>

    <div class="row" style="position:relative;justify-content:space-between;align-items:flex-start">
      <div>
        <div style="font-size:9.5px;letter-spacing:.12em;text-transform:uppercase;opacity:.8">
          Rekening Tujuan
        </div>
        <div style="font-size:13px;font-weight:700;margin-top:2px">{{ REKENING.bank }}</div>
      </div>
      <div style="background:#fff;border-radius:10px;padding:6px 11px;line-height:0">
        <img :src="briLogo" alt="BRI" style="height:16px;width:auto;display:block">
      </div>
    </div>

    <button type="button" @click="salin"
            style="position:relative;display:flex;align-items:center;gap:var(--space-2);
                   width:100%;margin-top:var(--space-4);padding:0;background:none;border:none;
                   cursor:pointer;text-align:left;color:inherit">
      <span class="num" style="font-size:22px;font-weight:700;letter-spacing:.03em;flex:1">
        {{ nomorRapi }}
      </span>
      <span style="display:flex;align-items:center;gap:5px;font-size:11.5px;font-weight:600;
                   background:rgba(255,255,255,.18);padding:7px 12px;border-radius:999px;flex:none">
        <template v-if="copied">✓ Disalin</template>
        <template v-else>
          <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor"
               stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round">
            <rect x="9" y="9" width="12" height="12" rx="2"></rect>
            <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path>
          </svg>
          Salin
        </template>
      </span>
    </button>

    <div style="position:relative;margin-top:var(--space-2);font-size:13px;opacity:.92">
      a.n. {{ REKENING.nama }}
    </div>
  </div>
</template>
