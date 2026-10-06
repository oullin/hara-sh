<script setup lang="ts">
import { Separator } from '@/components/ui/separator';
</script>

<template>
	<footer>
		<Separator />
		<div class="page-container flex flex-wrap justify-between gap-4 py-8 font-mono text-sm text-muted-foreground">
			<p>hara <span lang="ja">腹</span> · built on CLIProxyAPI</p>
			<p>Made by <a href="https://gocanto.sh" class="text-foreground underline-offset-4 hover:underline">gocanto.sh</a></p>
		</div>
	</footer>
</template>
