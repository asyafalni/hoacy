import { watch } from 'vue';

// `.dialog-backdrop` is `position:fixed; inset:0`, which only LOOKS like it
// blocks the page underneath — the body itself is still a normal scrollable
// element, so a wheel/touch gesture can scroll the page behind the sheet at
// the same time as the sheet's own internal overflow, which feels exactly
// like "the scroll is broken" once the page behind it has enough content to
// actually move (see the Bendahara "Riwayat Kas Masuk" bug this fixed).
// Counter-based so nested/sequential dialogs don't unlock each other early.
let lockCount = 0;
function lock() {
  lockCount += 1;
  if (lockCount === 1) document.body.style.overflow = 'hidden';
}
function unlock() {
  lockCount = Math.max(0, lockCount - 1);
  if (lockCount === 0) document.body.style.overflow = '';
}

/** Call with the ref/computed that controls a dialog or bottom-sheet's
 *  visibility (`showX`, or a `v-if="sel"` selection ref) — locks page scroll
 *  for as long as it's truthy. */
export function useScrollLock(isOpenRef) {
  watch(isOpenRef, (open) => { open ? lock() : unlock(); }, { immediate: true });
}
