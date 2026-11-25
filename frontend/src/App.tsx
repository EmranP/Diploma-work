import { useCallback, useState } from 'react'

import { DashboardPage } from './pages/DashboardPage'
import { LoginPage } from './pages/LoginPage'

import {
	clearSession,
	readSession,
	saveSession,
	type AuthSession,
} from './auth/session'

import type { LoginResponse } from './types'

export function App() {
	const [session, setSession] = useState<AuthSession | null>(() =>
		readSession()
	)

	const handleLogin = useCallback((response: LoginResponse) => {
		const nextSession = saveSession(response)
		setSession(nextSession)
	}, [])

	const handleLogout = useCallback(() => {
		clearSession()
		setSession(null)
	}, [])

	if (!session) {
		return <LoginPage onLogin={handleLogin} />
	}

	return (
		<DashboardPage
			key={session.accessToken}
			session={session}
			onLogout={handleLogout}
		/>
	)
}
