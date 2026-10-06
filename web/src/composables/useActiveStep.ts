import { onBeforeUnmount, onMounted, ref, type Ref } from 'vue';

// Tracks which step is in the middle of the viewport, for the sticky "how it works" panel.
// Elements carry `data-step="<id>"`. Without IntersectionObserver the first step stays active.
export function useActiveStep(container: Ref<HTMLElement | null>, initial: string) {
	const active = ref(initial);

	let observer: IntersectionObserver | undefined;

	onMounted(() => {
		if (!container.value || typeof IntersectionObserver === 'undefined') {
			return;
		}

		observer = new IntersectionObserver(
			(entries) => {
				for (const entry of entries) {
					const id = (entry.target as HTMLElement).dataset.step;

					if (entry.isIntersecting && id) {
						active.value = id;
					}
				}
			},
			{ rootMargin: '-45% 0px -45% 0px' },
		);

		for (const element of container.value.querySelectorAll('[data-step]')) {
			observer.observe(element);
		}
	});

	onBeforeUnmount(() => observer?.disconnect());

	return { active };
}
