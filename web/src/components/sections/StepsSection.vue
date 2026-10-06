<script setup lang="ts">
import { computed, ref } from 'vue';
import { useActiveStep } from '@/composables/useActiveStep';
import { steps } from '@/lib/content';

const list = ref<HTMLElement | null>(null);
const { active } = useActiveStep(list, steps[0].id);

const current = computed(() => steps.find((step) => step.id === active.value) ?? steps[0]);
</script>

<template>
	<section id="how" aria-labelledby="how-title" class="page-container py-28">
		<div class="mb-16 flex max-w-2xl flex-col gap-4">
			<p class="font-mono text-xs tracking-[0.2em] text-primary">HOW IT WORKS</p>
			<h2 id="how-title" class="text-4xl leading-tight font-medium tracking-[-0.035em] sm:text-5xl">Four steps, then you just code</h2>
		</div>
		<div class="grid gap-16 lg:grid-cols-[minmax(0,0.85fr)_minmax(0,1fr)]">
			<ol ref="list" class="flex flex-col gap-24 lg:gap-40 lg:pb-40">
				<li v-for="(step, index) in steps" :key="step.id" :data-step="step.id" class="flex flex-col gap-3">
					<span class="font-mono text-sm text-muted-foreground">{{ String(index + 1).padStart(2, '0') }}</span>
					<h3 class="text-2xl font-medium tracking-[-0.02em]">
						{{ step.title }}
						<code class="ml-1 rounded-md bg-ds-gray-100 px-1.5 py-0.5 font-mono text-[0.8em] text-foreground">{{ step.token }}</code>
					</h3>
					<p class="max-w-md text-muted-foreground">{{ step.body }}</p>
					<pre class="mt-3 overflow-x-auto rounded-xl bg-card p-4 font-mono text-[13px] leading-6 text-ds-gray-1000 shadow-border lg:hidden">{{ step.code }}</pre>
				</li>
			</ol>
			<div class="hidden lg:block">
				<figure class="sticky top-28 overflow-hidden rounded-2xl bg-card shadow-border-medium">
					<figcaption class="border-b px-5 py-3 font-mono text-[13px] text-muted-foreground">{{ current.file }}</figcaption>
					<pre class="min-h-56 overflow-x-auto p-5 font-mono text-[13px] leading-7 text-ds-gray-1000">{{ current.code }}</pre>
				</figure>
			</div>
		</div>
	</section>
</template>
