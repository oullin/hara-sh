<script setup lang="ts">
import HaraMark from '@/components/dots/HaraMark.vue';
import { Button } from '@/components/ui/button';
import { DOCS_URL } from '@/lib/content';

const links = [
	{ href: '#meaning', label: '腹' },
	{ href: '#how', label: 'How it works' },
	{ href: '#providers', label: 'Providers' },
];
</script>

<template>
	<header class="sticky top-0 z-10 border-b bg-background/80 backdrop-blur">
		<div class="page-container flex h-16 items-center justify-between gap-4">
			<div class="flex items-center gap-6">
				<a href="#top" aria-label="hara, home" class="flex items-center gap-2.5 text-foreground no-underline">
					<span class="text-primary"><HaraMark :size="4" /></span>
					<span class="text-lg font-semibold tracking-tight">hara</span>
				</a>
				<nav aria-label="Main" class="hidden items-center gap-1 sm:flex">
					<Button v-for="link in links" :key="link.href" as="a" :href="link.href" variant="ghost" size="sm" class="rounded-full text-muted-foreground">
						{{ link.label }}
					</Button>
				</nav>
			</div>
			<div class="flex items-center gap-2">
				<Button as="a" :href="DOCS_URL" variant="outline" size="sm" class="rounded-full px-3">Docs</Button>
				<Button as="a" href="#how" size="sm" class="rounded-full px-3">Get started</Button>
			</div>
		</div>
	</header>
</template>
