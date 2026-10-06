<script setup lang="ts">
import DotGrid from '@/components/dots/DotGrid.vue';
import { ringField } from '@/lib/dots';

// The hara mark: rings pulse inward to one glowing center (腹, the center).
withDefaults(
	defineProps<{ size?: number }>(),
	{ size: 4 },
);

const dots = ringField(5);
</script>

<template>
	<DotGrid :dots="dots" :size="size" />
</template>
