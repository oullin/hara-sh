import { onBeforeUnmount, ref } from 'vue';

// Copies text to the clipboard and flips `copied` for `resetMs`. A clipboard failure
// (no permission, insecure context) is reported through `failed` instead of throwing.
export function useCopy(resetMs = 1600) {
	const copied = ref(false);

	const failed = ref(false);

	let timer: ReturnType<typeof setTimeout> | undefined;

	async function copy(text: string): Promise<void> {
		try {
			await navigator.clipboard.writeText(text);

			copied.value = true;
			failed.value = false;
		} catch {
			copied.value = false;
			failed.value = true;
		}

		clearTimeout(timer);
		timer = setTimeout(() => {
			copied.value = false;
			failed.value = false;
		}, resetMs);
	}

	onBeforeUnmount(() => clearTimeout(timer));

	return { copied, copy, failed };
}
