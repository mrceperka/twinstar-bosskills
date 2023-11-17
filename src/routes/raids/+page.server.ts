import { raidLock } from '$lib/date';
import * as api from '$lib/server/api';
import { listAllLatestBossKills } from '$lib/server/api';
import { FilterOperator } from '$lib/server/api/filter';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async () => {
	const now = new Date();
	const { start: thisRaidLockStart, end: thisRaidLockEnd } = raidLock(now);

	const [raids, bosskills] = await Promise.all([
		api.getRaids(),
		listAllLatestBossKills({
			pageSize: 10_000,
			filters: [
				{ column: 'time', operator: FilterOperator.GTE, value: thisRaidLockStart },
				{ column: 'time', operator: FilterOperator.LTE, value: thisRaidLockEnd }
			],
			cache: false
		})
	]);
	const bosskillsByBoss: Record<number, number> = {};
	const bosskillsByBossByDifficulty: Record<number, Record<number, number>> = {};
	const bosskillsByRaid: Record<string, number> = {};
	const bosskillsByRaidByDifficulty: Record<string, Record<number, number>> = {};
	for (const bk of bosskills) {
		bosskillsByBoss[bk.entry] ??= 0;
		bosskillsByBoss[bk.entry]++;

		bosskillsByBossByDifficulty[bk.entry] ??= { [bk.mode]: 0 };
		if (typeof bosskillsByBossByDifficulty[bk.entry]?.[bk.mode] === 'undefined') {
			bosskillsByBossByDifficulty[bk.entry]![bk.mode] = 0;
		}
		bosskillsByBossByDifficulty[bk.entry]![bk.mode]++;

		bosskillsByRaid[bk.map] ??= 0;
		bosskillsByRaid[bk.map]++;
		bosskillsByRaidByDifficulty[bk.map] ??= { [bk.mode]: 0 };
		if (typeof bosskillsByRaidByDifficulty[bk.map]?.[bk.mode] === 'undefined') {
			bosskillsByRaidByDifficulty[bk.map]![bk.mode] = 0;
		}
		bosskillsByRaidByDifficulty[bk.map]![bk.mode]++;
	}

	return {
		raids,
		bosskillsByBoss,
		bosskillsByBossByDifficulty,
		bosskillsByRaid,
		bosskillsByRaidByDifficulty
	};
};
