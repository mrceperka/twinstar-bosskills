import { SECRET_TOKEN_SYNCHRONIZE } from '$env/static/private';
import { raidLock } from '$lib/date';
import { synchronize } from '$lib/server/db/synchronize';
import { safeGC } from '$lib/server/gc';
import { error } from '@sveltejs/kit';
import { format } from 'date-fns';
import type { RequestHandler } from './$types';
export const GET: RequestHandler = async ({ url, request }) => {
	const xtoken = request.headers.get('X-Synchronize-Token')?.[0] ?? null;
	const token = SECRET_TOKEN_SYNCHRONIZE;

	if (xtoken === null || xtoken !== token || xtoken.length === 0 || token.length === 0) {
		throw error(403, { message: `Synchronize token not found or does not match` });
	}

	safeGC();
	let startsAt: Date | undefined = undefined;
	let endsAt: Date | undefined = undefined;
	let raidLockOffset = 0;
	if (url.searchParams.has('raidLockOffset')) {
		raidLockOffset = Math.abs(Number(url.searchParams.get('raidLockOffset')));
		raidLockOffset = isFinite(raidLockOffset) ? raidLockOffset : 0;

		const now = new Date();
		const { start, end } = raidLock(now, raidLockOffset);
		startsAt = start;
		endsAt = end;
	}
	let bosskillIds = url.searchParams.getAll('bosskillIds').map(Number);
	let bossIds = url.searchParams.getAll('bossIds').map(Number);
	let page = url.searchParams.has('page') ? Number(url.searchParams.get('page')) : undefined;
	let pageSize = url.searchParams.has('pageSize')
		? Number(url.searchParams.get('pageSize'))
		: undefined;

	const encoder = new TextEncoder();
	const stream = new ReadableStream({
		async start(controller) {
			controller.enqueue(`raidLockOffset: ${raidLockOffset}` + '\n');
			controller.enqueue(
				`startsAt: ${startsAt ? format(startsAt, 'yyyy-MM-dd HH:mm:ss') : 'N/A'}` + '\n'
			);
			controller.enqueue(
				`endsAt: ${endsAt ? format(endsAt, 'yyyy-MM-dd HH:mm:ss') : 'N/A'}` + '\n'
			);
			controller.enqueue(`bosskillIds: ${bosskillIds.join(',')}` + '\n');
			try {
				await synchronize({
					onLog: (line: string) => {
						controller.enqueue(encoder.encode(line + '\n'));
					},
					startsAt,
					endsAt,
					bosskillIds,
					bossIds,
					page,
					pageSize
				});
			} catch (e: any) {
				controller.enqueue(encoder.encode(`error: ${e.message}` + '\n'));
				console.error(e);
			}
			controller.close();
		}
	});

	return new Response(stream, {
		headers: {
			'content-type': 'text/event-stream'
		}
	});
};
