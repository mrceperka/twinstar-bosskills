<script lang="ts">
	import { page } from '$app/stores';
	import Icon from '$lib/components/Icon.svelte';
	import Link from '$lib/components/Link.svelte';
	import TextColorError from '$lib/components/TextColorError.svelte';
	import TextColorSuccess from '$lib/components/TextColorSuccess.svelte';
	import TextColorWarning from '$lib/components/TextColorWarning.svelte';
	import BossPerformanceBoxChart from '$lib/components/echart/BossPerformanceBoxChart.svelte';
	import Table, { cellComponent } from '$lib/components/table/Table.svelte';
	import CharacterDps from '$lib/components/table/column/CharacterDPS.column.svelte';
	import CharacterHps from '$lib/components/table/column/CharacterHPS.column.svelte';
	import CharacterName from '$lib/components/table/column/CharacterName.column.svelte';
	import KilledAt from '$lib/components/table/column/KilledAt.column.svelte';
	import Spec from '$lib/components/table/column/Spec.column.svelte';
	import { formatCell } from '$lib/components/table/column/cell';
	import { formatSecondsInterval, fromServerTime } from '$lib/date';
	import {
		defaultDifficultyByExpansion,
		difficultiesByExpansion,
		difficultyToString,
		isRaidDifficulty,
		talentSpecToString,
		talentSpecsByExpansion
	} from '$lib/model';
	import { formatAvgItemLvl } from '$lib/number';
	import { realmToExpansion } from '$lib/realm';
	import { getDifficultyFromUrl } from '$lib/search-params';
	import { STATS_TYPE_DMG, STATS_TYPE_HEAL, type StatsType } from '$lib/stats-type';
	import { getTalentSpecIconUrl } from '$lib/talent';
	import type { ColumnDef } from '@tanstack/svelte-table';
	import type { PageData } from './$types';
	import BossKillDetailLink from './components/BossKillDetailLink.svelte';

	export let data: PageData;

	const expansion = realmToExpansion(data.realm);
	const talentSpecs = Object.values(talentSpecsByExpansion(expansion) ?? {});
	const difficulties = Object.values(difficultiesByExpansion(expansion) ?? {});
	const title = `Boss ${data.boss.name}`;

	let searchParams = new URLSearchParams($page.url.searchParams);
	const currentDifficulty = String(
		getDifficultyFromUrl($page.url) ?? defaultDifficultyByExpansion(expansion)
	);
	const currentSpec = searchParams.get('spec');

	const specs: { id: number; iconUrl: string; href: string; isActive: boolean }[] = [];
	for (const id of talentSpecs) {
		const isActive = currentSpec === String(id);
		searchParams.set('spec', String(id));
		specs.push({
			id,
			iconUrl: getTalentSpecIconUrl(data.realm, id),
			href: `?${searchParams}`,
			isActive
		});
	}
	searchParams.delete('spec');
	const specResetHref = `?${searchParams}`;

	searchParams = new URLSearchParams($page.url.searchParams);
	const diffs: { id: number; label: string; href: string; isActive: boolean }[] = [];
	for (const id of difficulties) {
		if (isRaidDifficulty(expansion, id)) {
			const isActive = currentDifficulty === String(id);
			searchParams.set('difficulty', String(id));
			diffs.push({
				id,
				label: difficultyToString(expansion, id),
				href: `?${searchParams}`,
				isActive
			});
		}
	}
	searchParams.delete('difficulty');
	const difficultyResetHref = `?${searchParams}`;

	const columnByStatsType: Record<StatsType | string, ColumnDef<unknown>[]> = {
		[STATS_TYPE_DMG]: [],
		[STATS_TYPE_HEAL]: []
	};
	for (const stat of data.stats) {
		type T = (typeof stat.value)[0];
		const isDmg = stat.type === STATS_TYPE_DMG;
		const columns: ColumnDef<T>[] = [
			{
				id: 'character',
				accessorFn: (row) => row.char,
				header: () => 'Character',
				cell: (info) => {
					const rows = info.table.getPreSortedRowModel().rows;
					const index = rows.findIndex((r) => r.id === info.row.id) ?? null;
					let rank = undefined;
					if (index !== null) {
						rank = index + 1;
					}

					return cellComponent(CharacterName, {
						realm: data.realm,
						character: info.row.original.char,
						rank
					});
				}
			},
			{
				id: 'spec',
				accessorFn: (row) => row.char.talent_spec,
				cell: ({ row }) => {
					const { original } = row;
					return cellComponent(Spec, {
						realm: data.realm,
						character: original.char
					});
				},
				header: () => 'Spec'
			},
			{
				id: 'valuePerSecond',
				accessorFn: (row) => row.valuePerSecond,
				cell: (info) => {
					return isDmg
						? cellComponent(CharacterDps, { character: info.row.original.char })
						: cellComponent(CharacterHps, { character: info.row.original.char });
				},

				header: () => (isDmg ? 'DPS' : 'HPS')
			},
			{
				id: 'valueTotal',
				accessorFn: (row) => row.valueTotal,
				cell: formatCell,
				header: () => (isDmg ? 'Dmg Done' : 'Healing Done')
			},
			{
				id: 'fightLength',
				accessorFn: (row) => row.char.boss_kills?.length ?? 0,
				cell: (info) => formatSecondsInterval(info.getValue() as number),
				header: () => 'Fight Length'
			},
			{
				id: 'killedAt',
				header: () => 'Killed',
				accessorFn: (row) => fromServerTime(row.char.boss_kills!.time),
				cell: (info) => cellComponent(KilledAt, { bosskill: info.row.original.char.boss_kills! })
			},
			{
				id: 'avgItemLvl',
				accessorFn: (row) => row.char.avg_item_lvl,
				cell: (info) => formatAvgItemLvl(info.getValue() as any),
				header: () => 'Avg iLvl'
			},
			{
				id: 'detail',
				cell: (info) => {
					const bossKillId = info.row.original.char.boss_kills?.id;
					return cellComponent(BossKillDetailLink, { realm: data.realm, id: bossKillId });
				},
				header: () => 'Details',
				enableSorting: false
			}
		];
		columnByStatsType[stat.type] = columns as any as ColumnDef<unknown>[];
	}
</script>

<svelte:head>
	<title>{title}</title>
</svelte:head>
<h1>{title}</h1>
{#if data.kw}
	<p>
		{data.boss.name} ({difficultyToString(expansion, currentDifficulty)}) was killed
		<TextColorSuccess>{data.kw.kills.total}</TextColorSuccess> times by raiders and wiped them <TextColorError
			>{data.kw.wipes.total}</TextColorError
		> times (<TextColorError>{data.kw.wipes.avg}</TextColorError>
		times on average).
	</p>
	<p>
		You have <TextColorSuccess>{data.kw.kills.chance.toFixed(2)}%</TextColorSuccess> chance to make a
		kill and <TextColorError>{data.kw.wipes.chance.toFixed(2)}%</TextColorError> chance to wipe.
	</p>
	<p>
		<TextColorSuccess>Fastest</TextColorSuccess> kill took
		<TextColorSuccess>{formatSecondsInterval(data.kw.fightDuration.min)}</TextColorSuccess>,
		<TextColorWarning>average</TextColorWarning> kill took
		<TextColorWarning>{formatSecondsInterval(data.kw.fightDuration.avg)}</TextColorWarning> and
		<TextColorError>slowest</TextColorError> kill took
		<TextColorError>{formatSecondsInterval(data.kw.fightDuration.max)}</TextColorError>
	</p>
{/if}

{#if data.realmIsPrivate === false}
	<h2>Top stats by spec</h2>
	<div class="filter">
		<ul>
			<li>
				<Link data-sveltekit-reload style="display: flex;" href={difficultyResetHref}>Reset</Link>
			</li>
			{#each diffs as { label, href, isActive }}
				<li class:active={isActive}>
					<div class:active={isActive}>
						<Link data-sveltekit-reload style="display: flex;" {href}>
							{label}
						</Link>
					</div>
				</li>
			{/each}
		</ul>
		<ul>
			<li><Link data-sveltekit-reload style="display: flex;" href={specResetHref}>Reset</Link></li>
			{#each specs as { id, iconUrl, href, isActive }}
				<li class:active={isActive}>
					<div class:active={isActive}>
						<Link data-sveltekit-reload style="display: flex;" {href}>
							<Icon src={iconUrl} label={talentSpecToString(expansion, id)} />
						</Link>
					</div>
				</li>
			{/each}
		</ul>
	</div>
	<div class="stats">
		{#each data.stats as stat}
			<div>
				{#if stat.value.length > 0}
					{@const isDmg = stat.type === STATS_TYPE_DMG}
					<h3>{isDmg ? 'Top DPS' : 'Top HPS'}</h3>
					<Table
						data={stat.value}
						columns={columnByStatsType[stat.type]}
						sorting={[{ id: 'valuePerSecond', desc: true }]}
					/>
				{/if}
			</div>
		{/each}
	</div>
{/if}

<h2>DPS by Talent Spec</h2>
<BossPerformanceBoxChart
	realm={data.realm}
	width={data.windowInnerWidth}
	field="dps"
	aggregated={data.aggregated.dps}
/>

<h2>HPS by Talent Spec</h2>
<BossPerformanceBoxChart
	realm={data.realm}
	width={data.windowInnerWidth}
	field="hps"
	aggregated={data.aggregated.hps}
/>

<style>
	.filter ul li div {
		padding: 0.25rem;
	}
	.filter ul li div.active {
		border-bottom: 2px solid rgba(var(--color-primary), 1);
	}
	.filter :global(a) {
		text-decoration: none;
	}
	ul {
		display: flex;
		align-items: center;
		flex-wrap: wrap;
		margin-bottom: 0.5rem;
	}
	ul li {
		margin-right: 0.25rem;
	}
	.stats {
		display: grid;
		grid-template-columns: 1fr 1fr;
	}
	.stats div {
		margin-right: 1rem;
		overflow: auto;
	}
	:global(.stats table) {
		max-height: 75vh;
	}
	@media (max-width: 900px) {
		.stats {
			display: flex;
			flex-wrap: wrap;
		}
	}
	@media (max-width: 720px) {
		:global(.stats table) {
			max-height: 500px;
		}
	}
</style>
