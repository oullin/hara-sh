<script setup lang="ts">
import HaraMark from '@/components/dots/HaraMark.vue';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent } from '@/components/ui/card';
import { clients, features, guarantees } from '@/lib/content';
</script>

<template>
	<section id="routing" aria-labelledby="routing-title" class="page-container flex flex-col gap-10 py-16">
		<h2 id="routing-title" class="font-display text-4xl leading-tight font-extrabold sm:text-5xl">Requests in. Accounts out.</h2>
		<Card>
			<CardContent class="flex flex-wrap items-center justify-center gap-6 py-6 text-primary">
				<ul class="flex flex-[0_1_14rem] flex-col gap-2.5" aria-label="Clients">
					<li v-for="client in clients" :key="client">
						<Badge variant="outline" class="h-10 w-full justify-start px-3.5 font-mono text-sm text-foreground">{{ client }}</Badge>
					</li>
				</ul>
				<div aria-hidden="true" class="relative h-1.5 min-w-20 flex-[1_1_7rem]">
					<span class="mover" />
					<span class="mover [animation-delay:800ms]" />
					<span class="mover [animation-delay:1600ms]" />
				</div>
				<div class="flex size-33 flex-col items-center justify-center gap-2 rounded-3xl bg-primary text-primary-foreground">
					<HaraMark :size="6" />
					<span class="font-display text-xl font-extrabold">hara</span>
				</div>
				<div aria-hidden="true" class="relative h-1.5 min-w-20 flex-[1_1_7rem]">
					<span class="mover" />
					<span class="mover [animation-delay:800ms]" />
					<span class="mover [animation-delay:1600ms]" />
				</div>
				<ul class="flex flex-[0_1_16rem] flex-col gap-2.5" aria-label="What the proxy guarantees">
					<li v-for="guarantee in guarantees" :key="guarantee">
						<Badge variant="outline" class="h-10 w-full justify-start px-3.5 font-mono text-sm text-foreground">{{ guarantee }}</Badge>
					</li>
				</ul>
			</CardContent>
		</Card>
		<div class="grid grid-cols-[repeat(auto-fit,minmax(16rem,1fr))] gap-6">
			<div v-for="feature in features" :key="feature.title" class="flex flex-col gap-2">
				<h3 class="text-lg font-semibold">{{ feature.title }}</h3>
				<p class="text-muted-foreground">{{ feature.body }}</p>
			</div>
		</div>
	</section>
</template>
