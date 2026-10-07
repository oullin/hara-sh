<script setup lang="ts">
import DotGrid from '@/components/dots/DotGrid.vue';
import { providers, DOCS_URL } from '@/lib/content';
import { glyphDots } from '@/lib/dots';

const items = providers.map((provider) => ({ ...provider, dots: glyphDots(provider.glyph) }));
</script>
<template>
	<section id="providers" aria-labelledby="providers-title" class="marketing-section providers-section">
		<div class="section-heading">
			<h2 id="providers-title">Bring your accounts.</h2>
			<p>OAuth subscriptions, API keys, and compatible endpoints.<br />Available integrations depend on your proxy version and plugins.</p>
		</div>
		<ul class="provider-grid">
			<li v-for="item in items" :key="item.name">
				<span class="provider-glyph"><DotGrid :dots="item.dots" :size="5" /></span
				><span
					><strong>{{ item.name }}</strong
					><small>{{ item.via }}</small></span
				>
			</li>
		</ul>
		<div class="routing-strip">
			<div>
				<h3>Keep the session.</h3>
				<p>Conversations and subagents stay together, preserving account-level prompt caches.</p>
			</div>
			<div>
				<h3>Use available capacity.</h3>
				<p>Quota routing favours capacity near its weekly reset. Failover uses another eligible account.</p>
			</div>
			<a :href="`${DOCS_URL}providers`">Connect a provider</a>
		</div>
	</section>
</template>
