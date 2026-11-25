import { useEffect, useState } from 'react'

import { api } from '../api'

export type ApiHealthStatus = 'checking' | 'online' | 'offline'

const HEALTH_CHECK_INTERVAL = 10_000
const HEALTH_CHECK_TIMEOUT = 4_000

export function useApiHealth(): ApiHealthStatus {
	const [status, setStatus] = useState<ApiHealthStatus>('checking')

	useEffect(() => {
		let active = true
		let timer: number | undefined
		let controller: AbortController | undefined

		async function check() {
			const currentController = new AbortController()
			controller = currentController

			const timeout = window.setTimeout(
				() => currentController.abort(),
				HEALTH_CHECK_TIMEOUT
			)

			try {
				const response = await api.health(currentController.signal)

				if (active) {
					setStatus(response.status === 'ok' ? 'online' : 'offline')
				}
			} catch {
				if (active) {
					setStatus('offline')
				}
			} finally {
				window.clearTimeout(timeout)

				if (active) {
					timer = window.setTimeout(check, HEALTH_CHECK_INTERVAL)
				}
			}
		}

		void check()

		return () => {
			active = false

			if (timer !== undefined) {
				window.clearTimeout(timer)
			}

			controller?.abort()
		}
	}, [])

	return status
}
