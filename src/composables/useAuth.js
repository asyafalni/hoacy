import { ref } from 'vue';

// PINs are checked by the server (Apps Script), never in the browser. Once the
// server accepts one, this device remembers it and sends it with every call —
// until the server rejects it (changed/revoked), which clears it again and sends
// the user back to the PIN screen. Per-device only; "Ganti rumah" forgets a
// resident's.
const BASE = 'iuran.akses.';
const baca = (k) => { try { return localStorage.getItem(BASE + k) || ''; } catch { return ''; } };
const tulis = (k, v) => {
  try { v ? localStorage.setItem(BASE + k, v) : localStorage.removeItem(BASE + k); } catch { /* private mode */ }
};

const pins = ref({ pos: baca('pos'), kas: baca('kas') });
/** A role whose remembered PIN the server just refused ("PIN diganti"). */
const ditolak = ref('');

export function useAuth() {
  return {
    pins, ditolak,
    setPin(role, pin) { pins.value = { ...pins.value, [role]: pin }; tulis(role, pin); ditolak.value = ''; },
    clearPin(role, karenaServer = false) {
      pins.value = { ...pins.value, [role]: '' };
      tulis(role, '');
      if (karenaServer) ditolak.value = role;
    },
    // one remembered PIN per house on this device
    pinWarga: (alamat) => baca('warga.' + alamat),
    setPinWarga: (alamat, pin) => tulis('warga.' + alamat, pin),
    lupaWarga: (alamat) => tulis('warga.' + alamat, ''),
  };
}
