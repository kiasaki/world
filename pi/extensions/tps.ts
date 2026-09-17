/**
 * tokens/sec status extension for pi.
 *
 * Shows live decode throughput while the model streams, and the exact
 * average (from provider `usage.output`) when the response finishes:
 *
 *   ⚡ ~84.2 tok/s                 (streaming, estimated)
 *   ⚡ 91.7 tok/s · 1423 out       (final, exact)
 *
 * The estimate assumes ~4 chars/token; the final number uses the real
 * `usage.output` (which already includes reasoning tokens), measured from
 * the first streamed token, so prefill/TTFT is excluded.
 *
 * Install: ~/.pi/agent/extensions/tps.ts  (auto-discovered; run /reload)
 */
import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";

const KEY = "tps";

function fmt(n: number): string {
	if (n >= 100) return n.toFixed(0);
	if (n >= 10) return n.toFixed(1);
	return n.toFixed(2);
}

export default function (pi: ExtensionAPI) {
	let firstTokenAt = 0;
	let streamChars = 0;
	let lastRenderAt = 0;
	let sawUsage = false;

	const reset = () => {
		firstTokenAt = 0;
		streamChars = 0;
		lastRenderAt = 0;
		sawUsage = false;
	};

	pi.on("turn_start", async (_event, ctx) => {
		reset();
		ctx.ui.setStatus(KEY, undefined);
	});

	pi.on("message_start", async (event, _ctx) => {
		if (event.message.role === "assistant") reset();
	});

	pi.on("message_update", async (event, ctx) => {
		const ev = event.assistantMessageEvent;
		if (
			!ev ||
			(ev.type !== "text_delta" &&
				ev.type !== "thinking_delta" &&
				ev.type !== "toolcall_delta")
		) {
			return;
		}
		const now = Date.now();
		if (!firstTokenAt) firstTokenAt = now;
		streamChars += ev.delta.length;
		if (now - lastRenderAt < 250) return; // throttle to ~4 fps
		lastRenderAt = now;
		const elapsed = (now - firstTokenAt) / 1000;
		if (elapsed <= 0) return;
		ctx.ui.setStatus(KEY, `⚡ ~${fmt(streamChars / 4 / elapsed)} tok/s`);
	});

	pi.on("message_end", async (event, ctx) => {
		if (event.message.role !== "assistant") return;
		const out = event.message.usage?.output ?? 0;
		if (!firstTokenAt || out <= 0) {
			ctx.ui.setStatus(KEY, undefined);
			return;
		}
		const elapsed = (Date.now() - firstTokenAt) / 1000;
		if (elapsed <= 0) return;
		sawUsage = true;
		ctx.ui.setStatus(KEY, `⚡ ${fmt(out / elapsed)} tok/s · ${out} out`);
	});

	pi.on("agent_end", async (_event, ctx) => {
		// Leave the final value up until the next turn clears it.
		if (!sawUsage) ctx.ui.setStatus(KEY, undefined);
	});
}
