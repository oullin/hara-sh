<script setup lang="ts">
import { computed, ref } from 'vue';
import { useActiveStep } from '@/composables/useActiveStep';
import { steps } from '@/lib/content';

const list = ref<HTMLElement | null>(null);
const { active } = useActiveStep(list, steps[0].id);

const current = computed(() => steps.find((step) => step.id === active.value) ?? steps[0]);
</script>
<template>
	<section id="how" aria-labelledby="how-title" class="marketing-section">
		<div class="section-heading">
			<h2 id="how-title">From Docker to your first request.</h2>
			<p>Start locally. Connect an account. Keep coding.</p>
		</div>
		<div class="steps-layout">
			<ol ref="list" class="setup-steps">
				<li v-for="(step, index) in steps" :key="step.id" :data-step="step.id">
					<span class="step-number">{{ String(index + 1).padStart(2, '0') }}</span>
					<div>
						<h3>{{ step.title }} {{ step.token }}</h3>
						<p>{{ step.body }}</p>
						<pre class="mobile-step-code">{{ step.code }}</pre>
					</div>
				</li>
			</ol>
			<div class="steps-example">
				<figure>
					<figcaption>
						<span class="terminal-dots" aria-hidden="true"><i /><i /><i /></span>{{ current.file }}
					</figcaption>
					<pre>{{ current.code }}</pre>
					<p>Run commands from your project directory.</p>
				</figure>
			</div>
		</div>
	</section>
</template>
