<script setup lang="ts">
import CallToAction from '@/components/sections/CallToAction.vue';
import HeroSection from '@/components/sections/HeroSection.vue';
import MeaningSection from '@/components/sections/MeaningSection.vue';
import ProvidersSection from '@/components/sections/ProvidersSection.vue';
import SiteFooter from '@/components/sections/SiteFooter.vue';
import SiteHeader from '@/components/sections/SiteHeader.vue';
import StepsSection from '@/components/sections/StepsSection.vue';
</script>

<template>
	<div class="min-h-screen bg-background font-sans text-foreground antialiased">
		<SiteHeader />
		<main id="top">
			<HeroSection />
			<MeaningSection />
			<StepsSection />
			<ProvidersSection />
			<CallToAction />
		</main>
		<SiteFooter />
	</div>
</template>
