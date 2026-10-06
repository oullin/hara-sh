<script setup lang="ts">
import DotGrid from '@/components/dots/DotGrid.vue';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { providers } from '@/lib/content';
import { glyphDots } from '@/lib/dots';

const cards = providers.map((provider) => ({ ...provider, dots: glyphDots(provider.glyph) }));
</script>

<template>
	<section id="providers" aria-labelledby="providers-title" class="page-container flex flex-col gap-10 pt-28 pb-14">
		<div class="flex flex-wrap items-end justify-between gap-5">
			<h2 id="providers-title" class="max-w-lg font-display text-4xl leading-tight font-extrabold sm:text-5xl">Every provider. One endpoint.</h2>
			<p class="max-w-sm text-muted-foreground">Sign in with OAuth or add an API key. Mix as many accounts per provider as you have.</p>
		</div>
		<ul class="grid grid-cols-[repeat(auto-fill,minmax(13rem,1fr))] gap-3.5">
			<li v-for="card in cards" :key="card.name">
				<Card class="h-full gap-5">
					<CardContent class="text-primary">
						<DotGrid :dots="card.dots" :size="6" />
					</CardContent>
					<CardHeader>
						<CardTitle>{{ card.name }}</CardTitle>
						<CardDescription class="font-mono text-xs">{{ card.via }}</CardDescription>
					</CardHeader>
				</Card>
			</li>
		</ul>
	</section>
</template>
