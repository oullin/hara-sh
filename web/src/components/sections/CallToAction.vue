<script setup lang="ts">
import HaraMark from '@/components/dots/HaraMark.vue';
import { Button } from '@/components/ui/button';
import { DOCS_URL } from '@/lib/content';
</script>
<template>
	<section aria-labelledby="cta-title" class="closing-section">
		<span class="text-primary"><HaraMark :size="6" /></span>
		<div>
			<h2 id="cta-title">Find your centre.</h2>
			<p>Your accounts, on your infrastructure.</p>
		</div>
		<Button as="a" :href="`${DOCS_URL}setup`" size="lg" class="marketing-button primary-action">Set up Hara</Button>
	</section>
</template>
