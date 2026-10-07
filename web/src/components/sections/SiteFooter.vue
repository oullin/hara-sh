<script setup lang="ts">
import { DOCS_URL, REPO_URL } from '@/lib/content';
</script>
<template>
	<footer class="site-footer">
		<p>hara <span lang="ja">腹</span> · built on CLIProxyAPI</p>
		<div><a :href="DOCS_URL">Documentation</a><a :href="REPO_URL">GitHub</a><a href="https://gocanto.sh">gocanto.sh</a></div>
	</footer>
</template>
