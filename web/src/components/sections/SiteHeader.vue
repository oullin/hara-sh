<script setup lang="ts">
import HaraMark from '@/components/dots/HaraMark.vue';
import { Button } from '@/components/ui/button';
import { DOCS_URL } from '@/lib/content';

const links = [
	{ href: '#how', label: 'How it works' },
	{ href: '#providers', label: 'Providers' },
];
</script>
<template>
	<header class="site-header">
		<div class="header-inner">
			<a href="#top" aria-label="hara, home" class="brand"
				><span class="text-primary"><HaraMark :size="4" /></span><span>hara</span></a
			>
			<nav aria-label="Main" class="header-nav">
				<a v-for="link in links" :key="link.href" :href="link.href">{{ link.label }}</a>
			</nav>
			<div class="header-actions">
				<a :href="DOCS_URL" class="docs-link">Docs</a><Button as="a" :href="`${DOCS_URL}setup`" size="sm" class="marketing-button primary-action">Get started</Button>
			</div>
		</div>
	</header>
</template>
