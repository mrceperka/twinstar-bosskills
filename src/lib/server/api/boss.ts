import { TWINSTAR_API_URL } from '$env/static/private';
import { prepareData, type PreparedData } from '$lib/components/echart/boxplot';
import { mutateCharacter, type Boss, type Character } from '$lib/model';
import { REALM_HELIOS } from '$lib/realm';
import { withCache } from '../cache';
import {
	getBossKillDetail,
	getLatestBossKills,
	listAllLatestBossKills,
	type BossKillQueryArgs,
	type LatestBossKillQueryArgs
} from './boss-kills';
import { FilterOperator, queryString, type QueryArgs } from './filter';
import { getRaids } from './raid';

export const getBoss = async (id: number): Promise<Boss | null> => {
	const fallback = async () => {
		try {
			const raids = await getRaids();
			for (const raid of raids) {
				for (const boss of raid.bosses) {
					if (boss.entry === id) {
						return boss;
					}
				}
			}
		} catch (e) {
			console.error(e);
			throw e;
		}

		return null;
	};

	return withCache({ deps: [`boss`, id], fallback, defaultValue: null });
};

type BossStats = {
	byClass: Record<number, Character[]>;
	bySpec: Record<number, Character[]>;
};
const EMPTY_STATS: BossStats = { byClass: {}, bySpec: {} };
export const getBossStats = async (id: number): Promise<BossStats> => {
	const q: BossKillQueryArgs = {
		page: 0,
		pageSize: 100,
		filters: [
			{
				column: 'entry',
				operator: FilterOperator.EQUALS,
				value: id
			}
			// this break API
			/*
			{
				column: 'difficulty',
				operator: FilterOperator.IN,
				value: [
					Difficulty.DIFFICULTY_10_N,
					Difficulty.DIFFICULTY_10_HC,
					Difficulty.DIFFICULTY_25_N,
					Difficulty.DIFFICULTY_25_HC
				]
			}
			*/
		]
	};

	const fallback = async (): Promise<BossStats> => {
		try {
			const bosskills = await getLatestBossKills(q);
			// TODO: byDifficulty
			const byClass: BossStats['byClass'] = {};
			const bySpec: BossStats['bySpec'] = {};
			const details = await Promise.all(bosskills.data.map((bk) => getBossKillDetail(bk.id)));
			for (const detail of details) {
				if (detail) {
					for (const character of detail.boss_kills_players) {
						// TODO: this eats memory
						byClass[character.class] ??= [];
						bySpec[character.talent_spec] ??= [];

						byClass[character.class]!.push(character);
						bySpec[character.talent_spec]!.push(character);
					}
				}
			}
			return { byClass, bySpec };
		} catch (e) {
			console.error(e);
			throw e;
		}
	};

	return withCache({ deps: [`boss-stats`, q], fallback, defaultValue: EMPTY_STATS });
};

type BossStatsQueryArgs = Pick<
	QueryArgs<'dmgDone' | 'healingDone' | 'dps' | 'hps'>,
	'sorter' | 'difficulty' | 'pageSize' | 'talentSpec'
>;
export const getBossStatsV2 = async (id: number, qa: BossStatsQueryArgs): Promise<BossStats> => {
	const q: BossStatsQueryArgs = {
		sorter: {
			column: 'dmgDone',
			order: 'desc'
		},
		...qa
	};

	const fallback = async (): Promise<BossStats> => {
		// https://twinstar-api.twinstar-wow.com/bosskills/top/59915?realm=Helios&pageSize=10&mode=4&sorter={%22column%22:%22dmgDone%22,%22order%22:%22desc%22}
		const url = `${TWINSTAR_API_URL}/bosskills/top/${id}?${queryString(q)}`;
		const byClass: BossStats['byClass'] = {};
		const bySpec: BossStats['bySpec'] = {};
		try {
			const r = await fetch(url);
			const data: Character[] = await r.json();

			for (const character of data) {
				mutateCharacter(character);
				// TODO: this eats memory
				byClass[character.class] ??= [];
				bySpec[character.talent_spec] ??= [];

				byClass[character.class]!.push(character);
				bySpec[character.talent_spec]!.push(character);
			}
			return { byClass, bySpec };
		} catch (e) {
			console.error(e, url);
			throw e;
		}
	};

	return withCache({ deps: [`boss-stats-v2`, id, q], fallback, defaultValue: EMPTY_STATS });
};

export const getBossKillsWipesTimes = async (id: number, mode: number | null) => {
	const fallback = async () => {
		const filters: LatestBossKillQueryArgs['filters'] = [
			{
				column: 'entry',
				operator: FilterOperator.EQUALS,
				value: id
			}
		];

		if (mode !== null) {
			filters.push({ column: 'mode', operator: FilterOperator.EQUALS, value: mode });
		}
		const bosskills = await listAllLatestBossKills({
			filters
		});

		const killsCount = bosskills.length;
		let wipesCount = 0;
		let minWipes = Infinity;
		let maxWipes = 0;
		let totalDuration = 0;
		let minFightDuration = Infinity;
		let maxFightDuration = 0;
		for (const bk of bosskills) {
			wipesCount += bk.wipes;
			if (bk.wipes <= minWipes) {
				minWipes = bk.wipes;
			}

			if (bk.wipes >= maxWipes) {
				maxWipes = bk.wipes;
			}

			totalDuration += bk.length;
			if (bk.length <= minFightDuration) {
				minFightDuration = bk.length;
			}

			if (bk.length >= maxFightDuration) {
				maxFightDuration = bk.length;
			}
		}
		const pullsCount = killsCount + wipesCount;
		const avgFightDuration = killsCount > 0 ? Math.ceil(totalDuration / killsCount) : 0;
		const avgWipes = killsCount > 0 ? Math.ceil(wipesCount / killsCount) : 0;
		const wipeChance = pullsCount === 0 ? 0 : Math.ceil(10000 * (wipesCount / pullsCount)) / 100;
		const killChance = pullsCount === 0 ? 0 : Math.ceil(10000 * (killsCount / pullsCount)) / 100;

		return {
			kills: {
				chance: killChance,
				total: killsCount
			},
			wipes: {
				min: isFinite(minWipes) ? minWipes : 0,
				avg: avgWipes,
				total: wipesCount,
				chance: wipeChance
			},
			fightDuration: {
				min: isFinite(minFightDuration) ? minFightDuration : 0,
				max: maxFightDuration,
				avg: avgFightDuration,
				total: totalDuration
			}
		};
	};
	return withCache({
		deps: ['boss-kwt', id, mode],
		fallback,
		defaultValue: null
	});
};

type IndexToSpecId = Record<number, number>;
export type BossAggregatedStats = { indexToSpecId: IndexToSpecId; prepared: PreparedData };
export const getBossAggregatedStats = async (
	id: number,
	field: 'dps' | 'hps',
	mode: number
): Promise<BossAggregatedStats> => {
	const fallback = async () => {
		// const items: { value: number; spec: number; label: string }[] = [];
		type Item = { spec: number; talent_spec: number; dps?: string; hps?: string };
		const url = `${TWINSTAR_API_URL}/bosskills/aggreggate?realm=${REALM_HELIOS}&entry=${id}&field=${field}&mode=${mode}`;
		try {
			const r = await fetch(url);
			const data: Item[] = await r.json();
			if (Array.isArray(data) === false) {
				throw new Error(`data is not an array`);
			}

			const aggregatedBySpec: Record<number, number[]> = {};
			for (const item of data) {
				const value = Number(item?.[field]);
				if (isFinite(value)) {
					// do not care about 0 dps/hps/...
					if (value <= 0) {
						continue;
					}

					// safety borders, value this big is probably bug
					if (field === 'dps' && value > 1_000_000) {
						continue;
					}

					// safety borders, value this big is probably bug
					if (field === 'hps' && value > 5_000_000) {
						continue;
					}

					const spec = item.spec ?? item.talent_spec ?? 0;
					// Unknown spec
					if (spec === 0) {
						continue;
					}

					aggregatedBySpec[spec] ??= [];
					aggregatedBySpec[spec]!.push(value);
				}
			}

			// remember which value index maps to specId
			const keys = [];
			const values = [];
			for (const [key, value] of Object.entries(aggregatedBySpec)) {
				keys.push(Number(key));
				values.push(value);
			}

			const prepared = prepareData(values);

			// add index dimension
			const boxDataWithIndex: [(typeof prepared.boxData)[0], number][] = [];
			for (let i = 0; i < prepared.boxData.length; ++i) {
				boxDataWithIndex.push([prepared.boxData[i]!, i]);
			}
			boxDataWithIndex.sort((a, b) => {
				// median
				return a[0][2] - b[0][2];
			});

			// remove index dimension and round numbers
			prepared.boxData = boxDataWithIndex.map(([d]) => {
				d[0] = Math.round(100 * d[0]) / 100;
				d[1] = Math.round(100 * d[1]) / 100;
				d[2] = Math.round(100 * d[2]) / 100;
				d[3] = Math.round(100 * d[3]) / 100;
				d[4] = Math.round(100 * d[4]) / 100;

				return d;
			});

			const indexToSpecId: IndexToSpecId = {};
			for (let i = 0; i < boxDataWithIndex.length; i++) {
				const keyIndex = boxDataWithIndex[i]![1];
				indexToSpecId[i] = keys[keyIndex]!;
			}

			return { indexToSpecId, prepared };
		} catch (e) {
			console.log(e, url);
			throw e;
		}
	};
	return await withCache({
		deps: ['boss-aggregated', id, field, mode],
		fallback,
		sliding: false,
		defaultValue: { indexToSpecId: {}, prepared: { axisData: [], boxData: [], outliers: [] } }
	});
};
