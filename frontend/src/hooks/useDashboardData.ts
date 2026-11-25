import { useCallback, useEffect, useRef, useState } from 'react'

import { ApiError, api } from '../api'
import type { AuthSession } from '../auth/session'
import type { ScanListResponse, Stats } from '../types'

const REFRESH_INTERVAL = 8_000
const DASHBOARD_SCAN_LIMIT = 100

type UseDashboardDataResult = {
	stats: Stats | null
	scanList: ScanListResponse | null
	loading: boolean
	refreshing: boolean
	loadError: string
	statsError: string
	refresh: () => Promise<void>
}

function getErrorMessage(error: unknown): string {
	return error instanceof Error ? error.message : 'Näsazlyk ýüze çykdy'
}

export function useDashboardData(
	session: AuthSession,
	onUnauthorized: () => void,
	enabled = true
): UseDashboardDataResult {
	const isAdmin = session.user.role === 'admin'

	const [stats, setStats] = useState<Stats | null>(null)
	const [scanList, setScanList] = useState<ScanListResponse | null>(null)

	const [loading, setLoading] = useState(true)
	const [refreshing, setRefreshing] = useState(false)

	const [loadError, setLoadError] = useState('')
	const [statsError, setStatsError] = useState('')

	const mountedRef = useRef(false)
	const hasLoadedRef = useRef(false)
	const requestInFlight = useRef(false)

	useEffect(() => {
		mountedRef.current = true

		return () => {
			mountedRef.current = false
		}
	}, [])

	const refresh = useCallback(async () => {
		if (requestInFlight.current) return

		requestInFlight.current = true

		if (!hasLoadedRef.current) {
			setLoading(true)
		} else {
			setRefreshing(true)
		}

		try {
			const nextScans = await api.scans(
				session.accessToken,
				DASHBOARD_SCAN_LIMIT
			)

			if (!mountedRef.current) return

			setScanList(nextScans)
			setLoadError('')

			if (isAdmin) {
				try {
					const nextStats = await api.stats(session.accessToken)

					if (mountedRef.current) {
						setStats(nextStats)
						setStatsError('')
					}
				} catch (error) {
					if (error instanceof ApiError && error.status === 401) {
						throw error
					}

					if (mountedRef.current) {
						setStatsError(getErrorMessage(error))
					}
				}
			} else if (mountedRef.current) {
				setStats(null)
				setStatsError('')
			}
		} catch (error) {
			if (error instanceof ApiError && error.status === 401) {
				onUnauthorized()
				return
			}

			if (mountedRef.current) {
				setLoadError(getErrorMessage(error))
			}
		} finally {
			requestInFlight.current = false
			hasLoadedRef.current = true

			if (mountedRef.current) {
				setLoading(false)
				setRefreshing(false)
			}
		}
	}, [session.accessToken, isAdmin, onUnauthorized])

	useEffect(() => {
		if (!enabled) return

		let active = true
		let timer: number | undefined

		async function poll() {
			await refresh()

			if (active) {
				timer = window.setTimeout(poll, REFRESH_INTERVAL)
			}
		}

		void poll()

		return () => {
			active = false

			if (timer !== undefined) {
				window.clearTimeout(timer)
			}
		}
	}, [refresh, enabled])

	return {
		stats,
		scanList,
		loading,
		refreshing,
		loadError,
		statsError,
		refresh,
	}
}
