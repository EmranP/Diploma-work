export type UserRole = 'admin' | 'user'

export type ScanStatus = 'pending' | 'processing' | 'completed' | 'failed'

export type RiskLevel = 'unknown' | 'low' | 'medium' | 'high' | 'critical'

export type HealthResponse = {
	status: string
}

export type Indicator = {
	id: number
	scanId: number
	code: string
	description: string
	weight: number
}

export type Scan = {
	id: number
	userId: number
	url: string
	normalizedUrl: string
	status: ScanStatus
	riskLevel: RiskLevel
	score: number
	error?: string
	createdAt: string
	updatedAt: string
	indicators: Indicator[] | null
}

export type User = {
	id: number
	email: string
	role: UserRole
}

export type LoginResponse = {
	accessToken: string
	tokenType: 'Bearer'
	expiresIn: number
	user: User
}

export type ScanListResponse = {
	items: Scan[]
	total: number
	limit: number
}

export type Stats = {
	totalScans: number
	highRiskScans: number
}

export type ApiErrorResponse = {
	error: string
}
