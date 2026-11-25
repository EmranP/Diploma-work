import { useEffect, useState } from 'react'

import { ApiError, api } from '../api'
import type { RiskLevel, Scan, ScanStatus, User } from '../types'

import { Sidebar } from '../components/layout/Sidebar'
import { EmptyState } from '../components/ui/EmptyState'
import { InlineAlert } from '../components/ui/InlineAlert'
import { ScanDetailsSkeleton } from '../components/ui/Skeleton'
import { Spinner } from '../components/ui/Spinner'
import { useI18n } from '../i18n/I18nProvider'
import { Language } from '../i18n/translations'

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

type ScanDetailsPageProps = {
	scanId: number
	token: string
	user: User
	onBack: () => void
	onLogout: () => void
}

function getErrorMessage(error: unknown): string {
	return error instanceof Error ? error.message : 'Näsazlyk ýüze çykdy'
}

export function ScanDetailsPage({
	scanId,
	token,
	user,
	onBack,
	onLogout,
}: ScanDetailsPageProps) {
	const [scan, setScan] = useState<Scan | null>(null)
	const { t, language } = useI18n()
	const [loading, setLoading] = useState(true)
	const [refreshing, setRefreshing] = useState(false)
	const [error, setError] = useState('')
	const [reloadVersion, setReloadVersion] = useState(0)
	const [notFound, setNotFound] = useState(false)
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

	function formatDate(value: string, language: Language): string {
		const date = new Date(value)

		if (Number.isNaN(date.getTime())) {
			return '—'
		}

		return date.toLocaleString(language === 'tk' ? 'tk-TM' : 'en-GB', {
			dateStyle: 'medium',
			timeStyle: 'medium',
		})
	}

	function refresh() {
		if (loading || refreshing) return

		setRefreshing(true)
		setReloadVersion(value => value + 1)
	}

	useEffect(() => {
		let active = true

		async function loadScan() {
			setLoading(true)
			setNotFound(false)
			setError('')

			try {
				const result = await api.scan(token, scanId)

				if (active) {
					setScan(result)
				}
			} catch (exception) {
				if (!active) return

				if (exception instanceof ApiError && exception.status === 401) {
					onLogout()
					return
				}

				if (exception instanceof ApiError && exception.status === 404) {
					setNotFound(true)
					return
				}

				setError(getErrorMessage(exception))
			} finally {
				if (active) {
					setLoading(false)
					setRefreshing(false)
				}
			}
		}

		void loadScan()

		return () => {
			active = false
		}
	}, [scanId, token, reloadVersion, onLogout])

	useEffect(() => {
		if (scan?.status !== 'pending' && scan?.status !== 'processing') {
			return
		}

		const timer = window.setTimeout(() => {
			setReloadVersion(value => value + 1)
		}, 5000)

		return () => window.clearTimeout(timer)
	}, [scan?.status, reloadVersion])

	const indicators = scan?.indicators ?? []

	return (
		<main className='app-shell'>
			<Sidebar user={user} onLogout={onLogout} onBack={onBack} />

			<section className='content'>
				<div className='details-toolbar'>
					<button type='button' className='ghost' onClick={onBack}>
						{t('back')}
					</button>

					<button
						type='button'
						className='ghost'
						onClick={refresh}
						disabled={loading || refreshing}
					>
						{refreshing ? (
							<>
								<Spinner />
								{t('refresh')}
							</>
						) : (
							t('refresh')
						)}
					</button>
				</div>

				{loading && !scan && <ScanDetailsSkeleton />}

				{error && !scan && (
					<section className='details-card'>
						<InlineAlert
							variant='error'
							title={t('detailsLoadErrorTitle')}
							action={{
								label: t('retry'),
								onClick: refresh,
							}}
						>
							{error}
						</InlineAlert>
					</section>
				)}

				{error && scan && <InlineAlert variant='warning'>{error}</InlineAlert>}

				{scan && (
					<>
						<header className='details-header'>
							<div>
								<span className='eyebrow'>SCAN #{scan.id}</span>

								<h1>URL seljermesiniň netijesi</h1>

								<p className='details-url'>{scan.url}</p>
							</div>

							<div className='details-badges'>
								<span className={`state ${scan.status}`}>
									{statusLabels[scan.status]}
								</span>

								<span className={`risk ${scan.riskLevel}`}>
									{riskLabels[scan.riskLevel]}
								</span>
							</div>
						</header>

						<section className='details-metrics'>
							<article>
								<span>Risk Score</span>

								<strong>{scan.score}/100</strong>

								<progress
									max={100}
									value={Math.min(100, Math.max(0, scan.score))}
									aria-label='Risk score'
								/>
							</article>

							<article>
								<span>Görkezijiler</span>
								<strong>{indicators.length}</strong>
								<small>Kesgitlenen alamatlar</small>
							</article>

							<article>
								<span>Ýagdaý</span>
								<strong className='details-status'>
									{statusLabels[scan.status]}
								</strong>
								<small>Worker ýagdaýy</small>
							</article>
						</section>

						{scan.error && (
							<section className='details-error'>
								<h2>Seljerme ýalňyşlygy</h2>
								<p>{scan.error}</p>
							</section>
						)}

						{notFound && !scan && (
							<EmptyState
								title={t('scanNotFoundTitle')}
								description={t('scanNotFoundDescription')}
								actionLabel={t('back')}
								onAction={onBack}
							/>
						)}

						<section className='details-card'>
							<div className='section-title'>
								<div>
									<span className='eyebrow'>URL MAGLUMATLARY</span>
									<h2>Esasy maglumatlar</h2>
								</div>
							</div>

							<dl className='details-list'>
								<div>
									<dt>Scan ID</dt>
									<dd>#{scan.id}</dd>
								</div>

								<div>
									<dt>Ulanyjy ID</dt>
									<dd>{scan.userId}</dd>
								</div>

								<div>
									<dt>Asyl URL</dt>
									<dd className='break-word'>{scan.url}</dd>
								</div>

								<div>
									<dt>Normallaşdyrylan URL</dt>
									<dd className='break-word'>{scan.normalizedUrl || '—'}</dd>
								</div>

								<div>
									<dt>Döredilen wagty</dt>
									<dd>{formatDate(scan.createdAt, language)}</dd>
								</div>

								<div>
									<dt>Soňky täzelenme</dt>
									<dd>{formatDate(scan.updatedAt, language)}</dd>
								</div>
							</dl>
						</section>

						<section className='details-card'>
							<div className='section-title'>
								<div>
									<span className='eyebrow'>SECURITY ANALYSIS</span>
									<h2>Howpsuzlyk görkezijileri</h2>
								</div>

								<span className='indicator-count'>{indicators.length}</span>
							</div>

							{indicators.length > 0 ? (
								<div className='indicator-list'>
									{indicators.map(indicator => (
										<article className='indicator-item' key={indicator.id}>
											<div className='indicator-heading'>
												<div>
													<strong>{indicator.code}</strong>
													<p>{indicator.description}</p>
												</div>

												<span className='indicator-weight'>
													+{indicator.weight}
												</span>
											</div>
										</article>
									))}
								</div>
							) : (
								<EmptyState
									title={t('noIndicatorsTitle')}
									description={
										scan.status === 'pending' || scan.status === 'processing'
											? t('noIndicatorsFound')
											: t('noIndicatorsDescription')
									}
								/>
							)}
						</section>

						{scan.status === 'pending' || scan.status === 'processing' ? (
							<p className='notice'>
								URL häzirki wagtda seljerilýär. Netijeleri görmek üçin Täzele
								düwmesine basyň.
							</p>
						) : null}
					</>
				)}
			</section>
		</main>
	)
}
