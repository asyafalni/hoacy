<script setup vapor>
defineProps({ elev: { type: String, default: 'sm' } });   // sm | md | lg
</script>

<template>
  <div class="card" :class="`elev-${elev}`"><slot /></div>
</template>
