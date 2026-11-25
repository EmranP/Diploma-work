import type { LoginResponse, User, UserRole } from '../types'

const SESSION_KEY = 'phishing-detector.session'

export type AuthSession = {
	accessToken: string
	user: User
	expiresAt: number
}

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === 'object' && value !== null
}

function isUserRole(value: unknown): value is UserRole {
	return value === 'admin' || value === 'user'
}

function isUser(value: unknown): value is User {
	return (
		isRecord(value) &&
		typeof value.id === 'number' &&
		value.id > 0 &&
		typeof value.email === 'string' &&
		isUserRole(value.role)
	)
}

function isAuthSession(value: unknown): value is AuthSession {
	return (
		isRecord(value) &&
		typeof value.accessToken === 'string' &&
		value.accessToken.length > 0 &&
		isUser(value.user) &&
		typeof value.expiresAt === 'number' &&
		Number.isFinite(value.expiresAt)
	)
}

export function readSession(): AuthSession | null {
	const raw = localStorage.getItem(SESSION_KEY)

	if (!raw) {
		return null
	}

	try {
		const parsed: unknown = JSON.parse(raw)

		if (!isAuthSession(parsed) || parsed.expiresAt <= Date.now()) {
			clearSession()
			return null
		}

		return parsed
	} catch {
		clearSession()
		return null
	}
}

export function saveSession(response: LoginResponse): AuthSession {
	const session: AuthSession = {
		accessToken: response.accessToken,
		user: response.user,
		expiresAt: Date.now() + response.expiresIn * 1000,
	}

	localStorage.setItem(SESSION_KEY, JSON.stringify(session))

	return session
}

export function clearSession(): void {
	localStorage.removeItem(SESSION_KEY)
}
