import { ref } from 'vue';

// Shared hover/tap-tooltip state for the hand-rolled SVG/CSS charts on
// RingkasanPublik.vue (aging pies, riwayat bars) — position is relative to
// `wrapEl` (a template ref on the chart's own wrapper), not the page, so it
// keeps working regardless of scroll position.
export function useChartTooltip() {
  const wrapEl = ref(null);
  const tip = ref(null);   // { x, y, ...whatever data the caller passes in }

  function show(e, data) {
    const rect = wrapEl.value.getBoundingClientRect();
    const point = e.touches ? e.touches[0] : e;
    tip.value = { x: point.clientX - rect.left, y: point.clientY - rect.top, ...data };
  }
  function hide() { tip.value = null; }

  return { wrapEl, tip, show, hide };
}
