import { type FormEvent, useState } from 'react'
import { LanguageSwitcher } from '../components/layout/LanguageSwitcher'
import { useI18n } from '../i18n/I18nProvider'

import { api } from '../api'
import { InlineAlert } from '../components/ui/InlineAlert'
import { Spinner } from '../components/ui/Spinner'
import type { LoginResponse } from '../types'

type LoginPageProps = {
	onLogin: (response: LoginResponse) => void
}

function getErrorMessage(error: unknown): string {
	return error instanceof Error ? error.message : 'Näsazlyk ýüze çykdy'
}

export function LoginPage({ onLogin }: LoginPageProps) {
	const { t } = useI18n()
	const [email, setEmail] = useState('')
	const [password, setPassword] = useState('')
	const [error, setError] = useState('')
	const [loading, setLoading] = useState(false)

	async function submit(event: FormEvent<HTMLFormElement>) {
		event.preventDefault()

		if (loading) return

		setLoading(true)
		setError('')

		try {
			const response = await api.login(email.trim(), password)
			onLogin(response)
		} catch (error) {
			setError(getErrorMessage(error))
		} finally {
			setLoading(false)
		}
	}

	return (
		<main className='login-page'>
			<form className='login-card' onSubmit={submit}>
				<LanguageSwitcher />

				<span className='eyebrow'>{t('loginEyebrow')}</span>

				<h1>{t('loginTitle')}</h1>
				<p>{t('loginDescription')}</p>

				<label>
					{t('email')}
					<input
						type='email'
						value={email}
						onChange={event => setEmail(event.target.value)}
						autoComplete='username'
						placeholder={t('emailPlaceholder')}
						required
						disabled={loading}
					/>
				</label>

				<label>
					{t('password')}
					<input
						type='password'
						value={password}
						onChange={event => setPassword(event.target.value)}
						autoComplete='current-password'
						placeholder={t('passwordPlaceholder')}
						required
						disabled={loading}
					/>
				</label>

				{error && <InlineAlert variant='error'>{error}</InlineAlert>}

				<button type='submit' disabled={loading}>
					{loading ? (
						<>
							<Spinner />
							{t('signingIn')}
						</>
					) : (
						t('login')
					)}
				</button>
			</form>
		</main>
	)
}
