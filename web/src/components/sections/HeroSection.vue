<script setup lang="ts">
import { Check, Copy } from '@lucide/vue';
import { computed, ref } from 'vue';
import HaraMark from '@/components/dots/HaraMark.vue';
import { Button } from '@/components/ui/button';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { useCopy } from '@/composables/useCopy';
import { clients, DOCS_URL } from '@/lib/content';

const { copied, copy, failed } = useCopy();

const selected = ref(clients[0].id);

const command = computed(() => clients.find((client) => client.id === selected.value)?.command ?? clients[0].command);

const copyLabel = computed(() => (copied.value ? 'Copied' : failed.value ? 'Copy failed' : 'Copy command'));
</script>

<template>
	<section aria-labelledby="hero-title" class="hero-section">
		<div class="hero-layout">
			<div class="hero-copy">
				<p class="hero-intro">Self-hosted AI infrastructure</p>
				<h1 id="hero-title">Your accounts.<br />One endpoint.</h1>
				<p class="hero-description">Connect your AI subscriptions to Claude Code, Codex, and the tools you already use. Hara handles the accounts behind one API.</p>
				<div class="hero-actions">
					<Button as="a" :href="`${DOCS_URL}setup`" size="lg" class="marketing-button primary-action">Get started</Button>
					<Button as="a" :href="DOCS_URL" variant="outline" size="lg" class="marketing-button">Read the docs</Button>
				</div>
				<div class="client-example">
					<Tabs v-model="selected" class="gap-0">
						<TabsList variant="line" class="client-tabs" aria-label="Choose a client">
							<TabsTrigger v-for="client in clients" :key="client.id" :value="client.id" class="client-tab">{{ client.label }}</TabsTrigger>
						</TabsList>
					</Tabs>
					<div class="command-line">
						<code>{{ command }}</code>
						<Button type="button" variant="ghost" size="icon" :aria-label="copyLabel" @click="copy(command)"><Check v-if="copied" /><Copy v-else /></Button>
					</div>
					<p role="status" class="sr-only">{{ copied ? 'Command copied' : failed ? 'Copy failed' : '' }}</p>
					<p class="command-note">Use your client key. <a :href="`${DOCS_URL}clients`">Client setup</a></p>
				</div>
			</div>
			<figure class="endpoint-visual" aria-label="Claude Code, Codex and API clients connect through Hara to your provider accounts">
				<div class="flow-grid" aria-hidden="true">
					<svg class="flow-lines" viewBox="0 0 480 320" fill="none" preserveAspectRatio="none">
						<path d="M100 66H145Q170 66 170 96V160H215 M100 160H215 M100 254H145Q170 254 170 224V160 M265 160H310V96Q310 66 335 66H380 M265 160H380 M310 160V224Q310 254 335 254H380" />
					</svg>
					<div class="flow-column clients-column"><span>Claude Code</span><span>Codex</span><span>API client</span></div>
					<div class="flow-center">
						<span class="flow-mark"><HaraMark :size="7" /></span><span class="flow-name">hara</span>
					</div>
					<div class="flow-column providers-column"><span>Claude</span><span>OpenAI</span><span>More providers</span></div>
				</div>
				<figcaption><span>One compatible API</span><code>localhost:8317</code></figcaption>
			</figure>
		</div>
		<aside id="meaning" class="hero-footnote" aria-label="About Hara">
			<p><span lang="ja">腹</span> Hara means the body's centre of gravity. One steady centre for your requests.</p>
			<span>Docker Compose · Private by default</span>
		</aside>
	</section>
</template>
