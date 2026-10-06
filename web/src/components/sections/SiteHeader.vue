<script setup lang="ts">
import HaraMark from '@/components/dots/HaraMark.vue';
import { Button } from '@/components/ui/button';

const links = [
	{ href: '#meaning', label: '腹' },
	{ href: '#providers', label: 'Providers' },
	{ href: '#routing', label: 'Routing' },
	{ href: '#setup', label: 'Setup' },
];
</script>

<template>
	<header class="border-b">
		<div class="page-container flex flex-wrap items-center justify-between gap-4 py-4">
			<a href="#top" aria-label="hara, home" class="flex items-center gap-3 no-underline">
				<span class="flex size-11 items-center justify-center rounded-xl bg-primary text-primary-foreground">
					<HaraMark :size="4" />
				</span>
				<span class="font-display text-3xl font-extrabold tracking-wide text-foreground">hara</span>
			</a>
			<nav aria-label="Main" class="flex flex-wrap gap-1">
				<Button v-for="link in links" :key="link.href" as="a" :href="link.href" variant="ghost" size="lg" class="text-muted-foreground">
					{{ link.label }}
				</Button>
			</nav>
		</div>
	</header>
</template>
