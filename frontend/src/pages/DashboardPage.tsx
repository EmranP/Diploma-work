import { useEffect, useMemo, useRef, useState } from 'react'
import { ApiError, api } from '../api'
import type { AuthSession } from '../auth/session'
import { MetricCard } from '../components/dashboard/MetricCard'
import { RiskDistribution } from '../components/dashboard/RiskDistribution'
import { ScanForm } from '../components/dashboard/ScanForm'
import { ScanTable } from '../components/dashboard/ScanTable'
import { Sidebar } from '../components/layout/Sidebar'
import { InlineAlert } from '../components/ui/InlineAlert'
import { Modal } from '../components/ui/Modal'
import { useToast } from '../components/ui/ToastProvider'
import { useApiHealth } from '../hooks/useApiHealth'
import { useDashboardData } from '../hooks/useDashboardData'
import { useI18n } from '../i18n/I18nProvider'
import { ScanDetailsPage } from './ScanDetailsPage'

type DashboardPageProps = {
	session: AuthSession
	onLogout: () => void
}

function getErrorMessage(error: unknown): string {
	return error instanceof Error ? error.message : 'Näsazlyk ýüze çykdy'
}

export function DashboardPage({ session, onLogout }: DashboardPageProps) {
	const isAdmin = session.user.role === 'admin'
	const { t } = useI18n()
	const { notify } = useToast()
	const apiHealth = useApiHealth()
	const previousHealth = useRef(apiHealth)
	const [selectedScanId, setSelectedScanId] = useState<number | null>(null)
	const {
		stats,
		scanList,
		loading,
		refreshing,
		loadError,
		statsError,
		refresh,
	} = useDashboardData(session, onLogout, selectedScanId === null)
	const [submitting, setSubmitting] = useState(false)
	const scans = scanList?.items ?? []
	const highRiskCount = useMemo(() => {
		if (isAdmin) {
			return stats?.highRiskScans ?? null
		}

		if (!scanList) return null

		return scans.filter(
			scan => scan.riskLevel === 'high' || scan.riskLevel === 'critical'
		).length
	}, [isAdmin, stats, scans, scanList])
	const activeScans = scans.filter(
		scan => scan.status === 'pending' || scan.status === 'processing'
	)
	const totalScans = isAdmin
		? stats?.totalScans ?? scanList?.total ?? 0
		: scanList?.total ?? 0
	const [isModal, setIsModal] = useState(false)

	useEffect(() => {
		const previous = previousHealth.current

		if (previous === 'offline' && apiHealth === 'online') {
			notify(t('apiRecovered'), 'success')
		}

		if (previous === 'online' && apiHealth === 'offline') {
			notify(t('apiUnavailable'), 'warning')
		}

		previousHealth.current = apiHealth
	}, [apiHealth, notify, t])

	async function handleCreateScan(url: string): Promise<boolean> {
		setSubmitting(true)
		try {
			await api.createScan(session.accessToken, url)

			notify(t('scanQueued'), 'success')
			void refresh()

			return true
		} catch (error) {
			if (error instanceof ApiError && error.status === 401) {
				onLogout()
				return false
			}

			notify(getErrorMessage(error), 'error')
			return false
		} finally {
			setSubmitting(false)
		}
	}

	const onBack = () => setSelectedScanId(null)
	const onCancelLogout = () => setIsModal(false)

	if (selectedScanId !== null) {
		return (
			<ScanDetailsPage
				scanId={selectedScanId}
				token={session.accessToken}
				user={session.user}
				onBack={onBack}
				onLogout={onLogout}
			/>
		)
	}

	return (
		<main className='app-shell' id='dashboard'>
			{isModal && (
				<Modal isModal={isModal} onOk={onLogout} onCancel={onCancelLogout} />
			)}
			<Sidebar
				user={session.user}
				onLogout={() => setIsModal(true)}
				onBack={onBack}
			/>

			<section className='content'>
				<header>
					<div>
						<span className='eyebrow'>{t('securityEyebrow')}</span>

						<h1>{t('dashboardTitle')}</h1>
					</div>

					<span className={`status api-status ${apiHealth}`}>
						<i />
						{t(
							apiHealth === 'online'
								? 'apiOnline'
								: apiHealth === 'offline'
								? 'apiOffline'
								: 'apiChecking'
						)}
					</span>
				</header>

				<section className='metrics'>
					<MetricCard
						title={t('totalScans')}
						value={scanList ? totalScans : '—'}
						description={isAdmin ? t('allUsersScans') : t('yourScans')}
						loading={loading}
					/>

					<MetricCard
						title={t('highRisk')}
						value={highRiskCount ?? '—'}
						description={isAdmin ? t('allScans') : t('latest100')}
						loading={loading}
					/>

					<MetricCard
						title={t('activeScans')}
						value={scanList ? activeScans.length : '—'}
						description={t('latest100Active')}
						loading={loading}
					/>
				</section>

				<RiskDistribution scans={scans} loading={loading} />

				{statsError && isAdmin && (
					<InlineAlert variant='warning'>
						{t('statsLoadError')} {statsError}
					</InlineAlert>
				)}

				<ScanForm loading={submitting} onSubmit={handleCreateScan} />

				<ScanTable
					scans={scans}
					totalCount={scanList?.total ?? 0}
					initialLoading={loading}
					refreshing={refreshing}
					error={loadError}
					onRefresh={() => void refresh()}
					onSelectScan={setSelectedScanId}
				/>
			</section>
		</main>
	)
}
