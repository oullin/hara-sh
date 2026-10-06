<script setup lang="ts">
import DotGrid from '@/components/dots/DotGrid.vue';
import { providers } from '@/lib/content';
import { glyphDots } from '@/lib/dots';

const items = providers.map((provider) => ({ ...provider, dots: glyphDots(provider.glyph) }));
</script>

<template>
	<section id="providers" aria-labelledby="providers-title" class="page-container py-28">
		<div class="mb-14 flex flex-col gap-4">
			<p class="font-mono text-xs tracking-[0.2em] text-primary">PROVIDERS</p>
			<h2 id="providers-title" class="max-w-2xl text-4xl leading-tight font-medium tracking-[-0.035em] sm:text-5xl">Every provider, one endpoint</h2>
			<p class="max-w-xl text-lg text-muted-foreground">Sign in with OAuth or add an API key, and mix as many accounts per provider as you have.</p>
		</div>
		<ul class="grid gap-x-10 gap-y-10 sm:grid-cols-2 lg:grid-cols-5">
			<li v-for="item in items" :key="item.name" class="flex items-center gap-4">
				<span class="text-primary"><DotGrid :dots="item.dots" :size="5" /></span>
				<span class="flex flex-col">
					<span class="font-medium tracking-tight">{{ item.name }}</span>
					<span class="font-mono text-xs text-muted-foreground">{{ item.via }}</span>
				</span>
			</li>
		</ul>
	</section>
</template>
