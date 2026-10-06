<script setup lang="ts">
import HaraMark from '@/components/dots/HaraMark.vue';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
</script>

<template>
	<section aria-labelledby="cta-title" class="page-container pt-14 pb-28">
		<Card>
			<CardContent class="flex flex-col items-center gap-6 py-12 text-center">
				<span class="text-primary"><HaraMark :size="8" /></span>
				<h2 id="cta-title" class="max-w-xl font-display text-4xl leading-tight font-extrabold sm:text-5xl">Find your center.</h2>
				<p class="max-w-md text-muted-foreground">Connect an account, point a client, send a request. A few minutes, start to finish.</p>
				<div class="flex flex-wrap justify-center gap-3">
					<Button as="a" href="[PANEL URL]" size="lg" class="h-12 px-6 text-base">Open the panel</Button>
					<Button as="a" href="[DOCS URL]" variant="outline" size="lg" class="h-12 px-6 text-base">Read the docs</Button>
				</div>
			</CardContent>
		</Card>
	</section>
</template>
