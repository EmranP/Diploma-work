import { useMemo } from 'react'

import { useI18n } from '../../i18n/I18nProvider'
import type { RiskLevel, Scan } from '../../types'
import { EmptyState } from '../ui/EmptyState'
import { RiskDistributionSkeleton } from '../ui/Skeleton'

const riskOrder: RiskLevel[] = ['critical', 'high', 'medium', 'low', 'unknown']

type RiskDistributionProps = {
	scans: Scan[]
	loading?: boolean
}

export function RiskDistribution({
	scans,
	loading = false,
}: RiskDistributionProps) {
	const { t } = useI18n()

	const counts = useMemo(() => {
		const result: Record<RiskLevel, number> = {
			unknown: 0,
			low: 0,
			medium: 0,
			high: 0,
			critical: 0,
		}

		for (const scan of scans) {
			result[scan.riskLevel]++
		}

		return result
	}, [scans])

	const riskLabels: Record<RiskLevel, string> = {
		unknown: t('unknown'),
		low: t('low'),
		medium: t('medium'),
		high: t('high'),
		critical: t('critical'),
	}

	if (loading && scans.length === 0) {
		return <RiskDistributionSkeleton />
	}

	return (
		<section className='risk-distribution'>
			<div className='section-title'>
				<div>
					<span className='eyebrow'>{t('riskAnalytics')}</span>
					<h2>{t('riskDistribution')}</h2>
					<p>
						{t('latestScans')}: {scans.length}
					</p>
				</div>
			</div>

			{scans.length === 0 ? (
				<EmptyState
					title={t('scansEmptyTitle')}
					description={t('scansEmptyDescription')}
				/>
			) : (
				<div className='risk-distribution-list'>
					{riskOrder.map(risk => {
						const count = counts[risk]
						const percentage = Math.round((count / scans.length) * 100)

						return (
							<div className='risk-distribution-row' key={risk}>
								<div className='risk-distribution-label'>
									<span className={`risk ${risk}`}>{riskLabels[risk]}</span>
									<strong>{count}</strong>
								</div>

								<div
									className='risk-bar-track'
									role='progressbar'
									aria-label={riskLabels[risk]}
									aria-valuemin={0}
									aria-valuemax={scans.length}
									aria-valuenow={count}
								>
									<div
										className={`risk-bar-fill ${risk}`}
										style={{ width: `${percentage}%` }}
									/>
								</div>

								<small>{percentage}%</small>
							</div>
						)
					})}
				</div>
			)}
		</section>
	)
}
