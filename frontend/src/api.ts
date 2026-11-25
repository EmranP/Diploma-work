import type {
	HealthResponse,
	LoginResponse,
	Scan,
	ScanListResponse,
	Stats,
} from './types'

const API_URL = (
	import.meta.env.VITE_API_URL ?? 'http://localhost:8080'
).replace(/\/+$/, '')

export class ApiError extends Error {
	constructor(message: string, public readonly status: number) {
		super(message)
		this.name = 'ApiError'
	}
}

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === 'object' && value !== null
}

async function request<T>(
	path: string,
	token?: string,
	init?: RequestInit
): Promise<T> {
	const headers = new Headers(init?.headers)

	if (init?.body !== undefined && !headers.has('Content-Type')) {
		headers.set('Content-Type', 'application/json')
	}

	if (token) {
		headers.set('Authorization', `Bearer ${token}`)
	}

	const response = await fetch(`${API_URL}${path}`, {
		...init,
		headers,
	})

	const responseText = await response.text()

	let body: unknown

	if (responseText) {
		try {
			body = JSON.parse(responseText)
		} catch {
			body = responseText
		}
	}

	if (!response.ok) {
		const message =
			isRecord(body) && typeof body.error === 'string'
				? body.error
				: `Request failed with status ${response.status}`

		throw new ApiError(message, response.status)
	}

	return body as T
}

export const api = {
	login: (email: string, password: string) =>
		request<LoginResponse>('/api/auth/login', undefined, {
			method: 'POST',
			body: JSON.stringify({ email, password }),
		}),

	stats: (token: string) => request<Stats>('/api/stats', token),

	scans: (token: string, limit = 20) => {
		const params = new URLSearchParams({
			limit: String(limit),
		})

		return request<ScanListResponse>(`/api/scans?${params.toString()}`, token)
	},

	createScan: (token: string, url: string) =>
		request<Scan>('/api/scans', token, {
			method: 'POST',
			body: JSON.stringify({ url }),
		}),

	scan: (token: string, id: number) => request<Scan>(`/api/scans/${id}`, token),

	health: (signal?: AbortSignal) =>
		request<HealthResponse>('/health', undefined, {
			signal,
		}),
}
