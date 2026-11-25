import { useEffect, useMemo, useState } from 'react'

import { useI18n } from '../../i18n/I18nProvider'
import type { RiskLevel, Scan, ScanStatus } from '../../types'
import { EmptyState } from '../ui/EmptyState'
import { InlineAlert } from '../ui/InlineAlert'
import { ScanTableSkeleton } from '../ui/Skeleton'
import { Spinner } from '../ui/Spinner'

const PAGE_SIZE = 10

const riskLabels: Record<RiskLevel, string> = {
	unknown: 'Näbelli',
	low: 'Pes',
	medium: 'Orta',
	high: 'Ýokary',
	critical: 'Örän ýokary',
}

const statusLabels: Record<ScanStatus, string> = {
	pending: 'Nobatda',
	processing: 'Seljerilýär',
	completed: 'Tamamlandy',
	failed: 'Ýalňyşlyk',
}

type StatusFilter = ScanStatus | 'all'
type RiskFilter = RiskLevel | 'all'

type ScanTableProps = {
	scans: Scan[]
	totalCount: number
	initialLoading: boolean
	refreshing: boolean
	error: string
	onRefresh: () => void
	onSelectScan: (scanId: number) => void
}

export function ScanTable({
	scans,
	totalCount,
	initialLoading,
	refreshing,
	error,
	onRefresh,
	onSelectScan,
}: ScanTableProps) {
	const { t, language } = useI18n()
	const [search, setSearch] = useState('')
	const [statusFilter, setStatusFilter] = useState<StatusFilter>('all')
	const [riskFilter, setRiskFilter] = useState<RiskFilter>('all')
	const [page, setPage] = useState(1)

	const riskLabels: Record<RiskLevel, string> = {
		unknown: t('unknown'),
		low: t('low'),
		medium: t('medium'),
		high: t('high'),
		critical: t('critical'),
	}

	const statusLabels: Record<ScanStatus, string> = {
		pending: t('pending'),
		processing: t('processing'),
		completed: t('completed'),
		failed: t('failed'),
	}

	const filteredScans = useMemo(() => {
		const query = search.trim().toLowerCase()

		return scans.filter(scan => {
			const matchesSearch =
				!query ||
				scan.url.toLowerCase().includes(query) ||
				scan.normalizedUrl.toLowerCase().includes(query)

			const matchesStatus =
				statusFilter === 'all' || scan.status === statusFilter

			const matchesRisk = riskFilter === 'all' || scan.riskLevel === riskFilter

			return matchesSearch && matchesStatus && matchesRisk
		})
	}, [scans, search, statusFilter, riskFilter])

	const pageCount = Math.max(1, Math.ceil(filteredScans.length / PAGE_SIZE))

	const currentPage = Math.min(page, pageCount)

	useEffect(() => {
		if (page > pageCount) {
			setPage(pageCount)
		}
	}, [page, pageCount])

	const startIndex = (currentPage - 1) * PAGE_SIZE
	const visibleScans = filteredScans.slice(startIndex, startIndex + PAGE_SIZE)

	function resetFilters() {
		setSearch('')
		setStatusFilter('all')
		setRiskFilter('all')
		setPage(1)
	}

	return (
		<section className='table-card' id='scans'>
			<div className='section-title'>
				<div>
					<span className='eyebrow'>{t('recentActivity')}</span>
					<h2>{t('scanHistory')}</h2>

					<p className='table-description'>
						{filteredScans.length} {t('result')} · {t('total')} {totalCount}{' '}
						{t('wrote')}
					</p>
				</div>

				<button
					type='button'
					className='ghost'
					onClick={onRefresh}
					disabled={initialLoading || refreshing}
				>
					{refreshing ? (
						<>
							<Spinner />
							{t('refreshing')}
						</>
					) : (
						t('refresh')
					)}
				</button>
			</div>

			<div className='scan-filters'>
				<input
					type='search'
					value={search}
					onChange={event => {
						setSearch(event.target.value)
						setPage(1)
					}}
					placeholder={t('searchURL')}
					aria-label={t('searchURL')}
				/>

				<select
					value={statusFilter}
					onChange={event => {
						setStatusFilter(event.target.value as StatusFilter)
						setPage(1)
					}}
				>
					<option value='all'>{t('allStatuses')}</option>
					<option value='pending'>{t('pending')}</option>
					<option value='processing'>{t('processing')}</option>
					<option value='completed'>{t('completed')}</option>
					<option value='failed'>{t('failed')}</option>
				</select>

				<select
					value={riskFilter}
					onChange={event => {
						setRiskFilter(event.target.value as RiskFilter)
						setPage(1)
					}}
				>
					<option value='all'>{t('allRisks')}</option>
					<option value='unknown'>{t('unknown')}</option>
					<option value='low'>{t('low')}</option>
					<option value='medium'>{t('medium')}</option>
					<option value='high'>{t('high')}</option>
					<option value='critical'>{t('critical')}</option>
				</select>

				<button type='button' className='ghost' onClick={resetFilters}>
					{t('clearFilters')}
				</button>
			</div>

			{error && (
				<p className='error' role='alert'>
					{error}
				</p>
			)}

			<div className='table-wrap'>
				<table>
					<thead>
						<tr>
							<th>{t('url')}</th>
							<th>{t('condition')}</th>
							<th>{t('risk')}</th>
							<th>{t('score')}</th>
							<th>{t('time')}</th>
						</tr>
					</thead>

					<tbody>
						{initialLoading ? (
							<ScanTableSkeleton rows={6} />
						) : error && scans.length === 0 ? (
							<tr>
								<td colSpan={5}>
									<InlineAlert
										variant='error'
										title={t('dashboardLoadErrorTitle')}
										action={{
											label: t('retry'),
											onClick: onRefresh,
										}}
									>
										{error}
									</InlineAlert>
								</td>
							</tr>
						) : scans.length === 0 ? (
							<tr>
								<td colSpan={5}>
									<EmptyState
										title={t('scansEmptyTitle')}
										description={t('scansEmptyDescription')}
									/>
								</td>
							</tr>
						) : filteredScans.length === 0 ? (
							<tr>
								<td colSpan={5}>
									<EmptyState
										title={t('searchEmptyTitle')}
										description={t('searchEmptyDescription')}
									/>
								</td>
							</tr>
						) : (
							visibleScans.map(scan => {
								const indicators = scan.indicators ?? []

								return (
									<tr key={scan.id}>
										<td>
											<button
												type='button'
												className='scan-url'
												onClick={() => onSelectScan(scan.id)}
											>
												{scan.url}
											</button>

											<small>
												{scan.error ||
													indicators.map(item => item.code).join(', ') ||
													t('noIndicatorsProcessing')}
											</small>
										</td>

										<td>
											<span className={`state ${scan.status}`}>
												{statusLabels[scan.status]}
											</span>
										</td>

										<td>
											<span className={`risk ${scan.riskLevel}`}>
												{riskLabels[scan.riskLevel]}
											</span>
										</td>

										<td>{scan.score}/100</td>

										<td>
											{new Date(scan.createdAt).toLocaleString(
												language === 'tk' ? 'tk-TM' : 'en-GB'
											)}
										</td>
									</tr>
								)
							})
						)}
					</tbody>
				</table>
			</div>

			{error && scans.length > 0 && (
				<InlineAlert variant='warning'>{error}</InlineAlert>
			)}

			<div className='pagination'>
				<span>
					{filteredScans.length > 0
						? `${startIndex + 1}–${Math.min(
								startIndex + PAGE_SIZE,
								filteredScans.length
						  )} / ${filteredScans.length} ${t('results')}`
						: `0 ${t('results')}`}
				</span>

				<div>
					<button
						type='button'
						className='ghost'
						onClick={() => setPage(value => Math.max(1, value - 1))}
						disabled={currentPage <= 1}
					>
						{t('previous')}
					</button>

					<span className='pagination-page'>
						{t('page')} {currentPage} {t('of')} {pageCount}
					</span>

					<button
						type='button'
						className='ghost'
						onClick={() => setPage(value => Math.min(pageCount, value + 1))}
						disabled={currentPage >= pageCount}
					>
						{t('next')}
					</button>
				</div>
			</div>

			<p className='table-footnote'>{t('filterScopeNote')}</p>
		</section>
	)
}
