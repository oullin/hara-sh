<script setup lang="ts">
// Replaces VitePress's VPNavBarSearch (see the alias in config.ts). Questions go to
// POST /api/search/query; if that fails, VitePress's local search opens with the same text.
import { onKeyStroke, useSessionStorage, watchDebounced } from '@vueuse/core';
import { defineAsyncComponent, nextTick, ref } from 'vue';
import { withBase } from 'vitepress';
import VPNavBarSearchButton from 'vitepress/dist/client/theme-default/components/VPNavBarSearchButton.vue';
import type { Result } from '@worker/types.ts';
import { SemanticSearch } from '@docs/.vitepress/theme/search.ts';

const VPLocalSearchBox = defineAsyncComponent(() => import('vitepress/dist/client/theme-default/components/VPLocalSearchBox.vue'));

// VPLocalSearchBox restores its query from this session key.
const localQuery = useSessionStorage('vitepress:local-search-filter', '');
const client = new SemanticSearch();

const open = ref(false);

const local = ref(false);

const loading = ref(false);

const q = ref('');

const results = ref<Array<Result>>(
	[],
);

const input = ref<HTMLInputElement>();

let latest = 0;

async function show() {
	open.value = true;

	await nextTick();

	input.value?.focus();
}

function close() {
	open.value = false;
}

function fallback(text: string) {
	localQuery.value = text;
	open.value = false;
	local.value = true;
}

watchDebounced(
	q,
	async (text) => {
		const request = ++latest;
		const trimmed = text.trim();

		if (!trimmed) {
			loading.value = false;
			results.value = [];

			return;
		}

		loading.value = true;

		const found = await client.query(trimmed);

		if (request !== latest) {
			return;
		}

		loading.value = false;

		if (found === null) {
			fallback(trimmed);

			return;
		}

		results.value = found;
	},
	{ debounce: 250 },
);

function editing(event: KeyboardEvent) {
	const element = event.target as HTMLElement;

	return element.isContentEditable || ['INPUT', 'SELECT', 'TEXTAREA'].includes(element.tagName);
}

onKeyStroke('k', (event) => {
	if (event.ctrlKey || event.metaKey) {
		event.preventDefault();
		show();
	}
});

onKeyStroke('/', (event) => {
	if (!editing(event)) {
		event.preventDefault();
		show();
	}
});

onKeyStroke('Escape', () => {
	if (open.value) {
		close();
	}
});
</script>

<template>
	<div class="VPNavBarSearch">
		<VPNavBarSearchButton text="Search" aria-label="Search" aria-keyshortcuts="/ control+k meta+k" @click="show" />

		<Teleport to="body">
			<div v-if="open" class="search-backdrop" @click.self="close">
				<div class="search-dialog" role="dialog" aria-modal="true" aria-label="Search the docs">
					<input ref="input" v-model="q" class="search-input" type="search" maxlength="500" placeholder="Ask about setup, providers, clients…" aria-label="Search the docs" />

					<p v-if="loading" class="search-note">Searching…</p>

					<ul v-else-if="results.length" class="search-results">
						<li v-for="result in results" :key="result.url">
							<a :href="withBase(result.url)" @click="close">{{ result.title }}</a>
						</li>
					</ul>

					<p v-else-if="q.trim()" class="search-note">No matching pages.</p>
				</div>
			</div>
		</Teleport>

		<VPLocalSearchBox v-if="local" @close="local = false" />
	</div>
</template>

<style scoped>
.VPNavBarSearch {
	display: flex;
	align-items: center;
}

@media (min-width: 48rem) {
	.VPNavBarSearch {
		padding-left: 1.5rem;
	}
}

@media (min-width: 60rem) {
	.VPNavBarSearch {
		padding-left: 2rem;
	}
}

.search-backdrop {
	position: fixed;
	inset: 0;
	z-index: 100;
	display: flex;
	justify-content: center;
	padding: 4rem 1rem 1rem;
	background: var(--vp-backdrop-bg-color);
}

.search-dialog {
	width: min(100%, 36rem);
	max-height: min(32rem, 100%);
	overflow: auto;
	padding: 0.75rem;
	border-radius: 8px;
	background: var(--vp-c-bg);
	box-shadow: var(--vp-shadow-3);
}

.search-input {
	width: 100%;
	padding: 0.625rem 0.75rem;
	border: 1px solid var(--vp-c-divider);
	border-radius: 6px;
	background: var(--vp-c-bg-alt);
	color: var(--vp-c-text-1);
	font-size: 1rem;
}

.search-input:focus {
	border-color: var(--vp-c-brand-1);
	outline: none;
}

.search-results {
	margin: 0.5rem 0 0;
	padding: 0;
	list-style: none;
}

.search-results a {
	display: block;
	padding: 0.625rem 0.75rem;
	border-radius: 6px;
	color: var(--vp-c-text-1);
	text-decoration: none;
}

.search-results a:hover,
.search-results a:focus-visible {
	background: var(--vp-c-default-soft);
	color: var(--vp-c-brand-1);
}

.search-note {
	margin: 0.75rem 0.75rem 0.25rem;
	color: var(--vp-c-text-2);
	font-size: 0.875rem;
}
</style>
