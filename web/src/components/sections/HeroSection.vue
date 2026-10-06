<script setup lang="ts">
import { Check, Copy } from '@lucide/vue';
import { computed, ref } from 'vue';
import HaraMark from '@/components/dots/HaraMark.vue';
import { Button } from '@/components/ui/button';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { useCopy } from '@/composables/useCopy';
import { clients, DOCS_URL } from '@/lib/content';
import { ringField } from '@/lib/dots';

const { copied, copy, failed } = useCopy();

const selected = ref(clients[0].id);

const command = computed(() => clients.find((client) => client.id === selected.value)?.command ?? clients[0].command);

const copyLabel = computed(() => (copied.value ? 'Copied' : failed.value ? 'Copy failed' : 'Copy command'));
const field = ringField(11);
</script>

<template>
	<section aria-labelledby="hero-title" class="page-container grid items-center gap-16 pt-24 pb-28 lg:grid-cols-[minmax(0,1fr)_minmax(0,0.8fr)]">
		<div class="flex min-w-0 flex-col gap-7">
			<p class="flex items-center gap-2.5 text-primary">
				<HaraMark :size="5" />
				<span class="text-sm font-medium tracking-tight text-foreground">hara <span lang="ja" class="text-muted-foreground">腹</span></span>
			</p>
			<h1 id="hero-title" class="text-5xl leading-[1.02] font-medium tracking-[-0.045em] text-balance sm:text-6xl lg:text-7xl">One center for every model</h1>
			<Tabs v-model="selected" class="gap-0">
				<TabsList variant="line" class="h-auto gap-1 p-0" aria-label="Choose a client">
					<TabsTrigger
						v-for="client in clients"
						:key="client.id"
						:value="client.id"
						class="h-7 flex-none rounded-full px-3 text-[13px] data-[state=active]:bg-ds-gray-100 data-[state=active]:text-foreground"
					>
						{{ client.label }}
					</TabsTrigger>
				</TabsList>
			</Tabs>
			<div class="flex flex-wrap items-center gap-3">
				<div class="flex h-11 max-w-full min-w-0 items-center gap-1 rounded-full border bg-card pr-1 pl-4">
					<span aria-hidden="true" class="font-mono text-sm text-muted-foreground">$</span>
					<code class="min-w-0 overflow-x-auto px-1.5 font-mono text-sm whitespace-nowrap">{{ command }}</code>
					<Button type="button" variant="ghost" size="icon" class="rounded-full" :aria-label="copyLabel" @click="copy(command)">
						<Check v-if="copied" />
						<Copy v-else />
					</Button>
				</div>
				<Button as="a" :href="DOCS_URL" size="lg" class="h-11 rounded-full bg-foreground px-5 text-[15px] text-background hover:bg-foreground/90">Read the docs</Button>
			</div>
			<p role="status" class="sr-only">{{ copied ? 'Command copied' : failed ? 'Copy failed' : '' }}</p>
			<p class="max-w-xl text-lg leading-relaxed text-muted-foreground">
				A self-hosted proxy for your AI subscriptions. One OpenAI- and Anthropic-compatible endpoint, every account behind it, and limits that fail over on their own.
			</p>
		</div>
		<div aria-hidden="true" class="flex justify-center text-primary lg:justify-end">
			<div class="grid w-full max-w-sm grid-cols-[repeat(11,minmax(0,1fr))] gap-3.5">
				<span v-for="(dot, index) in field" :key="index" :class="dot.class" class="aspect-square w-full" :style="{ animationDelay: `${dot.delay}ms` }" />
			</div>
		</div>
	</section>
</template>
