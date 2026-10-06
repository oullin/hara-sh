<script setup lang="ts">
import { Badge } from '@/components/ui/badge';
import { Separator } from '@/components/ui/separator';
</script>

<template>
	<section id="meaning" aria-labelledby="meaning-title">
		<Separator />
		<div class="page-container flex flex-wrap items-center gap-14 py-24">
			<p lang="ja" aria-hidden="true" class="flex-[0_1_18rem] text-center font-kanji text-[12rem] leading-none font-semibold text-primary">腹</p>
			<div class="flex min-w-0 flex-[1_1_32rem] flex-col gap-5">
				<Badge variant="outline" class="font-mono tracking-widest"> HARA · <span lang="ja">はら</span> · NOUN </Badge>
				<h2 id="meaning-title" class="font-display text-4xl leading-tight font-extrabold sm:text-5xl">The center everything moves from.</h2>
				<p class="max-w-xl text-muted-foreground">
					In Japanese, <span lang="ja">腹</span> (hara) is the belly, and with it the body's center of gravity: the place calm and focus come from. Martial artists learn to move from the
					hara, so every motion starts in one steady point.
				</p>
				<p class="max-w-xl text-muted-foreground">That is the idea here. Your tools stay where they are; every request starts from one steady center that knows which account to use.</p>
			</div>
		</div>
		<Separator />
	</section>
</template>
