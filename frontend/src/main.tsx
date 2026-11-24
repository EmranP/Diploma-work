import { FormEvent, useEffect, useMemo, useState } from 'react'
import { createRoot } from 'react-dom/client'
import { api } from './api/api'
import './style/styles.css'
import type { Scan, Stats } from './types/types'

const credentials = {
	email: 'admin@admin.com',
	password: 'qwert',
}
const labels: Record<Scan['riskLevel'], string> = {
	unknown: 'Näbelli',
	low: 'Pes',
	medium: 'Orta',
	high: 'Ýokary',
	critical: 'Örän ýokary',
}

function Login({ onLogin }: { onLogin: (token: string) => void }) {
	const [email, setEmail] = useState(credentials.email)
	const [password, setPassword] = useState(credentials.password)
	const [error, setError] = useState('')
	const [loading, setLoading] = useState(false)

	async function submit(event: FormEvent) {
		event.preventDefault()
		setLoading(true)
		setError('')
		try {
			const result = await api.login(email, password)
			onLogin(result.accessToken)
		} catch (exception) {
			setError(
				exception instanceof Error ? exception.message : 'Näsazlyk ýüze çykdy'
			)
		} finally {
			setLoading(false)
		}
	}

	return (
		<main className='login-page'>
			<form className='login-card' onSubmit={submit}>
				<span className='eyebrow'>TÜRKMEN TELEKOM</span>
				<h1>Fişing ýüze çykaryş ulgamy</h1>
				<p>
					Şübheli URL salgylaryny seljermek we töwekgelçiligi bahalandyrmak üçin
					analitik paneli.
				</p>
				<label>
					Email
					<input
						value={email}
						onChange={event => setEmail(event.target.value)}
						type='email'
					/>
				</label>
				<label>
					Parol
					<input
						value={password}
						onChange={event => setPassword(event.target.value)}
						type='password'
					/>
				</label>
				{error && <p className='error'>{error}</p>}
				<button disabled={loading}>
					{loading ? 'Girilýär...' : 'Ulgama girmek'}
				</button>
			</form>
		</main>
	)
}

function Dashboard({
	token,
	onLogout,
}: {
	token: string
	onLogout: () => void
}) {
	const [stats, setStats] = useState<Stats>({ totalScans: 0, highRiskScans: 0 })
	const [scans, setScans] = useState<Scan[]>([])
	const [url, setURL] = useState('')
	const [message, setMessage] = useState('')
	const [loading, setLoading] = useState(false)
	const highPercent = useMemo(
		() =>
			stats.totalScans
				? Math.round((stats.highRiskScans / stats.totalScans) * 100)
				: 0,
		[stats]
	)

	async function load() {
		const [nextStats, nextScans] = await Promise.all([
			api.stats(token),
			api.scans(token),
		])
		setStats(nextStats)
		setScans(nextScans.items)
	}
	useEffect(() => {
		load().catch((error: Error) => setMessage(error.message))
		const timer = window.setInterval(() => load().catch(() => undefined), 3000)
		return () => window.clearInterval(timer)
	}, [])
	async function submit(event: FormEvent) {
		event.preventDefault()
		if (!url.trim()) return
		setLoading(true)
		setMessage('')
		try {
			await api.createScan(token, url)
			setURL('')
			setMessage('URL nobata iberildi')
			await load()
		} catch (exception) {
			setMessage(
				exception instanceof Error ? exception.message : 'Näsazlyk ýüze çykdy'
			)
		} finally {
			setLoading(false)
		}
	}
	return (
		<main className='app-shell'>
			<aside>
				<div className='brand'>
					<span>TT</span>
					<strong>PhishGuard</strong>
				</div>
				<nav>
					<a className='active'>Dolandyryş paneli</a>
					<a>Barlaglaryň taryhy</a>
					<a>Audit žurnaly</a>
				</nav>
				<button className='logout' onClick={onLogout}>
					Çykmak
				</button>
			</aside>
			<section className='content'>
				<header>
					<div>
						<span className='eyebrow'>HOWPSUZLYK GÖZEGÇILIGI</span>
						<h1>Fişing hüjümleriniň seljermesi</h1>
					</div>
					<span className='status'>
						<i /> Ulgam işleýär
					</span>
				</header>
				<div className='metrics'>
					<article>
						<span>Jemi barlaglar</span>
						<strong>{stats.totalScans}</strong>
						<small>Ýazga alnan URL-ler</small>
					</article>
					<article>
						<span>Ýokary töwekgelçilik</span>
						<strong>{stats.highRiskScans}</strong>
						<small>{highPercent}% umumy barlagdan</small>
					</article>
					<article>
						<span>Soňky täzelenme</span>
						<strong>3 s</strong>
						<small>Awtomatiki sinhronizasiýa</small>
					</article>
				</div>
				<section className='scan-card'>
					<div>
						<h2>URL salgysyny barlamak</h2>
						<p>
							Salgy URL gurluşy, domen, şübheli sözler we howpsuzlyk alamatlary
							boýunça seljeriler.
						</p>
					</div>
					<form onSubmit={submit}>
						<input
							value={url}
							onChange={event => setURL(event.target.value)}
							placeholder='https://example.com/login'
						/>
						<button disabled={loading}>
							{loading ? 'Barlanýar...' : 'Barlamak'}
						</button>
					</form>
					{message && <p className='notice'>{message}</p>}
				</section>
				<section className='table-card'>
					<div className='section-title'>
						<div>
							<span className='eyebrow'>SOŇKY IŞJEŇLIK</span>
							<h2>Soňky barlaglar</h2>
						</div>
						<button className='ghost' onClick={() => load()}>
							Täzelemek
						</button>
					</div>
					<div className='table-wrap'>
						<table>
							<thead>
								<tr>
									<th>URL</th>
									<th>Ýagdaý</th>
									<th>Töwekgelçilik</th>
									<th>Bal</th>
									<th>Wagt</th>
								</tr>
							</thead>
							<tbody>
								{scans.map(scan => (
									<tr key={scan.id}>
										<td>
											<strong>{scan.url}</strong>
											<small>
												{scan.indicators
													.map(indicator => indicator.code)
													.join(', ') || 'Seljerme garaşylýar'}
											</small>
										</td>
										<td>
											<span className={`state ${scan.status}`}>
												{scan.status}
											</span>
										</td>
										<td>
											<span className={`risk ${scan.riskLevel}`}>
												{labels[scan.riskLevel]}
											</span>
										</td>
										<td>{scan.score}/100</td>
										<td>{new Date(scan.createdAt).toLocaleString('tk-TM')}</td>
									</tr>
								))}
								{!scans.length && (
									<tr>
										<td colSpan={5} className='empty'>
											Soňky barlaglar ýok
										</td>
									</tr>
								)}
							</tbody>
						</table>
					</div>
				</section>
			</section>
		</main>
	)
}

function App() {
	const [token, setToken] = useState(
		() => localStorage.getItem('accessToken') ?? ''
	)
	const logout = () => {
		localStorage.removeItem('accessToken')
		setToken('')
	}
	const login = (nextToken: string) => {
		localStorage.setItem('accessToken', nextToken)
		setToken(nextToken)
	}
	return token ? (
		<Dashboard token={token} onLogout={logout} />
	) : (
		<Login onLogin={login} />
	)
}

createRoot(document.getElementById('root')!).render(<App />)
