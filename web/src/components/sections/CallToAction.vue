<script setup lang="ts">
import HaraMark from '@/components/dots/HaraMark.vue';
import { Button } from '@/components/ui/button';
import { DOCS_URL } from '@/lib/content';
</script>

<template>
	<section aria-labelledby="cta-title" class="page-container flex flex-col items-start gap-8 py-32">
		<span class="text-primary"><HaraMark :size="7" /></span>
		<h2 id="cta-title" class="max-w-3xl text-5xl leading-[1.02] font-medium tracking-[-0.045em] sm:text-6xl">Find your center</h2>
		<p class="max-w-lg text-lg text-muted-foreground">Connect an account, point a client, send a request. A few minutes, start to finish.</p>
		<div class="flex flex-wrap gap-3">
			<Button as="a" href="#how" size="lg" class="h-11 rounded-full bg-foreground px-5 text-[15px] text-background hover:bg-foreground/90">Get started</Button>
			<Button as="a" :href="DOCS_URL" variant="outline" size="lg" class="h-11 rounded-full px-5 text-[15px]">Read the CLIProxyAPI docs</Button>
		</div>
	</section>
</template>
