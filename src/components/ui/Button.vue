<script setup vapor>
// shadcn-style primitive: variant + size props, styling from the Organic .btn classes.
const props = defineProps({
  variant: { type: String, default: 'primary' },   // primary | secondary | ghost
  block:   { type: Boolean, default: false },
  as:      { type: String, default: 'button' },
});
</script>

<template>
  <component
    :is="as"
    class="btn"
    :class="[`btn-${props.variant}`, { 'btn-block': props.block }]"
    style="min-height:44px">
    <slot />
  </component>
</template>
