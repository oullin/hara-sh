<script setup lang="ts">
import { Card, CardContent } from '@/components/ui/card';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { SETUP } from '@/lib/content';
</script>

<template>
	<section id="setup" aria-labelledby="setup-title" class="page-container flex flex-col gap-10 py-16">
		<h2 id="setup-title" class="font-display text-4xl leading-tight font-extrabold sm:text-5xl">Two lines. Then just code.</h2>
		<Tabs default-value="claude" class="gap-4 data-[orientation=horizontal]:flex-col">
			<TabsList>
				<TabsTrigger value="claude">Claude Code</TabsTrigger>
				<TabsTrigger value="codex">Codex</TabsTrigger>
			</TabsList>
			<TabsContent value="claude">
				<Card>
					<CardContent>
						<pre class="overflow-x-auto font-mono text-sm leading-7">{{ SETUP.claude }}</pre>
					</CardContent>
				</Card>
			</TabsContent>
			<TabsContent value="codex">
				<Card>
					<CardContent>
						<p class="mb-3 font-mono text-xs text-muted-foreground">~/.codex/config.toml</p>
						<pre class="overflow-x-auto font-mono text-sm leading-7">{{ SETUP.codex }}</pre>
					</CardContent>
				</Card>
			</TabsContent>
		</Tabs>
	</section>
</template>
