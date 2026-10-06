<script setup lang="ts">
import type { Dot } from '@/lib/dots';

withDefaults(
	defineProps<{ dots: Array<Dot>; size?: number }>(),
	{ size: 6 },
);
</script>

<template>
	<span aria-hidden="true" class="grid grid-cols-5" :style="{ gap: `${size - 1}px`, width: `${size * 5 + (size - 1) * 4}px` }">
		<span v-for="(dot, index) in dots" :key="index" :class="dot.class" :style="{ animationDelay: `${dot.delay}ms`, height: `${size}px`, width: `${size}px` }" />
	</span>
</template>
