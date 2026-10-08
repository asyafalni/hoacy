import { watch, onScopeDispose } from 'vue';

// `.dialog-backdrop` is `position:fixed; inset:0`, which only LOOKS like it
// blocks the page underneath — the body itself is still a normal scrollable
// element, so a wheel/touch gesture can scroll the page behind the sheet at
// the same time as the sheet's own internal overflow, which feels exactly
// like "the scroll is broken" once the page behind it has enough content to
// actually move (see the Bendahara "Riwayat Kas Masuk" bug this fixed).
// Counter-based so nested/sequential dialogs don't unlock each other early.
let lockCount = 0;
function apply() { document.body.style.overflow = lockCount > 0 ? 'hidden' : ''; }

/** Call with the ref/computed that controls a dialog or bottom-sheet's
 *  visibility (`showX`, or a `v-if="sel"` selection ref) — locks page scroll
 *  for as long as it's truthy. Each caller holds at most one lock (a truthy →
 *  truthy change, e.g. picking another house, doesn't stack a second one),
 *  and releases it when its component unmounts even if the sheet was open. */
export function useScrollLock(isOpenRef) {
  let held = false;
  const set = (want) => {
    if (want === held) return;
    held = want;
    lockCount += want ? 1 : -1;
    apply();
  };
  watch(isOpenRef, (open) => set(!!open), { immediate: true });
  onScopeDispose(() => set(false));
}
