<script setup vapor>
// status -> Organic tag variant
const props = defineProps({ status: { type: String, default: 'neutral' } });
const map = {
  Lunas: 'tag-accent-2', Tunggak: 'tag-accent',
  Pending: 'tag-neutral', Belum: 'tag-outline', neutral: 'tag-neutral',
};
</script>

<template>
  <span class="tag" :class="map[props.status] || 'tag-neutral'"><slot>{{ props.status }}</slot></span>
</template>
