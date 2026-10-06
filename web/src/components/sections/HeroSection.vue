<script setup lang="ts">
import { Check, Copy } from '@lucide/vue';
import { computed } from 'vue';
import { Button } from '@/components/ui/button';
import { useCopy } from '@/composables/useCopy';
import { SETUP } from '@/lib/content';

const { copied, copy, failed } = useCopy();

const copyLabel = computed(() => (copied.value ? 'Copied' : failed.value ? 'Copy failed' : 'Copy command'));
</script>

<template>
	<section aria-labelledby="hero-title" class="page-container pt-16 pb-24">
		<div class="flex max-w-4xl flex-col gap-8">
			<h1 id="hero-title" class="font-display text-5xl leading-[0.98] font-extrabold sm:text-7xl">One center for every model.</h1>
			<p class="max-w-2xl text-lg text-muted-foreground sm:text-xl">
				A self-hosted proxy that pools your AI subscriptions and API keys behind one OpenAI- and Anthropic-compatible endpoint. Claude Code, Codex and any OpenAI client route through it; each
				conversation keeps its account, and limits fail over on their own.
			</p>
			<div class="flex min-h-13 max-w-2xl items-center gap-2 rounded-xl border bg-card py-1.5 pr-1.5 pl-4">
				<code class="min-w-0 flex-1 overflow-x-auto font-mono text-sm whitespace-nowrap text-card-foreground">{{ SETUP.hero }}</code>
				<Button type="button" variant="outline" size="icon-lg" :aria-label="copyLabel" @click="copy(SETUP.hero)">
					<Check v-if="copied" />
					<Copy v-else />
				</Button>
			</div>
			<p role="status" class="sr-only">{{ copied ? 'Command copied' : failed ? 'Copy failed' : '' }}</p>
		</div>
	</section>
</template>
